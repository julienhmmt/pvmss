// Command pvmss-recover is the one-time v0.3 → v0.4 data migration tool
// (T16 — Bascule). It reads a legacy SQLite database and upserts
// admin-configured facts into a fresh, already-migrated v0.4 database.
//
// Usage:
//
//	pvmss-recover --legacy-db /path/to/legacy.db --v0.4-db /path/to/v04.db --cluster-name default
//
// Proxmox credentials (--proxmox-url, --proxmox-token-id, --proxmox-token-secret,
// or their PROXMOX_URL / PROXMOX_API_TOKEN_NAME / PROXMOX_API_TOKEN_VALUE env
// equivalents) populate the clusters row and, when all three are present,
// opportunistically enable live storage-node expansion via one
// cluster.Client.Snapshot call (FR-011). Without credentials, or if that
// call fails, storage catalog entries are skipped with a named reason and
// must be populated separately after cutover via the admin UI or API.
//
// See specs/017-t16-bascule/contracts/cutover.md for the full flag reference.
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"os"
	"pvmss/server/internal/recovery"

	_ "modernc.org/sqlite" // pure-Go SQLite driver
)

func main() {
	os.Exit(run())
}

func run() int {
	var (
		legacyPath    = flag.String("legacy-db", "", "path to the legacy v0.3 SQLite file (read-only)")
		v04Path       = flag.String("v0.4-db", "", "path to the v0.4 SQLite file (must be already migrated)")
		clusterName   = flag.String("cluster-name", "", "name for the clusters row and every migrated row [a-z0-9-]+")
		proxmoxURL    = flag.String("proxmox-url", "", "override PROXMOX_URL")
		proxmoxToken  = flag.String("proxmox-token-id", "", "override PROXMOX_API_TOKEN_NAME")
		proxmoxSecret = flag.String("proxmox-token-secret", "", "override PROXMOX_API_TOKEN_VALUE")
		sessionSecret = flag.String("session-secret", "", "override SESSION_SECRET (used for token encryption)")
		dryRun        = flag.Bool("dry-run", false, "perform every read and mapping, print the summary, write nothing")
	)

	flag.Parse()

	if *legacyPath == "" || *v04Path == "" || *clusterName == "" {
		fmt.Fprintln(os.Stderr, "pvmss-recover: --legacy-db, --v0.4-db, and --cluster-name are required")
		flag.Usage()

		return 1
	}

	if err := recovery.ValidateClusterName(*clusterName); err != nil {
		fmt.Fprintf(os.Stderr, "pvmss-recover: %v\n", err)
		return 2
	}

	legacyDB, err := openSQLite(*legacyPath, true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pvmss-recover: cannot open legacy db: %v\n", err)
		return 1
	}
	defer func() { _ = legacyDB.Close() }()

	v04DB, err := openSQLite(*v04Path, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pvmss-recover: cannot open v0.4 db: %v\n", err)
		return 1
	}
	defer func() { _ = v04DB.Close() }()

	creds := recovery.ProxmoxCreds{
		URL:         *proxmoxURL,
		TokenID:     *proxmoxToken,
		TokenSecret: *proxmoxSecret,
	}

	secret := *sessionSecret
	if secret == "" {
		secret = os.Getenv("SESSION_SECRET")
	}

	opts := recovery.RunOptions{
		ClusterName:   *clusterName,
		ProxmoxCreds:  creds,
		SessionSecret: secret,
		DryRun:        *dryRun,
	}

	sum, err := recovery.Run(context.Background(), legacyDB, v04DB, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pvmss-recover: %v\n", err)
		return 1
	}

	fmt.Print(recovery.RenderSummary(sum, *legacyPath, *v04Path, *clusterName))

	return 0
}

// openSQLite opens a SQLite database file. When readOnly is true, the
// connection is opened in read-only mode — the recovery tool never writes
// to the legacy database.
func openSQLite(path string, readOnly bool) (*sql.DB, error) {
	if path == "" {
		return nil, errors.New("empty database path")
	}

	dsn := "file:" + path
	if readOnly {
		dsn += "?mode=ro"
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open %q: %w", path, err)
	}

	if err := db.PingContext(context.Background()); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping %q: %w", path, err)
	}

	return db, nil
}
