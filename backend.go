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
	TableName          string
	CapacityUnits      float64
	ReadCapacityUnits  float64
	WriteCapacityUnits float64
}

// ---------------------------------------------------------------------------
// GetItem
// ---------------------------------------------------------------------------

// GetItemRequest describes a GetItem operation.
type GetItemRequest struct {
	TableName                string
	Key                      map[string]AttributeValue
	ConsistentRead           bool
	ProjectionExpression     string
	ExpressionAttributeNames map[string]string
}

// GetItemResponse is the result of a GetItem operation. Item is nil when the
// key does not exist.
type GetItemResponse struct {
	Item             map[string]AttributeValue
	ConsumedCapacity *ConsumedCapacity
}

// ---------------------------------------------------------------------------
// PutItem
// ---------------------------------------------------------------------------

// PutItemRequest describes a PutItem operation.
type PutItemRequest struct {
	TableName                 string
	Item                      map[string]AttributeValue
	ConditionExpression       string
	ExpressionAttributeNames  map[string]string
	ExpressionAttributeValues map[string]AttributeValue
	ReturnValues              string
}

// PutItemResponse is the result of a PutItem operation.
type PutItemResponse struct {
	ConsumedCapacity *ConsumedCapacity
}

// ---------------------------------------------------------------------------
// DeleteItem
// ---------------------------------------------------------------------------

// DeleteItemRequest describes a DeleteItem operation.
type DeleteItemRequest struct {
	TableName                 string
	Key                       map[string]AttributeValue
	ConditionExpression       string
	ExpressionAttributeNames  map[string]string
	ExpressionAttributeValues map[string]AttributeValue
	ReturnValues              string
}

// DeleteItemResponse is the result of a DeleteItem operation.
type DeleteItemResponse struct {
	ConsumedCapacity *ConsumedCapacity
}

// ---------------------------------------------------------------------------
// UpdateItem
// ---------------------------------------------------------------------------

// UpdateItemRequest describes an UpdateItem operation.
type UpdateItemRequest struct {
	TableName                 string
	Key                       map[string]AttributeValue
	UpdateExpression          string
	ConditionExpression       string
	ExpressionAttributeNames  map[string]string
	ExpressionAttributeValues map[string]AttributeValue
	ReturnValues              string
}

// UpdateItemResponse is the result of an UpdateItem operation. Attributes is
// populated when ReturnValues is set.
type UpdateItemResponse struct {
	Attributes       map[string]AttributeValue
	ConsumedCapacity *ConsumedCapacity
}

// ---------------------------------------------------------------------------
// Query
// ---------------------------------------------------------------------------

// QueryRequest describes a Query operation.
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

// QueryResponse is the result of a Query operation. LastEvaluatedKey is
// non-nil when more pages are available.
type QueryResponse struct {
	Items            []map[string]AttributeValue
	Count            int32
	ScannedCount     int32
	LastEvaluatedKey map[string]AttributeValue
	ConsumedCapacity *ConsumedCapacity
}

// ---------------------------------------------------------------------------
// Scan
// ---------------------------------------------------------------------------

// ScanRequest describes a Scan operation.
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

// ScanResponse is the result of a Scan operation. LastEvaluatedKey is non-nil
// when more pages are available.
type ScanResponse struct {
	Items            []map[string]AttributeValue
	Count            int32
	ScannedCount     int32
	LastEvaluatedKey map[string]AttributeValue
	ConsumedCapacity *ConsumedCapacity
}

// ---------------------------------------------------------------------------
// BatchGetItem
// ---------------------------------------------------------------------------

// BatchGetItemRequest describes a BatchGetItem operation.
type BatchGetItemRequest struct {
	RequestItems map[string]KeysAndProjection
}

// KeysAndProjection groups keys and an optional projection for a single table
// in a BatchGetItem request.
type KeysAndProjection struct {
	Keys                     []map[string]AttributeValue
	ProjectionExpression     string
	ExpressionAttributeNames map[string]string
	ConsistentRead           bool
}

