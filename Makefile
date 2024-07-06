#### Dynamically Generated Interactive Menu ####

# Error Handling
SHELL := /bin/bash
.SHELLFLAGS := -o pipefail -c

# Name of this Makefile
MAKEFILE_NAME := $(lastword $(MAKEFILE_LIST))

# Special targets that should not be listed
EXCLUDE_LIST := menu all .PHONY

# Function to extract targets from the Makefile
define extract_targets
	$(shell awk -F: '/^[a-zA-Z0-9_-]+:/ {print $$1}' $(MAKEFILE_NAME) | grep -v -E '^($(EXCLUDE_LIST))$$')
endef

TARGETS := $(call extract_targets)

.PHONY: $(TARGETS) menu all clean test

menu: ## Makefile Interactive Menu
	@# Check if fzf is installed
	@if command -v fzf >/dev/null 2>&1; then \
		echo "Using fzf for selection..."; \
		echo "$(TARGETS)" | tr ' ' '\n' | fzf > .selected_target; \
		target_choice=$$(cat .selected_target); \
	else \
		echo "fzf not found, using numbered menu:"; \
		echo "$(TARGETS)" | tr ' ' '\n' > .targets; \
		awk '{print NR " - " $$0}' .targets; \
		read -p "Enter choice: " choice; \
		target_choice=$$(awk 'NR == '$$choice' {print}' .targets); \
	fi; \
	if [ -n "$$target_choice" ]; then \
		$(MAKE) $$target_choice; \
	else \
		echo "Invalid choice"; \
	fi

# Default target
all: menu

help: ## This help function
	@egrep '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

clean: ## Clean
	rm -rf ./dist dockerapi*.zip dockerapi *.log

debug-server: ## Debug server
	dlv debug --headless --api-version=2 --listen=127.0.0.1:43000 .

debug-client: ## Debug client
	dlv connect 127.0.0.1:43000

# Targets (example targets listed below)
lint: ## Run lint
	gofmt -w -s .

test: ## Run test
	@if [ -n "$(shell find . -name '*_test.go')" ]; then \
		go test -v ./...; \
	else \
		echo "No tests found"; \
	fi

build: ## Run build
	@$(eval DOCKERAPI_VERSION := $(shell if [ -z "$(DOCKERAPI_VERSION)" ]; then echo "$(shell git describe --tags --abbrev=0)"; else echo "$(DOCKERAPI_VERSION)"; fi))
	@echo "Bumping version to: $(DOCKERAPI_VERSION)"
	@export DOCKERAPI_VERSION=$(DOCKERAPI_VERSION)
	@if [ "$(shell uname)" == "Darwin" ]; then \
		sed -i '' -e "s/Version = \".*\"/Version = \"$(DOCKERAPI_VERSION)\"/g" main.go ; \
	else \
		sed -i -e "s/Version = \".*\"/Version = \"$(DOCKERAPI_VERSION)\"/g" main.go ; \
	fi

	@go build -v -ldflags="-X 'main.Version=$(DOCKERAPI_VERSION)'"
	@echo "Build completed, run ./dockerapi"

ci: ## build
	$(eval DOCKERAPI_VERSION := $(shell if [ -z "$(DOCKERAPI_VERSION)" ]; then echo "$(shell git describe --tags --abbrev=0)"; else echo "$(DOCKERAPI_VERSION)"; fi))
	@if [ "$(shell uname)" == "Darwin" ]; then \
		sed -i '' -e "s/Version = \".*\"/Version = \"$(DOCKERAPI_VERSION)\"/g" main.go ; \
	else \
		sed -i -e "s/Version = \".*\"/Version = \"$(DOCKERAPI_VERSION)\"/g" main.go ; \
	fi
	@echo "Building with version: $(DOCKERAPI_VERSION)"

	@mkdir -p ./dist/linux_amd64 ./dist/linux_arm64
	GOOS=linux GOARCH=amd64 go build -v -ldflags="-X 'main.Version=$(DOCKERAPI_VERSION)'" -o ./dist/linux_amd64/
	GOOS=linux GOARCH=arm64 go build -v -ldflags="-X 'main.Version=$(DOCKERAPI_VERSION)'" -o ./dist/linux_arm64/

	@zip -r dockerapi-linux-amd64.zip ./dist/linux_amd64/dockerapi
	@zip -r dockerapi-linux-arm64.zip ./dist/linux_arm64/dockerapi

	@echo "Build completed"
	@echo "Linux (amd64): ./dist/linux_amd64/dockerapi"
	@echo "Linux (arm64): ./dist/linux_arm64/dockerapi"

install: ## Install latest
	go install github.com/sammcj/dockerapi@latest

run: ## Run
	@go run $(shell find *.go -not -name '*_test.go')
