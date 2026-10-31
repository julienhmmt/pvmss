package httpapi

import (
	"pvmss/server/internal/cluster"
)

// fakeSingleProvider serves one cluster.Client for every name, the internal
// counterpart of the external package's singleClientProvider.
type fakeSingleProvider struct {
	client cluster.Client
}

func (provider fakeSingleProvider) Client(string) (cluster.Client, error) {
	return provider.client, nil
}

func (provider fakeSingleProvider) List() []string {
	return []string{"default"}
}
