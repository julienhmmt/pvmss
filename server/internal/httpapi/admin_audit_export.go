package httpapi

import (
	"encoding/csv"
	"net/http"
	"pvmss/server/internal/logctx"
	"strconv"
	"time"
)

// maxAuditExportRows caps one CSV export; a larger result sets
// X-PVMSS-Truncated so the admin narrows the date range.
const maxAuditExportRows = 50_000

var auditCSVHeader = []string{
	"timestamp", "actor", "action", auditKeyCluster, auditKeyVMID,
	"target_type", "target_id", "detail", "ip", "severity",
}

// ServeAuditExport handles GET /api/v1/admin/audit/export. It takes the
// same filters as ServeAudit, ignores pagination and answers text/csv,
// newest first.
func (h *AdminOps) ServeAuditExport(w http.ResponseWriter, r *http.Request) {
	filter, ok := parseAuditFilter(w, r)
	if !ok {
		return
	}

	filter.Page = 1
	filter.PageSize = maxAuditExportRows

	result, err := h.store.ListAuditLog(r.Context(), filter)
	if err != nil {
		SetErrorMsg(w, "admin audit export failed", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition",
		`attachment; filename="pvmss-audit-`+time.Now().UTC().Format("20060102-150405")+`.csv"`)

	if result.Total > len(result.Items) {
		w.Header().Set("X-PVMSS-Truncated", "1")
	}

	cw := csv.NewWriter(w)
	_ = cw.Write(auditCSVHeader)

	for _, e := range result.Items {
		vmid := ""
		if e.VMID != nil {
			vmid = strconv.Itoa(*e.VMID)
		}

		_ = cw.Write([]string{
			e.Timestamp.UTC().Format(time.RFC3339),
			csvSafe(e.Actor), csvSafe(e.Action), csvSafe(e.Cluster), vmid,
			csvSafe(e.TargetType.String), csvSafe(e.TargetID.String),
			csvSafe(e.Detail.String), csvSafe(e.IPAddress.String), e.Severity,
		})
	}

	cw.Flush()

	// Headers are sent: a write failure here is a dropped client, log only.
	if err := cw.Error(); err != nil {
		logctx.From(r.Context()).WarnContext(r.Context(), "audit export write failed", "error", err)
	}
}

// csvSafe neutralises spreadsheet formula injection: a cell an admin opens
// in Excel/LibreOffice must not start with = + - @ or a tab/CR.
func csvSafe(s string) string {
	if s == "" {
		return s
	}

	switch s[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + s
	}

	return s
}
