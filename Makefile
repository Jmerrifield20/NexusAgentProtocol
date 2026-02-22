.PHONY: dev test test-integration build migrate lint clean proto gen-ca show-ca

# Default target
all: build

# Start local dev stack (Postgres + registry + resolver)
dev:
	docker compose -f docker/docker-compose.yml up --build

dev-down:
	docker compose -f docker/docker-compose.yml down -v

# Build all binaries
build:
	go build -o bin/registry ./cmd/registry
	go build -o bin/resolver ./cmd/resolver
	go build -o bin/nap ./cmd/nap

# Run all tests
test:
	go test -race -count=1 ./...

# Run integration tests (requires Postgres via docker-compose)
test-integration:
	docker compose -f docker/docker-compose.yml up -d postgres
	@sleep 3
	$(MAKE) migrate
	DATABASE_URL="postgres://nexus:nexus@localhost:5432/nexus?sslmode=disable" \
		go test -tags integration -race -count=1 ./internal/registry/...
	docker compose -f docker/docker-compose.yml down

# Run tests with coverage
test-cover:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Lint
lint:
	golangci-lint run ./...

# Run DB migrations
migrate:
	go run ./cmd/migrate

migrate-down:
	@echo "migrate-down: use 'psql \$$DATABASE_URL' and revert manually, or roll back with golang-migrate CLI"

# Generate protobuf + grpc-gateway (requires protoc in PATH or uses buf if available)
proto:
	@if command -v buf >/dev/null 2>&1; then \
		buf generate; \
	else \
		GOOGLEAPIS=$$(go env GOPATH)/pkg/mod/github.com/grpc-ecosystem/grpc-gateway/v2@*/third_party/googleapis; \
		protoc \
			-I api/proto \
			-I /tmp/protoc-bin/include \
			--go_out=. --go_opt=paths=source_relative \
			--go-grpc_out=. --go-grpc_opt=paths=source_relative \
			--grpc-gateway_out=. --grpc-gateway_opt=paths=source_relative \
			--grpc-gateway_opt=generate_unbound_methods=true \
			api/proto/resolver.proto && \
		mkdir -p api/proto/resolver/v1 && \
		mv resolver.pb.go resolver_grpc.pb.go resolver.pb.gw.go api/proto/resolver/v1/ 2>/dev/null || true; \
	fi

# Generate (or regenerate) the local Nexus CA in ./certs/
# The registry does this automatically on startup, but you can also run it manually
# to pre-generate certs before starting (e.g. for use in scripts or docker-compose).
gen-ca:
	@mkdir -p certs
	@go run ./cmd/registry &
	@sleep 1
	@kill %1 2>/dev/null || true
	@echo "CA generated in certs/ — copy certs/ca.crt to trust it in your OS/browser."

# Print the CA certificate subject & validity
show-ca:
	@openssl x509 -in certs/ca.crt -noout -subject -dates 2>/dev/null || echo "No CA found in certs/ — run 'make gen-ca' first."

# Clean build artifacts
clean:
	rm -rf bin/ coverage.out coverage.html

# Install dev tools
tools:
	go install github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest
