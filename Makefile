# Makefile pour PVMSS
# Permet de construire, démarrer, arrêter, nettoyer et tester l'application

.DEFAULT_GOAL := help

# Couleurs pour l'affichage
BLUE  := \033[0;34m
GREEN := \033[0;32m
RED   := \033[0;31m
NC    := \033[0m

# Répertoires
BACKEND_DIR  := backend
FRONTEND_DIR := frontend

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
        coverage test-unit test-integration test-offline test-offline-race test-online qualif \
        go-lint go-fmt go-update \
        buildkit-start buildkit-stop buildkit-status \
        frontend-install frontend-build frontend-dev frontend-test frontend-check

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

build: frontend-build ## Construit le binaire Go et l'image Docker dev
	@echo "$(BLUE)Construction du binaire Go...$(NC)"
	go build -C $(BACKEND_DIR) -o ../pvmss
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
	rm -f pvmss app.log backend/coverage.out
	rm -rf $(FRONTEND_DIR)/build
	@echo "$(GREEN)✓ Nettoyage terminé$(NC)"

# =============================================================================
# Commandes de test et qualification

coverage: ## Génère un rapport de couverture de code (avec race detector)
	@echo "$(BLUE)Génération du rapport de couverture...$(NC)"
	cd $(BACKEND_DIR) && $(GO_TEST_ENV) go test -v -race -coverprofile=coverage.out ./... && go tool cover -func=coverage.out
	@echo "$(GREEN)✓ Rapport généré: $(BACKEND_DIR)/coverage.out$(NC)"

test-unit: ## Lance les tests unitaires Go (offline, race detector)
	@echo "$(BLUE)Tests unitaires Go...$(NC)"
	cd $(BACKEND_DIR) && $(GO_TEST_ENV) go test $(GO_TEST_FLAGS) -v -race -coverprofile=coverage.out ./...
	@echo "$(GREEN)✓ Tests unitaires terminés$(NC)"

test-integration: ## Lance les tests d'intégration (offline, ./tests)
	@echo "$(BLUE)Tests d'intégration...$(NC)"
	cd $(BACKEND_DIR) && $(GO_TEST_ENV) go test $(GO_TEST_FLAGS) -v -race $(GO_TEST_TIMEOUT) ./tests
	@echo "$(GREEN)✓ Tests d'intégration terminés$(NC)"

test-offline: ## Lance tous les tests en mode offline (CI standard, sans Proxmox)
	@echo "$(BLUE)Tests offline...$(NC)"
	cd $(BACKEND_DIR) && $(GO_TEST_ENV) go test $(GO_TEST_FLAGS) $(GO_TEST_TIMEOUT) ./...
	@echo "$(GREEN)✓ Tests offline terminés$(NC)"

test-offline-race: ## Lance tous les tests offline avec race detector (lent mais complet)
	@echo "$(BLUE)Tests offline + race detector...$(NC)"
	cd $(BACKEND_DIR) && $(GO_TEST_ENV) go test $(GO_TEST_FLAGS) -race -timeout=10m ./...
	@echo "$(GREEN)✓ Tests offline race terminés$(NC)"

test-online: ## Lance tous les tests en mode online (requiert Proxmox)
	@echo "$(BLUE)Tests online (Proxmox requis)...$(NC)"
	cd $(BACKEND_DIR) && GO_TEST_ENVIRONMENT=1 PVMSS_OFFLINE=false go test $(GO_TEST_FLAGS) -v -race -coverprofile=coverage.out ./...
	@echo "$(GREEN)✓ Tests online terminés$(NC)"

qualif: go-fmt go-lint test-offline ## Lance tous les contrôles qualité (fmt → lint → tests) puis dev
	@echo "$(GREEN)✓ Contrôles et tests réussis$(NC)"
	@echo ""
	@echo "$(BLUE)Démarrage de l'application...$(NC)"
	@$(MAKE) dev

# =============================================================================
# Commandes Go

go-lint: ## Lance le linter Go (golangci-lint)
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
# Commandes Frontend (SvelteKit)

frontend-install: ## Installe les dépendances bun du frontend SvelteKit
	@echo "$(BLUE)Installation des dépendances frontend...$(NC)"
	cd $(FRONTEND_DIR) && bun install --frozen-lockfile
	@echo "$(GREEN)✓ Dépendances installées$(NC)"

frontend-build: ## Construit le frontend SvelteKit (output: frontend/build/)
	@echo "$(BLUE)Construction du frontend SvelteKit...$(NC)"
	cd $(FRONTEND_DIR) && bun run build
	@echo "$(GREEN)✓ Frontend construit dans $(FRONTEND_DIR)/build/$(NC)"

frontend-dev: ## Démarre le serveur de dev SvelteKit (port 5173, proxy → :50000)
	@echo "$(BLUE)Serveur de dev frontend (port 5173)...$(NC)"
	@echo "$(BLUE)Proxy API → http://localhost:50000$(NC)"
	cd $(FRONTEND_DIR) && bun run dev

frontend-test: ## Lance les tests unitaires frontend (vitest)
	@echo "$(BLUE)Tests unitaires frontend (vitest)...$(NC)"
	cd $(FRONTEND_DIR) && bun run test
	@echo "$(GREEN)✓ Tests frontend terminés$(NC)"

frontend-check: ## Vérifie le typage du frontend (svelte-check)
	@echo "$(BLUE)Vérification du typage frontend (svelte-check)...$(NC)"
	cd $(FRONTEND_DIR) && bun run check
	@echo "$(GREEN)✓ Vérification frontend terminée$(NC)"
