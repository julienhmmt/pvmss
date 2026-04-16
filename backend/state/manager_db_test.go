package state

import (
	"encoding/json"
	"sync"
	"testing"

	"pvmss/database"
)

// openTestDB opens a transient in-memory SQLite database for use in tests.
func openTestDB(t *testing.T) database.DB {
	t.Helper()
	db, err := database.OpenMemory()
	if err != nil {
		t.Fatalf("openTestDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// makeVMProfileConfig encodes a VMProfileConfig into the JSON blob stored in
// database.VMProfile.Config.
func makeVMProfileConfig(t *testing.T, sockets, cores, ramGB, diskGB int, diskBus, icon, color string) string {
	t.Helper()
	blob := map[string]interface{}{
		"sockets":  sockets,
		"cores":    cores,
		"ram_gb":   ramGB,
		"disk_gb":  diskGB,
		"disk_bus": diskBus,
		"icon":     icon,
		"color":    color,
	}
	data, err := json.Marshal(blob)
	if err != nil {
		t.Fatalf("makeVMProfileConfig: %v", err)
	}
	return string(data)
}

// ── T117: manager_test.go SetSettings test now uses in-memory DB ─────────────

func TestAppState_SetSettings_WithDB(t *testing.T) {
	db := openTestDB(t)
	sm := MakeAppStateWithDB(db)

	newSettings := &AppSettings{
		VMProfiles: []VMProfileConfig{
			{ID: "test", Name: "Test", Sockets: 1, Cores: 1, RAMGB: 2, DiskGB: 24, DiskBus: "virtio", Enabled: true},
		},
	}

	if err := sm.SetSettings(newSettings); err != nil {
		t.Fatalf("SetSettings() error: %v", err)
	}

	retrieved := sm.GetSettings()
	if len(retrieved.VMProfiles) != 1 {
		t.Errorf("GetSettings() returned %d profiles, want 1", len(retrieved.VMProfiles))
	}
}

// ── T118: cache invalidation after writes ────────────────────────────────────

func TestAppState_SetTags_CacheInvalidation(t *testing.T) {
	db := openTestDB(t)
	sm := MakeAppStateWithDB(db)
	if err := sm.LoadSettingsFromDB(); err != nil {
		t.Fatalf("LoadSettingsFromDB: %v", err)
	}

	before := sm.GetTags()
	if err := sm.SetTags([]string{"alpha", "beta"}, "admin"); err != nil {
		t.Fatalf("SetTags: %v", err)
	}
	after := sm.GetTags()
	if len(after) != 2 {
		t.Errorf("GetTags() after SetTags = %v, want [alpha beta]", after)
	}
	_ = before
}

func TestAppState_SetEnabledNodes_CacheInvalidation(t *testing.T) {
	db := openTestDB(t)
	sm := MakeAppStateWithDB(db)
	if err := sm.LoadSettingsFromDB(); err != nil {
		t.Fatalf("LoadSettingsFromDB: %v", err)
	}

	if err := sm.SetEnabledNodes([]string{"node1", "node2"}, "admin"); err != nil {
		t.Fatalf("SetEnabledNodes: %v", err)
	}
	settings := sm.GetSettings()
	if len(settings.EnabledNodes) != 2 {
		t.Errorf("EnabledNodes = %v, want [node1 node2]", settings.EnabledNodes)
	}
}

func TestAppState_SetEnabledStorages_CacheInvalidation(t *testing.T) {
	db := openTestDB(t)
	sm := MakeAppStateWithDB(db)
	if err := sm.LoadSettingsFromDB(); err != nil {
		t.Fatalf("LoadSettingsFromDB: %v", err)
	}

	if err := sm.SetEnabledStorages([]string{"local", "local-lvm"}, "admin"); err != nil {
		t.Fatalf("SetEnabledStorages: %v", err)
	}
	storages := sm.GetStorages()
	if len(storages) != 2 {
		t.Errorf("GetStorages() = %v, want [local local-lvm]", storages)
	}
}

func TestAppState_SetEnabledISOs_CacheInvalidation(t *testing.T) {
	db := openTestDB(t)
	sm := MakeAppStateWithDB(db)
	if err := sm.LoadSettingsFromDB(); err != nil {
		t.Fatalf("LoadSettingsFromDB: %v", err)
	}

	if err := sm.SetEnabledISOs([]string{"ubuntu.iso", "debian.iso"}, "admin"); err != nil {
		t.Fatalf("SetEnabledISOs: %v", err)
	}
	isos := sm.GetISOs()
	if len(isos) != 2 {
		t.Errorf("GetISOs() = %v, want [ubuntu.iso debian.iso]", isos)
	}
}

func TestAppState_SetEnabledVMBRs_CacheInvalidation(t *testing.T) {
	db := openTestDB(t)
	sm := MakeAppStateWithDB(db)
	if err := sm.LoadSettingsFromDB(); err != nil {
		t.Fatalf("LoadSettingsFromDB: %v", err)
	}

	if err := sm.SetEnabledVMBRs([]string{"vmbr0", "vmbr1"}, "admin"); err != nil {
		t.Fatalf("SetEnabledVMBRs: %v", err)
	}
	vmbrs := sm.GetVMBRs()
	if len(vmbrs) != 2 {
		t.Errorf("GetVMBRs() = %v, want [vmbr0 vmbr1]", vmbrs)
	}
}

func TestAppState_SetVMLimits_CacheInvalidation(t *testing.T) {
	db := openTestDB(t)
	sm := MakeAppStateWithDB(db)
	if err := sm.LoadSettingsFromDB(); err != nil {
		t.Fatalf("LoadSettingsFromDB: %v", err)
	}

	limits := &database.VMLimits{
		MaxVMs:          20,
		MaxVMPerUser:    5,
		MaxNetworkCards: 3,
		MaxDiskPerVM:    6,
		AllowCustomYAML: true,
		MaxSnapshots:    10,
	}
	if err := sm.SetVMLimits(limits, "admin"); err != nil {
		t.Fatalf("SetVMLimits: %v", err)
	}
	settings := sm.GetSettings()
	if settings.MaxVMPerUser != 5 {
		t.Errorf("MaxVMPerUser = %d, want 5", settings.MaxVMPerUser)
	}
	if settings.MaxNetworkCards != 3 {
		t.Errorf("MaxNetworkCards = %d, want 3", settings.MaxNetworkCards)
	}
	if settings.Limits.MaxSnapshots != 10 {
		t.Errorf("MaxSnapshots = %d, want 10", settings.Limits.MaxSnapshots)
	}
}

func TestAppState_SetNodeLimit_CacheInvalidation(t *testing.T) {
	db := openTestDB(t)
	sm := MakeAppStateWithDB(db)
	if err := sm.LoadSettingsFromDB(); err != nil {
		t.Fatalf("LoadSettingsFromDB: %v", err)
	}

	if err := sm.SetNodeLimit("pve1", 8, "admin"); err != nil {
		t.Fatalf("SetNodeLimit: %v", err)
	}
	if err := sm.DeleteNodeLimit("pve1", "admin"); err != nil {
		t.Fatalf("DeleteNodeLimit: %v", err)
	}
}

func TestAppState_CloudInitTemplate_CRUD(t *testing.T) {
	db := openTestDB(t)
	sm := MakeAppStateWithDB(db)
	if err := sm.LoadSettingsFromDB(); err != nil {
		t.Fatalf("LoadSettingsFromDB: %v", err)
	}

	tpl := &database.CloudInitTemplate{
		ID:          "pvmss-basic",
		Name:        "Basic",
		Description: "Basic template",
		YAMLContent: "#cloud-config\n",
		Enabled:     true,
	}
	if err := sm.CreateCloudInitTemplate(tpl, "admin"); err != nil {
		t.Fatalf("CreateCloudInitTemplate: %v", err)
	}

	settings := sm.GetSettings()
	if len(settings.CloudInitTemplates) != 1 {
		t.Fatalf("expected 1 template, got %d", len(settings.CloudInitTemplates))
	}
	if settings.CloudInitTemplates[0].ID != "pvmss-basic" {
		t.Errorf("template ID = %q, want pvmss-basic", settings.CloudInitTemplates[0].ID)
	}

	tpl.Name = "Basic Updated"
	if err := sm.UpdateCloudInitTemplate(tpl, "admin"); err != nil {
		t.Fatalf("UpdateCloudInitTemplate: %v", err)
	}
	settings = sm.GetSettings()
	if settings.CloudInitTemplates[0].Name != "Basic Updated" {
		t.Errorf("template Name = %q, want 'Basic Updated'", settings.CloudInitTemplates[0].Name)
	}

	if err := sm.DeleteCloudInitTemplate("pvmss-basic", "admin"); err != nil {
		t.Fatalf("DeleteCloudInitTemplate: %v", err)
	}
	settings = sm.GetSettings()
	if len(settings.CloudInitTemplates) != 0 {
		t.Errorf("expected 0 templates after delete, got %d", len(settings.CloudInitTemplates))
	}
}

func TestAppState_VMProfile_CRUD(t *testing.T) {
	db := openTestDB(t)
	sm := MakeAppStateWithDB(db)
	if err := sm.LoadSettingsFromDB(); err != nil {
		t.Fatalf("LoadSettingsFromDB: %v", err)
	}

	profile := &database.VMProfile{
		ID:          "web-server",
		Name:        "Web Server",
		Description: "Hosts websites",
		Config:      makeVMProfileConfig(t, 1, 2, 4, 32, "virtio", "Globe", "blue"),
		Enabled:     true,
	}
	if err := sm.CreateVMProfile(profile, "admin"); err != nil {
		t.Fatalf("CreateVMProfile: %v", err)
	}

	settings := sm.GetSettings()
	if len(settings.VMProfiles) != 1 {
		t.Fatalf("expected 1 VM profile, got %d", len(settings.VMProfiles))
	}
	if settings.VMProfiles[0].Cores != 2 {
		t.Errorf("VMProfile Cores = %d, want 2", settings.VMProfiles[0].Cores)
	}

	profile.Name = "Web Server (Updated)"
	if err := sm.UpdateVMProfile(profile, "admin"); err != nil {
		t.Fatalf("UpdateVMProfile: %v", err)
	}
	settings = sm.GetSettings()
	if settings.VMProfiles[0].Name != "Web Server (Updated)" {
		t.Errorf("VMProfile Name = %q, want 'Web Server (Updated)'", settings.VMProfiles[0].Name)
	}

	if err := sm.DeleteVMProfile("web-server", "admin"); err != nil {
		t.Fatalf("DeleteVMProfile: %v", err)
	}
	settings = sm.GetSettings()
	if len(settings.VMProfiles) != 0 {
		t.Errorf("expected 0 profiles after delete, got %d", len(settings.VMProfiles))
	}
}

func TestAppState_SetSFTPConfig_CacheInvalidation(t *testing.T) {
	db := openTestDB(t)
	sm := MakeAppStateWithDB(db)
	if err := sm.LoadSettingsFromDB(); err != nil {
		t.Fatalf("LoadSettingsFromDB: %v", err)
	}

	cfg := &database.SFTPConfig{
		Enabled:        true,
		Host:           "192.168.1.10",
		Port:           22,
		Username:       "pvmss-snippets",
		PrivateKeyPath: "/app/key",
		RemotePath:     "/var/lib/vz/snippets",
	}
	if err := sm.SetSFTPConfig(cfg, "admin"); err != nil {
		t.Fatalf("SetSFTPConfig: %v", err)
	}
	settings := sm.GetSettings()
	if !settings.CloudInitSFTP.Enabled {
		t.Error("CloudInitSFTP.Enabled should be true after SetSFTPConfig")
	}
	if settings.CloudInitSFTP.Host != "192.168.1.10" {
		t.Errorf("CloudInitSFTP.Host = %q, want 192.168.1.10", settings.CloudInitSFTP.Host)
	}
	if settings.CloudInitSFTP.SnippetBaseDir != "/var/lib/vz/snippets" {
		t.Errorf("SnippetBaseDir = %q, want /var/lib/vz/snippets", settings.CloudInitSFTP.SnippetBaseDir)
	}
}

// ── T119: concurrent read/write access ───────────────────────────────────────

func TestAppState_ConcurrentReadWrite(t *testing.T) {
	db := openTestDB(t)
	sm := MakeAppStateWithDB(db)
	if err := sm.LoadSettingsFromDB(); err != nil {
		t.Fatalf("LoadSettingsFromDB: %v", err)
	}

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_ = sm.GetSettings()
			_ = sm.GetTags()
			_ = sm.GetISOs()
			_ = sm.GetVMBRs()
			_ = sm.GetStorages()
			_ = sm.GetLimits()
		}()
		go func() {
			defer wg.Done()
			_ = sm.SetTags([]string{"concurrent"}, "admin")
		}()
	}
	wg.Wait()
}

// ── T120: LoadSettingsFromDB returns non-nil default settings ─────────────────

func TestAppState_LoadSettingsFromDB_Defaults(t *testing.T) {
	db := openTestDB(t)
	sm := MakeAppStateWithDB(db)
	if err := sm.LoadSettingsFromDB(); err != nil {
		t.Fatalf("LoadSettingsFromDB: %v", err)
	}
	settings := sm.GetSettings()
	if settings == nil {
		t.Fatal("GetSettings() returned nil after LoadSettingsFromDB")
	}
	if settings.MaxVMPerUser <= 0 {
		t.Errorf("MaxVMPerUser should be > 0, got %d", settings.MaxVMPerUser)
	}
}

// ── T121: nil DB gracefully returns errors, not panics ────────────────────────

func TestAppState_NilDB_SettersReturnError(t *testing.T) {
	sm := MakeAppState()

	tests := []struct {
		name    string
		fn      func() error
		wantErr bool // true if the call must return an error when no DB is configured
	}{
		{"SetTags", func() error { return sm.SetTags([]string{"x"}, "admin") }, true},
		{"SetEnabledNodes", func() error { return sm.SetEnabledNodes([]string{"n1"}, "admin") }, true},
		{"SetEnabledStorages", func() error { return sm.SetEnabledStorages([]string{"s1"}, "admin") }, true},
		{"SetEnabledISOs", func() error { return sm.SetEnabledISOs([]string{"i1"}, "admin") }, true},
		{"SetEnabledVMBRs", func() error { return sm.SetEnabledVMBRs([]string{"vmbr0"}, "admin") }, true},
		{"SetVMLimits", func() error {
			return sm.SetVMLimits(&database.VMLimits{MaxVMPerUser: 5}, "admin")
		}, true},
		{"SetNodeLimit", func() error { return sm.SetNodeLimit("n", 5, "admin") }, true},
		{"DeleteNodeLimit", func() error { return sm.DeleteNodeLimit("n", "admin") }, true},
		{"CreateCloudInitTemplate", func() error {
			return sm.CreateCloudInitTemplate(&database.CloudInitTemplate{ID: "x", Name: "X"}, "admin")
		}, true},
		{"UpdateCloudInitTemplate", func() error {
			return sm.UpdateCloudInitTemplate(&database.CloudInitTemplate{ID: "x", Name: "X"}, "admin")
		}, true},
		{"DeleteCloudInitTemplate", func() error { return sm.DeleteCloudInitTemplate("x", "admin") }, true},
		{"CreateVMProfile", func() error {
			return sm.CreateVMProfile(&database.VMProfile{ID: "x", Name: "X", Config: "{}"}, "admin")
		}, true},
		{"UpdateVMProfile", func() error {
			return sm.UpdateVMProfile(&database.VMProfile{ID: "x", Name: "X", Config: "{}"}, "admin")
		}, true},
		{"DeleteVMProfile", func() error { return sm.DeleteVMProfile("x", "admin") }, true},
		{"SetSFTPConfig", func() error {
			return sm.SetSFTPConfig(&database.SFTPConfig{Host: "h"}, "admin")
		}, true},
		{"LoadSettingsFromDB", func() error { return sm.LoadSettingsFromDB() }, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.fn()
			if tc.wantErr {
				if err == nil {
					t.Errorf("%s: expected error when no DB configured, got nil", tc.name)
				}
			} else {
				// LoadSettingsFromDB is a no-op when no DB is configured
				if err != nil {
					t.Errorf("%s: unexpected error: %v", tc.name, err)
				}
			}
		})
	}
}

// ── T122: zero DB reads during normal GetSettings calls ───────────────────────
//
// After initial load, GetSettings must never hit the DB.  We verify this
// structurally: GetSettings holds only settingsMu and reads s.settings; it
// does not call any database method.  The test below confirms that repeated
// GetSettings calls return consistent data without causing any write-backs.

func TestAppState_GetSettings_NoDBReads(t *testing.T) {
	db := openTestDB(t)
	sm := MakeAppStateWithDB(db)
	if err := sm.LoadSettingsFromDB(); err != nil {
		t.Fatalf("LoadSettingsFromDB: %v", err)
	}

	// Close the DB to make any accidental DB read fail immediately.
	if err := db.Close(); err != nil {
		t.Fatalf("db.Close: %v", err)
	}

	// All read-only accessors must work off the in-memory cache.
	if sm.GetSettings() == nil {
		t.Error("GetSettings() returned nil after DB close")
	}
	_ = sm.GetTags()
	_ = sm.GetISOs()
	_ = sm.GetVMBRs()
	_ = sm.GetStorages()
	_ = sm.GetLimits()
}
