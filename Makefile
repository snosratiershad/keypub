BUILD_NAME := ssh_server.nogit.
BUILD_DIR := .

.PHONY: all
all: build

.PHONY: build
build:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux go build -o $(BUILD_DIR)/$(BUILD_NAME) ./cmd/ssh_server

.PHONY: lint
lint:
	golangci-lint run

.PHONY: help
help:
	@echo "Usage:"
	@echo "  make build          Build the ssh-server"
	@echo "  make lint           Lints using golangci-lint"
	@echo "  make help           Show this help menu"
