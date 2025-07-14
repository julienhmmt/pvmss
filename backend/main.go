package main

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/joho/godotenv"
	"pvmss/backend/proxmox"
	"golang.org/x/crypto/bcrypt"
	"github.com/alexedwards/scs/v2"
)

var templates *template.Template
var sessionManager *scs.SessionManager

func main() {
	if err := godotenv.Load("../.env"); err != nil {
		log.Warn().Msg("Error loading .env file, relying on environment variables")
	}

	// Log the loaded Proxmox URL to verify it's loaded correctly
	proxmoxURL := os.Getenv("PROXMOX_URL")
	if proxmoxURL == "" {
		log.Warn().Msg("PROXMOX_URL is not set")
	} else {
		log.Info().Str("PROXMOX_URL", proxmoxURL).Msg("Proxmox URL loaded")
	}

	// Initialize logger
	zerolog.TimeFieldFormat = time.RFC3339Nano
	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: "2006-01-02 15:04:05",
	})

	// Initialize i18n
	initI18n()

	// Parse templates
	funcMap := template.FuncMap{
		"div": func(a, b float64) float64 {
			if b == 0 {
				return 0
			}
			return a / b
		},
		"mul": func(a, b float64) float64 {
			return a * b
		},
		"humanBytes": func(b interface{}) string {
			var bytes uint64
			switch v := b.(type) {
			case float64:
				bytes = uint64(v)
			case uint64:
				bytes = v
			case int:
				bytes = uint64(v)
			case int64:
				bytes = uint64(v)
			default:
				return "N/A"
			}

			const unit = 1024
			if bytes < unit {
				return fmt.Sprintf("%d B", bytes)
			}
			div, exp := uint64(unit), 0
			for n := bytes / unit; n >= unit; n /= unit {
				div *= unit
				exp++
			}
			return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
		},
		"formatMemory": formatMemory,
	}

	var err error
	templates, err = template.New("layout.html").Funcs(funcMap).ParseGlob("../frontend/*.html")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to parse templates")
	}

	// Get port from environment
	port := os.Getenv("PORT")
	if port == "" {
		port = "50000"
		log.Info().Msgf("Using default port: %s", port)
	}

	// Initialize session manager
	sessionManager = scs.New()
	sessionManager.Lifetime = 24 * time.Hour

	// Create router
	r := http.NewServeMux()

	// Serve static files
	fs := http.FileServer(http.Dir("../frontend/css"))
	r.Handle("/css/", http.StripPrefix("/css/", fs))

	// Handlers
	r.HandleFunc("/", indexHandler)
	r.HandleFunc("/search", searchHandler)
	r.HandleFunc("/login", loginHandler)
	r.HandleFunc("/logout", logoutHandler)

	// Documentation routes
	r.HandleFunc("/docs/admin", serveDocHandler("admin"))
	r.HandleFunc("/docs/user", serveDocHandler("user"))

	authedRoutes := http.NewServeMux()
	authedRoutes.HandleFunc("/admin", adminHandler)

	r.Handle("/admin", authMiddleware(authedRoutes))
	r.HandleFunc("/storage", storagePageHandler)
	r.HandleFunc("/iso", isoPageHandler)
	r.HandleFunc("/vmbr", vmbrPageHandler)
	r.HandleFunc("/health", healthHandler)

	// API handlers
	r.HandleFunc("/api/tags", tagsHandler)
	r.HandleFunc("/api/tags/", tagsHandler)
	r.HandleFunc("/api/storage", storageHandler)
	r.HandleFunc("/api/iso/all", allIsosHandler)
	r.HandleFunc("/api/vmbr/all", allVmbrsHandler)
	r.HandleFunc("/api/settings", settingsHandler)
	r.HandleFunc("/api/iso/settings", updateIsoSettingsHandler)
	r.HandleFunc("/api/vmbr/settings", updateVmbrSettingsHandler)
	r.HandleFunc("/api/limits", limitsHandler)

	// Configure server
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: sessionManager.LoadAndSave(r),
	}

	// Graceful shutdown
	go func() {
		log.Info().Str("port", port).Msg("Starting server...")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Server failed to start")
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info().Msg("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Server forced to shutdown")
	}

	log.Info().Msg("Server exiting")
}

func formatMemory(mem interface{}) string {
	if mem == nil {
		return "N/A"
	}
	memFloat, ok := mem.(float64)
	if !ok {
		return "N/A"
	}

	memBytes := int64(memFloat)
	memMB := memBytes / 1024 / 1024

	if memMB > 1024 {
		memGB := float64(memBytes) / 1024 / 1024 / 1024
		return fmt.Sprintf("%.1f GB", memGB)
	}
	return fmt.Sprintf("%d MB", memMB)
}

