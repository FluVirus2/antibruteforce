.PHONY: restore-dev restore generate build run start clean test-up test-down test

PROTOC_GEN_GO_VERSION = v1.36.10
PROTOC_GEN_GO_GRPC_VERSION = v1.6.0
GOLANG_CI_LINT_VERSION = v2.7.1

BUILD_FOLDER = bin
BUILD_SERVER_OUTPUT_PATH = $(BUILD_FOLDER)/server.bin
BUILD_CLI_OUTPUT_PATH = $(BUILD_FOLDER)/cli.bin

restore-dev: restore
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANG_CI_LINT_VERSION) && \
	go install golang.org/x/tools/cmd/goimports@latest

restore:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION) && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GO_GRPC_VERSION)

generate:
	rm -rf api/gen && \
	mkdir -p api/gen && \
	protoc -I=api/proto --go-grpc_out=api/gen --go_out=api/gen api/proto/*/*.proto

build: generate
	go mod download -x && \
	go build -o $(BUILD_SERVER_OUTPUT_PATH) cmd/server/main.go && \
	go build -o $(BUILD_CLI_OUTPUT_PATH) cmd/cli/main.go

run:
	./$(BUILD_SERVER_OUTPUT_PATH)

start: build run

clean:
	rm -rf $(BUILD_FOLDER)

test:
	CGO_ENABLED=1 go test ./... -race -count 100

test-up:
	docker compose -f test/manual/docker-compose.yml up --build

test-down:
	docker compose -f test/manual/docker-compose.yml down -v --rmi local

format:
	goimports -w .
	golangci-lint fmt ./...

lint:
	golangci-lint run ./...
