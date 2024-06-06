
{{- define "terraform-controller.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}


{{- define "terraform-controller.fullname" -}}
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


{{- define "terraform-controller.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}


{{- define "terraform-controller.labels" -}}
helm.sh/chart: {{ include "terraform-controller.chart" . }}
{{ include "terraform-controller.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app: terraform-controller
{{- end }}


{{- define "terraform-controller.selectorLabels" -}}
app.kubernetes.io/name: {{ include "terraform-controller.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app: terraform-controller
{{- end }}


{{- define "terraform-controller.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "terraform-controller.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}
