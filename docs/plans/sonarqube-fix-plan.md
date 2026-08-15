# Plan de correction SonarQube — pvmss-server

## Récapitulatif — ÉTAT FINAL (après corrections)

**Problèmes de Go (code smells) : RÉSOLUS.** Le re-scan SonarQube confirme
`new_violations = 0` sur le new-code. Toutes les règles go:S1192 / go:S107 /
go:S1871 / go:S3776 sur les lignes modifiées sont corrigées (19 commits).

**Seul bloqueur restant du Quality Gate : `new_coverage = 78.1%` (seuil 80%).**
Conformément à la consigne « ne fait pas trop de coverage pour l'instant », la
remontée de la coverage new-code vers 80% n'a pas été traitée en priorité.

Répartition initiale (75 issues) :

| Catégorie | Count | Exemple le plus lourd |
|---|---|---|
| `go:S3776` Cognitive Complexity | 28 | `contract_test.go:21` CC 52, `log_test.go:15` CC 48, `health_test.go:25` CC 45 |
| `go:S1192` Littéraux dupliqués | 20+ | `vm_detail.go:362` "internal server error" ×17 |
| `go:S107` >7 paramètres | 15 | `cloudinit.go:43` 11 params, `actions.go:137` 10 params |
| `go:S1871` Branches dupliquées | 2 | `fake.go:443/446` |

**Measures (re-scan) :** new-code coverage 78.1%, new_duplicated_lines_density 0.30% (OK),
new_violations 0 (OK). Gate échoue uniquement sur new_coverage < 80%.

Les ~115 « leak issues » restantes sont des issues **historiques** sur du code non
modifié (ou des artefacts générés `graphify-out/graph.html` désormais exclus du
scan) — elles n'empêchent pas le gate new-code.
Fixer les issues UNIQUEMENT ne suffit pas — il faut aussi remonter la coverage.

---

## Priorisation

Ordre d'attaque recommandé:

1. **S1192 (littéraux)** — Correction mécanique, faible risque, grande valeur. On introduit des constantes, les tests ne bougent pas.attaque recommandée.
2. **S107 (params)** — Refactor en struct de dépendances. Modifie les signatures mais pas la logique.
3. **S3776 (cognitive complexity)** — Refactor, touche surtout les tests. Risque modéré mais chaque refactor doit être verifié.
4. **S1871 (branches)** — Deux lignes dans fake.go, facile.

---

## A. go:S3776 — Réduction de Complexité Cognitive

### Approche générale

**Règle d'or**: `go:S3776` est la Cognitive Complexity de SonarQube (seuil: 15). Ce n'est PAS la Cyclomatic Complexity de golangci-lint. Les `//nolint:gocyclo` et `//nolint:funlen` N'ONT AUCUN EFFET sur go:S3776 — ne PAS ajouter de nolint.

**Pour les tests table-driven** (la majorité des S3776): la stratégie est d'extraire les cas et les assertions dans des helpers séparés, ou de décomposer la fonction en sous-fonctions.

**Pour le code de production**: extraire des helpers, décomposer les switchs.

### Fichiers et fonctions à refactoriser

#### Tests (CC > 15, classés par sévérité)

##### CC 52 — `server/internal/cluster/contract_test.go:21` — `TestContract_Snapshot`
Test contractuel qui vérifie tous les invariants du snapshot (nodes, VMs, storages) sur fake et proxmox.
**Problème**: un seul gros `for name, impl := range impls { t.Run(...) { ... } }` avec toutes les assertions inline.

**Refactor proposé**:
- Extraire `assertSnapshotNodes(t *testing.T, snap)` — vérifie les 3 boucles sur nodes
- Extraire `assertSnapshotVMs(t *testing.T, snap)` — vérifie les VMs
- Extraire `assertSnapshotStorages(t *testing.T, snap)` — vérifie les storages
- La fonction de test devient: charger les impls, pour chaque faire le t.Run, appeler les helpers. CC tombe sous 15.

##### CC 48 — `server/internal/config/log_test.go:15` — `TestNewLogger`
Test table-driven avec ~10+ cases, chaque case ayant un bloc validate() inline avec plusieurs assertions.
**Refactor proposé**:
- Extraire `assertLogOutput(t *testing.T, path, expectedEntries)` en helper séparé
- Simplifier chaque case: ne garder que la config + le nom, déléguer la validation au helper
- Si le nombre de cases est trop gros, grouper par famille (json vs text, avec erreur vs sans)

