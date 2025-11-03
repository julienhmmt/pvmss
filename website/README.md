# Misc informations

## Développement

Docker est utilisé pour développer le site web, en utilisant le container node:22.21-alpine et Astro. Pour lancer le container, il faut exécuter la commande suivante :

```bash
docker run -it --rm \
  -v "$(pwd)/website":/website \
  -w /website \
  node:22.21-alpine sh
```

La création de la base du site web est faite avec la commande suivante :

```bash
npm create astro@latest -- --add svelte
```

## Déploiement

Le déploiement est fait avec le container nginx:stable-alpine-slim.

```bash
docker compose -f docker-compose.website.yml build
docker compose -f docker-compose.website.yml up -d
```
