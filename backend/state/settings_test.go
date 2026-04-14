package state

import (
	"testing"
)

func TestGetMaxDisksForBus(t *testing.T) {
	tests := []struct {
		name    string
		busType string
		want    int
	}{
		{
			name:    "IDE bus",
			busType: DiskBusIDE,
			want:    MaxDisksIDE,
		},
		{
			name:    "SATA bus",
			busType: DiskBusSATA,
			want:    MaxDisksSATA,
		},
		{
			name:    "VirtIO bus",
			busType: DiskBusVirtIO,
			want:    MaxDisksVirtIO,
		},
		{
			name:    "SCSI bus",
			busType: DiskBusSCSI,
			want:    MaxDisksSCSI,
		},
		{
			name:    "Unknown bus defaults to VirtIO",
			busType: "unknown",
			want:    MaxDisksVirtIO,
		},
		{
			name:    "Empty string defaults to VirtIO",
			busType: "",
			want:    MaxDisksVirtIO,
		},
		{
			name:    "Mixed case defaults to VirtIO",
			busType: "VIRTIO",
			want:    MaxDisksVirtIO,
			// Note: Case-insensitive matching is intentional to handle user input variations
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetMaxDisksForBus(tt.busType); got != tt.want {
				t.Errorf("GetMaxDisksForBus(%q) = %v, want %v", tt.busType, got, tt.want)
			}
		})
	}
}

func TestDiskBusConstants(t *testing.T) {
	// Verify disk bus constants have correct values
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"DiskBusIDE", DiskBusIDE, "ide"},
		{"DiskBusSATA", DiskBusSATA, "sata"},
		{"DiskBusVirtIO", DiskBusVirtIO, "virtio"},
		{"DiskBusSCSI", DiskBusSCSI, "scsi"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value != tt.want {
				t.Errorf("%s = %q, want %q", tt.name, tt.value, tt.want)
			}
		})
	}
}

func TestMaxDisksConstants(t *testing.T) {
	// Verify max disks constants have correct values
	tests := []struct {
		name  string
		value int
		want  int
	}{
		{"MaxDisksIDE", MaxDisksIDE, 4},
		{"MaxDisksSATA", MaxDisksSATA, 6},
		{"MaxDisksVirtIO", MaxDisksVirtIO, 16},
		{"MaxDisksSCSI", MaxDisksSCSI, 14},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value != tt.want {
				t.Errorf("%s = %v, want %v", tt.name, tt.value, tt.want)
			}
		})
	}
}

func TestSettingsConstants(t *testing.T) {
	// Verify settings constants have reasonable values
	tests := []struct {
		name  string
		value int
		min   int
		max   int
	}{
		{"MinNetworkCards", MinNetworkCards, 1, 1},
		{"MaxNetworkCards", MaxNetworkCards, 2, 64},
		{"MinDiskPerVM", MinDiskPerVM, 0, 1},
		{"MaxDiskPerVM", MaxDiskPerVM, 1, 32},
		{"DefaultDiskPerVM", DefaultDiskPerVM, MinDiskPerVM, MaxDiskPerVM},
		{"MinVMPerUser", MinVMPerUser, 0, 0},
		{"MaxVMPerUser", MaxVMPerUser, 1, 1000},
		{"DefaultVMPerUser", DefaultVMPerUser, MinVMPerUser, MaxVMPerUser},
		{"MinSnapshotsPerVM", MinSnapshotsPerVM, 0, 0},
		{"MaxSnapshotsPerVM", MaxSnapshotsPerVM, 1, 100},
		{"DefaultSnapshotsPerVM", DefaultSnapshotsPerVM, MinSnapshotsPerVM, MaxSnapshotsPerVM},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value < tt.min || tt.value > tt.max {
				t.Errorf("%s = %v, want between %v and %v", tt.name, tt.value, tt.min, tt.max)
			}
		})
	}
}

func TestDefaultVMProfiles(t *testing.T) {
	profiles := DefaultVMProfiles()

	if len(profiles) == 0 {
		t.Fatal("DefaultVMProfiles() returned empty slice")
	}

	// Verify all profiles have required fields
	for _, p := range profiles {
		if p.ID == "" {
			t.Error("Profile has empty ID")
		}
		if p.Name == "" {
			t.Error("Profile has empty Name")
		}
		if p.Sockets < 1 {
			t.Errorf("Profile %s has invalid Sockets: %v", p.ID, p.Sockets)
		}
		if p.Cores < 1 {
			t.Errorf("Profile %s has invalid Cores: %v", p.ID, p.Cores)
		}
		if p.RAMGB < 1 {
			t.Errorf("Profile %s has invalid RAMGB: %v", p.ID, p.RAMGB)
		}
		if p.DiskGB < 1 {
			t.Errorf("Profile %s has invalid DiskGB: %v", p.ID, p.DiskGB)
		}
		if p.DiskBus == "" {
			t.Errorf("Profile %s has empty DiskBus", p.ID)
		}
	}
}