##### CC 45 — `server/internal/httpapi/health_test.go:25` — `TestHealth`
3 cases seulement (healthy, unhealthy, method not allowed) mais chaque case a un gros bloc d'assertions inline.
**Refactor proposé**:
- Extraire `assertHealthResponse(t *testing.T, rec, wantStatus, wantBody)` 
- Extraire `assertHealthAllowHeader(t *testing.T, rec)` pour le cas 405
- Le corps du test devient une loop simple sur les cases

##### CC 29 — `server/internal/store/store_test.go:62` — `TestOpen`
3 cases, chaque case avec setup + assertions inline.
**Refactor proposé**:
- Extraire `runOpenCase(t *testing.T, c)` qui encapsule le setup et les assertions
- Le test devient une loop sur les cases en appelant le helper

##### CC 24 — `server/internal/httpapi/vm_detail.go:381` — `handleDisk` (PRODUCTION)
Handler avec un switch sur POST/PUT/DELETE qui fait à chaque branche: decode JSON → vérifier inventory → appeler vm.* → re-resolve → return.
**Problème**: la branche PUT (resize) a un bloc re-resolve identique à celui de PATCH dans handlePatch (ligne 340-375).

**Refactor proposé**:
- Extraire `resolveAndWriteEntity(h, w, identity, clusterName, vmid)` qui fait le Load() + Resolve() + writeEntity, ou writeDetailError sur failure
- Les branches POST/PUT/DELETE deviennent plus courtes

##### CC 24 — `server/internal/cluster/client_test.go:111`
Test VNC proxy avec plusieurs sous-tests.
**Refactor proposé**: identifier les sous-blocs répétitifs, extraire des helpers.

##### CC 23 — `server/internal/vm/create_policy_test.go:15` — `TestCreate_PolicyGuardsRejectBeforeAllocation`
3 policy cases (quota, gabarit, node capacity), chaque case avec un bloc prepare() inline + assertions.
**Refactor proposé**:
- Extraire `assertPolicyRejection(t *testing.T, t, deps, wantErr)` helper pour le motif commun: créer la VM, vérifier l'erreur
- Chaque case ne garde que le prepare()

##### CC 23 — `server/internal/vm/cdrom_test.go:15`
Test des opérations CDROM. Table-driven avec assertions inline.
**Refactor proposé**: extraire `assertCDROMState(t, ...)` helper.

##### CC 23 — `server/internal/vm/resolve_test.go:36`
Test de résolution VM (ownership, not found, etc.).
**Refactor proposé**: extraire les cas en sous-fonctions.

##### CC 23 — `server/internal/cluster/fake_create_test.go:15` — `TestFake_CreateVM_RecordsVMInDataset`
Test de création VM avec plusieurs assertions sur l'état du fake après création.
**Refactor proposé**: extraire `assertCreatedVM(t, client, vmid, expectedID, expectedOwner, ...)` helper.

##### CC 22 — `server/internal/vm/network_test.go:16` — `TestUpdateNetwork_ValidatesBridgeAndCardCount`
4 cases (approved bridge, unapproved, invalid model, non-owner). Chaque case a le même motif: setup deps → call UpdateNetwork → assert.
**Refactor proposé**:
- Extraire `runNetworkCase(t *testing.T, test)` helper qui fait le setup+call+assert
- Les 4 cases ne gardent que la définition du test case struct

##### CC 22 — `server/internal/cluster/contract_test.go:315`
Autre fonction de test contractuel.
**Refactor proposé**: même approche — extraire les assertions en helpers.

##### CC 22 — `server/internal/vm/snapshots_test.go:53` — `TestValidateSnapshotName`
7 cases de validation de nom de snapshot. Chaque case est un t.Run avec une assertion simple.
**Refactor proposé**:
- Le pattern est déjà assez propre (t.Run par case). La CC vient probablement des imbrications t.Run → t.Parallel → assertions.
- Extraire `assertSnapshotName(t *testing.T, input, wantErr)` helper appelé depuis chaque case

##### CC 22 — `server/internal/httpapi/inventory.go:36` — `ServeHTTP` (ClusterRefresh) (PRODUCTION)
Handler avec un switch sur les erreurs de refresh (too soon, unreachable, default) + marshaling JSON.
**Refactor proposé**:
- Extraire `writeRefreshTooSoon(w, retryAfter)` 
- Extraire `writeRefreshUnreachable(w)`
- Le switch devient plus lisible

