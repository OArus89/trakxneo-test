.PHONY: test test-v report lint deps

TARGET ?= trakxneo

# Install dependencies
deps:
	go mod tidy

# Run all E2E tests
test:
	TARGET=$(TARGET) go test ./scenarios/ -v -timeout=5m -count=1

# Verbose with race detector
test-v:
	TARGET=$(TARGET) go test ./scenarios/ -v -race -timeout=5m -count=1

# Run a single scenario
# Usage: make test-one SCENARIO=TestTelemetryPipeline
test-one:
	TARGET=$(TARGET) go test ./scenarios/ -v -run $(SCENARIO) -timeout=5m -count=1

# Run tests and generate HTML report
report:
	@mkdir -p report
	TARGET=$(TARGET) go test ./scenarios/ -v -timeout=5m -count=1 -json 2>&1 | \
		go run ./report/generate.go
	@echo "Open report/index.html to view results"

# Lint
lint:
	golangci-lint run ./...
