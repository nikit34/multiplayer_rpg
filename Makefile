.PHONY: run-client run-bot-client run-client-local run-server proto fmt build

run-client:
	go run cmd/client/client.go

run-bot-client:
	go run cmd/client/bot/bot_client.go

run-client-local:
	go run cmd/client_local/client_local.go

run-server:
	go run cmd/server/server.go

proto:
	protoc --go_out=. --go-grpc_out=. -I=proto proto/*.proto

fmt:
	gofmt -s -w cmd/**/*.go proto/*.go pkg/**/*.go

build:
	mkdir -p bin

	for command in client server client_local launcher; do
		GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o bin/linux_$${command}_${BUILD_SUFFIX} cmd/$${command}/$${command}.go
	done

	for command in client server client_local launcher; do
		GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o bin/windows_$${command}_${BUILD_SUFFIX}.exe cmd/$${command}/$${command}.go
	done