##### CC 22 — `server/internal/httpapi/vm_detail_test.go`
Test VMDetail_Get (phase 3 du fichier). La lecture du fichier montre que chaque fonction de test (`TestVMDetail_Get_OwnerSeesFullEntity`, `TestVMDetail_Get_UnauthenticatedRejected`, etc.) est relativement courte mais la CC agrégée du fichier est élevée car beaucoup de fonctions de test dans le même fichier. Vérifier si la CC 22 est sur une fonction spécifique ou agrégée.
**Refactor proposé**: si c'est une fonction spécifique, l'identifier et extraire ses assertions.

##### CC 21 — `server/internal/policy/policy_test.go:61`
**Refactor proposé**: extraire helpers d'assertion.

##### CC 19 — `server/internal/httpapi/router.go:63` — `NewRouter` (PRODUCTION)
Registration de routes avec nolint:gocyclo,funlen déjà présent. CC 19 > 15.
**Refactor proposé**:
- Extraire `registerVMROutes(mux, cfg)` pour regrouper les routes VM
- Extraire `registerAdminRoutes(mux, cfg)` pour les routes admin
- Extraire `registerAuthRoutes(mux, cfg)` pour auth
- Extraire `setupRateLimiter(mux, cfg)` pour le rate limiting

##### CC 19 — `server/internal/cluster/rfb_fake_test.go:18`
**Refactor proposé**: extraire helpers.

##### CC 19 — `server/internal/vm/disks_test.go:17` — `TestDiskOperations_GuardsAndWrites`
Table-driven avec ~8+ cases, chaque case avec operation func + assertions.
**Refactor proposé**:
- Extraire `assertDiskOperationResult(t *testing.T, deps, operation, wantErr, wantAction, vmid, status, calls)` helper
- Les cases ne gardent que la définition

##### CC 19 — `server/internal/vm/query_test.go:301`
**Refactor proposé**: extraire helpers.

##### CC 18 — `server/internal/httpapi/auth_cluster_test.go:104` — `TestAuth_CrossClusterLogin`
Test login cross-cluster avec assertions inline.
**Refactor proposé**:
- Extraire `assertClusterInIdentity(t *testing.T, identity, wantCluster)` helper

##### CC 18 — `server/internal/cluster/contract_test.go:126`
**Refactor proposé**: extraire helpers.

##### CC 18 — `server/internal/config/config_test.go:27`
**Refactor proposé**: extraire helpers.

##### CC 17 — `server/internal/store/import.go:282` — `replaceTable` (PRODUCTION)
Fonction qui introspect une table upload, lit les colonnes, et fait le replace.
**Refactor proposé**:
- Extraire `copyTableRows(ctx, tx, uploadDB, table, cols)` helper
- Extraire `truncateTable(ctx, tx, table)` helper

##### CC 17 — `server/internal/httpapi/admin_nodes_test.go:196`
**Refactor proposé**: extraire helpers.

##### CC 16 — `server/internal/store/import_test.go:57`
**Refactor proposé**: extraire helpers.

##### CC 16 — `server/internal/store/run_migrations.go:41` — `validateMigrations` (PRODUCTION)
Validation des migrations avec un gros for loop et multiples if/return.
**Refactor proposé**:
- Extraire `validateMigrationVersion(m, i, previous)` qui retourne error
- Extraire `validateMigrationDDL(m)` helper
- La fonction devient un loop simple avec des appels aux helpers

##### CC 16 — `server/internal/cluster/fake.go:443,446` — `applyAction` (PRODUCTION, S1871 aussi)
Voir section D.

##### CC 16 — `server/internal/httpapi/admin_nodes_test.go:196`
Déjà listé.

#### Autres S3776 (CC 15-17, moins prioritaires)
- `server/internal/cluster/contract_test.go` (autres fonctions)
- `server/internal/store/run_migrations.go:41` (déjà couvert ci-dessus)

### Stratégie pour les tests table-driven

Le pattern récurrent dans les tests PVMSS est:

```go
func TestX(t *testing.T) {
    cases := []struct{ ... }{...}
    for _, c := range cases {
        t.Run(c.name, func(t *testing.T) {
            // setup
            // call
            // assert
        })
    }
}
```

Quand les assertions sont complexes (plusieurs check, debuggage 오류), la CC monte. La solution est de **sortir les assertions dans des fonctions helpers nommées** qui prennent `t` et les valeurs pertinentes, et de n'appeler que le helper dans le t.Run.

