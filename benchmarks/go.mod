module github.com/danielmensah/dynago/benchmarks

go 1.26.1

require (
	github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue v1.15.22
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.57.1
	github.com/danielmensah/dynago v0.0.0
	github.com/danielmensah/dynago/dynagotel v0.0.0
	github.com/guregu/dynamo/v2 v2.2.1
)

require (
	github.com/aws/aws-sdk-go-v2 v1.41.5 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.21 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.21 // indirect
	github.com/aws/aws-sdk-go-v2/service/dynamodbstreams v1.24.10 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/endpoint-discovery v1.11.21 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.35.18 // indirect
	github.com/aws/smithy-go v1.24.2 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	go.opentelemetry.io/otel v1.42.0 // indirect
	go.opentelemetry.io/otel/metric v1.42.0 // indirect
	go.opentelemetry.io/otel/trace v1.42.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
)

replace (
	github.com/danielmensah/dynago => ../
	github.com/danielmensah/dynago/dynagotel => ../dynagotel
)
