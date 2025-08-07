package handlers

import (
	"net/http"
	"sort"
	"strings"

	"github.com/julienschmidt/httprouter"

	"pvmss/i18n"
	"pvmss/logger"
	"pvmss/state"
)

// VMHandler gère les routes liées aux machines virtuelles
type VMHandler struct {
	stateManager state.StateManager
}

// NewVMHandler crée une nouvelle instance de VMHandler
func NewVMHandler(stateManager state.StateManager) *VMHandler {
	return &VMHandler{
		stateManager: stateManager,
	}
}

// IndexHandler gère la page d'accueil avec la liste des VMs
func (h *VMHandler) IndexHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := logger.Get().With().
		Str("handler", "VMHandler").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Logger()

	log.Debug().Msg("Début du traitement de la requête IndexHandler")

	// Récupérer la liste des VMs (mock temporaire)
	vms := []map[string]interface{}{
		{"id": 100, "name": "vm1", "status": "running"},
		{"id": 101, "name": "vm2", "status": "stopped"},
	}

	log.Debug().Int("vm_count", len(vms)).Msg("Liste des VMs récupérée")

	// Préparer les données pour le template
	data := map[string]interface{}{
		"Title": "Liste des machines virtuelles",
		"VMs":   vms,
	}

	// Ajouter les données de traduction
	i18n.LocalizePage(w, r, data)

	log.Debug().Msg("Rendu du template index.html")
	renderTemplateInternal(w, r, "index.html", data)
}

// VMDetailsHandler gère la page de détails d'une VM
func (h *VMHandler) VMDetailsHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	log := logger.Get().With().
		Str("handler", "VMHandler").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Logger()

	vmID := ps.ByName("id")
	if vmID == "" {
		log.Warn().Msg("ID de VM non spécifié dans la requête")
		http.Error(w, "VM ID is required", http.StatusBadRequest)
		return
	}

	log = log.With().Str("vm_id", vmID).Logger()
	log.Debug().Msg("Récupération des détails de la VM")

	// Récupérer les détails de la VM depuis Proxmox
	// À implémenter: Récupérer les vrais détails de la VM
	vmDetails := map[string]interface{}{
		"ID":      vmID,
		"Name":    "VM " + vmID,
		"Status":  "running",
		"CPUs":    2,
		"Memory":  "4GB",
		"Storage": "50GB",
	}

	log.Debug().Interface("vm_details", vmDetails).Msg("Détails de la VM récupérés")

	data := map[string]interface{}{
		"Title": "Détails de la VM " + vmID,
		"VM":    vmDetails,
	}

	// Ajouter les données de traduction
	i18n.LocalizePage(w, r, data)

	log.Debug().Msg("Rendu du template vm_details.html")
	renderTemplateInternal(w, r, "vm_details.html", data)
}

