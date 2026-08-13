# SonarQube SQ-05 — `checklist/checklist_test.go:86` go:S3776 (Cognitive Complexity 21 → 15)

- **Sévérité** : CRITICAL · **Effort estimé** : 11 min · **Règle** : go:S3776
- **Fonction** : test table-driven autour de la ligne 86 (vérification du rendu checklist)
- **Gate impact** : compte dans les « 10 New Code issues » → Quality Gate FAILED

## Contexte
Test table-driven avec plusieurs cas et assertions répétées sur la sortie
rendue (`strings.Contains` / comparaisons de `SUMMARY:`). La CC (21) vient du
nombre de cas + assertions inline.

## Plan
1. Identifier la fonction de test à la ligne 86 (ouvrir `checklist`-style).
2. Extraire un helper d'assertion du rendu :
   ```go
   func assertChecklistRender(t *testing.T, got, wantSummary string, wantContains ...string) {
       t.Helper()
       for _, w := range wantContains {
           if !strings.Contains(got, w) {
               t.Errorf("render missing %q:\n%s", w, got)
           }
       }
       if !strings.Contains(got, wantSummary) {
           t.Errorf("SUMMARY mismatch:\n%s", got)
       }
   }
   ```
3. Réduire la boucle `t.Run` à un appel à `assertChecklistRender`.
4. Garder le tableau de cas tel quel (ses champs ne comptent pas pour la CC une
   fois l'assertion externalisée).

## Vérification
- `cd server && go test ./internal/checklist/ -run <TestName> -v` (vert).
- `make sonar-scan-server` → l'issue `df951d3b...` (checklist_test.go:86) disparaît.
