#!/bin/sh
set -u
set -e
set -x

#go install github.com/kyleconroy/sqlc/cmd/sqlc@latest
#go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
#go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
#go get -u github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway
#go get -u github.com/grpc-ecosystem/grpc-gateway/v3/protoc-gen-openapi
#go get github.com/micro/micro/v3/cmd/protoc-gen-openapi

rm -rf internal proto go.mod go.sum main.#go registry.go

/data/data/com.termux/files/home/go/bin/sqlc generate
sqlc-grpc -m flight
