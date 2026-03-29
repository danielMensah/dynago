package aws

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/danielmensah/dynago"
)

// mockClient implements DynamoDBAPI for testing.
type mockClient struct {
	// Captured inputs
	getItemInput            *dynamodb.GetItemInput
	putItemInput            *dynamodb.PutItemInput
	deleteItemInput         *dynamodb.DeleteItemInput
	updateItemInput         *dynamodb.UpdateItemInput
	queryInput              *dynamodb.QueryInput
	scanInput               *dynamodb.ScanInput
	batchGetItemInput       *dynamodb.BatchGetItemInput
	batchWriteItemInput     *dynamodb.BatchWriteItemInput
	transactGetItemsInput   *dynamodb.TransactGetItemsInput
	transactWriteItemsInput *dynamodb.TransactWriteItemsInput

	// Configured outputs
	getItemOutput            *dynamodb.GetItemOutput
	putItemOutput            *dynamodb.PutItemOutput
	deleteItemOutput         *dynamodb.DeleteItemOutput
	updateItemOutput         *dynamodb.UpdateItemOutput
	queryOutput              *dynamodb.QueryOutput
	scanOutput               *dynamodb.ScanOutput
	batchGetItemOutput       *dynamodb.BatchGetItemOutput
	batchWriteItemOutput     *dynamodb.BatchWriteItemOutput
	transactGetItemsOutput   *dynamodb.TransactGetItemsOutput
	transactWriteItemsOutput *dynamodb.TransactWriteItemsOutput

	// Configured error
	err error
}

func (m *mockClient) GetItem(_ context.Context, input *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	m.getItemInput = input
	if m.err != nil {
		return nil, m.err
	}
	return m.getItemOutput, nil
}

func (m *mockClient) PutItem(_ context.Context, input *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	m.putItemInput = input
	if m.err != nil {
		return nil, m.err
	}
	return m.putItemOutput, nil
}

func (m *mockClient) DeleteItem(_ context.Context, input *dynamodb.DeleteItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
	m.deleteItemInput = input
	if m.err != nil {
		return nil, m.err
	}
	return m.deleteItemOutput, nil
}

func (m *mockClient) UpdateItem(_ context.Context, input *dynamodb.UpdateItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	m.updateItemInput = input
	if m.err != nil {
		return nil, m.err
	}
	return m.updateItemOutput, nil
}

func (m *mockClient) Query(_ context.Context, input *dynamodb.QueryInput, _ ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
	m.queryInput = input
	if m.err != nil {
		return nil, m.err
	}
	return m.queryOutput, nil
}

func (m *mockClient) Scan(_ context.Context, input *dynamodb.ScanInput, _ ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
	m.scanInput = input
	if m.err != nil {
		return nil, m.err
	}
	return m.scanOutput, nil
}

func (m *mockClient) BatchGetItem(_ context.Context, input *dynamodb.BatchGetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.BatchGetItemOutput, error) {
	m.batchGetItemInput = input
	if m.err != nil {
		return nil, m.err
	}
	return m.batchGetItemOutput, nil
}

func (m *mockClient) BatchWriteItem(_ context.Context, input *dynamodb.BatchWriteItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.BatchWriteItemOutput, error) {
	m.batchWriteItemInput = input
	if m.err != nil {
		return nil, m.err
	}
	return m.batchWriteItemOutput, nil
}

func (m *mockClient) TransactGetItems(_ context.Context, input *dynamodb.TransactGetItemsInput, _ ...func(*dynamodb.Options)) (*dynamodb.TransactGetItemsOutput, error) {
	m.transactGetItemsInput = input
	if m.err != nil {
		return nil, m.err
	}
	return m.transactGetItemsOutput, nil
}

func (m *mockClient) TransactWriteItems(_ context.Context, input *dynamodb.TransactWriteItemsInput, _ ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error) {
	m.transactWriteItemsInput = input
	if m.err != nil {
		return nil, m.err
	}
	return m.transactWriteItemsOutput, nil
}

// ---------------------------------------------------------------------------
// GetItem tests
// ---------------------------------------------------------------------------

