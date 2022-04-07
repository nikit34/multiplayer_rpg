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

build:
	mkdir -p bin
	GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o bin/linux_client cmd/client/client.go
	GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o bin/linux_server cmd/server/server.go
	GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o bin/linux_client_local cmd/client_local.go
	GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o bin/windows_client.exe cmd/client/client.go
	GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o bin/windows_server.exe cmd/server/server.go
	GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o bin/windows_client_local.exe cmd/client_local.go