// BatchGetItemResponse is the result of a BatchGetItem operation.
// UnprocessedKeys contains keys that were not processed due to throughput limits.
type BatchGetItemResponse struct {
	Responses        map[string][]map[string]AttributeValue
	UnprocessedKeys  map[string]KeysAndProjection
	ConsumedCapacity []ConsumedCapacity
}

// ---------------------------------------------------------------------------
// BatchWriteItem
// ---------------------------------------------------------------------------

// BatchWriteItemRequest describes a BatchWriteItem operation.
type BatchWriteItemRequest struct {
	RequestItems map[string][]WriteRequest
}

// WriteRequest is a single put or delete within a BatchWriteItem call.
// Exactly one of PutItem or DeleteItem must be set.
type WriteRequest struct {
	PutItem    *PutRequest
	DeleteItem *DeleteRequest
}

// PutRequest is the put-item payload within a WriteRequest.
type PutRequest struct {
	Item map[string]AttributeValue
}

// DeleteRequest is the delete-item payload within a WriteRequest.
type DeleteRequest struct {
	Key map[string]AttributeValue
}

// BatchWriteItemResponse is the result of a BatchWriteItem operation.
// UnprocessedItems contains requests that were not processed due to throughput limits.
type BatchWriteItemResponse struct {
	UnprocessedItems map[string][]WriteRequest
	ConsumedCapacity []ConsumedCapacity
}

// ---------------------------------------------------------------------------
// TransactGetItems
// ---------------------------------------------------------------------------

// TransactGetItemsRequest describes a TransactGetItems operation.
type TransactGetItemsRequest struct {
	TransactItems []TransactGetItem
}

// TransactGetItem is a single get within a read transaction.
type TransactGetItem struct {
	TableName                string
	Key                      map[string]AttributeValue
	ProjectionExpression     string
	ExpressionAttributeNames map[string]string
}

// TransactGetItemsResponse is the result of a TransactGetItems operation.
// Responses are ordered to match the input TransactItems.
type TransactGetItemsResponse struct {
	Responses        []map[string]AttributeValue
	ConsumedCapacity []ConsumedCapacity
}

// ---------------------------------------------------------------------------
// TransactWriteItems
// ---------------------------------------------------------------------------

// TransactWriteItemsRequest describes a TransactWriteItems operation.
type TransactWriteItemsRequest struct {
	TransactItems []TransactWriteItem
}

// TransactWriteItem is a single operation within a write transaction. Exactly
// one of Put, Delete, Update, or ConditionCheck must be set.
type TransactWriteItem struct {
	Put            *TransactPut
	Delete         *TransactDelete
	Update         *TransactUpdate
	ConditionCheck *TransactConditionCheck
}

// TransactPut is a put operation within a write transaction.
type TransactPut struct {
	TableName                 string
	Item                      map[string]AttributeValue
	ConditionExpression       string
	ExpressionAttributeNames  map[string]string
	ExpressionAttributeValues map[string]AttributeValue
}

// TransactDelete is a delete operation within a write transaction.
type TransactDelete struct {
	TableName                 string
	Key                       map[string]AttributeValue
	ConditionExpression       string
	ExpressionAttributeNames  map[string]string
	ExpressionAttributeValues map[string]AttributeValue
}

// TransactUpdate is an update operation within a write transaction.
type TransactUpdate struct {
	TableName                 string
	Key                       map[string]AttributeValue
	UpdateExpression          string
	ConditionExpression       string
	ExpressionAttributeNames  map[string]string
	ExpressionAttributeValues map[string]AttributeValue
}

// TransactConditionCheck is a condition-only check within a write transaction.
// It verifies a condition without modifying any data.
type TransactConditionCheck struct {
	TableName                 string
	Key                       map[string]AttributeValue
	ConditionExpression       string
	ExpressionAttributeNames  map[string]string
	ExpressionAttributeValues map[string]AttributeValue
}

// TransactWriteItemsResponse is the result of a TransactWriteItems operation.
type TransactWriteItemsResponse struct {
	ConsumedCapacity []ConsumedCapacity
}
