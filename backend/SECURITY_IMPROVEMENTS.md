# PVMSS Security & Code Quality Improvements

## 🔒 Améliorations de Sécurité

### 1. Protection CSRF

- **Nouveau fichier**: `security.go`
- **Implémentation**: Tokens CSRF uniques générés pour chaque formulaire
- **Validation**: Middleware CSRF qui valide les tokens pour toutes les requêtes POST/PUT/DELETE
- **Intégration**: Template login.html mis à jour avec le token CSRF

### 2. Limitation du Taux de Tentatives (Rate Limiting)

- **Protection login**: Maximum 5 tentatives par IP en 15 minutes
- **Logging**: Enregistrement des tentatives de connexion échouées
- **Blocage temporaire**: IP bloquées après dépassement du seuil

### 3. Validation d'Entrée

- **Échappement HTML**: Toutes les entrées utilisateur sont échappées
- **Longueur limitée**: Validation de la longueur maximale des champs
- **Caractères valides**: Validation des noms de VM (alphanumériques + - _ espace)

### 4. Headers de Sécurité

- **X-Content-Type-Options**: `nosniff`
- **X-Frame-Options**: `DENY` (protection clickjacking)
- **X-XSS-Protection**: `1; mode=block`
- **Content-Security-Policy**: CSP restrictive
- **Referrer-Policy**: `strict-origin-when-cross-origin`

## 🔧 Optimisations de Code

### 1. Suppression des Doublons

- **Settings struct**: Suppression du doublon dans `main.go`, utilisation de `AppSettings` depuis `settings.go`
- **Outil sécurisé**: `cmd/hashgen/main.go` : Outil sécurisé pour générer des hash bcrypt (masque le mot de passe)
- **Logger consistant**: Uniformisation de l'utilisation du logger via `logger.Get()`

### 2. Cache des Settings

- **Optimisation**: Les settings sont maintenant cachées en mémoire au lieu d'être relues du disque
- **Performance**: Réduction des accès disque dans `adminHandler`

### 3. Logging Cohérent

- **Standardisation**: Tous les modules utilisent `logger.Get()` au lieu de `log` direct
- **Consistance**: Messages de log uniformisés avec contexte (IP, path, handler)

## 📋 Détails Techniques

### Middleware Chain

```
Request → Security Headers → Session → CSRF → Router → Handler
```

### Nouveaux Fichiers

- `security.go`: Fonctions de sécurité et middlewares

### Fichiers Modifiés

- `main.go`: Intégration des middlewares de sécurité, optimisation cache
- `settings.go`: Standardisation du logger
- `login.html`: Ajout protection CSRF

### Variables d'Environnement

- `PROXMOX_VERIFY_SSL=false`: Option SSL désactivable maintenue ✅

## 🚀 Impact Performance

### Avant

- Settings relues du disque à chaque requête admin
- Logger inconsistant
- Pas de cache des templates (déjà optimisé)

### Après

- Settings mises en cache au démarrage
- Logger unifié
- Protection sécurité sans impact performance significatif

## ⚡ Tests Recommandés

1. **Test CSRF**: Tentative POST sans token → 403 Forbidden
2. **Test Rate Limit**: 5+ tentatives login → Blocage temporaire
3. **Test Headers**: Vérification headers sécurité dans réponses
4. **Test Performance**: Temps de réponse admin page amélioré

## 📝 Notes Importantes

- **SSL Optional**: L'option `PROXMOX_VERIFY_SSL=false` est conservée comme demandé
- **Compatibilité**: Aucun changement breaking pour les utilisateurs
- **Logging**: Niveau DEBUG maintenu, logs plus détaillés avec contexte IP
- **Bulma CSS**: Framework CSS conservé, pas d'ajout JavaScript superflu
