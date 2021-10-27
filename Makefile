clean:
	rm -f ./pb/proto/* && rmdir --ignore-fail-on-non-empty pb/proto


gen:
	protoc --proto_path=.  output.proto  --go_out=: --go-grpc_out=:.
	
server:
	go run cmd/server/main.go -port 8080

client:
	go run cmd/client/main.go -address 0.0.0.0:8080


test:
	go test -cover -race ./...
createdb:
	createdb --username=admin --owner=admin bookings

dropdb:
	dropdb bookings

migrateup:
	migrate -path migration -database "postgresql://niini:admin@localhost:5432/bookings?sslmode=disable" -verbose up
	
migratedown:
	migrate -path migration -database "postgresql://niini:admin@localhost:5432/bookings?sslmode=disable" -verbose down
	
sqlc:
	sqlc generate

test:
	go test -v -cover ./...

mock:
	mockgen -package mockdb -destination db/mock/store.go github.com/techschool/simplebank/db/sqlc Store
	
web:
	npm run /data/data/com.termux/files/home/pro/pss/vue-element-admin dev

.PHONY: 

.PHONY: clean gen server client test createdb dropdb migrateup migratedown sqlc test mock web