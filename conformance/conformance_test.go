//go:build conformance

package conformance

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	awscfg "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/danielmensah/dynago"
	awsbackend "github.com/danielmensah/dynago/awsbackend"
	"github.com/danielmensah/dynago/memdb"
)

type backendSetup struct {
	name    string
	backend dynago.Backend
	table   string
	cleanup func()
}

// backends returns the list of backends to test against.
// memdb is always included; AWS is included when DYNAMODB_LOCAL_ENDPOINT is set.
func backends(t *testing.T) []backendSetup {
	t.Helper()
	setups := []backendSetup{memdbSetup(t)}

	if endpoint := os.Getenv("DYNAMODB_LOCAL_ENDPOINT"); endpoint != "" {
		setups = append(setups, awsSetup(t, endpoint))
	}

	return setups
}

func memdbSetup(t *testing.T) backendSetup {
	t.Helper()
	m := memdb.New()
	m.CreateTable("conformance", memdb.TableSchema{
		HashKey:  memdb.KeyDef{Name: "PK", Type: memdb.StringKey},
		RangeKey: &memdb.KeyDef{Name: "SK", Type: memdb.StringKey},
	})
	return backendSetup{
		name:    "memdb",
		backend: m,
		table:   "conformance",
		cleanup: func() {},
	}
}

func awsSetup(t *testing.T, endpoint string) backendSetup {
	t.Helper()

	cfg := awscfg.Config{
		Region: "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider(
			"fakeMyKeyId", "fakeSecretAccessKey", ""),
	}

	client := dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
		o.BaseEndpoint = &endpoint
	})

	tableName := fmt.Sprintf("conformance_%d", time.Now().UnixNano())

	_, err := client.CreateTable(context.Background(), &dynamodb.CreateTableInput{
		TableName: &tableName,
		KeySchema: []types.KeySchemaElement{
			{AttributeName: awscfg.String("PK"), KeyType: types.KeyTypeHash},
			{AttributeName: awscfg.String("SK"), KeyType: types.KeyTypeRange},
		},
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: awscfg.String("PK"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: awscfg.String("SK"), AttributeType: types.ScalarAttributeTypeS},
		},
		BillingMode: types.BillingModePayPerRequest,
	})
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Wait for table to become active.
	waiter := dynamodb.NewTableExistsWaiter(client)
	if err := waiter.Wait(context.Background(), &dynamodb.DescribeTableInput{
		TableName: &tableName,
	}, 30*time.Second); err != nil {
		t.Fatalf("table did not become active: %v", err)
	}

	backend := awsbackend.NewAWSBackend(client)

	return backendSetup{
		name:    "aws-local",
		backend: backend,
		table:   tableName,
		cleanup: func() {
			client.DeleteTable(context.Background(), &dynamodb.DeleteTableInput{
				TableName: &tableName,
			})
		},
	}
}

func strAV(s string) dynago.AttributeValue {
	return dynago.AttributeValue{Type: dynago.TypeS, S: s}
}

func numAV(n string) dynago.AttributeValue {
	return dynago.AttributeValue{Type: dynago.TypeN, N: n}
}

func boolAV(b bool) dynago.AttributeValue {
	return dynago.AttributeValue{Type: dynago.TypeBOOL, BOOL: b}
}

// --- Conformance Tests ---

