# Makefile pour PVMSS
# Permet de construire, démarrer, arrêter, nettoyer et tester l'application

.DEFAULT_GOAL := help

# Couleurs pour l'affichage
BLUE  := \033[0;34m
GREEN := \033[0;32m
RED   := \033[0;31m
NC    := \033[0m

# Répertoires (v0.4 — le legacy backend/ + frontend/ a été supprimé au T16)
SERVER_DIR   := server
WEB_DIR      := web

# Docker Compose dev (raccourci pour éviter de répéter -f ...)
COMPOSE_DEV  := docker compose -f docker-compose.dev.yml

# Variables Go test — surchargeables en ligne de commande:
#   make test-offline GO_TEST_FLAGS=-v                 # verbose (remplace test-offline-verbose)
#   make test-offline GO_TEST_FLAGS=-short             # quick (remplace quick-test)
#   make test-offline 'GO_TEST_FLAGS=-p 4 -parallel 4' # parallèle (remplace test-offline-parallel)
GO_TEST_ENV     := GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true
GO_TEST_TIMEOUT := -timeout=5m
GO_TEST_FLAGS   ?=

# Tag Docker par défaut pour docker-build (surcharge: make docker-build PVMSS_TAG=v1.2.3)
PVMSS_TAG ?= latest

.PHONY: help dev dev-logs build docker-build helm-package helm-upgrade \
        up down restart logs clean \
        qualif \
        buildkit-start buildkit-stop buildkit-status \
        server-lint server-fmt server-vet server-test \
        web-install web-lint web-lint-fix web-check web-test \
        lint \
        sonar sonar-up sonar-down sonar-logs sonar-status sonar-bootstrap sonar-coverage sonar-lint sonar-scan sonar-scan-server sonar-scan-web sonar-query sonar-clean

# qualif doit exécuter ses prérequis séquentiellement (fmt → lint → test), jamais en parallèle
.NOTPARALLEL: qualif

# =============================================================================
# Commandes de base

help: ## Affiche cette aide
	@echo "$(BLUE)PVMSS - Commandes disponibles:$(NC)"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-18s$(NC) %s\n", $$1, $$2}'
	@echo ""

dev: build up ## Build et démarre l'application (http://localhost:50000)
	@echo "$(GREEN)Application démarrée sur http://localhost:50000$(NC)"
	@echo ""

dev-logs: up logs ## Démarre et affiche les logs
	@echo "$(GREEN)Logs de l'application$(NC)"
	@echo ""

build: ## Construit le binaire Go v0.4 et l'image Docker dev
	@echo "$(BLUE)Construction du binaire Go (v0.4 server)...$(NC)"
	go build -C $(SERVER_DIR) -o pvmss ./cmd/pvmss
	@echo "$(GREEN)✓ Binaire construit$(NC)"
	@echo "$(BLUE)Construction de l'image Docker dev...$(NC)"
	$(COMPOSE_DEV) build
	@echo "$(GREEN)✓ Image construite$(NC)"

docker-build: buildkit-start ## Construit les images multi-arch (amd64+arm64) et les push
	@echo "$(BLUE)Build Docker multi-arch (tag: $(PVMSS_TAG))...$(NC)"
	docker buildx build --builder pvmss-builder --platform linux/amd64,linux/arm64 -t jhmmt/pvmss:$(PVMSS_TAG) --push .
	@echo "$(GREEN)✓ Images construites et poussées (jhmmt/pvmss:$(PVMSS_TAG))$(NC)"

helm-package: ## Construit le package Helm
	@echo "$(BLUE)Construction du package Helm...$(NC)"
	helm package ./helm
	@echo "$(GREEN)✓ Package Helm construit$(NC)"
# TODO: pousser le package vers un dépôt Helm

helm-upgrade: ## Met à jour l'application avec Helm
	@echo "$(BLUE)Mise à jour Helm...$(NC)"
	helm upgrade --install pvmss ./helm
	@echo "$(GREEN)✓ Application mise à jour$(NC)"

up: ## Démarre les conteneurs dev en arrière-plan
# touch app.log garantit un fichier (et non un dossier) pour le bind mount Docker
	@touch app.log
	$(COMPOSE_DEV) up -d

down: ## Arrête et supprime les conteneurs dev
	$(COMPOSE_DEV) down

restart: down up ## Redémarre les conteneurs dev (down puis up)

logs: ## Suit les logs des conteneurs dev
	$(COMPOSE_DEV) logs -f

clean: ## Nettoie les artéfacts de build et arrête les conteneurs dev
	@echo "$(BLUE)Nettoyage...$(NC)"
	-$(COMPOSE_DEV) down --remove-orphans
	rm -f pvmss app.log
	@echo "$(GREEN)✓ Nettoyage terminé$(NC)"

# =============================================================================
# Commandes de test et qualification (v0.4)

qualif: server-fmt server-lint server-test ## Lance tous les contrôles qualité v0.4 (fmt → lint → tests) puis dev
	@echo "$(GREEN)✓ Contrôles et tests réussis$(NC)"
	@echo ""
	@echo "$(BLUE)Démarrage de l'application...$(NC)"
	@$(MAKE) dev

# =============================================================================
# Commandes BuildKit (pour builds multi-architecture)

buildkit-start: ## Démarre le conteneur buildkit pour les builds multi-architecture
	@echo "$(BLUE)Démarrage de buildkit...$(NC)"
	@if ! docker buildx ls | grep -q "pvmss-builder"; then \
		echo "Création de l'instance buildkit pvmss-builder..."; \
		docker buildx create --name pvmss-builder --driver docker-container --use --bootstrap; \
	else \
		echo "L'instance buildkit pvmss-builder existe déjà, vérification de l'état..."; \
		docker buildx inspect --bootstrap; \
	fi
	@echo "$(GREEN)✓ Buildkit prêt pour les builds multi-architecture$(NC)"

buildkit-stop: ## Arrête le conteneur buildkit
	@echo "$(BLUE)Arrêt de buildkit...$(NC)"
	@if docker buildx ls | grep -q "pvmss-builder"; then \
		echo "Arrêt de l'instance buildkit pvmss-builder..."; \
		docker buildx rm pvmss-builder; \
		echo "$(GREEN)✓ Buildkit arrêté$(NC)"; \
	else \
		echo "Buildkit n'est pas démarré"; \
	fi

buildkit-status: ## Vérifie le statut de buildkit
	@echo "$(BLUE)Statut de buildkit:$(NC)"
	@if docker buildx ls | grep -q "pvmss-builder"; then \
		echo "$(GREEN)✓ Buildkit est actif$(NC)"; \
		docker buildx inspect pvmss-builder; \
	else \
		echo "$(RED)❌ Buildkit n'est pas démarré$(NC)"; \
		echo "Lancez 'make buildkit-start' pour l'activer"; \
	fi

# =============================================================================
# Commandes Next-gen (server/ Go + web/ SvelteKit)
# Module Go séparé `pvmss/server` et app SvelteKit `pvmss-web` (v0.4 rewrite).
# Non connectés au Makefile principal — outillage indépendant.

# --- server/ (Go backend, module pvmss/server) ---

server-lint: ## Lance golangci-lint sur le next-gen server/ (config server/.golangci.yml)
	@echo "$(BLUE)Lancement du linter Go sur $(SERVER_DIR)/...$(NC)"
	cd $(SERVER_DIR) && golangci-lint run -v --timeout=5m ./...
	@echo "$(GREEN)✓ Lint server terminé$(NC)"

server-fmt: ## Formate le code Go du next-gen server/ (golangci-lint fmt)
	@echo "$(BLUE)Formatage du code Go $(SERVER_DIR)/...$(NC)"
	cd $(SERVER_DIR) && golangci-lint fmt ./...
	@echo "$(GREEN)✓ Formatage server terminé$(NC)"

server-vet: ## Lance go vet sur le next-gen server/ (vérification légère sans golangci-lint)
	@echo "$(BLUE)go vet sur $(SERVER_DIR)/...$(NC)"
	cd $(SERVER_DIR) && go vet ./...
	@echo "$(GREEN)✓ go vet server terminé$(NC)"

server-test: ## Lance les tests Go du next-gen server/
	@echo "$(BLUE)Tests Go $(SERVER_DIR)/...$(NC)"
	cd $(SERVER_DIR) && go test $(GO_TEST_FLAGS) -race -timeout=5m ./...
	@echo "$(GREEN)✓ Tests server terminés$(NC)"

# --- web/ (SvelteKit + TypeScript, app pvmss-web) ---

web-install: ## Installe les dépendances bun du next-gen web/ (SvelteKit)
	@echo "$(BLUE)Installation des dépendances $(WEB_DIR)/...$(NC)"
	cd $(WEB_DIR) && bun install --frozen-lockfile
	@echo "$(GREEN)✓ Dépendances web installées$(NC)"

web-lint: ## Lance eslint sur le next-gen web/ (SvelteKit + TypeScript)
	@echo "$(BLUE)Lancement d'eslint sur $(WEB_DIR)/...$(NC)"
	cd $(WEB_DIR) && bun run lint
	@echo "$(GREEN)✓ Lint web terminé$(NC)"

web-lint-fix: ## Corige automatiquement les problèmes eslint du next-gen web/
	@echo "$(BLUE)Correction eslint $(WEB_DIR)/...$(NC)"
	cd $(WEB_DIR) && bun run lint:fix
	@echo "$(GREEN)✓ Lint web corrigé$(NC)"

web-check: ## Vérifie le typage du next-gen web/ (svelte-check)
	@echo "$(BLUE)Vérification du typage $(WEB_DIR)/ (svelte-check)...$(NC)"
	cd $(WEB_DIR) && bun run check
	@echo "$(GREEN)✓ Vérification web terminée$(NC)"

web-test: ## Lance les tests unitaires du next-gen web/ (vitest)
	@echo "$(BLUE)Tests unitaires $(WEB_DIR)/ (vitest)...$(NC)"
	cd $(WEB_DIR) && bun run test
	@echo "$(GREEN)✓ Tests web terminés$(NC)"

# --- Lint combiné (server + web) ---

lint: server-lint web-lint ## Lance le lint sur server/ (Go) et web/ (SvelteKit)
	@echo "$(GREEN)✓ Lint next-gen (server + web) terminé$(NC)"

# =============================================================================
# SonarQube local analysis
# =============================================================================

COMPOSE_SONAR := docker compose -f docker-compose.sonarqube.yml

sonar-up: ## Start the SonarQube server (http://localhost:9000)
	@echo "$(BLUE)Starting SonarQube...$(NC)"
	$(COMPOSE_SONAR) up -d sonarqube
	@echo "$(GREEN)✓ SonarQube started at http://localhost:9000$(NC)"

sonar-down: ## Stop the SonarQube server
	@echo "$(BLUE)Stopping SonarQube...$(NC)"
	$(COMPOSE_SONAR) down
	@echo "$(GREEN)✓ SonarQube stopped$(NC)"

sonar-logs: ## Follow SonarQube logs
	$(COMPOSE_SONAR) logs -f sonarqube

sonar-status: ## Check SonarQube server status
	@docker ps --filter "name=pvmss-sonarqube" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"

sonar-bootstrap: ## Provision or rotate the SonarQube analysis token
	@echo "$(BLUE)Bootstrapping SonarQube token...$(NC)"
	@chmod +x tools/sonar-bootstrap.sh
	@tools/sonar-bootstrap.sh
	@echo "$(GREEN)✓ SonarQube token ready in .sonar/token$(NC)"

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

sonar-scan: sonar-coverage sonar-lint ## Run SonarScanner for both projects (requires sonar-up + sonar-bootstrap)
	@echo "$(BLUE)Running SonarScanner for all projects...$(NC)"
	@chmod +x tools/sonar-scan.sh
	@tools/sonar-scan.sh
	@echo "$(GREEN)✓ All scans complete. See http://localhost:9000/projects$(NC)"

sonar-scan-server: ## Scan only the server/ Go project
	@chmod +x tools/sonar-scan.sh
	@tools/sonar-scan.sh server

sonar-scan-web: sonar-lint ## Scan only the web/ SvelteKit project (includes ESLint on .svelte)
	@chmod +x tools/sonar-scan.sh
	@tools/sonar-scan.sh web

sonar: sonar-up sonar-bootstrap sonar-scan ## Full pipeline: start, token, coverage, lint, scan all projects
	@echo "$(GREEN)✓ SonarQube analysis complete. See http://localhost:9000/projects$(NC)"

sonar-query: ## Query SonarQube results (usage: make sonar-query CMD="summary")
	@python3 tools/sonar-query.py $(CMD)

sonar-clean: sonar-down ## Stop SonarQube and remove its data and local token
	@echo "$(BLUE)Cleaning SonarQube data...$(NC)"
	$(COMPOSE_SONAR) down -v
	rm -rf .sonar
	@echo "$(GREEN)✓ SonarQube data cleaned$(NC)"

