---
paths:
  - "pkg/webhooks/**"
---

# Webhook conventions

- Defaulters implement `admission.CustomDefaulter` - only set defaults, never validate
- Validators implement `admission.CustomValidator` - delegate to runtime plugins via `Runtime.ValidateObjects()`
- Do not put validation logic in webhook files - implement `CustomValidationPlugin` in the relevant plugin
- Register in `setup.go` via private `setupWebhookFor*()` functions
- Use `field.NewPath()` and `field.ErrorList` for structured errors
- Use `clock.PassiveClock` for time-dependent logic to keep webhooks testable
