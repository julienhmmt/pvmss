# SonarQube — Plans des 10 issues du New Code (pvmss-server v0.4.0)

Source : `http://localhost:9000/dashboard?id=pvmss-server` — Quality Gate
**FAILED** sur le New Code (depuis le 12 août). Les 2 conditions en échec :

- **10 issues** sur le nouveau code (seuil : 0)
- **Couverture 8,0 %** sur le nouveau code (seuil : ≥ 80,0 %)

> ⚠️ Les plans ci-dessous couvrent les **10 issues SonarQube**. Ils ne résolvent
> PAS le critère de couverture (8 % → 80 %). Voir la section « Couverture » en bas.

## Les 10 issues (par sévérité)

| ID | Fichier : ligne | Règle | Sév. | Effort | Plan |
|----|----------------|-------|------|--------|------|
| SQ-01 | `recovery/run.go:81` (`runFull`) | go:S3776 CC 56→15 | CRIT | 46 min | [plan](sonarqube_SQ-01-recovery-runfull-cc56.md) |
| SQ-02 | `httpapi/health_test.go:198` | go:S3776 CC 28→15 | CRIT | 18 min | [plan](sonarqube_SQ-02-health-test-clusters-cc28.md) |
| SQ-03 | `recovery/fixture_test.go:106` (`seedLegacyDB`) | go:S3776 CC 27→15 | CRIT | 17 min | [plan](sonarqube_SQ-03-recovery-fixture-seed-cc27.md) |
| SQ-04 | `checklist/checklist.go:203` (`walkFiches`) | go:S3776 CC 22→15 | CRIT | 12 min | [plan](sonarqube_SQ-04-checklist-walkfiches-cc22.md) |
| SQ-05 | `checklist/checklist_test.go:86` | go:S3776 CC 21→15 | CRIT | 11 min | [plan](sonarqube_SQ-05-checklist-test-cc21.md) |
| SQ-06 | `recovery/recovery_test.go:75` (`TestRecoveryCLI_EndToEnd`) | go:S3776 CC 16→15 | CRIT | 6 min | [plan](sonarqube_SQ-06-recovery-e2e-test-cc16.md) |
| SQ-07 | `recovery/run_test.go:17` (`TestRun_FullSequence_WritesAllTables`) | go:S3776 CC 17→15 | CRIT | 7 min | [plan](sonarqube_SQ-07-recovery-run-test-cc17.md) |
| SQ-08 | `httpapi/admin_cloudinit_templates.go:45` | go:S1192 (littéral ×3) | CRIT | 6 min | [plan](sonarqube_SQ-08-admin-cloudinit-s1192.md) |
| SQ-09 | `vm/create.go:145` (`Create`) | go:S107 (10 params) | MAJOR | 20 min | [plan](sonarqube_SQ-09-vm-create-s107.md) |
| SQ-10 | `vm/bulk.go:90` (`BulkAction`) | go:S107 (8 params) | MAJOR | 20 min | [plan](sonarqube_SQ-10-vm-bulk-s107.md) |

## Répartition
- **8 × go:S3776** (Cognitive Complexity) — tous des extractions de fonctions /
  helpers ; comportement inchangé, tests doivent rester verts.
- **1 × go:S1192** — constante de package pour les messages d'erreur dupliqués.
- **2 × go:S107** (trop de paramètres) — struct de dépendances +
  mise à jour des call sites (`vm.Create` / `vm.BulkAction`).

## Ordre d'exécution suggéré
1. SQ-08 (6 min, trivial — constante) et SQ-06/SQ-07 (6-7 min, tests) d'abord :
   gains rapides, risque nul.
2. SQ-03, SQ-04, SQ-05 (helpers d'extraction, ~11-17 min).
3. SQ-02 (helper de cas de test, 18 min).
4. SQ-09, SQ-10 (struct de deps + grep des call sites, 20 min chacun).
5. **SQ-01 en dernier** (`runFull` CC 56) — le plus gros morceau (46 min),
   extraire une fonction par étape.

## Couverture (séparé des 10 issues)
Le gate échoue aussi sur **8,0 % de couverture < 80 %** (688 lignes à couvrir
sur le nouveau code). Les 10 plans ci-dessus ne le corrigent pas. Pour le gate
vert il faut **en plus** ajouter des tests sur le nouveau code jusqu'à ~80 %.
À traiter dans un plan distinct (ex. `sonarqube_newcode-coverage.md`).

## Vérification globale
```bash
cd server && go test ./... && go vet ./... && cd .. && make sonar-scan-server
```
Puis re-checker `http://localhost:9000/dashboard?id=pvmss-server` → Quality Gate
doit passer à **Passed** une fois les 10 issues + la couverture résolues.
