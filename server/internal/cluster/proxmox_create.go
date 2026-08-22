package cluster

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// NextVMID implements Creator via GET /cluster/nextid — the single
// allocation point (FR-012), delegated entirely to Proxmox's own cluster-wide
// counter rather than reimplemented client-side.
func (p Proxmox) NextVMID(ctx context.Context) (int, error) {
	raw, err := p.rest().do(ctx, http.MethodGet, "/cluster/nextid", nil)
	if err != nil {
		return 0, err
	}

	var value any
	if err := decodeData(raw, &value); err != nil {
		return 0, fmt.Errorf("decode next vmid: %w", err)
	}

	switch v := value.(type) {
	case string:
		n, err := strconv.Atoi(v)
		if err != nil {
			return 0, fmt.Errorf("parse next vmid %q: %w", v, err)
		}

		return n, nil
	case float64:
		return int(v), nil
	default:
		return 0, fmt.Errorf("unexpected next vmid payload: %v", value)
	}
}

// CreateVM implements Creator via POST /nodes/{node}/qemu. spec's single
// CPUCores value becomes a one-socket VM with that many cores — matching how
// VM.CPUCores is itself derived elsewhere (fake.go's UpdateHardware:
// CPUCores = sockets * cores). Proxmox's own start=1 param folds the initial
// boot into the same task rather than a separate Action call, matching
// FR-022 exactly.
func (p Proxmox) CreateVM(ctx context.Context, spec VMSpec) (string, error) {
	form := url.Values{
		"vmid":    {strconv.Itoa(spec.VMID)},
		"name":    {spec.Name},
		"sockets": {"1"},
		"cores":   {strconv.Itoa(spec.CPUCores)},
		"memory":  {strconv.Itoa(spec.MemoryMB)},
	}

	if spec.Pool != "" {
		form.Set("pool", spec.Pool)
	}

	if len(spec.Tags) > 0 {
		form.Set("tags", strings.Join(spec.Tags, ";"))
	}

	if spec.Disk.Storage != "" {
		form.Set(spec.Disk.Bus+"0", fmt.Sprintf("%s:%d", spec.Disk.Storage, spec.Disk.SizeGB))

		if spec.Disk.Bus == string(DiskBusSCSI) {
			form.Set("scsihw", "virtio-scsi-pci")
		}
	}

	if spec.Network.Bridge != "" {
		form.Set("net0", encodeNetValue(NetworkInterface{Model: spec.Network.Model, Bridge: spec.Network.Bridge}))
	}

	// Always provision a serial port (serial0) backed by a socket. This makes
	// the PVMSS Text/serial console work out of the box for every VM — without
	// it, Proxmox opens the termproxy tunnel and immediately closes it (EOF),
	// which surfaces as a black screen in the serial console. A socket-backed
	// serial port needs no host device and is safe to add unconditionally.
	form.Set("serial0", "socket")

	if spec.ISO != nil {
		form.Set(cdromDiskKey, fmt.Sprintf("%s:iso/%s,media=cdrom", spec.ISO.Storage, spec.ISO.File))
	}

	if spec.StartAfterCreate {
		form.Set("start", "1")
	}

	raw, err := p.rest().do(ctx, http.MethodPost, fmt.Sprintf("/nodes/%s/qemu", url.PathEscape(spec.Node)), form)
	if err != nil {
		return "", err
	}

	var upid string
	if err := decodeData(raw, &upid); err != nil {
		return "", fmt.Errorf("decode create task: %w", err)
	}

	return upid, nil
}

// TaskStatus implements Creator. The node a task ran on is embedded in its
// own UPID ("UPID:<node>:..."), which is how the Proxmox API itself expects
// task status to be looked up — there is no node-independent endpoint.
func (p Proxmox) TaskStatus(ctx context.Context, upid string) (TaskStatus, error) {
	node, err := proxmoxUPIDNode(upid)
	if err != nil {
		return TaskStatus{}, err
	}

	rest := p.rest()

	raw, err := rest.do(ctx, http.MethodGet, fmt.Sprintf("/nodes/%s/tasks/%s/status", url.PathEscape(node), url.PathEscape(upid)), nil)
	if err != nil {
		return TaskStatus{}, err
	}

	var status struct {
		Status     string `json:"status"`     // "running" | "stopped"
		ExitStatus string `json:"exitstatus"` // "OK" on success; present only once stopped
	}
	if err := decodeData(raw, &status); err != nil {
		return TaskStatus{}, fmt.Errorf("decode task status: %w", err)
	}

	// Best-effort: a log fetch failure should not hide the actual task state.
	log, _ := proxmoxTaskLog(ctx, rest, node, upid)

	result := TaskStatus{UPID: upid, Log: log}

	switch {
	case status.Status == string(VMRunning):
		result.State = TaskRunning
	case status.ExitStatus == "OK":
		result.State = TaskOK
	default:
		result.State = TaskError
		result.ExitMessage = status.ExitStatus
	}

	return result, nil
}

func proxmoxUPIDNode(upid string) (string, error) {
	parts := strings.Split(upid, ":")
	if len(parts) < 2 || parts[0] != "UPID" || parts[1] == "" {
		return "", fmt.Errorf("%w: malformed upid %q", ErrNotFound, upid)
	}

	return parts[1], nil
}

func proxmoxTaskLog(ctx context.Context, rest proxmoxRESTClient, node, upid string) ([]string, error) {
	raw, err := rest.do(ctx, http.MethodGet, fmt.Sprintf("/nodes/%s/tasks/%s/log", url.PathEscape(node), url.PathEscape(upid)), nil)
	if err != nil {
		return nil, err
	}

	var rows []struct {
		T string `json:"t"`
	}
	if err := decodeData(raw, &rows); err != nil {
		return nil, fmt.Errorf("decode task log: %w", err)
	}

	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		lines = append(lines, row.T)
	}

	return lines, nil
}
