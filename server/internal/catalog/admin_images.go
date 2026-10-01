package catalog

import (
	"context"
	"database/sql"
	"errors"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/store"
	"slices"
)

// ImageApproval is one discovered cloud image with its admin approval state.
// Missing is true for a stored approval whose image file Proxmox no longer
// reports (see NodeApproval for the enabled-orphan auto-remove rule).
type ImageApproval struct {
	Storage   string
	Node      string
	File      string
	SizeBytes int64
	Enabled   bool
	Missing   bool
}

// AdminListImages returns every cloud image the cluster reports, unioned with
// its stored approval state keyed by (node, storage, file). Orphan approvals
// (image file gone from Proxmox) are handled as in AdminListISOs: enabled
// orphans are auto-removed, disabled orphans are surfaced with Missing=true.
func AdminListImages(ctx context.Context, st *store.Store, client cluster.Client, clusterName string) ([]ImageApproval, error) {
	discovered, err := client.ListCloudImages(ctx)
	if err != nil {
		return nil, err
	}

	return adminListFiles(ctx, st, clusterName, fileListing[store.CatalogImageEnabled, ImageApproval]{
		discovered:   imageFileEntries(discovered),
		listRows:     st.CatalogImagesEnabled,
		toOrphanRow:  imageOrphanRow,
		removeOrphan: removeImageOrphan,
		viewOf:       imageApprovalView,
		missing:      missingImageApproval,
	})
}

// ImageRef identifies one discovered cloud image by its (node, storage, file)
// triple - the same key the enabled-state store and discovery check use.
type ImageRef struct {
	Node    string
	Storage string
	File    string
}

// SetImageEnabled upserts the enabled state for one cloud image identified by
// ref. See SetISOEnabled for the discovery-error contract.
func SetImageEnabled(ctx context.Context, st *store.Store, client cluster.Client, clusterName string, ref ImageRef, enabled bool) error {
	discovered, err := imageDiscovered(ctx, client, ref.Node, ref.Storage, ref.File)
	if err != nil {
		return err
	}

	if !discovered {
		return cluster.ErrNotFound
	}

	sizeBytes := int64(0)

	images, err := client.ListCloudImages(ctx)
	if err != nil {
		return err
	}

	for _, image := range images {
		if image.Node == ref.Node && image.Storage == ref.Storage && image.File == ref.File {
			sizeBytes = image.SizeBytes
			break
		}
	}

	return st.SetImageEnabled(ctx, clusterName, ref.Node, ref.Storage, ref.File, sizeBytes, enabled)
}

// ErrImageNotFound is returned when a cloud image approval row does not exist.
var ErrImageNotFound = errors.New("image not found")

// DeleteImage removes a cloud image approval row. Returns ErrImageNotFound
// when the cluster has no approval for the (node, storage, file) triple.
func DeleteImage(ctx context.Context, st *store.Store, cluster, node, storage, file string) error {
	err := st.DeleteImage(ctx, cluster, node, storage, file)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrImageNotFound
	}

	return err
}

// imageDiscovered reports whether the cluster reports a cloud image with the
// given (node, storage, file) triple. See nodeDiscovered for the error contract.
func imageDiscovered(ctx context.Context, client cluster.Client, node, storage, file string) (bool, error) {
	images, err := client.ListCloudImages(ctx)
	if err != nil {
		return false, err
	}

	return slices.ContainsFunc(images, func(i cluster.CloudImage) bool {
		return i.Node == node && i.Storage == storage && i.File == file
	}), nil
}
