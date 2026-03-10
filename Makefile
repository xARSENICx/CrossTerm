# CROSS-TERM Makefile
# Grouped into binaries: crossterm, relay, and bootstrap

BINARY_DIR=bin
BINARY_NAME=crossterm
RELAY_NAME=relay
BOOTSTRAP_NAME=bootstrap

# Go settings
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTIDY=$(GOCMD) mod tidy

# Default: Build for the current platform
all: build

build: clean tidy
	mkdir -p $(BINARY_DIR)
	@echo "Building for $(shell go env GOOS)/$(shell go env GOARCH)..."
	$(GOBUILD) -o $(BINARY_DIR)/$(BINARY_NAME) ./cmd/$(BINARY_NAME)
	$(GOBUILD) -o $(BINARY_DIR)/$(RELAY_NAME) ./cmd/$(RELAY_NAME)
	$(GOBUILD) -o $(BINARY_DIR)/$(BOOTSTRAP_NAME) ./cmd/$(BOOTSTRAP_NAME)
	@echo "Done. Binaries are in $(BINARY_DIR)/"

# --- Cross-Compilation ---

# Linux (amd64 and arm64)
linux:
	mkdir -p $(BINARY_DIR)/linux
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BINARY_DIR)/linux/$(BINARY_NAME)-amd64 ./cmd/$(BINARY_NAME)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GOBUILD) -o $(BINARY_DIR)/linux/$(BINARY_NAME)-arm64 ./cmd/$(BINARY_NAME)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BINARY_DIR)/linux/$(RELAY_NAME)-amd64 ./cmd/$(RELAY_NAME)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GOBUILD) -o $(BINARY_DIR)/linux/$(RELAY_NAME)-arm64 ./cmd/$(RELAY_NAME)

# Windows (amd64)
windows:
	mkdir -p $(BINARY_DIR)/windows
	GOOS=windows GOARCH=amd64 $(GOBUILD) -o $(BINARY_DIR)/windows/$(BINARY_NAME).exe ./cmd/$(BINARY_NAME)
	GOOS=windows GOARCH=amd64 $(GOBUILD) -o $(BINARY_DIR)/windows/$(RELAY_NAME).exe ./cmd/$(RELAY_NAME)

# macOS (Universal: x86 and Apple Silicon)
macos:
	mkdir -p $(BINARY_DIR)/macos
	# Build Intel version
	GOOS=darwin GOARCH=amd64 $(GOBUILD) -o $(BINARY_DIR)/macos/$(BINARY_NAME)-intel ./cmd/$(BINARY_NAME)
	# Build ARM (M1/M2/M3) version
	GOOS=darwin GOARCH=arm64 $(GOBUILD) -o $(BINARY_DIR)/macos/$(BINARY_NAME)-apple-silicon ./cmd/$(BINARY_NAME)

# Build for all supported platforms
release: linux windows macos
	@echo "All release binaries generated in $(BINARY_DIR)/"

# --- Utility ---

run: build
	./$(BINARY_DIR)/$(BINARY_NAME)

clean:
	rm -rf $(BINARY_DIR)

tidy:
	$(GOTIDY)

# Help
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build    - Compile for the current OS"
	@echo "  linux    - Cross-compile for Linux (amd64 & arm64)"
	@echo "  windows  - Cross-compile for Windows (exe)"
	@echo "  macos    - Cross-compile for macOS (Intel & Silicon)"
	@echo "  release  - Generate binaries for all platforms"
	@echo "  run      - Build and run locally"
	@echo "  clean    - Remove the bin/ directory"
	@echo "  tidy     - Run go mod tidy"

.PHONY: all build linux windows macos release clean tidy run help