func renderTemplate(w http.ResponseWriter, r *http.Request, name string, data map[string]interface{}) {
    // Inject authentication flag
    data["IsAuthenticated"] = sessionManager.GetBool(r.Context(), "authenticated")

    // Apply translations and other localization helpers
    localizePage(w, r, data)

    // If a template name is supplied, render it and store the result in the Content field.
    // When name is empty we assume the caller already populated data["Content"].
    if name != "" {
        buf := new(bytes.Buffer)
        if err := templates.ExecuteTemplate(buf, name, data); err != nil {
            log.Error().Err(err).Msgf("Error executing page template: %s", name)
            http.Error(w, "Could not execute page template", http.StatusInternalServerError)
            return
        }
        data["Content"] = template.HTML(buf.String())
    }

    // Render the main layout which wraps whatever is present in data["Content"]
    if err := templates.ExecuteTemplate(w, "layout", data); err != nil {
        log.Error().Err(err).Msg("Error executing layout template")
        http.Error(w, "Could not execute layout template", http.StatusInternalServerError)
    }
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	log.Info().Str("path", r.URL.Path).Msg("Request received for index")
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data := make(map[string]interface{})
	renderTemplate(w, r, "index.html", data)
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	log.Info().Str("path", r.URL.Path).Msg("Request received for search")
	data := make(map[string]interface{})

	if r.Method == http.MethodPost {
		r.ParseForm()
		vmid := r.FormValue("vmid")
		name := r.FormValue("name")

		log.Info().Str("vmid", vmid).Str("name", name).Msg("Processing search request")

		apiURL := os.Getenv("PROXMOX_URL")
		apiTokenID := os.Getenv("PROXMOX_API_TOKEN_NAME")
		apiTokenSecret := os.Getenv("PROXMOX_API_TOKEN_VALUE")
		insecure := os.Getenv("PROXMOX_VERIFY_SSL") == "false"

		client, err := proxmox.NewClient(apiURL, apiTokenID, apiTokenSecret, insecure)
		if err != nil {
			log.Error().Err(err).Msg("Failed to create proxmox client")
			data["Error"] = "Failed to connect to Proxmox API"
		} else {
			results, err := searchVM(client, vmid, name)
			if err != nil {
				log.Error().Err(err).Msg("Failed to search for VM")
				data["Error"] = "Failed to retrieve VM information from Proxmox"
			} else {
				data["Results"] = results
				if vmid != "" {
					data["Query"] = vmid
				} else {
					data["Query"] = name
				}
			}
		}
	}

	renderTemplate(w, r, "search.html", data)
}

