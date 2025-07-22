package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gomarkdown/markdown"
	htmlrenderer "github.com/gomarkdown/markdown/html"
	htmltemplate "html/template"

	"pvmss/logger"
)

// serveDocHandler serves admin/user documentation as HTML with language support
func serveDocHandler(docType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get language from query parameter or default to English
		lang := r.URL.Query().Get("lang")
		if lang == "" {
			lang = "en"
		}
		
		// Validate docType for security
		if docType != "admin" && docType != "user" {
			logger.Get().Warn().
				Str("docType", docType).
				Msg("Invalid documentation type requested")
			http.Error(w, "Invalid documentation type", http.StatusBadRequest)
			return
		}

		logger.Get().Info().
			Str("handler", "serveDocHandler").
			Str("docType", docType).
			Str("lang", lang).
			Msg("Serving documentation")

		// Initialize data map with language support
		data := map[string]interface{}{"Lang": lang}
		
		// Add i18n translations
		localizePage(w, r, data)
		
		// Set current URL for navigation
		currentURL := r.URL.Path

		// Get the docs directory path with ultra-robust fallback
		var docsDir string
		
		// Try multiple path options for docs directory
		docsPaths := []string{
			"./docs",                     // Current directory
			"../docs",                    // Parent directory  
			"./backend/docs",             // From project root
			"/app/backend/docs",           // Container path
			"/app/docs",                   // Alternative container path
			"/usr/src/app/backend/docs",   // Alternative container path
			"/opt/pvmss/docs",             // Alternative container path
		}
		
		// Add runtime.Caller path
		if _, filename, _, ok := runtime.Caller(0); ok {
			dir := filepath.Dir(filename)
			docsPaths = append(docsPaths, filepath.Join(dir, "docs"))
			// Also try parent of runtime location
			parentDir := filepath.Dir(dir)
			docsPaths = append(docsPaths, filepath.Join(parentDir, "docs"))
		}
		
		// Find the first existing docs directory
		for _, path := range docsPaths {
			if info, err := os.Stat(path); err == nil && info.IsDir() {
				docsDir = path
				logger.Get().Debug().Str("docsDir", docsDir).Msg("Found docs directory")
				break
			}
		}
		
		// Ultimate fallback: create temporary docs directory with embedded content
		if docsDir == "" {
			logger.Get().Warn().Strs("searchPaths", docsPaths).Msg("No docs directory found, creating fallback")
			
			// Create temporary docs directory
			tempDocsDir := "/tmp/pvmss-docs"
			if err := os.MkdirAll(tempDocsDir, 0755); err == nil {
				// Create embedded documentation content
				if err := createFallbackDocs(tempDocsDir); err == nil {
					docsDir = tempDocsDir
					logger.Get().Info().Str("docsDir", docsDir).Msg("Created fallback docs directory")
				}
			}
		}
		
		if docsDir == "" {
			logger.Get().Error().Strs("searchPaths", docsPaths).Msg("Unable to find or create docs directory")
			data["Error"] = "Documentation system unavailable"
			renderTemplate(w, r, "error.html", data)
			return
		}

		// Build path to markdown file with language support
		mdPath := filepath.Join(docsDir, fmt.Sprintf("%s.%s.md", docType, lang))
		
		// Try fallback to English if the requested language file doesn't exist
		mdBytes, err := os.ReadFile(mdPath)
		if err != nil {
			if lang != "en" {
				logger.Get().Debug().
					Str("originalPath", mdPath).
					Msg("Language file not found, trying English fallback")
				mdPath = filepath.Join(docsDir, fmt.Sprintf("%s.en.md", docType))
				mdBytes, err = os.ReadFile(mdPath)
			}
			
			if err != nil {
				logger.Get().Error().
					Err(err).
					Str("docType", docType).
					Str("lang", lang).
					Str("path", mdPath).
					Msg("Failed to read documentation file")
				data["Error"] = "Documentation not found"
				renderTemplate(w, r, "error.html", data)
				return
			}
		}

		// Convert markdown to HTML
		renderer := htmlrenderer.NewRenderer(htmlrenderer.RendererOptions{
			Flags: htmlrenderer.CommonFlags | htmlrenderer.HrefTargetBlank,
		})
		htmlBytes := markdown.ToHTML(mdBytes, nil, renderer)
		htmlStr := string(htmlBytes)

		// Prepare template data with i18n support
		titleKey := fmt.Sprintf("Docs.%s.Title", strings.Title(docType))
		descriptionKey := fmt.Sprintf("Docs.%s.Description", strings.Title(docType))
		
		data["Title"] = data[titleKey] // Use i18n title if available
		if data["Title"] == nil {
			data["Title"] = fmt.Sprintf("%s Documentation", strings.Title(docType))
		}
		
		data["Description"] = data[descriptionKey] // Use i18n description if available  
		if data["Description"] == nil {
			data["Description"] = fmt.Sprintf("Documentation for %s features and configuration", docType)
		}
		
		data["Content"] = htmltemplate.HTML(htmlStr)
		data["CurrentURL"] = currentURL
		
		// Build language switch URLs
		baseURL := strings.Split(currentURL, "?")[0]
		data["LangEN"] = fmt.Sprintf("%s?lang=en", baseURL)
		data["LangFR"] = fmt.Sprintf("%s?lang=fr", baseURL)

		// Add authentication data if user is logged in
		if sessionManager.Exists(r.Context(), "authenticated") {
			data["IsAuthenticated"] = true
			if username := sessionManager.GetString(r.Context(), "username"); username != "" {
				data["Username"] = username
			}
		}

		// Render using the documentation template
		renderTemplate(w, r, "docs.html", data)
	}
}

