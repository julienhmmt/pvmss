package httpapi

// Pinger is the dependency the health check probes.
type Pinger interface {
	Ping() error
}
