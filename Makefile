# =============================================================================
# AWO ERP — Makefile
# Ref: https://tech.davis-hansson.com/p/make/
# =============================================================================

SHELL        := bash
.ONESHELL:
.SHELLFLAGS  := -eu -o pipefail -c
.DELETE_ON_ERROR:
MAKEFLAGS    += --warn-undefined-variables --no-builtin-rules --no-print-directory
.DEFAULT_GOAL := help

# =============================================================================
#  Configuration
# =============================================================================

# Database
DB_NAME  ?= awo
DB_USER  ?= admin
DB_PSSWD ?= admin
DB_HOST  ?= localhost
DB_PORT  ?= 5432
TEST_DATABASE_URL="postgres://user:pass@localhost:5432/erp_test?sslmode=di
DB_URL   ?= postgresql://$(DB_USER):$(DB_PSSWD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=disable
# Paths
# MIGRATION_PATH := framework/db/migrations
DOCS_PATH      := docs
COVERAGE_FILE  := coverage.out
COVERAGE_HTML  := coverage.html

# Ports
DOC_PORT  ?= 8081
GRPC_PORT ?= 9090

# Required tools (checked by check-tools)
REQUIRED_TOOLS := sqlc mockgen migrate psql golangci-lint mkdocs sleek awoctl

# Test configuration
TEST_TIMEOUT             := 10m
TEST_TIMEOUT_FAST        := 5m
TEST_TIMEOUT_E2E         := 15m
TEST_TIMEOUT_INTEGRATION := 8m
TEST_PARALLEL_JOBS       := 4
DOCKER_COMPOSE_TEST      := docker-compose.test.yml

# Test directories
UNIT_TEST_DIRS        := ./internal/core/... ./internal/shared/... ./internal/adapters/...
INTEGRATION_TEST_DIRS := ./test/integration/...
E2E_TEST_DIRS         := ./test/e2e/...

# Server log / PID
LOG_FILE := server.log
PID_FILE := server.pid

# Terminal colours
ESC    := $(shell printf '\033')
RED    := $(ESC)[0;31m
GREEN  := $(ESC)[0;32m
YELLOW := $(ESC)[0;33m
BLUE   := $(ESC)[0;34m
PURPLE := $(ESC)[0;35m
CYAN   := $(ESC)[0;36m
NC     := $(ESC)[0m

# =============================================================================
# 🆘 Help & Utilities
# =============================================================================

.PHONY: help
help: ##  Show this help message
	@echo "$(CYAN)Available targets:$(NC)"
	@awk 'BEGIN {FS = ":.*##"; printf "\n"} \
		/^[a-zA-Z0-9_.-]+:.*?##/ { \
			gsub(/^[ \t]+/, "", $$2); \
			printf "  $(GREEN)%-26s$(NC) %s\n", $$1, $$2 \
		}' $(MAKEFILE_LIST)
	@echo ""

.PHONY: check-tools
check-tools: ##  Verify required tools are installed
	@echo "$(BLUE)Checking required tools...$(NC)"
	@$(foreach tool,$(REQUIRED_TOOLS), \
		command -v $(tool) >/dev/null 2>&1 || { \
			echo "$(RED)❌  Missing: $(tool)$(NC)"; exit 1; \
		};)
	@echo "$(GREEN)✅  All required tools are installed$(NC)"

.PHONY: status
status: ##  Show project configuration
	@echo "$(CYAN)Project Configuration:$(NC)"
	@echo "  Database      : $(DB_USER)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)"
	@echo "  Migrations    : $(MIGRATION_PATH)"
	@echo "  Docs port     : $(DOC_PORT)"
	@echo "  Test timeout  : $(TEST_TIMEOUT)"
	@echo "  Parallel jobs : $(TEST_PARALLEL_JOBS)"

# =============================================================================
# 🚀 Server (cmd/awo serve)
# =============================================================================

SERVER_PORT ?= 8080
BIN_DIR     := bin
AWO_BIN     := $(BIN_DIR)/awo

.PHONY: build
build: ##  Build the awo binary into bin/awo
	@echo "$(BLUE)Building $(AWO_BIN)...$(NC)"
	@mkdir -p $(BIN_DIR)
	@go build -o $(AWO_BIN) ./cmd/awo
	@echo "$(GREEN)✅  Built $(AWO_BIN)$(NC)"

.PHONY: start
start: build ##  Start the server in the background (logs: server.log, pid: server.pid)
	@if [ -f $(PID_FILE) ] && kill -0 "$$(cat $(PID_FILE))" 2>/dev/null; then \
		echo "$(YELLOW)⚠️  Server already running (pid $$(cat $(PID_FILE)))$(NC)"; \
		exit 0; \
	fi
	@echo "$(BLUE)Starting server on port $(SERVER_PORT)...$(NC)"
	@DATABASE_URL="$(DB_URL)" nohup $(AWO_BIN) serve --db "$(DB_URL)" --port $(SERVER_PORT) \
		> $(LOG_FILE) 2>&1 & echo $$! > $(PID_FILE)
	@sleep 1
	@if kill -0 "$$(cat $(PID_FILE))" 2>/dev/null; then \
		echo "$(GREEN)✅  Server started (pid $$(cat $(PID_FILE))), log: $(LOG_FILE)$(NC)"; \
	else \
		echo "$(RED)❌  Server failed to start — see $(LOG_FILE)$(NC)"; \
		rm -f $(PID_FILE); exit 1; \
	fi

.PHONY: stop
stop: ##  Stop the background server (by pid file, falling back to port scan)
	@if [ -f $(PID_FILE) ] && kill -0 "$$(cat $(PID_FILE))" 2>/dev/null; then \
		pid="$$(cat $(PID_FILE))"; \
		echo "$(BLUE)Stopping server (pid $$pid)...$(NC)"; \
		kill "$$pid" 2>/dev/null; \
		sleep 1; \
		kill -0 "$$pid" 2>/dev/null && kill -9 "$$pid" 2>/dev/null || true; \
		rm -f $(PID_FILE); \
		echo "$(GREEN)✅  Server stopped$(NC)"; \
	else \
		echo "$(YELLOW)⚠️  No pid file / process not running — scanning by cmdline...$(NC)"; \
		pid=$$(for p in /proc/[0-9]*; do \
			c=$$(tr '\0' ' ' < "$$p/cmdline" 2>/dev/null); \
			case "$$c" in *awo*serve*--port*$(SERVER_PORT)*) echo "$${p#/proc/}"; break;; esac; \
		done); \
		if [ -n "$$pid" ]; then \
			echo "$(BLUE)Found server at pid $$pid, stopping...$(NC)"; \
			kill "$$pid" 2>/dev/null; sleep 1; kill -0 "$$pid" 2>/dev/null && kill -9 "$$pid" 2>/dev/null || true; \
			echo "$(GREEN)✅  Server stopped$(NC)"; \
		else \
			echo "$(YELLOW)⚠️  No running server found$(NC)"; \
		fi; \
		rm -f $(PID_FILE); \
	fi

.PHONY: restart
restart: stop start ##  Stop and start the server again

.PHONY: logs
logs: ##  Tail the server log
	@tail -f $(LOG_FILE)

