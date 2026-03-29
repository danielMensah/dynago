package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/danielmensah/dynago"
)

// Query implements dynago.Backend.
func (b *AWSBackend) Query(ctx context.Context, req *dynago.QueryRequest) (*dynago.QueryResponse, error) {
	input := &dynamodb.QueryInput{
		TableName: aws.String(req.TableName),
	}
	if req.IndexName != "" {
		input.IndexName = aws.String(req.IndexName)
	}
	if req.KeyConditionExpression != "" {
		input.KeyConditionExpression = aws.String(req.KeyConditionExpression)
	}
	if req.FilterExpression != "" {
		input.FilterExpression = aws.String(req.FilterExpression)
	}
	if req.ProjectionExpression != "" {
		input.ProjectionExpression = aws.String(req.ProjectionExpression)
	}
	if len(req.ExpressionAttributeNames) > 0 {
		input.ExpressionAttributeNames = req.ExpressionAttributeNames
	}
	if len(req.ExpressionAttributeValues) > 0 {
		input.ExpressionAttributeValues = toAWSItem(req.ExpressionAttributeValues)
	}
	if req.Limit > 0 {
		input.Limit = aws.Int32(req.Limit)
	}
	if req.ScanIndexForward != nil {
		input.ScanIndexForward = req.ScanIndexForward
	}
	if len(req.ExclusiveStartKey) > 0 {
		input.ExclusiveStartKey = toAWSKey(req.ExclusiveStartKey)
	}
	if req.ConsistentRead {
		input.ConsistentRead = aws.Bool(true)
	}

	out, err := b.client.Query(ctx, input)
	if err != nil {
		return nil, wrapAWSError(err)
	}

	items := make([]map[string]dynago.AttributeValue, len(out.Items))
	for i, item := range out.Items {
		items[i] = fromAWSItem(item)
	}

	resp := &dynago.QueryResponse{
		Items:            items,
		Count:            out.Count,
		LastEvaluatedKey: fromAWSItem(out.LastEvaluatedKey),
	}
	if out.ConsumedCapacity != nil {
		resp.ConsumedCapacity = fromAWSConsumedCapacity(out.ConsumedCapacity)
	}
	return resp, nil
}

// Scan implements dynago.Backend.
func (b *AWSBackend) Scan(ctx context.Context, req *dynago.ScanRequest) (*dynago.ScanResponse, error) {
	input := &dynamodb.ScanInput{
		TableName: aws.String(req.TableName),
	}
	if req.IndexName != "" {
		input.IndexName = aws.String(req.IndexName)
	}
	if req.FilterExpression != "" {
		input.FilterExpression = aws.String(req.FilterExpression)
	}
	if req.ProjectionExpression != "" {
		input.ProjectionExpression = aws.String(req.ProjectionExpression)
	}
	if len(req.ExpressionAttributeNames) > 0 {
		input.ExpressionAttributeNames = req.ExpressionAttributeNames
	}
	if len(req.ExpressionAttributeValues) > 0 {
		input.ExpressionAttributeValues = toAWSItem(req.ExpressionAttributeValues)
	}
	if req.Limit > 0 {
		input.Limit = aws.Int32(req.Limit)
	}
	if len(req.ExclusiveStartKey) > 0 {
		input.ExclusiveStartKey = toAWSKey(req.ExclusiveStartKey)
	}
	if req.ConsistentRead {
		input.ConsistentRead = aws.Bool(true)
	}

	out, err := b.client.Scan(ctx, input)
	if err != nil {
		return nil, wrapAWSError(err)
	}

	items := make([]map[string]dynago.AttributeValue, len(out.Items))
	for i, item := range out.Items {
		items[i] = fromAWSItem(item)
	}

	resp := &dynago.ScanResponse{
		Items:            items,
		Count:            out.Count,
		LastEvaluatedKey: fromAWSItem(out.LastEvaluatedKey),
	}
	if out.ConsumedCapacity != nil {
		resp.ConsumedCapacity = fromAWSConsumedCapacity(out.ConsumedCapacity)
	}
	return resp, nil
}