func TestPutGetRoundTrip(t *testing.T) {
	for _, setup := range backends(t) {
		t.Run(setup.name, func(t *testing.T) {
			defer setup.cleanup()
			ctx := context.Background()

			item := map[string]dynago.AttributeValue{
				"PK":   strAV("user#1"),
				"SK":   strAV("profile"),
				"Name": strAV("Alice"),
				"Age":  numAV("30"),
			}

			_, err := setup.backend.PutItem(ctx, &dynago.PutItemRequest{
				TableName: setup.table,
				Item:      item,
			})
			if err != nil {
				t.Fatal(err)
			}

			resp, err := setup.backend.GetItem(ctx, &dynago.GetItemRequest{
				TableName: setup.table,
				Key: map[string]dynago.AttributeValue{
					"PK": strAV("user#1"),
					"SK": strAV("profile"),
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			if resp.Item["Name"].S != "Alice" {
				t.Fatalf("expected Name=Alice, got %q", resp.Item["Name"].S)
			}
			if resp.Item["Age"].N != "30" {
				t.Fatalf("expected Age=30, got %q", resp.Item["Age"].N)
			}
		})
	}
}

func TestConditionExpressionPut(t *testing.T) {
	for _, setup := range backends(t) {
		t.Run(setup.name, func(t *testing.T) {
			defer setup.cleanup()
			ctx := context.Background()

			item := map[string]dynago.AttributeValue{
				"PK":   strAV("cond#1"),
				"SK":   strAV("test"),
				"Name": strAV("First"),
			}

			// First put should succeed.
			_, err := setup.backend.PutItem(ctx, &dynago.PutItemRequest{
				TableName:           setup.table,
				Item:                item,
				ConditionExpression: "attribute_not_exists(#pk)",
				ExpressionAttributeNames: map[string]string{"#pk": "PK"},
			})
			if err != nil {
				t.Fatal(err)
			}

			// Second put with same condition should fail.
			_, err = setup.backend.PutItem(ctx, &dynago.PutItemRequest{
				TableName:           setup.table,
				Item:                item,
				ConditionExpression: "attribute_not_exists(#pk)",
				ExpressionAttributeNames: map[string]string{"#pk": "PK"},
			})
			if !errors.Is(err, dynago.ErrConditionFailed) {
				t.Fatalf("expected ErrConditionFailed, got %v", err)
			}
		})
	}
}

func TestUpdateItem(t *testing.T) {
	for _, setup := range backends(t) {
		t.Run(setup.name, func(t *testing.T) {
			defer setup.cleanup()
			ctx := context.Background()

			_, _ = setup.backend.PutItem(ctx, &dynago.PutItemRequest{
				TableName: setup.table,
				Item: map[string]dynago.AttributeValue{
					"PK":   strAV("update#1"),
					"SK":   strAV("test"),
					"Name": strAV("Original"),
					"Count": numAV("5"),
				},
			})

			resp, err := setup.backend.UpdateItem(ctx, &dynago.UpdateItemRequest{
				TableName:        setup.table,
				Key:              map[string]dynago.AttributeValue{"PK": strAV("update#1"), "SK": strAV("test")},
				UpdateExpression: "SET #name = :name ADD #count :inc",
				ExpressionAttributeNames:  map[string]string{"#name": "Name", "#count": "Count"},
				ExpressionAttributeValues: map[string]dynago.AttributeValue{":name": strAV("Updated"), ":inc": numAV("3")},
				ReturnValues:     "ALL_NEW",
			})
			if err != nil {
				t.Fatal(err)
			}
			if resp.Attributes["Name"].S != "Updated" {
				t.Fatalf("expected Name=Updated, got %q", resp.Attributes["Name"].S)
			}
			if resp.Attributes["Count"].N != "8" {
				t.Fatalf("expected Count=8, got %q", resp.Attributes["Count"].N)
			}
		})
	}
}

func TestQueryWithSortKeys(t *testing.T) {
	for _, setup := range backends(t) {
		t.Run(setup.name, func(t *testing.T) {
			defer setup.cleanup()
			ctx := context.Background()

			// Seed data.
			for i := 0; i < 5; i++ {
				_, err := setup.backend.PutItem(ctx, &dynago.PutItemRequest{
					TableName: setup.table,
					Item: map[string]dynago.AttributeValue{
						"PK":   strAV("query#1"),
						"SK":   strAV(fmt.Sprintf("item#%03d", i)),
						"Data": strAV(fmt.Sprintf("data-%d", i)),
					},
				})
				if err != nil {
					t.Fatal(err)
				}
			}

			// Query with begins_with.
			resp, err := setup.backend.Query(ctx, &dynago.QueryRequest{
				TableName:                 setup.table,
				KeyConditionExpression:    "#pk = :pk0 AND begins_with(#sk, :sk0)",
				ExpressionAttributeNames:  map[string]string{"#pk": "PK", "#sk": "SK"},
				ExpressionAttributeValues: map[string]dynago.AttributeValue{":pk0": strAV("query#1"), ":sk0": strAV("item#")},
			})
			if err != nil {
				t.Fatal(err)
			}
			if resp.Count != 5 {
				t.Fatalf("expected 5 items, got %d", resp.Count)
			}
			// Verify ascending order.
			if resp.Items[0]["SK"].S != "item#000" {
				t.Fatalf("expected first item SK=item#000, got %q", resp.Items[0]["SK"].S)
			}
		})
	}
}

func TestQueryWithFilter(t *testing.T) {
	for _, setup := range backends(t) {
		t.Run(setup.name, func(t *testing.T) {
			defer setup.cleanup()
			ctx := context.Background()

			for i := 0; i < 5; i++ {
				status := "active"
				if i%2 == 0 {
					status = "inactive"
				}
				_, err := setup.backend.PutItem(ctx, &dynago.PutItemRequest{
					TableName: setup.table,
					Item: map[string]dynago.AttributeValue{
						"PK":     strAV("filter#1"),
						"SK":     strAV(fmt.Sprintf("item#%03d", i)),
						"Status": strAV(status),
					},
				})
				if err != nil {
					t.Fatal(err)
				}
			}

			resp, err := setup.backend.Query(ctx, &dynago.QueryRequest{
				TableName:                 setup.table,
				KeyConditionExpression:    "#pk = :pk0",
				FilterExpression:          "#status = :status",
				ExpressionAttributeNames:  map[string]string{"#pk": "PK", "#status": "Status"},
				ExpressionAttributeValues: map[string]dynago.AttributeValue{":pk0": strAV("filter#1"), ":status": strAV("active")},
			})
			if err != nil {
				t.Fatal(err)
			}
			if resp.Count != 2 {
				t.Fatalf("expected 2 active items, got %d", resp.Count)
			}
		})
	}
}

func TestQueryPagination(t *testing.T) {
	for _, setup := range backends(t) {
		t.Run(setup.name, func(t *testing.T) {
			defer setup.cleanup()
			ctx := context.Background()

			for i := 0; i < 10; i++ {
				_, err := setup.backend.PutItem(ctx, &dynago.PutItemRequest{
					TableName: setup.table,
					Item: map[string]dynago.AttributeValue{
						"PK": strAV("page#1"),
						"SK": strAV(fmt.Sprintf("item#%03d", i)),
					},
				})
				if err != nil {
					t.Fatal(err)
				}
			}

			var allItems []map[string]dynago.AttributeValue
			var startKey map[string]dynago.AttributeValue

			for {
				resp, err := setup.backend.Query(ctx, &dynago.QueryRequest{
					TableName:                 setup.table,
					KeyConditionExpression:    "#pk = :pk0",
					ExpressionAttributeNames:  map[string]string{"#pk": "PK"},
					ExpressionAttributeValues: map[string]dynago.AttributeValue{":pk0": strAV("page#1")},
					Limit:                     3,
					ExclusiveStartKey:         startKey,
				})
				if err != nil {
					t.Fatal(err)
				}
				allItems = append(allItems, resp.Items...)
				if len(resp.LastEvaluatedKey) == 0 {
					break
				}
				startKey = resp.LastEvaluatedKey
			}

			if len(allItems) != 10 {
				t.Fatalf("expected 10 items across pages, got %d", len(allItems))
			}
		})
	}
}

func TestScan(t *testing.T) {
	for _, setup := range backends(t) {
		t.Run(setup.name, func(t *testing.T) {
			defer setup.cleanup()
			ctx := context.Background()

			for i := 0; i < 3; i++ {
				_, err := setup.backend.PutItem(ctx, &dynago.PutItemRequest{
					TableName: setup.table,
					Item: map[string]dynago.AttributeValue{
						"PK":   strAV(fmt.Sprintf("scan#%d", i)),
						"SK":   strAV("test"),
						"Data": strAV(fmt.Sprintf("data-%d", i)),
					},
				})
				if err != nil {
					t.Fatal(err)
				}
			}

			resp, err := setup.backend.Scan(ctx, &dynago.ScanRequest{
				TableName:                 setup.table,
				FilterExpression:          "begins_with(#pk, :prefix)",
				ExpressionAttributeNames:  map[string]string{"#pk": "PK"},
				ExpressionAttributeValues: map[string]dynago.AttributeValue{":prefix": strAV("scan#")},
			})
			if err != nil {
				t.Fatal(err)
			}
			if resp.Count < 3 {
				t.Fatalf("expected at least 3 scan items, got %d", resp.Count)
			}
		})
	}
}

func TestDeleteItem(t *testing.T) {
	for _, setup := range backends(t) {
		t.Run(setup.name, func(t *testing.T) {
			defer setup.cleanup()
			ctx := context.Background()

			key := map[string]dynago.AttributeValue{
				"PK": strAV("del#1"),
				"SK": strAV("test"),
			}

			_, _ = setup.backend.PutItem(ctx, &dynago.PutItemRequest{
				TableName: setup.table,
				Item: map[string]dynago.AttributeValue{
					"PK": strAV("del#1"), "SK": strAV("test"), "Name": strAV("Delete Me"),
				},
			})

			_, err := setup.backend.DeleteItem(ctx, &dynago.DeleteItemRequest{
				TableName: setup.table,
				Key:       key,
			})
			if err != nil {
				t.Fatal(err)
			}

			resp, err := setup.backend.GetItem(ctx, &dynago.GetItemRequest{
				TableName: setup.table,
				Key:       key,
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(resp.Item) > 0 {
				t.Fatal("expected item to be deleted")
			}
		})
	}
}

func TestTypeMarshalingRoundTrip(t *testing.T) {
	for _, setup := range backends(t) {
		t.Run(setup.name, func(t *testing.T) {
			defer setup.cleanup()
			ctx := context.Background()

			item := map[string]dynago.AttributeValue{
				"PK":     strAV("types#1"),
				"SK":     strAV("test"),
				"Str":    strAV("hello"),
				"Num":    numAV("42"),
				"Bool":   boolAV(true),
				"Null":   {Type: dynago.TypeNULL, NULL: true},
				"StrSet": {Type: dynago.TypeSS, SS: []string{"a", "b", "c"}},
				"NumSet": {Type: dynago.TypeNS, NS: []string{"1", "2", "3"}},
				"List":   {Type: dynago.TypeL, L: []dynago.AttributeValue{strAV("x"), numAV("99")}},
				"Map": {Type: dynago.TypeM, M: map[string]dynago.AttributeValue{
					"Nested": strAV("value"),
				}},
			}

			_, err := setup.backend.PutItem(ctx, &dynago.PutItemRequest{
				TableName: setup.table,
				Item:      item,
			})
			if err != nil {
				t.Fatal(err)
			}

			resp, err := setup.backend.GetItem(ctx, &dynago.GetItemRequest{
				TableName: setup.table,
				Key: map[string]dynago.AttributeValue{
					"PK": strAV("types#1"),
					"SK": strAV("test"),
				},
			})
			if err != nil {
				t.Fatal(err)
			}

			if resp.Item["Str"].S != "hello" {
				t.Fatal("string mismatch")
			}
			if resp.Item["Num"].N != "42" {
				t.Fatal("number mismatch")
			}
			if resp.Item["Bool"].BOOL != true {
				t.Fatal("bool mismatch")
			}
			if resp.Item["Null"].Type != dynago.TypeNULL {
				t.Fatal("null type mismatch")
			}
			if len(resp.Item["StrSet"].SS) != 3 {
				t.Fatalf("expected 3 SS values, got %d", len(resp.Item["StrSet"].SS))
			}
			if len(resp.Item["NumSet"].NS) != 3 {
				t.Fatalf("expected 3 NS values, got %d", len(resp.Item["NumSet"].NS))
			}
			if len(resp.Item["List"].L) != 2 {
				t.Fatalf("expected 2 list items, got %d", len(resp.Item["List"].L))
			}
			if resp.Item["Map"].M["Nested"].S != "value" {
				t.Fatal("nested map mismatch")
			}
		})
	}
}
