package apiv1

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/julienschmidt/httprouter"

	"pvmss/constants"
	"pvmss/logger"
	"pvmss/proxmox"
)

// --- User Pool ---

// poolMember is a single entry in a Proxmox pool's members list.
type poolMember struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// ListPools handles GET /api/v1/admin/userpool.
// The Proxmox GET /pools list endpoint does NOT return members; we must call
// GET /pools/{poolid} per pool to get accurate member counts.
func (h *AdminMutationsHandler) ListPools(w http.ResponseWriter, r *http.Request) {
	if h.state.IsOfflineMode() {
		writeJSON(w, []AdminPoolResponse{})
		return
	}
	cfg := h.state.GetEnvConfig()
	restyClient, err := proxmox.MakeRestyClientFromEnvConfig(cfg, 10*time.Second)
	if err != nil {
		writeAppError(w, err)
		return
	}

	// Step 1: list all pools (no members here)
	var listResp struct {
		Data []struct {
			PoolID  string `json:"poolid"`
			Comment string `json:"comment"`
		} `json:"data"`
	}
	if err := restyClient.Get(r.Context(), "/pools", &listResp); err != nil {
		writeAppError(w, err)
		return
	}

	// Step 2: for each pvmss-managed pool, fetch detail to get members
	type detailResp struct {
		Data struct {
			PoolID  string       `json:"poolid"`
			Comment string       `json:"comment"`
			Members []poolMember `json:"members"`
		} `json:"data"`
	}

	result := make([]AdminPoolResponse, 0, len(listResp.Data))
	for _, p := range listResp.Data {
		if !strings.HasPrefix(p.PoolID, constants.PoolPrefix) {
			continue
		}
		var detail detailResp
		if err := restyClient.Get(r.Context(), "/pools/"+url.PathEscape(p.PoolID), &detail); err != nil {
			logger.Get().Warn().Err(err).Str("pool", p.PoolID).Msg("failed to fetch pool detail")
			result = append(result, AdminPoolResponse{
				PoolID:  p.PoolID,
				Comment: p.Comment,
				Members: []string{},
				VMCount: 0,
			})
			continue
		}
		members := make([]string, 0, len(detail.Data.Members))
		vmCount := 0
		for _, m := range detail.Data.Members {
			members = append(members, m.ID)
			t := strings.ToLower(m.Type)
			if t == "qemu" || t == "lxc" {
				vmCount++
			}
		}
		result = append(result, AdminPoolResponse{
			PoolID:  detail.Data.PoolID,
			Comment: detail.Data.Comment,
			Members: members,
			VMCount: vmCount,
		})
	}
	writeJSON(w, result)
}

// CreatePool handles POST /api/v1/admin/userpool.
func (h *AdminMutationsHandler) CreatePool(w http.ResponseWriter, r *http.Request) {
	if h.state.IsOfflineMode() {
		errOffline(w)
		return
	}
	var req CreatePoolRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Pool == "" || req.Password == "" {
		errBadRequest(w, "pool and password are required")
		return
	}

	cfg := h.state.GetEnvConfig()
	restyClient, err := proxmox.MakeRestyClientFromEnvConfig(cfg, 10*time.Second)
	if err != nil {
		writeAppError(w, err)
		return
	}
	ctx := r.Context()

	if err := proxmox.EnsureRoleResty(ctx, restyClient, "PVMSSUser", []string{
		"VM.Allocate", "VM.Audit", "VM.Console", "VM.Config.Disk",
		"VM.Config.Network", "VM.Config.CPU", "VM.Config.Memory",
		"VM.Config.Options", "VM.Config.Cloudinit", "VM.Config.CDROM",
		"VM.PowerMgmt", "VM.Snapshot", "VM.Snapshot.Rollback",
		"Datastore.AllocateSpace", "Datastore.Audit",
		"SDN.Use",
	}); err != nil {
		writeAppError(w, err)
		return
	}

	username := req.Pool + constants.UserSuffix
	if err := proxmox.EnsureUserResty(ctx, restyClient, username, req.Password, "", fmt.Sprintf("PVMSS user for pool %s", req.Pool), "pve", true); err != nil {
		logger.Get().Error().Err(err).Str("username", username).Str("pool", req.Pool).Msg("api/v1: failed to ensure user")
		writeError(w, http.StatusInternalServerError, "user_creation_failed", "Failed to create user")
		return
	}

	poolID := constants.PoolPrefix + req.Pool
	if err := proxmox.EnsurePoolResty(ctx, restyClient, poolID, fmt.Sprintf("PVMSS managed pool for %s", req.Pool)); err != nil {
		logger.Get().Error().Err(err).Str("poolid", poolID).Msg("api/v1: failed to ensure pool")
		writeError(w, http.StatusInternalServerError, "pool_creation_failed", "Failed to create pool")
		return
	}

	if err := proxmox.EnsurePoolACLResty(ctx, restyClient, username, poolID, "PVMSSUser", true); err != nil {
		logger.Get().Error().Err(err).Str("username", username).Str("poolid", poolID).Msg("api/v1: failed to ensure pool ACL")
		writeError(w, http.StatusInternalServerError, "acl_creation_failed", "Failed to set ACL")
		return
	}

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, map[string]string{"poolid": poolID, "username": username})
}

