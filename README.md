# DynaGo

A generics-first DynamoDB library for Go. DynaGo provides compile-time type
safety through Go generics, treats single-table design as a first-class pattern,
and includes an in-memory backend for fast, dependency-free testing.

**Status: Work in Progress**

## Requirements

- Go 1.23+

## Install

```sh
go get github.com/danielmensah/dynago
```

## Quick Start

Define a struct with `dynamo` tags and use `Put` and `Get[T]` to store and
retrieve items:

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/danielmensah/dynago"
)

type User struct {
	PK    string `dynamo:"PK"`
	SK    string `dynamo:"SK"`
	Name  string `dynamo:"Name"`
	Email string `dynamo:"Email"`
}

func main() {
	ctx := context.Background()

	// Create a DB with your backend and get a table reference.
	db := dynago.New(backend)
	table := db.Table("my-table")

	// Put an item.
	user := User{
		PK:    "USER#123",
		SK:    "PROFILE",
		Name:  "Alice",
		Email: "alice@example.com",
	}
	if err := table.Put(ctx, user); err != nil {
		log.Fatal(err)
	}

	// Get the item back with full type safety.
	result, err := dynago.Get[User](ctx, table, dynago.Key("PK", "USER#123", "SK", "PROFILE"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(result.Name) // Alice
}
```

## License

MIT
