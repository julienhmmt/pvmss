package main

import (
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

var bundle *i18n.Bundle

// initI18n initializes the i18n bundle with the available languages.
func initI18n() {
	bundle = i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
	
	// Load English translations
	if _, err := bundle.LoadMessageFile("i18n/active.en.toml"); err != nil {
		log.Printf("Error loading English translations: %v", err)
	}

	// Load French translations
	if _, err := bundle.LoadMessageFile("i18n/active.fr.toml"); err != nil {
		log.Printf("Error loading French translations: %v", err)
	}

	// Verify specific keys
	frLocalizer := i18n.NewLocalizer(bundle, "fr")
	_, err := frLocalizer.Localize(&i18n.LocalizeConfig{
		MessageID: "Limits.Disk",
	})
	if err != nil {
		log.Printf("Error loading 'Limits.Disk' in French: %v", err)
	}
}

// localizePage populates the data map with localized strings.
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

	localizer := i18n.NewLocalizer(bundle, lang)

	// Search page
	data["NoResults"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Search.NoResults"})
	
	// VM Creation page
	data["CreateVMTitle"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "VM.Create.Title"})
	data["NoISOsMessage"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "VM.Create.NoISOs"})
	data["NoBridgesMessage"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "VM.Create.NoBridges"})
	data["TagsTitle"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "VM.Create.Tags"})
	data["TagsHelp"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "VM.Create.TagsHelp"})
	data["VMName"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "VM.Create.VMName"})
	data["VMNamePlaceholder"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "VM.Create.VMNamePlaceholder"})
	data["VMNameHelp"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "VM.Create.VMNameHelp"})
	data["Description"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "VM.Create.Description"})
	data["DescriptionPlaceholder"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "VM.Create.DescriptionPlaceholder"})
	data["VMID"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "VM.Create.VMID"})
	data["VMIDPlaceholder"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "VM.Create.VMIDPlaceholder"})
	data["CPUSockets"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "VM.Create.CPUSockets"})
	data["CPUCores"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "VM.Create.CPUCores"})
	data["Memory"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "VM.Create.Memory"})
	data["Storage"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "VM.Create.Storage"})
	data["ISO"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "VM.Create.ISO"})
	data["SelectISO"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "VM.Create.SelectISO"})
	data["Network"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "VM.Create.Network"})
	data["SelectNetwork"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "VM.Create.SelectNetwork"})
	data["CreateButton"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "VM.Create.CreateButton"})
	data["ResetButton"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "VM.Create.ResetButton"})
	data["BridgeDescription"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "VM.Bridge.Description"})
	data["DefaultBridge"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "VM.Bridge.Default"})
	
	// Admin page
	data["AdminNodes"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Admin.Nodes"})
	data["AdminPage"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Admin.Page"})
	data["AdminTagsTitle"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Admin.Tags.Title"})
	data["AdminTagsDescription"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Admin.Tags.Description"})
	data["AdminTagsAddButton"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Admin.Tags.AddButton"})
	data["AdminStorageTitle"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Admin.Storage.Title"})
	data["AdminStorageDescription"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Admin.Storage.Description"})
	data["AdminStorageHeaderName"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Admin.Storage.Header.Name"})
	data["AdminStorageHeaderType"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Admin.Storage.Header.Type"})
	data["AdminStorageHeaderContent"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Admin.Storage.Header.Content"})
	data["AdminStorageNoStorages"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Admin.Storage.NoStorages"})
	data["AdminStorageError"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Admin.Storage.Error"})
	data["AdminISOTitle"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Admin.ISO.Title"})
	data["AdminISODescription"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Admin.ISO.Description"})
	data["AdminISOHeaderEnabled"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Admin.ISO.Header.Enabled"})
	data["AdminISOHeaderStorage"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Admin.ISO.Header.Storage"})
	data["AdminISOHeaderName"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Admin.ISO.Header.Name"})
	data["AdminISOHeaderSize"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Admin.ISO.Header.Size"})
	data["AdminVMBRTitle"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Admin.VMBR.Title"})
	data["AdminVMBRDescription"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Admin.VMBR.Description"})
	data["AdminVMBRHeaderEnabled"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Admin.VMBR.Header.Enabled"})
	data["AdminVMBRHeaderNode"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Admin.VMBR.Header.Node"})
	data["AdminVMBRHeaderName"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Admin.VMBR.Header.Name"})
	data["AdminVMBRHeaderDescription"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Admin.VMBR.Header.Description"})
	data["AdminVMBRNoVMBRs"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Admin.VMBR.NoVMBRs"})

	// VM Limits page
	data["VMLimitsTitle"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Limits.Title"})
	data["VMLimitsDescription"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Limits.Description"})
	data["VMLimitsNodeLabel"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Limits.NodeLabel"})
	data["VMLimitsSockets"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Limits.Sockets"})
	data["VMLimitsCores"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Limits.Cores"})
	data["VMLimitsMemory"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Limits.Memory"})
	data["VMLimitsDisk"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Limits.Disk"})
	data["VMLimitsMin"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Limits.Min"})
	data["VMLimitsMax"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Limits.Max"})
	data["VMLimitsGB"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Limits.GB"})
	data["VMLimitsSaveButton"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Limits.SaveButton"})
	data["VMLimitsResetButton"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Common.Reset"})
	data["VMLimitsSaveSuccess"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Limits.SaveSuccess"})
	data["VMLimitsSaveError"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Limits.SaveError"})
	data["VMLimitsValidationMinMax"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Limits.Validation.MinMax"})
	data["VMLimitsValidationRequired"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Limits.Validation.Required"})

	// Node Limits page
	data["NodesLimitsTitle"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Nodes.Limits.Title"})
	data["NodesLimitsDescription"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Nodes.Limits.Description"})
	data["NodesLimitsNodeLabel"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Nodes.Limits.NodeLabel"})
	data["NodesLimitsSockets"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Nodes.Limits.Sockets"})
	data["NodesLimitsCores"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Nodes.Limits.Cores"})
	data["NodesLimitsMemory"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Nodes.Limits.MemoryGB"})
	data["NodesLimitsMin"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Nodes.Limits.MinPlaceholder"})
	data["NodesLimitsMax"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Nodes.Limits.MaxPlaceholder"})
	data["NodesLimitsGB"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Nodes.Limits.MemoryGB"})
	data["NodesLimitsSaveButton"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Nodes.Limits.SaveButton"})
	data["NodesLimitsSaveSuccess"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Nodes.Limits.SaveSuccess"})
	data["NodesLimitsSaveError"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Nodes.Limits.ErrorOccurred"})
	data["NodesLimitsNoNodes"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Nodes.Limits.NoNodes"})

	data["Body"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Body"})
	data["ButtonSearchVM"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Button.Search"})
	data["Footer"] = template.HTML(localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Footer"}))
	data["Header"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Header"})
	data["Lang"] = lang
	data["NavbarAdmin"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Navbar.Admin"})
	data["NavbarLogin"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Navbar.Login"})
	data["NavbarLogout"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Navbar.Logout"})
	data["NavbarHome"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Navbar.Home"})
	data["NavbarSearchVM"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Navbar.SearchVM"})
	data["NavbarVMs"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Navbar.VMs"})
	data["NavbarAdminDocs"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "NavbarAdminDocs"})
	data["NavbarUserDocs"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "NavbarUserDocs"})
	data["SearchTitle"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Search.Title"})
	data["PlaceholderVMID"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Search.PlaceholderVMID"})
	data["TitleVMID"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Search.TitleVMID"})
	data["PlaceholderName"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Search.PlaceholderName"})
	data["SearchVMID"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Search.VMID"})
	data["SearchName"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Search.Name"})
	data["SearchStatus"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Search.Status"})
	data["SearchCPUs"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Search.CPUs"})
	data["SearchMemory"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Search.Memory"})
	data["SearchResults"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Search.Results"})
	data["SearchYouSearchedFor"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Search.YouSearchedFor"})
	data["SearchActionsHeader"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Search.ActionsHeader"})
	data["SearchVMDetailsButton"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Search.VMDetailsButton"})
	data["Subtitle"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Subtitle"})
	data["Title"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Title"})
	data["DetailLabelName"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "VMDetails.Name"})
	data["DetailLabelID"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "VMDetails.ID"})
	data["DetailLabelStatus"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "VMDetails.Status"})
	data["DetailLabelUptime"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "VMDetails.Uptime"})
	data["DetailLabelCPU"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "VMDetails.CPU"})
	data["DetailLabelRAM"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "VMDetails.RAM"})
	data["DetailLabelDisk"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "VMDetails.Disks"})
	data["DetailLabelNetwork"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "VMDetails.Network"})
	data["DetailLabelDescription"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "VMDetails.Description"})
	data["BackToSearch"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "VMDetails.BackToSearch"})

	// VM Action Buttons
	data["VMDetailsActionStart"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "VMDetails.Action.Start"})
	data["VMDetailsActionReboot"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "VMDetails.Action.Reboot"})
	data["VMDetailsActionShutdown"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "VMDetails.Action.Shutdown"})
	data["VMDetailsActionStop"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "VMDetails.Action.Stop"})
	data["VMDetailsActionReset"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "VMDetails.Action.Reset"})
	data["VMDetailsActionProcessing"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "VMDetails.Action.Processing"})
	data["VMDetailsActionSuccess"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "VMDetails.Action.Success"})
	data["VMDetailsActionFailed"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "VMDetails.Action.Failed"})
	data["VMDetailsActionRefresh"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "VMDetails.Action.Refresh"})

	// Login page
	data["LoginTitle"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Login.Title"})
	data["LoginPasswordLabel"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Login.PasswordLabel"})
	data["LoginButton"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Login.Button"})

	// Nodes page
	data["NodesNoNodes"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Nodes.NoNodes"})
	data["NodesHeaderNode"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Nodes.Header.Node"})
	data["NodesHeaderStatus"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Nodes.Header.Status"})
	data["NodesHeaderCPUUsage"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Nodes.Header.CPUUsage"})
	data["NodesHeaderMemoryUsage"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Nodes.Header.MemoryUsage"})
	data["NodesHeaderDiskUsage"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Nodes.Header.DiskUsage"})
	data["NodesStatusOnline"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Nodes.Status.Online"})
	data["NodesStatusOffline"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "Nodes.Status.Offline"})

	// Create VM page
	data["CreateVMTitle"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "CreateVM.Title"})
	data["CreateVMHeader"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "CreateVM.Header"})
	data["CreateVMVMID"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "CreateVM.VMID"})
	data["CreateVMVMIDHelp"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "CreateVM.VMID.Help"})
	data["CreateVMName"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "CreateVM.Name"})
	data["CreateVMDescription"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "CreateVM.Description"})
	data["CreateVMCores"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "CreateVM.Cores"})
	data["CreateVMMemory"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "CreateVM.Memory"})
	data["CreateVMDisk"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "CreateVM.Disk"})
	data["CreateVMButton"] = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "CreateVM.Button"})

	q := r.URL.Query()
	q.Set("lang", "en")
	data["LangEN"] = "?" + q.Encode()
	q.Set("lang", "fr")
	data["LangFR"] = "?" + q.Encode()
}
