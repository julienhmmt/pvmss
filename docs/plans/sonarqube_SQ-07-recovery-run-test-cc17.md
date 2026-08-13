# SonarQube SQ-07 — `recovery/run_test.go:17` go:S3776 (Cognitive Complexity 17 → 15)

- **Sévérité** : CRITICAL · **Effort estimé** : 7 min · **Règle** : go:S3776
- **Fonction** : `TestRun_FullSequence_WritesAllTables(t *testing.T)`
- **Gate impact** : compte dans les « 10 New Code issues » → Quality Gate FAILED

## Contexte
Test qui ouvre les 2 DB, seed la legacy, appelle `recovery.Run`, puis vérifie
les compteurs de chaque table (nodes/storages/bridges/isos/profiles/tags/...).
La CC (17) vient des nombreuses assertions `want N rows` répétées. La fonction
porte déjà `//nolint:gocyclo` (non pertinent pour go:S3776).

## Plan
1. Extraire un helper d'assertion de compteur :
   ```go
   func assertRowCount(t *testing.T, db *sql.DB, table string, want int) {
       t.Helper()
       var got int
       if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&got); err != nil {
           t.Fatalf("count %s: %v", table, err)
       }
       if got != want {
           t.Errorf("%s: got %d rows, want %d", table, got, want)
       }
   }
   ```
2. Remplacer les blocs d'assertion par `assertRowCount(t, v04DB, "catalog_nodes", N)`.
   La CC de la fonction principale tombe sous 15.
3. Conserver la boucle de setup (open/seed) telle quelle.

## Vérification
- `cd server && go test ./internal/recovery/ -run TestRun_FullSequence_WritesAllTables -v` (vert).
- `make sonar-scan-server` → l'issue `624c14fb...` (run_test.go:17) disparaît.
