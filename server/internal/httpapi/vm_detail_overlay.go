package httpapi

import "pvmss/server/internal/vm"

// withPatch returns entity with the just-written name/description applied.
// The projection reads Proxmox /cluster/resources, which lags a config write
// by seconds, so the re-resolved entity can still carry the old values.
func withPatch(entity vm.Entity, req patchRequest) vm.Entity {
	if req.Name != "" {
		entity.Name = req.Name
	}

	if req.Description != nil {
		entity.Description = *req.Description
	}

	return entity
}

// withHardware returns entity with the just-written sockets/cores/memory
// applied, for the same lag reason as withPatch.
func withHardware(entity vm.Entity, req hardwareRequest) vm.Entity {
	if req.Sockets != nil {
		entity.Sockets = *req.Sockets
	}

	if req.Cores != nil {
		entity.Cores = *req.Cores
	}

	if req.Sockets != nil || req.Cores != nil {
		entity.CPUCores = entity.Sockets * entity.Cores
	}

	if req.MemoryMB != nil {
		entity.MemoryTotal = int64(*req.MemoryMB) << 20
	}

	return entity
}
