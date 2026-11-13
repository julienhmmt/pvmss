# Makefile for PVMSS
# Build, start, stop, clean and test the application

.DEFAULT_GOAL := help

# Output colors
BLUE  := \033[0;34m
GREEN := \033[0;32m
RED   := \033[0;31m
NC    := \033[0m

# Directories
SERVER_DIR   := server
WEB_DIR      := web

# Dev Docker Compose (shortcut to avoid repeating -f ...)
COMPOSE_DEV  := docker compose -f docker-compose.dev.yml --profile oidc

# Go test variables - overridable on the command line:
#   make test-offline GO_TEST_FLAGS=-v                 # verbose (replaces test-offline-verbose)
#   make test-offline GO_TEST_FLAGS=-short             # quick (replaces quick-test)
#   make test-offline 'GO_TEST_FLAGS=-p 4 -parallel 4' # parallel (replaces test-offline-parallel)
GO_TEST_ENV     := GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true
GO_TEST_TIMEOUT := -timeout=5m
GO_TEST_FLAGS   ?=
GO_UPDATE_FLAGS ?= -u=patch
GO_VERSION      ?= latest

# golangci-lint pinned outside go.mod: 'go run @version' builds it with the
# local toolchain (a prebuilt binary cannot read Go 1.27 export data).
# >= v2.15.0 required: x/tools v0.52.0 minimum to decode Go 1.27 export data.
# Keep in sync with .github/workflows/lint.yml.
GOLANGCI_LINT   := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0

# Default Docker tag for docker-build (override: make docker-build PVMSS_TAG=v1.2.3)
PVMSS_TAG ?= latest

.PHONY: help proxy-e2e dev dev-logs dev-web build docker-build helm-package helm-upgrade \
        up up-siem up-oidc down restart logs clean \
        qualif \
        buildkit-start buildkit-stop buildkit-status \
        server-lint server-fmt server-vet server-test server-go-update \
        server-deps-update server-update server-reset \
        web-install web-deps-update web-lint web-lint-fix web-check web-test \
        lint \
        sonar sonar-up sonar-down sonar-coverage sonar-lint sonar-scan sonar-scan-server sonar-scan-web sonar-query

# qualif must run its prerequisites sequentially (fmt -> lint -> test), never in parallel
.NOTPARALLEL: qualif

# =============================================================================
# Basic commands

help: ## Show this help
	@echo "$(BLUE)PVMSS - Commandes disponibles:$(NC)"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-18s$(NC) %s\n", $$1, $$2}'
	@echo ""

dev: build up ## Build and start the app (http://localhost:50000)
	@echo "$(GREEN)Application started on http://localhost:50000$(NC)"
	@echo ""

dev-logs: up logs ## Start and follow the logs
	@echo "$(GREEN)Logs de l'application$(NC)"
	@echo ""

dev-web: up ## Live web dev: docker backend (mTLS) + Vite dev server on http://localhost:5173
	@for i in $$(seq 1 30); do \
		[ -f dev/proxies/certs/proxy-client.crt ] && exit 0; \
		sleep 1; \
	done; \
	echo "$(RED)Dev certs not found under dev/proxies/certs$(NC)"; exit 1
	@[ -d $(WEB_DIR)/node_modules ] || $(MAKE) web-install
	@echo "$(GREEN)Web UI (hot reload) on http://localhost:5173 - API via https://localhost:50000$(NC)"
	cd $(WEB_DIR) && VITE_BACKEND_URL=https://localhost:50000 VITE_BACKEND_TLS_DIR=../dev/proxies/certs bun run dev

build: ## Build the Go binary and the dev Docker image
	@echo "$(BLUE)Construction du binaire Go...$(NC)"
	go build -C $(SERVER_DIR) -o ../pvmss ./cmd/pvmss
	@echo "$(GREEN)✓ Binaire construit$(NC)"
	@echo "$(BLUE)Construction de l'image Docker dev...$(NC)"
	$(COMPOSE_DEV) build
	@echo "$(GREEN)✓ Image construite$(NC)"

docker-build: buildkit-start ## Build and push the multi-arch images (amd64+arm64)
	@echo "$(BLUE)Build Docker multi-arch (tag: $(PVMSS_TAG))...$(NC)"
	docker buildx build --builder pvmss-builder --platform linux/amd64,linux/arm64 -t jhmmt/pvmss:$(PVMSS_TAG) --push .
	@echo "$(GREEN)✓ Images built and pushed (jhmmt/pvmss:$(PVMSS_TAG))$(NC)"

helm-package: ## Build the Helm package
	@echo "$(BLUE)Construction du package Helm...$(NC)"
	helm package ./helm
	@echo "$(GREEN)✓ Package Helm construit$(NC)"
# TODO: push the package to a Helm repository

helm-upgrade: ## Upgrade the app with Helm
	@echo "$(BLUE)Helm upgrade...$(NC)"
	helm upgrade --install pvmss ./helm
	@echo "$(GREEN)✓ Application upgraded$(NC)"

up: ## Start the dev environment (pvmss-dev behind the dev proxies) in the background
	$(COMPOSE_DEV) up -d

up-siem: ## Start dev + the VictoriaLogs SIEM receiver (TLS syslog 6514, UI http://127.0.0.1:9428/select/vmui)
	PVMSS_SYSLOG_ADDR=victorialogs:6514 PVMSS_SYSLOG_TLS_CA_FILE=/syslog-tls/ca.crt $(COMPOSE_DEV) --profile siem up -d

up-oidc: ## Start dev + the Zitadel test IdP on :8080 (set ZITADEL_DOMAIN to an IP Proxmox reaches)
	$(COMPOSE_DEV) --profile oidc up -d

down: ## Stop and remove the dev containers (including the siem and oidc profiles)
	$(COMPOSE_DEV) --profile siem --profile oidc down

restart: down up ## Restart the dev containers (down then up)

PROXIES := proxy-haproxy proxy-nginx proxy-caddy proxy-traefik proxy-npm

proxy-e2e: ## Test the VNC console through each dev reverse proxy (fake cluster, Playwright)
	$(COMPOSE_DEV) -f dev/proxies/docker-compose.fake.yml up -d --build --wait pvmss-dev $(PROXIES)
	dev/proxies/npm-setup.sh
	# Always put pvmss-dev back on the .env config (real cluster, real DB) afterwards.
	cd $(WEB_DIR) && bunx playwright test -c playwright.proxies.config.ts; \
		status=$$?; $(COMPOSE_DEV) up -d pvmss-dev; exit $$status

logs: ## Follow the dev container logs
	$(COMPOSE_DEV) logs -f

clean: ## Remove build artifacts and stop the dev containers
	@echo "$(BLUE)Cleaning...$(NC)"
	-$(COMPOSE_DEV) down --remove-orphans
	rm -f pvmss
	@echo "$(GREEN)✓ Cleanup done$(NC)"

server-reset: ## Reset the app: delete the DB (dev volume + local pvmss.db*), rebuild and restart
	@echo "$(BLUE)Stopping the dev containers...$(NC)"
	-$(COMPOSE_DEV) --profile siem down
	@echo "$(BLUE)Deleting the database...$(NC)"
	docker volume rm $$(docker volume ls -q -f name='pvmss_dev_data$$') 2>/dev/null || true
	rm -f pvmss.db pvmss.db-shm pvmss.db-wal
	@echo "$(GREEN)✓ Database reset$(NC)"
	@$(MAKE) dev

# =============================================================================
# Test and quality commands

qualif: server-fmt server-lint server-test ## Run all quality checks (fmt -> lint -> tests) then dev
	@echo "$(GREEN)✓ Checks and tests passed$(NC)"
	@echo ""
	@echo "$(BLUE)Starting the application...$(NC)"
	@$(MAKE) dev

# =============================================================================
# BuildKit commands (for multi-arch builds)

buildkit-start: ## Start the buildkit container for multi-arch builds
	@echo "$(BLUE)Starting buildkit...$(NC)"
	@if ! docker buildx ls | grep -q "pvmss-builder"; then \
		echo "Creating the pvmss-builder buildkit instance..."; \
		docker buildx create --name pvmss-builder --driver docker-container --use --bootstrap; \
	else \
		echo "The pvmss-builder buildkit instance already exists, checking its state..."; \
		docker buildx inspect --bootstrap; \
	fi
	@echo "$(GREEN)✓ Buildkit ready for multi-arch builds$(NC)"

buildkit-stop: ## Stop the buildkit container
	@echo "$(BLUE)Stopping buildkit...$(NC)"
	@if docker buildx ls | grep -q "pvmss-builder"; then \
		echo "Stopping the pvmss-builder buildkit instance..."; \
		docker buildx rm pvmss-builder; \
		echo "$(GREEN)✓ Buildkit stopped$(NC)"; \
	else \
		echo "Buildkit is not running"; \
	fi

buildkit-status: ## Check buildkit status
	@echo "$(BLUE)Statut de buildkit:$(NC)"
	@if docker buildx ls | grep -q "pvmss-builder"; then \
		echo "$(GREEN)✓ Buildkit is running$(NC)"; \
		docker buildx inspect pvmss-builder; \
	else \
		echo "$(RED)❌ Buildkit is not running$(NC)"; \
		echo "Run 'make buildkit-start' to start it"; \
	fi

# =============================================================================
# Server and web commands (server/ Go + web/ SvelteKit)
# Separate Go module `pvmss/server` and SvelteKit app `pvmss-web`.
# Independent tooling for the two build units.

# --- server/ (Go backend, module pvmss/server) ---

server-lint: ## Run golangci-lint on server/ (config server/.golangci.yml)
	@echo "$(BLUE)Running the Go linter on $(SERVER_DIR)/...$(NC)"
	cd $(SERVER_DIR) && $(GOLANGCI_LINT) run -v --timeout=5m ./...
	@echo "$(GREEN)✓ Server lint done$(NC)"

server-fmt: ## Format server/ Go code (golangci-lint fmt)
	@echo "$(BLUE)Formatage du code Go $(SERVER_DIR)/...$(NC)"
	cd $(SERVER_DIR) && $(GOLANGCI_LINT) fmt ./...
	@echo "$(GREEN)✓ Server fmt done$(NC)"

server-vet: ## Run go vet on server/ (light check, no golangci-lint)
	@echo "$(BLUE)go vet sur $(SERVER_DIR)/...$(NC)"
	cd $(SERVER_DIR) && go vet ./...
	@echo "$(GREEN)✓ Server vet done$(NC)"

server-test: ## Run the server/ Go tests
	@echo "$(BLUE)Tests Go $(SERVER_DIR)/...$(NC)"
	cd $(SERVER_DIR) && go test $(GO_TEST_FLAGS) -race -timeout=5m ./...
	@echo "$(GREEN)✓ Server tests done$(NC)"

server-go-update: ## Update the Go version used by the server module (GO_VERSION=latest)
	@echo "$(BLUE)Updating Go version in $(SERVER_DIR)/...$(NC)"
	cd $(SERVER_DIR) && go get go@$(GO_VERSION) && go mod tidy
	@echo "$(GREEN)✓ Go version updated$(NC)"

server-deps-update: ## Update Go module dependencies (patch releases by default)
	@echo "$(BLUE)Updating Go dependencies in $(SERVER_DIR)/...$(NC)"
	cd $(SERVER_DIR) && go get $(GO_UPDATE_FLAGS) ./... && go mod tidy
	@echo "$(GREEN)✓ Go dependencies updated$(NC)"

server-update: ## Update the Go version and module dependencies
	@echo "$(BLUE)Updating Go and dependencies in $(SERVER_DIR)/...$(NC)"
	cd $(SERVER_DIR) && go get go@$(GO_VERSION) && go get $(GO_UPDATE_FLAGS) ./... && go mod tidy
	@echo "$(GREEN)✓ Go and dependencies updated$(NC)"

# --- web/ (SvelteKit + TypeScript, app pvmss-web) ---

web-install: ## Install web/ bun dependencies (SvelteKit)
	@echo "$(BLUE)Installing $(WEB_DIR)/ dependencies...$(NC)"
	cd $(WEB_DIR) && bun install --frozen-lockfile
	@echo "$(GREEN)✓ Web dependencies installed$(NC)"

web-deps-update: ## Update all Bun dependencies in web/ to latest versions
	@echo "$(BLUE)Updating Bun dependencies in $(WEB_DIR)/...$(NC)"
	cd $(WEB_DIR) && bun update --latest
	@echo "$(GREEN)Web dependencies updated$(NC)"

web-lint: ## Run eslint on web/ (SvelteKit + TypeScript)
	@echo "$(BLUE)Running eslint on $(WEB_DIR)/...$(NC)"
	cd $(WEB_DIR) && bun run lint
	@echo "$(GREEN)✓ Web lint done$(NC)"

web-lint-fix: ## Auto-fix eslint problems in web/
	@echo "$(BLUE)Correction eslint $(WEB_DIR)/...$(NC)"
	cd $(WEB_DIR) && bun run lint:fix
	@echo "$(GREEN)✓ Web lint fixed$(NC)"

web-check: ## Type-check web/ (svelte-check)
	@echo "$(BLUE)Type-checking $(WEB_DIR)/ (svelte-check)...$(NC)"
	cd $(WEB_DIR) && bun run check
	@echo "$(GREEN)✓ Web check done$(NC)"

web-test: ## Run web/ unit tests (vitest)
	@echo "$(BLUE)Tests unitaires $(WEB_DIR)/ (vitest)...$(NC)"
	cd $(WEB_DIR) && bun run test
	@echo "$(GREEN)✓ Web tests done$(NC)"

# --- Combined lint (server + web) ---

lint: server-lint web-lint ## Lint server/ (Go) and web/ (SvelteKit)
	@echo "$(GREEN)✓ Lint (server + web) done$(NC)"

# =============================================================================
# SonarQube local analysis
# =============================================================================

SONAR_HOME ?= $(HOME)/sonarqube

sonar-up: ## Start the shared SonarQube server (http://localhost:9000)
	@$(MAKE) -C $(SONAR_HOME) up

sonar-down: ## Stop the shared SonarQube server
	@$(MAKE) -C $(SONAR_HOME) down

sonar-coverage: ## Generate Go coverage reports for SonarQube
	@echo "$(BLUE)Generating Go coverage reports...$(NC)"
	@chmod +x tools/sonar-coverage.sh
	@GO_TEST_FLAGS="$(GO_TEST_FLAGS)" SERVER_DIR="$(SERVER_DIR)" tools/sonar-coverage.sh
	@echo "$(GREEN)✓ Coverage reports ready in .sonar/$(NC)"

sonar-lint: ## Run ESLint on web/ (including .svelte) for SonarQube import
	@echo "$(BLUE)Running ESLint for SonarQube import...$(NC)"
	@chmod +x tools/sonar-frontend-lint.sh
	@tools/sonar-frontend-lint.sh
	@echo "$(GREEN)✓ ESLint reports ready in .sonar/$(NC)"

sonar-scan: sonar-coverage sonar-lint ## Scan both projects into the shared SonarQube (starts it if needed)
	@$(SONAR_HOME)/scan.sh

sonar-scan-server: sonar-coverage ## Scan only the server/ Go project
	@$(SONAR_HOME)/scan.sh server

sonar-scan-web: sonar-lint ## Scan only the web/ SvelteKit project (includes ESLint on .svelte)
	@$(SONAR_HOME)/scan.sh web

sonar: sonar-scan ## Full pipeline: coverage, lint, scan all projects

sonar-query: ## Query SonarQube results (usage: make sonar-query CMD="summary")
	@python3 $(SONAR_HOME)/query.py $(CMD)

