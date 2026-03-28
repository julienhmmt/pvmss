package apiv1

import (
	"net/http"

	"github.com/julienschmidt/httprouter"

	"pvmss/state"
)

// RegisterRoutes mounts all /api/v1/ routes onto the provided router.
// JWT-protected routes are wrapped with JWTMiddleware; the auth exchange
// endpoint only needs a session (session is loaded by the API middleware chain).
func RegisterRoutes(router *httprouter.Router, s state.StateManager) {
	authHandler := MakeAuthHandler(s)
	vmHandler := MakeVMHandler(s)
	vmActionHandler := MakeVMActionHandler(s)

	// Auth routes — no JWT required (login/exchange issue tokens)
	router.POST("/api/v1/auth/login", wrap(authHandler.Login))
	router.POST("/api/v1/auth/proxmox-admin-login", wrap(authHandler.ProxmoxAdminLogin))
	router.POST("/api/v1/auth/exchange", wrap(authHandler.Exchange))
	router.POST("/api/v1/auth/refresh", wrap(authHandler.Refresh))
	router.POST("/api/v1/auth/logout", wrap(authHandler.Logout))

	// Authenticated auth routes
	router.GET("/api/v1/auth/me", jwtWrap(s, authHandler.Me))

	// VM routes — JWT required
	router.GET("/api/v1/vms", jwtWrap(s, vmHandler.ListVMs))
	router.GET("/api/v1/vms/:id", jwtWrap(s, vmHandler.GetVM))
	router.POST("/api/v1/vms/:id/action", jwtWrap(s, vmActionHandler.VMAction))

	// Admin API routes — JWT + isAdmin required
	adminHandler := MakeAdminHandler(s)
	adminVMsHandler := MakeAdminVMsAPIHandler(s)
	adminMutHandler := MakeAdminMutationsHandler(s)

	router.GET("/api/v1/admin/nodes", adminJWTWrap(s, adminHandler.Nodes))
	router.GET("/api/v1/admin/storage", adminJWTWrap(s, adminHandler.Storage))
	router.GET("/api/v1/admin/vmbr", adminJWTWrap(s, adminHandler.VMBR))
	router.GET("/api/v1/admin/iso", adminJWTWrap(s, adminHandler.ISO))
	router.GET("/api/v1/admin/appinfo", adminJWTWrap(s, adminHandler.AppInfo))
	router.GET("/api/v1/admin/settings", adminJWTWrap(s, adminHandler.Settings))

	router.GET("/api/v1/admin/vms", adminJWTWrap(s, adminVMsHandler.ListAllVMs))
	router.POST("/api/v1/admin/vms/:id/action", adminJWTWrap(s, adminVMsHandler.VMAction))

	router.GET("/api/v1/admin/userpool", adminJWTWrap(s, adminMutHandler.ListPools))
	router.POST("/api/v1/admin/userpool", adminJWTWrap(s, adminMutHandler.CreatePool))
	router.DELETE("/api/v1/admin/userpool/:name", adminJWTWrap(s, adminMutHandler.DeletePool))

	router.GET("/api/v1/admin/tags", adminJWTWrap(s, adminMutHandler.ListTags))
	router.POST("/api/v1/admin/tags", adminJWTWrap(s, adminMutHandler.CreateTag))
	router.DELETE("/api/v1/admin/tags/:name", adminJWTWrap(s, adminMutHandler.DeleteTag))

	router.GET("/api/v1/admin/limits", adminJWTWrap(s, adminMutHandler.GetLimits))
	router.PUT("/api/v1/admin/limits", adminJWTWrap(s, adminMutHandler.UpdateLimits))

	router.GET("/api/v1/admin/cloudinit/storages", adminJWTWrap(s, adminHandler.CloudInitStorages))
	router.GET("/api/v1/admin/cloudinit", adminJWTWrap(s, adminMutHandler.ListCloudInit))
	router.POST("/api/v1/admin/cloudinit", adminJWTWrap(s, adminMutHandler.CreateCloudInit))
	router.PUT("/api/v1/admin/cloudinit/:id", adminJWTWrap(s, adminMutHandler.UpdateCloudInit))
	router.DELETE("/api/v1/admin/cloudinit/:id", adminJWTWrap(s, adminMutHandler.DeleteCloudInit))
	router.POST("/api/v1/admin/cloudinit/:id/toggle", adminJWTWrap(s, adminMutHandler.ToggleCloudInit))

	router.POST("/api/v1/admin/storage/toggle", adminJWTWrap(s, adminMutHandler.ToggleStorage))
	router.POST("/api/v1/admin/vmbr/toggle", adminJWTWrap(s, adminMutHandler.ToggleVMBR))
	router.POST("/api/v1/admin/iso/toggle", adminJWTWrap(s, adminMutHandler.ToggleISO))
}

// wrap converts a plain http.HandlerFunc into the httprouter.Handle signature.
func wrap(h http.HandlerFunc) httprouter.Handle {
	return httprouterWrap(h)
}

// jwtWrap wraps a handler with JWT authentication and converts it to httprouter.Handle.
func jwtWrap(s state.StateManager, h http.HandlerFunc) httprouter.Handle {
	return httprouterWrap(JWTMiddleware(s, h))
}

// adminJWTWrap wraps a handler with JWT + isAdmin check.
func adminJWTWrap(s state.StateManager, h http.HandlerFunc) httprouter.Handle {
	return httprouterWrap(JWTAdminMiddleware(s, h))
}
