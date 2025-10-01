package main

// AppSettings represents the application configuration
// This struct is used to unmarshal the settings.json file
// and make the configuration available throughout the application
type AppSettings struct {
	Tags   []string               `json:"tags"`
	ISOs   []string               `json:"isos"`
	VMBRs  []string               `json:"vmbrs"`
	Limits map[string]interface{} `json:"limits"`
}
