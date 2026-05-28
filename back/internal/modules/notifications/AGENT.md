# AGENT — módulo notifications

## Escopo

`back/internal/modules/notifications` — entrega de notificações in-app com adapter
pattern extensível (email, WhatsApp, push como stubs no MVP).

## Responsabilidade

- Persistir `user_notifications` no banco e publicar WS no canal `notifications:user:{userId}`
- Gerenciar preferências de canal por usuário/módulo/evento
- Respeitar mutes por recurso com TTL configurável
- Expor endpoints REST de leitura, marcação e preferências
- Registrar delivery_log para observabilidade

## Não é responsabilidade deste módulo

- Decidir quando notificar — os módulos consumidores (tasks, etc.) disparam
- Transporte WebSocket — delegado ao módulo realtime
- Autenticação — middleware da plataforma
- Execução de regras de negócio (ex: "quando assinar tarefa, notificar") — isso fica no service.go de tasks

## Estrutura de arquivos esperada

```
back/internal/modules/notifications/
  AGENT.md
  model.go        Notification, NotificationChannel, DeliveryLog, Mute
  errors.go       ErrNotConfigured, ErrMuted
  module.go       implementa modules.Module (optional — tasks declara OptionalModules)
  service.go      Dispatch, MarkRead, MarkAllRead, Preferences, Mute
  repository_postgres.go
  http.go         endpoints REST
  adapter_inapp.go    InAppAdapter (funcional no MVP)
  adapter_email.go    EmailAdapter  (stub: retorna ErrNotConfigured)
  adapter_whatsapp.go WhatsAppAdapter (stub)
  adapter_push.go     PushAdapter     (stub)
  ports.go        Notifier interface, ChannelAdapter interface
```

## Contrato HTTP

```
GET  /v1/notifications              paginado (cursor)
POST /v1/notifications/:id/read
POST /v1/notifications/mark-all-read
GET  /v1/notifications/preferences
PUT  /v1/notifications/preferences
POST /v1/notifications/mute         { resourceType, resourceId, durationMinutes }
```

## Interfaces

```go
type Notifier interface {
    Dispatch(ctx context.Context, input DispatchInput) error
}

type ChannelAdapter interface {
    Channel() string
    Send(ctx context.Context, n Notification) error
}
```

## Adapter pattern

O `Notifier` orquestra: verifica preferências → verifica mute → chama adapters habilitados → grava delivery_log.

MVP registra apenas `InAppAdapter`. Stubs `EmailAdapter`, `WhatsAppAdapter`, `PushAdapter` retornam `ErrNotConfigured` — mantidos para deixar a integração futura clara e evitar nil checks.

## Schema (migration 0109)

```
notifications.user_notifications      (id, account_id, user_id, source_module, source_event,
                                       title, body, link_path, payload jsonb, read_at,
                                       archived_at, created_at)
notifications.notification_channels   (user_id, account_id, channel, source_module,
                                       source_event, enabled)
notifications.delivery_log            (id, notification_id, channel, status, error, attempted_at)
notifications.mutes                   (user_id, resource_type, resource_id, until)
```

## Integração com tasks

O módulo `tasks` declara `OptionalModules: ["notifications"]`. Se ausente no registry, recebe `NoopNotifier`. Triggers implementados no `tasks/service.go`:

| Evento tasks               | Notificados |
|----------------------------|-------------|
| AssignTask                 | novo responsável |
| AddComment (com @mention)  | cada `task_mentions.user_id` |
| AddComment (sem mention)   | `task_subscribers` |
| MoveTask (mudança status)  | `task_subscribers` |

## Eventos WS publicados

Canal `notifications:user:{userId}`:
- `notification.created` — payload completo (é tão pequeno que REST seria desperdício)
- `notification.read`

## Padrão Go

Mesmo padrão de `back/internal/modules/operations/`: UUID como string, scan nullable com `*string`, permissões catalogadas no banco.

## Evolução esperada

1. MVP: InAppAdapter funcional; stubs para os outros
2. v1.4: EmailAdapter via SMTP (config por account)
3. v1.5: WhatsAppAdapter via provider (Twilio/Z-API)
4. v2.0: PushAdapter via Firebase/APNs