Exemple pour `store_test.go:62`:
```go
// AVANT
for _, c := range cases {
    t.Run(c.name, func(t *testing.T) {
        // ~30 lignes de setup + assertions
    })
}

// APRÈS
for _, c := range cases {
    t.Run(c.name, func(t *testing.T) {
        runOpenCase(t, c)
    })
}

func runOpenCase(t *testing.T, c testCase) {
    // setup
    // assertions
}
```

---

## B. go:S1192 — Littéraux dupliqués

### Approche générale

Créer des **constantes de paquet** au début de chaque fichier concerné (ou dans un fichier central si pertinent). Sinon, extraire les chaînes littérales dans des consts.

Les helpers d'erreur (`writeAuthError`, `writeAdminError`, `writeDetailError`, `writeClusterError`) prennent déjà un paramètre `message string`, donc les constantes s'utilisent directement comme arguments.

### Fichiers concernés et constantes à extraire

#### `server/internal/httpapi/vm_detail.go`
Chaînes les plus dupliquées:
- `"authentication required"` — 9+ occurrences (dans handleAction, handleDelete, handlePatch, handleDisk, handleCDROM, handleHardware, handleNetwork, handleSnapshots, handleCloudInit)
- `"internal server error"` — 17+ occurrences
- `"method not allowed"` — 6 occurrences
- `"invalid VM path"` — 9 occurrences
- `"inventory has not been populated yet"` — 9 occurrences
- `"invalid request body"` — 7 occurrences
- `"read hardware catalog failed"` — 4 occurrences
- `"not your VM"` — 6 occurrences
- `"VM not found"` — 6 occurrences
- `"cluster rejected the request"` — 3 occurrences

**Solution**: consts au début du fichier:
```go
const (
    msgAuthRequired           = "authentication required"
    msgInternalServerError    = "internal server error"
    msgMethodNotAllowed       = "method not allowed"
    msgInvalidVMPath          = "invalid VM path"
    msgInventoryNotReady      = "inventory has not been populated yet"
    msgInvalidRequestBody     = "invalid request body"
    msgHardwareCatalogFailed  = "read hardware catalog failed"
    msgNotYourVM              = "not your VM"
    msgVMNotFound             = "VM not found"
    msgClusterRejected        = "cluster rejected the request"
    msgPolicyUnavailable      = "policy service is not configured"
    msgReadCatalogFailed      = "read hardware catalog failed"
)
```

Et remplacer chaque `writeDetailError(w, status, code, "literal")` par `writeDetailError(w, status, code, msgXxx)`.

#### `server/internal/httpapi/auth.go`
- `"internal server error"` — 7 occurrences
- `"method not allowed"` — 4 occurrences
- `"invalid credentials"` — 4 occurrences
- `"authentication required"` — 7 occurrences

**Solution**: consts dans auth.go:
```go
const (
    msgAuthRequired       = "authentication required"
    msgInternalServerError = "internal server error"
    msgMethodNotAllowed   = "method not allowed"
    msgInvalidCredentials = "invalid credentials"
)
```

Note: auth.go et vm_detail.go ont tous deux `"authentication required"` et `"internal server error"`. Pour éviter la duplication de constantes entre fichiers, on peut soit:
(a) Définir les consts dans chaque fichier (plus simple, pas de dépendance cross-package)
(b) Centraliser dans `server/internal/httpapi/errmsg.go`

Option (a) est plus rapide et suffisante pour régler le S1192 (la règle ne compare pas entre fichiers).

#### `server/internal/httpapi/admin_catalog.go`
- `"cluster not found"` — 8 occurrences
- `"internal server error"` — 9 occurrences
- `"invalid request body"` — 4 occurrences
- `"\" not reported by the cluster"` — 4 occurrences (via les fonctions nodeNotFoundMsg, storageNotFoundMsg, bridgeNotFoundMsg, isoNotFoundMsg)

**Solution**:
```go
const (
    msgClusterNotFound      = "cluster not found"
    msgInternalServerError  = "internal server error"
    msgInvalidRequestBody   = "invalid request body"
)
```

Les fonctions nodeNotFoundMsg/storageNotFoundMsg/etc. construisent la chaîne avec des noms dynamiques — ce sont des fonctions, pas des littéraux, donc S1192 ne les touchait pas pour le motif `"\" not reported by the cluster"`. Vérifier si la constante doit être extraite au niveau de la fonction ou si les fonctions existantes suffisent.

#### `server/internal/httpapi/vm_create.go`
- `"internal server error"` — 5 occurrences

