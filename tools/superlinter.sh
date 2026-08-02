#!/bin/bash

# Détecter la branche actuelle pour le linting offline
CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)

docker run --rm -e RUN_LOCAL=true --platform linux/amd64 \
  -e DEFAULT_BRANCH="$CURRENT_BRANCH" \
  -e VALIDATE_CSS=true \
  -e VALIDATE_MARKDOWN=true \
  -e VALIDATE_TOML=true \
  -e VALIDATE_ALL_CODEBASE=false \
  -e FILTER_REGEX_INCLUDE=".*\.css$" \
  -e FIX_CSS_PRETTIER=true \
  -e FIX_MARKDOWN_PRETTIER=true \
  -e FIX_YAML_PRETTIER=true \
  -e VALIDATE_ALL_CODEBASE=true \
  -v $(pwd)../.:/tmp/lint \
  ghcr.io/github/super-linter:latest

#  -e FILTER_REGEX_EXCLUDE="^.*/frontend/css/.*\.min\.css$,^.*/frontend/components/noVNC-1\.6\.0/.*$,^.*/\.github/.*$,.*\.scss$" \