package apiv1

import (
	"net/http"

	"github.com/julienschmidt/httprouter"

	"pvmss/database"
	"pvmss/state"
)

// RegisterRoutes mounts all /api/v1/ routes onto the provided router.
// JWT-protected routes are wrapped with JWTMiddleware; the auth exchange
// endpoint only needs a session (session is loaded by the API middleware chain).
func RegisterRoutes(router *httprouter.Router, s state.StateManager) {
	jwtSecret := s.GetEnvConfig().JWTSecret
	authHandler := MakeAuthHandler(s, jwtSecret)
	vmHandler := MakeVMHandler(s)
	vmActionHandler := MakeVMActionHandler(s)
	healthHandler := MakeHealthHandler(s)
	adminHandler := MakeAdminHandler(s)

	// Health routes — public, no auth required
	router.GET("/api/v1/health", wrap(healthHandler.Health))
	router.GET("/api/v1/health/proxmox", wrap(healthHandler.HealthProxmox))

	// Public version endpoint — no auth required
	router.GET("/api/v1/public/version", wrap(adminHandler.Version))

	// Auth routes — no JWT required (login/exchange issue tokens)
	router.POST("/api/v1/auth/login", wrap(authHandler.Login))
	router.POST("/api/v1/auth/proxmox-admin-login", wrap(authHandler.ProxmoxAdminLogin))
	router.POST("/api/v1/auth/exchange", wrap(authHandler.Exchange))
	router.POST("/api/v1/auth/refresh", wrap(authHandler.Refresh))
	router.POST("/api/v1/auth/logout", wrap(authHandler.Logout))

	// Authenticated auth routes
	router.GET("/api/v1/auth/me", jwtWrap(jwtSecret, authHandler.Me))
	router.PUT("/api/v1/auth/me/password", jwtWrap(jwtSecret, authHandler.ChangePassword))

	// VM routes — JWT required
	router.GET("/api/v1/vms", jwtWrap(jwtSecret, vmHandler.ListVMs))
	router.GET("/api/v1/vms/:id", jwtWrap(jwtSecret, vmHandler.GetVM))
	router.POST("/api/v1/vms/:id/action", jwtWrap(jwtSecret, vmActionHandler.VMAction))

	// Search route — JWT required; accepts ?q= and ?filter=vmid|name|tags
	router.GET("/api/v1/search/vms", jwtWrap(jwtSecret, vmHandler.ListVMs))

	// VM creation routes — JWT required
	vmCreateHandler := MakeVMCreateHandler(s)
	router.GET("/api/v1/vm-create/settings", jwtWrap(jwtSecret, vmCreateHandler.GetSettings))
	router.POST("/api/v1/vms", jwtWrap(jwtSecret, vmCreateHandler.CreateVM))

	// VM detail routes — JWT required
	vmDetailsHandler := MakeVMDetailsHandler(s)
	router.GET("/api/v1/vms/:id/config", jwtWrap(jwtSecret, vmDetailsHandler.GetVMConfig))
	router.GET("/api/v1/vms/:id/metrics", jwtWrap(jwtSecret, vmDetailsHandler.GetVMMetrics))
	router.PATCH("/api/v1/vms/:id/config", jwtWrap(jwtSecret, vmDetailsHandler.UpdateVMConfig))
	router.GET("/api/v1/vms/:id/snapshots", jwtWrap(jwtSecret, vmDetailsHandler.GetVMSnapshots))
	router.POST("/api/v1/vms/:id/snapshots", jwtWrap(jwtSecret, vmDetailsHandler.CreateSnapshot))
	router.DELETE("/api/v1/vms/:id/snapshots/:name", jwtWrap(jwtSecret, vmDetailsHandler.DeleteSnapshot))
	router.POST("/api/v1/vms/:id/snapshots/:name/rollback", jwtWrap(jwtSecret, vmDetailsHandler.RollbackSnapshot))
	router.GET("/api/v1/vms/:id/settings", jwtWrap(jwtSecret, vmDetailsHandler.GetVMSettings))
	router.PUT("/api/v1/vms/:id/hardware", jwtWrap(jwtSecret, vmDetailsHandler.UpdateVMHardware))
	router.POST("/api/v1/vms/:id/network/:iface/toggle", jwtWrap(jwtSecret, vmDetailsHandler.ToggleNIC))
	router.DELETE("/api/v1/vms/:id", jwtWrap(jwtSecret, vmHandler.DeleteVM))

	// Disk management routes — JWT required
	vmDiskHandler := MakeVMDiskHandler(s)
	router.POST("/api/v1/vms/:id/disks", jwtWrap(jwtSecret, vmDiskHandler.AddDisk))
	router.PUT("/api/v1/vms/:id/disks/:diskId/resize", jwtWrap(jwtSecret, vmDiskHandler.ResizeDisk))
	router.DELETE("/api/v1/vms/:id/disks/:diskId", jwtWrap(jwtSecret, vmDiskHandler.DeleteDisk))

	// Task status routes — JWT required
	taskHandler := MakeTaskHandler(s)
	router.GET("/api/v1/tasks/status", jwtWrap(jwtSecret, taskHandler.GetTaskStatus))
	router.GET("/api/v1/tasks/log", jwtWrap(jwtSecret, taskHandler.GetTaskLog))

	// VNC console routes — JWT required
	vncHandler := MakeVNCHandler(s)
	router.POST("/api/v1/vms/:id/vnc-ticket", jwtWrap(jwtSecret, vncHandler.GetVNCTicket))
	router.GET("/api/v1/vms/:id/console/websocket", jwtWrap(jwtSecret, vncHandler.ConsoleWebSocket))

	// Docs routes — public; auth checked inside handler for admin-only types
	docsHandler := MakeDocsAPIHandler(s, jwtSecret)
	router.GET("/api/v1/docs/:type", wrap(docsHandler.GetDoc))

	// Admin API routes — JWT + isAdmin required
	adminVMsHandler := MakeAdminVMsAPIHandler(s)
	adminMutHandler := MakeAdminMutationsHandler(s)

	router.GET("/api/v1/admin/nodes", adminJWTWrap(jwtSecret, adminHandler.Nodes))
	router.GET("/api/v1/admin/storage", adminJWTWrap(jwtSecret, adminHandler.Storage))
	router.GET("/api/v1/admin/vmbr", adminJWTWrap(jwtSecret, adminHandler.VMBR))
	router.GET("/api/v1/admin/iso", adminJWTWrap(jwtSecret, adminHandler.ISO))
	router.GET("/api/v1/admin/appinfo", adminJWTWrap(jwtSecret, adminHandler.AppInfo))
	router.GET("/api/v1/admin/settings", adminJWTWrap(jwtSecret, adminHandler.Settings))

	router.GET("/api/v1/admin/vms", adminJWTWrap(jwtSecret, adminVMsHandler.ListAllVMs))
	router.GET("/api/v1/admin/vms/paginated", adminJWTWrap(jwtSecret, adminVMsHandler.ListAllVMsPaginated))
	router.POST("/api/v1/admin/vms/:id/action", adminJWTWrap(jwtSecret, adminVMsHandler.VMAction))
	router.DELETE("/api/v1/admin/vms/:id", adminJWTWrap(jwtSecret, adminVMsHandler.DeleteVM))

	router.GET("/api/v1/admin/userpool", adminJWTWrap(jwtSecret, adminMutHandler.ListPools))
	router.POST("/api/v1/admin/userpool", adminJWTWrap(jwtSecret, adminMutHandler.CreatePool))
	router.DELETE("/api/v1/admin/userpool/:name", adminJWTWrap(jwtSecret, adminMutHandler.DeletePool))

	router.GET("/api/v1/admin/tags", adminJWTWrap(jwtSecret, adminMutHandler.ListTags))
	router.POST("/api/v1/admin/tags", adminJWTWrap(jwtSecret, adminMutHandler.CreateTag))
	router.DELETE("/api/v1/admin/tags/:name", adminJWTWrap(jwtSecret, adminMutHandler.DeleteTag))

	router.GET("/api/v1/admin/limits", adminJWTWrap(jwtSecret, adminMutHandler.GetLimits))
	router.PUT("/api/v1/admin/limits", adminJWTWrap(jwtSecret, adminMutHandler.UpdateLimits))

	router.GET("/api/v1/admin/cloudinit/storages", adminJWTWrap(jwtSecret, adminHandler.CloudInitStorages))
	router.GET("/api/v1/admin/cloudinit", adminJWTWrap(jwtSecret, adminMutHandler.ListCloudInit))
	router.POST("/api/v1/admin/cloudinit", adminJWTWrap(jwtSecret, adminMutHandler.CreateCloudInit))
	router.PUT("/api/v1/admin/cloudinit/:id", adminJWTWrap(jwtSecret, adminMutHandler.UpdateCloudInit))
	router.DELETE("/api/v1/admin/cloudinit/:id", adminJWTWrap(jwtSecret, adminMutHandler.DeleteCloudInit))
	router.POST("/api/v1/admin/cloudinit/:id/toggle", adminJWTWrap(jwtSecret, adminMutHandler.ToggleCloudInit))

	router.GET("/api/v1/admin/vm-profiles", adminJWTWrap(jwtSecret, adminMutHandler.ListVMProfiles))
	router.POST("/api/v1/admin/vm-profiles", adminJWTWrap(jwtSecret, adminMutHandler.CreateVMProfile))
	router.PUT("/api/v1/admin/vm-profiles/:id", adminJWTWrap(jwtSecret, adminMutHandler.UpdateVMProfile))
	router.DELETE("/api/v1/admin/vm-profiles/:id", adminJWTWrap(jwtSecret, adminMutHandler.DeleteVMProfile))
	router.POST("/api/v1/admin/vm-profiles/:id/toggle", adminJWTWrap(jwtSecret, adminMutHandler.ToggleVMProfile))

	router.POST("/api/v1/admin/storage/toggle", adminJWTWrap(jwtSecret, adminMutHandler.ToggleStorage))
	router.POST("/api/v1/admin/vmbr/toggle", adminJWTWrap(jwtSecret, adminMutHandler.ToggleVMBR))
	router.POST("/api/v1/admin/iso/toggle", adminJWTWrap(jwtSecret, adminMutHandler.ToggleISO))
}

