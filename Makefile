.PHONY: test test-coverage test-short lint clean

# Run all tests with verbose output and race detector
test:
	go test -v -race ./...

# Generate test coverage report
test-coverage:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Run short tests (skip long-running tests)
test-short:
	go test -short ./...

# Run linter
lint:
	golangci-lint run

# Clean test artifacts
clean:
	rm -f coverage.out coverage.html
	go clean -testcache
