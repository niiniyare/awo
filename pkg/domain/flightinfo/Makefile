
GOPATH:=$(shell go env GOPATH)
.PHONY: init
init:
	go get -u github.com/golang/protobuf/proto
	go get -u github.com/golang/protobuf/protoc-gen-go
	go get github.com/micro/micro/v3/cmd/protoc-gen-micro
	go get github.com/micro/micro/v3/cmd/protoc-gen-openapi

.PHONY: api
api:
	protoc --openapi_out=. --proto_path=. proto/*.proto

.PHONY: proto
proto:
	protoc --proto_path=. --micro_out=proto --go_out=:proto proto/*.proto
	
.PHONY: build
build:
	go build -o flightinfo *.go

.PHONY: test
test:
	go test -v ./... -cover

.PHONY: docker
docker:
	docker build . -t flightinfo:latest
