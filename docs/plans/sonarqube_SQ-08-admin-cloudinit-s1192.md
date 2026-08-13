# SonarQube SQ-08 — `httpapi/admin_cloudinit_templates.go:45` go:S1192 (littéral dupliqué)

- **Sévérité** : CRITICAL · **Effort estimé** : 6 min · **Règle** : go:S1192
- **Ligne** : 45 — `"internal server error"` dupliqué 3× dans ce fichier
- **Gate impact** : compte dans les « 10 New Code issues » → Quality Gate FAILED

## Contexte
`ServeCloudInitTemplates` (ligne 45) écrit `writeAdminError(w,
http.StatusInternalServerError, "internal_error", "internal server error")`.
Le même littéral `"internal server error"` apparaît 3× dans ce fichier, et des
dizaines de fois dans tout le package `httpapi` (ex. `vm_detail.go` en a 17).

## Plan
1. Créer (ou étendre) un fichier de constantes du package `httpapi`, ex.
   `server/internal/httpapi/consts.go` :
   ```go
   package httpapi

   // Message texts shared across admin handlers (go:S1192).
   const (
       msgInternalServerError = "internal server error"
       codeInternalError      = "internal_error"
   )
   ```
2. Remplacer les 3 occurrences de ce fichier par `msgInternalServerError` /
   `codeInternalError`.
3. **Bonus (hors New Code, ne bloque pas le gate mais recommandé)** : migrer
   aussi les autres handlers du package (`vm_detail.go`, `admin_catalog.go`,
   `auth.go`, `admin_tags.go`, `admin_profiles.go`, `vm_create.go`…) vers ces
   constantes — couvre les ~80 autres alertes go:S1192 hors période de fuite.
4. Ne pas changer le `code` `"internal_error"` (déjà une chaîne distincte, à
   constantiser aussi pour cohérence).

## Vérification
- `cd server && go test ./internal/httpapi/...` (vert — strings identiques).
- `make sonar-scan-server` → l'issue `02be3baa...` (admin_cloudinit_templates.go:45) disparaît.
- Si le bonus est fait : la majorité des alertes go:S1192 du Overall Code
  disparaît aussi (maintenabilité A préservée).
