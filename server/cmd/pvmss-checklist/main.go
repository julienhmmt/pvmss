// Command pvmss-checklist mechanically generates the parity checklist
//
//	by walking the fiche directories under.claude/v0.4/{auth,vm,admin,plateforme}/
//
// and cross-referencing each fiche against the table.
//
// Usage:
//
//	pvmss-checklist --repo-root /path/to/pvmss
//
// Exit code is always 0 — this is a report, not a gate.
package main

import (
	"flag"
	"fmt"
	"os"
	"pvmss/server/internal/checklist"
)

func main() {
	repoRoot := flag.String("repo-root", ".", "path to the pvmss repository root")

	flag.Parse()

	if err := checklist.Generate(os.Stdout, *repoRoot); err != nil {
		fmt.Fprintf(os.Stderr, "pvmss-checklist: %v\n", err)
		os.Exit(1)
	}
}
