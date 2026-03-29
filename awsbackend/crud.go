package awsbackend

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/danielmensah/dynago"
)

// GetItem implements dynago.Backend.
func (b *AWSBackend) GetItem(ctx context.Context, req *dynago.GetItemRequest) (*dynago.GetItemResponse, error) {
	input := &dynamodb.GetItemInput{
		TableName:      aws.String(req.TableName),
		Key:            toAWSKey(req.Key),
		ConsistentRead: aws.Bool(req.ConsistentRead),
	}
	if req.ProjectionExpression != "" {
		input.ProjectionExpression = aws.String(req.ProjectionExpression)
	}
	if len(req.ExpressionAttributeNames) > 0 {
		input.ExpressionAttributeNames = req.ExpressionAttributeNames
	}

	out, err := b.client.GetItem(ctx, input)
	if err != nil {
		return nil, wrapAWSError(err)
	}

	resp := &dynago.GetItemResponse{
		Item: fromAWSItem(out.Item),
	}
	if out.ConsumedCapacity != nil {
		resp.ConsumedCapacity = fromAWSConsumedCapacity(out.ConsumedCapacity)
	}
	return resp, nil
}

// PutItem implements dynago.Backend.
func (b *AWSBackend) PutItem(ctx context.Context, req *dynago.PutItemRequest) (*dynago.PutItemResponse, error) {
	input := &dynamodb.PutItemInput{
		TableName: aws.String(req.TableName),
		Item:      toAWSItem(req.Item),
	}
	if req.ConditionExpression != "" {
		input.ConditionExpression = aws.String(req.ConditionExpression)
	}
	if len(req.ExpressionAttributeNames) > 0 {
		input.ExpressionAttributeNames = req.ExpressionAttributeNames
	}
	if len(req.ExpressionAttributeValues) > 0 {
		input.ExpressionAttributeValues = toAWSItem(req.ExpressionAttributeValues)
	}
	if req.ReturnValues != "" {
		input.ReturnValues = dbtypes.ReturnValue(req.ReturnValues)
	}

	out, err := b.client.PutItem(ctx, input)
	if err != nil {
		return nil, wrapAWSError(err)
	}

	resp := &dynago.PutItemResponse{}
	if out.ConsumedCapacity != nil {
		resp.ConsumedCapacity = fromAWSConsumedCapacity(out.ConsumedCapacity)
	}
	return resp, nil
}

// DeleteItem implements dynago.Backend.
func (b *AWSBackend) DeleteItem(ctx context.Context, req *dynago.DeleteItemRequest) (*dynago.DeleteItemResponse, error) {
	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(req.TableName),
		Key:       toAWSKey(req.Key),
	}
	if req.ConditionExpression != "" {
		input.ConditionExpression = aws.String(req.ConditionExpression)
	}
	if len(req.ExpressionAttributeNames) > 0 {
		input.ExpressionAttributeNames = req.ExpressionAttributeNames
	}
	if len(req.ExpressionAttributeValues) > 0 {
		input.ExpressionAttributeValues = toAWSItem(req.ExpressionAttributeValues)
	}
	if req.ReturnValues != "" {
		input.ReturnValues = dbtypes.ReturnValue(req.ReturnValues)
	}

	out, err := b.client.DeleteItem(ctx, input)
	if err != nil {
		return nil, wrapAWSError(err)
	}

	resp := &dynago.DeleteItemResponse{}
	if out.ConsumedCapacity != nil {
		resp.ConsumedCapacity = fromAWSConsumedCapacity(out.ConsumedCapacity)
	}
	return resp, nil
}

// UpdateItem implements dynago.Backend.
func (b *AWSBackend) UpdateItem(ctx context.Context, req *dynago.UpdateItemRequest) (*dynago.UpdateItemResponse, error) {
	input := &dynamodb.UpdateItemInput{
		TableName: aws.String(req.TableName),
		Key:       toAWSKey(req.Key),
	}
	if req.UpdateExpression != "" {
		input.UpdateExpression = aws.String(req.UpdateExpression)
	}
	if req.ConditionExpression != "" {
		input.ConditionExpression = aws.String(req.ConditionExpression)
	}
	if len(req.ExpressionAttributeNames) > 0 {
		input.ExpressionAttributeNames = req.ExpressionAttributeNames
	}
	if len(req.ExpressionAttributeValues) > 0 {
		input.ExpressionAttributeValues = toAWSItem(req.ExpressionAttributeValues)
	}
	if req.ReturnValues != "" {
		input.ReturnValues = dbtypes.ReturnValue(req.ReturnValues)
	}

	out, err := b.client.UpdateItem(ctx, input)
	if err != nil {
		return nil, wrapAWSError(err)
	}

	resp := &dynago.UpdateItemResponse{
		Attributes: fromAWSItem(out.Attributes),
	}
	if out.ConsumedCapacity != nil {
		resp.ConsumedCapacity = fromAWSConsumedCapacity(out.ConsumedCapacity)
	}
	return resp, nil
}

// fromAWSConsumedCapacity converts AWS ConsumedCapacity to dynago.ConsumedCapacity.
func fromAWSConsumedCapacity(cc *dbtypes.ConsumedCapacity) *dynago.ConsumedCapacity {
	if cc == nil {
		return nil
	}
	out := &dynago.ConsumedCapacity{}
	if cc.TableName != nil {
		out.TableName = *cc.TableName
	}
	if cc.CapacityUnits != nil {
		out.CapacityUnits = *cc.CapacityUnits
	}
	if cc.ReadCapacityUnits != nil {
		out.ReadCapacityUnits = *cc.ReadCapacityUnits
	}
	if cc.WriteCapacityUnits != nil {
		out.WriteCapacityUnits = *cc.WriteCapacityUnits
	}
	return out
}
