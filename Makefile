BINARY_NAME=icopy
BUILD_DIR=build

.PHONY: all build clean test verify release

all: build

build:
	@echo "Building $(BINARY_NAME)..."
	go build -o $(BINARY_NAME) main.go

clean:
	@echo "Cleaning up..."
	rm -f $(BINARY_NAME)
	rm -rf $(BUILD_DIR)
	rm -rf test_src test_dst badger custom.log .file_status.txt

test:
	@echo "Running tests..."
	go test ./...

verify: build
	@echo "Running verification..."
	./verify.sh

release: clean
	@echo "Building for multiple platforms..."
	mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 main.go
	GOOS=darwin GOARCH=arm64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 main.go
	GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 main.go
	GOOS=windows GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe main.go
	@echo "Release builds created in $(BUILD_DIR)"
