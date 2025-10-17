# Makefile pour PVMSS
# Permet de construire, démarrer, arrêter, nettoyer et tester l'application

.PHONY: help build up down logs test restart go-routes go-test go-lint go-fmt dev dev-logs qualif quick-test settings-show env-example

# Couleurs pour l'affichage
BLUE=\033[0;34m
GREEN=\033[0;32m
RED=\033[0;31m
NC=\033[0m # No Color

help: ## Affiche cette aide
	@echo "$(BLUE)PVMSS - Commandes disponibles:$(NC)"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-15s$(NC) %s\n", $$1, $$2}'
	@echo ""

# =============================================================================
# Commandes de développement
# =============================================================================

build: ## Construit le binaire Go
	@echo "Building binary" && cd /Users/jh/git/pvmss && go clean -cache && go build -C backend -o ../pvmss
	@echo "Binary built successfully"
	@echo "Building docker container"
	docker compose -f docker-compose.dev.yml down
	docker compose -f docker-compose.dev.yml build --no-cache
	docker compose -f docker-compose.dev.yml up -d

docker-build: ## Construit les images docker
	@echo "Building Docker images for multiple architectures..."
	@echo "Usage: make docker-build PVMSS_TAG=your-tag"
	@echo "Default tag: latest"
	@docker buildx build --platform linux/amd64,linux/arm64 -t jhmmt/pvmss:$(or $(PVMSS_TAG),latest) --push .
	@echo "Docker images built successfully"

docker-push: ## Push les images docker
	@docker push jhmmt/pvmss:$(or $(PVMSS_TAG),latest)
	@echo "Docker images pushed successfully"

up:	## Démarre l'application
	@docker compose -f docker-compose.dev.yml up -d

down:	## Arrête l'application
	@docker compose -f docker-compose.dev.yml down

logs:	## Affiche les logs de l'application
	@docker compose -f docker-compose.dev.yml logs -f pvmss

restart: ## Redémarre l'application
	@down up

# =============================================================================
# Commandes de test
# =============================================================================

# test: ## Lance les tests automatisés
# 	@echo "$(BLUE)Lancement des tests automatisés...$(NC)"
# 	docker compose -f docker-compose.test.yml up --build --abort-on-container-exit
# 	@echo "$(GREEN)Tests terminés!$(NC)"

# test-verbose: ## Lance les tests en mode verbose
# 	@echo "$(BLUE)Lancement des tests (mode verbose)...$(NC)"
# 	VERBOSE=1 docker compose -f docker-compose.test.yml up --build --abort-on-container-exit

# test-clean: ## Lance les tests puis nettoie
# 	@echo "$(BLUE)Lancement des tests avec nettoyage...$(NC)"
# 	docker compose -f docker-compose.test.yml up --build --abort-on-container-exit; \
# 	EXIT_CODE=$$?; \
# 	docker compose -f docker-compose.test.yml down -v; \
# 	exit $$EXIT_CODE

# test-shell: ## Ouvre un shell dans le conteneur de test
# 	docker compose -f docker-compose.test.yml run --rm tests /bin/bash

test-local: ## Lance les tests localement (requiert l'app en cours d'exécution)
	@echo "$(BLUE)Lancement des tests locaux...$(NC)"
	BASE_URL="http://localhost:50000" \
	ADMIN_USERNAME="admin" \
	ADMIN_PASSWORD="admin" \
	VERBOSE=1 \
	./tests/test-routes.sh

test-go-routes: ## Lance les tests de routes en Go (requiert l'app en cours d'exécution)
	@echo "$(BLUE)Compilation et lancement des tests de routes Go...$(NC)"
	@cd tests && go run test-routes.go -verbose

# =============================================================================
# Commandes Go
# =============================================================================

go-test: ## Lance les tests Go unitaires
	@echo "$(BLUE)Lancement des tests Go...$(NC)"
	cd backend && go test ./... -v

go-lint: ## Lance le linter Go
	@echo "$(BLUE)Lancement du linter Go...$(NC)"
	cd backend && golangci-lint run

go-fmt: ## Formate le code Go
	@echo "$(BLUE)Formatage du code Go...$(NC)"
	cd backend && go fmt ./...

go-routes: ## Lance les tests de routes en Go
	@echo "$(BLUE)Compilation et lancement des tests de routes Go...$(NC)"
	@cd tests && go run test-routes.go -verbose

# =============================================================================
# Commandes de développement rapide
# =============================================================================

dev: build up ## Build et démarre l'application
	@echo "$(GREEN)Application démarrée sur http://localhost:50000$(NC)"

dev-logs: up logs ## Démarre et affiche les logs

qualif: ## Lance tous les contrôles qualité (format, lint, tests)
	@echo "$(BLUE)========================================$(NC)"
	@echo "$(BLUE)Contrôles qualité PVMSS$(NC)"
	@echo "$(BLUE)========================================$(NC)"
	@echo ""
	@echo "$(BLUE)[1/4] Formatage du code Go...$(NC)"
	@$(MAKE) go-fmt || echo "$(RED)❌ Formatage échoué$(NC)"
	@echo ""
	@echo "$(BLUE)[2/4] Linting du code Go...$(NC)"
	@$(MAKE) go-lint || echo "$(RED)❌ Linting échoué$(NC)"
	@echo ""
	@echo "$(BLUE)[3/4] Tests de routes...$(NC)"
	@$(MAKE) go-routes || echo "$(RED)❌ Tests de routes échoués$(NC)"
	@echo ""
	@echo "$(BLUE)[4/4] Tests Go unitaires...$(NC)"
	@$(MAKE) go-test || echo "$(RED)❌ Tests Go échoués$(NC)"
	@echo ""
	@echo "$(BLUE)========================================$(NC)"
	@echo "$(GREEN)✓ Contrôles qualité terminés$(NC)"
	@echo "$(BLUE)========================================$(NC)"

quick-test: up ## Démarre l'app et lance les tests rapidement
	@echo "$(BLUE)Attente que l'application soit prête...$(NC)"
	@sleep 2
	@$(MAKE) test-local && $(MAKE) go-test

# =============================================================================
# Commandes utilitaires
# =============================================================================

settings-show: ## Affiche le fichier settings.json
	@cat backend/settings.json

env-example: ## Copie env.example vers .env
	@if [ -f .env ]; then \
		echo "$(BLUE)Le fichier .env existe déjà. Voulez-vous le remplacer? [y/N]$(NC)"; \
		read -r response; \
		if [ "$$response" = "y" ]; then \
			cp env.example .env; \
			echo "$(GREEN).env créé depuis env.example$(NC)"; \
		fi; \
	else \
		cp env.example .env; \
		echo "$(GREEN).env créé depuis env.example$(NC)"; \
	fi

# =============================================================================
.DEFAULT_GOAL := help
