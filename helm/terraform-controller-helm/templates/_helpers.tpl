
{{- define "terraform-controller-helm.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}


{{- define "terraform-controller-helm.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}


{{- define "terraform-controller-helm.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}


{{- define "terraform-controller-helm.labels" -}}
helm.sh/chart: {{ include "terraform-controller-helm.chart" . }}
{{ include "terraform-controller-helm.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app: terraform-controller-helm
{{- end }}


{{- define "terraform-controller-helm.selectorLabels" -}}
app.kubernetes.io/name: {{ include "terraform-controller-helm.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app: terraform-controller-helm
{{- end }}


{{- define "terraform-controller-helm.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "terraform-controller-helm.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}
