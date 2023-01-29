# See https://tech.davis-hansson.com/p/make/
SHELL := bash
.DELETE_ON_ERROR:
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := all
MAKEFLAGS += --warn-undefined-variables
MAKEFLAGS += --no-builtin-rules
MAKEFLAGS += --no-print-directory
BIN=$(abspath ~/go/bin)

# Set to use a different compiler. For example, `GO=go1.18rc1 make test`.
GO ?= go
.DEFAULT_GOAL := help
# DB_URL=postgresql://admin:admin@localhost:5432/flight?sslmode=disable
DB_URL=postgres://wegmjdaf:khexFaRIW0eslZ6GPRY5VFyCM7w_vMVc@tyke.db.elephantsql.com/wegmjdaf?sslmode=disable
API_VERSION := v1
PROTO := pkg/api/$(API_VERSION)/proto
PB := pkg/api/$(API_VERSION)/pb

BUF_VERSION:=0.55.0 88ii8i8i8
# Only list test and build dependencies
# Standard dependencies are installed via go get
DEPEND=\
	google.golang.org/protobuf/cmd/protoc-gen-go@latest \
	google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest \
	github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway \
	github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2 \
	google.golang.org/protobuf/cmd/protoc-gen-go \
	google.golang.org/grpc/cmd/protoc-gen-go-grpc


DB_USER=admin
DB_PSSWD=admin

clean: ## clean command 
	@rm -f $(PB)/*.pb.go
	# @rm -f doc/swagger/*.swagger.json
# gen: ## generates grpc and grpc gatewayes 
# 	@protoc --proto_path=$(PROTO)  $(PROTO)/*.proto --proto_path=third_party  --go_out=:pkg/api/v1 --go-grpc_out=:# pkg/api/v1 --grpc-gateway_out=$(PROTO) --openapiv2_out=swagger
#

server:
	@go run cmd/server/main.go -port 8080


	

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


help: ## show help message
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m\033[0m\n"} /^[$$()% a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)


testdb: ## runs testing involved or talks to the database
	@go test -v -cover -count=1 ./db/sqlc/...

seed-airport: ## seed airport sample records to database
	@psql flight --command='\i cmd/airport/airport.sql'


seed-aircraft:  ## seed aircraft sample records to database
	@psql flight --command='\i cmd/aircraft/aircraft.sql'


test/html: ## Go: tests with HTML coverage report
	bash ./script/test_coverage_browser.sh && rm script/coverage.txt

redis-graph: ## test redis-graph database one the cloud, This is just a test database for this project
	redis-cli -u redis://default:w1w7iRHouMVw01VqDK0ulA0zV7nTMmAs@redis-16650.c212.ap-south-1-1.ec2.cloud.redislabs.com:16650


proto: ## removes files generated before re-generates grpc, grpc gatewayes and swagger doc if needed using protoc compiler 
	@rm -f $(PB)/*.go
	@rm -f doc/swagger/*.swagger.json
	@protoc --proto_path=$(PROTO) --go_out=$(PB) --go_opt=paths=source_relative \
	--go-grpc_out=$(PB) --go-grpc_opt=paths=source_relative \
	--grpc-gateway_out=$(PB) --grpc-gateway_opt=paths=source_relative \
	$(PROTO)/*.proto
	

evans:
	evans --host localhost --port 9090 -r repl

buf: ## removes files generated before re-generates grpc, grpc gatewayes and swagger doc if needed using buf
	buf generate --output $(PB)
	
buf-lint: ## lint protobuf files using buf tool
	@buf lint --error-format=json --exclude-path=pkg/api/v1/proto/google,pkg/api/v1/proto/protoc-gen-openapiv2 | jq
.PHONY: clean gen server client test testdb install  createdb  migrateup migratedown migratedrop dropdb sqlc test mock dbdocs sql2dbml migrateup-doc migratedown-doc help seed-airport seed-aircraft test/html redis-graph proto evans buf

