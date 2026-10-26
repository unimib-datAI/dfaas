{{/* Common labels */}}
{{- define "openwhisk.labels" -}}
heritage: {{ .Release.Service | quote }}
release: {{ .Release.Name | quote }}
chart: "{{ .Chart.Name }}-{{ .Chart.Version }}"
app: {{ printf "%s-%s" .Release.Name .Chart.Name | trunc 63 | trimSuffix "-" }}
{{- end -}}

{{/* FQDN of a chart Service, e.g. include "openwhisk.svc" (list . "controller") */}}
{{- define "openwhisk.svc" -}}
{{- $root := index . 0 -}}
{{ $root.Release.Name }}-{{ index . 1 }}.{{ $root.Release.Namespace }}.svc.{{ $root.Values.k8s.domain }}
{{- end -}}

{{/* openwhisk/* image tagged with the short git commit, e.g. include "openwhisk.image" (list . "controller") */}}
{{- define "openwhisk.image" -}}
{{- $root := index . 0 -}}
openwhisk/{{ index . 1 }}:{{ $root.Values.whisk.gitTag | trunc 7 }}
{{- end -}}

{{/* PersistentVolumeClaim, e.g. include "openwhisk.pvc" (list . "couchdb-pvc" "2Gi") */}}
{{- define "openwhisk.pvc" -}}
{{- $root := index . 0 -}}
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: {{ $root.Release.Name }}-{{ index . 1 }}
  labels:
{{ include "openwhisk.labels" $root | indent 4 }}
spec:
{{- with $root.Values.k8s.storageClass }}
  storageClassName: {{ . }}
{{- end }}
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: {{ index . 2 }}
{{- end -}}

{{/* Env var reading a key of the CouchDB credentials Secret, e.g. include "openwhisk.dbSecretEnv" (list . "COUCHDB_USER" "db_username") */}}
{{- define "openwhisk.dbSecretEnv" -}}
- name: {{ index . 1 | quote }}
  valueFrom:
    secretKeyRef:
      name: {{ (index . 0).Release.Name }}-db.auth
      key: {{ index . 2 }}
{{- end -}}

{{/* Init container that waits for the controller to be ready */}}
{{- define "openwhisk.waitForController" -}}
- name: wait-for-controller
  image: busybox:latest
  imagePullPolicy: IfNotPresent
  command: ["sh", "-c", "until wget -T 5 --spider http://{{ include "openwhisk.svc" (list . "controller") }}:8080/ping; do echo 'waiting for controller'; sleep 1; done"]
{{- end -}}

{{/* Environment shared by controller and invoker */}}
{{- define "openwhisk.coreEnv" -}}
- name: "CONFIG_whisk_info_date"
  value: "2022-10-14-13:44:50Z"
- name: "RUNTIMES_MANIFEST"
  value: {{ .Files.Get "runtimes.json" | quote }}
- name: "OPENWHISK_ENCODED_CONFIG"
  value: {{ .Files.Get "whiskconfig.conf" | b64enc }}
- name: "CONFIG_whisk_userEvents_enabled"
  value: "true"
- name: "CONFIG_logback_log_level"
  value: "INFO"

# Kafka
- name: "KAFKA_HOSTS"
  value: "{{ .Release.Name }}-kafka-0.{{ include "openwhisk.svc" (list . "kafka") }}:9092"

# CouchDB
{{ include "openwhisk.dbSecretEnv" (list . "CONFIG_whisk_couchdb_username" "db_username") }}
{{ include "openwhisk.dbSecretEnv" (list . "CONFIG_whisk_couchdb_password" "db_password") }}
- name: "CONFIG_whisk_couchdb_protocol"
  value: "http"
- name: "CONFIG_whisk_couchdb_host"
  value: {{ include "openwhisk.svc" (list . "couchdb") | quote }}
- name: "CONFIG_whisk_couchdb_port"
  value: "5984"
- name: "CONFIG_whisk_couchdb_provider"
  value: "CouchDB"
- name: "CONFIG_whisk_couchdb_databases_WhiskActivation"
  value: "{{ .Values.db.prefix }}activations"
- name: "CONFIG_whisk_couchdb_databases_WhiskEntity"
  value: "{{ .Values.db.prefix }}whisks"
- name: "CONFIG_whisk_couchdb_databases_WhiskAuth"
  value: "{{ .Values.db.prefix }}subjects"

# Action limits
- name: "LIMITS_ACTIONS_INVOKES_PERMINUTE"
  value: "60"
- name: "LIMITS_ACTIONS_INVOKES_CONCURRENT"
  value: "30"
- name: "LIMITS_TRIGGERS_FIRES_PERMINUTE"
  value: "60"
- name: "LIMITS_ACTIONS_SEQUENCE_MAXLENGTH"
  value: "50"
- name: "CONFIG_whisk_timeLimit_min"
  value: "100ms"
- name: "CONFIG_whisk_timeLimit_max"
  value: "5m"
- name: "CONFIG_whisk_timeLimit_std"
  value: "1m"
- name: "CONFIG_whisk_memory_min"
  value: "128m"
- name: "CONFIG_whisk_memory_max"
  value: "512m"
- name: "CONFIG_whisk_memory_std"
  value: "256m"
- name: "CONFIG_whisk_concurrencyLimit_min"
  value: "1"
- name: "CONFIG_whisk_concurrencyLimit_max"
  value: "1"
- name: "CONFIG_whisk_concurrencyLimit_std"
  value: "1"
- name: "CONFIG_whisk_logLimit_min"
  value: "0m"
- name: "CONFIG_whisk_logLimit_max"
  value: "10m"
- name: "CONFIG_whisk_logLimit_std"
  value: "10m"
- name: "CONFIG_whisk_activation_payload_max"
  value: "1048576"
{{- end -}}
