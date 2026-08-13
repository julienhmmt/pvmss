# SonarQube SQ-02 — `httpapi/health_test.go:198` go:S3776 (Cognitive Complexity 28 → 15)

- **Sévérité** : CRITICAL · **Effort estimé** : 18 min · **Règle** : go:S3776
- **Fonction** : `TestHealth_ClustersAggregate(t *testing.T)` (table-driven, ~20 cas)
- **Gate impact** : compte dans les « 10 New Code issues » → Quality Gate FAILED

## Contexte
Test table-driven avec une grosse `cases := []struct{...}` (~20 entrées) et une
boucle `t.Run`. La CC (28) vient du nombre de champs du struct de cas + des
assertions répétées. La fonction porte `//nolint:paralleltest` (serial, fixture
partagée) mais pas de nolint gocyclo — et go:S3776 (Cognitive) n'est pas
couvert par gocyclo de toute façon.

## Plan
1. Extraire un helper d'exécution + assertion d'un cas :
   ```go
   func runClustersCase(t *testing.T, tc clustersCase) {
       t.Helper()
       // construit la requête, appelle le handler, décode la réponse,
       // assert wantStatus / wantClustersStat / wantClustersDtl / wantDemoMode
   }
   ```
2. Réduire `TestHealth_ClustersAggregate` à :
   ```go
   func TestHealth_ClustersAggregate(t *testing.T) {
       //nolint:paralleltest // serial: shared health fixture
       for _, tc := range cases {
           t.Run(tc.name, func(t *testing.T) { runClustersCase(t, tc) })
       }
   }
   ```
   La boucle + helper ramène la CC de la fonction principale sous 15.
3. Garder le struct `cases` tel quel (ses champs ne comptent pas pour la CC de
   la fonction une fois l'exécution externalisée).

## Vérification
- `cd server && go test ./internal/httpapi/ -run TestHealth_ClustersAggregate -v`
  (vert, comportement identique).
- `make sonar-scan-server` → l'issue `6e023405...` (health_test.go:198) disparaît.
