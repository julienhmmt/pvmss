# Misc informations

## Développement

Docker est utilisé pour développer le site web, en utilisant le container node:25.2-alpine et Astro. Pour lancer le container, il faut exécuter la commande suivante :

```bash
docker run -it --rm \
  -v "$(pwd)/website":/website \
  -w /website \
  node:25.2-alpine ash
```

La création de la base du site web est faite avec la commande suivante dans le conteneur :

```bash
npm create astro@latest -- --add svelte
apk add git
npm i @astrojs/check typescript
```

### Base de données

Une base de données intégrée à Astro est initiée mais n'est pas utilisée.

```bash
npx astro db

  astro db [command] [...flags]

  Commands 
    push  Push table schema updates to libSQL.
    verify  Test schema updates with libSQL (good for CI).
    execute <file-path>  Execute a ts/js file using astro:db. Use --remote to connect to libSQL.
    shell --query <sql-string>  Execute a SQL string. Use --remote to connect to libSQL.
```

## Mise à jour

```bash

docker run -it --rm \
  -v "$(pwd)/website":/website \
  -w /website \
  node:25.2-alpine npx astro check
```

## Déploiement

Le déploiement est fait avec le container nginx:stable-alpine-slim.

```bash
docker compose -f docker-compose.website.yml build
docker compose -f docker-compose.website.yml up -d
```
