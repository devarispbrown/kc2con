# Makefile for kc2con testing

.PHONY: test test-unit test-integration test-all clean build deps lint coverage help

# Variables
BINARY_NAME=kc2con
TEST_TIMEOUT=30m
COVERAGE_FILE=coverage.out
INTEGRATION_TAG=integration

# Default target
all: deps lint test build

# Install dependencies
deps:
	@echo "📦 Installing dependencies..."
	go mod tidy
	go mod download

# Build the binary
build:
	@echo "🔨 Building $(BINARY_NAME)..."
	go build -o $(BINARY_NAME) .

# Build binary with coverage support
build-cover:
	@echo "🔨 Building $(BINARY_NAME) with coverage..."
	go build -cover -o $(BINARY_NAME) .

# Run unit tests
test-unit:
	@echo "🧪 Running unit tests..."
	go test -v -race -timeout $(TEST_TIMEOUT) ./internal/...

# Run integration tests
test-integration: build-cover
	@echo "🔧 Running integration tests..."
	mkdir -p coverage
	go test -v -tags $(INTEGRATION_TAG) -timeout $(TEST_TIMEOUT) ./test/

# Run all tests
test-all: test-unit test-integration

# Run tests with coverage
coverage:
	@echo "📊 Running tests with coverage..."
	mkdir -p coverage
	go test -v -race -coverprofile=$(COVERAGE_FILE) -covermode=atomic ./internal/...
	@if [ -f $(COVERAGE_FILE) ]; then \
		go tool cover -html=$(COVERAGE_FILE) -o coverage.html; \
		echo "Coverage report generated: coverage.html"; \
	fi

# Run integration tests with coverage
coverage-integration: build-cover
	@echo "📊 Running integration tests with coverage..."
	mkdir -p coverage
	GOCOVERDIR=./coverage go test -v -tags $(INTEGRATION_TAG) -timeout $(TEST_TIMEOUT) ./test/
	@if [ -d ./coverage ]; then \
		go tool covdata textfmt -i=./coverage -o coverage-integration.out; \
		go tool cover -html=coverage-integration.out -o coverage-integration.html; \
		echo "Integration coverage report generated: coverage-integration.html"; \
	fi

# Run all tests with coverage
coverage-all: coverage coverage-integration

# Run linting
lint:
	@echo "🔍 Running linters..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "⚠️  golangci-lint not installed. Run: curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b \$$(go env GOPATH)/bin v1.54.2"; \
		go vet ./...; \
		go fmt ./...; \
	fi

# Clean up build artifacts
clean:
	@echo "🧹 Cleaning up..."
	rm -f $(BINARY_NAME)
	rm -f $(COVERAGE_FILE)
	rm -f coverage.html
	rm -f coverage-integration.out
	rm -f coverage-integration.html
	rm -rf coverage
	rm -rf test_data

# Run benchmarks
bench:
	@echo "⚡ Running benchmarks..."
	go test -bench=. -benchmem ./...

# Generate test data for manual testing
test-data:
	@echo "📝 Generating test data..."
	mkdir -p test_data
	@echo '{"name":"test-mysql","connector.class":"io.debezium.connector.mysql.MySqlConnector","tasks.max":1,"database.hostname":"localhost","database.port":3306,"database.user":"debezium","database.password":"dbz","database.server.name":"myserver"}' > test_data/mysql-connector.json
	@echo 'name=test-jdbc\nconnector.class=io.confluent.connect.jdbc.JdbcSourceConnector\ntasks.max=1\nconnection.url=jdbc:postgresql://localhost:5432/test' > test_data/jdbc-connector.properties
	@echo 'bootstrap.servers=localhost:9092\ngroup.id=connect-cluster\nkey.converter=org.apache.kafka.connect.json.JsonConverter' > test_data/worker.properties
	@echo "✅ Test data created in test_data/"

# Install development tools
dev-tools:
	@echo "🛠️ Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run quick smoke tests
smoke-test: build
	@echo "💨 Running smoke tests..."
	./$(BINARY_NAME) --version
	./$(BINARY_NAME) --help
	./$(BINARY_NAME) compatibility
	./$(BINARY_NAME) connectors list
	@echo "✅ Smoke tests passed!"

# Watch mode for development (requires watchexec)
watch:
	@echo "👀 Watching for changes..."
	@if command -v watchexec >/dev/null 2>&1; then \
		watchexec -c -r -e go "make test-unit"; \
	else \
		echo "⚠️  watchexec not installed. Install with: cargo install watchexec-cli"; \
		echo "Running tests once..."; \
		make test-unit; \
	fi

# Docker testing environment
docker-test:
	@echo "🐳 Running tests in Docker..."
	docker run --rm -v $(PWD):/app -w /app golang:1.21 make test-all

# Performance testing
perf-test: build-cover
	@echo "🚀 Running performance tests..."
	go test -v -tags $(INTEGRATION_TAG) -timeout $(TEST_TIMEOUT) -run TestPerformance ./test/

# Check for security vulnerabilities
security:
	@echo "🔒 Checking for security vulnerabilities..."
	@if command -v govulncheck >/dev/null 2>&1; then \
		govulncheck ./...; \
	else \
		echo "⚠️  govulncheck not installed. Run: go install golang.org/x/vuln/cmd/govulncheck@latest"; \
	fi

# Release preparation
pre-release: clean deps lint test-all security coverage-all
	@echo "🚀 Pre-release checks completed successfully!"

# Help target
help:
	@echo "Available targets:"
	@echo "  build          - Build the binary"
	@echo "  build-cover    - Build binary with coverage support"
	@echo "  test-unit      - Run unit tests"
	@echo "  test-integration - Run integration tests"
	@echo "  test-all       - Run all tests"
	@echo "  coverage       - Run unit tests with coverage"
	@echo "  coverage-integration - Run integration tests with coverage"
	@echo "  coverage-all   - Run all tests with coverage"
	@echo "  lint           - Run linters"
	@echo "  clean          - Clean build artifacts"
	@echo "  bench          - Run benchmarks"
	@echo "  test-data      - Generate test data"
	@echo "  dev-tools      - Install development tools"
	@echo "  smoke-test     - Run quick smoke tests"
	@echo "  watch          - Watch for changes and run tests"
	@echo "  docker-test    - Run tests in Docker"
	@echo "  perf-test      - Run performance tests"
	@echo "  security       - Check for security vulnerabilities"
	@echo "  pre-release    - Run all pre-release checks"
	@echo "  help           - Show this help message"