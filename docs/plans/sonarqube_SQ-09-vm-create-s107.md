# SonarQube SQ-09 — `vm/create.go:145` go:S107 (10 paramètres > 7)

- **Sévérité** : MAJOR · **Effort estimé** : 20 min · **Règle** : go:S107
- **Fonction** : `Create(ctx, actor, clusterName string, req CreateRequest, st *store.Store, creator cluster.Creator, pusher CloudInitPusher, audit AuditRecorder, log *slog.Logger, services ...*policy.Policy)`
- **Gate impact** : compte dans les « 10 New Code issues » → Quality Gate FAILED

## Contexte
`Create` prend 10 paramètres (dont le variadique `services`). go:S107 exige ≤ 7.
Les dépendances (`st`, `creator`, `pusher`, `audit`, `log`) forment un groupe
cohérent — candidat idéal à une struct de dépendances.

## Plan
1. Définir une struct de dépendances (même package `vm`) :
   ```go
   type CreateDeps struct {
       Store    *store.Store
       Creator  cluster.Creator
       Pusher   CloudInitPusher
       Audit    AuditRecorder
       Log      *slog.Logger
       Services []*policy.Policy // optionnel
   }
   ```
2. Nouvelle signature :
   ```go
   func Create(ctx context.Context, actor auth.Identity, clusterName string,
       req CreateRequest, deps CreateDeps) (CreateResult, error)
   ```
3. À l'intérieur, `policyService := selectPolicyService(deps.Store, deps.Services)`
   et utiliser `deps.X` partout.
4. **Mettre à jour les call sites** : `grep -rn "vm.Create(" server/` pour tous
   les appelants (handler `vm_create.go` + tests) → construire `vm.CreateDeps{...}`.
5. Vérifier qu'aucun autre paramètre (ctx/actor/clusterName/req) n'est perdu.

## Vérification
- `cd server && go build ./... && go test ./internal/vm/... ./internal/httpapi/...`
  (vert — call sites mis à jour).
- `make sonar-scan-server` → l'issue `d47a2826...` (create.go:145) disparaît.
