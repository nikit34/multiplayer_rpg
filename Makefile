.PHONY: run-client run-client-local run-server proto fmt

run-client:
	go run cmd/client/client.go

run-client-local:
	go run cmd/client_local.go

run-server:
	go run cmd/server/server.go

proto:
	protoc --go_out=. --go-grpc_out=. -I=proto proto/*.proto

fmt:
	gofmt -s -w cmd/**/*.go proto/*.go pkg/**/*.go