// DeletePool handles DELETE /api/v1/admin/userpool/:name.
func (h *AdminMutationsHandler) DeletePool(w http.ResponseWriter, r *http.Request) {
	if h.state.IsOfflineMode() {
		errOffline(w)
		return
	}
	ps := httprouter.ParamsFromContext(r.Context())
	name := ps.ByName("name")
	if name == "" {
		errBadRequest(w, "missing pool name")
		return
	}
	cfg := h.state.GetEnvConfig()
	restyClient, err := proxmox.MakeRestyClientFromEnvConfig(cfg, 10*time.Second)
	if err != nil {
		writeAppError(w, err)
		return
	}
	ctx := r.Context()
	poolID := name
	if !strings.HasPrefix(poolID, constants.PoolPrefix) {
		poolID = constants.PoolPrefix + poolID
	}

	// Step 1: get pool members
	var detailResp struct {
		Data struct {
			Members []struct {
				Type string `json:"type"`
				VMID int    `json:"vmid"`
				Node string `json:"node"`
			} `json:"members"`
		} `json:"data"`
	}
	if err := restyClient.Get(ctx, "/pools/"+url.PathEscape(poolID), &detailResp); err != nil {
		logger.Get().Error().Err(err).Str("poolid", poolID).Msg("api/v1: failed to get pool members")
		writeError(w, http.StatusInternalServerError, "pool_members_failed", "Failed to get pool members")
		return
	}

	// Step 2: stop all QEMU VMs concurrently
	{
		var wg sync.WaitGroup
		for _, m := range detailResp.Data.Members {
			if m.VMID <= 0 || m.Node == "" || strings.ToLower(m.Type) != "qemu" {
				continue
			}
			m := m
			wg.Add(1)
			go func() {
				defer wg.Done()
				c, err := proxmox.MakeRestyClientFromEnvConfig(cfg, 10*time.Second)
				if err != nil {
					return
				}
				if _, err := proxmox.VMActionResty(ctx, c, m.Node, strconv.Itoa(m.VMID), "stop"); err != nil {
					logger.Get().Warn().Err(err).Int("vmid", m.VMID).Msg("stop VM before pool delete failed")
				}
			}()
		}
		wg.Wait()
		time.Sleep(3 * time.Second)
	}

	// Step 3: delete all VMs (purge)
	for _, m := range detailResp.Data.Members {
		if m.VMID <= 0 || m.Node == "" {
			continue
		}
		var path string
		switch strings.ToLower(m.Type) {
		case "qemu":
			path = "/nodes/" + url.PathEscape(m.Node) + "/qemu/" + url.PathEscape(strconv.Itoa(m.VMID)) + "?purge=1"
		case "lxc":
			path = "/nodes/" + url.PathEscape(m.Node) + "/lxc/" + url.PathEscape(strconv.Itoa(m.VMID)) + "?purge=1"
		default:
			continue
		}
		if err := restyClient.Delete(ctx, path, nil); err != nil {
			logger.Get().Error().Err(err).Str("path", path).Msg("api/v1: failed to delete VM during pool purge")
			writeError(w, http.StatusInternalServerError, "vm_delete_failed", "Failed to delete VM")
			return
		}
	}

	// Step 4: wait until pool is empty (up to 15s)
	deadline := time.Now().Add(15 * time.Second)
	for {
		var check struct {
			Data struct {
				Members []any `json:"members"`
			} `json:"data"`
		}
		if err := restyClient.Get(ctx, "/pools/"+url.PathEscape(poolID), &check); err == nil {
			if len(check.Data.Members) == 0 {
				break
			}
		}
		if time.Now().After(deadline) {
			logger.Get().Warn().Str("pool", poolID).Msg("pool still not empty after deletions; proceeding anyway")
			break
		}
		time.Sleep(1 * time.Second)
	}

	// Step 5: delete pool
	if err := restyClient.Delete(ctx, "/pools/"+url.PathEscape(poolID), nil); err != nil {
		logger.Get().Error().Err(err).Str("poolid", poolID).Msg("api/v1: failed to delete pool")
		writeError(w, http.StatusInternalServerError, "pool_delete_failed", "Failed to delete pool")
		return
	}

	// Step 6: delete user (best-effort)
	username := strings.TrimPrefix(poolID, constants.PoolPrefix)
	if !strings.Contains(username, "@") {
		username = username + constants.UserSuffix
	}
	_ = restyClient.Delete(ctx, fmt.Sprintf("/access/users/%s", username), nil)

	w.WriteHeader(http.StatusNoContent)
}
