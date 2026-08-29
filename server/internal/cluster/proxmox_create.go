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
// allocation point (FR-012), delegated to Proxmox's own cluster-wide counter
// rather than reimplemented client-side. The endpoint returns the smallest
// free ID at call time without reserving it, so two concurrent creations can
// collide; the caller handles ErrVMIDTaken by retrying (US5/issue-05 D5c).
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

// CreateVM implements Creator via POST /nodes/{node}/qemu. spec's Sockets
// and CPUCores values become the Proxmox form's sockets and cores keys —
// matching how VM.CPUCores is itself derived elsewhere (fake.go's
// UpdateHardware: CPUCores = sockets * cores). Proxmox's own start=1 param
// folds the initial boot into the same task rather than a separate Action
// call, matching FR-022 exactly.
func (p Proxmox) CreateVM(ctx context.Context, spec VMSpec) (string, error) {
	form := url.Values{
		"vmid":    {strconv.Itoa(spec.VMID)},
		"name":    {spec.Name},
		"sockets": {strconv.Itoa(spec.Sockets)},
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
		diskValue := fmt.Sprintf("%s:%d,discard=on", spec.Disk.Storage, spec.Disk.SizeGB)

		// US6/issue-06 D6a: iothread is gated on SCSI — it is not supported
		// on virtio/IDE/SATA and Proxmox silently ignores the option there,
		// but emitting it only where it works keeps the form clean.
		if spec.Disk.Bus == string(DiskBusSCSI) {
			diskValue += ",iothread=1"

			form.Set("scsihw", "virtio-scsi-pci")
		}

		form.Set(spec.Disk.Bus+"0", diskValue)
	}

	for i, nic := range spec.Network {
		if nic.Bridge == "" {
			continue
		}

		// US6/issue-06: pass VLAN, Firewall, MAC, and RateMbps through to
		// the encoder. Firewall is always true (D6a — imposed, not exposed).
		// VLAN is the admin-imposed isolation tag (D6b).
		form.Set(fmt.Sprintf("net%d", i), encodeNetValue(NetworkInterface{
			Model: nic.Model, Bridge: nic.Bridge, VLAN: nic.VLAN,
			Firewall: true, MAC: nic.MAC, RateMbps: nic.RateMbps,
		}))
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

	// US6/issue-06: UEFI (bios=ovmf) and TPM 2.0.
	setUEFIFormKeys(form, spec)

	if spec.StartAfterCreate {
		form.Set("start", "1")
	}

	raw, err := p.rest().do(ctx, http.MethodPost, fmt.Sprintf("/nodes/%s/qemu", url.PathEscape(spec.Node)), form)
	if err != nil {
		return "", wrapVMIDCollision(err)
	}

	var upid string
	if err := decodeData(raw, &upid); err != nil {
		return "", fmt.Errorf("decode create task: %w", err)
	}

	return upid, nil
}

// setUEFIFormKeys emits the UEFI/TPM form keys when BIOS is ovmf (US6/issue-06
// D6a). When BIOS is ovmf, machine is forced to q35 (UEFI requires q35 —
// pegaprox rule), efidisk0 is provisioned on the disk's storage, and
// tpmstate0 is added when TPM is set — never omitted silently (the pegaprox
// preset bug where tpm_version was set without tpm_storage). Extracted from
// CreateVM to keep its cyclomatic complexity under gocyclo's ceiling.
func setUEFIFormKeys(form url.Values, spec VMSpec) {
	if spec.BIOS != "ovmf" {
		return
	}

	form.Set("bios", "ovmf")

	machine := spec.Machine
	if machine == "" || machine == "i440fx" || machine == "pc" {
		machine = "q35"
	}

	form.Set("machine", machine)

	efiStorage := spec.Disk.Storage
	if efiStorage == "" {
		efiStorage = "local-lvm"
	}

	form.Set("efidisk0", efiStorage+":1,efitype=4m,pre-enrolled-keys=1")

	if spec.TPM {
		form.Set("tpmstate0", efiStorage+":1,version=v2.0")
	}
}

// wrapVMIDCollision inspects a Proxmox error for a VMID-already-exists
// rejection and wraps it with ErrVMIDTaken so the caller can retry with a
// fresh VMID (US5/issue-05 D5c). Proxmox returns HTTP 500 with a body like
// {"errors":{"vmid":"VMID '100' already exists"}}; the low-level client
// flattens that into a single error string, so a substring match is the
// only detection available without re-parsing the raw body.
func wrapVMIDCollision(err error) error {
	if err == nil {
		return nil
	}

	if strings.Contains(err.Error(), "already exists") {
		return fmt.Errorf("%w: %w", ErrVMIDTaken, err)
	}

	return err
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
	case status.ExitStatus == "OK" || strings.HasPrefix(status.ExitStatus, "WARNINGS"):
		// PVE returns WARNINGS for benign conditions (NUMA mismatch, local
		// disks) — both references accept it as success (lifecycle-04).
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
