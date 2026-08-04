package config

// Configuration holds the values required for the server to start.
// All required values are loaded from the environment; WebDir is optional
// and will be resolved at startup if omitted. Host defaults to 127.0.0.1.
type Configuration struct {
	Host              string
	Port              int
	DBPath            string
	LogLevel          string
	LogFormat         string
	LogOutput         string
	WebDir            string
	ClusterSource     string
	SessionSecret     string
	AdminPasswordHash string
}
