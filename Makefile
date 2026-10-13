.PHONY: test build docker-do

test:
	go test ./internal/easystock/... -count=1

build:
	go build -o easystock ./cmd/easystock

docker-do:
	docker build --platform linux/amd64 -f Dockerfile.digitalocean -t easystock:local .
