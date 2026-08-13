package recovery

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"pvmss/server/internal/cluster"
	"strings"
)

// RunOptions configures a recovery run.
type RunOptions struct {
	ClusterName     string
	ProxmoxCreds    ProxmoxCreds // from flags, may be zero-valued
	SessionSecret   string       // for token encryption (SESSION_SECRET)
	DryRun          bool
	Environ         Environ             // defaults to os.Getenv if nil
	StorageResolver StorageNodeResolver // nil = skip storage expansion
}

// Run executes the full recovery sequence (data-model.md "Sequence"):
//
//  1. Map cluster from env/flags → upsert clusters row
//  2. Map enabled_nodes → upsert catalog_nodes
//  3. Map enabled_storages → expand nodes → upsert catalog_storages
//  4. Map enabled_vmbrs → upsert catalog_bridges
//  5. Map enabled_isos → split volids → upsert catalog_isos
//  6. Map vm_profiles → parse JSON → upsert catalog_profiles
//  7. Map tags → assign colors → upsert catalog_tags
//  8. Map vm_limits → upsert vm_limits (5 fields only, SC-002)
//  9. Map node_limits → upsert node_limits
//
// Every step is per-row error tolerant: a single malformed row is skipped
// and named in the summary, never aborting the whole run (plan.md research
// decisions). The only step that touches live Proxmox is step 3's
// storage-node expansion (FR-011).
func Run(ctx context.Context, legacyDB, v04DB *sql.DB, opts RunOptions) (Summary, error) {
	var sum Summary

	env := opts.Environ
	if env == nil {
		env = osEnviron{}
	}

	// Step 1: cluster row
	clusterRow, resolvedCreds, err := MapCluster(env, opts.ClusterName, opts.ProxmoxCreds, opts.SessionSecret)
	if err != nil {
		return sum, err
	}

	sum.Cluster.Written = 1

	if !opts.DryRun {
		if err := upsertCluster(ctx, v04DB, clusterRow); err != nil {
			return sum, fmt.Errorf("write cluster: %w", err)
		}
	}

	// FR-011: when Proxmox credentials are available and the caller didn't
	// already inject a resolver (tests do), wire live storage-node
	// expansion. A Snapshot failure is isolated to per-storage skip
	// reasons (liveStorageResolver), never aborts the run.
	if opts.StorageResolver == nil && resolvedCreds.URL != "" && resolvedCreds.TokenID != "" && resolvedCreds.TokenSecret != "" {
		opts.StorageResolver = newLiveStorageResolver(cluster.Proxmox{
			BaseURL:       resolvedCreds.URL,
			APITokenName:  resolvedCreds.TokenID,
			APITokenValue: resolvedCreds.TokenSecret,
		})
	}

	// Steps 2-9: catalog, profiles, tags, policy
	return runFull(ctx, legacyDB, v04DB, opts, sum)
}

// runFull is the actual orchestration body, separated from Run's initial
// cluster step for readability. It processes each catalog table in
// data-model.md's sequence order and accumulates the summary.
func runFull(ctx context.Context, legacyDB, v04DB *sql.DB, opts RunOptions, sum Summary) (Summary, error) {
	if err := stepNodes(ctx, legacyDB, v04DB, opts, &sum); err != nil {
		return sum, err
	}

	if err := stepStorages(ctx, legacyDB, v04DB, opts, &sum); err != nil {
		return sum, err
	}

	if err := stepBridges(ctx, legacyDB, v04DB, opts, &sum); err != nil {
		return sum, err
	}

	if err := stepISOs(ctx, legacyDB, v04DB, opts, &sum); err != nil {
		return sum, err
	}

	if err := stepProfiles(ctx, legacyDB, v04DB, opts, &sum); err != nil {
		return sum, err
	}

	if err := stepTags(ctx, legacyDB, v04DB, opts, &sum); err != nil {
		return sum, err
	}

	if err := stepVMLimits(ctx, legacyDB, v04DB, opts, &sum); err != nil {
		return sum, err
	}

	if err := stepNodeLimits(ctx, legacyDB, v04DB, opts, &sum); err != nil {
		return sum, err
	}

	return sum, nil
}

// stepNodes maps enabled_nodes → catalog_nodes (data-model.md step 2).
func stepNodes(ctx context.Context, legacyDB, v04DB *sql.DB, opts RunOptions, sum *Summary) error {
	nodeRows, err := mapNodes(ctx, legacyDB)
	if err != nil {
		return err
	}

	sum.CatalogNodes.Read = len(nodeRows)
	for _, r := range nodeRows {
		if !opts.DryRun {
			if err := upsertNode(ctx, v04DB, opts.ClusterName, r); err != nil {
				return err
			}
		}

		sum.CatalogNodes.Written++
	}

	return nil
}

