# Makefile pour PVMSS
# Permet de construire, démarrer, arrêter, nettoyer et tester l'application

.PHONY: help dev dev-logs build docker-build up down restart logs test coverage test-unit test-integration test-routes test-offline test-offline-verbose test-offline-race test-offline-parallel go-lint go-fmt buildkit-start buildkit-stop buildkit-status frontend-install frontend-build frontend-dev frontend-test

# Couleurs pour l'affichage
BLUE=\033[0;34m
GREEN=\033[0;32m
RED=\033[0;31m
NC=\033[0m # No Color

# Detect git worktree root (works for both main worktree and linked worktrees)
GIT_ROOT := $(shell git rev-parse --show-toplevel)
# Get current directory relative to git root
CURRENT_DIR := $(shell git rev-parse --show-prefix)
# If we're in a worktree subdirectory, adjust paths accordingly
ifeq ($(CURRENT_DIR),)
    # We're in the main worktree
    BACKEND_DIR := backend
else
    # We're in a worktree, backend is still at ./backend relative to current dir
    BACKEND_DIR := backend
endif

# Test configuration
TEST_SETTINGS_PATH=/tmp/settings.test.json

# =============================================================================
# Commandes de base

help:
	@echo "$(BLUE)PVMSS - Commandes disponibles:$(NC)"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-15s$(NC) %s\n", $$1, $$2}'
	@echo ""

dev: build up ## Build et démarre l'application
	@echo "$(GREEN)Application démarrée sur http://localhost:50000$(NC)"
	@echo ""

dev-logs: up logs ## Démarre et affiche les logs
	@echo "$(GREEN)Logs de l'application$(NC)"
	@echo ""

build: frontend-build ## Construit le binaire Go et le frontend Svelte
	@echo "Building binary" && go clean -cache && go build -C $(BACKEND_DIR) -o ../pvmss
	@echo "Binary built successfully"
	@echo ""
	@echo "Remove old app logs and create new one"
	@rm -f app.log
	@touch app.log
	@echo "Successfully cleaned app logs"
	@echo "Building and running docker container"
	docker compose -f docker-compose.dev.yml down
	docker compose -f docker-compose.dev.yml build
	docker compose -f docker-compose.dev.yml up -d
	@echo ""

docker-build: buildkit-start ## Construit les images docker (arm64 et amd64) et push sur Docker Hub
	@echo "Building Docker images for multiple architectures..."
	@echo "Usage: make docker-build PVMSS_TAG=your-tag"
	@echo "Default tag: latest"
	@docker buildx build --builder pvmss-builder --platform linux/amd64,linux/arm64 -t jhmmt/pvmss:$(or $(PVMSS_TAG),latest) --push .
	@echo "Docker images built successfully"
	@echo ""

helm-package: ## Construit le package Helm
	@echo "Building Helm package..."
	@helm package ./helm
	@echo "Helm package built successfully"
	@echo ""
# need to push the package to a repository 

helm-upgrade: ## Met à jour l'application avec Helm
	@echo "Upgrading application with Helm..."
	@helm upgrade --install pvmss ./helm
	@echo ""

up:
	@docker compose -f docker-compose.dev.yml up -d
	@echo ""

down:
	@docker compose -f docker-compose.dev.yml down
	@echo ""

restart:
	@down up
	@echo ""

logs:
	@docker logs -f pvmss-dev
	@echo ""

# =============================================================================
# Commandes de test et qualification

coverage: ## Génère un rapport de couverture de code
	@echo "$(BLUE)Génération du rapport de couverture...$(NC)"
	@cp $(BACKEND_DIR)/settings.dev.json $(TEST_SETTINGS_PATH) 2>/dev/null || true
	cd $(BACKEND_DIR) && PVMSS_SETTINGS_PATH=$(TEST_SETTINGS_PATH) GO_TEST_ENVIRONMENT=1 go test -v -race -coverprofile=coverage.out ./...
	@echo "$(GREEN)✓ Rapport généré: $(BACKEND_DIR)/coverage.out$(NC)"

test-unit: ## Lance les tests unitaires Go (offline-compatible)
	@echo "$(BLUE)Lancement des tests unitaires Go...$(NC)"
	@cp $(BACKEND_DIR)/settings.dev.json $(TEST_SETTINGS_PATH) 2>/dev/null || true
	cd $(BACKEND_DIR) && PVMSS_SETTINGS_PATH=$(TEST_SETTINGS_PATH) GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test -v -race -coverprofile=coverage.out ./...
	@echo "$(GREEN)✓ Tests unitaires terminés$(NC)"

test-integration: ## Lance les tests d'intégration (offline-compatible)
	@echo "$(BLUE)Lancement des tests d'intégration...$(NC)"
	@cp $(BACKEND_DIR)/settings.dev.json $(TEST_SETTINGS_PATH) 2>/dev/null || true
	cd $(BACKEND_DIR) && PVMSS_SETTINGS_PATH=$(TEST_SETTINGS_PATH) GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test -v -race -timeout=5m ./tests
	@echo "$(GREEN)✓ Tests d'intégration terminés$(NC)"

test-offline: ## Lance tous les tests en mode offline (rapide, pour GitHub Actions)
	@echo "$(BLUE)Lancement de tous les tests en mode offline (optimisé)...$(NC)"
	@cp $(BACKEND_DIR)/settings.dev.json $(TEST_SETTINGS_PATH) 2>/dev/null || true
	cd $(BACKEND_DIR) && PVMSS_SETTINGS_PATH=$(TEST_SETTINGS_PATH) GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test -timeout=5m ./...
	@echo "$(GREEN)✓ Tests offline terminés$(NC)"

test-offline-verbose: ## Lance tous les tests offline avec sortie détaillée
	@echo "$(BLUE)Lancement de tous les tests en mode offline (verbose)...$(NC)"
	@cp $(BACKEND_DIR)/settings.dev.json $(TEST_SETTINGS_PATH) 2>/dev/null || true
	cd $(BACKEND_DIR) && PVMSS_SETTINGS_PATH=$(TEST_SETTINGS_PATH) GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test -v -timeout=5m ./...
	@echo "$(GREEN)✓ Tests offline verbose terminés$(NC)"

test-offline-race: ## Lance tous les tests offline avec race detector (lent mais complet)
	@echo "$(BLUE)Lancement de tous les tests en mode offline avec race detector...$(NC)"
	@cp $(BACKEND_DIR)/settings.dev.json $(TEST_SETTINGS_PATH) 2>/dev/null || true
	cd $(BACKEND_DIR) && PVMSS_SETTINGS_PATH=$(TEST_SETTINGS_PATH) GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test -race -timeout=10m ./...
	@echo "$(GREEN)✓ Tests offline avec race detector terminés$(NC)"

test-offline-parallel: ## Lance tous les tests offline en parallèle (maximum vitesse)
	@echo "$(BLUE)Lancement de tous les tests en mode offline (parallèle)...$(NC)"
	@cp $(BACKEND_DIR)/settings.dev.json $(TEST_SETTINGS_PATH) 2>/dev/null || true
	cd $(BACKEND_DIR) && PVMSS_SETTINGS_PATH=$(TEST_SETTINGS_PATH) GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test -p 4 -parallel 4 -timeout=5m ./...
	@echo "$(GREEN)✓ Tests offline parallèles terminés$(NC)"

test-online: ## Lance tous les tests en mode online (requiert Proxmox)
	@echo "$(BLUE)Lancement de tous les tests en mode online...$(NC)"
	@cp $(BACKEND_DIR)/settings.dev.json $(TEST_SETTINGS_PATH) 2>/dev/null || true
	cd $(BACKEND_DIR) && PVMSS_SETTINGS_PATH=$(TEST_SETTINGS_PATH) GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=false go test -v -race -coverprofile=coverage.out ./...
	@echo "$(GREEN)✓ Tests online terminés$(NC)"

test-all: test-offline ## Lance tous les tests (mode offline par défaut)
	@echo "$(BLUE)Lancement de tous les tests Go...$(NC)"
	@echo "$(GREEN)✓ Tests terminés$(NC)"

quick-test: ## Lance les tests rapides en mode offline
	@echo "$(BLUE)Lancement des tests rapides...$(NC)"
	@cp $(BACKEND_DIR)/settings.dev.json $(TEST_SETTINGS_PATH) 2>/dev/null || true
	cd $(BACKEND_DIR) && PVMSS_SETTINGS_PATH=$(TEST_SETTINGS_PATH) GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=true go test -v -short ./...
	@echo "$(GREEN)✓ Tests rapides terminés$(NC)"

test-svelte: ## Lance les tests Svelte
	@echo "$(BLUE)Lancement des tests Svelte...$(NC)"
	cd frontend && npx svelte-check --tsconfig ./tsconfig.json
	@echo "$(GREEN)✓ Tests Svelte terminés$(NC)"

# =============================================================================
# Commandes Go

go-lint: ## Lance le linter Go
	@echo "$(BLUE)Lancement du linter Go...$(NC)"
	cd $(BACKEND_DIR) && golangci-lint run -v --timeout=3m

go-fmt: ## Formate le code Go
	@echo "$(BLUE)Formatage du code Go...$(NC)"
	cd $(BACKEND_DIR) && go fmt ./...

go-update: ## Met à jour les dépendances Go
	@echo "$(BLUE)Mise à jour des dépendances Go...$(NC)"
	cd $(BACKEND_DIR) && go get -u ./... && go mod tidy

# =============================================================================
# Commandes BuildKit (pour builds multi-architecture)

buildkit-start: ## Démarre le conteneur buildkit pour les builds multi-architecture
	@echo "$(BLUE)Démarrage de buildkit...$(NC)"
	@if ! docker buildx ls | grep -q "pvmss-builder"; then \
		echo "Création de l'instance buildkit pvmss-builder avec driver docker-container..."; \
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
# Commandes Frontend (SvelteKit)

FRONTEND_DIR := frontend

frontend-install: ## Installe les dépendances npm du frontend SvelteKit
	@echo "$(BLUE)Installation des dépendances frontend...$(NC)"
	cd $(FRONTEND_DIR) && npm ci
	@echo "$(GREEN)✓ Dépendances installées$(NC)"

frontend-build: ## Construit le frontend SvelteKit (output: frontend/build/)
	@echo "$(BLUE)Construction du frontend SvelteKit...$(NC)"
	cd $(FRONTEND_DIR) && npm run build
	@echo "$(GREEN)✓ Frontend construit dans $(FRONTEND_DIR)/build/$(NC)"

frontend-dev: ## Démarre le serveur de développement SvelteKit (port 5173, proxy vers :50000)
	@echo "$(BLUE)Démarrage du serveur de développement frontend (port 5173)...$(NC)"
	@echo "$(BLUE)Proxy API → http://localhost:50000$(NC)"
	cd $(FRONTEND_DIR) && npm run dev

frontend-test: ## Lance les tests unitaires frontend (vitest)
	@echo "$(BLUE)Lancement des tests unitaires frontend...$(NC)"
	cd $(FRONTEND_DIR) && npm run test
	@echo "$(GREEN)✓ Tests frontend terminés$(NC)"

# =============================================================================
# Commandes de développement rapide

qualif: ## Lance tous les contrôles qualité (format, lint, tests offline)
	@echo "$(BLUE)[1/3] Formatage du code Go...$(NC)"
	@$(MAKE) go-fmt || { echo "$(RED)❌ Formatage échoué$(NC)"; exit 1; }
	@echo ""
	@echo "$(BLUE)[2/3] Linting du code Go...$(NC)"
	@$(MAKE) go-lint || { echo "$(RED)❌ Linting échoué$(NC)"; exit 1; }
	@echo ""
	@echo "$(BLUE)[3/3] Tests Go (offline mode)...$(NC)"
	@$(MAKE) test-offline || { echo "$(RED)❌ Tests échoués$(NC)"; exit 1; }
	@echo ""
	@echo "$(GREEN)✓ Contrôles et tests réussis!$(NC)"
	@echo ""
	@echo "$(BLUE)Démarrage de l'application...$(NC)"
	@$(MAKE) dev
	@echo ""

.DEFAULT_GOAL := help
