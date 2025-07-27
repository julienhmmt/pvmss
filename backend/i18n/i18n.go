package i18n

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"

	"pvmss/logger"
)

// Bundle is the global i18n bundle.
// It holds all the message catalogs for different languages and is used to create localizers.
// It is used by localizers to find and format translated strings.
var Bundle *i18n.Bundle

// InitI18n initializes the internationalization bundle.
// It sets up English as the default language, registers the TOML unmarshal function,
// and loads the English and French translation files from the 'i18n' directory.
// loadTranslationFile tente de charger un fichier de traduction depuis plusieurs emplacements possibles
func loadTranslationFile(bundle *i18n.Bundle, filename string) {
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
		logger.Get().Debug().Str("path", absPath).Msgf("Tentative de chargement du fichier de traduction: %s", filename)

		if _, err := os.Stat(path); err == nil {
			if _, err := bundle.LoadMessageFile(path); err == nil {
				logger.Get().Info().Str("file", absPath).Msg("Fichier de traduction chargé avec succès")
				loaded = true
				break
			} else {
				lastError = err
				logger.Get().Warn().Err(err).Str("path", absPath).Msg("Erreur lors du chargement du fichier de traduction")
			}
		} else {
			logger.Get().Debug().Str("path", absPath).Msg("Fichier de traduction introuvable à cet emplacement")
		}
	}

	if !loaded {
		logger.Get().Error().
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
}

// getLocalizer creates a new localizer for the specified language.
// If no language is provided, it falls back to the default language.
func getLocalizer(lang string) *i18n.Localizer {
	if lang == "" {
		lang = "en" // Default to English
	}
	return i18n.NewLocalizer(Bundle, lang)
}

// GetLocalizer creates a new localizer for the language specified in the request.
func GetLocalizer(r *http.Request) *i18n.Localizer {
	return getLocalizer(GetLanguage(r))
}

// Localize translates a message ID to the specified language.
// If the translation fails, it returns the message ID and logs a warning.
func Localize(localizer *i18n.Localizer, messageID string) string {
	if localizer == nil || messageID == "" {
		return messageID
	}

	localized, err := localizer.Localize(&i18n.LocalizeConfig{
		MessageID: messageID,
	})

	if err != nil {
		logger.Get().Warn().
			Err(err).
			Str("message_id", messageID).
			Msg("Translation not found")
		return messageID
	}

	return localized
}

// GetLanguage extracts the language from the request (cookie, header, or default)
func GetLanguage(r *http.Request) string {
	// Check URL parameter first
	if lang := r.URL.Query().Get("lang"); lang != "" {
		return lang
	}

	// Check cookie
	if cookie, err := r.Cookie("pvmss_lang"); err == nil && cookie.Value != "" {
		return cookie.Value
	}

	// Check Accept-Language header
	acceptLang := r.Header.Get("Accept-Language")
	if acceptLang != "" {
		// Parse first language from header (e.g., "en-US,en;q=0.9" -> "en")
		if strings.Contains(acceptLang, ";") {
			acceptLang = strings.Split(acceptLang, ";")[0]
		}
		if strings.Contains(acceptLang, ",") {
			acceptLang = strings.Split(acceptLang, ",")[0]
		}
		if strings.Contains(acceptLang, "-") {
			acceptLang = strings.Split(acceptLang, "-")[0]
		}
		return strings.ToLower(acceptLang)
	}

	// Default to English
	return "en"
}

// GetI18nData returns i18n data for the specified language
func GetI18nData(lang string) map[string]interface{} {
	data := make(map[string]interface{})

	// Create a request with the specified language
	req, _ := http.NewRequest("GET", "/", nil)
	q := req.URL.Query()
	q.Add("lang", lang)
	req.URL.RawQuery = q.Encode()

	// Use localizePage to populate translations
	LocalizePage(nil, req, data)
	data["Lang"] = lang
	return data
}

