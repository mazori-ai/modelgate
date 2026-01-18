{{/*
Expand the name of the chart.
*/}}
{{- define "modelgate.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "modelgate.fullname" -}}
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

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "modelgate.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "modelgate.labels" -}}
helm.sh/chart: {{ include "modelgate.chart" . }}
{{ include "modelgate.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "modelgate.selectorLabels" -}}
app.kubernetes.io/name: {{ include "modelgate.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "modelgate.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "modelgate.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Database host - use PostgreSQL subchart if enabled
*/}}
{{- define "modelgate.databaseHost" -}}
{{- if .Values.postgresql.enabled }}
{{- printf "%s-postgresql" .Release.Name }}
{{- else }}
{{- .Values.config.database.host }}
{{- end }}
{{- end }}

{{/*
Database password - from subchart or values
*/}}
{{- define "modelgate.databasePassword" -}}
{{- if .Values.postgresql.enabled }}
{{- .Values.postgresql.auth.password }}
{{- else }}
{{- .Values.config.database.password }}
{{- end }}
{{- end }}

{{/*
Database user - from subchart or values
*/}}
{{- define "modelgate.databaseUser" -}}
{{- if .Values.postgresql.enabled }}
{{- .Values.postgresql.auth.username }}
{{- else }}
{{- .Values.config.database.user }}
{{- end }}
{{- end }}

{{/*
Database name - from subchart or values
*/}}
{{- define "modelgate.databaseName" -}}
{{- if .Values.postgresql.enabled }}
{{- .Values.postgresql.auth.database }}
{{- else }}
{{- .Values.config.database.database }}
{{- end }}
{{- end }}

{{/*
Ollama URL - use Ollama deployment if enabled
*/}}
{{- define "modelgate.ollamaUrl" -}}
{{- if .Values.ollama.enabled }}
{{- printf "http://%s-ollama:%d" .Release.Name (int .Values.ollama.service.port) }}
{{- else }}
{{- .Values.config.embedder.baseUrl }}
{{- end }}
{{- end }}

{{/*
Ollama fullname
*/}}
{{- define "modelgate.ollama.fullname" -}}
{{- printf "%s-ollama" .Release.Name | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Ollama labels
*/}}
{{- define "modelgate.ollama.labels" -}}
helm.sh/chart: {{ include "modelgate.chart" . }}
{{ include "modelgate.ollama.selectorLabels" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Ollama selector labels
*/}}
{{- define "modelgate.ollama.selectorLabels" -}}
app.kubernetes.io/name: {{ include "modelgate.name" . }}-ollama
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: ollama
{{- end }}