// stepStorages maps enabled_storages → catalog_storages with live node
// expansion (FR-011). Skips are recorded but never abort the run.
func stepStorages(ctx context.Context, legacyDB, v04DB *sql.DB, opts RunOptions, sum *Summary) error {
	storageRows, storageSkips, err := mapStorages(ctx, legacyDB, opts.ClusterName, opts.StorageResolver)
	if err != nil {
		return err
	}

	sum.CatalogStorages.Read = len(storageRows) + len(storageSkips)

	sum.CatalogStorages.Skipped = len(storageSkips)
	for _, sr := range storageSkips {
		sum.CatalogStorages.SkipReasons = append(sum.CatalogStorages.SkipReasons,
			fmt.Sprintf("storage %q: %s", sr.Row, sr.Reason))
	}

	for _, r := range storageRows {
		if !opts.DryRun {
			if err := upsertStorage(ctx, v04DB, opts.ClusterName, r); err != nil {
				return err
			}
		}

		sum.CatalogStorages.Written++
	}

	return nil
}

// stepBridges maps enabled_vmbrs → catalog_bridges (data-model.md step 4).
func stepBridges(ctx context.Context, legacyDB, v04DB *sql.DB, opts RunOptions, sum *Summary) error {
	bridgeRows, err := mapBridges(ctx, legacyDB)
	if err != nil {
		return err
	}

	sum.CatalogBridges.Read = len(bridgeRows)
	for _, r := range bridgeRows {
		if !opts.DryRun {
			if err := upsertBridge(ctx, v04DB, opts.ClusterName, r); err != nil {
				return err
			}
		}

		sum.CatalogBridges.Written++
	}

	return nil
}

// stepISOs maps enabled_isos → catalog_isos with volid split (step 5).
//
//nolint:dupl // sibling step* helpers share this exact map→skip→upsert shape by design
func stepISOs(ctx context.Context, legacyDB, v04DB *sql.DB, opts RunOptions, sum *Summary) error {
	isoRows, isoSkips, err := mapISOs(ctx, legacyDB)
	if err != nil {
		return err
	}

	sum.CatalogISOs.Read = len(isoRows) + len(isoSkips)

	sum.CatalogISOs.Skipped = len(isoSkips)
	for _, sr := range isoSkips {
		sum.CatalogISOs.SkipReasons = append(sum.CatalogISOs.SkipReasons,
			fmt.Sprintf("row %q: %s", sr.Row, sr.Reason))
	}

	for _, r := range isoRows {
		if !opts.DryRun {
			if err := upsertISO(ctx, v04DB, opts.ClusterName, r); err != nil {
				return err
			}
		}

		sum.CatalogISOs.Written++
	}

	return nil
}

// stepProfiles maps vm_profiles → catalog_profiles with JSON parse (step 6).
//
//nolint:dupl // sibling step* helpers share this exact map→skip→upsert shape by design
func stepProfiles(ctx context.Context, legacyDB, v04DB *sql.DB, opts RunOptions, sum *Summary) error {
	profileRows, profileSkips, err := MapProfiles(ctx, legacyDB)
	if err != nil {
		return err
	}

	sum.CatalogProfiles.Read = len(profileRows) + len(profileSkips)

	sum.CatalogProfiles.Skipped = len(profileSkips)
	for _, sr := range profileSkips {
		sum.CatalogProfiles.SkipReasons = append(sum.CatalogProfiles.SkipReasons,
			fmt.Sprintf("profile %q: %s", sr.Row, sr.Reason))
	}

	for _, r := range profileRows {
		if !opts.DryRun {
			if err := upsertProfile(ctx, v04DB, opts.ClusterName, r); err != nil {
				return err
			}
		}

		sum.CatalogProfiles.Written++
	}

	return nil
}

// stepTags maps tags → catalog_tags with the default palette (step 7).
func stepTags(ctx context.Context, legacyDB, v04DB *sql.DB, opts RunOptions, sum *Summary) error {
	tagRows, err := MapTags(ctx, legacyDB)
	if err != nil {
		return err
	}

	sum.CatalogTags.Read = len(tagRows)
	for _, r := range tagRows {
		if !opts.DryRun {
			if err := upsertTag(ctx, v04DB, opts.ClusterName, r); err != nil {
				return err
			}
		}

		sum.CatalogTags.Written++
	}

	return nil
}

