# SonarQube SQ-10 — `vm/bulk.go:90` go:S107 (8 paramètres > 7)

- **Sévérité** : MAJOR · **Effort estimé** : 20 min · **Règle** : go:S107
- **Fonction** : `BulkAction(ctx, resolver ClusterIndexResolver, actor auth.Identity, targets []BulkTarget, kind string, writer cluster.Writer, audit AuditRecorder, refresher IndexRefresher)`
- **Gate impact** : compte dans les « 10 New Code issues » → Quality Gate FAILED

## Contexte
`BulkAction` prend 8 paramètres. Les 5 dépendances
(`resolver`, `actor`, `writer`, `audit`, `refresher`) + `kind` forment un
groupe cohérent. Note : `BulkAction` appelle `Action(ctx, index, actor,
target.Cluster, target.VMID, kind, writer, audit, refresher)` (ligne 118) qui a
aussi 9 paramètres — traiter les deux ensemble.

## Plan
1. Définir une struct de dépendances (package `vm`) :
   ```go
   type BulkDeps struct {
       Resolver  ClusterIndexResolver
       Actor     auth.Identity
       Writer    cluster.Writer
       Audit     AuditRecorder
       Refresher IndexRefresher
   }
   ```
2. Nouvelle signature :
   ```go
   func BulkAction(ctx context.Context, deps BulkDeps, targets []BulkTarget, kind string) []BulkTargetResult
   ```
3. Faire de même pour `Action` si elle est exportée/partagée (sinon laisser
   `Action` interne mais réduire ses params en passant `deps` + `index`) :
   ```go
   func Action(ctx context.Context, deps BulkDeps, index *ClusterIndex, cluster string, vmid int, kind string) error
   ```
4. **Mettre à jour les call sites** : `grep -rn "BulkAction(\|Action(" server/`
   pour tous les appelants (handler bulk + tests) → construire `vm.BulkDeps{...}`.
5. Préserver le comportement : `result.Status`/`result.Message` inchangés.

## Vérification
- `cd server && go build ./... && go test ./internal/vm/... ./internal/httpapi/...`
  (vert — call sites mis à jour).
- `make sonar-scan-server` → l'issue `c0256d0b...` (bulk.go:90) disparaît.
