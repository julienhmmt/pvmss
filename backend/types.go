package main

// AppSettings represents the application configuration
// This struct is populated from the database and cached in memory
type AppSettings struct {
	Tags   []string               `json:"tags"`
	ISOs   []string               `json:"isos"`
	VMBRs  []string               `json:"vmbrs"`
	Limits map[string]interface{} `json:"limits"`
}
