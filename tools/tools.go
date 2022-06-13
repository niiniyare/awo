//go:build tools
// +build tools

package tools

import (
	_ "github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway"
	_ "github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2"
	_ "github.com/rakyll/statik"
	_ "google.golang.org/grpc/cmd/protoc-gen-go-grpc"
	_ "google.golang.org/protobuf/cmd/protoc-gen-go"
	_ "github.com/kyleconroy/sqlc/cmd/sqlc"
  	_ "github.com/micro/micro/v3"
 // Use flowing command to install golang-migrate
 //go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
)
