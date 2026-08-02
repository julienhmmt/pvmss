package config

// Configuration holds the values required for the server to start.
// All required values are loaded from the environment; WebDir is optional
// and will be resolved at startup if omitted.
type Configuration struct {
	Port      int
	DBPath    string
	LogLevel  string
	LogFormat string
	LogOutput string
	WebDir    string
}
