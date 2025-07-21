package main

import (
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/rs/zerolog/log"
	"golang.org/x/text/language"
)

// Bundle is a global variable that holds all loaded translation data.
// It is used by localizers to find and format translated strings.
var Bundle *i18n.Bundle

// getLanguage determines the user's preferred language by checking (in order):
// 1. A 'lang' URL query parameter.
// 2. A 'pvmss_lang' cookie.
// 3. The function defaults to English ('en') if no preference is found.
func getLanguage(r *http.Request) string {
	// Check URL parameter first
	if lang := r.URL.Query().Get("lang"); lang == "fr" || lang == "en" {
		return lang
	}
	// Check cookie
	if cookie, err := r.Cookie("pvmss_lang"); err == nil && (cookie.Value == "fr" || cookie.Value == "en") {
		return cookie.Value
	}
	// Default to English
	return "en"
}

// InitI18n initializes the internationalization bundle.
// It sets up English as the default language, registers the TOML unmarshal function,
// and loads the English and French translation files from the 'i18n' directory.
// loadTranslationFile tente de charger un fichier de traduction depuis plusieurs emplacements possibles
func loadTranslationFile(bundle *i18n.Bundle, filename string) {
	// Liste des emplacements à essayer
	possiblePaths := []string{
		// Emplacement standard (pour l'exécution locale)
		filepath.Join("i18n", filename),
		// Emplacement dans le conteneur
		filepath.Join("/app", "i18n", filename),
		// Emplacement parent (si exécuté depuis le dossier backend)
		filepath.Join("..", "i18n", filename),
	}

	var loaded bool
	var lastError error

	for _, path := range possiblePaths {
		absPath, _ := filepath.Abs(path)
		log.Debug().Str("path", absPath).Msgf("Tentative de chargement du fichier de traduction: %s", filename)

		if _, err := os.Stat(path); err == nil {
			if _, err := bundle.LoadMessageFile(path); err == nil {
				log.Info().Str("file", absPath).Msg("Fichier de traduction chargé avec succès")
				loaded = true
				break
			} else {
				lastError = err
				log.Warn().Err(err).Str("path", absPath).Msg("Erreur lors du chargement du fichier de traduction")
			}
		} else {
			log.Debug().Str("path", absPath).Msg("Fichier de traduction introuvable à cet emplacement")
		}
	}

	if !loaded {
		log.Error().
			Str("file", filename).
			Err(lastError).
			Msg("Impossible de charger le fichier de traduction depuis aucun des emplacements connus")
	}
}

func InitI18n() {
	// Créer un nouveau bundle avec l'anglais comme langue par défaut
	Bundle = i18n.NewBundle(language.English)
	Bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)

	// Charger les traductions anglaises
	loadTranslationFile(Bundle, "active.en.toml")

	// Charger les traductions françaises
	loadTranslationFile(Bundle, "active.fr.toml")

	// Vérifier que les fichiers ont été chargés en affichant une clé de test
	enLocalizer := i18n.NewLocalizer(Bundle, "en")
	_, err := enLocalizer.Localize(&i18n.LocalizeConfig{
		MessageID: "Navbar.Home",
	})
	if err != nil {
		log.Error().Err(err).Msg("Erreur lors de la vérification de la clé 'Navbar.Home' en anglais")
	}

	frLocalizer := i18n.NewLocalizer(Bundle, "fr")
	_, err = frLocalizer.Localize(&i18n.LocalizeConfig{
		MessageID: "Navbar.Home",
	})
	if err != nil {
		log.Error().Err(err).Msg("Erreur lors de la vérification de la clé 'Navbar.Home' en français")
	}
}

// safeLocalize provides a panic-safe way to get a translated string.
// If a translation key is not found for the given language, it logs a warning
// and returns the message ID itself, preventing the application from crashing.
func safeLocalize(localizer *i18n.Localizer, messageID string) string {
	localized, err := localizer.Localize(&i18n.LocalizeConfig{MessageID: messageID})
	if err != nil {
		log.Warn().Err(err).Str("MessageID", messageID).Msg("Missing translation")
		return messageID // Fallback to the key itself
	}
	return localized
}

