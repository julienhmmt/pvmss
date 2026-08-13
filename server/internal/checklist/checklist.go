// Package checklist mechanically generates the T16 parity checklist by
// walking the fiche directories under .claude/v0.4/{auth,vm,admin,plateforme}/
// and cross-referencing each fiche ID against spec.md's FR-006 table.
//
// The tool reads no database and has no dependency on tranche completion —
// it only reads fiche filenames and the hardcoded FR-006 mapping. Its
// conclusions about legacy removal readiness are only valid once every
// tranche it names is actually merged (spec Edge Cases).
//
//nolint:goconst // tranche labels and gap/deliberate strings are data, not constants
package checklist

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ficheIDRe extracts the fiche ID (e.g. "A01", "V27") from a filename.
var ficheIDRe = regexp.MustCompile(`^([AVXP]\d{2})-`)

// ficheDir maps one top-level fiche directory to its prefix and display name.
type ficheDir struct {
	dir     string
	prefix  string
	display string
}

// ficheDirs maps each top-level directory to its fiche prefix and display name.
var ficheDirs = []ficheDir{
	{"auth", "A", "auth"},
	{"vm", "V", "vm"},
	{"admin", "X", "admin"},
	{"plateforme", "P", "plateforme"},
}

// FicheEntry is one row in the generated checklist.
type FicheEntry struct {
	ID       string
	Label    string
	Tranche  string
	IsNone   bool
	NoneType string // "gap" or "deliberate" — only set when IsNone
}

// fr006Table is spec.md's FR-006 fiche→tranche mapping, hardcoded as the
// lookup this tool joins against (T026). 55 mapped + 6 "none"
// (3 real gaps: X13/P01/P02; 2 deliberate: X12/X18).
// Update this table, not the tool's logic, when a gap's disposition changes.
var fr006Table = map[string]struct {
	tranche  string
	label    string
	noneType string // "" = not none, "gap" = real gap, "deliberate" = design decision
}{
	// Auth (A01-A06) — all T02
	"A01": {"T02", "Se connecter avec un compte Proxmox", ""},
	"A02": {"T02", "Se connecter en admin local", ""},
	"A03": {"T02", "Se connecter en admin Proxmox", ""},
	"A04": {"T02", "Se déconnecter", ""},
	"A05": {"T02", "Rafraîchir la session", ""},
	"A06": {"T02", "Changer le mot de passe", ""},

	// VM (V01-V27) — V13 is a real gap
	"V01": {"T04", "Tableau de bord — mes VMs", ""},
	"V02": {"T04", "Rechercher une VM par nom", ""},
	"V03": {"T04", "Rechercher une VM par tag", ""},
	"V04": {"T04", "Rechercher une VM par VMID", ""},
	"V05": {"T04", "Filtrer les VMs", ""},
	"V06": {"T04", "Trier et paginer les VMs", ""},
	"V07": {"T04", "Consulter le quota", ""},
	"V08": {"T06", "Créer une VM — mode simple", ""},
	"V09": {"T06", "Créer une VM — formulaire détaillé", ""},
	"V10": {"T06", "Brouillon de création de VM", ""},
	"V11": {"T06", "Suivi des tâches", ""},
	"V12": {"T05", "Démarrer / arrêter / redémarrer", ""},
	"V13": {"T17", "Actions groupées", ""},
	"V14": {"T05", "Supprimer une VM", ""},
	"V15": {"T05", "Consulter le détail d'une VM", ""},
	"V16": {"T05", "Renommer une VM", ""},
	"V17": {"T05", "Modifier la description d'une VM", ""},
	"V18": {"T07", "Modifier le matériel d'une VM", ""},
	"V19": {"T07", "Ajouter un disque", ""},
	"V20": {"T07", "Redimensionner un disque", ""},
	"V21": {"T07", "Supprimer un disque", ""},
	"V22": {"T07", "Gérer le CD-ROM / ISO", ""},
	"V23": {"T05", "Consulter le réseau d'une VM", ""},
	"V24": {"T08", "Modifier le cloud-init", ""},
	"V25": {"T08", "Éditer un snippet cloud-init", ""},
	"V26": {"T09", "Snapshots", ""},
	"V27": {"T10", "Console VNC", ""},

	// Admin (X01-X19) — X11, X12, X13, X18 are none
	"X01": {"T11", "Tableau de bord admin", ""},
	"X02": {"T11", "Gérer les nœuds", ""},
	"X03": {"T11", "Gérer les stockages", ""},
	"X04": {"T11", "Gérer les bridges vmbr", ""},
	"X05": {"T11", "Gérer les ISOs", ""},
	"X06": {"T11", "Gérer les tags", ""},
	"X07": {"T13", "Gérer les pools utilisateurs", ""},
	"X08": {"T12", "Gérer les limites VM", ""},
	"X09": {"T12", "Gérer les limites par nœud", ""},
	"X10": {"T11", "Gérer les profils VM", ""},
	"X11": {"T18", "Modèles cloud-init", ""},
	"X12": {"", "Configurer SFTP cloud-init", "deliberate"},
	"X13": {"", "Administer toutes les VMs", "gap"},
	"X14": {"T05", "Journal d'audit", ""},
	"X15": {"T14", "Exporter la base", ""},
	"X16": {"T14", "Importer la base", ""},
	"X17": {"T14", "Infos application", ""},
	"X18": {"", "Panneau de paramètres unifié", "deliberate"},
	"X19": {"T15", "Gérer les clusters", ""},

	// Plateforme (P01-P06) — all none (real gaps)
	"P01": {"", "Assistant d'installation", "gap"},
	"P02": {"", "Consulter la documentation", "gap"},
	"P03": {"T19", "Changer la langue", ""},
	"P04": {"T19", "Changer le thème", ""},
	"P05": {"T19", "État du service / mode hors ligne", ""},
	"P06": {"T19", "Page d'accueil publique", ""},
}

