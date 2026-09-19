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
MIGRATION_PATH := framework/db/migrations
SQLC_OUT       := db/sqlc
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

