package inventory

import "testing"

// TestRegistryConstructors_InternalMapsInitialized guards the invariant that
// every constructor leaves entries and handles writable - a nil handles map
// panics on the next makeEntry write.
func TestRegistryConstructors_InternalMapsInitialized(t *testing.T) {
	t.Parallel()

	registries := map[string]*Registry{
		"NewRegistryFromIndexes":     NewRegistryFromIndexes(map[string]*Index{}),
		"NewRegistryFromProjections": NewRegistryFromProjections(map[string]*Projection{}),
	}

	for name, registry := range registries {
		if registry.entries == nil {
			t.Errorf("%s: entries map is nil", name)
		}

		if registry.handles == nil {
			t.Errorf("%s: handles map is nil", name)
		}
	}
}
