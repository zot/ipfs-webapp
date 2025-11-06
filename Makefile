.PHONY: all build client demo clean deps test release

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

# Build Go application for current platform (depends on client)
build: client
	@echo "Building Go application for current platform..."
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
	./p2p-webapp

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

# Build release binaries for all platforms
release: build
	@echo "Building release binaries for all platforms..."
	@mkdir -p build/release
	@echo "Preparing demo site..."
	@bash scripts/prepare-demo.sh
	@echo ""
	@echo "Building platform binaries..."
	@GOOS=linux GOARCH=amd64 go build -o build/release/p2p-webapp-linux-amd64-temp ./cmd/p2p-webapp
	@echo "  ✓ Linux amd64"
	@GOOS=windows GOARCH=amd64 go build -o build/release/p2p-webapp-windows-amd64-temp.exe ./cmd/p2p-webapp
	@echo "  ✓ Windows amd64"
	@GOOS=darwin GOARCH=amd64 go build -o build/release/p2p-webapp-darwin-amd64-temp ./cmd/p2p-webapp
	@echo "  ✓ macOS amd64 (Intel)"
	@GOOS=darwin GOARCH=arm64 go build -o build/release/p2p-webapp-darwin-arm64-temp ./cmd/p2p-webapp
	@echo "  ✓ macOS arm64 (Apple Silicon)"
	@echo ""
	@echo "Bundling demo into binaries..."
	@go run scripts/bundle-all-platforms.go build/release/p2p-webapp-linux-amd64-temp build/demo-staging build/release/p2p-webapp-linux-amd64
	@go run scripts/bundle-all-platforms.go build/release/p2p-webapp-windows-amd64-temp.exe build/demo-staging build/release/p2p-webapp-windows-amd64.exe
	@go run scripts/bundle-all-platforms.go build/release/p2p-webapp-darwin-amd64-temp build/demo-staging build/release/p2p-webapp-darwin-amd64
	@go run scripts/bundle-all-platforms.go build/release/p2p-webapp-darwin-arm64-temp build/demo-staging build/release/p2p-webapp-darwin-arm64
	@echo ""
	@echo "Cleaning up temporary files..."
	@rm -f build/release/*-temp build/release/*-temp.exe
	@rm -rf build/demo-staging
	@echo ""
	@echo "Release binaries ready in build/release/:"
	@ls -lh build/release/ | tail -n +2 | awk '{printf "  %s (%s)\n", $$9, $$5}'
