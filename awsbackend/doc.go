// Package awsbackend provides a dynago.Backend implementation backed by the
// AWS SDK for Go v2. It translates DynaGo's library-owned request/response
// types to AWS DynamoDB API calls.
//
// Basic usage:
//
//	cfg, err := config.LoadDefaultConfig(ctx)
//	if err != nil { /* handle error */ }
//	backend := awsbackend.NewFromConfig(cfg)
//	db := dynago.New(backend)
package awsbackend
