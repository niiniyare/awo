clean:
	rm pkg/api/v1/*.go
	rm swagger/*
	
gen:
	protoc --proto_path=api/proto/v1 api/proto/v1/*.proto --proto_path=third_party  --go_out=:pkg/api/v1 --go-grpc_out=:pkg/api/v1 --grpc-gateway_out=:pkg/api/v1 --openapiv2_out=:swagger
server:
	go run cmd/server/main.go -port 8080

generate:
	cd api/proto/v1; buf generate

lint:
	buf lint
	buf breaking --against 'https://github.com/johanbrandhorst/grpc-gateway-boilerplate.git#branch=master'

BUF_VERSION:=0.55.0

install:
	curl -sSL \
    	"https://github.com/bufbuild/buf/releases/download/v${BUF_VERSION}/buf-$(shell uname -s)-$(shell uname -m)" \
    	-o "$(shell go env GOPATH)/bin/buf" && \
  	chmod +x "$(shell go env GOPATH)/bin/buf"
  	

generatenetwork:
	docker network create bank-network

postgres:
	docker run --name postgres12 --network bank-network -p 5432:5432 -e POSTGRES_USER=root -e POSTGRES_PASSWORD=secret -d postgres:12-alpine

createdb:
	docker exec -it postgres12 createdb --username=root --owner=root simple_bank

dropdb:
	docker exec -it postgres12 dropdb simple_bank

migrateup:
	migrate -path db/migration -database "postgresql://root:secret@localhost:5432/simple_bank?sslmode=disable" -verbose up

migratedown:
	migrate -path db/migration -database "postgresql://root:secret@localhost:5432/simple_bank?sslmode=disable" -verbose down

sqlc:
	sqlc generate

test:
	go test -v -cover ./...
mock:
	mockgen -package mockdb -destination db/mock/store.go github.com/techschool/simplebank/db/sqlc Store
.PHONY: clean gen server client test cert install lint postgres createdb generatenetwork migrateup migratedown dropdb sqlc test mock