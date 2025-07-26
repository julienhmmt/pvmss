package state

// AppSettings represents the application configuration
type AppSettings struct {
	// AdminPassword is the bcrypt hashed password for admin access
	AdminPassword string                 `json:"admin_password"`
	Tags          []string               `json:"tags"`
	ISOs          []string               `json:"isos"`
	VMBRs         []string               `json:"vmbrs"`
	Limits        map[string]interface{} `json:"limits"`
}
