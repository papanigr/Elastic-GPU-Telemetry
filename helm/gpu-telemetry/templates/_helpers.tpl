{{/*
Expand the name of the chart.
*/}}
{{- define "gpu-telemetry.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "gpu-telemetry.fullname" -}}
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
{{- define "gpu-telemetry.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "gpu-telemetry.labels" -}}
helm.sh/chart: {{ include "gpu-telemetry.chart" . }}
{{ include "gpu-telemetry.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "gpu-telemetry.selectorLabels" -}}
app.kubernetes.io/name: {{ include "gpu-telemetry.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the image path
*/}}
{{- define "gpu-telemetry.image" -}}
{{- $registry := .Values.global.image.registry -}}
{{- $name := .image -}}
{{- $tag := .Values.global.image.tag -}}
{{- printf "%s/%s:%s" $registry $name $tag -}}
{{- end }}

{{/*
PostgreSQL connection string
*/}}
{{- define "gpu-telemetry.postgresUrl" -}}
{{- $user := .Values.postgres.credentials.username -}}
{{- $pass := .Values.postgres.credentials.password -}}
{{- $db := .Values.postgres.credentials.database -}}
{{- printf "postgres://%s:%s@postgres:%d/%s?sslmode=disable" $user $pass (int .Values.postgres.port) $db -}}
{{- end }}

{{/*
MQ broker address for gRPC
*/}}
{{- define "gpu-telemetry.mqGrpcAddr" -}}
{{- printf "mq:%d" (int .Values.mq.ports.grpc) -}}
{{- end }}