#### `server/internal/httpapi/admin_pools.go`
- `"admin only"` — 3 occurrences

#### `server/internal/httpapi/admin_profiles.go`
- `"internal server error"` — 3 occurrences

#### `server/internal/httpapi/admin_tags.go`
- `"internal server error"` — 4 occurrences
- `"tag \""` — 3 occurrences

#### `server/internal/httpapi/vm_snapshots.go`
- `"inventory has not been populated yet"` — 3 occurrences

#### `server/internal/httpapi/vm_cloudinit.go`
- `"inventory has not been populated yet"` — 4 occurrences

#### `server/internal/httpapi/vm_detail.go` (autres)
- `"policy service is not configured"` — 3 occurrences (déjà listé plus haut)
- `"cluster rejected the request"` — 3 occurrences (déjà listé)
- `"read hardware catalog failed"` — 4 occurrences (déjà listé)

#### `server/internal/httpapi/vm_detail.go:203,212,218,235,266`
Déjà couvert par les consts ci-dessus.

#### `server/internal/store/clusters.go:344`
- `"pvmss@pve!service"` — 3 occurrences

#### `server/internal/cluster/cloudinit_fake.go:8,11`
- `"ssh-ed25519 AAAA-demo-alice@laptop"` — 3 occurrences
- `"example.internal"` — 3 occurrences

#### `server/internal/vm/actions.go:102`
- `"record audit: %w"` — 3 occurrences

### Fichiers à modifier pour S1192 (liste complète)

1. `server/internal/httpapi/vm_detail.go` — consts + remplacement
2. `server/internal/httpapi/auth.go` — consts + remplacement
3. `server/internal/httpapi/admin_catalog.go` — consts + remplacement
4. `server/internal/httpapi/vm_create.go` — consts + remplacement
5. `server/internal/httpapi/admin_pools.go` — consts + remplacement
6. `server/internal/httpapi/admin_profiles.go` — consts + remplacement
7. `server/internal/httpapi/admin_tags.go` — consts + remplacement
8. `server/internal/httpapi/vm_snapshots.go` — consts + remplacement
9. `server/internal/httpapi/vm_cloudinit.go` — consts + remplacement
10. `server/internal/store/clusters.go` — const
11. `server/internal/cluster/cloudinit_fake.go` — consts
12. `server/internal/vm/actions.go` — const

---

## C. go:S107 — Trop de paramètres

### Approche générale

Regrouper les paramètres dans une **struct de dépendances** avec un nom clair. Le pattern existant dans le codebase est déjà utilisé pour certains cas (`vm.DiskDependencies`, `vm.HardwareDependencies`, `vm.CDROMDependencies`, `vm.CreateDeps`, `vm.BulkDeps`), donc il faut suivre ce pattern.

### Fonctions concernées avec struct proposée

#### `server/internal/vm/cloudinit.go:43` — `SetCloudInitConfig` (11 params)
```go
func SetCloudInitConfig(ctx context.Context, index *inventory.Index, actor auth.Identity, clusterName string, vmid int, update cluster.CloudInitUpdate, rebootNow bool, reader cluster.CloudInitReader, writer cluster.Writer, audit AuditRecorder, refresher IndexRefresher) (bool, error)
```

**Struct proposée**: `CloudInitConfigDeps` (déjà existe en partie? Vérifier)
```go
type CloudInitConfigDeps struct {
    Index       *inventory.Index
    Actor       auth.Identity
    ClusterName string
    VMID        int
    Update      cluster.CloudInitUpdate
    RebootNow   bool
    Reader      cluster.CloudInitReader
    Writer      cluster.Writer
    Audit       AuditRecorder
    Refresher   IndexRefresher
}
```

Call site dans `vm_cloudinit.go:162` — mettre à jour.

#### `server/internal/vm/actions.go:137` — `Patch` (10 params)
```go
func Patch(ctx context.Context, index *inventory.Index, actor auth.Identity, clusterName string, vmid int, name, description string, writer cluster.Writer, audit AuditRecorder, refresher IndexRefresher) error
```

**Note**: `name` et `description` sont des valeurs métier, pas des dépendances. On peut soit:
(a) Les garder comme params séparés ( acceptance: la règle S107 compte TOUS les params)
(b) Les regrouper dans un `PatchRequest` struct

