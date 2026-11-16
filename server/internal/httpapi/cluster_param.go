//nolint:wsl_v5 // parameter resolution keeps ambiguity handling explicit
package httpapi

import (
	"errors"
	"net/http"
	"pvmss/server/internal/cluster"
	"slices"
)

// ErrClusterRequired indicates that an ambiguous multi-cluster request omitted its choice.
var ErrClusterRequired = errors.New("cluster parameter is required when multiple clusters are configured")

// ClusterLister is the minimal runtime cluster registry needed by admin parameters.
type ClusterLister interface {
	List() []string
}

// ResolveClusterParam resolves a query parameter without guessing once names are ambiguous.
func ResolveClusterParam(request *http.Request, lister ClusterLister) (string, error) {
	return resolveClusterValue(request.URL.Query().Get("cluster"), lister)
}

// ResolveClusterValue resolves a JSON body cluster field with the same rule as query parameters.
func ResolveClusterValue(value string, lister ClusterLister) (string, error) {
	return resolveClusterValue(value, lister)
}

func resolveClusterValue(value string, lister ClusterLister) (string, error) {
	if lister == nil {
		if value == "" {
			return "", ErrClusterRequired
		}
		return value, nil
	}
	names := lister.List()
	if value != "" {
		if slices.Contains(names, value) {
			return value, nil
		}
		return "", cluster.ErrClusterNotFound
	}
	if len(names) == 1 {
		return names[0], nil
	}
	if len(names) > 1 {
		return "", ErrClusterRequired
	}
	return "", ErrClusterRequired
}

// clusterParamError maps a resolution failure to its response: an unknown
// cluster name is a 404, a missing-but-required choice stays a 400.
func clusterParamError(err error) (int, string, string) {
	if errors.Is(err, ErrClusterRequired) {
		return http.StatusBadRequest, "cluster_required", "cluster parameter is required when multiple clusters are configured"
	}
	if errors.Is(err, cluster.ErrClusterNotFound) {
		return http.StatusNotFound, "cluster_not_found", "cluster not found"
	}
	return http.StatusBadRequest, codeInvalidRequest, "invalid cluster parameter"
}
