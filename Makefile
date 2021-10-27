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


.PHONY: clean gen gen-all server client test