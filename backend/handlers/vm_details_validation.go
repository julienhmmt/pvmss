package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"pvmss/i18n"
	"pvmss/proxmox"

	"github.com/julienschmidt/httprouter"
)

// ValidationRequest represents a validation request payload.
type ValidationRequest struct {
	Value string `json:"value"`
	Node  string `json:"node"`
	Pool  string `json:"pool"`
}

// ValidationResponse represents a validation response.
type ValidationResponse struct {
	Valid   bool   `json:"valid"`
	Message string `json:"message"`
}

// ValidateVMIDHandler validates VM ID uniqueness.
func (h *VMHandler) ValidateVMIDHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("ValidateVMIDHandler", r)
	localizer := i18n.GetLocalizerFromRequest(r)

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	var req ValidationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error().Err(err).Msg("Failed to decode validation request")
		w.WriteHeader(http.StatusBadRequest)
		if encodeErr := json.NewEncoder(w).Encode(ValidationResponse{Valid: false, Message: i18n.Localize(localizer, "VM.Create.Validation.InvalidRequest")}); encodeErr != nil {
			log.Error().Err(encodeErr).Msg("Failed to encode error response")
		}
		return
	}

	vmidStr := strings.TrimSpace(req.Value)
	if vmidStr == "" {
		if encodeErr := json.NewEncoder(w).Encode(ValidationResponse{Valid: false, Message: i18n.Localize(localizer, "VM.Create.Validation.VMIDRequired")}); encodeErr != nil {
			log.Error().Err(encodeErr).Msg("Failed to encode error response")
		}
		return
	}

	vmidInt, err := strconv.Atoi(vmidStr)
	if err != nil || vmidInt <= 0 || vmidInt > 999999999 {
		if encodeErr := json.NewEncoder(w).Encode(ValidationResponse{Valid: false, Message: i18n.Localize(localizer, "VM.Create.Validation.VMIDRange")}); encodeErr != nil {
			log.Error().Err(encodeErr).Msg("Failed to encode error response")
		}
		return
	}

	client := h.stateManager.GetProxmoxClient()
	if client == nil {
		if encodeErr := json.NewEncoder(w).Encode(ValidationResponse{Valid: false, Message: i18n.Localize(localizer, "Proxmox.ConnectionError")}); encodeErr != nil {
			log.Error().Err(encodeErr).Msg("Failed to encode error response")
		}
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	restyClient, err := getDefaultRestyClient()
	if err != nil {
		log.Error().Err(err).Msg("Failed to create resty client")
		if encodeErr := json.NewEncoder(w).Encode(ValidationResponse{Valid: false, Message: i18n.Localize(localizer, "Error.InternalServer")}); encodeErr != nil {
			log.Error().Err(encodeErr).Msg("Failed to encode error response")
		}
		return
	}

	_, err = proxmox.GetVMConfigResty(ctx, restyClient, req.Node, vmidInt)
	if err == nil {
		if encodeErr := json.NewEncoder(w).Encode(ValidationResponse{Valid: false, Message: i18n.Localize(localizer, "VM.Create.Validation.VMIDExists")}); encodeErr != nil {
			log.Error().Err(encodeErr).Msg("Failed to encode error response")
		}
		return
	}

	if encodeErr := json.NewEncoder(w).Encode(ValidationResponse{Valid: true, Message: i18n.Localize(localizer, "VM.Create.Validation.VMIDAvailable")}); encodeErr != nil {
		log.Error().Err(encodeErr).Msg("Failed to encode error response")
	}
}

// ValidateVMNameHandler validates VM name.
func (h *VMHandler) ValidateVMNameHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("ValidateVMNameHandler", r)
	localizer := i18n.GetLocalizerFromRequest(r)

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	var req ValidationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error().Err(err).Msg("Failed to decode validation request")
		w.WriteHeader(http.StatusBadRequest)
		if encodeErr := json.NewEncoder(w).Encode(ValidationResponse{Valid: false, Message: "Invalid request"}); encodeErr != nil {
			log.Error().Err(encodeErr).Msg("Failed to encode error response")
		}
		return
	}

	name := strings.TrimSpace(req.Value)
	if name == "" {
		if encodeErr := json.NewEncoder(w).Encode(ValidationResponse{Valid: false, Message: i18n.Localize(localizer, "VM.Create.Validation.VMNameRequired")}); encodeErr != nil {
			log.Error().Err(encodeErr).Msg("Failed to encode error response")
		}
		return
	}

	if len(name) < 1 || len(name) > 100 {
		if encodeErr := json.NewEncoder(w).Encode(ValidationResponse{Valid: false, Message: i18n.Localize(localizer, "VM.Create.Validation.VMNameLength")}); encodeErr != nil {
			log.Error().Err(encodeErr).Msg("Failed to encode error response")
		}
		return
	}

	if strings.ContainsAny(name, "<>\"'&") {
		if encodeErr := json.NewEncoder(w).Encode(ValidationResponse{Valid: false, Message: i18n.Localize(localizer, "VM.Create.Validation.VMNameInvalidChars")}); encodeErr != nil {
			log.Error().Err(encodeErr).Msg("Failed to encode error response")
		}
		return
	}

	if encodeErr := json.NewEncoder(w).Encode(ValidationResponse{Valid: true, Message: i18n.Localize(localizer, "VM.Create.Validation.VMNameValid")}); encodeErr != nil {
		log.Error().Err(encodeErr).Msg("Failed to encode error response")
	}
}

// ValidateVLANHandler validates VLAN tag values (1-4096).
func (h *VMHandler) ValidateVLANHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("ValidateVLANHandler", r)
	localizer := i18n.GetLocalizerFromRequest(r)

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	var req ValidationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error().Err(err).Msg("Failed to decode validation request")
		w.WriteHeader(http.StatusBadRequest)
		if encodeErr := json.NewEncoder(w).Encode(ValidationResponse{Valid: false, Message: "Invalid request"}); encodeErr != nil {
			log.Error().Err(encodeErr).Msg("Failed to encode error response")
		}
		return
	}

	vlanStr := strings.TrimSpace(req.Value)
	if vlanStr == "" {
		if encodeErr := json.NewEncoder(w).Encode(ValidationResponse{Valid: true, Message: i18n.Localize(localizer, "VM.Create.Validation.VLANValid")}); encodeErr != nil {
			log.Error().Err(encodeErr).Msg("Failed to encode error response")
		}
		return
	}

	vlanID, err := strconv.Atoi(vlanStr)
	if err != nil {
		if encodeErr := json.NewEncoder(w).Encode(ValidationResponse{Valid: false, Message: i18n.Localize(localizer, "VM.Create.Validation.VLANNumeric")}); encodeErr != nil {
			log.Error().Err(encodeErr).Msg("Failed to encode error response")
		}
		return
	}

	if vlanID < 1 || vlanID > 4096 {
		if encodeErr := json.NewEncoder(w).Encode(ValidationResponse{Valid: false, Message: i18n.Localize(localizer, "VM.Create.Validation.VLANRange")}); encodeErr != nil {
			log.Error().Err(encodeErr).Msg("Failed to encode error response")
		}
		return
	}

	if encodeErr := json.NewEncoder(w).Encode(ValidationResponse{Valid: true, Message: i18n.Localize(localizer, "VM.Create.Validation.VLANValid")}); encodeErr != nil {
		log.Error().Err(encodeErr).Msg("Failed to encode error response")
	}
}