func TestAppSettings_GetVMProfiles(t *testing.T) {
	tests := []struct {
		name         string
		profiles     []VMProfileConfig
		wantMinCount int
		wantDefaults bool
	}{
		{
			name:         "Empty profiles returns defaults",
			profiles:     []VMProfileConfig{},
			wantMinCount: 1,
			wantDefaults: true,
		},
		{
			name: "Custom profiles are returned",
			profiles: []VMProfileConfig{
				{ID: "custom-1", Name: "Custom 1", Sockets: 1, Cores: 1, RAMGB: 2, DiskGB: 24, DiskBus: "virtio", Enabled: true},
			},
			wantMinCount: 1,
			wantDefaults: false,
		},
		{
			name: "Multiple custom profiles",
			profiles: []VMProfileConfig{
				{ID: "custom-1", Name: "Custom 1", Sockets: 1, Cores: 1, RAMGB: 2, DiskGB: 24, DiskBus: "virtio", Enabled: true},
				{ID: "custom-2", Name: "Custom 2", Sockets: 1, Cores: 2, RAMGB: 4, DiskGB: 32, DiskBus: "virtio", Enabled: true},
			},
			wantMinCount: 2,
			wantDefaults: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := &AppSettings{VMProfiles: tt.profiles}
			got := settings.GetVMProfiles()

			if len(got) < tt.wantMinCount {
				t.Errorf("GetVMProfiles() returned %d profiles, want at least %d", len(got), tt.wantMinCount)
			}

			if tt.wantDefaults && len(got) == 0 {
				t.Error("GetVMProfiles() should return defaults when VMProfiles is empty")
			}

			if !tt.wantDefaults && len(got) != len(tt.profiles) {
				t.Errorf("GetVMProfiles() returned %d profiles, want %d", len(got), len(tt.profiles))
			}
		})
	}
}

func TestAppSettings_GetEnabledVMProfiles(t *testing.T) {
	tests := []struct {
		name      string
		profiles  []VMProfileConfig
		wantCount int
	}{
		{
			name:      "Empty profiles returns enabled defaults",
			profiles:  []VMProfileConfig{},
			wantCount: len(DefaultVMProfiles()), // All default profiles are enabled
		},
		{
			name: "Only enabled profiles returned",
			profiles: []VMProfileConfig{
				{ID: "enabled-1", Name: "Enabled 1", Sockets: 1, Cores: 1, RAMGB: 2, DiskGB: 24, DiskBus: "virtio", Enabled: true},
				{ID: "disabled-1", Name: "Disabled 1", Sockets: 1, Cores: 1, RAMGB: 2, DiskGB: 24, DiskBus: "virtio", Enabled: false},
			},
			wantCount: 1,
		},
		{
			name: "All profiles enabled",
			profiles: []VMProfileConfig{
				{ID: "enabled-1", Name: "Enabled 1", Sockets: 1, Cores: 1, RAMGB: 2, DiskGB: 24, DiskBus: "virtio", Enabled: true},
				{ID: "enabled-2", Name: "Enabled 2", Sockets: 1, Cores: 2, RAMGB: 4, DiskGB: 32, DiskBus: "virtio", Enabled: true},
			},
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := &AppSettings{VMProfiles: tt.profiles}
			got := settings.GetEnabledVMProfiles()

			if len(got) != tt.wantCount {
				t.Errorf("GetEnabledVMProfiles() returned %d profiles, want %d", len(got), tt.wantCount)
			}

			// Verify all returned profiles are enabled
			for _, p := range got {
				if !p.Enabled {
					t.Errorf("GetEnabledVMProfiles() returned disabled profile: %s", p.ID)
				}
			}
		})
	}
}

func TestAppSettings_AddOrUpdateVMProfile(t *testing.T) {
	settings := &AppSettings{VMProfiles: []VMProfileConfig{}}

	// Add new profile
	profile1 := VMProfileConfig{
		ID: "test-1", Name: "Test 1", Sockets: 1, Cores: 1, RAMGB: 2, DiskGB: 24, DiskBus: "virtio", Enabled: true,
	}
	settings.AddOrUpdateVMProfile(profile1)

	if len(settings.VMProfiles) != 1 {
		t.Errorf("AddOrUpdateVMProfile() added profile, expected 1, got %d", len(settings.VMProfiles))
	}

	if settings.VMProfiles[0].ID != "test-1" {
		t.Errorf("AddOrUpdateVMProfile() added wrong profile ID")
	}

	// Update existing profile
	profile1Updated := VMProfileConfig{
		ID: "test-1", Name: "Test 1 Updated", Sockets: 2, Cores: 2, RAMGB: 4, DiskGB: 32, DiskBus: "virtio", Enabled: true,
	}
	settings.AddOrUpdateVMProfile(profile1Updated)

	if len(settings.VMProfiles) != 1 {
		t.Errorf("AddOrUpdateVMProfile() updated profile, expected 1, got %d", len(settings.VMProfiles))
	}

	if settings.VMProfiles[0].Name != "Test 1 Updated" {
		t.Errorf("AddOrUpdateVMProfile() did not update profile name")
	}
}

func TestAppSettings_RemoveVMProfile(t *testing.T) {
	settings := &AppSettings{
		VMProfiles: []VMProfileConfig{
			{ID: "test-1", Name: "Test 1", Sockets: 1, Cores: 1, RAMGB: 2, DiskGB: 24, DiskBus: "virtio", Enabled: true},
			{ID: "test-2", Name: "Test 2", Sockets: 1, Cores: 2, RAMGB: 4, DiskGB: 32, DiskBus: "virtio", Enabled: true},
		},
	}

	// Remove existing profile
	removed := settings.RemoveVMProfile("test-1")
	if !removed {
		t.Error("RemoveVMProfile() should return true when profile exists")
	}

	if len(settings.VMProfiles) != 1 {
		t.Errorf("RemoveVMProfile() removed profile, expected 1 remaining, got %d", len(settings.VMProfiles))
	}

	if settings.VMProfiles[0].ID != "test-2" {
		t.Error("RemoveVMProfile() removed wrong profile")
	}

	// Remove non-existent profile
	removed = settings.RemoveVMProfile("test-3")
	if removed {
		t.Error("RemoveVMProfile() should return false when profile does not exist")
	}

	if len(settings.VMProfiles) != 1 {
		t.Error("RemoveVMProfile() should not remove any profiles when ID not found")
	}
}
