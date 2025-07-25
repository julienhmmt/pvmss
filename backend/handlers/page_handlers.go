// Package handlers - Page-specific HTTP handlers
package handlers

import (
	"bytes"
	"html/template"
	"net/http"

	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/state"
)

// StoragePageHandler handles storage management page
func StoragePageHandler(w http.ResponseWriter, r *http.Request) {
	lang := getLanguage(r)
	data := getI18nData(lang)

	// Get storage information
	client := state.GetProxmoxClient()
	if client != nil {
		storage, err := proxmox.GetStorages(client)
		if err != nil {
			logger.Get().Error().Err(err).Msg("Error getting storage for page")
		} else {
			data["Storage"] = storage
		}
	}

	// Create a buffer to capture the template output
	var buf bytes.Buffer

	tmpl := state.GetTemplates()
	if tmpl == nil {
		logger.Get().Error().Msg("Templates not initialized")
		http.Error(w, "Templates not initialized", http.StatusInternalServerError)
		return
	}

	// Execute the storage template
	if err := tmpl.ExecuteTemplate(&buf, "storage.html", data); err != nil {
		logger.Get().Error().Err(err).Msg("Error rendering storage template")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Add the rendered template to the data map as SafeContent (expected by layout)
	data["SafeContent"] = template.HTML(buf.String())

	// Execute the layout template with the content
	if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		logger.Get().Error().Err(err).Msg("Error executing layout template")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// IsoPageHandler handles ISO management page
func IsoPageHandler(w http.ResponseWriter, r *http.Request) {
	lang := getLanguage(r)
	data := getI18nData(lang)

	// Get ISO information - simplified for now
	client := state.GetProxmoxClient()
	if client != nil {
		// In a real implementation, you'd iterate through nodes and storages
		data["ISOs"] = []interface{}{} // Empty for now
	}

	// Create a buffer to capture the template output
	var buf bytes.Buffer

	tmpl := state.GetTemplates()
	if tmpl == nil {
		logger.Get().Error().Msg("Templates not initialized")
		http.Error(w, "Templates not initialized", http.StatusInternalServerError)
		return
	}

	// Execute the ISO template
	if err := tmpl.ExecuteTemplate(&buf, "iso.html", data); err != nil {
		logger.Get().Error().Err(err).Msg("Error rendering ISO template")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Add the rendered template to the data map as SafeContent (expected by layout)
	data["SafeContent"] = template.HTML(buf.String())

	// Execute the layout template with the content
	if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		logger.Get().Error().Err(err).Msg("Error executing layout template")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// VmbrPageHandler handles VMBR management page
func VmbrPageHandler(w http.ResponseWriter, r *http.Request) {
	lang := getLanguage(r)
	data := getI18nData(lang)

	// Get VMBR information - simplified for now
	client := state.GetProxmoxClient()
	if client != nil {
		// In a real implementation, you'd iterate through nodes
		data["VMBRs"] = []interface{}{} // Empty for now
	}

	// Create a buffer to capture the template output
	var buf bytes.Buffer

	tmpl := state.GetTemplates()
	if tmpl == nil {
		logger.Get().Error().Msg("Templates not initialized")
		http.Error(w, "Templates not initialized", http.StatusInternalServerError)
		return
	}

	// Execute the VMBR template
	if err := tmpl.ExecuteTemplate(&buf, "vmbr.html", data); err != nil {
		logger.Get().Error().Err(err).Msg("Error rendering VMBR template")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Add the rendered template to the data map as SafeContent (expected by layout)
	data["SafeContent"] = template.HTML(buf.String())

	// Execute the layout template with the content
	if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		logger.Get().Error().Err(err).Msg("Error executing layout template")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// DocsHandler handles documentation page requests
func DocsHandler(w http.ResponseWriter, r *http.Request, docType, lang string) {
	data := getI18nData(lang)
	
	// Add documentation-specific data
	data["DocType"] = docType
	data["Language"] = lang

	// Create a buffer to capture the template output
	var buf bytes.Buffer

	tmpl := state.GetTemplates()
	if tmpl == nil {
		logger.Get().Error().Msg("Templates not initialized")
		http.Error(w, "Templates not initialized", http.StatusInternalServerError)
		return
	}

	// Execute the docs template
	if err := tmpl.ExecuteTemplate(&buf, "docs.html", data); err != nil {
		logger.Get().Error().Err(err).Msg("Error rendering documentation template")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Add the rendered template to the data map as SafeContent (expected by layout)
	data["SafeContent"] = template.HTML(buf.String())

	// Execute the layout template with the content
	if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		logger.Get().Error().Err(err).Msg("Error executing layout template")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
