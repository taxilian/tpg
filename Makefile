.PHONY: build test install clean help

# Build the binary
build:
	go build -o tasks ./cmd/tasks

# Run tests
test:
	go test ./... -v

# Run tests with coverage
cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Install to GOPATH/bin
install:
	go install ./cmd/tasks

# Clean build artifacts
clean:
	rm -f tasks coverage.out coverage.html

# Format code
fmt:
	go fmt ./...

# Run linter (requires golangci-lint)
lint:
	golangci-lint run

# Show help
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build    Build the binary"
	@echo "  test     Run tests"
	@echo "  cover    Run tests with coverage report"
	@echo "  install  Install to GOPATH/bin"
	@echo "  clean    Remove build artifacts"
	@echo "  fmt      Format code"
	@echo "  lint     Run linter"
	@echo "  help     Show this help"
