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
