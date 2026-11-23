# Pacote `mailer` — envio SMTP genérico

`Mailer.Send(ctx, to, subject, textBody)` envia texto simples UTF-8 por SMTP
(STARTTLS por padrão, TLS implícito ou `none`), com timeout, `InsecureSkipVerify`
opcional e assunto codificado (RFC 2047, sem quebra de linha). A configuração usa os
mesmos campos `SMTP_*` de `platform/config`. `Configured()` exige host e remetente
válidos; sem isso `Send` devolve `ErrNotConfigured`.

Usado hoje pelos alertas do módulo `observability`.

## Pendência

`modules/auth/password_reset_delivery.go` ainda tem sua própria cópia do transporte
SMTP (com mensagens de erro prefixadas `auth:`). Não foi migrado para evitar mexer no
fluxo de recuperação de senha nesta entrega; ao migrar, delegar `send` a este pacote.
