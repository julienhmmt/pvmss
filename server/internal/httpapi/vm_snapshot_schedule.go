package httpapi

import (
	"errors"
	"net/http"
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
	"strings"
)

// snapshotScheduleRequest is the PUT body: frequency is daily, weekly or off.
type snapshotScheduleRequest struct {
	Frequency string `json:"frequency"`
	Retain    int    `json:"retain"`
}

// snapshotScheduleDTO is the schedule as shown on the VM page. LastRunAt is
// empty until the first automatic snapshot.
type snapshotScheduleDTO struct {
	Frequency string `json:"frequency"`
	Retain    int    `json:"retain"`
	LastRunAt string `json:"lastRunAt"`
}

// handleSchedule serves PUT and DELETE .../snapshot-schedule.
func (h *VMSnapshots) handleSchedule(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPut:
		h.handleScheduleSet(w, r)
	case http.MethodDelete:
		h.handleScheduleClear(w, r)
	default:
		w.Header().Set("Allow", "PUT, DELETE")
		h.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", msgMethodNotAllowed)
	}
}

func (h *VMSnapshots) handleScheduleSet(w http.ResponseWriter, r *http.Request) {
	var request snapshotScheduleRequest
	if err := decodeJSON(w, r, &request); err != nil {
		h.writeError(w, http.StatusBadRequest, codeInvalidRequest, msgInvalidRequestBody)
		return
	}

	deps, ok := h.scheduleDependencies(w, r)
	if !ok {
		return
	}

	if err := vm.SetSnapshotSchedule(r.Context(), deps, request.Frequency, request.Retain); err != nil {
		h.writeScheduleError(w, err)
		return
	}

	h.writeScheduleResult(w, r, deps, http.StatusOK)
}

func (h *VMSnapshots) handleScheduleClear(w http.ResponseWriter, r *http.Request) {
	deps, ok := h.scheduleDependencies(w, r)
	if !ok {
		return
	}

	if err := vm.ClearSnapshotSchedule(r.Context(), deps); err != nil {
		h.writeSnapshotError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// scheduleDependencies resolves the request's VM and cluster index. It does
// not need the cluster snapshot clients: a schedule is only stored here, the
// sweeper takes the snapshots later.
func (h *VMSnapshots) scheduleDependencies(w http.ResponseWriter, r *http.Request) (vm.ScheduleDependencies, bool) {
	identity, clusterName, vmid, ok := h.requestTarget(w, r)
	if !ok {
		return vm.ScheduleDependencies{}, false
	}

	index, ok := loadClusterIndex(h.resolver, clusterName, func(status int, code, message string) { h.writeError(w, status, code, message) })
	if !ok {
		return vm.ScheduleDependencies{}, false
	}

	return vm.ScheduleDependencies{Index: index, Actor: identity, ClusterName: clusterName, VMID: vmid, Policy: h.policy, Store: h.store}, true
}

// writeScheduleError maps the schedule-only validation errors to 400 and
// defers everything else to the shared snapshot error mapping.
func (h *VMSnapshots) writeScheduleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, vm.ErrInvalidSnapshotSchedule):
		h.writeError(w, http.StatusBadRequest, "invalid_schedule", err.Error())
	case errors.Is(err, vm.ErrScheduleRetainExceedsMax):
		h.writeError(w, http.StatusBadRequest, "retain_exceeds_max_snapshots", err.Error())
	default:
		h.writeSnapshotError(w, err)
	}
}

// writeScheduleResult answers a set with the stored schedule.
func (h *VMSnapshots) writeScheduleResult(w http.ResponseWriter, r *http.Request, deps vm.ScheduleDependencies, status int) {
	schedule, found, err := h.store.GetSnapshotSchedule(r.Context(), deps.ClusterName, deps.VMID)
	if err != nil || !found {
		h.writeError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)
		return
	}

	h.writePayload(w, status, scheduleDTOFromModel(schedule))
}

// scheduleListDTO returns the schedule for the list response, or nil when the
// VM has none. A read failure is logged and reported as no schedule so the
// snapshot list still renders.
func (h *VMSnapshots) scheduleListDTO(r *http.Request, clusterName string, vmid int) *snapshotScheduleDTO {
	if h.store == nil {
		return nil
	}

	schedule, found, err := h.store.GetSnapshotSchedule(r.Context(), clusterName, vmid)
	if err != nil {
		h.log.WarnContext(r.Context(), "failed to read snapshot schedule", "component", "httpapi", "error", err)
		return nil
	}

	if !found {
		return nil
	}

	dto := scheduleDTOFromModel(schedule)

	return &dto
}

func scheduleDTOFromModel(schedule store.SnapshotSchedule) snapshotScheduleDTO {
	dto := snapshotScheduleDTO{Frequency: schedule.Frequency, Retain: schedule.Retain}
	if !schedule.LastRunAt.IsZero() {
		dto.LastRunAt = schedule.LastRunAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00")
	}

	return dto
}

// isScheduleRoute reports whether the request targets the schedule resource
// rather than the snapshot collection or one snapshot.
func isScheduleRoute(path string) bool {
	return strings.HasSuffix(path, "/snapshot-schedule")
}
