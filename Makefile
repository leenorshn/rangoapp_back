.PHONY: test build

test:
	go test ./internal/easystock/... -count=1

build:
	go build -o easystock ./cmd/easystock
