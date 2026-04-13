package proxmox

import (
	"context"
	"fmt"
	"net/url"
)

// TaskStatus represents the status of a Proxmox task.
type TaskStatus struct {
	UPID       string `json:"upid"`
	Status     string `json:"status"`
	ExitStatus string `json:"exitstatus"`
}

// GetTaskStatusResty fetches the status of a task by UPID.
// Endpoint: GET /nodes/{node}/tasks/{upid}/status
func GetTaskStatusResty(ctx context.Context, restyClient *RestyClient, node string, upid string) (*TaskStatus, error) {
	if restyClient == nil {
		return nil, fmt.Errorf("resty client is required")
	}
	if node == "" || upid == "" {
		return nil, fmt.Errorf("node and upid are required")
	}

	path := fmt.Sprintf("/nodes/%s/tasks/%s/status", url.PathEscape(node), url.PathEscape(upid))

	var response Response[TaskStatus]
	if err := restyClient.Get(ctx, path, &response); err != nil {
		return nil, err
	}

	return &response.Data, nil
}

// TaskLogEntry represents a single log line from a Proxmox task.
type TaskLogEntry struct {
	N int    `json:"n"`
	T string `json:"t"`
}

// GetTaskLogResty fetches the log entries of a task by UPID.
// Endpoint: GET /nodes/{node}/tasks/{upid}/log
func GetTaskLogResty(ctx context.Context, restyClient *RestyClient, node string, upid string) ([]TaskLogEntry, error) {
	if restyClient == nil {
		return nil, fmt.Errorf("resty client is required")
	}
	if node == "" || upid == "" {
		return nil, fmt.Errorf("node and upid are required")
	}

	path := fmt.Sprintf("/nodes/%s/tasks/%s/log", url.PathEscape(node), url.PathEscape(upid))

	var response Response[[]TaskLogEntry]
	if err := restyClient.Get(ctx, path, &response); err != nil {
		return nil, err
	}

	return response.Data, nil
}
