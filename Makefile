.PHONY: help test test-coverage test-security build clean

# Help
help:
	@echo "Lucendex - Open Source Build"
	@echo ""
	@echo "Usage:"
	@echo "  make test              Run all tests"
	@echo "  make test-coverage     Run tests with coverage report"
	@echo "  make test-security     Run security-critical tests only"
	@echo "  make build             Build all binaries"
	@echo "  make clean             Clean build artifacts"
	@echo ""

# Testing
test:
	@echo "Running all tests..."
	@cd backend && go test ./... -v

test-coverage:
	@echo "Running tests with coverage..."
	@cd backend && go test ./... -coverprofile=coverage.out
	@cd backend && go tool cover -func=coverage.out
	@echo ""
	@echo "HTML coverage report:"
	@cd backend && go tool cover -html=coverage.out -o coverage.html
	@echo "  backend/coverage.html"

test-security:
	@echo "Running security tests..."
	@cd backend && go test ./internal/api/... -v
	@cd backend && go test ./internal/kv/security_test.go -v
	@cd backend && go test ./internal/router/validator_test.go -v

# Build
build:
	@echo "Building binaries..."
	@mkdir -p backend/bin
	@cd backend && go build -o bin/api ./cmd/api
	@cd backend && go build -o bin/indexer ./cmd/indexer
	@cd backend && go build -o bin/router ./cmd/router
	@echo "✓ Binaries in backend/bin/"

# Clean
clean:
	@echo "Cleaning..."
	@rm -rf backend/bin/
	@rm -f backend/coverage.out backend/coverage.html
	@echo "✓ Clean complete"
