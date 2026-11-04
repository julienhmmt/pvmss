package store

import (
	"maps"
	"slices"
)

// ExcludedImportTables exposes the excluded-table map to store_test.
func ExcludedImportTables() map[string]string {
	return maps.Clone(excludedImportTables)
}

// ClusterScopedTables exposes the rename table list to store_test.
func ClusterScopedTables() []string {
	return slices.Clone(clusterScopedTables)
}
