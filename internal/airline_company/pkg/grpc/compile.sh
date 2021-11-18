#! /bin/bash
export PATH="$PATH:$(go env GOPATH)/bin"
protoc --proto_path=. pb/*.proto  --go_out=:pb --go-grpc_out=:pb