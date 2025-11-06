package tests

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNoHardcodedDefaults verifies that there are NO hardcoded defaults in vm_create.go
// ALL values MUST come from settings.json
func TestNoHardcodedDefaults(t *testing.T) {
	// Read vm_create.go source code
	sourceCode, err := os.ReadFile("../handlers/vm_create.go")
	if err != nil {
		t.Fatal("Could not read vm_create.go")
	}

	code := string(sourceCode)

	// These patterns should NOT exist (hardcoded defaults)
	forbiddenPatterns := []struct {
		pattern     string
		description string
	}{
		{"vmRamMinMB, vmRamMaxMB := 512", "Hardcoded RAM min/max 512/32768"},
		{"vmRamMinMB, vmRamMaxMB := 1024, 4096", "Hardcoded RAM min/max 1024/4096"},
		{"socketsMin, socketsMax := 1, 8", "Hardcoded sockets 1/8"},
		{"coresMin, coresMax := 1, 32", "Hardcoded cores 1/32"},
		{"diskMin, diskMax := 1, 1024", "Hardcoded disk 1/1024"},
		{"memoryMin, memoryMax = 512, 32768", "Hardcoded memory 512/32768"},
		{"memoryMin, memoryMax = 1024, 4096", "Hardcoded memory 1024/4096"},
		{"diskMin, diskMax = 1, 1024", "Hardcoded disk 1/1024"},
		{"diskMin, diskMax = 6, 12", "Hardcoded disk 6/12"},
	}

	for _, forbidden := range forbiddenPatterns {
		assert.NotContains(t, code, forbidden.pattern,
			"vm_create.go should NOT contain hardcoded defaults: %s", forbidden.description)
	}

	// These patterns MUST exist (reading from settings)
	requiredPatterns := []struct {
		pattern     string
		description string
	}{
		{"var vmRamMinMB, vmRamMaxMB int", "RAM variables without initialization"},
		{"var socketsMin, socketsMax int", "Sockets variables without initialization"},
		{"var coresMin, coresMax int", "Cores variables without initialization"},
		{"var diskMin, diskMax int", "Disk variables without initialization"},
		{"if settings != nil && settings.Limits != nil", "Settings check"},
		{"int(minVal) * 1024", "GB to MB conversion for RAM"},
	}

	for _, required := range requiredPatterns {
		assert.Contains(t, code, required.pattern,
			"vm_create.go MUST contain: %s", required.description)
	}
}

// TestSettingsJSONUsage verifies settings.json format
func TestSettingsJSONUsage(t *testing.T) {
	// Read settings.json
	data, err := os.ReadFile("../settings.json")
	if err != nil {
		t.Skip("settings.json not found")
	}

	var settings map[string]interface{}
	err = json.Unmarshal(data, &settings)
	assert.NoError(t, err, "settings.json should be valid JSON")

	limits, ok := settings["limits"].(map[string]interface{})
	assert.True(t, ok, "limits should exist in settings.json")

	vmLimits, ok := limits["vm"].(map[string]interface{})
	assert.True(t, ok, "vm limits should exist")

	// Verify units in settings.json
	t.Run("RAM and Disk are in GB", func(t *testing.T) {
		ramLimits, ok := vmLimits["ram"].(map[string]interface{})
		assert.True(t, ok, "RAM limits should exist")

		minRAM := ramLimits["min"].(float64)
		maxRAM := ramLimits["max"].(float64)

		// RAM should be in reasonable GB range (0.5GB to 256GB)
		assert.GreaterOrEqual(t, minRAM, 0.5, "Min RAM should be >= 0.5GB")
		assert.LessOrEqual(t, maxRAM, 256.0, "Max RAM should be <= 256GB")
		assert.Less(t, minRAM, maxRAM, "Min RAM < Max RAM")

		diskLimits, ok := vmLimits["disk"].(map[string]interface{})
		assert.True(t, ok, "Disk limits should exist")

		minDisk := diskLimits["min"].(float64)
		maxDisk := diskLimits["max"].(float64)

		// Disk should be in reasonable GB range (1GB to 10TB)
		assert.GreaterOrEqual(t, minDisk, 1.0, "Min disk should be >= 1GB")
		assert.LessOrEqual(t, maxDisk, 10240.0, "Max disk should be <= 10TB")
		assert.Less(t, minDisk, maxDisk, "Min disk < Max disk")
	})

	t.Run("Sockets and Cores are integers", func(t *testing.T) {
		socketsLimits, ok := vmLimits["sockets"].(map[string]interface{})
		assert.True(t, ok, "Sockets limits should exist")

		minSockets := socketsLimits["min"].(float64)
		maxSockets := socketsLimits["max"].(float64)

		// Should be whole numbers
		assert.Equal(t, minSockets, float64(int(minSockets)), "Min sockets should be integer")
		assert.Equal(t, maxSockets, float64(int(maxSockets)), "Max sockets should be integer")

		coresLimits, ok := vmLimits["cores"].(map[string]interface{})
		assert.True(t, ok, "Cores limits should exist")

		minCores := coresLimits["min"].(float64)
		maxCores := coresLimits["max"].(float64)

		// Should be whole numbers
		assert.Equal(t, minCores, float64(int(minCores)), "Min cores should be integer")
		assert.Equal(t, maxCores, float64(int(maxCores)), "Max cores should be integer")
	})
}

