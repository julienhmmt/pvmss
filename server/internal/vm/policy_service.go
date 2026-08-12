package vm

import (
	"pvmss/server/internal/policy"
	"pvmss/server/internal/store"
)

func selectPolicyService(st *store.Store, services []*policy.Policy) *policy.Policy {
	if len(services) > 0 && services[0] != nil {
		return services[0]
	}

	return policy.New(st, nil, nil)
}
