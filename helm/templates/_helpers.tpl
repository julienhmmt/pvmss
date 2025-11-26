{{/*
Nom complet du chart, limité à 63 caractères (convention K8s)
*/}}
{{- define "pvmss.fullname" -}}
{{- printf "%s-%s" .Release.Name "pvmss" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Labels standards appliqués à chaque ressource
*/}}
{{- define "pvmss.labels" -}}
app.kubernetes.io/name: {{ include "pvmss.fullname" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion }}
{{- end -}}

{{/*
Fonction utilitaire pour fusionner des maps (utile pour extraAnnotations/extraLabels)
*/}}
{{- define "pvmss.merge" -}}
{{- $dst := dict -}}
{{- range $k, $v := . -}}
{{- $_ := set $dst $k $v -}}
{{- end -}}
{{- $dst -}}
{{- end -}}