// TestSettingsJSONStructure validates settings.json structure
func TestSettingsJSONStructure(t *testing.T) {
	data, err := os.ReadFile("../settings.json")
	if err != nil {
		t.Skip("settings.json not found")
	}

	var settings map[string]interface{}
	err = json.Unmarshal(data, &settings)
	assert.NoError(t, err, "settings.json should be valid JSON")

	// Check required fields
	t.Run("Required top-level fields", func(t *testing.T) {
		assert.Contains(t, settings, "tags", "tags field should exist")
		assert.Contains(t, settings, "isos", "isos field should exist")
		assert.Contains(t, settings, "vmbrs", "vmbrs field should exist")
		assert.Contains(t, settings, "limits", "limits field should exist")
		assert.Contains(t, settings, "max_network_cards", "max_network_cards field should exist")
		assert.Contains(t, settings, "max_disk_per_vm", "max_disk_per_vm field should exist")
	})

	// Check limits structure
	t.Run("Limits structure", func(t *testing.T) {
		limits, ok := settings["limits"].(map[string]interface{})
		assert.True(t, ok, "limits should be an object")

		assert.Contains(t, limits, "vm", "vm limits should exist")
		assert.Contains(t, limits, "nodes", "nodes limits should exist")

		vmLimits, ok := limits["vm"].(map[string]interface{})
		assert.True(t, ok, "vm limits should be an object")

		assert.Contains(t, vmLimits, "ram", "RAM limits should exist")
		assert.Contains(t, vmLimits, "cores", "Cores limits should exist")
		assert.Contains(t, vmLimits, "sockets", "Sockets limits should exist")
		assert.Contains(t, vmLimits, "disk", "Disk limits should exist")
	})

	// Check limit values are reasonable
	t.Run("Limit values are reasonable", func(t *testing.T) {
		limits := settings["limits"].(map[string]interface{})
		vmLimits := limits["vm"].(map[string]interface{})

		// RAM: should be between 0.5GB and 256GB
		ramLimits := vmLimits["ram"].(map[string]interface{})
		minRAM := ramLimits["min"].(float64)
		maxRAM := ramLimits["max"].(float64)
		assert.GreaterOrEqual(t, minRAM, 0.5, "Min RAM should be at least 512MB")
		assert.LessOrEqual(t, maxRAM, 256.0, "Max RAM should be at most 256GB")
		assert.Less(t, minRAM, maxRAM, "Min RAM should be less than max RAM")

		// Cores: should be between 1 and 128
		coresLimits := vmLimits["cores"].(map[string]interface{})
		minCores := coresLimits["min"].(float64)
		maxCores := coresLimits["max"].(float64)
		assert.GreaterOrEqual(t, minCores, 1.0, "Min cores should be at least 1")
		assert.LessOrEqual(t, maxCores, 128.0, "Max cores should be at most 128")
		assert.LessOrEqual(t, minCores, maxCores, "Min cores should be <= max cores")

		// Disk: should be between 1GB and 10TB
		diskLimits := vmLimits["disk"].(map[string]interface{})
		minDisk := diskLimits["min"].(float64)
		maxDisk := diskLimits["max"].(float64)
		assert.GreaterOrEqual(t, minDisk, 1.0, "Min disk should be at least 1GB")
		assert.LessOrEqual(t, maxDisk, 10240.0, "Max disk should be at most 10TB")
		assert.Less(t, minDisk, maxDisk, "Min disk should be less than max disk")
	})
}
