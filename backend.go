package dynago

import "context"

// Backend is the interface that must be implemented by any DynamoDB backend
// (e.g. AWS SDK adapter, in-memory test backend). All request and response
// types are library-owned — no AWS SDK types leak through this interface.
type Backend interface {
	GetItem(ctx context.Context, req *GetItemRequest) (*GetItemResponse, error)
	PutItem(ctx context.Context, req *PutItemRequest) (*PutItemResponse, error)
	DeleteItem(ctx context.Context, req *DeleteItemRequest) (*DeleteItemResponse, error)
	UpdateItem(ctx context.Context, req *UpdateItemRequest) (*UpdateItemResponse, error)
	Query(ctx context.Context, req *QueryRequest) (*QueryResponse, error)
	Scan(ctx context.Context, req *ScanRequest) (*ScanResponse, error)
	BatchGetItem(ctx context.Context, req *BatchGetItemRequest) (*BatchGetItemResponse, error)
	BatchWriteItem(ctx context.Context, req *BatchWriteItemRequest) (*BatchWriteItemResponse, error)
	TransactGetItems(ctx context.Context, req *TransactGetItemsRequest) (*TransactGetItemsResponse, error)
	TransactWriteItems(ctx context.Context, req *TransactWriteItemsRequest) (*TransactWriteItemsResponse, error)
}

// ---------------------------------------------------------------------------
// ConsumedCapacity
// ---------------------------------------------------------------------------

// ConsumedCapacity reports the capacity units consumed by an operation.
type ConsumedCapacity struct {
	TableName    string
	CapacityUnits float64
	ReadCapacityUnits  float64
	WriteCapacityUnits float64
}

// ---------------------------------------------------------------------------
// GetItem
// ---------------------------------------------------------------------------

type GetItemRequest struct {
	TableName               string
	Key                     map[string]AttributeValue
	ConsistentRead          bool
	ProjectionExpression    string
	ExpressionAttributeNames map[string]string
}

type GetItemResponse struct {
	Item             map[string]AttributeValue
	ConsumedCapacity *ConsumedCapacity
}

// ---------------------------------------------------------------------------
// PutItem
// ---------------------------------------------------------------------------

type PutItemRequest struct {
	TableName                 string
	Item                      map[string]AttributeValue
	ConditionExpression       string
	ExpressionAttributeNames  map[string]string
	ExpressionAttributeValues map[string]AttributeValue
	ReturnValues              string
}

type PutItemResponse struct {
	ConsumedCapacity *ConsumedCapacity
}

// ---------------------------------------------------------------------------
// DeleteItem
// ---------------------------------------------------------------------------

type DeleteItemRequest struct {
	TableName                 string
	Key                       map[string]AttributeValue
	ConditionExpression       string
	ExpressionAttributeNames  map[string]string
	ExpressionAttributeValues map[string]AttributeValue
	ReturnValues              string
}

type DeleteItemResponse struct {
	ConsumedCapacity *ConsumedCapacity
}

// ---------------------------------------------------------------------------
// UpdateItem
// ---------------------------------------------------------------------------

type UpdateItemRequest struct {
	TableName                 string
	Key                       map[string]AttributeValue
	UpdateExpression          string
	ConditionExpression       string
	ExpressionAttributeNames  map[string]string
	ExpressionAttributeValues map[string]AttributeValue
	ReturnValues              string
}

type UpdateItemResponse struct {
	Attributes       map[string]AttributeValue
	ConsumedCapacity *ConsumedCapacity
}

// ---------------------------------------------------------------------------
// Query
// ---------------------------------------------------------------------------

type QueryRequest struct {
	TableName                 string
	IndexName                 string
	KeyConditionExpression    string
	FilterExpression          string
	ProjectionExpression      string
	ExpressionAttributeNames  map[string]string
	ExpressionAttributeValues map[string]AttributeValue
	Limit                     int32
	ScanIndexForward          *bool
	ExclusiveStartKey         map[string]AttributeValue
	ConsistentRead            bool
}