Option (b) est plus propre:
```go
type PatchRequest struct {
    Name        string
    Description string
}

func Patch(ctx context.Context, index *inventory.Index, actor auth.Identity, clusterName string, vmid int, req PatchRequest, writer cluster.Writer, audit AuditRecorder, refresher IndexRefresher) error
```

Call sites: `vm_detail.go:354` (handlePatch).

#### `server/internal/vm/actions.go:112` — `Delete` (8 params)
```go
func Delete(ctx context.Context, index *inventory.Index, actor auth.Identity, clusterName string, vmid int, writer cluster.Writer, audit AuditRecorder, refresher IndexRefresher) error
```

**Note**: `index`, `actor`, `clusterName`, `vmid` sont les 4 params de résolution ( métier). `writer`, `audit`, `refresher` sont les dépendances.
**Struct proposée**: pas de struct car 8 params avec 4 métier + 3 déps + 1 ctx. Créer un `DeleteDeps` regoupant les 3 dernières dépendances et garder les 5 premiers tels quels? Non — S107 compte tous les params après ctx. 

Approche: regrouper les 3 dépendances + le audience/cluster info:
```go
type DeleteDeps struct {
    Index       *inventory.Index
    Actor       auth.Identity
    ClusterName string
    VMID        int
    Writer      cluster.Writer
    Audit       AuditRecorder
    Refresher   IndexRefresher
}
```

