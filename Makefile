.PHONY: test test-unit test-integration test-e2e test-coverage test-all help

# Default target
help:
	@echo "Available targets:"
	@echo "  make test              - Run all tests"
	@echo "  make test-unit         - Run unit tests only"
	@echo "  make test-integration  - Run integration tests (requires TEST_MONGO_URI)"
	@echo "  make test-e2e          - Run end-to-end tests"
	@echo "  make test-coverage     - Run tests with coverage report"
	@echo "  make test-all          - Run all test suites"

# Run all tests
test:
	go test ./... -v

# Run unit tests only (no database required)
test-unit:
	go test ./utils/... ./validators/... ./middlewares/... ./services/... -v -short

# Run integration tests (requires TEST_MONGO_URI)
test-integration:
	@if [ -z "$$TEST_MONGO_URI" ]; then \
		echo "Error: TEST_MONGO_URI environment variable is required"; \
		exit 1; \
	fi
	go test ./database/... -v -run Test

# Run end-to-end tests
test-e2e:
	@if [ -z "$$TEST_MONGO_URI" ]; then \
		echo "Error: TEST_MONGO_URI environment variable is required"; \
		exit 1; \
	fi
	go test ./e2e/... -v

# Run tests with coverage
test-coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run all test suites
test-all: test-unit test-integration test-e2e

# Clean test artifacts
clean:
	rm -f coverage.out coverage.html
	go clean -testcache





