.PHONY: build run test clean deps fmt vet

BINARY_NAME=mocknroll
BUILD_DIR=bin

build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(BINARY_NAME) .

run:
	@go run .

test:
	@go test -v ./...

clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)

deps:
	@go mod tidy
	@go mod vendor

fmt:
	@go fmt ./...

vet:
	@go vet ./...
