package httpapi

import (
	"strings"

	"pvmss/server/internal/cluster"
	"pvmss/server/internal/store"
)

// tlsRequest carries per-cluster TLS material. A nil field keeps the stored
// value; an empty string clears it. The client key is write-only.
type tlsRequest struct {
	CAPEM            *string `json:"caPem"`
	NodeFingerprints *string `json:"nodeFingerprints"`
	ClientCertPEM    *string `json:"clientCertPem"`
	ClientKeyPEM     *string `json:"clientKeyPem"`
}

// tlsDTO is the response view: certificate subjects and expiry only, never
// the private key (just whether one is set).
type tlsDTO struct {
	CASet              bool     `json:"caSet"`
	CASubject          string   `json:"caSubject,omitempty"`
	CANotAfter         *string  `json:"caNotAfter"`
	NodeFingerprints   []string `json:"nodeFingerprints"`
	ClientCertSet      bool     `json:"clientCertSet"`
	ClientCertSubject  string   `json:"clientCertSubject,omitempty"`
	ClientCertNotAfter *string  `json:"clientCertNotAfter"`
	ClientKeySet       bool     `json:"clientKeySet"`
}

// resolveTLS merges a request over the current material and validates it.
// The error text is safe to return to the admin.
func resolveTLS(current store.ClusterTLS, req *tlsRequest) (store.ClusterTLS, error) {
	merged := current
	if req != nil {
		if req.CAPEM != nil {
			merged.CAPEM = *req.CAPEM
		}
		if req.NodeFingerprints != nil {
			merged.NodeFingerprints = *req.NodeFingerprints
		}
		if req.ClientCertPEM != nil {
			merged.ClientCertPEM = *req.ClientCertPEM
		}
		if req.ClientKeyPEM != nil {
			merged.ClientKeyPEM = *req.ClientKeyPEM
		}
	}
	return cluster.ValidateTLS(merged)
}

// trustChanged reports whether the verification anchors differ, which would
// let the stored token secret reach a different peer.
func trustChanged(a, b store.ClusterTLS) bool {
	return strings.TrimSpace(a.CAPEM) != strings.TrimSpace(b.CAPEM) || a.NodeFingerprints != b.NodeFingerprints
}

func certTime(info *cluster.CertInfo) *string {
	if info == nil {
		return nil
	}
	return formatTime(&info.NotAfter)
}

func toTLSDTO(t store.ClusterTLS) tlsDTO {
	dto := tlsDTO{NodeFingerprints: []string{}, ClientKeySet: t.ClientKeyPEM != ""}
	if ca := cluster.CertSummary(t.CAPEM); ca != nil {
		dto.CASet, dto.CASubject, dto.CANotAfter = true, ca.Subject, certTime(ca)
	}
	if fps, err := cluster.NormalizeFingerprints(t.NodeFingerprints); err == nil && len(fps) > 0 {
		dto.NodeFingerprints = fps
	}
	if cc := cluster.CertSummary(t.ClientCertPEM); cc != nil {
		dto.ClientCertSet, dto.ClientCertSubject, dto.ClientCertNotAfter = true, cc.Subject, certTime(cc)
	}
	return dto
}
