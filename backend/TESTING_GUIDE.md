# Guide de Test des Améliorations PVMSS

## 🧪 Tests de Sécurité

### 1. Test CSRF Protection
```bash
# Test sans token CSRF - devrait retourner 403
curl -X POST http://localhost:50000/login \
  -d "password=test" \
  -H "Content-Type: application/x-www-form-urlencoded"
```
**Résultat attendu**: `403 Forbidden - Invalid CSRF token`

### 2. Test Rate Limiting
```bash
# Faire 6 tentatives de login rapides
for i in {1..6}; do
  echo "Tentative $i"
  curl -X POST http://localhost:50000/login \
    -d "password=mauvais_mot_de_passe" \
    -H "Content-Type: application/x-www-form-urlencoded"
  sleep 1
done
```
**Résultat attendu**: Après 5 tentatives, message "Too many login attempts"

### 3. Test Headers de Sécurité
```bash
curl -I http://localhost:50000/
```
**Headers attendus**:
- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `X-XSS-Protection: 1; mode=block`
- `Content-Security-Policy: default-src 'self'...`

### 4. Test Validation d'Entrée
```bash
# Test avec entrée trop longue
curl -X POST http://localhost:50000/api/search \
  -d "name=$(python3 -c 'print("A" * 1000)')" \
  -H "Content-Type: application/x-www-form-urlencoded"
```
**Résultat attendu**: Entrée tronquée à 100 caractères

## 🚀 Tests de Performance

### 1. Test Cache Settings
```bash
# Mesurer le temps de réponse de la page admin
time curl -s http://localhost:50000/admin > /dev/null
```
**Résultat attendu**: Temps réduit grâce au cache des settings

### 2. Test Proxmox Cache
```bash
# Vérifier les logs pour les hits de cache
tail -f logs/app.log | grep "cached"
```

## 🔍 Tests Fonctionnels

### 1. Test Login avec CSRF
1. Ouvrir http://localhost:50000/login dans un navigateur
2. Inspecter le formulaire pour vérifier la présence du champ `csrf_token`
3. Tenter de se connecter avec un mot de passe valide
4. Vérifier la redirection vers `/admin`

### 2. Test Validation VM Name
1. Aller sur la page de création de VM
2. Essayer des noms avec caractères spéciaux
3. Vérifier que seuls les caractères alphanumériques, `-`, `_` et espaces sont acceptés

## 📊 Monitoring des Logs

### Structure des Logs Améliorée
```bash
tail -f logs/app.log
```
**Rechercher**:
- Messages avec IP pour les events de sécurité
- Logs d'erreur détaillés avec contexte
- Messages de cache hit/miss

### Niveaux de Log
- `ERROR`: Erreurs critiques
- `WARN`: Tentatives de connexion échouées, violations CSRF
- `INFO`: Requêtes normales avec contexte IP
- `DEBUG`: Détails cache, settings loading

## ⚙️ Outils Utiles

### Générer un Hash de Mot de Passe
```bash
cd backend/cmd/hashgen
# Compiler et utiliser l'outil sécurisé:
go build -o hashgen .
./hashgen
# L'outil masquera automatiquement votre saisie
```

### Tester les Endpoints API
```bash
# Health check
curl http://localhost:50000/health

# Settings (nécessite authentification)
curl -H "Cookie: session=XXX" http://localhost:50000/api/settings
```

## 🛡️ Vérification de Sécurité

### Checklist Post-Déploiement
- [ ] CSRF tokens présents sur tous les formulaires
- [ ] Rate limiting actif sur login
- [ ] Headers de sécurité présents
- [ ] Validation d'entrée active
- [ ] SSL optionnel fonctionnel (`PROXMOX_VERIFY_SSL=false`)
- [ ] Logs de sécurité détaillés
- [ ] Pas de fuite d'information sensible dans les logs
- [ ] Build réussi sans warnings

### Scan de Sécurité Basique
```bash
# Test XSS simple
curl -X POST http://localhost:50000/api/search \
  -d "name=<script>alert('xss')</script>"

# Test injection
curl -X POST http://localhost:50000/api/search \
  -d "vmid='; DROP TABLE users; --"
```
**Résultat attendu**: Entrées échappées/validées, pas d'exécution malveillante
