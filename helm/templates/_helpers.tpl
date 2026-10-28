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
{{- if and $hash (not (hasPrefix "$2" $hash)) -}}
{{- fail "secrets.adminPasswordHash must be a bcrypt hash starting with $2 (check that $ was not stripped; prefer secrets.adminPasswordHashBase64)" -}}
{{- end -}}
{{- $hash -}}
{{- end -}}
