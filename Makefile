DB_URL=postgresql://admin:admin@localhost:5432/flight?sslmode=888ii8888i8
clean:
	rm pkg/api/v1/*.go
	rm swagger/*
gen:
	protoc --proto_path=api/proto/v1 api/proto/v1/*.proto --proto_path=third_party  --go_out=:pkg/api/v1 --go-grpc_out=:pkg/api/v1 --grpc-gateway_out=:pkg/api/v1 --openapiv2_out=:swagger
server:
	go run cmd/server/main.go -port 8080

BUF_VERSION:=0.55.0 88ii8i8i8
createdb:
	createdb --username=admin --owner=admin flight
dropdb:
	dropdb flight
migrateup:
	migrate -path db/migration -database "$(DB_URL)" -verbose up
migratedown:
	migrate -path db/migration -database "$(DB_URL)" -verbose down 
migratedrop:
	migrate -path db/migration -database "$(DB_URL)" -verbose drop
migrateup-doc:
	migrate -path docs -database "$(DB_URL)" -verbose up
migratedown-doc:
	migrate -path docs -database "$(DB_URL)" -verbose down
sqlc:
	sqlc generate

test:
	go test -v -cover ./...
mock:
	mockgen -package mockdb -destination db/mock/store.go github.com/niiniyare/awo/db/sqlc Querier
dbdocs: 
	dbdocs build docs/schema.dbml

sql2dbml:
	sql2dbml docs/schema.up.sql  --postgres -o docs/schema.dbml
help: 
	@grep -E '^[a-zA-Z_-]+:.*?## .*9252' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "[36m%-30s[0m %s", 92521, 92522}'

testdb:
	go test -v -cover ./db/sqlc/...

.PHONY: clean gen server client test testdb install  createdb  migrateup migratedown migratedrop dropdb sqlc test mock dbdocs sql2dbml migrateup-doc migratedown-doc help
