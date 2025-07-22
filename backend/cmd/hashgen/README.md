# Password Hash Generator

Outil sécurisé pour générer des hash bcrypt pour l'authentification PVMSS.

## Utilisation

```bash
# Compiler l'outil
go build -o hashgen .

# Utiliser l'outil
./hashgen
```

## Exemple d'utilisation

```bash
$ ./hashgen
Enter password to hash: [mot de passe masqué]
Bcrypt hash: $2a$10$example...
For .env file: ADMIN_PASSWORD_HASH="$2a$10$example..."
Remember to escape '$' characters with '$$' in .env file!
```

## Configuration dans .env

```bash
# Dans votre fichier .env, escapez les '$' avec '$$'
ADMIN_PASSWORD_HASH="$$2a$$10$$example..."
```

## Sécurité

- ✅ Le mot de passe n'apparaît pas dans l'historique des commandes
- ✅ Saisie masquée avec `golang.org/x/term`
- ✅ Utilise bcrypt avec le coût par défaut (sécurisé)
- ✅ Rappel d'échapper les caractères '$' pour les fichiers .env