// Generate walks the fiche directories under repoRoot/.claude/v0.4/ and
// produces the parity checklist. The output is written to w.
func Generate(w io.Writer, repoRoot string) error {
	entries, err := walkFiches(repoRoot)
	if err != nil {
		return err
	}

	// Sort by ID
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].ID < entries[j].ID
	})

	// Count per-directory
	counts := make(map[string]int)

	for _, e := range entries {
		dir := ficheDirForID(e.ID)
		counts[dir]++
	}

	// Build output in a buffer to avoid per-write error checks.
	var b strings.Builder

	// Header
	fmt.Fprintf(&b, "pvmss-checklist: %d fiches found", len(entries))

	if len(counts) > 0 {
		parts := make([]string, 0, len(ficheDirs))
		for _, fd := range ficheDirs {
			if c := counts[fd.display]; c > 0 {
				parts = append(parts, fmt.Sprintf("%s=%d", fd.display, c))
			}
		}

		fmt.Fprintf(&b, " (%s)", strings.Join(parts, " "))
	}

	fmt.Fprintln(&b)
	fmt.Fprintln(&b)

	// Rows
	closed := 0
	openGaps := 0
	openDeliberate := 0

	for _, e := range entries {
		if e.IsNone {
			switch e.NoneType {
			case "gap":
				fmt.Fprintf(&b, "%s  %-45s → NONE (real gap — see spec.md FR-006)\n", e.ID, e.Label)

				openGaps++
			case "deliberate":
				fmt.Fprintf(&b, "%s  %-45s → NONE (deliberate — superseded by design, not a gap)\n", e.ID, e.Label)

				openDeliberate++
			default:
				fmt.Fprintf(&b, "%s  %-45s → NONE\n", e.ID, e.Label)

				openGaps++
			}
		} else {
			fmt.Fprintf(&b, "%s  %-45s → %s\n", e.ID, e.Label, e.Tranche)

			closed++
		}
	}

	// Summary line
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "SUMMARY: %d closed, %d open (%d real gaps, %d deliberate design decisions)\n",
		closed, openGaps+openDeliberate, openGaps, openDeliberate)

	_, err = w.Write([]byte(b.String()))

	return err
}

// walkFiches walks the four fiche subdirectories and returns entries
// cross-referenced against the FR-006 table.
func walkFiches(repoRoot string) ([]FicheEntry, error) {
	baseDir := filepath.Join(repoRoot, ".claude", "v0.4")

	var entries []FicheEntry

	for _, fd := range ficheDirs {
		es, err := collectFicheEntries(filepath.Join(baseDir, fd.dir), fd)
		if err != nil {
			return nil, err
		}

		entries = append(entries, es...)
	}

	return entries, nil
}

// collectFicheEntries reads one fiche directory, filters .md files whose name
// starts with a fiche ID, and builds FicheEntry rows cross-referenced against
// the FR-006 table. A missing directory (os.IsNotExist) yields nil, nil.
func collectFicheEntries(dirPath string, _ ficheDir) ([]FicheEntry, error) {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("read fiche dir %q: %w", dirPath, err)
	}

	var entries []FicheEntry

	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") {
			continue
		}

		match := ficheIDRe.FindStringSubmatch(f.Name())
		if match == nil {
			continue
		}

		id := match[1]

		info, ok := fr006Table[id]
		if !ok {
			// Fiche file exists but not in FR-006 table — report as unknown
			entries = append(entries, FicheEntry{
				ID:    id,
				Label: labelFromFilename(f.Name()),
			})

			continue
		}

		entry := FicheEntry{
			ID:    id,
			Label: info.label,
		}
		if info.noneType != "" {
			entry.IsNone = true
			entry.NoneType = info.noneType
		} else {
			entry.Tranche = info.tranche
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

// labelFromFilename extracts a human-readable label from a fiche filename
// by stripping the ID prefix and file extension, replacing hyphens with spaces.
func labelFromFilename(filename string) string {
	base := strings.TrimSuffix(filename, ".md")
	if idx := strings.Index(base, "-"); idx >= 0 {
		base = base[idx+1:]
	}

	return strings.ReplaceAll(base, "-", " ")
}

// ficheDirForID returns the display name of the directory a fiche belongs to.
func ficheDirForID(id string) string {
	if len(id) == 0 {
		return ""
	}

	for _, fd := range ficheDirs {
		if string(id[0]) == fd.prefix {
			return fd.display
		}
	}

	return ""
}
