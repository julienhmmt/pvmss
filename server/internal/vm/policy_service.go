package vm

import (
	"context"
	"fmt"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/store"
)

func selectPolicyService(st *store.Store, services []*policy.Policy) *policy.Policy {
	if len(services) > 0 && services[0] != nil {
		return services[0]
	}

	return policy.New(st, nil, nil)
}

// resolveGabarit returns the effective gabarit, reading from the policy service
// when available and falling back to the pre-loaded value. When no policy is
// configured and the availability check fails, it returns policy.ErrUnavailable.
func resolveGabarit(ctx context.Context, service *policy.Policy, fallback policy.Gabarit, clusterName string, available func(policy.Gabarit) bool) (policy.Gabarit, error) {
	gabarit := fallback

	if service != nil {
		resolved, err := service.Gabarit(ctx, clusterName)
		if err != nil {
			return policy.Gabarit{}, fmt.Errorf("read gabarit: %w", err)
		}

		gabarit = resolved
	}

	if service == nil && !available(gabarit) {
		return policy.Gabarit{}, policy.ErrUnavailable
	}

	return gabarit, nil
}
