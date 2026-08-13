# SonarQube SQ-06 — `recovery/recovery_test.go:75` go:S3776 (Cognitive Complexity 16 → 15)

- **Sévérité** : CRITICAL · **Effort estimé** : 6 min · **Règle** : go:S3776
- **Fonction** : `TestRecoveryCLI_EndToEnd(t *testing.T)`
- **Gate impact** : compte dans les « 10 New Code issues » → Quality Gate FAILED

## Contexte
Test e2e qui (1) build les binaires `pvmss-recover`/`pvmss-checklist`, (2) lance
`pvmss-checklist` et vérifie le golden SC-004, (3) seed une legacy DB, (4) lance
`pvmss-recover` et vérifie le résultat. La CC (16) est juste au-dessus du seuil
(15) — un petit refactoring suffit.

## Plan
1. Extraire la partie checklist golden :
   ```go
   func runChecklistGolden(t *testing.T, bin, repoRoot string) {
       t.Helper()
       out, err := exec.CommandContext(...).CombinedOutput()
       if err != nil { t.Fatalf(...) }
       if !strings.Contains(string(out), "58 fiches found") { t.Errorf(...) }
       if !strings.Contains(string(out), "SUMMARY: 47 closed, 11 open (...)") { t.Errorf(...) }
   }
   ```
2. Extraire la partie recover golden (seed legacy, migrate v04, run, assert).
3. `TestRecoveryCLI_EndToEnd` ne garde que l'orchestration des deux helpers.
   La CC tombe à ~4.
4. Ne rien changer au comportement ni aux chaînes golden (SC-004).

## Vérification
- `cd server && go test ./internal/recovery/ -run TestRecoveryCLI_EndToEnd -v` (vert).
- `make sonar-scan-server` → l'issue `0afea018...` (recovery_test.go:75) disparaît.
