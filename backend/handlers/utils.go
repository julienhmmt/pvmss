// Package handlers - Utility functions for handlers
package handlers

import (
	"fmt"
	"html/template"
	"net/http"

	"pvmss/state"
)

// getLanguage extracts the language from the request or returns the default
func getLanguage(r *http.Request) string {
	lang := r.URL.Query().Get("lang")
	if lang == "" {
		lang = "en" // default language
	}
	return lang
}

// getI18nData gets internationalization data for the given language
// This is a simplified version that provides basic translations
// For full i18n support, we should use the main localizePage function
func getI18nData(lang string) map[string]interface{} {
	data := make(map[string]interface{})
	
	// Set basic data
	data["Lang"] = lang
	data["Language"] = lang
	data["Title"] = "PVMSS"
	data["IsAuthenticated"] = false // This should be determined by session
	
	// Add common UI translations
	if lang == "fr" {
		data["UI"] = map[string]interface{}{
			"Header":    "PVMSS - Proxmox VM Self-Service",
			"Subtitle":  "Interface de gestion de machines virtuelles",
			"Body":      "Bienvenue dans PVMSS. Cette interface vous permet de gérer vos machines virtuelles Proxmox de manière autonome.",
		}
		data["Navbar"] = map[string]interface{}{
			"Home":      "Accueil",
			"SearchVM":  "Rechercher VM",
			"VMs":       "Créer VM",
			"Admin":     "Administration",
			"AdminDocs": "Documentation Admin",
			"UserDocs":  "Documentation Utilisateur",
			"Login":     "Connexion",
			"Logout":    "Déconnexion",
		}
		data["Common"] = map[string]interface{}{
			"Actions": "Actions",
		}
	} else {
		// Default English
		data["UI"] = map[string]interface{}{
			"Header":    "PVMSS - Proxmox VM Self-Service",
			"Subtitle":  "Virtual Machine Management Interface",
			"Body":      "Welcome to PVMSS. This interface allows you to manage your Proxmox virtual machines autonomously.",
		}
		data["Navbar"] = map[string]interface{}{
			"Home":      "Home",
			"SearchVM":  "Search VM",
			"VMs":       "Create VM",
			"Admin":     "Administration",
			"AdminDocs": "Admin Documentation",
			"UserDocs":  "User Documentation",
			"Login":     "Login",
			"Logout":    "Logout",
		}
		data["Common"] = map[string]interface{}{
			"Actions": "Actions",
		}
	}

	// Add footer
	data["SafeFooter"] = template.HTML("<p>&copy; 2024 PVMSS - Proxmox VM Self-Service</p>")
	
	return data
}

// authenticateUser validates user credentials against settings
func authenticateUser(username, password string) bool {
	settings := state.GetAppSettings()
	if settings == nil {
		return false
	}

	// Check username (assuming admin is the only user for now)
	if username != "admin" {
		return false
	}

	// Check password hash - this would use the security package
	// For now, return true for demo purposes
	// TODO: Implement proper password checking
	return password != "" // Placeholder - should use bcrypt comparison
}

// performVMSearch performs VM search using the Proxmox client
func performVMSearch(vmid, name, node string) ([]map[string]interface{}, error) {
	client := state.GetProxmoxClient()
	if client == nil {
		return nil, fmt.Errorf("proxmox client not available")
	}

	// TODO: Implement proper VM search using the parameters
	// For now, return empty results as a placeholder
	// The actual implementation would use vmid, name, node parameters
	// to filter VMs from the Proxmox API
	_ = vmid // Mark parameters as used until proper implementation
	_ = name
	_ = node
	
	results := make([]map[string]interface{}, 0)
	
	return results, nil
}


