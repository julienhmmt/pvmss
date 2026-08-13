# SonarQube SQ-04 — `checklist/checklist.go:203` go:S3776 (Cognitive Complexity 22 → 15)

- **Sévérité** : CRITICAL · **Effort estimé** : 12 min · **Règle** : go:S3776
- **Fonction** : `walkFiches(repoRoot string) ([]FicheEntry, error)`
- **Gate impact** : compte dans les « 10 New Code issues » → Quality Gate FAILED

## Contexte
`walkFiches` itère sur `ficheDirs`, lit chaque répertoire, filtre les `.md`,
matche une regex d'ID, puis gère 3 cas (match FR-006 / inconnu / erreur de
lecture) avec des branches imbriquées qui font monter la CC à 22.

## Plan
1. Extraire la logique par répertoire dans :
   ```go
   func collectFicheEntries(dirPath string, fd ficheDir) ([]FicheEntry, error) {
       // ReadDir + boucle de filtrage + match regex + construction entry
       // retourne les entries pour UN seul fd (gère os.IsNotExist → nil)
   }
   ```
2. `walkFiches` devient :
   ```go
   for _, fd := range ficheDirs {
       es, err := collectFicheEntries(filepath.Join(repoRoot, ".claude", "v0.4", fd.dir), fd)
       if err != nil {
           return nil, err
       }
       entries = append(entries, es...)
   }
   ```
3. Si le `switch` sur `e.NoneType` (gap/deliberate) dans la fonction appelante
   (`renderChecklist` ?) dépasse aussi le seuil, l'extraire dans
   `appendFicheRow(&b, e, &counts)`.

## Vérification
- `cd server && go test ./internal/checklist/...` (vert).
- `make sonar-scan-server` → l'issue `fe7c29d0...` (checklist.go:203) disparaît.
- Vérifier que la sortie `pvmss-checklist --repo-root .` reste identique
  (golden test SC-004 non régression).