// localizePage injects all necessary localized strings into the data map for a given request.
// This function is called by renderTemplate before executing a template to ensure all UI text is translated.
// It uses safeLocalize to prevent panics on missing keys.
func localizePage(w http.ResponseWriter, r *http.Request, data map[string]interface{}) {
	lang := r.URL.Query().Get("lang")
	if lang != "" {
		http.SetCookie(w, &http.Cookie{
			Name:    "pvmss_lang",
			Value:   lang,
			Path:    "/",
			Expires: time.Now().Add(365 * 24 * time.Hour),
		})
	} else {
		cookie, err := r.Cookie("pvmss_lang")
		if err == nil {
			lang = cookie.Value
		}
	}

	if lang == "" {
		lang = r.Header.Get("Accept-Language")
	}

	localizer := i18n.NewLocalizer(Bundle, lang)

	// Common
	data["Common.Cancel"] = safeLocalize(localizer, "Common.Cancel")
	data["Common.Error"] = safeLocalize(localizer, "Common.Error")
	data["Common.Reset"] = safeLocalize(localizer, "Common.Reset")
	data["Common.Save"] = safeLocalize(localizer, "Common.Save")
	data["Common.Saved"] = safeLocalize(localizer, "Common.Saved")
	data["Common.Saving"] = safeLocalize(localizer, "Common.Saving")
	data["Common.Min"] = safeLocalize(localizer, "Common.Min")
	data["Common.Max"] = safeLocalize(localizer, "Common.Max")

	// General UI
	data["Title"] = safeLocalize(localizer, "Title")
	data["UI.Body"] = safeLocalize(localizer, "UI.Body")
	data["UI.Button.Search"] = safeLocalize(localizer, "UI.Button.Search")
	data["UI.Footer"] = safeLocalize(localizer, "UI.Footer")
	data["UI.Header"] = safeLocalize(localizer, "UI.Header")
	data["UI.Subtitle"] = safeLocalize(localizer, "UI.Subtitle")

	// Navbar
	data["Navbar.Admin"] = safeLocalize(localizer, "Navbar.Admin")
	data["Navbar.AdminDocs"] = safeLocalize(localizer, "Navbar.AdminDocs")
	data["Navbar.Home"] = safeLocalize(localizer, "Navbar.Home")
	data["Navbar.Login"] = safeLocalize(localizer, "Navbar.Login")
	data["Navbar.Logout"] = safeLocalize(localizer, "Navbar.Logout")
	data["Navbar.SearchVM"] = safeLocalize(localizer, "Navbar.SearchVM")
	data["Navbar.UserDocs"] = safeLocalize(localizer, "Navbar.UserDocs")
	data["Navbar.VMs"] = safeLocalize(localizer, "Navbar.VMs")

	// Login Page
	data["Login.Title"] = safeLocalize(localizer, "Login.Title")
	data["Login.PasswordLabel"] = safeLocalize(localizer, "Login.PasswordLabel")
	data["Login.Button"] = safeLocalize(localizer, "Login.Button")

	// Search Page
	data["Search.ActionsHeader"] = safeLocalize(localizer, "Search.ActionsHeader")
	data["Search.CPUs"] = safeLocalize(localizer, "Search.CPUs")
	data["Search.Memory"] = safeLocalize(localizer, "Search.Memory")
	data["Search.Name"] = safeLocalize(localizer, "Search.Name")
	data["Search.NoResults"] = safeLocalize(localizer, "Search.NoResults")
	data["Search.PlaceholderName"] = safeLocalize(localizer, "Search.PlaceholderName")
	data["Search.PlaceholderVMID"] = safeLocalize(localizer, "Search.PlaceholderVMID")
	data["Search.Results"] = safeLocalize(localizer, "Search.Results")
	data["Search.Status"] = safeLocalize(localizer, "Search.Status")
	data["Search.TitleVMID"] = safeLocalize(localizer, "Search.TitleVMID")
	data["Search.VMDetailsButton"] = safeLocalize(localizer, "Search.VMDetailsButton")
	data["Search.VMID"] = safeLocalize(localizer, "Search.VMID")
	data["Search.YouSearchedFor"] = safeLocalize(localizer, "Search.YouSearchedFor")

	// Nodes Page
	data["Nodes.Title"] = safeLocalize(localizer, "Nodes.Title")
	data["Nodes.NoNodes"] = safeLocalize(localizer, "Nodes.NoNodes")
	data["Nodes.Header.Node"] = safeLocalize(localizer, "Nodes.Header.Node")
	data["Nodes.Header.Status"] = safeLocalize(localizer, "Nodes.Header.Status")
	data["Nodes.Header.CPUUsage"] = safeLocalize(localizer, "Nodes.Header.CPUUsage")
	data["Nodes.Header.MemoryUsage"] = safeLocalize(localizer, "Nodes.Header.MemoryUsage")
	data["Nodes.Header.DiskUsage"] = safeLocalize(localizer, "Nodes.Header.DiskUsage")
	data["Nodes.Status.Online"] = safeLocalize(localizer, "Nodes.Status.Online")
	data["Nodes.Status.Offline"] = safeLocalize(localizer, "Nodes.Status.Offline")

	// VM Creation Page
	data["VM.Create.Title"] = safeLocalize(localizer, "VM.Create.Title")
	data["VM.Create.Header"] = safeLocalize(localizer, "VM.Create.Header")
	data["VM.Create.NoISOs"] = safeLocalize(localizer, "VM.Create.NoISOs")
	data["VM.Create.NoBridges"] = safeLocalize(localizer, "VM.Create.NoBridges")
	data["VM.Create.Tags"] = safeLocalize(localizer, "VM.Create.Tags")
	data["VM.Create.TagsHelp"] = safeLocalize(localizer, "VM.Create.TagsHelp")
	data["VM.Create.VMName"] = safeLocalize(localizer, "VM.Create.VMName")
	data["VM.Create.VMNamePlaceholder"] = safeLocalize(localizer, "VM.Create.VMNamePlaceholder")
	data["VM.Create.VMNameHelp"] = safeLocalize(localizer, "VM.Create.VMNameHelp")
	data["VM.Create.Description"] = safeLocalize(localizer, "VM.Create.Description")
	data["VM.Create.DescriptionPlaceholder"] = safeLocalize(localizer, "VM.Create.DescriptionPlaceholder")
	data["VM.Create.VMID"] = safeLocalize(localizer, "VM.Create.VMID")
	data["VM.Create.VMIDPlaceholder"] = safeLocalize(localizer, "VM.Create.VMIDPlaceholder")
	data["VM.Create.VMID.Help"] = safeLocalize(localizer, "VM.Create.VMID.Help")
	data["VM.Create.CPUSockets"] = safeLocalize(localizer, "VM.Create.CPUSockets")
	data["VM.Create.CPUCores"] = safeLocalize(localizer, "VM.Create.CPUCores")
	data["VM.Create.Memory"] = safeLocalize(localizer, "VM.Create.Memory")
	data["VM.Create.Storage"] = safeLocalize(localizer, "VM.Create.Storage")
	data["VM.Create.ISO"] = safeLocalize(localizer, "VM.Create.ISO")
	data["VM.Create.SelectISO"] = safeLocalize(localizer, "VM.Create.SelectISO")
	data["VM.Create.Network"] = safeLocalize(localizer, "VM.Create.Network")
	data["VM.Create.SelectNetwork"] = safeLocalize(localizer, "VM.Create.SelectNetwork")
	data["VM.Create.CreateButton"] = safeLocalize(localizer, "VM.Create.CreateButton")
	data["VM.Create.ResetButton"] = safeLocalize(localizer, "VM.Create.ResetButton")
	data["VM.Bridge.Description"] = safeLocalize(localizer, "VM.Bridge.Description")

	// VM Details Page
	data["VMDetails.Action.Refresh"] = safeLocalize(localizer, "VMDetails.Action.Refresh")

	// Admin - ISO Management
	data["Admin.ISO.Title"] = safeLocalize(localizer, "Admin.ISO.Title")
	data["Admin.ISO.Description"] = safeLocalize(localizer, "Admin.ISO.Description")
	data["Admin.ISO.Header.Enabled"] = safeLocalize(localizer, "Admin.ISO.Header.Enabled")
	data["Admin.ISO.Header.Name"] = safeLocalize(localizer, "Admin.ISO.Header.Name")
	data["Admin.ISO.Header.Size"] = safeLocalize(localizer, "Admin.ISO.Header.Size")
	data["Admin.ISO.Header.Storage"] = safeLocalize(localizer, "Admin.ISO.Header.Storage")

	// Admin - Storage Management
	data["Admin.Storage.Title"] = safeLocalize(localizer, "Admin.Storage.Title")
	data["Admin.Storage.Description"] = safeLocalize(localizer, "Admin.Storage.Description")
	data["Admin.Storage.NoStorages"] = safeLocalize(localizer, "Admin.Storage.NoStorages")
	data["Admin.Storage.Header.Name"] = safeLocalize(localizer, "Admin.Storage.Header.Name")
	data["Admin.Storage.Header.Type"] = safeLocalize(localizer, "Admin.Storage.Header.Type")
	data["Admin.Storage.Header.Content"] = safeLocalize(localizer, "Admin.Storage.Header.Content")
	data["Admin.Storage.Error"] = safeLocalize(localizer, "Admin.Storage.Error")

	// Admin - VMBR Management
	data["Admin.VMBR.Title"] = safeLocalize(localizer, "Admin.VMBR.Title")
	data["Admin.VMBR.Description"] = safeLocalize(localizer, "Admin.VMBR.Description")
	data["Admin.VMBR.NoVMBRs"] = safeLocalize(localizer, "Admin.VMBR.NoVMBRs")
	data["Admin.VMBR.Header.Name"] = safeLocalize(localizer, "Admin.VMBR.Header.Name")
	data["Admin.VMBR.Header.Node"] = safeLocalize(localizer, "Admin.VMBR.Header.Node")
	data["Admin.VMBR.Header.Description"] = safeLocalize(localizer, "Admin.VMBR.Header.Description")
	data["Admin.VMBR.Header.Enabled"] = safeLocalize(localizer, "Admin.VMBR.Header.Enabled")
	data["Admin.VMBR.Error"] = safeLocalize(localizer, "Admin.VMBR.Error")

	// Admin - Tags Management
	data["Admin.Tags.Title"] = safeLocalize(localizer, "Admin.Tags.Title")
	data["Admin.Tags.Description"] = safeLocalize(localizer, "Admin.Tags.Description")
	data["Admin.Tags.AddButton"] = safeLocalize(localizer, "Admin.Tags.AddButton")

	// Admin - Resource Limits
	data["Admin.Limits.Title"] = safeLocalize(localizer, "Admin.Limits.Title")
	data["Admin.Limits.Description"] = safeLocalize(localizer, "Admin.Limits.Description")
	data["Admin.Limits.VMSection"] = safeLocalize(localizer, "Admin.Limits.VMSection")
	data["Admin.Limits.NodeSection"] = safeLocalize(localizer, "Admin.Limits.NodeSection")
	data["Admin.Limits.NoNodes"] = safeLocalize(localizer, "Admin.Limits.NoNodes")
	data["Admin.Limits.NodeLabel"] = safeLocalize(localizer, "Admin.Limits.NodeLabel")
	data["Admin.Limits.Sockets"] = safeLocalize(localizer, "Admin.Limits.Sockets")
	data["Admin.Limits.Cores"] = safeLocalize(localizer, "Admin.Limits.Cores")
	data["Admin.Limits.Memory"] = safeLocalize(localizer, "Admin.Limits.Memory")
	data["Admin.Limits.Disk"] = safeLocalize(localizer, "Admin.Limits.Disk")
	data["Admin.Limits.Min"] = safeLocalize(localizer, "Admin.Limits.Min")
	data["Admin.Limits.Max"] = safeLocalize(localizer, "Admin.Limits.Max")
	data["Admin.Limits.GB"] = safeLocalize(localizer, "Admin.Limits.GB")
	data["Admin.Limits.ResetToDefaults"] = safeLocalize(localizer, "Admin.Limits.ResetToDefaults")
	data["Admin.Limits.Saving"] = safeLocalize(localizer, "Admin.Limits.Saving")
	data["Admin.Limits.Saved"] = safeLocalize(localizer, "Admin.Limits.Saved")
	data["Admin.Limits.Error"] = safeLocalize(localizer, "Admin.Limits.Error")
	data["Admin.Limits.DefaultsRestored"] = safeLocalize(localizer, "Admin.Limits.DefaultsRestored")
	data["Admin.Limits.Validation.InProgress"] = safeLocalize(localizer, "Admin.Limits.Validation.InProgress")
	data["Admin.Limits.Validation.Success"] = safeLocalize(localizer, "Admin.Limits.Validation.Success")
	data["Admin.Limits.Validation.MinMax"] = safeLocalize(localizer, "Admin.Limits.Validation.MinMax")
	data["Admin.Limits.Validation.Required"] = safeLocalize(localizer, "Admin.Limits.Validation.Required")
	data["Admin.Limits.Validation.Positive"] = safeLocalize(localizer, "Admin.Limits.Validation.Positive")
	data["Admin.Limits.NetworkError"] = safeLocalize(localizer, "Admin.Limits.NetworkError")

	q := r.URL.Query()
	q.Set("lang", "en")
	data["LangEN"] = "?" + q.Encode()
	q.Set("lang", "fr")
	data["LangFR"] = "?" + q.Encode()
}