func TestGetItem_TranslatesRequest(t *testing.T) {
	mock := &mockClient{
		getItemOutput: &dynamodb.GetItemOutput{
			Item: map[string]dbtypes.AttributeValue{
				"pk": &dbtypes.AttributeValueMemberS{Value: "user#1"},
				"name": &dbtypes.AttributeValueMemberS{Value: "Alice"},
			},
		},
	}
	backend := NewAWSBackend(mock)

	resp, err := backend.GetItem(context.Background(), &dynago.GetItemRequest{
		TableName: "Users",
		Key: map[string]dynago.AttributeValue{
			"pk": {Type: dynago.TypeS, S: "user#1"},
		},
		ConsistentRead:       true,
		ProjectionExpression: "#n",
		ExpressionAttributeNames: map[string]string{"#n": "name"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify input was translated
	if *mock.getItemInput.TableName != "Users" {
		t.Errorf("expected table Users, got %s", *mock.getItemInput.TableName)
	}
	if !*mock.getItemInput.ConsistentRead {
		t.Error("expected ConsistentRead=true")
	}
	if *mock.getItemInput.ProjectionExpression != "#n" {
		t.Errorf("expected projection #n, got %s", *mock.getItemInput.ProjectionExpression)
	}

	// Verify response was translated
	if resp.Item["pk"].S != "user#1" {
		t.Errorf("expected pk=user#1, got %s", resp.Item["pk"].S)
	}
	if resp.Item["name"].S != "Alice" {
		t.Errorf("expected name=Alice, got %s", resp.Item["name"].S)
	}
}

func TestGetItem_EmptyResponse(t *testing.T) {
	mock := &mockClient{
		getItemOutput: &dynamodb.GetItemOutput{},
	}
	backend := NewAWSBackend(mock)

	resp, err := backend.GetItem(context.Background(), &dynago.GetItemRequest{
		TableName: "Users",
		Key: map[string]dynago.AttributeValue{
			"pk": {Type: dynago.TypeS, S: "missing"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Backend returns nil item; higher-level Get[T] checks for not-found
	if resp.Item != nil {
		t.Errorf("expected nil item, got %v", resp.Item)
	}
}

// ---------------------------------------------------------------------------
// PutItem tests
// ---------------------------------------------------------------------------

func TestPutItem_TranslatesCondition(t *testing.T) {
	mock := &mockClient{
		putItemOutput: &dynamodb.PutItemOutput{},
	}
	backend := NewAWSBackend(mock)

	_, err := backend.PutItem(context.Background(), &dynago.PutItemRequest{
		TableName: "Users",
		Item: map[string]dynago.AttributeValue{
			"pk":   {Type: dynago.TypeS, S: "user#1"},
			"name": {Type: dynago.TypeS, S: "Alice"},
		},
		ConditionExpression:      "attribute_not_exists(pk)",
		ExpressionAttributeNames: map[string]string{"#pk": "pk"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if *mock.putItemInput.ConditionExpression != "attribute_not_exists(pk)" {
		t.Errorf("condition not passed through")
	}
	if mock.putItemInput.ExpressionAttributeNames["#pk"] != "pk" {
		t.Errorf("expression attribute names not passed through")
	}
}

// ---------------------------------------------------------------------------
// UpdateItem tests
// ---------------------------------------------------------------------------

func TestUpdateItem_TranslatesExpressions(t *testing.T) {
	mock := &mockClient{
		updateItemOutput: &dynamodb.UpdateItemOutput{
			Attributes: map[string]dbtypes.AttributeValue{
				"pk":   &dbtypes.AttributeValueMemberS{Value: "user#1"},
				"name": &dbtypes.AttributeValueMemberS{Value: "Bob"},
			},
		},
	}
	backend := NewAWSBackend(mock)

	resp, err := backend.UpdateItem(context.Background(), &dynago.UpdateItemRequest{
		TableName: "Users",
		Key: map[string]dynago.AttributeValue{
			"pk": {Type: dynago.TypeS, S: "user#1"},
		},
		UpdateExpression:    "SET #n = :name",
		ConditionExpression: "attribute_exists(pk)",
		ExpressionAttributeNames: map[string]string{
			"#n": "name",
		},
		ExpressionAttributeValues: map[string]dynago.AttributeValue{
			":name": {Type: dynago.TypeS, S: "Bob"},
		},
		ReturnValues: "ALL_NEW",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if *mock.updateItemInput.UpdateExpression != "SET #n = :name" {
		t.Error("update expression not passed through")
	}
	if *mock.updateItemInput.ConditionExpression != "attribute_exists(pk)" {
		t.Error("condition expression not passed through")
	}
	if mock.updateItemInput.ReturnValues != dbtypes.ReturnValueAllNew {
		t.Errorf("expected ALL_NEW, got %s", mock.updateItemInput.ReturnValues)
	}
	// Verify expression attribute values were translated
	nameVal, ok := mock.updateItemInput.ExpressionAttributeValues[":name"]
	if !ok {
		t.Fatal("expected :name in expression attribute values")
	}
	if s, ok := nameVal.(*dbtypes.AttributeValueMemberS); !ok || s.Value != "Bob" {
		t.Errorf("expected :name=Bob, got %v", nameVal)
	}

	// Verify response attributes
	if resp.Attributes["name"].S != "Bob" {
		t.Errorf("expected name=Bob in response, got %s", resp.Attributes["name"].S)
	}
}

// ---------------------------------------------------------------------------
// DeleteItem tests
// ---------------------------------------------------------------------------

func TestDeleteItem_TranslatesRequest(t *testing.T) {
	mock := &mockClient{
		deleteItemOutput: &dynamodb.DeleteItemOutput{},
	}
	backend := NewAWSBackend(mock)

	_, err := backend.DeleteItem(context.Background(), &dynago.DeleteItemRequest{
		TableName: "Users",
		Key: map[string]dynago.AttributeValue{
			"pk": {Type: dynago.TypeS, S: "user#1"},
		},
		ConditionExpression: "attribute_exists(pk)",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if *mock.deleteItemInput.TableName != "Users" {
		t.Error("table name not passed through")
	}
	if *mock.deleteItemInput.ConditionExpression != "attribute_exists(pk)" {
		t.Error("condition expression not passed through")
	}
}

// ---------------------------------------------------------------------------
// Error wrapping tests
// ---------------------------------------------------------------------------

func TestWrapAWSError_ConditionalCheckFailed(t *testing.T) {
	awsErr := &dbtypes.ConditionalCheckFailedException{
		Message: aws.String("condition failed"),
	}
	err := wrapAWSError(awsErr)
	if !errors.Is(err, dynago.ErrConditionFailed) {
		t.Errorf("expected ErrConditionFailed, got %v", err)
	}
}

func TestWrapAWSError_TransactionCancelled(t *testing.T) {
	awsErr := &dbtypes.TransactionCanceledException{
		Message: aws.String("tx cancelled"),
	}
	err := wrapAWSError(awsErr)
	if !errors.Is(err, dynago.ErrTransactionCancelled) {
		t.Errorf("expected ErrTransactionCancelled, got %v", err)
	}
}

func TestWrapAWSError_ResourceNotFound(t *testing.T) {
	awsErr := &dbtypes.ResourceNotFoundException{
		Message: aws.String("table not found"),
	}
	err := wrapAWSError(awsErr)
	if !errors.Is(err, dynago.ErrValidation) {
		t.Errorf("expected ErrValidation, got %v", err)
	}
}

func TestWrapAWSError_ValidationException(t *testing.T) {
	// ValidationException is not a typed error in the DynamoDB SDK;
	// it comes as a generic error with "ValidationException" in the message.
	awsErr := errors.New("ValidationException: bad input")
	err := wrapAWSError(awsErr)
	if !errors.Is(err, dynago.ErrValidation) {
		t.Errorf("expected ErrValidation, got %v", err)
	}
}

func TestWrapAWSError_Nil(t *testing.T) {
	if err := wrapAWSError(nil); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestWrapAWSError_Unknown(t *testing.T) {
	orig := errors.New("some unknown error")
	err := wrapAWSError(orig)
	if err != orig {
		t.Errorf("expected original error to pass through, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Query tests
// ---------------------------------------------------------------------------

func TestQuery_TranslatesRequest(t *testing.T) {
	scanForward := true
	mock := &mockClient{
		queryOutput: &dynamodb.QueryOutput{
			Items: []map[string]dbtypes.AttributeValue{
				{"pk": &dbtypes.AttributeValueMemberS{Value: "user#1"}},
			},
			Count: 1,
			LastEvaluatedKey: map[string]dbtypes.AttributeValue{
				"pk": &dbtypes.AttributeValueMemberS{Value: "user#1"},
			},
		},
	}
	backend := NewAWSBackend(mock)

	resp, err := backend.Query(context.Background(), &dynago.QueryRequest{
		TableName:              "Users",
		IndexName:              "GSI1",
		KeyConditionExpression: "pk = :pk",
		FilterExpression:       "age > :age",
		ProjectionExpression:   "#n, #a",
		ExpressionAttributeNames: map[string]string{
			"#n": "name", "#a": "age",
		},
		ExpressionAttributeValues: map[string]dynago.AttributeValue{
			":pk":  {Type: dynago.TypeS, S: "user#1"},
			":age": {Type: dynago.TypeN, N: "18"},
		},
		Limit:            10,
		ScanIndexForward: &scanForward,
		ConsistentRead:   true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if *mock.queryInput.TableName != "Users" {
		t.Error("table name mismatch")
	}
	if *mock.queryInput.IndexName != "GSI1" {
		t.Error("index name mismatch")
	}
	if *mock.queryInput.KeyConditionExpression != "pk = :pk" {
		t.Error("key condition mismatch")
	}
	if *mock.queryInput.Limit != 10 {
		t.Errorf("expected limit 10, got %d", *mock.queryInput.Limit)
	}

	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(resp.Items))
	}
	if resp.Items[0]["pk"].S != "user#1" {
		t.Error("item translation failed")
	}
	if resp.Count != 1 {
		t.Errorf("expected count 1, got %d", resp.Count)
	}
	if resp.LastEvaluatedKey["pk"].S != "user#1" {
		t.Error("last evaluated key translation failed")
	}
}

// ---------------------------------------------------------------------------
// Scan tests
// ---------------------------------------------------------------------------

func TestScan_TranslatesRequest(t *testing.T) {
	mock := &mockClient{
		scanOutput: &dynamodb.ScanOutput{
			Items: []map[string]dbtypes.AttributeValue{
				{"pk": &dbtypes.AttributeValueMemberS{Value: "item#1"}},
				{"pk": &dbtypes.AttributeValueMemberS{Value: "item#2"}},
			},
			Count: 2,
		},
	}
	backend := NewAWSBackend(mock)

	resp, err := backend.Scan(context.Background(), &dynago.ScanRequest{
		TableName:        "Items",
		FilterExpression: "active = :v",
		ExpressionAttributeValues: map[string]dynago.AttributeValue{
			":v": {Type: dynago.TypeBOOL, BOOL: true},
		},
		Limit: 5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if *mock.scanInput.TableName != "Items" {
		t.Error("table name mismatch")
	}
	if *mock.scanInput.Limit != 5 {
		t.Errorf("expected limit 5, got %d", *mock.scanInput.Limit)
	}
	if len(resp.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(resp.Items))
	}
	if resp.Count != 2 {
		t.Errorf("expected count 2, got %d", resp.Count)
	}
}

// ---------------------------------------------------------------------------
// PutItem error wrapping integration test
// ---------------------------------------------------------------------------

func TestPutItem_ConditionFailed(t *testing.T) {
	mock := &mockClient{
		err: &dbtypes.ConditionalCheckFailedException{
			Message: aws.String("condition not met"),
		},
	}
	backend := NewAWSBackend(mock)

	_, err := backend.PutItem(context.Background(), &dynago.PutItemRequest{
		TableName: "Users",
		Item: map[string]dynago.AttributeValue{
			"pk": {Type: dynago.TypeS, S: "user#1"},
		},
		ConditionExpression: "attribute_not_exists(pk)",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, dynago.ErrConditionFailed) {
		t.Errorf("expected ErrConditionFailed, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Backend interface compliance
// ---------------------------------------------------------------------------

func TestAWSBackend_ImplementsBackend(t *testing.T) {
	var _ dynago.Backend = (*AWSBackend)(nil)
}
