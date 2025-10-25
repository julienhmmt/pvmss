package handlers

import (
	"net/http"
	"os"
	"runtime"

	"github.com/julienschmidt/httprouter"

	"pvmss/constants"
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

	data := AdminPageDataWithMessage("Application Info", "appinfo", "", "")
	data["BuildInfo"] = buildInfo

	log.Info().Msg("Rendering Application Info page")
	renderTemplateInternal(w, r, "admin_appinfo", data)
}
