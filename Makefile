clean:
	rm -f pb/*.go
gen:
	protoc --proto_path=proto proto/*.proto  --go_out=:. --go-grpc_out=:.
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

.PHONY: clean gen test createdb dropdb migrateup migratedown sqlc