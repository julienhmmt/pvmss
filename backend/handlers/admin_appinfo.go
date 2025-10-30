package handlers

import (
	"net/http"
	"os"
	"runtime"

	"github.com/julienschmidt/httprouter"

	"pvmss/constants"
	"pvmss/proxmox"
	"pvmss/utils"
)

// AppInfoPageHandler renders the Application Info admin page
func (h *AdminHandler) AppInfoPageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("AppInfoPageHandler", r)

	// Collect build information
	buildInfo := map[string]interface{}{
		"version":   constants.AppVersion,
		"goVersion": runtime.Version(),
		"goOS":      runtime.GOOS,
		"goArch":    runtime.GOARCH,
	}

	// Collect environment information (safe variables only - no secrets)
	safeEnvVars := []string{
		"LOG_LEVEL",
		"PROXMOX_URL",
		"PROXMOX_VERIFY_SSL",
		"PVMSS_ENV",
		"PVMSS_OFFLINE",
		"PVMSS_SETTINGS_PATH",
	}

	envInfo := make(map[string]string)
	for _, key := range safeEnvVars {
		if val := os.Getenv(key); val != "" {
			envInfo[key] = val
		}
	}

	// Detect environment using PVMSS_ENV
	environment := "production"
	isOffline := os.Getenv("PVMSS_OFFLINE") == "true"

	if isOffline {
		environment = "offline"
	} else if !utils.IsProduction() {
		environment = "development"
	}

	buildInfo["environment"] = environment
	buildInfo["environmentDetails"] = map[string]interface{}{
		"isDevelopment": environment == "development",
		"isProduction":  environment == "production",
		"isOffline":     environment == "offline",
	}

	// Environment variables (safe only)
	buildInfo["environmentVariables"] = envInfo

	// Detect Proxmox cluster information
	clusterInfo := map[string]interface{}{
		"isCluster":   false,
		"clusterName": "",
		"nodeCount":   0,
	}

	if sm := getStateManager(r); sm != nil {
		if client := sm.GetProxmoxClient(); client != nil {
			// Try to get cluster status using the new API method
			if clusterStatus, err := proxmox.GetClusterStatus(r.Context(), client); err == nil {
				clusterInfo["isCluster"] = clusterStatus.IsCluster
				clusterInfo["clusterName"] = clusterStatus.ClusterName
				clusterInfo["nodeCount"] = clusterStatus.NodeCount
				if clusterStatus.IsCluster {
					log.Info().
						Str("cluster_name", clusterStatus.ClusterName).
						Int("nodes", clusterStatus.NodeCount).
						Msg("Proxmox cluster detected via /cluster/status")
				} else {
					log.Info().Msg("Proxmox standalone mode detected via /cluster/status")
				}
			} else {
				// Fallback to the old method using cluster name from ticket
				log.Warn().Err(err).Msg("Failed to get cluster status, falling back to cluster name detection")
				clusterName := client.GetClusterName()
				if clusterName != "" {
					clusterInfo["isCluster"] = true
					clusterInfo["clusterName"] = clusterName
					log.Info().Str("cluster_name", clusterName).Msg("Proxmox cluster detected via fallback method")
				}
			}
		}
	}

	buildInfo["clusterInfo"] = clusterInfo

	builder := NewTemplateData("").
		SetAdminActive("appinfo").
		SetAuth(r).
		SetProxmoxStatus(h.stateManager).
		ParseMessages(r).
		AddData("TitleKey", "Admin.AppInfo.Title").
		AddData("BuildInfo", buildInfo)

	data := builder.Build().ToMap()
	log.Info().Msg("Rendering Application Info page")
	renderTemplateInternal(w, r, "admin_appinfo", data)
}
