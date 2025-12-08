package proxmox

// Package proxmox previously exposed the Telmate-based Client in this file. As part of the
// refactor to separate Telmate and Resty clients, all Telmate logic now lives in
// telmate_client.go and Resty logic in resty_client.go. This stub file remains to satisfy
// existing tooling and structure checks that expect client.go to exist. It intentionally
// contains only documentation and does not declare any additional types or functions to
// avoid redeclaration conflicts with the real Telmate client defined in telmate_client.go.
//
// Summary of where to find functionality:
// - Telmate client (legacy): telmate_client.go
// - Resty client (preferred): resty_client.go
// - Client interfaces: interfaces.go
//
// When adding new Proxmox client features, prefer implementing them in the Resty client.
// The Telmate client is maintained only for backward compatibility during the migration.
