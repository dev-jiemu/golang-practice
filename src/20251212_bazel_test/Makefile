.PHONY: help setup build run docker-build docker-run clean deps

DOCKER_IMAGE_NAME ?= docker_image_name
DOCKER_IMAGE_TAG ?= latest

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

setup: ## Setup development environment (install Bazel and ONNX Runtime for macOS)
	@echo "Setting up development environment..."
	@if ! command -v bazel &> /dev/null; then \
		echo "Installing Bazel via Homebrew..."; \
		brew install bazel; \
	else \
		echo "Bazel is already installed"; \
	fi
	@if [ ! -d "/usr/local/onnxruntime-osx-arm64-1.18.1" ]; then \
		echo "Downloading ONNX Runtime for macOS..."; \
		cd /usr/local && \
		sudo wget https://github.com/microsoft/onnxruntime/releases/download/v1.18.1/onnxruntime-osx-arm64-1.18.1.tgz && \
		sudo tar -xzf onnxruntime-osx-arm64-1.18.1.tgz && \
		sudo rm onnxruntime-osx-arm64-1.18.1.tgz; \
	else \
		echo "ONNX Runtime is already installed"; \
	fi
	@echo "Setup complete!"

deps: ## Update Go dependencies with Gazelle
	bazel run //:gazelle -- update-repos -from_file=go.mod -to_macro=deps.bzl%go_dependencies
	bazel run //:gazelle

build: ## Build with Bazel (for current platform)
	bazel build //:vad_filter_test

build-linux: ## Build for Linux platform (for Docker)
	bazel build --platforms=@io_bazel_rules_go//go/toolchain:linux_amd64 //:vad_filter_test

run: build ## Build and run locally
	bazel run //:vad_filter_test

docker-build: build-linux ## Build Docker image using Bazel build artifact
	docker build -t $(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG) .

docker-run: ## Run Docker container
	docker run --rm $(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG)

docker-push: docker-build ## Push Docker image to registry
	docker push $(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG)

bazel-docker: ## Build Docker image using Bazel rules_docker
	bazel build //:vad_filter_image

bazel-docker-push: ## Push Docker image using Bazel rules_docker
	bazel run //:push_image

clean: ## Clean build artifacts
	bazel clean
	rm -rf bazel-*

test: ## Run tests
	bazel test //...