func adminHandler(w http.ResponseWriter, r *http.Request) {
	log.Info().Str("path", r.URL.Path).Msg("Request received for admin")
	data := make(map[string]interface{})

	// Read settings first, to pass them to the template even if proxmox fails
	settings, err := readSettings()
	if err != nil {
		log.Error().Err(err).Msg("Failed to read settings")
		data["Error"] = "Failed to read application settings."
		renderTemplate(w, r, "admin.html", data)
		return
	}
	data["Settings"] = settings

	apiURL := os.Getenv("PROXMOX_URL")
	apiTokenID := os.Getenv("PROXMOX_API_TOKEN_NAME")
	apiTokenSecret := os.Getenv("PROXMOX_API_TOKEN_VALUE")
	insecure := os.Getenv("PROXMOX_VERIFY_SSL") == "false"

	client, err := proxmox.NewClient(apiURL, apiTokenID, apiTokenSecret, insecure)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create proxmox client")
		data["Error"] = "Failed to connect to Proxmox API"
		renderTemplate(w, r, "admin.html", data)
		return
	}

	// Fetch all node details
	nodeNames, err := proxmox.GetNodeNames(client)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get node names")
		data["Error"] = "Failed to retrieve node list from Proxmox"
		renderTemplate(w, r, "admin.html", data)
		return
	}

	log.Info().Int("count", len(nodeNames)).Msg("Processing nodes for admin page")

	nodeDetailsList := make([]*proxmox.NodeDetails, 0)
	nodesList := make([]map[string]interface{}, 0)
	for _, nodeName := range nodeNames {
		details, err := proxmox.GetNodeDetails(client, nodeName)
		if err != nil {
			log.Error().Err(err).Str("node", nodeName).Msg("Failed to get details for node")
			continue // Skip nodes that fail
		}
		nodeDetailsList = append(nodeDetailsList, details)
		
		// Create a map for the Nodes template - convert int64 to float64 for template compatibility
		nodeMap := map[string]interface{}{
			"node": details.Node,
			"status": "online",
			"cpu": details.CPU,
			"maxcpu": float64(details.MaxCPU),
			"mem": float64(details.Memory),
			"maxmem": float64(details.MaxMemory),
			"disk": float64(details.Disk),
			"maxdisk": float64(details.MaxDisk),
		}
		nodesList = append(nodesList, nodeMap)
	}
	data["NodeDetails"] = nodeDetailsList
	data["Nodes"] = nodesList

	// Also fetch other necessary data for the admin page
	storagesResult, err := proxmox.GetStorages(client)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get storages")
	}
	data["Storages"] = storagesResult

	// Fetch all ISOs from all storages on all nodes
	allISOs := make([]map[string]interface{}, 0)
	if storagesMap, ok := storagesResult.(map[string]interface{}); ok {
		if storagesData, ok := storagesMap["data"].([]interface{}); ok {
			for _, nodeName := range nodeNames {
				for _, storage := range storagesData {
					if storageMap, ok := storage.(map[string]interface{}); ok {
						if contentType, ok := storageMap["content"].(string); ok && strings.Contains(contentType, "iso") {
							storageName := storageMap["storage"].(string)
							isos, err := proxmox.GetISOList(client, nodeName, storageName)
							if err != nil {
								log.Error().Err(err).Str("node", nodeName).Str("storage", storageName).Msg("Failed to get ISOs")
								continue
							}
							if isoData, ok := isos["data"].([]interface{}); ok {
								for _, iso := range isoData {
									if isoMap, ok := iso.(map[string]interface{}); ok {
										allISOs = append(allISOs, isoMap)
									}
								}
							}
						}
					}
				}
			}
		}
	}
	data["ISOs"] = allISOs

	allVMBRs := make([]map[string]interface{}, 0)
	for _, nodeName := range nodeNames {
		vmbrs, err := proxmox.GetVMBRs(client, nodeName)
		if err != nil {
			log.Error().Err(err).Str("node", nodeName).Msg("Failed to get VMBRs")
			continue
		}
		if vmbrData, ok := vmbrs["data"].([]interface{}); ok {
			for _, vmbr := range vmbrData {
				if vmbrMap, ok := vmbr.(map[string]interface{}); ok {
					vmbrMap["node"] = nodeName
					allVMBRs = append(allVMBRs, vmbrMap)
				}
			}
		}
	}
	data["VMBRs"] = allVMBRs

	data["Tags"] = settings.Tags

	renderTemplate(w, r, "admin.html", data)
}

func storagePageHandler(w http.ResponseWriter, r *http.Request) {
	log.Info().Str("path", r.URL.Path).Msg("Request received for storage page")
	data := make(map[string]interface{})
	renderTemplate(w, r, "storage.html", data)
}

func isoPageHandler(w http.ResponseWriter, r *http.Request) {
	log.Info().Str("path", r.URL.Path).Msg("Request received for iso page")
	data := make(map[string]interface{})
	renderTemplate(w, r, "iso.html", data)
}

func vmbrPageHandler(w http.ResponseWriter, r *http.Request) {
	log.Info().Str("path", r.URL.Path).Msg("Request received for vmbr page")
	data := make(map[string]interface{})
	renderTemplate(w, r, "vmbr.html", data)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		password := r.FormValue("password")
		adminPasswordHash := os.Getenv("ADMIN_PASSWORD_HASH")

		if adminPasswordHash == "" {
			log.Error().Msg("ADMIN_PASSWORD_HASH is not set")
			data := map[string]interface{}{"Error": "Server configuration error"}
			renderTemplate(w, r, "login.html", data)
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(adminPasswordHash), []byte(password)); err == nil {
			sessionManager.Put(r.Context(), "authenticated", true)
			http.Redirect(w, r, "/admin", http.StatusFound)
			return
		} else {
			log.Warn().Msg("Failed login attempt")
			data := map[string]interface{}{"Error": "Invalid password"}
			renderTemplate(w, r, "login.html", data)
			return
		}
	}

	renderTemplate(w, r, "login.html", make(map[string]interface{}))
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	sessionManager.Destroy(r.Context())
	http.Redirect(w, r, "/login", http.StatusFound)
}

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !sessionManager.GetBool(r.Context(), "authenticated") {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		next.ServeHTTP(w, r)
	})
}