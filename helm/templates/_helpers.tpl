{{/*
Full name of the chart, truncated at 63 chars (Kubernetes naming convention).
*/}}
{{- define "pvmss.fullname" -}}
{{- printf "%s-%s" .Release.Name "pvmss" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Selector labels (immutable across upgrades; do not include appVersion here).
*/}}
{{- define "pvmss.selectorLabels" -}}
app.kubernetes.io/name: {{ include "pvmss.fullname" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{/*
Standard labels applied to every resource.
*/}}
{{- define "pvmss.labels" -}}
{{ include "pvmss.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- with .Values.extraLabels }}
{{ toYaml . }}
{{- end }}
{{- end -}}

{{/*
Name of the Secret holding the three secret keys (chart-managed or existing).
*/}}
{{- define "pvmss.secretName" -}}
{{- default (printf "%s-secrets" (include "pvmss.fullname" .)) .Values.secrets.existingSecret -}}
{{- end -}}

{{/*
Bcrypt admin hash, from the plain value or its base64 form. The base64 form
exists because "$" in "$2a$10$..." is mangled by ArgoCD parameter/env
substitution and by shells; base64 has no "$".
*/}}
{{- define "pvmss.adminPasswordHash" -}}
{{- $hash := .Values.secrets.adminPasswordHash -}}
{{- with .Values.secrets.adminPasswordHashBase64 -}}
{{- $hash = b64dec . | trim -}}
{{- end -}}
{{- if and $hash (not (regexMatch "^\\$2[abxy]?\\$[0-9]{2}\\$[./A-Za-z0-9]{53}$" $hash)) -}}
{{- fail "secrets.adminPasswordHash must be a full bcrypt hash ($2y$10$ + 53 chars); a $ was probably stripped, prefer secrets.adminPasswordHashBase64" -}}
{{- end -}}
{{- $hash -}}
{{- end -}}

{{/*
Kubernetes TLS Secret holding the PVMSS server certificate.
*/}}
{{- define "pvmss.tlsSecretName" -}}
{{- default (printf "%s-tls" (include "pvmss.fullname" .)) .Values.tls.secretName -}}
{{- end -}}

{{/*
Secret holding the client CA bundle (mTLS). Required, never the TLS Secret:
its ca.crt is the server issuer, which may sign far more than the clients
allowed to reach PVMSS (a cluster-wide ClusterIssuer, for example).
*/}}
{{- define "pvmss.clientCASecretName" -}}
{{- required "tls.clientCA.secretName is required when tls.clientCA.enabled: use a CA dedicated to the proxies' client certificates" .Values.tls.clientCA.secretName -}}
{{- end -}}

{{/*
Probe handler. Strict mTLS refuses the kubelet (no client certificate), so it
gets a tcpSocket probe; otherwise /health over HTTP or HTTPS.
*/}}
{{- define "pvmss.probeHandler" -}}
{{- if and .Values.tls.enabled .Values.tls.clientCA.enabled (eq .Values.tls.clientCA.clientAuth "require") -}}
tcpSocket:
  port: backend
{{- else -}}
httpGet:
  path: /health
  port: backend
  scheme: {{ ternary "HTTPS" "HTTP" .Values.tls.enabled }}
{{- end -}}
{{- end -}}