// stepVMLimits maps vm_limits → vm_limits (5 fields only, SC-002, step 8).
func stepVMLimits(ctx context.Context, legacyDB, v04DB *sql.DB, opts RunOptions, sum *Summary) error {
	vmLimits, err := mapVMLimits(ctx, legacyDB)
	if err != nil {
		return err
	}

	sum.VMLimits.Read = 1
	sum.VMLimits.Note = "max_disk_per_vm_gb, max_network_cards, max_snapshots, max_vm_per_user, allow_custom_yaml — max_sockets/max_cores/max_memory_mb left at shipped defaults, no legacy source"

	if !opts.DryRun {
		if err := upsertVMLimits(ctx, v04DB, opts.ClusterName, vmLimits); err != nil {
			return err
		}
	}

	sum.VMLimits.Written = 1

	return nil
}

// stepNodeLimits maps node_limits → node_limits (data-model.md step 9).
func stepNodeLimits(ctx context.Context, legacyDB, v04DB *sql.DB, opts RunOptions, sum *Summary) error {
	nodeLimitRows, err := mapNodeLimits(ctx, legacyDB)
	if err != nil {
		return err
	}

	sum.NodeLimits.Read = len(nodeLimitRows)
	for _, r := range nodeLimitRows {
		if !opts.DryRun {
			if err := upsertNodeLimits(ctx, v04DB, opts.ClusterName, r); err != nil {
				return err
			}
		}

		sum.NodeLimits.Written++
	}

	return nil
}

// RenderSummary produces the human-readable stdout output per
// contracts/cutover.md's exact output shape.
func RenderSummary(sum Summary, legacyPath, v04Path, clusterName string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "pvmss-recover: legacy=%s v0.4=%s cluster=%s\n\n", legacyPath, v04Path, clusterName)

	fmt.Fprintf(&b, "clusters        %d written  (from PROXMOX_URL / PROXMOX_API_TOKEN_NAME / PROXMOX_API_TOKEN_VALUE)\n", sum.Cluster.Written)
	renderTableResult(&b, "catalog_nodes", sum.CatalogNodes)
	renderTableResult(&b, "catalog_storages", sum.CatalogStorages)
	renderTableResult(&b, "catalog_bridges", sum.CatalogBridges)
	renderTableResult(&b, "catalog_isos", sum.CatalogISOs)
	renderTableResult(&b, "catalog_profiles", sum.CatalogProfiles)
	renderTableResult(&b, "catalog_tags", sum.CatalogTags)

	if sum.VMLimits.Note != "" {
		fmt.Fprintf(&b, "vm_limits       %d row updated (%s)\n", sum.VMLimits.Written, sum.VMLimits.Note)
	} else {
		fmt.Fprintf(&b, "vm_limits       %d read,  %d written,  %d skipped\n", sum.VMLimits.Read, sum.VMLimits.Written, sum.VMLimits.Skipped)
	}

	fmt.Fprintf(&b, "node_limits     %d read,  %d written,  %d skipped\n", sum.NodeLimits.Read, sum.NodeLimits.Written, sum.NodeLimits.Skipped)

	totalWritten := sum.Cluster.Written + sum.CatalogNodes.Written + sum.CatalogStorages.Written +
		sum.CatalogBridges.Written + sum.CatalogISOs.Written + sum.CatalogProfiles.Written +
		sum.CatalogTags.Written + sum.VMLimits.Written + sum.NodeLimits.Written
	totalSkipped := sum.CatalogStorages.Skipped + sum.CatalogISOs.Skipped + sum.CatalogProfiles.Skipped
	fmt.Fprintf(&b, "\nSUMMARY: written=%d skipped=%d errors=0\n", totalWritten, totalSkipped)

	return b.String()
}

func renderTableResult(b *strings.Builder, name string, tr TableResult) {
	fmt.Fprintf(b, "%-16s %d read,  %d written,  %d skipped", name, tr.Read, tr.Written, tr.Skipped)

	if len(tr.SkipReasons) > 0 {
		fmt.Fprintf(b, "  (%s)", strings.Join(tr.SkipReasons, "; "))
	}

	if tr.Note != "" {
		fmt.Fprintf(b, "  (%s)", tr.Note)
	}

	fmt.Fprintln(b)
}

// getEnv is a package-level function so tests can override it via
// linker injection if needed. By default it calls os.Getenv.
var getEnv = os.Getenv
