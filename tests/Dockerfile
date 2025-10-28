# Conteneur de test pour PVMSS
# Ce conteneur contient curl et bash pour exécuter les tests automatisés
FROM alpine:latest

# Installation des dépendances pour les tests
RUN apk add --no-cache \
    bash \
    curl \
    grep \
    sed \
    coreutils

# Création du répertoire de travail
WORKDIR /tests

# Copie du script de test
COPY test-routes.sh /tests/
RUN chmod +x /tests/test-routes.sh

# Point d'entrée par défaut
ENTRYPOINT ["/tests/test-routes.sh"]
