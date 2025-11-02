.PHONY: all build client demo clean deps test

# Default target
all: build

# Install dependencies (only if needed)
deps:
	@if [ ! -d "pkg/client/node_modules" ]; then \
		echo "Installing TypeScript dependencies..."; \
		cd pkg/client && npm install; \
	else \
		echo "Dependencies already installed"; \
	fi

# Build TypeScript client library
client: deps
	@echo "Building TypeScript client library..."
	cd pkg/client && npm run build
	@echo "Copying client library to demo directory..."
	mkdir -p internal/commands/demo
	cp pkg/client/dist/*.js internal/commands/demo/
	cp pkg/client/dist/*.d.ts internal/commands/demo/
	@echo "Client library built and copied successfully"

# Build Go application (depends on client)
build: client
	@echo "Building Go application..."
	go build -o ipfs-webapp ./cmd/ipfs-webapp
	@echo "Build complete: ./ipfs-webapp"

# Run demo
demo: build
	./ipfs-webapp demo

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf pkg/client/dist
	rm -rf pkg/client/node_modules
	rm -f internal/commands/demo/ipfs-webapp-client.js
	rm -f internal/commands/demo/ipfs-webapp-client.d.ts
	rm -f ipfs-webapp
	@echo "Clean complete"

# Run tests
test:
	go test ./...