// CreateVMHandler gère la création d'une nouvelle VM
func (h *VMHandler) CreateVMHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := logger.Get().With().
		Str("handler", "VMHandler").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Logger()

	log.Debug().Str("http_method", r.Method).Msg("Traitement de la requête CreateVMHandler")

	// Vérifier si Proxmox est connecté
	connected, _ := h.stateManager.GetProxmoxStatus()
	if !connected {
		log.Warn().Msg("Tentative d'accès à la création de VM alors que Proxmox n'est pas connecté")
		http.Redirect(w, r, "/?error=proxmox_disconnected", http.StatusSeeOther)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.renderCreateVMForm(w, r)
	case http.MethodPost:
		h.handleCreateVM(w, r)
	default:
		log.Warn().Str("method", r.Method).Msg("Méthode HTTP non autorisée")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// renderCreateVMForm displays the VM creation form
func (h *VMHandler) renderCreateVMForm(w http.ResponseWriter, r *http.Request) {
	log := logger.Get().With().
		Str("handler", "VMHandler").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Logger()

	log.Debug().Msg("Displaying VM creation form")

	// Get Proxmox connection status
	connected, errMsg := h.stateManager.GetProxmoxStatus()
	if !connected {
		log.Warn().Str("error", errMsg).Msg("Proxmox is not connected, showing disabled form")
	}

	// Get the global state
	stateMgr := state.GetGlobalState()
	if stateMgr == nil {
		log.Error().Msg("State manager not initialized")
		http.Error(w, "Internal server error: state not initialized", http.StatusInternalServerError)
		return
	}

	// Get settings
	settings := stateMgr.GetSettings()
	if settings == nil {
		log.Error().Msg("Settings not available")
		http.Error(w, "Settings not available", http.StatusInternalServerError)
		return
	}

	log.Debug().
		Int("total_isos", len(settings.ISOs)).
		Int("total_vmbrs", len(settings.VMBRs)).
		Msg("Loaded VM creation settings")

	// Create ISO items with proper path and display name
	type isoItem struct {
		Path string `json:"path"`
		Name string `json:"name"`
	}

	var isos []isoItem
	for _, path := range settings.ISOs {
		if path == "" {
			continue
		}

		// Extract filename from path
		name := path
		if lastSlash := strings.LastIndex(path, "/"); lastSlash >= 0 {
			name = path[lastSlash+1:]
		}

		isos = append(isos, isoItem{
			Path: path,
			Name: name,
		})

		log.Debug().
			Str("path", path).
			Str("name", name).
			Msg("Added ISO to selection")
	}

	// Get network bridges
	bridges := settings.VMBRs
	if len(bridges) == 0 {
		bridges = []string{"vmbr0"}
		log.Warn().Msg("No network bridges found in settings, using default 'vmbr0'")
	}

	// Get and sort tags
	tags := settings.Tags
	sort.Strings(tags) // Sort tags alphabetically

	// Prepare template data
	data := make(map[string]interface{})
	data["ISOs"] = isos
	data["Bridges"] = bridges
	data["AvailableTags"] = tags
	data["ProxmoxConnected"] = connected
	data["ProxmoxError"] = errMsg

	// Add translation data
	i18n.LocalizePage(w, r, data)
	data["Title"] = data["VM.Create.Title"]

	log.Debug().
		Int("isos_count", len(isos)).
		Int("bridges_count", len(bridges)).
		Msg("Rendering VM creation form")

	renderTemplateInternal(w, r, "create_vm", data)
}

// handleCreateVM traite la soumission du formulaire de création de VM
func (h *VMHandler) handleCreateVM(w http.ResponseWriter, r *http.Request) {
	log := logger.Get().With().
		Str("handler", "VMHandler").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Logger()

	log.Debug().Msg("Traitement de la soumission du formulaire de création de VM")

	// Parser le formulaire
	if err := r.ParseForm(); err != nil {
		log.Error().Err(err).Msg("Échec de l'analyse du formulaire")
		h.renderCreateVMFormWithError(w, r, "Erreur lors de l'analyse du formulaire")
		return
	}

	vmName := r.FormValue("name")
	node := r.FormValue("node")
	image := r.FormValue("image")

	log = log.With().
		Str("vm_name", vmName).
		Str("node", node).
		Str("image", image).
		Logger()

	log.Info().Msg("Tentative de création d'une nouvelle VM")

	// Valider les entrées
	if vmName == "" || node == "" || image == "" {
		h.renderCreateVMFormWithError(w, r, "Tous les champs sont obligatoires")
		return
	}

	// Créer la VM dans Proxmox
	// À implémenter: Appeler l'API Proxmox pour créer la VM

	// Rediriger vers la page de détails de la VM
	http.Redirect(w, r, "/vm/details/123", http.StatusSeeOther) // Remplacer 123 par l'ID de la VM créée
}

// renderCreateVMFormWithError affiche le formulaire avec un message d'erreur
func (h *VMHandler) renderCreateVMFormWithError(w http.ResponseWriter, r *http.Request, errorMessage string) {
	nodes := []string{"node1", "node2"}
	images := []string{"ubuntu-20.04", "debian-11"}

	data := map[string]interface{}{
		"Title":        "Créer une nouvelle VM",
		"Nodes":        nodes,
		"Images":       images,
		"ErrorMessage": errorMessage,
	}

	renderTemplateInternal(w, r, "create_vm.html", data)
}

// RegisterRoutes enregistre les routes liées aux VMs
func (h *VMHandler) RegisterRoutes(router *httprouter.Router) {
	// Route de création de VM (publique)
	router.GET("/vm/create", HandlerFuncToHTTPrHandle(func(w http.ResponseWriter, r *http.Request) {
		h.CreateVMHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	}))
	router.POST("/vm/create", HandlerFuncToHTTPrHandle(func(w http.ResponseWriter, r *http.Request) {
		h.CreateVMHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	}))

	// Détails de la VM (protégé par authentification)
	router.GET("/vm/details/:id", HandlerFuncToHTTPrHandle(func(w http.ResponseWriter, r *http.Request) {
		h.VMDetailsHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	}))
}

// VMDetailsHandlerFunc est une fonction wrapper pour la compatibilité avec le code existant
func VMDetailsHandlerFunc(w http.ResponseWriter, r *http.Request) {
	vmid := r.URL.Query().Get("vmid")
	if vmid == "" {
		http.Error(w, "VM ID required", http.StatusBadRequest)
		return
	}

	logger.Get().Info().Str("vmid", vmid).Msg("VM details request")

	// TODO: Get actual VM details from Proxmox
	data := map[string]interface{}{
		"VM": map[string]string{
			"ID":     vmid,
			"Name":   "Sample VM",
			"Status": "running",
		},
	}

	renderTemplateInternal(w, r, "vm_details", data)
}

// CreateVMHandlerFunc est une fonction wrapper pour la compatibilité avec le code existant
func CreateVMHandlerFunc(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		renderTemplateInternal(w, r, "create_vm", nil)
		return
	}

	if r.Method == http.MethodPost {
		name := r.FormValue("name")
		logger.Get().Info().Str("name", name).Msg("VM creation request")

		data := map[string]interface{}{
			"Success": "VM creation initiated",
		}
		renderTemplateInternal(w, r, "create_vm", data)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}