Mais ça change beaucoup de call sites. Alternative plus conservatrice: garder l'API actuelle et juste documenter que c'est acceptable (le seuil est 7, la fonction a 8, c'est marginal). Mais l'utilisateur veut résoudre les S107, donc on refactorise.

#### `server/internal/vm/console.go:194` — 8 params
Voir le fichier pour l'identifier précisément.

#### `server/internal/vm/hardware.go:91` — `applyHardware` (8 params)
Déjà a un pattern `HardwareDependencies` + `HardwarePatch` pour les autres fonctions. Vérifier si cette func peut utiliser le même pattern.

#### `server/internal/vm/cloudinit.go:92` — `SetCloudInitSnippet` (10 params)
Déjà a des déps dans `resolveSnippetArtifact`. Regrouper dans un struct.

#### `server/internal/pools/delete.go:29` — `Delete` (9 params)
```go
func Delete(ctx context.Context, actor auth.Identity, client cluster.Client, projection *inventory.Projection, clusterName, name string, writer cluster.Writer, audit vm.AuditRecorder, refresher vm.IndexRefresher) (DeleteResult, error)
```

**Struct proposée**: `PoolDeleteDeps`
```go
type PoolDeleteDeps struct {
    Actor       auth.Identity
    Client      cluster.Client
    Projection  *inventory.Projection
    ClusterName string
    Writer      cluster.Writer
    Audit       vm.AuditRecorder
    Refresher   vm.IndexRefresher
}
```
Et garder `name string` comme paramètre métier séparé.

#### `server/internal/pools/delete.go:65,98` — deux autres fonctions avec 8 params
Voir les fichiers pour identifier.

#### `server/internal/httpapi/vm_cloudinit.go:35` — 8 params
Handler VMCloudInit. Voir le fichier pour l'identifier.

#### `server/internal/httpapi/vm_detail.go:54` — `NewVMDetailWithRegistry` (8 params)
```go
func NewVMDetailWithRegistry(source inventory.LookupSource, projection *inventory.Projection, authHandler *Auth, writer cluster.Writer, st *store.Store, refresher vm.IndexRefresher, log *slog.Logger, services ...*policy.Policy) *VMDetail
```

Déjà a `NewVMDetail` en dessous. Le pattern de regrouper dans `VMDetailConfig` existe probablement.

#### `server/internal/httpapi/admin_clusters_test.go:142` — 8 params (TEST)
Voir le fichier.

#### `server/internal/catalog/profiles_admin.go:97,134` — 8 et 9 params
`CreateProfile` et `UpdateProfile`. Déjà passent `st *store.Store, cluster, label string` + champs profile. Regrouper label+champs dans un `ProfileCreateOrUpdate` struct.

#### `server/internal/store/catalog_admin.go:176,187` — 8 params chacune
Voir le fichier.

### Call sites à mettre à jour (scope)

Chaque refactor de signature va impacter les call sites dans:
- `server/internal/httpapi/*.go` (handlers)
- `server/internal/pools/*.go`
- `server/internal/cluster/*.go` (tests)
- `server/internal/httpapi/*_test.go` (tests)
- eventuellement d'autres fichiers

**Approche incrémentale**: pour chaque fonction, faire le refactor en un seul edit (changer la signature + tous les call sites + les tests) pour éviter un état intermédiaire break.

---

## D. go:S1871 — Branches dupliquées

### `server/internal/cluster/fake.go:443,446`

Le code:
```go
case "reboot":
    state.vms[idx].Status = VMRunning
    state.vms[idx].Uptime = fakeUptimeOnStart
```

est identique à la branche `actionStart` (ligne 437-438):
```go
case actionStart:
    state.vms[idx].Status = VMRunning
    state.vms[idx].Uptime = fakeUptimeOnStart
```

**Correction**: fusionner les deux cas:
```go
case actionStart, "reboot":
    state.vms[idx].Status = VMRunning
    state.vms[idx].Uptime = fakeUptimeOnStart
```

Un seul edit, zéro risque.

---

## Tests et validation

### Comment vérifier après chaque batch

```bash
cd ~/git/gh/pvmss/server && go build ./... && go vet ./... && go test ./... -count=1
```

Pour les changements S3776 (refactor de tests), lancer les tests des fichiers concernés en priorité:
```bash
go test ./internal/cluster/... ./internal/config/... ./internal/httpapi/... ./internal/vm/... ./internal/store/... -count=1 -v -run TestName
```

### Coverage: actions nécessaires pour atteindre 80%

New-code coverage actuelle: 51.5%. Seuil gate: 80%.
Il faut ajouter ~28.5 points de coverage sur le nouveau code.

**Où ajouter des tests**:
1. Après les refactors S107 (struct de dépendances), écrire des tests unitaires sur les nouvelles fonctions helper si elles n'existent pas.
2. Après les refactors S3776, si on extrait des helpers publics/exported, les tester.
3. Les zones de code peu couvertes identifiées par `go test -cover` devraient être ciblées.

**Approche pratique**: après chaque batch de refactor, lancer `go test -coverprofile=coverage.out ./...` et identifier les fichiers avec la coverage la plus basse pour y ajouter des tests.

---

## Plan d'attaque incrémental

### Étape 0 — Pré-requis (15 min)
```bash
cd ~/git/gh/pvmss/server
go build ./... && go vet ./... && go test ./... -count=1 -short
```
Confirmer que tout passe avant de toucher quoi que ce soit.

### Étape 1 — S1192: constantes (1-2h, risque faible)
Modifier les 12 fichiers listés ci-dessus pour extraire les constantes et remplacer les littéraux.
- Fichiers modifiés: auth.go, vm_detail.go, admin_catalog.go, vm_create.go, admin_pools.go, admin_profiles.go, admin_tags.go, vm_snapshots.go, vm_cloudinit.go, clusters.go, cloudinit_fake.go, actions.go
- Vérification: `go build ./... && go vet ./... && go test ./... -count=1 -short`
- Sonar: re-scan ou vérifier manuellement que les S1192 disparaissent

### Étape 2 — S107: struct de dépendances (2-3h, risque modéré)
Refactor des fonctions avec trop de params, un par un, du plus simple au plus complexe:
1. `fake.go:443,446` — S1871 aussi, fusion des case
2. `actions.go:112` — Delete (8→ struct)
3. `actions.go:137` — Patch (10→ struct + PatchRequest)
4. `cloudinit.go:43` — SetCloudInitConfig (11→ struct)
5. `cloudinit.go:92` — SetCloudInitSnippet (10→ struct)
6. `pools/delete.go` — 3 fonctions (29, 65, 98)
7. `catalog/profiles_admin.go` — CreateProfile, UpdateProfile
8. `console.go:194`, `hardware.go:91`
9. `httpapi/vm_cloudinit.go:35`, `httpapi/vm_detail.go:54`
10. `store/catalog_admin.go:176,187`
11. `httpapi/admin_clusters_test.go:142`

Mettre à jour tous les call sites à chaque étape.

### Étape 3 — S3776: refactor de tests (3-5h, risque modéré)
Refactor des tests CC > 15, par fichier, du plus gros au plus petit:
1. `cluster/contract_test.go:21` — CC 52 (extraire 3 helpers)
2. `config/log_test.go:15` — CC 48 (extraire helper validate)
3. `httpapi/health_test.go:25` — CC 45 (extraire helpers)
4. `store/store_test.go:62` — CC 29 (extraire runOpenCase)
5. `vm/create_policy_test.go:15` — CC 23
6. `vm/cdrom_test.go:15` — CC 23
7. `vm/resolve_test.go:36` — CC 23
8. `cluster/fake_create_test.go:15` — CC 23
9. `vm/network_test.go:16` — CC 22
10. `cluster/contract_test.go:315` — CC 22
11. `vm/snapshots_test.go:53` — CC 22
12. `httpapi/inventory.go:36` — CC 22 (PRODUCTION, extraire helpers d'erreur)
13. `httpapi/vm_detail_test.go` — CC 22 (identifier la fonction précise)
14. `policy/policy_test.go:61` — CC 21
15. `httpapi/router.go:63` — CC 19 (PRODUCTION, extraire registerXRoutes)
16. `cluster/rfb_fake_test.go:18` — CC 19
17. `vm/disks_test.go:17` — CC 19
18. `vm/query_test.go:301` — CC 19
19. `httpapi/auth_cluster_test.go:104` — CC 18
20. `cluster/contract_test.go:126` — CC 18
21. `config/config_test.go:27` — CC 18
22. `store/import.go:282` — CC 17 (PRODUCTION)
23. `httpapi/admin_nodes_test.go:196` — CC 17
24. `store/import_test.go:57` — CC 16
25. `store/run_migrations.go:41` — CC 16 (PRODUCTION)
26. `cluster/fake.go:443,446` — CC 16 (PRODUCTION, S1871 aussi)

### Étape 4 — S1871: fake.go (déjà dans l'étape 2/3)
Fusion des 2 branches identiques.

### Étape 5 — Coverage (1-2h)
Après tous les refactors:
```bash
go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out | sort -k3
```
Identifier les fichiers < 80% et écrire des tests ciblés pour remonter la new-code coverage à 80%.

### Étape 6 — Sonar re-scan
```bash
cd ~/git/gh/pvmss && make sonar-scan-server
```
Vérifier que la gate est green: 0 nouvelles issues, coverage ≥ 80%.

---

## Résumé des fichiers modifiés

### S1192 (12 fichiers)
1. `server/internal/httpapi/auth.go`
2. `server/internal/httpapi/vm_detail.go`
3. `server/internal/httpapi/admin_catalog.go`
4. `server/internal/httpapi/vm_create.go`
5. `server/internal/httpapi/admin_pools.go`
6. `server/internal/httpapi/admin_profiles.go`
7. `server/internal/httpapi/admin_tags.go`
8. `server/internal/httpapi/vm_snapshots.go`
9. `server/internal/httpapi/vm_cloudinit.go`
10. `server/internal/store/clusters.go`
11. `server/internal/cluster/cloudinit_fake.go`
12. `server/internal/vm/actions.go`

### S107 (12+ fichiers)
1. `server/internal/vm/cloudinit.go`
2. `server/internal/vm/actions.go`
3. `server/internal/vm/console.go`
4. `server/internal/vm/hardware.go`
5. `server/internal/pools/delete.go`
6. `server/internal/httpapi/vm_cloudinit.go`
7. `server/internal/httpapi/vm_detail.go`
8. `server/internal/httpapi/admin_clusters_test.go`
9. `server/internal/catalog/profiles_admin.go`
10. `server/internal/store/catalog_admin.go`
11. (plus selon les call sites identifiés)

### S3776 (28 fichiers)
Liste complète dans la section A. Les fichiers de test les plus impactés:
- `server/internal/cluster/contract_test.go`
- `server/internal/config/log_test.go`
- `server/internal/httpapi/health_test.go`
- `server/internal/store/store_test.go`
- `server/internal/vm/create_policy_test.go`
- `server/internal/vm/cdrom_test.go`
- `server/internal/vm/resolve_test.go`
- `server/internal/cluster/fake_create_test.go`
- `server/internal/vm/network_test.go`
- `server/internal/vm/snapshots_test.go`
- `server/internal/httpapi/vm_detail_test.go`
- `server/internal/policy/policy_test.go`
- `server/internal/httpapi/auth_cluster_test.go`
- `server/internal/httpapi/admin_nodes_test.go`
- `server/internal/vm/disks_test.go`
- `server/internal/vm/query_test.go`
- `server/internal/config/config_test.go`
- `server/internal/cluster/rfb_fake_test.go`
- `server/internal/store/import_test.go`
- `server/internal/store/import.go`
- `server/internal/store/run_migrations.go`
- `server/internal/cluster/fake.go`
- `server/internal/httpapi/router.go`
- `server/internal/httpapi/inventory.go`

### S1871 (1 fichier)
- `server/internal/cluster/fake.go`
