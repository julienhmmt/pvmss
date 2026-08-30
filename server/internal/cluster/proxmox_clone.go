package cluster

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// CloneVM implements Creator via POST /nodes/{sourceNode}/qemu/{sourceVMID}/clone
// (US2/issue-02). The form fields mirror ProxMate's minimal clone: newid, name,
// full, pool, and optional storage (only sent for a full clone to a different
// storage). A linked clone omits storage so Proxmox keeps the disk on the
// source's storage. TargetNode is intentionally not sent (D2b: cross-node clone
// is forbidden; the clone stays on SourceNode). Pool is always sent so the
// cloned VM lands in the actor's personal pool (FR-004).
func (p Proxmox) CloneVM(ctx context.Context, spec CloneSpec) (string, error) {
	form := url.Values{
		"newid": {strconv.Itoa(spec.NewVMID)},
		"name":  {spec.Name},
		"full":  {boolToStr(spec.Full)},
		"pool":  {spec.Pool},
	}

	if spec.Full && spec.Storage != "" {
		form.Set("storage", spec.Storage)
	}

	raw, err := p.rest().do(ctx, http.MethodPost,
		fmt.Sprintf("/nodes/%s/qemu/%d/clone", url.PathEscape(spec.SourceNode), spec.SourceVMID), form)
	if err != nil {
		return "", wrapVMIDCollision(err)
	}

	var upid string
	if err := decodeData(raw, &upid); err != nil {
		return "", fmt.Errorf("decode clone task: %w", err)
	}

	return upid, nil
}

// boolToStr returns "1" for true and "0" for false — the Proxmox API's boolean
// encoding (matches ProxMate's `opts.full ? '1' : '0'`).
func boolToStr(b bool) string {
	if b {
		return "1"
	}

	return "0"
}
