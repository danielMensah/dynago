---
title: "DynaGo"
type: docs
---

# DynaGo

A type-safe DynamoDB library for Go, powered by generics.

[![CI](https://github.com/danielMensah/dynago/actions/workflows/ci.yml/badge.svg)](https://github.com/danielMensah/dynago/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/danielmensah/dynago.svg)](https://pkg.go.dev/github.com/danielmensah/dynago)
[![Go Report Card](https://goreportcard.com/badge/github.com/danielmensah/dynago)](https://goreportcard.com/report/github.com/danielmensah/dynago)

---

DynaGo leverages Go generics to provide a clean, expressive API for single-table and multi-table DynamoDB designs. It ships with an in-memory backend for fast, deterministic tests and optional OpenTelemetry middleware for production observability.

## Key Features

- **Type-safe generics API** -- `Get[T]`, `Put`, `Query[T]`, and `Scan[T]` work with your Go structs directly
- **Single-table design** -- A `Registry` maps discriminator values to concrete types for polymorphic queries
- **In-memory testing** -- The `memdb` package provides a complete in-memory DynamoDB backend
- **OpenTelemetry observability** -- The `dynagotel` middleware adds tracing spans and latency/count metrics
- **Middleware architecture** -- Wrap the backend with custom middleware for logging, retries, caching, or any cross-cutting concern

## Quick Install

```bash
go get github.com/danielmensah/dynago
```

## Get Started

{{< columns >}}

### [Getting Started]({{< relref "/docs/getting-started" >}})

Install DynaGo and write your first CRUD operations in minutes.

<--->

### [Guides]({{< relref "/docs/guides" >}})

In-depth guides for single-table design, migration, and more.

{{< /columns >}}

## Packages

| Package | Description |
|---------|-------------|
| [`dynago`](https://pkg.go.dev/github.com/danielmensah/dynago) | Core library: DB, Table, Get, Put, Query, Scan, Registry, Collection |
| [`memdb`](https://pkg.go.dev/github.com/danielmensah/dynago/memdb) | In-memory Backend for testing |
| [`dynagotest`](https://pkg.go.dev/github.com/danielmensah/dynago/dynagotest) | Test helpers: Seed, SeedFromJSON, AssertItemExists, AssertCount |
| [`dynagotel`](https://pkg.go.dev/github.com/danielmensah/dynago/dynagotel) | OpenTelemetry tracing and metrics middleware |
