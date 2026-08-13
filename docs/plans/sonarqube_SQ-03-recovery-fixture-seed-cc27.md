# SonarQube SQ-03 — `recovery/fixture_test.go:106` go:S3776 (Cognitive Complexity 27 → 15)

- **Sévérité** : CRITICAL · **Effort estimé** : 17 min · **Règle** : go:S3776
- **Fonction** : `seedLegacyDB(t *testing.T, db *sql.DB, seed legacySeed)`
- **Gate impact** : compte dans les « 10 New Code issues » → Quality Gate FAILED

## Contexte
`seedLegacyDB` insère dans ~15 tables legacy en répétant le motif
`db.ExecContext(ctx, query, args...)` + `if err != nil { t.Fatalf(...) }`.
La fonction porte déjà `//nolint:gocyclo`, mais go:S3776 (Cognitive) n'est pas
couvert → alerte Sonar.

## Plan
1. Extraire un helper d'insertion avec `t.Helper()` :
   ```go
   func mustExec(t *testing.T, db *sql.DB, query string, args ...any) {
       t.Helper()
       if _, err := db.ExecContext(context.Background(), query, args...); err != nil {
           t.Fatalf("seed: %v", err)
       }
   }
   ```
2. Remplacer chaque bloc répété par `mustExec(t, db, "...", n.name, n.enabled)`.
   Cela supprime ~15 branches `if err` → CC bien sous 15.
3. Conserver la boucle par table (elles ne contribuent qu'à 1 de CC chacune une
   fois le `if err` extracté).
4. Ne pas toucher à `legacySchemaDDL` ni à `openLegacyDB`.

## Vérification
- `cd server && go test ./internal/recovery/...` (vert — seed identique).
- `make sonar-scan-server` → l'issue `46fd8d2f...` (fixture_test.go:106) disparaît.
