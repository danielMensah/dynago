package awsbackend

import (
	"context"
	"errors"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/danielmensah/dynago"
)

// DynamoDBAPI is the subset of [dynamodb.Client] methods used by [AWSBackend].
// Accepting an interface enables unit testing with mock implementations.
type DynamoDBAPI interface {
	GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	PutItem(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
	DeleteItem(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error)
	UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
	Query(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
	Scan(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error)
	BatchGetItem(ctx context.Context, params *dynamodb.BatchGetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.BatchGetItemOutput, error)
	BatchWriteItem(ctx context.Context, params *dynamodb.BatchWriteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.BatchWriteItemOutput, error)
	TransactGetItems(ctx context.Context, params *dynamodb.TransactGetItemsInput, optFns ...func(*dynamodb.Options)) (*dynamodb.TransactGetItemsOutput, error)
	TransactWriteItems(ctx context.Context, params *dynamodb.TransactWriteItemsInput, optFns ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error)
}

// compile-time check that *dynamodb.Client satisfies our interface
var _ DynamoDBAPI = (*dynamodb.Client)(nil)

// AWSBackend implements dynago.Backend using the AWS SDK for Go v2.
type AWSBackend struct {
	client DynamoDBAPI
}

// compile-time check that AWSBackend satisfies dynago.Backend
var _ dynago.Backend = (*AWSBackend)(nil)

// NewAWSBackend creates a new AWSBackend wrapping the given DynamoDB client.
// For most use cases prefer [NewFromConfig], which creates the client internally.
func NewAWSBackend(client DynamoDBAPI) *AWSBackend {
	return &AWSBackend{client: client}
}

// NewFromConfig creates a new AWSBackend using the given AWS config.
// Options are passed through to dynamodb.NewFromConfig.
//
// Example:
//
//	cfg, err := config.LoadDefaultConfig(ctx)
//	if err != nil { /* handle error */ }
//	backend := awsbackend.NewFromConfig(cfg)
func NewFromConfig(cfg aws.Config, opts ...func(*dynamodb.Options)) *AWSBackend {
	client := dynamodb.NewFromConfig(cfg, opts...)
	return &AWSBackend{client: client}
}

// wrapAWSError translates AWS SDK errors into dynago error types.
func wrapAWSError(err error) error {
	if err == nil {
		return nil
	}

	var condFailed *types.ConditionalCheckFailedException
	if errors.As(err, &condFailed) {
		return &dynago.Error{Sentinel: dynago.ErrConditionFailed, Cause: err}
	}

	var txCancelled *types.TransactionCanceledException
	if errors.As(err, &txCancelled) {
		reasons := make([]dynago.TxCancelReason, len(txCancelled.CancellationReasons))
		for i, cr := range txCancelled.CancellationReasons {
			var code, msg string
			if cr.Code != nil {
				code = *cr.Code
			}
			if cr.Message != nil {
				msg = *cr.Message
			}
			reasons[i] = dynago.TxCancelReason{
				Code:    code,
				Message: msg,
			}
		}
		return &dynago.TxCancelledError{Reasons: reasons}
	}

	var resNotFound *types.ResourceNotFoundException
	if errors.As(err, &resNotFound) {
		return &dynago.Error{Sentinel: dynago.ErrValidation, Cause: err, Message: "table not found"}
	}

	// Check error message for validation errors
	if strings.Contains(err.Error(), "ValidationException") {
		return &dynago.Error{Sentinel: dynago.ErrValidation, Cause: err}
	}

	return err
}

