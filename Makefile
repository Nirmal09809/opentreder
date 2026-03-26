.PHONY: all build run test clean docker-build docker-up docker-down lint format

all: build

build:
	go build -o bin/opentreder ./cmd/cli

run:
	go run ./cmd/cli

test:
	go test -v -race -cover ./...

test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

lint:
	golangci-lint run ./...

format:
	go fmt ./...
	goimports -w .

vet:
	go vet ./...

docker-build:
	docker build -t opentreder:latest .

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

docker-logs:
	docker-compose logs -f

clean:
	rm -rf bin/
	rm -f coverage.out coverage.html
	docker-compose down -v --remove-orphans

generate-proto:
	protoc --go_out=. --go-grpc_out=. proto/*.proto

migrate:
	go run ./cmd/migrate

bench:
	go test -bench=. -benchmem ./...

profile:
	go test -cpuprofile=cpu.prof -memprofile=mem.prof -bench=. ./...
	go tool pprof -http=:8080 cpu.prof

install-deps:
	go mod download
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

setup:
	cp configs/config.example.yaml configs/config.yaml
	make install-deps

help:
	@echo "OpenTrader Makefile Commands:"
	@echo "  make build          - Build the binary"
	@echo "  make run           - Run the application"
	@echo "  make test          - Run tests"
	@echo "  make test-coverage - Run tests with coverage"
	@echo "  make lint          - Run linters"
	@echo "  make format        - Format code"
	@echo "  make docker-build  - Build Docker image"
	@echo "  make docker-up     - Start Docker containers"
	@echo "  make docker-down   - Stop Docker containers"
	@echo "  make clean         - Clean build artifacts"
	@echo "  make bench         - Run benchmarks"
	@echo "  make profile       - Profile the application"
	@echo "  make setup         - Initial setup"
