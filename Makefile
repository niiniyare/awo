DB_URL=postgresql://admin:admin@localhost:5432/flight?sslmode=disable
clean:
	rm pkg/api/v1/*.go
	rm swagger/*
	
gen:
	protoc --proto_path=api/proto/v1 api/proto/v1/*.proto --proto_path=third_party  --go_out=:pkg/api/v1 --go-grpc_out=:pkg/api/v1 --grpc-gateway_out=:pkg/api/v1 --openapiv2_out=:swagger
server:
	go run cmd/server/main.go -port 8080

BUF_VERSION:=0.55.0

install:
	curl -sSL \
    	"https://github.com/bufbuild/buf/releases/download/v${BUF_VERSION}/buf-$(shell uname -s)-$(shell uname -m)" \
    	-o "$(shell go env GOPATH)/bin/buf" && \
  	chmod +x "$(shell go env GOPATH)/bin/buf" && \
  	go install github.com/kyleconroy/sqlc/cmd/sqlc@latest && \
  	go install github.com/micro/micro/v3@latest  && \
  	go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
  	

createdb:
	createdb --username=admin --owner=admin flight
dropdb:
	dropdb flight
migrateup:
	migrate -path db/migration -database "$(DB_URL)" -verbose up
migrateup1:
	migrate -path docs -database "$(DB_URL)" -verbose up
migratedown1:
	migrate -path docs -database "$(DB_URL)" -verbose down
sqlc:
	sqlc generate

test:
	go test -v -cover ./...
mock:
	mockgen -package mockdb -destination db/mock/store.go github.com/techschool/simplebank/db/sqlc Store
dbdocs: 
	dbdocs build docs/schema.dbml

sql2dbml:
	sql2dbml schema.up.sql  --postgres -o schema.dbml
.PHONY: clean gen server client testinstall  createdb  migrateup migratedown dropdb sqlc test mock dbdocs sql2dbml migrateup1 migratedown1