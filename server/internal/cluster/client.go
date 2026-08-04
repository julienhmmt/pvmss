// Package cluster defines the contract for reading cluster data and its two
// production implementations: a real Proxmox client and a fake substitute.
package cluster

import (
	"context"
	"errors"
)

// Sentinel errors so callers can distinguish failure modes without string matching.
var (
	ErrUnreachable    = errors.New("cluster unreachable")
	ErrNotFound       = errors.New("not found")
	ErrNotImplemented = errors.New("not implemented")
)

// Client is the single contract for reading cluster data. Every implementation
// must behave identically from a caller's perspective (constitution XI).
type Client interface {
	ListNodes(ctx context.Context) ([]Node, error)
}

// NodeStatus is the operational state of a cluster node.
type NodeStatus string

const (
	NodeOnline  NodeStatus = "online"
	NodeOffline NodeStatus = "offline"
	NodeUnknown NodeStatus = "unknown"
)

// Node is a machine in the cluster.
type Node struct {
	Name         string
	Status       NodeStatus
	CPUCores     int
	CPUUsage     float64
	MemoryTotal  int64
	MemoryUsed   int64
	StorageTotal int64
	StorageUsed  int64
}

// VMStatus is the run state of a virtual machine.
type VMStatus string

const (
	VMRunning VMStatus = "running"
	VMStopped VMStatus = "stopped"
	VMPaused  VMStatus = "paused"
)

// VM is a guest belonging to a node. Carried by the fake dataset from T01 so
// later tranches have data to work with, but not surfaced by any endpoint
// until T04.
type VM struct {
	VMID        int
	Name        string
	Node        string
	Status      VMStatus
	Pool        string
	Tags        []string
	CPUCores    int
	MemoryTotal int64
}

// Storage is a storage backend attached to a node.
type Storage struct {
	Name  string
	Node  string
	Type  string
	Total int64
	Used  int64
}

// Pool is a tenancy anchor — one pool maps to one user (PD00).
type Pool struct {
	Name    string
	Comment string
}