// RegisterAdminDBRoutes mounts the audit log and database management routes.
// These require admin JWT auth and are separated from RegisterRoutes so that
// the database.DB handle only needs to be threaded where it is actually used.
func RegisterAdminDBRoutes(router *httprouter.Router, s state.StateManager, db database.DB) {
	jwtSecret := s.GetEnvConfig().JWTSecret
	h := MakeAdminDBHandler(s, db)

	router.GET("/api/v1/admin/audit", adminJWTWrap(jwtSecret, h.ListAuditLog))
	router.GET("/api/v1/admin/db/export", adminJWTWrap(jwtSecret, h.ExportDB))
	router.POST("/api/v1/admin/db/import", adminJWTWrap(jwtSecret, h.ImportDB))
	router.POST("/api/v1/admin/migrate-from-json", adminJWTWrap(jwtSecret, h.MigrateFromJSON))
}

// RegisterSetupRoutes mounts the first-run setup wizard routes onto the provided
// router.  The status endpoint is always public; the mutating endpoints are
// wrapped with requireSetupIncomplete so they return 404 once bootstrap is done.
func RegisterSetupRoutes(router *httprouter.Router, s state.StateManager, db database.DB) {
	h := MakeSetupHandler(s, db)
	router.GET("/api/v1/setup/status", wrap(h.Status))
	router.POST("/api/v1/setup/test-connection", wrap(requireSetupIncomplete(db, h.TestConnection)))
	router.GET("/api/v1/setup/proxmox-data", wrap(requireSetupIncomplete(db, h.ProxmoxData)))
	router.POST("/api/v1/setup/complete", wrap(requireSetupIncomplete(db, h.Complete)))
}

// wrap converts a plain http.HandlerFunc into the httprouter.Handle signature.
func wrap(h http.HandlerFunc) httprouter.Handle {
	return httprouterWrap(h)
}

// jwtWrap wraps a handler with JWT authentication and converts it to httprouter.Handle.
func jwtWrap(jwtSecret string, h http.HandlerFunc) httprouter.Handle {
	return httprouterWrap(JWTMiddleware(jwtSecret, h))
}

// adminJWTWrap wraps a handler with JWT + isAdmin check.
func adminJWTWrap(jwtSecret string, h http.HandlerFunc) httprouter.Handle {
	return httprouterWrap(JWTAdminMiddleware(jwtSecret, h))
}
