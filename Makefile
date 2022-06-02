clean:
	rm pkg/api/v1/*.go
	rm swagger/*
	
gen:
	protoc --proto_path=api/proto/v1 api/proto/v1/*.proto --proto_path=third_party  --go_out=:pkg/api/v1 --go-grpc_out=:pkg/api/v1 --grpc-gateway_out=:pkg/api/v1 --openapiv2_out=:swagger
server:
	go run cmd/server/main.go -port 8080

BUF_VERSION:=0.55.0

buf-install:
	curl -sSL \
    	"https://github.com/bufbuild/buf/releases/download/v${BUF_VERSION}/buf-$(shell uname -s)-$(shell uname -m)" \
    	-o "$(shell go env GOPATH)/bin/buf" && \
  	chmod +x "$(shell go env GOPATH)/bin/buf"
  	

createdb:
	createdb --username=admin --owner=admin flight
dropdb:
	dropdb flight
migrateup:
	migrate -path db/migration -database "postgresql://admin:admin@localhost:5432/flight?sslmode=disable" -verbose up
migratedown:
	migrate -path db/migration -database "postgresql://admin:admin@localhost:5432/flight?sslmode=disable" -verbose down
sqlc:
	sqlc generate

test:
	go test -v -cover ./...
mock:
	mockgen -package mockdb -destination db/mock/store.go github.com/techschool/simplebank/db/sqlc Store
.PHONY: clean gen server client test cert buf-install lint postgres createdb generatenetwork migrateup migratedown dropdb sqlc test mock