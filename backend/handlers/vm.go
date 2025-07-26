package handlers

import (
	"net/http"

	"github.com/julienschmidt/httprouter"

	"pvmss/i18n"
	"pvmss/logger"
)

// VMHandler gère les routes liées aux machines virtuelles
type VMHandler struct{}

// NewVMHandler crée une nouvelle instance de VMHandler
func NewVMHandler() *VMHandler {
	return &VMHandler{}
}

// IndexHandler gère la page d'accueil avec la liste des VMs
func (h *VMHandler) IndexHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// Récupérer la liste des VMs (mock temporaire)
	vms := []map[string]interface{}{
		{"id": 100, "name": "vm1", "status": "running"},
		{"id": 101, "name": "vm2", "status": "stopped"},
	}

	// Préparer les données pour le template
	data := map[string]interface{}{
		"Title": "Liste des machines virtuelles",
		"VMs":   vms,
	}

	renderTemplateInternal(w, r, "index.html", data)
}

// VMDetailsHandler gère la page de détails d'une VM
func (h *VMHandler) VMDetailsHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	vmID := ps.ByName("id")
	if vmID == "" {
		http.Error(w, "VM ID is required", http.StatusBadRequest)
		return
	}

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

	data := map[string]interface{}{
		"Title": "Détails de la VM " + vmID,
		"VM":    vmDetails,
	}

	renderTemplateInternal(w, r, "vm_details.html", data)
}

// CreateVMHandler gère la création d'une nouvelle VM
func (h *VMHandler) CreateVMHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	switch r.Method {
	case http.MethodGet:
		h.renderCreateVMForm(w, r)
	case http.MethodPost:
		h.handleCreateVM(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// renderCreateVMForm affiche le formulaire de création de VM
func (h *VMHandler) renderCreateVMForm(w http.ResponseWriter, r *http.Request) {
	// Récupérer les données nécessaires pour le formulaire
	nodes := []string{"node1", "node2"}             // À remplacer par les vrais nœuds
	images := []string{"ubuntu-20.04", "debian-11"} // À remplacer par les vraies images

	// Créer les données pour le template
	data := make(map[string]interface{})
	data["Nodes"] = nodes
	data["Images"] = images

	// Ajouter les données de traduction
	i18n.LocalizePage(w, r, data)

	// Définir le titre de la page
	data["Title"] = data["VM.Create.Title"]

	// Rendre le template avec le bon nom (sans extension .html)
	renderTemplateInternal(w, r, "create_vm", data)
}

// handleCreateVM traite la soumission du formulaire de création de VM
func (h *VMHandler) handleCreateVM(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	vmName := r.FormValue("name")
	node := r.FormValue("node")
	image := r.FormValue("image")

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
