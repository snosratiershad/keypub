.PHONY: lint generate build
generate:
	go generate ./...
lint: generate
	go run github.com/golangci/golangci-lint/cmd/golangci-lint run
build: generate
	go build -o ssh_server.nogit. ./cmd/ssh_server