package tests

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/julienschmidt/httprouter"
	"pvmss/handlers"
	"pvmss/state"
)

// TestStoragePageHandler tests the StoragePageHandler with mock data
func TestStoragePageHandler(t *testing.T) {
	// Initialize the mock state manager
	mockStateManager := NewMockStateManager()

	// Create mock storages
	mockStorages := []MockStorage{
		{
			Storage:     "local",
			Type:        "dir",
			Used:        "1073741824",
			Total:       "53687091200",
			Avail:       "52613332992",
			Active:      1,
			Description: "Local storage",
		},
		{
			Storage:     "nfs",
			Type:        "nfs",
			Used:        "2147483648",
			Total:       "107374182400",
			Avail:       "105226698752",
			Active:      1,
			Description: "Network storage",
		},
	}

	// Create a mock Proxmox client with our mock data
	mockClient := NewMockProxmoxClient(
		"https://mock-proxmox:8006/api2/json",
		"mock-user@pve!test",
		"mock-token",
		true,
		WithMockStorages(mockStorages),
	)

	// Set the mock client in the mock state manager
	mockStateManager.SetProxmoxClient(mockClient)

	// Create mock settings
	mockSettings := &state.AppSettings{
		EnabledStorages: []string{"local"}, // Only local storage enabled
	}

	// Set the mock settings in the mock state manager
	mockStateManager.SetSettingsWithoutSave(mockSettings)

	// Create storage handler
	storageHandler := handlers.NewStorageHandler(mockStateManager)

	// Create a request
	req, err := http.NewRequest("GET", "/admin/storage", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create a response recorder
	rr := httptest.NewRecorder()

	// Create router
	router := httprouter.New()

	// Register the storage handler routes
	storageHandler.RegisterRoutes(router)

	// Serve the request
	router.ServeHTTP(rr, req)

	// Check the status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Check the response body for expected content
	expectedStrings := []string{
		"local",
		"nfs",
		"Local storage",
		"Network storage",
	}

	body := rr.Body.String()
	for _, expected := range expectedStrings {
		if !contains(body, expected) {
			t.Errorf("handler response body does not contain expected string: %s", expected)
		}
	}
}

// TestUpdateStorageHandler tests the UpdateStorageHandler with mock data
func TestUpdateStorageHandler(t *testing.T) {
	// Initialize the mock state manager
	mockStateManager := NewMockStateManager()

	// Create mock settings
	mockSettings := &state.AppSettings{
		EnabledStorages: []string{"local", "nfs"}, // Both storages enabled
	}

	// Set the mock settings in the mock state manager
	mockStateManager.SetSettingsWithoutSave(mockSettings)

	// Create storage handler
	storageHandler := handlers.NewStorageHandler(mockStateManager)

	// Create a form request with updated storage settings
	formData := "enabled_storages=local&enabled_storages=nfs"
	req, err := http.NewRequest("POST", "/admin/storage/update", strings.NewReader(formData))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Create a response recorder
	rr := httptest.NewRecorder()

	// Create router
	router := httprouter.New()

	// Register the storage handler routes
	storageHandler.RegisterRoutes(router)

	// Serve the request
	router.ServeHTTP(rr, req)

	// Check the status code (should be redirect)
	if status := rr.Code; status != http.StatusSeeOther {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusSeeOther)
	}

	// Check the redirect location
	expectedLocation := "/admin/storage?success=true"
	if location := rr.Header().Get("Location"); location != expectedLocation {
		t.Errorf("handler returned wrong redirect location: got %v want %v",
			location, expectedLocation)
	}
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
