// export_test.go exposes unexported recovery functions for black-box
// testing in the recovery_test package. This file is only compiled during
// `go test` and never ships in the binary.
package recovery

import (
	"context"
	"database/sql"
)

// UpsertClusterForTest exposes upsertCluster for testing.
func UpsertClusterForTest(ctx context.Context, db *sql.DB, row ClusterRow) error {
	return upsertCluster(ctx, db, row)
}

// UpsertNodeForTest exposes upsertNode for testing.
func UpsertNodeForTest(ctx context.Context, db *sql.DB, cluster string, r NodeRow) error {
	return upsertNode(ctx, db, cluster, r)
}

// UpsertStorageForTest exposes upsertStorage for testing.
func UpsertStorageForTest(ctx context.Context, db *sql.DB, cluster string, r StorageRow) error {
	return upsertStorage(ctx, db, cluster, r)
}

// UpsertBridgeForTest exposes upsertBridge for testing.
func UpsertBridgeForTest(ctx context.Context, db *sql.DB, cluster string, r BridgeRow) error {
	return upsertBridge(ctx, db, cluster, r)
}

// UpsertISOForTest exposes upsertISO for testing.
func UpsertISOForTest(ctx context.Context, db *sql.DB, cluster string, r ISORow) error {
	return upsertISO(ctx, db, cluster, r)
}

// UpsertProfileForTest exposes upsertProfile for testing.
func UpsertProfileForTest(ctx context.Context, db *sql.DB, cluster string, r ProfileRow) error {
	return upsertProfile(ctx, db, cluster, r)
}

// UpsertTagForTest exposes upsertTag for testing.
func UpsertTagForTest(ctx context.Context, db *sql.DB, cluster string, r TagRow) error {
	return upsertTag(ctx, db, cluster, r)
}

// UpsertVMLimitsForTest exposes upsertVMLimits for testing.
func UpsertVMLimitsForTest(ctx context.Context, db *sql.DB, cluster string, r VMLimitsRow) error {
	return upsertVMLimits(ctx, db, cluster, r)
}

// UpsertNodeLimitsForTest exposes upsertNodeLimits for testing.
func UpsertNodeLimitsForTest(ctx context.Context, db *sql.DB, cluster string, r NodeLimitsRow) error {
	return upsertNodeLimits(ctx, db, cluster, r)
}

// SplitISOVolidForTest exposes splitISOVolid for testing.
func SplitISOVolidForTest(name string) (storage, file string, ok bool) {
	return splitISOVolid(name)
}

// EncryptTokenForTest exposes encryptToken for testing.
func EncryptTokenForTest(secret, sessionSecret string) ([]byte, error) {
	return encryptToken(secret, sessionSecret)
}

// MapNodesForTest exposes mapNodes for testing.
func MapNodesForTest(ctx context.Context, db *sql.DB) ([]NodeRow, error) {
	return mapNodes(ctx, db)
}

// MapStoragesForTest exposes mapStorages for testing.
func MapStoragesForTest(ctx context.Context, db *sql.DB, cluster string, resolver StorageNodeResolver) ([]StorageRow, []SkipReason, error) {
	return mapStorages(ctx, db, cluster, resolver)
}

// MapBridgesForTest exposes mapBridges for testing.
func MapBridgesForTest(ctx context.Context, db *sql.DB) ([]BridgeRow, error) {
	return mapBridges(ctx, db)
}

// MapISOsForTest exposes mapISOs for testing.
func MapISOsForTest(ctx context.Context, db *sql.DB) ([]ISORow, []SkipReason, error) {
	return mapISOs(ctx, db)
}

// MapVMLimitsForTest exposes mapVMLimits for testing.
func MapVMLimitsForTest(ctx context.Context, db *sql.DB) (VMLimitsRow, error) {
	return mapVMLimits(ctx, db)
}

// MapNodeLimitsForTest exposes mapNodeLimits for testing.
func MapNodeLimitsForTest(ctx context.Context, db *sql.DB) ([]NodeLimitsRow, error) {
	return mapNodeLimits(ctx, db)
}
