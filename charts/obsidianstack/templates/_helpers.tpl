{{/*
Expand the name of the chart.
*/}}
{{- define "obsidianstack.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "obsidianstack.fullname" -}}
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
Chart label (name + version).
*/}}
{{- define "obsidianstack.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels applied to every resource.
*/}}
{{- define "obsidianstack.labels" -}}
helm.sh/chart: {{ include "obsidianstack.chart" . }}
{{ include "obsidianstack.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- with .Values.commonLabels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
Selector labels — used by Deployments and Services.
*/}}
{{- define "obsidianstack.selectorLabels" -}}
app.kubernetes.io/name: {{ include "obsidianstack.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Target namespace (from values or Release.Namespace).
*/}}
{{- define "obsidianstack.namespace" -}}
{{- .Values.namespace.name | default .Release.Namespace }}
{{- end }}

{{/*
Agent image reference (repository:tag).
*/}}
{{- define "obsidianstack.agentImage" -}}
{{- $tag := .Values.agent.image.tag | default .Chart.AppVersion }}
{{- printf "%s:%s" .Values.agent.image.repository $tag }}
{{- end }}

{{/*
Server image reference (repository:tag).
*/}}
{{- define "obsidianstack.serverImage" -}}
{{- $tag := .Values.server.image.tag | default .Chart.AppVersion }}
{{- printf "%s:%s" .Values.server.image.repository $tag }}
{{- end }}

{{/*
Return true if agent secrets map is non-empty.
*/}}
{{- define "obsidianstack.agentHasSecrets" -}}
{{- if .Values.agent.secrets }}true{{- end }}
{{- end }}

{{/*
Return true if server secrets map is non-empty.
*/}}
{{- define "obsidianstack.serverHasSecrets" -}}
{{- if .Values.server.secrets }}true{{- end }}
{{- end }}
