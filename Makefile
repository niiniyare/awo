# ---------- Configuration ----------
APP_NAME      ?= awo
SERVE_CMD     ?= $(APP_NAME) serve
SERVER_PORT   ?= 8080
HOST          ?= 127.0.0.1
PID_FILE      ?= .server.pid
LOG_FILE      ?= .server.log
BUILD_DIR     ?= ./build
BIN           ?= $(BUILD_DIR)/$(APP_NAME)
GO            ?= go
PKG           ?= ./...
PORT_SCAN_GLOB ?= *$(APP_NAME)*serve*--port*$(SERVER_PORT)*

# ANSI colors (disable with NO_COLOR=1)
ifeq ($(NO_COLOR),1)
  BLUE   :=
  GREEN  :=
  YELLOW :=
  RED    :=
  NC     :=
else
  BLUE   := \033[34m
  GREEN  := \033[32m
  YELLOW := \033[33m
  RED    := \033[31m
  NC     := \033[0m
endif

.DEFAULT_GOAL := help

# ---------- Help ----------
.PHONY: help
help: ## Show this help
	@echo "Usage: make <target>"
	@echo ""
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

# ---------- Build / test ----------
.PHONY: build
build: ## Compile the binary into $(BIN)
	@echo "$(BLUE)Building $(BIN)...$(NC)"
	@mkdir -p $(BUILD_DIR)
	$(GO) build -o $(BIN) ./cmd/$(APP_NAME)
	@echo "$(GREEN)Build complete: $(BIN)$(NC)"

.PHONY: clean
clean: ## Remove build artifacts and logs
	@echo "$(BLUE)Cleaning...$(NC)"
	@rm -rf $(BUILD_DIR)
	@rm -f $(PID_FILE) $(LOG_FILE)
	@echo "$(GREEN)Clean complete$(NC)"

.PHONY: test
test: ## Run the test suite
	@echo "$(BLUE)Running tests...$(NC)"
	$(GO) test $(PKG)

.PHONY: vet
vet: ## Run go vet
	@echo "$(BLUE)Running go vet...$(NC)"
	$(GO) vet $(PKG)

.PHONY: fmt
fmt: ## Format source files
	@echo "$(BLUE)Formatting...$(NC)"
	$(GO) fmt $(PKG)

.PHONY: lint
lint: vet fmt ## Vet and format

# ---------- Server lifecycle ----------
.PHONY: start
start: build ## Start the server in the background (no-op if already running)
	@pid=""; \
	if [ -f $(PID_FILE) ]; then \
		pid="$$(cat $(PID_FILE) 2>/dev/null)"; \
		if [ -n "$$pid" ] && kill -0 "$$pid" 2>/dev/null; then \
			echo "$(YELLOW)Server already running (pid $$pid) on port $(SERVER_PORT)$(NC)"; \
			exit 0; \
		fi; \
		echo "$(YELLOW)Removing stale pid file (pid $${pid:-?} not running)$(NC)"; \
		rm -f $(PID_FILE); \
	fi; \
	echo "$(BLUE)Starting $(BIN) serve --port $(SERVER_PORT) --host $(HOST)...$(NC)"; \
	nohup $(BIN) serve --port $(SERVER_PORT) --host $(HOST) > $(LOG_FILE) 2>&1 & \
	pid=$$!; \
	echo $$pid > $(PID_FILE); \
	sleep 1; \
	if kill -0 "$$pid" 2>/dev/null; then \
		echo "$(GREEN)Server started (pid $$pid)$(NC)"; \
		echo "  URL:  http://$(HOST):$(SERVER_PORT)"; \
		echo "  Logs: $(LOG_FILE)  (tail with: make logs)"; \
	else \
		echo "$(RED)Server failed to start. Last log lines:$(NC)"; \
		tail -n 20 $(LOG_FILE) 2>/dev/null || true; \
		rm -f $(PID_FILE); \
		exit 1; \
	fi

# .PHONY: stop
# stop: ## Stop the background server (pid file first, then port scan fallback)
# 	@pid=""; \
# 	if [ -f $(PID_FILE) ]; then \
# 		pid="$$(cat $(PID_FILE) 2>/dev/null)"; \
# 		if [ -n "$$pid" ] && kill -0 "$$pid" 2>/dev/null; then \
# 			echo "$(BLUE)Stopping server (pid $$pid)...$(NC)"; \
# 		else \
# 			echo "$(YELLOW)Stale pid file (pid $${pid:-?} not running)$(NC)"; \
# 			pid=""; \
# 		fi; \
# 	fi; \
# 	if [ -z "$$pid" ]; then \
# 		echo "$(YELLOW)No live pid file - scanning processes by command line...$(NC)"; \
# 		for p in /proc/[0-9]*; do \
# 			c="$$(tr '\0' ' ' < "$$p/cmdline" 2>/dev/null)" || continue; \
# 			case "$$c" in \
# 				$(PORT_SCAN_GLOB)) \
# 					pid="$${p#/proc/}"; \
# 					echo "$(BLUE)Found server at pid $$pid$(NC)"; \
# 					break ;; \
# 			esac; \
# 		done; \
# 	fi; \
# 	if [ -n "$$pid" ]; then \
# 		kill "$$pid" 2>/dev/null || true; \
# 		for _ in 1 2 3 4 5; do \
# 			kill -0 "$$pid" 2>/dev/null || break; \
# 			sleep 0.2; \
# 		done; \
# 		if kill -0 "$$pid" 2>/dev/null; then \
# 			echo "$(YELLOW)pid $$pid ignored TERM, sending KILL$(NC)"; \
# 			kill -9 "$$pid" 2>/dev/null || true; \
# 		fi; \
# 		echo "$(GREEN)Server stopped (pid $$pid)$(NC)"; \
# 	else \
# 		echo "$(YELLOW)No running server found$(NC)"; \
# 	fi; \
# 	rm -f $(PID_FILE)

# .PHONY: restart
# restart: stop start ## Restart the server


.PHONY: stop
stop: ## Stop the background server (pid file first, then port scan fallback)
	@pid=""; \
	if [ -f $(PID_FILE) ]; then \
		pid="$$(cat $(PID_FILE) 2>/dev/null)"; \
		if [ -n "$$pid" ] && kill -0 "$$pid" 2>/dev/null; then \
			echo "$(BLUE)Stopping server (pid $$pid)...$(NC)"; \
		else \
			echo "$(YELLOW)Stale pid file (pid $${pid:-?} not running)$(NC)"; \
			pid=""; \
		fi; \
	fi; \
	if [ -z "$$pid" ]; then \
		echo "$(YELLOW)No live pid file - scanning processes by command line...$(NC)"; \
		self=$$$$; \
		ppid_of_self=$$PPID; \
		for p in /proc/[0-9]*; do \
			c="$$(tr '\0' ' ' < "$$p/cmdline" 2>/dev/null)" || continue; \
			procpid="$${p#/proc/}"; \
			if [ "$$procpid" = "$$self" ] || [ "$$procpid" = "$$ppid_of_self" ]; then \
				continue; \
			fi; \
			case "$$c" in \
				$(PORT_SCAN_GLOB)) \
					pid="$$procpid"; \
					echo "$(BLUE)Found server at pid $$pid$(NC)"; \
					break ;; \
			esac; \
		done; \
	fi; \
	if [ -n "$$pid" ]; then \
		kill "$$pid" 2>/dev/null || true; \
		for _ in 1 2 3 4 5; do \
			kill -0 "$$pid" 2>/dev/null || break; \
			sleep 0.2; \
		done; \
		if kill -0 "$$pid" 2>/dev/null; then \
			echo "$(YELLOW)pid $$pid ignored TERM, sending KILL$(NC)"; \
			kill -9 "$$pid" 2>/dev/null || true; \
		fi; \
		echo "$(GREEN)Server stopped (pid $$pid)$(NC)"; \
	else \
		echo "$(YELLOW)No running server found$(NC)"; \
	fi; \
	rm -f $(PID_FILE)
	@true

.PHONY: restart
restart: ## Restart the server (starts it fresh even if none was running)
	@$(MAKE) --no-print-directory stop || true
	@$(MAKE) --no-print-directory start

.PHONY: status
status: ## Show whether the server is running
	@pid=""; \
	if [ -f $(PID_FILE) ]; then \
		pid="$$(cat $(PID_FILE) 2>/dev/null)"; \
	fi; \
	if [ -n "$$pid" ] && kill -0 "$$pid" 2>/dev/null; then \
		echo "$(GREEN)Running (pid $$pid) on http://$(HOST):$(SERVER_PORT)$(NC)"; \
	else \
		echo "$(YELLOW)Not running$(NC)"; \
		exit 1; \
	fi

# ---------- Logs ----------
.PHONY: logs
logs: ## Follow server logs (Ctrl-C to exit)
	@if [ ! -f $(LOG_FILE) ]; then \
		echo "$(YELLOW)No log file at $(LOG_FILE) - start the server first$(NC)"; \
		exit 1; \
	fi; \
	echo "$(BLUE)Tailing $(LOG_FILE) (Ctrl-C to stop)$(NC)"; \
	tail -f $(LOG_FILE)

.PHONY: logs-tail
logs-tail: ## Show the last 50 log lines and exit
	@if [ ! -f $(LOG_FILE) ]; then \
		echo "$(YELLOW)No log file at $(LOG_FILE)$(NC)"; \
		exit 1; \
	fi; \
	tail -n 50 $(LOG_FILE)

.PHONY: logs-clear
logs-clear: ## Truncate the log file
	@: > $(LOG_FILE)
	@echo "$(GREEN)Log cleared: $(LOG_FILE)$(NC)"

# ---------- Dev workflow ----------
.PHONY: dev
dev: ## Run the server in the foreground for development
	@echo "$(BLUE)Running in foreground on http://$(HOST):$(SERVER_PORT)$(NC)"
	$(BIN) serve --port $(SERVER_PORT) --host $(HOST)

.PHONY: run
run: build dev ## Build then run in foreground

.PHONY: check-port
check-port: ## Check whether the configured port is in use
	@if command -v ss >/dev/null 2>&1; then \
		ss -ltn "sport = :$(SERVER_PORT)" | grep -q LISTEN && \
			echo "$(YELLOW)Port $(SERVER_PORT) is in use$(NC)" || \
			echo "$(GREEN)Port $(SERVER_PORT) is free$(NC)"; \
	elif command -v lsof >/dev/null 2>&1; then \
		lsof -iTCP:$(SERVER_PORT) -sTCP:LISTEN >/dev/null 2>&1 && \
			echo "$(YELLOW)Port $(SERVER_PORT) is in use$(NC)" || \
			echo "$(GREEN)Port $(SERVER_PORT) is free$(NC)"; \
	else \
		echo "$(YELLOW)Neither ss nor lsof found; cannot check port$(NC)"; \
	fi
