# youtube-music-mcp Makefile
#
# Usage:
#   make build         - compile the server binary to bin/server
#   make run           - run in stdio mode (for MCP client testing)
#   make run-sse       - run in SSE/HTTP mode (for Railway or browser testing)
#   make test          - run all tests
#   make vet           - run go vet
#   make lint          - run go vet + build (basic lint)
#   make docker-build  - build Docker image
#   make docker-run    - run Docker container in SSE mode (requires .env file)
#   make clean         - remove build artifacts

.PHONY: build run run-sse test vet lint docker-build docker-run clean

# Default target
build:
	go build -o bin/server ./cmd/server

run:
	go run ./cmd/server

run-sse:
	TRANSPORT=sse go run ./cmd/server

test:
	go test ./...

vet:
	go vet ./...

lint:
	go vet ./... && go build ./...

docker-build:
	docker build -t youtube-music-mcp .

docker-run:
	docker run --rm --env-file .env -p 8080:8080 -e TRANSPORT=sse youtube-music-mcp

clean:
	rm -rf bin/
