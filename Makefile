# Makefile pour PVMSS
# Permet de construire, démarrer, arrêter, nettoyer et tester l'application

.PHONY: help dev dev-logs build docker-build up down restart logs test coverage test-unit test-integration test-routes go-lint go-fmt dev dev-logs

# Couleurs pour l'affichage
BLUE=\033[0;34m
GREEN=\033[0;32m
RED=\033[0;31m
NC=\033[0m # No Color

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

build: ## Construit le binaire Go et le container docker
	@echo "Building binary" && cd /Users/jh/git/pvmss && go clean -cache && go build -C backend -o ../pvmss
	@echo "Binary built successfully"
	@echo "Building and running docker container"
	docker compose -f docker-compose.dev.yml down
	docker compose -f docker-compose.dev.yml build --no-cache
	docker compose -f docker-compose.dev.yml up -d
	@echo ""

docker-build: ## Construit les images docker (arm64 et amd64) et push sur Docker Hub
	@echo "Building Docker images for multiple architectures..."
	@echo "Usage: make docker-build PVMSS_TAG=your-tag"
	@echo "Default tag: latest"
	@docker buildx build --platform linux/amd64,linux/arm64 -t jhmmt/pvmss:$(or $(PVMSS_TAG),latest) --push .
	@echo "Docker images built successfully"
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
	cd backend && go test -v -race -coverprofile=coverage.out ./...
	@echo "$(GREEN)✓ Rapport généré: backend/coverage.out$(NC)"

test-unit: ## Lance les tests unitaires Go
	@echo "$(BLUE)Lancement des tests unitaires Go...$(NC)"
	cd backend && go test -v -race -coverprofile=coverage.out ./...
	@echo "$(GREEN)✓ Tests unitaires terminés$(NC)"

test-integration: ## Lance les tests d'intégration (requiert docker-compose.test.yml)
	@echo "$(BLUE)Lancement des tests d'intégration...$(NC)"
	cd backend && go test -v -race -tags=integration -timeout=5m ./tests/...
	@echo "$(GREEN)✓ Tests d'intégration terminés$(NC)"

test-routes: ## Lance les tests de routes (requiert l'app en cours d'exécution)
	@echo "$(BLUE)Lancement des tests de routes...$(NC)"
	cd backend && go test -v -run TestRouteAccessibility ./tests
	@echo "$(GREEN)✓ Tests de routes terminés$(NC)"

test: test-unit test-integration test-routes ## Lance tous les tests Go
	@echo "$(BLUE)Lancement de tous les tests Go...$(NC)"
	@echo "$(GREEN)✓ Tests terminés$(NC)"

# =============================================================================
# Commandes Go

go-lint: ## Lance le linter Go
	@echo "$(BLUE)Lancement du linter Go...$(NC)"
	cd backend && golangci-lint run -v --timeout=3m

go-fmt: ## Formate le code Go
	@echo "$(BLUE)Formatage du code Go...$(NC)"
	cd backend && go fmt ./...

# =============================================================================
# Commandes de développement rapide

qualif: ## Lance tous les contrôles qualité (format, lint, tests)
	@echo "$(BLUE)[1/3] Formatage du code Go...$(NC)"
	@$(MAKE) go-fmt || { echo "$(RED)❌ Formatage échoué$(NC)"; exit 1; }
	@echo ""
	@echo "$(BLUE)[2/3] Linting du code Go...$(NC)"
	@$(MAKE) go-lint || { echo "$(RED)❌ Linting échoué$(NC)"; exit 1; }
	@echo ""
	@echo "$(BLUE)[3/3] Tests Go...$(NC)"
	@$(MAKE) test || { echo "$(RED)❌ Tests échoués$(NC)"; exit 1; }
	@echo ""
	@echo "$(GREEN)✓ Contrôles et tests réussis!$(NC)"
	@echo ""
	@echo "$(BLUE)Démarrage de l'application...$(NC)"
	@$(MAKE) dev-logs
	@echo ""

.DEFAULT_GOAL := help
