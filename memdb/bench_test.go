package memdb

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/danielmensah/dynago"
)

func seedBenchItems(b *testing.B, m *MemoryBackend, tableName string, count int) {
	b.Helper()
	ctx := context.Background()
	for i := 0; i < count; i++ {
		_, err := m.PutItem(ctx, &dynago.PutItemRequest{
			TableName: tableName,
			Item: map[string]dynago.AttributeValue{
				"PK": {Type: dynago.TypeS, S: "partition#1"},
				"SK": {Type: dynago.TypeS, S: fmt.Sprintf("item#%05d", i)},
				"Status": {Type: dynago.TypeS, S: func() string {
					if i%2 == 0 {
						return "active"
					}
					return "inactive"
				}()},
				"Amount": {Type: dynago.TypeN, N: fmt.Sprintf("%d", i*10)},
				"Name":   {Type: dynago.TypeS, S: fmt.Sprintf("Item %d", i)},
			},
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func seedBenchScanItems(b *testing.B, m *MemoryBackend, tableName string, count int) {
	b.Helper()
	ctx := context.Background()
	for i := 0; i < count; i++ {
		pk := fmt.Sprintf("partition#%03d", i/100)
		_, err := m.PutItem(ctx, &dynago.PutItemRequest{
			TableName: tableName,
			Item: map[string]dynago.AttributeValue{
				"PK": {Type: dynago.TypeS, S: pk},
				"SK": {Type: dynago.TypeS, S: fmt.Sprintf("item#%05d", i)},
				"Status": {Type: dynago.TypeS, S: func() string {
					if i%3 == 0 {
						return "active"
					}
					return "inactive"
				}()},
				"Amount": {Type: dynago.TypeN, N: fmt.Sprintf("%d", i)},
			},
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func newBenchBackend() *MemoryBackend {
	m := New()
	m.CreateTable("bench", TableSchema{
		HashKey:  KeyDef{Name: "PK", Type: StringKey},
		RangeKey: &KeyDef{Name: "SK", Type: StringKey},
	})
	return m
}

func BenchmarkMemDB_Query_100Items(b *testing.B) {
	m := newBenchBackend()
	seedBenchItems(b, m, "bench", 100)
	ctx := context.Background()
	req := &dynago.QueryRequest{
		TableName:                 "bench",
		KeyConditionExpression:    "#pk = :pk0",
		ExpressionAttributeNames:  map[string]string{"#pk": "PK"},
		ExpressionAttributeValues: map[string]dynago.AttributeValue{":pk0": {Type: dynago.TypeS, S: "partition#1"}},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := m.Query(ctx, req)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMemDB_Query_1000Items(b *testing.B) {
	m := newBenchBackend()
	seedBenchItems(b, m, "bench", 1000)
	ctx := context.Background()
	req := &dynago.QueryRequest{
		TableName:                 "bench",
		KeyConditionExpression:    "#pk = :pk0",
		ExpressionAttributeNames:  map[string]string{"#pk": "PK"},
		ExpressionAttributeValues: map[string]dynago.AttributeValue{":pk0": {Type: dynago.TypeS, S: "partition#1"}},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := m.Query(ctx, req)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMemDB_ScanWithFilter_10000Items(b *testing.B) {
	m := newBenchBackend()
	seedBenchScanItems(b, m, "bench", 10000)
	ctx := context.Background()
	req := &dynago.ScanRequest{
		TableName:                 "bench",
		FilterExpression:          "#status = :status",
		ExpressionAttributeNames:  map[string]string{"#status": "Status"},
		ExpressionAttributeValues: map[string]dynago.AttributeValue{":status": {Type: dynago.TypeS, S: "active"}},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := m.Scan(ctx, req)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMemDB_ConcurrentReads(b *testing.B) {
	m := newBenchBackend()
	seedBenchItems(b, m, "bench", 100)
	ctx := context.Background()
	req := &dynago.GetItemRequest{
		TableName: "bench",
		Key: map[string]dynago.AttributeValue{
			"PK": {Type: dynago.TypeS, S: "partition#1"},
			"SK": {Type: dynago.TypeS, S: "item#00050"},
		},
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := m.GetItem(ctx, req)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkMemDB_ConcurrentMixedReadWrite(b *testing.B) {
	m := newBenchBackend()
	seedBenchItems(b, m, "bench", 100)
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()

	var counter int64
	var mu sync.Mutex

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			mu.Lock()
			counter++
			c := counter
			mu.Unlock()

			if c%5 == 0 {
				// Write (20% of operations).
				_, err := m.PutItem(ctx, &dynago.PutItemRequest{
					TableName: "bench",
					Item: map[string]dynago.AttributeValue{
						"PK":   {Type: dynago.TypeS, S: "partition#1"},
						"SK":   {Type: dynago.TypeS, S: fmt.Sprintf("item#%05d", c%100)},
						"Name": {Type: dynago.TypeS, S: fmt.Sprintf("Updated %d", c)},
					},
				})
				if err != nil {
					b.Fatal(err)
				}
			} else {
				// Read (80% of operations).
				_, err := m.GetItem(ctx, &dynago.GetItemRequest{
					TableName: "bench",
					Key: map[string]dynago.AttributeValue{
						"PK": {Type: dynago.TypeS, S: "partition#1"},
						"SK": {Type: dynago.TypeS, S: fmt.Sprintf("item#%05d", c%100)},
					},
				})
				if err != nil {
					b.Fatal(err)
				}
			}
		}
	})
}
