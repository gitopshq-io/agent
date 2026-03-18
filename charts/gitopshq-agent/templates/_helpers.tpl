{{- define "gitopshq-agent.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "gitopshq-agent.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- include "gitopshq-agent.name" . -}}
{{- end -}}
{{- end -}}

{{- define "gitopshq-agent.serviceAccountName" -}}
{{- if .Values.serviceAccount.name -}}
{{- .Values.serviceAccount.name -}}
{{- else -}}
{{- include "gitopshq-agent.fullname" . -}}
{{- end -}}
{{- end -}}

{{- define "gitopshq-agent.identitySecretName" -}}
{{- if .Values.persistence.secretName -}}
{{- .Values.persistence.secretName -}}
{{- else -}}
{{- printf "%s-identity" (include "gitopshq-agent.fullname" .) -}}
{{- end -}}
{{- end -}}

{{- define "gitopshq-agent.imageTag" -}}
{{- default .Chart.AppVersion .Values.image.tag -}}
{{- end -}}

{{- define "gitopshq-agent.capabilities" -}}
{{- $caps := list -}}
{{- if .Values.capabilities.observe -}}
{{- $caps = append $caps "observe" -}}
{{- end -}}
{{- if .Values.capabilities.diagnosticsRead -}}
{{- $caps = append $caps "diagnostics.read" -}}
{{- end -}}
{{- if .Values.capabilities.argocdRead -}}
{{- $caps = append $caps "argocd.read" -}}
{{- end -}}
{{- if .Values.capabilities.argocdWrite -}}
{{- $caps = append $caps "argocd.write" -}}
{{- end -}}
{{- if .Values.capabilities.directDeploy -}}
{{- $caps = append $caps "deploy.direct" -}}
{{- end -}}
{{- if .Values.capabilities.kubernetesRestart -}}
{{- $caps = append $caps "k8s.restart" -}}
{{- end -}}
{{- if .Values.capabilities.kubernetesScale -}}
{{- $caps = append $caps "k8s.scale" -}}
{{- end -}}
{{- if .Values.capabilities.credentialSync -}}
{{- $caps = append $caps "credentials.sync" -}}
{{- end -}}
{{- if .Values.capabilities.tokenRotate -}}
{{- $caps = append $caps "token.rotate" -}}
{{- end -}}
{{- join "," $caps -}}
{{- end -}}
