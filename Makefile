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
	@echo "Cleaning temporary files from demo directory..."
	find internal/commands/demo -name '.*' -type f -delete
	find internal/commands/demo -name '#*' -type f -delete
	@echo "Client library built and copied successfully"

# Build Go application (depends on client)
build: client
	@echo "Building Go application..."
	go build -o p2p-webapp-temp ./cmd/p2p-webapp
	@echo "Preparing demo site for bundling..."
	@bash scripts/prepare-demo.sh
	@echo "Bundling demo site into binary..."
	@./p2p-webapp-temp bundle build/demo-staging -o p2p-webapp
	@rm -f p2p-webapp-temp
	@rm -rf build/demo-staging
	@echo "Build complete: ./p2p-webapp (with bundled demo)"

# Run demo
demo: build
	./p2p-webapp serve

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf pkg/client/dist
	rm -rf pkg/client/node_modules
	rm -f p2p-webapp p2p-webapp-temp
	rm -rf build
	@echo "Clean complete"

# Run tests
test:
	go test ./...