// LocalizePage injects all necessary localized strings into the data map for a given request.
// This function is called by renderTemplate before executing a template to ensure all UI text is translated.
// It uses the Localize function to safely handle missing translations.
func LocalizePage(w http.ResponseWriter, r *http.Request, data map[string]interface{}) {
	if data == nil {
		data = make(map[string]interface{})
	}

	// Get language from request and set cookie if needed
	lang := GetLanguage(r)
	if langParam := r.URL.Query().Get("lang"); langParam != "" {
		http.SetCookie(w, &http.Cookie{
			Name:    "pvmss_lang",
			Value:   langParam,
			Path:    "/",
			Expires: time.Now().Add(365 * 24 * time.Hour),
		})
		lang = langParam
	}

	// Create localizer for the current language
	localizer := getLocalizer(lang)

	// Create the translation map that templates expect
	t := make(map[string]string)

	// Common Elements
	t["Common.Add"] = Localize(localizer, "Common.Add")
	t["Common.BackToSearch"] = Localize(localizer, "Common.BackToSearch")
	t["Common.Cancel"] = Localize(localizer, "Common.Cancel")
	t["Common.Create"] = Localize(localizer, "Common.Create")
	t["Common.Error"] = Localize(localizer, "Common.Error")
	t["Common.Max"] = Localize(localizer, "Common.Max")
	t["Common.Memory"] = Localize(localizer, "Common.Memory")
	t["Common.Min"] = Localize(localizer, "Common.Min")
	t["Common.Name"] = Localize(localizer, "Common.Name")
	t["Common.Node"] = Localize(localizer, "Common.Node")
	t["Common.Reset"] = Localize(localizer, "Common.Reset")
	t["Common.Save"] = Localize(localizer, "Common.Save")
	t["Common.Saved"] = Localize(localizer, "Common.Saved")
	t["Common.Saving"] = Localize(localizer, "Common.Saving")
	t["Common.Search"] = Localize(localizer, "Common.Search")
	t["Common.SelectAll"] = Localize(localizer, "Common.SelectAll")
	t["Common.Selected"] = Localize(localizer, "Common.Selected")
	t["Common.SelectNone"] = Localize(localizer, "Common.SelectNone")
	t["Common.Settings"] = Localize(localizer, "Common.Settings")
	t["Common.Status"] = Localize(localizer, "Common.Status")
	t["Common.Storage"] = Localize(localizer, "Common.Storage")
	t["Common.Submit"] = Localize(localizer, "Common.Submit")
	t["Common.Success"] = Localize(localizer, "Common.Success")
	t["Common.Tags"] = Localize(localizer, "Common.Tags")
	t["Common.Unauthorized"] = Localize(localizer, "Common.Unauthorized")
	t["Common.Update"] = Localize(localizer, "Common.Update")
	t["Common.User"] = Localize(localizer, "Common.User")
	t["Common.Users"] = Localize(localizer, "Common.Users")
	t["Common.Yes"] = Localize(localizer, "Common.Yes")

	// Error messages
	t["Error.Generic"] = Localize(localizer, "Error.Generic")
	t["Error.InternalServer"] = Localize(localizer, "Error.InternalServer")
	t["Error.NotFound"] = Localize(localizer, "Error.NotFound")
	t["Error.Title"] = Localize(localizer, "Error.Title")
	t["Error.Unauthorized"] = Localize(localizer, "Error.Unauthorized")

	// Messages
	t["Message.ActionFailed"] = Localize(localizer, "Message.ActionFailed")
	t["Message.CreatedSuccessfully"] = Localize(localizer, "Message.CreatedSuccessfully")
	t["Message.DeletedSuccessfully"] = Localize(localizer, "Message.DeletedSuccessfully")
	t["Message.SavedSuccessfully"] = Localize(localizer, "Message.SavedSuccessfully")
	t["Message.UpdatedSuccessfully"] = Localize(localizer, "Message.UpdatedSuccessfully")

	// Status
	t["Status.Active"] = Localize(localizer, "Status.Active")
	t["Status.Connected"] = Localize(localizer, "Status.Connected")
	t["Status.Disabled"] = Localize(localizer, "Status.Disabled")
	t["Status.Disconnected"] = Localize(localizer, "Status.Disconnected")
	t["Status.Down"] = Localize(localizer, "Status.Down")
	t["Status.Error"] = Localize(localizer, "Status.Error")
	t["Status.Failed"] = Localize(localizer, "Status.Failed")
	t["Status.Offline"] = Localize(localizer, "Status.Offline")
	t["Status.Online"] = Localize(localizer, "Status.Online")
	t["Status.Paused"] = Localize(localizer, "Status.Paused")
	t["Status.Pending"] = Localize(localizer, "Status.Pending")
	t["Status.Running"] = Localize(localizer, "Status.Running")
	t["Status.Stopped"] = Localize(localizer, "Status.Stopped")
	t["Status.Success"] = Localize(localizer, "Status.Success")
	t["Status.Unknown"] = Localize(localizer, "Status.Unknown")
	t["Status.Updating"] = Localize(localizer, "Status.Updating")
	t["Status.Warning"] = Localize(localizer, "Status.Warning")

	// Validation
	t["Validation.Required"] = Localize(localizer, "Validation.Required")
	t["Validation.InvalidFormat"] = Localize(localizer, "Validation.InvalidFormat")
	t["Validation.MinLength"] = Localize(localizer, "Validation.MinLength")
	t["Validation.MaxLength"] = Localize(localizer, "Validation.MaxLength")
	t["Validation.Email"] = Localize(localizer, "Validation.Email")
	t["Validation.URL"] = Localize(localizer, "Validation.URL")
	t["Validation.Numeric"] = Localize(localizer, "Validation.Numeric")
	t["Validation.IP"] = Localize(localizer, "Validation.IP")
	t["Validation.Positive"] = Localize(localizer, "Validation.Positive")
	t["Validation.Negative"] = Localize(localizer, "Validation.Negative")
	t["Validation.MinLength"] = Localize(localizer, "Validation.MinLength")
	t["Validation.MaxLength"] = Localize(localizer, "Validation.MaxLength")
	t["Validation.Email"] = Localize(localizer, "Validation.Email")
	t["Validation.URL"] = Localize(localizer, "Validation.URL")
	t["Validation.Numeric"] = Localize(localizer, "Validation.Numeric")
	t["Validation.IP"] = Localize(localizer, "Validation.IP")
	t["Validation.Positive"] = Localize(localizer, "Validation.Positive")
	t["Validation.Negative"] = Localize(localizer, "Validation.Negative")

	// General UI
	t["Title"] = Localize(localizer, "Title")
	t["UI.AccessAdmin"] = Localize(localizer, "UI.AccessAdmin")
	t["UI.AdminDescription"] = Localize(localizer, "UI.AdminDescription")
	t["UI.Body"] = Localize(localizer, "UI.Body")
	t["UI.CreateVMDescription"] = Localize(localizer, "UI.CreateVMDescription")
	t["UI.DocsDescription"] = Localize(localizer, "UI.DocsDescription")
	t["UI.Footer"] = Localize(localizer, "UI.Footer")
	t["UI.GetStarted"] = Localize(localizer, "UI.GetStarted")
	t["UI.Header"] = Localize(localizer, "UI.Header")
	t["UI.Subtitle"] = Localize(localizer, "UI.Subtitle")
	t["UI.ViewDocumentation"] = Localize(localizer, "UI.ViewDocumentation")

	// Navbar
	t["Navbar.Admin"] = Localize(localizer, "Navbar.Admin")
	t["Navbar.AdminDocs"] = Localize(localizer, "Navbar.AdminDocs")
	t["Navbar.Documentation"] = Localize(localizer, "Navbar.Documentation")
	t["Navbar.Home"] = Localize(localizer, "Navbar.Home")
	t["Navbar.Login"] = Localize(localizer, "Navbar.Login")
	t["Navbar.Logout"] = Localize(localizer, "Navbar.Logout")
	t["Navbar.SearchVM"] = Localize(localizer, "Navbar.SearchVM")
	t["Navbar.UserDocs"] = Localize(localizer, "Navbar.UserDocs")
	t["Navbar.CreateVM"] = Localize(localizer, "Navbar.CreateVM")

	// Login Page
	t["Login.Button"] = Localize(localizer, "Login.Button")
	t["Login.PasswordLabel"] = Localize(localizer, "Login.PasswordLabel")
	t["Login.Title"] = Localize(localizer, "Login.Title")

	// Search Page
	t["Search.ActionsHeader"] = Localize(localizer, "Search.ActionsHeader")
	t["Search.Actions"] = Localize(localizer, "Search.Actions")
	t["Search.CardView"] = Localize(localizer, "Search.CardView")
	t["Search.Clear"] = Localize(localizer, "Search.Clear")
	t["Search.CPUs"] = Localize(localizer, "Search.CPUs")
	t["Search.Memory"] = Localize(localizer, "Search.Memory")
	t["Search.Name"] = Localize(localizer, "Search.Name")
	t["Search.NoResults"] = Localize(localizer, "Search.NoResults")
	t["Search.NoResultsMessage"] = Localize(localizer, "Search.NoResultsMessage")
	t["Search.Node"] = Localize(localizer, "Search.Node")
	t["Search.Placeholder"] = Localize(localizer, "Search.Placeholder")
	t["Search.PlaceholderName"] = Localize(localizer, "Search.PlaceholderName")
	t["Search.PlaceholderVMID"] = Localize(localizer, "Search.PlaceholderVMID")
	t["Search.Results"] = Localize(localizer, "Search.Results")
	t["Search.ResultsFor"] = Localize(localizer, "Search.ResultsFor")
	t["Search.ResultsFound"] = Localize(localizer, "Search.ResultsFound")
	t["Search.Status"] = Localize(localizer, "Search.Status")
	t["Search.Storage"] = Localize(localizer, "Search.Storage")
	t["Search.Submit"] = Localize(localizer, "Search.Submit")
	t["Search.Subtitle"] = Localize(localizer, "Search.Subtitle")
	t["Search.TableView"] = Localize(localizer, "Search.TableView")
	t["Search.Title"] = Localize(localizer, "Search.Title")
	t["Search.TitleName"] = Localize(localizer, "Search.TitleName")
	t["Search.TitleVMID"] = Localize(localizer, "Search.TitleVMID")
	t["Search.VMDetailsButton"] = Localize(localizer, "Search.VMDetailsButton")
	t["Search.VMID"] = Localize(localizer, "Search.VMID")
	t["Search.YouSearchedFor"] = Localize(localizer, "Search.YouSearchedFor")

	// Nodes Page
	t["Nodes.Header.CPUUsage"] = Localize(localizer, "Nodes.Header.CPUUsage")
	t["Nodes.Header.DiskUsage"] = Localize(localizer, "Nodes.Header.DiskUsage")
	t["Nodes.Header.MemoryUsage"] = Localize(localizer, "Nodes.Header.MemoryUsage")
	t["Nodes.Header.Node"] = Localize(localizer, "Nodes.Header.Node")
	t["Nodes.Header.Status"] = Localize(localizer, "Nodes.Header.Status")
	t["Nodes.NoNodes"] = Localize(localizer, "Nodes.NoNodes")
	t["Nodes.Status.Offline"] = Localize(localizer, "Nodes.Status.Offline")
	t["Nodes.Status.Online"] = Localize(localizer, "Nodes.Status.Online")
	t["Nodes.Title"] = Localize(localizer, "Nodes.Title")

	// VM Creation Page
	t["VM.Bridge.Description"] = Localize(localizer, "VM.Bridge.Description")
	t["VM.Create.BasicInfo"] = Localize(localizer, "VM.Create.BasicInfo")
	t["VM.Create.CPUCores"] = Localize(localizer, "VM.Create.CPUCores")
	t["VM.Create.CPUSockets"] = Localize(localizer, "VM.Create.CPUSockets")
	t["VM.Create.CreateButton"] = Localize(localizer, "VM.Create.CreateButton")
	t["VM.Create.Description"] = Localize(localizer, "VM.Create.Description")
	t["VM.Create.DescriptionPlaceholder"] = Localize(localizer, "VM.Create.DescriptionPlaceholder")
	t["VM.Create.DiskSize"] = Localize(localizer, "VM.Create.DiskSize")
	t["VM.Create.DiskSizeHelp"] = Localize(localizer, "VM.Create.DiskSizeHelp")
	t["VM.Create.Header"] = Localize(localizer, "VM.Create.Header")
	t["VM.Create.ISO"] = Localize(localizer, "VM.Create.ISO")
	t["VM.Create.ISOImage"] = Localize(localizer, "VM.Create.ISOImage")
	t["VM.Create.ISOImageHelp"] = Localize(localizer, "VM.Create.ISOImageHelp")
	t["VM.Create.MediaNetwork"] = Localize(localizer, "VM.Create.MediaNetwork")
	t["VM.Create.Memory"] = Localize(localizer, "VM.Create.Memory")
	t["VM.Create.MemoryHelp"] = Localize(localizer, "VM.Create.MemoryHelp")
	t["VM.Create.Network"] = Localize(localizer, "VM.Create.Network")
	t["VM.Create.NetworkBridgeHelp"] = Localize(localizer, "VM.Create.NetworkBridgeHelp")
	t["VM.Create.NoBridgesAvailable"] = Localize(localizer, "VM.Create.NoBridgesAvailable")
	t["VM.Create.NoISOs"] = Localize(localizer, "VM.Create.NoISOs")
	t["VM.Create.NoISOsAvailable"] = Localize(localizer, "VM.Create.NoISOsAvailable")
	t["VM.Create.ResetButton"] = Localize(localizer, "VM.Create.ResetButton")
	t["VM.Create.Resources"] = Localize(localizer, "VM.Create.Resources")
	t["VM.Create.SelectISO"] = Localize(localizer, "VM.Create.SelectISO")
	t["VM.Create.SelectNetwork"] = Localize(localizer, "VM.Create.SelectNetwork")
	t["VM.Create.Storage"] = Localize(localizer, "VM.Create.Storage")
	t["VM.Create.Tags"] = Localize(localizer, "VM.Create.Tags")
	t["VM.Create.TagsHelp"] = Localize(localizer, "VM.Create.TagsHelp")
	t["VM.Create.TagsPlaceholder"] = Localize(localizer, "VM.Create.TagsPlaceholder")
	t["VM.Create.Title"] = Localize(localizer, "VM.Create.Title")
	t["VM.Create.VMID"] = Localize(localizer, "VM.Create.VMID")
	t["VM.Create.VMID.Help"] = Localize(localizer, "VM.Create.VMID.Help")
	t["VM.Create.VMIDPlaceholder"] = Localize(localizer, "VM.Create.VMIDPlaceholder")
	t["VM.Create.VMName"] = Localize(localizer, "VM.Create.VMName")
	t["VM.Create.VMNameHelp"] = Localize(localizer, "VM.Create.VMNameHelp")
	t["VM.Create.VMNamePlaceholder"] = Localize(localizer, "VM.Create.VMNamePlaceholder")

	// VM Details Page
	t["VMDetails.Label.CPU"] = Localize(localizer, "VMDetails.Label.CPU")
	t["VMDetails.Label.Description"] = Localize(localizer, "VMDetails.Label.Description")
	t["VMDetails.Label.Disk"] = Localize(localizer, "VMDetails.Label.Disk")
	t["VMDetails.Label.ID"] = Localize(localizer, "VMDetails.Label.ID")
	t["VMDetails.Label.Name"] = Localize(localizer, "VMDetails.Label.Name")
	t["VMDetails.Label.Network"] = Localize(localizer, "VMDetails.Label.Network")
	t["VMDetails.Label.RAM"] = Localize(localizer, "VMDetails.Label.RAM")
	t["VMDetails.Label.Status"] = Localize(localizer, "VMDetails.Label.Status")
	t["VMDetails.Label.Uptime"] = Localize(localizer, "VMDetails.Label.Uptime")
	t["VMDetails.Action.Failed"] = Localize(localizer, "VMDetails.Action.Failed")
	t["VMDetails.Action.Processing"] = Localize(localizer, "VMDetails.Action.Processing")
	t["VMDetails.Action.Reboot"] = Localize(localizer, "VMDetails.Action.Reboot")
	t["VMDetails.Action.Refresh"] = Localize(localizer, "VMDetails.Action.Refresh")
	t["VMDetails.Action.Reset"] = Localize(localizer, "VMDetails.Action.Reset")
	t["VMDetails.Action.Shutdown"] = Localize(localizer, "VMDetails.Action.Shutdown")
	t["VMDetails.Action.Start"] = Localize(localizer, "VMDetails.Action.Start")
	t["VMDetails.Action.Stop"] = Localize(localizer, "VMDetails.Action.Stop")
	t["VMDetails.Action.Success"] = Localize(localizer, "VMDetails.Action.Success")

	// Documentation
	t["Docs.Admin.Description"] = Localize(localizer, "Docs.Admin.Description")
	t["Docs.Admin.Title"] = Localize(localizer, "Docs.Admin.Title")
	t["Docs.User.Description"] = Localize(localizer, "Docs.User.Description")
	t["Docs.User.Title"] = Localize(localizer, "Docs.User.Title")

	// Admin Section - ISO
	t["Admin.ISO.Description"] = Localize(localizer, "Admin.ISO.Description")
	t["Admin.ISO.Header.Enabled"] = Localize(localizer, "Admin.ISO.Header.Enabled")
	t["Admin.ISO.Header.Name"] = Localize(localizer, "Admin.ISO.Header.Name")
	t["Admin.ISO.Header.Size"] = Localize(localizer, "Admin.ISO.Header.Size")
	t["Admin.ISO.Header.Storage"] = Localize(localizer, "Admin.ISO.Header.Storage")
	t["Admin.ISO.Title"] = Localize(localizer, "Admin.ISO.Title")

	// Admin Section - Storage
	t["Admin.Storage.Description"] = Localize(localizer, "Admin.Storage.Description")
	t["Admin.Storage.Error"] = Localize(localizer, "Admin.Storage.Error")
	t["Admin.Storage.Header.Content"] = Localize(localizer, "Admin.Storage.Header.Content")
	t["Admin.Storage.Header.Name"] = Localize(localizer, "Admin.Storage.Header.Name")
	t["Admin.Storage.Header.Type"] = Localize(localizer, "Admin.Storage.Header.Type")
	t["Admin.Storage.NoStorages"] = Localize(localizer, "Admin.Storage.NoStorages")
	t["Admin.Storage.Title"] = Localize(localizer, "Admin.Storage.Title")

	// Admin Section - VMBR
	t["Admin.VMBR.Description"] = Localize(localizer, "Admin.VMBR.Description")
	t["Admin.VMBR.Error"] = Localize(localizer, "Admin.VMBR.Error")
	t["Admin.VMBR.Header.Description"] = Localize(localizer, "Admin.VMBR.Header.Description")
	t["Admin.VMBR.Header.Enabled"] = Localize(localizer, "Admin.VMBR.Header.Enabled")
	t["Admin.VMBR.Header.Name"] = Localize(localizer, "Admin.VMBR.Header.Name")
	t["Admin.VMBR.Header.Node"] = Localize(localizer, "Admin.VMBR.Header.Node")
	t["Admin.VMBR.NoVMBRs"] = Localize(localizer, "Admin.VMBR.NoVMBRs")
	t["Admin.VMBR.Title"] = Localize(localizer, "Admin.VMBR.Title")

	// Admin Section - Tags
	t["Admin.Tags.AddButton"] = Localize(localizer, "Admin.Tags.AddButton")
	t["Admin.Tags.Description"] = Localize(localizer, "Admin.Tags.Description")
	t["Admin.Tags.Title"] = Localize(localizer, "Admin.Tags.Title")

	// Admin Section - Limits
	t["Admin.Limits.Cores"] = Localize(localizer, "Admin.Limits.Cores")
	t["Admin.Limits.DefaultsRestored"] = Localize(localizer, "Admin.Limits.DefaultsRestored")
	t["Admin.Limits.Description"] = Localize(localizer, "Admin.Limits.Description")
	t["Admin.Limits.Disk"] = Localize(localizer, "Admin.Limits.Disk")
	t["Admin.Limits.GB"] = Localize(localizer, "Admin.Limits.GB")
	t["Admin.Limits.Memory"] = Localize(localizer, "Admin.Limits.Memory")
	t["Admin.Limits.NetworkError"] = Localize(localizer, "Admin.Limits.NetworkError")
	t["Admin.Limits.NodeLabel"] = Localize(localizer, "Admin.Limits.NodeLabel")
	t["Admin.Limits.NodeSection"] = Localize(localizer, "Admin.Limits.NodeSection")
	t["Admin.Limits.NoNodes"] = Localize(localizer, "Admin.Limits.NoNodes")
	t["Admin.Limits.ResetToDefaults"] = Localize(localizer, "Admin.Limits.ResetToDefaults")
	t["Admin.Limits.Saved"] = Localize(localizer, "Admin.Limits.Saved")
	t["Admin.Limits.Saving"] = Localize(localizer, "Admin.Limits.Saving")
	t["Admin.Limits.Sockets"] = Localize(localizer, "Admin.Limits.Sockets")
	t["Admin.Limits.Title"] = Localize(localizer, "Admin.Limits.Title")
	t["Admin.Limits.Validation.InProgress"] = Localize(localizer, "Admin.Limits.Validation.InProgress")
	t["Admin.Limits.Validation.MinMax"] = Localize(localizer, "Admin.Limits.Validation.MinMax")
	t["Admin.Limits.Validation.Required"] = Localize(localizer, "Admin.Limits.Validation.Required")
	t["Admin.Limits.Validation.Success"] = Localize(localizer, "Admin.Limits.Validation.Success")
	t["Admin.Limits.VMSection"] = Localize(localizer, "Admin.Limits.VMSection")

	// Add the completed translation map to the main data map
	data["t"] = t

	// Add language switcher URLs
	q := r.URL.Query()
	q.Set("lang", "en")
	data["LangEN"] = "?" + q.Encode()
	q.Set("lang", "fr")
	data["LangFR"] = "?" + q.Encode()
}
