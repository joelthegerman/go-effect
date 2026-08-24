.PHONY: build run test lint check

build:
	go build ./...

# Runs the HTTP app on :8080 (override with PORT=9090 make run).
run:
	go run ./cmd/server

test:
	go test ./...

# Enforces core purity via depguard (belt-and-suspenders; `test` already
# guarantees it via TestCoreIsPure). Requires golangci-lint on PATH.
lint:
	golangci-lint run

check: test lint
