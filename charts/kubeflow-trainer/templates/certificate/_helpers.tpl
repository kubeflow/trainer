{{- define "trainer.webhook.cert.name" -}}
{{ .Values.webhook.cert.name }}
{{- end }}

{{- define "trainer.webhook.secret.name" -}}
{{ .Values.webhook.cert.secretName }}
{{- end }}

{{- define "trainer.webhook.service.name" -}}
{{ .Values.webhook.cert.serviceName }}
{{- end }}

{{- define "trainer.webhook.issuer.name" -}}
{{ .Values.webhook.cert.issuerRef.name }}
{{- end }}
