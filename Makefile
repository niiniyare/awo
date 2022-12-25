.DEFAULT_GOAL := help
DB_URL=postgresql://admin:admin@localhost:5432/flight?sslmode=disable
# DB_URL=postgres://wegmjdaf:khexFaRIW0eslZ6GPRY5VFyCM7w_vMVc@tyke.db.elephantsql.com/wegmjdaf?sslmode=disable
DB_USER=admin
DB_PSSWD=admin
clean: ## clean command 
	@rm pkg/api/v1/*.go
	@rm swagger/*
gen: ## generates grpc and grpc gatewayes 
	@protoc --proto_path=api/proto/v1 api/proto/v1/*.proto --proto_path=third_party  --go_out=:pkg/api/v1 --go-grpc_out=:pkg/api/v1 --grpc-gateway_out=:pkg/api/v1 --openapiv2_out=:swagger
server:
	@go run cmd/server/main.go -port 8080

BUF_VERSION:=0.55.0 88ii8i8i8
createdb: ## create postgres database
	@createdb --username="$(DB_USER)" --owner="$(DB_PSSWD)" flight
dropdb:  ## drop postgres database
	@dropdb flight
migrateup:  ## migrates up   last one version of thev database schema 	
	@migrate -path db/migration -database "$(DB_URL)" -verbose up
migratedown:  ## brings down  last one version of thev database schema 
	@migrate -path db/migration -database "$(DB_URL)" -verbose down 
migratedrop: ##  drops database schema
	@migrate -path db/migration -database "$(DB_URL)" -verbose drop
migrateup-doc:
	@migrate -path docs -database "$(DB_URL)" -verbose up
migratedown-doc:
	@migrate -path docs -database "$(DB_URL)" -verbose down
sqlc: ## generates go files from sql Query files using config files  in ./sqlc.yaml
	@sqlc generate

test: ## all go unit test 
	@go test -v -cover ./...
mock: ## generates mock from interfaces 
	@mockgen -package mockdb -destination db/mock/store.go github.com/niiniyare/awo/db/sqlc Querier
dbdocs: 
	@dbdocs build docs/schema.dbml

sql2dbml:
	@sql2dbml docs/schema.up.sql  --postgres -o docs/schema.dbml
# help: 
# 	@grep -E '^[a-zA-Z_-]+:.*?## .*9252' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "[36m%-30s[0m %s", 92521, 92522}'
#

help: ## show help message
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m\033[0m\n"} /^[$$()% a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
testdb: ## runs testing involved or talks to the database
	@go test -v -cover -count=1 ./db/sqlc/...
seed-airport: ## seed airport sample records to database
	@psql flight --command='\i cmd/airport/airport.sql'

seed-aircraft:  ## seed aircraft sample records to database
	@psql flight --command='\i cmd/aircraft/aircraft.sql'

test/html: ## Go: tests with HTML coverage report
	bash ./script/test_coverage_browser.sh
.PHONY: clean gen server client test testdb install  createdb  migrateup migratedown migratedrop dropdb sqlc test mock dbdocs sql2dbml migrateup-doc migratedown-doc help seed-airport seed-aircraft test/html
