package state

import (
	"pvmss/database"
	"testing"
)

func TestMakeAppState(t *testing.T) {
	state := MakeAppState()

	if state == nil {
		t.Fatal("MakeAppState() returned nil")
	}

	// Verify initial state
	if state.GetSettings() == nil {
		t.Error("MakeAppState() should initialize settings")
	}

	if state.GetFrontendPath() != "" {
		t.Error("MakeAppState() should initialize empty frontend path")
	}

	if state.IsOfflineMode() {
		t.Error("MakeAppState() should not start in offline mode")
	}
}

func TestAppState_GetFrontendPath(t *testing.T) {
	state := MakeAppState()

	path := state.GetFrontendPath()
	if path != "" {
		t.Errorf("GetFrontendPath() should return empty string initially, got %q", path)
	}

	// Verify the same from MakeAppState
	if state.GetFrontendPath() != "" {
		t.Error("MakeAppState() should initialize with empty frontend path")
	}
}

func TestAppState_SetFrontendPath(t *testing.T) {
	state := MakeAppState()

	testPath := "/test/frontend"
	state.SetFrontendPath(testPath)

	path := state.GetFrontendPath()
	if path != testPath {
		t.Errorf("GetFrontendPath() = %q, want %q", path, testPath)
	}
}

func TestAppState_SetGuestAgentCleanupFunc(t *testing.T) {
	state := MakeAppState()

	cleanupFunc := func() {
		// Cleanup function placeholder
	}

	// Verify the function can be set without panicking
	// The actual cleanup runs in a background goroutine with a ticker
	// and is difficult to test without waiting or adding internal getters
	state.SetGuestAgentCleanupFunc(cleanupFunc)

	// If we reach here without panic, the function was registered successfully
}

func TestAppState_GetSettings(t *testing.T) {
	state := MakeAppState()

	settings := state.GetSettings()
	if settings == nil {
		t.Fatal("GetSettings() returned nil")
	}

	// Verify settings is initialized and GetVMProfiles returns defaults
	profiles := settings.GetVMProfiles()
	if len(profiles) == 0 {
		t.Error("GetVMProfiles() should return default profiles when VMProfiles is empty")
	}
}

func TestAppState_SetSettings(t *testing.T) {
	db, err := database.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer func() { _ = db.Close() }()

	state := MakeAppStateWithDB(db)

	newSettings := &AppSettings{
		VMProfiles: []VMProfileConfig{
			{ID: "test", Name: "Test", Sockets: 1, Cores: 1, RAMGB: 2, DiskGB: 24, DiskBus: "virtio", Enabled: true},
		},
	}

	if err := state.SetSettings(newSettings); err != nil {
		t.Errorf("SetSettings() returned error: %v", err)
	}

	retrieved := state.GetSettings()
	if len(retrieved.VMProfiles) != 1 {
		t.Errorf("SetSettings() did not update settings, got %d profiles", len(retrieved.VMProfiles))
	}
}

func TestAppState_SetSettingsWithoutSave(t *testing.T) {
	state := MakeAppState()

	newSettings := &AppSettings{
		VMProfiles: []VMProfileConfig{
			{ID: "test", Name: "Test", Sockets: 1, Cores: 1, RAMGB: 2, DiskGB: 24, DiskBus: "virtio", Enabled: true},
		},
	}

	state.SetSettingsWithoutSave(newSettings)

	retrieved := state.GetSettings()
	if len(retrieved.VMProfiles) != 1 {
		t.Errorf("SetSettingsWithoutSave() did not update settings, got %d profiles", len(retrieved.VMProfiles))
	}
}

func TestAppState_OfflineMode(t *testing.T) {
	state := MakeAppState()

	// Initially not in offline mode
	if state.IsOfflineMode() {
		t.Error("Should not be in offline mode initially")
	}

	// Enable offline mode
	state.SetOfflineMode()

	if !state.IsOfflineMode() {
		t.Error("Should be in offline mode after SetOfflineMode()")
	}

	// Check Proxmox status in offline mode
	connected, msg := state.GetProxmoxStatus()
	if connected {
		t.Error("GetProxmoxStatus() should return false in offline mode")
	}
	if msg == "" {
		t.Error("GetProxmoxStatus() should return error message in offline mode")
	}
}

func TestAppState_GetTags(t *testing.T) {
	state := MakeAppState()

	// Initially empty
	tags := state.GetTags()
	if tags == nil {
		t.Error("GetTags() should return empty slice, not nil")
	}

	// Set settings with tags
	settings := &AppSettings{
		Tags: []string{"tag1", "tag2", "tag3"},
	}
	state.SetSettingsWithoutSave(settings)

	tags = state.GetTags()
	if len(tags) != 3 {
		t.Errorf("GetTags() returned %d tags, want 3", len(tags))
	}
}

func TestAppState_GetISOs(t *testing.T) {
	state := MakeAppState()

	// Initially empty
	isos := state.GetISOs()
	if isos == nil {
		t.Error("GetISOs() should return empty slice, not nil")
	}

	// Set settings with ISOs
	settings := &AppSettings{
		ISOs: []string{"iso1.iso", "iso2.iso"},
	}
	state.SetSettingsWithoutSave(settings)

	isos = state.GetISOs()
	if len(isos) != 2 {
		t.Errorf("GetISOs() returned %d ISOs, want 2", len(isos))
	}
}

func TestAppState_GetVMBRs(t *testing.T) {
	state := MakeAppState()

	// Initially empty
	vmbrs := state.GetVMBRs()
	if vmbrs == nil {
		t.Error("GetVMBRs() should return empty slice, not nil")
	}

	// Set settings with VMBRs
	settings := &AppSettings{
		VMBRs: []string{"vmbr0", "vmbr1"},
	}
	state.SetSettingsWithoutSave(settings)

	vmbrs = state.GetVMBRs()
	if len(vmbrs) != 2 {
		t.Errorf("GetVMBRs() returned %d VMBRs, want 2", len(vmbrs))
	}
}

func TestAppState_GetLimits(t *testing.T) {
	state := MakeAppState()

	// Initially empty
	limits := state.GetLimits()
	if limits == nil {
		t.Error("GetLimits() should return empty map, not nil")
	}

	// Set settings with limits
	settings := &AppSettings{
		Limits: LimitsConfig{
			VM: VMResourceLimits{
				Sockets: ResourceRange{Min: 1, Max: 4},
				Cores:   ResourceRange{Min: 1, Max: 16},
			},
		},
	}
	state.SetSettingsWithoutSave(settings)

	limits = state.GetLimits()
	if len(limits) == 0 {
		t.Error("GetLimits() should return limits when set")
	}
}

func TestAppState_GetStorages(t *testing.T) {
	state := MakeAppState()

	// Initially nil
	storages := state.GetStorages()
	if storages != nil {
		t.Error("GetStorages() should return nil initially")
	}

	// Set settings with storages
	settings := &AppSettings{
		EnabledStorages: []string{"local", "local-lvm"},
	}
	state.SetSettingsWithoutSave(settings)

	storages = state.GetStorages()
	if len(storages) != 2 {
		t.Errorf("GetStorages() returned %d storages, want 2", len(storages))
	}
}
