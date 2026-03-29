package awsbackend

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/danielmensah/dynago"
)

// BatchGetItem implements dynago.Backend.
func (b *AWSBackend) BatchGetItem(ctx context.Context, req *dynago.BatchGetItemRequest) (*dynago.BatchGetItemResponse, error) {
	input := &dynamodb.BatchGetItemInput{
		RequestItems: make(map[string]dbtypes.KeysAndAttributes, len(req.RequestItems)),
	}

	for table, kp := range req.RequestItems {
		keys := make([]map[string]dbtypes.AttributeValue, len(kp.Keys))
		for i, k := range kp.Keys {
			keys[i] = toAWSKey(k)
		}
		ka := dbtypes.KeysAndAttributes{
			Keys: keys,
		}
		if kp.ProjectionExpression != "" {
			ka.ProjectionExpression = aws.String(kp.ProjectionExpression)
		}
		if len(kp.ExpressionAttributeNames) > 0 {
			ka.ExpressionAttributeNames = kp.ExpressionAttributeNames
		}
		if kp.ConsistentRead {
			ka.ConsistentRead = aws.Bool(true)
		}
		input.RequestItems[table] = ka
	}

	out, err := b.client.BatchGetItem(ctx, input)
	if err != nil {
		return nil, wrapAWSError(err)
	}

	resp := &dynago.BatchGetItemResponse{}

	// Convert responses
	if len(out.Responses) > 0 {
		resp.Responses = make(map[string][]map[string]dynago.AttributeValue, len(out.Responses))
		for table, items := range out.Responses {
			converted := make([]map[string]dynago.AttributeValue, len(items))
			for i, item := range items {
				converted[i] = fromAWSItem(item)
			}
			resp.Responses[table] = converted
		}
	}

	// Convert unprocessed keys
	if len(out.UnprocessedKeys) > 0 {
		resp.UnprocessedKeys = make(map[string]dynago.KeysAndProjection, len(out.UnprocessedKeys))
		for table, ka := range out.UnprocessedKeys {
			keys := make([]map[string]dynago.AttributeValue, len(ka.Keys))
			for i, k := range ka.Keys {
				keys[i] = fromAWSItem(k)
			}
			kp := dynago.KeysAndProjection{
				Keys: keys,
			}
			if ka.ProjectionExpression != nil {
				kp.ProjectionExpression = *ka.ProjectionExpression
			}
			if ka.ExpressionAttributeNames != nil {
				kp.ExpressionAttributeNames = ka.ExpressionAttributeNames
			}
			if ka.ConsistentRead != nil {
				kp.ConsistentRead = *ka.ConsistentRead
			}
			resp.UnprocessedKeys[table] = kp
		}
	}

	// Convert consumed capacity
	if len(out.ConsumedCapacity) > 0 {
		resp.ConsumedCapacity = make([]dynago.ConsumedCapacity, len(out.ConsumedCapacity))
		for i, cc := range out.ConsumedCapacity {
			if c := fromAWSConsumedCapacity(&cc); c != nil {
				resp.ConsumedCapacity[i] = *c
			}
		}
	}

	return resp, nil
}

// BatchWriteItem implements dynago.Backend.
func (b *AWSBackend) BatchWriteItem(ctx context.Context, req *dynago.BatchWriteItemRequest) (*dynago.BatchWriteItemResponse, error) {
	input := &dynamodb.BatchWriteItemInput{
		RequestItems: make(map[string][]dbtypes.WriteRequest, len(req.RequestItems)),
	}

	for table, wrs := range req.RequestItems {
		awsWrs := make([]dbtypes.WriteRequest, len(wrs))
		for i, wr := range wrs {
			if wr.PutItem != nil {
				awsWrs[i] = dbtypes.WriteRequest{
					PutRequest: &dbtypes.PutRequest{
						Item: toAWSItem(wr.PutItem.Item),
					},
				}
			} else if wr.DeleteItem != nil {
				awsWrs[i] = dbtypes.WriteRequest{
					DeleteRequest: &dbtypes.DeleteRequest{
						Key: toAWSKey(wr.DeleteItem.Key),
					},
				}
			}
		}
		input.RequestItems[table] = awsWrs
	}

	out, err := b.client.BatchWriteItem(ctx, input)
	if err != nil {
		return nil, wrapAWSError(err)
	}

	resp := &dynago.BatchWriteItemResponse{}

	// Convert unprocessed items
	if len(out.UnprocessedItems) > 0 {
		resp.UnprocessedItems = make(map[string][]dynago.WriteRequest, len(out.UnprocessedItems))
		for table, wrs := range out.UnprocessedItems {
			converted := make([]dynago.WriteRequest, len(wrs))
			for i, wr := range wrs {
				if wr.PutRequest != nil {
					converted[i] = dynago.WriteRequest{
						PutItem: &dynago.PutRequest{
							Item: fromAWSItem(wr.PutRequest.Item),
						},
					}
				} else if wr.DeleteRequest != nil {
					converted[i] = dynago.WriteRequest{
						DeleteItem: &dynago.DeleteRequest{
							Key: fromAWSItem(wr.DeleteRequest.Key),
						},
					}
				}
			}
			resp.UnprocessedItems[table] = converted
		}
	}

	// Convert consumed capacity
	if len(out.ConsumedCapacity) > 0 {
		resp.ConsumedCapacity = make([]dynago.ConsumedCapacity, len(out.ConsumedCapacity))
		for i, cc := range out.ConsumedCapacity {
			if c := fromAWSConsumedCapacity(&cc); c != nil {
				resp.ConsumedCapacity[i] = *c
			}
		}
	}

	return resp, nil
}

