package store

import "maps"

// ExcludedImportTables exposes the excluded-table map to store_test.
func ExcludedImportTables() map[string]string {
	return maps.Clone(excludedImportTables)
}
