package handlers

import (
	"net/http"

	"pvmss/components"
	"pvmss/i18n"
	"pvmss/proxmox"
)

func renderVMDetailsTempl(w http.ResponseWriter, r *http.Request, data map[string]interface{}) {
	templData := convertToVMDetailsData(data, r)
	localizer := i18n.GetLocalizerFromRequest(r)
	T := func(key string) string {
		return i18n.Localize(localizer, key)
	}
	if err := components.VMDetailsPage(templData, T).Render(r.Context(), w); err != nil {
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
	}
}

func convertToVMDetailsData(data map[string]interface{}, r *http.Request) components.VMDetailsData {
	result := components.VMDetailsData{
		Lang:      getLangFromRequest(r),
		CSRFToken: getStringFromMap(data, "CSRFToken"),
	}

	if vm, ok := data["VM"].(*proxmox.VM); ok && vm != nil {
		result.VM = components.VMInfo{
			VMID:   vm.VMID,
			Name:   vm.Name,
			Node:   vm.Node,
			Status: vm.Status,
			CPU:    vm.CPU,
			CPUs:   vm.CPUs,
			Mem:    vm.Mem,
			MaxMem: vm.MaxMem,
		}
	}

	result.ProxmoxConnected = getBoolFromMap(data, "ProxmoxConnected")
	result.IsAdmin = getBoolFromMap(data, "IsAdmin")
	result.Username = getStringFromMap(data, "Username")
	result.Success = getBoolFromMap(data, "Success")
	result.SuccessMessage = getStringFromMap(data, "SuccessMessage")
	result.Error = getStringFromMap(data, "ErrorMessage") != ""
	result.ErrorMessage = getStringFromMap(data, "ErrorMessage")
	result.Warning = getStringFromMap(data, "WarningMessage") != ""
	result.WarningMessage = getStringFromMap(data, "WarningMessage")
	result.IsNewlyCreated = getBoolFromMap(data, "IsNewlyCreated")
	result.Description = getStringFromMap(data, "Description")
	result.DescriptionHTML = getStringFromMap(data, "DescriptionHTML")
	result.Tags = getStringFromMap(data, "Tags")
	result.CurrentISO = getStringFromMap(data, "CurrentISO")
	result.EFIEnabled = getBoolFromMap(data, "EFIEnabled")
	result.EFIStorage = getStringFromMap(data, "EFIStorage")
	result.TPMEnabled = getBoolFromMap(data, "TPMEnabled")
	result.CloudInitEnabled = getBoolFromMap(data, "CloudInitEnabled")
	result.ShowDescriptionEditor = getBoolFromMap(data, "ShowDescriptionEditor")
	result.ShowTagsEditor = getBoolFromMap(data, "ShowTagsEditor")
	result.ShowResourcesEditor = getBoolFromMap(data, "ShowResourcesEditor")
	result.FormattedMemGB = getStringFromMap(data, "FormattedMemGB")
	result.FormattedMaxMemGB = getStringFromMap(data, "FormattedMaxMemGB")
	result.DisksTotalLabel = getStringFromMap(data, "DisksTotalLabel")
	result.CurrentBridge = getStringFromMap(data, "CurrentVMBR")
	result.CurrentNetworkModel = getStringFromMap(data, "CurrentNetworkModel")
	result.CurrentSnapshotName = getStringFromMap(data, "CurrentSnapshotName")
	result.MaxSnapshots = getIntFromMap(data, "MaxSnapshotsPerVM")
	result.VMRamMinMB = getIntFromMap(data, "VMRamMinMB")
	result.VMRamMaxMB = getIntFromMap(data, "VMRamMaxMB")
	result.VMSocketsMin = getIntFromMapWithDefault(data, "VMSocketsMin", 1)
	result.VMSocketsMax = getIntFromMapWithDefault(data, "VMSocketsMax", 4)
	result.VMCoresMin = getIntFromMapWithDefault(data, "VMCoresMin", 1)
	result.VMCoresMax = getIntFromMapWithDefault(data, "VMCoresMax", 32)

	result.Disks = convertDisksData(data)
	result.NetworkInterfaces = convertNetworkInterfaces(data)
	result.CloudInitData = convertCloudInitData(data)
	result.Snapshots = convertSnapshots(data)
	result.AvailableTags = convertStringSlice(data, "AllTags")
	result.Bridges = convertStringSlice(data, "AvailableVMBRs")
	result.ISOs = convertStringSlice(data, "AvailableISOs")

	return result
}

func convertDisksData(data map[string]interface{}) []components.DiskInfo {
	disks, ok := data["Disks"].([]diskTemplateData)
	if !ok {
		return nil
	}
	result := make([]components.DiskInfo, 0, len(disks))
	for _, d := range disks {
		result = append(result, components.DiskInfo{
			Index:     d.Index,
			Bus:       d.Bus,
			Storage:   d.Storage,
			Raw:       d.Raw,
			SizeGB:    d.SizeGB,
			SizeLabel: formatSizeLabelGB(d.SizeGB),
			Color:     busColor(d.Bus),
		})
	}
	return result
}

func convertNetworkInterfaces(data map[string]interface{}) []components.NetworkInterface {
	ifaces, ok := data["NetworkInterfaces"].([]proxmox.NetworkInterface)
	if !ok {
		return nil
	}
	result := make([]components.NetworkInterface, 0, len(ifaces))
	for _, iface := range ifaces {
		ni := components.NetworkInterface{
			Index:       iface.Index,
			Bridge:      iface.Bridge,
			Model:       iface.Model,
			ModelLabel:  iface.ModelLabel,
			MACAddress:  iface.MACAddress,
			VLAN:        iface.VLAN,
			Rate:        iface.Rate,
			MTU:         iface.MTU,
			LinkDown:    iface.LinkDown,
			IPAddresses: iface.IPAddresses,
		}
		result = append(result, ni)
	}
	return result
}

func convertCloudInitData(data map[string]interface{}) components.CloudInitData {
	ciData, ok := data["CloudInitData"].(map[string]string)
	if !ok {
		return components.CloudInitData{}
	}
	return components.CloudInitData{
		User:         ciData["user"],
		IPConfig:     ciData["ipConfig"],
		Nameserver:   ciData["nameserver"],
		Cicustom:     ciData["cicustom"],
		TemplateName: ciData["templateName"],
		TemplateYAML: ciData["templateYAML"],
		SSHKeys:      ciData["sshKeys"],
	}
}

func convertSnapshots(data map[string]interface{}) []components.SnapshotInfo {
	snaps, ok := data["Snapshots"].([]proxmox.VMSnapshot)
	if !ok {
		return nil
	}
	result := make([]components.SnapshotInfo, 0, len(snaps))
	for _, s := range snaps {
		result = append(result, components.SnapshotInfo{
			Name:        s.Name,
			Description: s.Description,
			Snaptime:    s.Snaptime,
			Vmstate:     s.Vmstate,
			Parent:      s.Parent,
		})
	}
	return result
}

func convertStringSlice(data map[string]interface{}, key string) []string {
	val, ok := data[key].([]string)
	if !ok {
		return nil
	}
	return val
}

func getLangFromRequest(r *http.Request) string {
	lang := r.URL.Query().Get("lang")
	if lang == "" {
		if cookie, err := r.Cookie("pvmss_lang"); err == nil {
			lang = cookie.Value
		}
	}
	if lang == "" {
		lang = "en"
	}
	return lang
}