// TransactGetItems implements dynago.Backend.
func (b *AWSBackend) TransactGetItems(ctx context.Context, req *dynago.TransactGetItemsRequest) (*dynago.TransactGetItemsResponse, error) {
	items := make([]dbtypes.TransactGetItem, len(req.TransactItems))
	for i, tgi := range req.TransactItems {
		get := &dbtypes.Get{
			TableName: aws.String(tgi.TableName),
			Key:       toAWSKey(tgi.Key),
		}
		if tgi.ProjectionExpression != "" {
			get.ProjectionExpression = aws.String(tgi.ProjectionExpression)
		}
		if len(tgi.ExpressionAttributeNames) > 0 {
			get.ExpressionAttributeNames = tgi.ExpressionAttributeNames
		}
		items[i] = dbtypes.TransactGetItem{Get: get}
	}

	out, err := b.client.TransactGetItems(ctx, &dynamodb.TransactGetItemsInput{
		TransactItems: items,
	})
	if err != nil {
		return nil, wrapAWSError(err)
	}

	resp := &dynago.TransactGetItemsResponse{}
	if len(out.Responses) > 0 {
		resp.Responses = make([]map[string]dynago.AttributeValue, len(out.Responses))
		for i, r := range out.Responses {
			resp.Responses[i] = fromAWSItem(r.Item)
		}
	}

	if len(out.ConsumedCapacity) > 0 {
		resp.ConsumedCapacity = make([]dynago.ConsumedCapacity, len(out.ConsumedCapacity))
		for i, cc := range out.ConsumedCapacity {
			if c := fromAWSConsumedCapacity(&cc); c != nil {
				resp.ConsumedCapacity[i] = *c
			}
		}
	}

	return resp, nil
}

// TransactWriteItems implements dynago.Backend.
func (b *AWSBackend) TransactWriteItems(ctx context.Context, req *dynago.TransactWriteItemsRequest) (*dynago.TransactWriteItemsResponse, error) {
	items := make([]dbtypes.TransactWriteItem, len(req.TransactItems))
	for i, twi := range req.TransactItems {
		var awsTwi dbtypes.TransactWriteItem

		if twi.Put != nil {
			put := &dbtypes.Put{
				TableName: aws.String(twi.Put.TableName),
				Item:      toAWSItem(twi.Put.Item),
			}
			if twi.Put.ConditionExpression != "" {
				put.ConditionExpression = aws.String(twi.Put.ConditionExpression)
			}
			if len(twi.Put.ExpressionAttributeNames) > 0 {
				put.ExpressionAttributeNames = twi.Put.ExpressionAttributeNames
			}
			if len(twi.Put.ExpressionAttributeValues) > 0 {
				put.ExpressionAttributeValues = toAWSItem(twi.Put.ExpressionAttributeValues)
			}
			awsTwi.Put = put
		}

		if twi.Delete != nil {
			del := &dbtypes.Delete{
				TableName: aws.String(twi.Delete.TableName),
				Key:       toAWSKey(twi.Delete.Key),
			}
			if twi.Delete.ConditionExpression != "" {
				del.ConditionExpression = aws.String(twi.Delete.ConditionExpression)
			}
			if len(twi.Delete.ExpressionAttributeNames) > 0 {
				del.ExpressionAttributeNames = twi.Delete.ExpressionAttributeNames
			}
			if len(twi.Delete.ExpressionAttributeValues) > 0 {
				del.ExpressionAttributeValues = toAWSItem(twi.Delete.ExpressionAttributeValues)
			}
			awsTwi.Delete = del
		}

		if twi.Update != nil {
			upd := &dbtypes.Update{
				TableName: aws.String(twi.Update.TableName),
				Key:       toAWSKey(twi.Update.Key),
			}
			if twi.Update.UpdateExpression != "" {
				upd.UpdateExpression = aws.String(twi.Update.UpdateExpression)
			}
			if twi.Update.ConditionExpression != "" {
				upd.ConditionExpression = aws.String(twi.Update.ConditionExpression)
			}
			if len(twi.Update.ExpressionAttributeNames) > 0 {
				upd.ExpressionAttributeNames = twi.Update.ExpressionAttributeNames
			}
			if len(twi.Update.ExpressionAttributeValues) > 0 {
				upd.ExpressionAttributeValues = toAWSItem(twi.Update.ExpressionAttributeValues)
			}
			awsTwi.Update = upd
		}

		if twi.ConditionCheck != nil {
			cc := &dbtypes.ConditionCheck{
				TableName: aws.String(twi.ConditionCheck.TableName),
				Key:       toAWSKey(twi.ConditionCheck.Key),
			}
			if twi.ConditionCheck.ConditionExpression != "" {
				cc.ConditionExpression = aws.String(twi.ConditionCheck.ConditionExpression)
			}
			if len(twi.ConditionCheck.ExpressionAttributeNames) > 0 {
				cc.ExpressionAttributeNames = twi.ConditionCheck.ExpressionAttributeNames
			}
			if len(twi.ConditionCheck.ExpressionAttributeValues) > 0 {
				cc.ExpressionAttributeValues = toAWSItem(twi.ConditionCheck.ExpressionAttributeValues)
			}
			awsTwi.ConditionCheck = cc
		}

		items[i] = awsTwi
	}

	out, err := b.client.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{
		TransactItems: items,
	})
	if err != nil {
		return nil, wrapAWSError(err)
	}

	resp := &dynago.TransactWriteItemsResponse{}
	if len(out.ConsumedCapacity) > 0 {
		resp.ConsumedCapacity = make([]dynago.ConsumedCapacity, len(out.ConsumedCapacity))
		for i, cc := range out.ConsumedCapacity {
			if c := fromAWSConsumedCapacity(&cc); c != nil {
				resp.ConsumedCapacity[i] = *c
			}
		}
	}

	return resp, nil
}
