.PHONY: build run dev test clean deps

# Build the application
build: deps
	go build -o bin/rd-downloader ./cmd/app

# Run the application (requires PATH environment variable)
run: build
	./bin/rd-downloader --path=$(PATH) --api-key=$(API_KEY)

# Development mode with hot reload (requires PATH)
dev: deps
	go run ./cmd/app --path=$(PATH) --api-key=$(API_KEY)

# Run tests
test:
	go test ./...

# Clean build artifacts
clean:
	rm -rf bin/

# Install dependencies
deps:
	go mod tidy

# Build for multiple platforms
build-all: deps
	GOOS=linux GOARCH=amd64 go build -o bin/rd-downloader-linux-amd64 ./cmd/app
	GOOS=linux GOARCH=arm64 go build -o bin/rd-downloader-linux-arm64 ./cmd/app
	GOOS=darwin GOARCH=amd64 go build -o bin/rd-downloader-darwin-amd64 ./cmd/app
	GOOS=darwin GOARCH=arm64 go build -o bin/rd-downloader-darwin-arm64 ./cmd/app
	GOOS=windows GOARCH=amd64 go build -o bin/rd-downloader-windows-amd64.exe ./cmd/app

# Help
help:
	@echo "RD Downloader - Real-Debrid Movie Downloader"
	@echo ""
	@echo "Usage:"
	@echo "  make build              Build the application"
	@echo "  make run PATH=/movies API_KEY=xxx   Run with specified path and API key"
	@echo "  make dev PATH=/movies API_KEY=xxx   Development mode"
	@echo "  make test               Run tests"
	@echo "  make clean              Clean build artifacts"
	@echo "  make deps               Install dependencies"
	@echo "  make build-all          Build for all platforms"