// createFallbackDocs creates basic documentation files when docs directory is not found
func createFallbackDocs(docsDir string) error {
	// Embedded documentation content
	docContent := map[string]string{
		"admin.en.md": `# PVMSS Administrator Guide

## Overview

This guide covers all administrative features available in PVMSS (Proxmox Virtual Machine Self-Service).

## Administrative Features

### System Configuration
- **ISO Management**: Control which ISO images are available to users
- **Storage Management**: View and manage available storage resources
- **Network Bridges**: Configure available network bridges
- **Resource Limits**: Set limits on CPU, memory, and storage resources

### Getting Started
1. Access the admin panel at /admin (requires authentication)
2. Configure ISO images for user VM creation
3. Set appropriate resource limits
4. Monitor system logs regularly

### Security Best Practices
- Regularly update admin passwords
- Monitor user access logs
- Keep PVMSS updated
- Use HTTPS in production`,
		"admin.fr.md": `# Guide Administrateur PVMSS

## Vue d'Ensemble

Ce guide couvre toutes les fonctionnalités administratives disponibles dans PVMSS (Proxmox Virtual Machine Self-Service).

## Fonctionnalités Administratives

### Configuration Système
- **Gestion des ISO**: Contrôler quelles images ISO sont disponibles aux utilisateurs
- **Gestion du Stockage**: Voir et gérer les ressources de stockage disponibles
- **Ponts Réseau**: Configurer les ponts réseau disponibles
- **Limites de Ressources**: Définir des limites sur les ressources CPU, mémoire et stockage

### Guide de Démarrage
1. Accéder au panneau d'administration sur /admin (authentification requise)
2. Configurer les images ISO pour la création de VM utilisateur
3. Définir des limites de ressources appropriées
4. Surveiller régulièrement les logs système

### Bonnes Pratiques de Sécurité
- Mettre à jour régulièrement les mots de passe administrateur
- Surveiller les logs d'accès utilisateur
- Maintenir PVMSS à jour
- Utiliser HTTPS en production`,
		"user.en.md": `# PVMSS User Guide

## Getting Started

PVMSS (Proxmox Virtual Machine Self-Service) allows you to create and manage virtual machines through a simple web interface.

## Key Features

### Creating Virtual Machines
- **Simple Creation Process**: Use the intuitive web interface
- **Pre-configured Templates**: Choose from available OS templates and ISO images
- **Resource Selection**: Specify CPU, memory, and storage requirements

### Managing Your VMs
- **VM Search**: Find your VMs by name or ID
- **VM Control**: Start, stop, restart, and shutdown your virtual machines
- **Status Monitoring**: View real-time status and resource usage

### Quick Start Guide
1. **Search for VMs**: Use the search page to find existing virtual machines
2. **Create New VM**: Click on "Create VM" to start the creation process
3. **Configure VM**: Select OS template and set resource limits
4. **Monitor VMs**: Use the VM details page to monitor and manage your machines

### Best Practices
- Always properly shutdown VMs before stopping them
- Monitor resource usage for optimal performance
- Use descriptive names for easy identification`,
		"user.fr.md": `# Guide Utilisateur PVMSS

## Démarrage

PVMSS (Proxmox Virtual Machine Self-Service) vous permet de créer et gérer des machines virtuelles via une interface web simple.

## Fonctionnalités Principales

### Création de Machines Virtuelles
- **Processus simplifié**: Interface web intuitive
- **Modèles préconfigurés**: Choix parmi les images ISO et modèles OS disponibles
- **Sélection des ressources**: Définir les besoins en CPU, mémoire et stockage

### Gestion de vos VM
- **Recherche de VM**: Trouvez vos machines par nom ou ID
- **Contrôle des VM**: Démarrer, arrêter, redémarrer et éteindre vos machines virtuelles
- **Surveillance du statut**: Voir l'état en temps réel et l'utilisation des ressources

### Guide de Démarrage Rapide
1. **Rechercher des VM**: Utilisez la page de recherche pour trouver les machines virtuelles existantes
2. **Créer une nouvelle VM**: Cliquez sur "Créer VM" pour démarrer le processus
3. **Configurer la VM**: Sélectionnez le modèle OS et définissez les limites de ressources
4. **Surveiller les VM**: Utilisez la page de détails pour surveiller et gérer vos machines

### Bonnes Pratiques
- Toujours éteindre proprement les VM avant de les arrêter
- Surveiller l'utilisation des ressources pour des performances optimales
- Utiliser des noms descriptifs pour faciliter l'identification`,
	}

	// Create each documentation file
	for filename, content := range docContent {
		filePath := filepath.Join(docsDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			logger.Get().Error().Err(err).Str("file", filename).Msg("Failed to create fallback doc file")
			return err
		}
		logger.Get().Debug().Str("file", filePath).Msg("Created fallback doc file")
	}

	logger.Get().Info().Str("docsDir", docsDir).Int("filesCreated", len(docContent)).Msg("Successfully created fallback documentation")
	return nil
}
