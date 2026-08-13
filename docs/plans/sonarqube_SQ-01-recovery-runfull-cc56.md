# SonarQube SQ-01 — `recovery/run.go:81` go:S3776 (Cognitive Complexity 56 → 15)

- **Sévérité** : CRITICAL · **Effort estimé** : 46 min · **Règle** : go:S3776
- **Fonction** : `runFull(ctx, legacyDB, v04DB, opts, sum)` — orchestration Steps 2-9
- **Gate impact** : compte dans les « 10 New Code issues » → Quality Gate FAILED

## Contexte
`runFull` est une séquence linéaire de 8 étapes (nodes → storages → bridges →
isos → profiles → tags → vm_limits → node_limits). Chaque étape répète le même
motif : `map*()` → check err → boucle `if !opts.DryRun { upsert*() }` → incrément
`sum.*.Written`. La complexité cognitive (CC) atteint **56** (seuil 15).

Note : la fonction porte déjà `//nolint:gocyclo,funlen`, mais go:S3776 mesure la
**Cognitive Complexity**, pas la cyclomatique — le nolint golangci-lint ne
supprime PAS l'alerte SonarQube. Un vrai découpage est requis.

## Plan
1. Créer un helper générique pour le motif read/map/upsert :
   ```go
   type mapStep[T any] struct {
       label    string
       read     int
       skipped  int
       skipRsn  []string
       rows     []T
   }
   func runStep[T any](ctx, db, opts, m mapper[T], u upsertor[T], sum *Summary) error
   ```
   Alternative plus simple et idiomatique : extraire **une fonction par étape**
   (`stepNodes`, `stepStorages`, `stepBridges`, `stepISOs`, `stepProfiles`,
   `stepTags`, `stepVMLimits`, `stepNodeLimits`), chacune prenant
   `(ctx, legacyDB, v04DB, opts, *sum)` et retournant `error`. Chaque étape
   isolée tombe sous 15 CC.
2. `runFull` ne garde que l'ordre d'appel :
   ```go
   if err := stepNodes(ctx, legacyDB, v04DB, opts, &sum); err != nil { return sum, err }
   if err := stepStorages(...); err != nil { return sum, err }
   // ... etc
   ```
3. Rester sous le `//nolint:gocyclo` sur `runFull` (désormais justifié) ; le
   retirer sur les helpers extraits s'ils passent sous 15.
4. Vérifier que `Run` (le wrapper step 1) reste inchangé et que la signature
   publique de `recovery.Run` ne bouge pas.

## Vérification
- `cd server && go test ./internal/recovery/...` (doit rester vert — aucun
  changement de comportement, uniquement extraction de fonctions).
- `make sonar-scan-server` → l'issue `f510...` (run.go:81) disparaît.
- Relire la CC Sonar : `runFull` < 15.