type QueryResponse struct {
	Items            []map[string]AttributeValue
	Count            int32
	LastEvaluatedKey map[string]AttributeValue
	ConsumedCapacity *ConsumedCapacity
}

// ---------------------------------------------------------------------------
// Scan
// ---------------------------------------------------------------------------

type ScanRequest struct {
	TableName                 string
	IndexName                 string
	FilterExpression          string
	ProjectionExpression      string
	ExpressionAttributeNames  map[string]string
	ExpressionAttributeValues map[string]AttributeValue
	Limit                     int32
	ExclusiveStartKey         map[string]AttributeValue
	ConsistentRead            bool
}

type ScanResponse struct {
	Items            []map[string]AttributeValue
	Count            int32
	LastEvaluatedKey map[string]AttributeValue
	ConsumedCapacity *ConsumedCapacity
}

// ---------------------------------------------------------------------------
// BatchGetItem
// ---------------------------------------------------------------------------

type BatchGetItemRequest struct {
	RequestItems map[string]KeysAndProjection
}

type KeysAndProjection struct {
	Keys                     []map[string]AttributeValue
	ProjectionExpression     string
	ExpressionAttributeNames map[string]string
	ConsistentRead           bool
}

type BatchGetItemResponse struct {
	Responses        map[string][]map[string]AttributeValue
	UnprocessedKeys  map[string]KeysAndProjection
	ConsumedCapacity []ConsumedCapacity
}

// ---------------------------------------------------------------------------
// BatchWriteItem
// ---------------------------------------------------------------------------

type BatchWriteItemRequest struct {
	RequestItems map[string][]WriteRequest
}

type WriteRequest struct {
	PutItem    *PutRequest
	DeleteItem *DeleteRequest
}

type PutRequest struct {
	Item map[string]AttributeValue
}

type DeleteRequest struct {
	Key map[string]AttributeValue
}

type BatchWriteItemResponse struct {
	UnprocessedItems map[string][]WriteRequest
	ConsumedCapacity []ConsumedCapacity
}

// ---------------------------------------------------------------------------
// TransactGetItems
// ---------------------------------------------------------------------------

type TransactGetItemsRequest struct {
	TransactItems []TransactGetItem
}

type TransactGetItem struct {
	TableName               string
	Key                     map[string]AttributeValue
	ProjectionExpression    string
	ExpressionAttributeNames map[string]string
}

type TransactGetItemsResponse struct {
	Responses        []map[string]AttributeValue
	ConsumedCapacity []ConsumedCapacity
}

// ---------------------------------------------------------------------------
// TransactWriteItems
// ---------------------------------------------------------------------------

type TransactWriteItemsRequest struct {
	TransactItems []TransactWriteItem
}

type TransactWriteItem struct {
	Put            *TransactPut
	Delete         *TransactDelete
	Update         *TransactUpdate
	ConditionCheck *TransactConditionCheck
}

type TransactPut struct {
	TableName                 string
	Item                      map[string]AttributeValue
	ConditionExpression       string
	ExpressionAttributeNames  map[string]string
	ExpressionAttributeValues map[string]AttributeValue
}

type TransactDelete struct {
	TableName                 string
	Key                       map[string]AttributeValue
	ConditionExpression       string
	ExpressionAttributeNames  map[string]string
	ExpressionAttributeValues map[string]AttributeValue
}

type TransactUpdate struct {
	TableName                 string
	Key                       map[string]AttributeValue
	UpdateExpression          string
	ConditionExpression       string
	ExpressionAttributeNames  map[string]string
	ExpressionAttributeValues map[string]AttributeValue
}

type TransactConditionCheck struct {
	TableName                 string
	Key                       map[string]AttributeValue
	ConditionExpression       string
	ExpressionAttributeNames  map[string]string
	ExpressionAttributeValues map[string]AttributeValue
}

type TransactWriteItemsResponse struct {
	ConsumedCapacity []ConsumedCapacity
}
