# AGENT — module `queue`

## Escopo

Módulo `back/internal/modules/queue/`. Domínio central de fila de atendimento:
operações, alertas, analytics, relatórios, feedback, consultores e configurações.

Branch alvo: `refactor/multi-tenant-core`. Plano canônico:
`docs/MULTITENANT_COMPLETION_PLAN.md`.

## Estrutura de arquivos

```
queue/
  module.go                  — implementa modules.Module (ID, Metadata, Permissions, RoleTemplates, Build)
  operations/                — subpacote queue/operations
    model.go
    store_postgres.go
    service.go
    http.go
    store_scope_adapter.go   — adapta escopo de loja (operations_store_scope_adapter.go original)
    AGENT.md
    CONCURRENT_SERVICES.md
  alerts/                    — subpacote queue/alerts
    model.go
    store_postgres.go
    service.go
    http.go
    AGENT.md
  analytics/                 — subpacote queue/analytics
    model.go
    store_postgres.go
    service.go
    http.go
    AGENT.md
  reports/                   — subpacote queue/reports
    model.go
    store_postgres.go
    service.go
    http.go
    AGENT.md
  feedback/                  — subpacote queue/feedback
    model.go
    store_postgres.go
    service.go
    http.go
    AGENT.md
  consultants/               — subpacote queue/consultants
    model.go
    store_postgres.go
    service.go
    http.go
    AGENT.md
  settings/                  — subpacote queue/settings
    model.go
    store_postgres.go
    service.go
    http.go
    AGENT.md
  communications/            — comunicados por conta/loja
    model.go
    store_postgres.go
    service.go
    http.go
    AGENT.md
```

## Estado atual (C4 — 2026-05-29)

### C4 concluído

- 75 arquivos Go movidos fisicamente de `back/internal/modules/<nome>/` para
  `back/internal/modules/queue/<nome>/`.
- Imports atualizados em 19 arquivos (paths `modules/<nome>` → `modules/queue/<nome>`).
- `queue/module.go` criado implementando `modules.Module`:
  - `ID() = "queue"`
- 9 permissões declaradas (ver seção abaixo)
  - 2 role templates: `queue.supervisor` e `queue.consultant`
  - `Build()` retorna handle vazio — rotas ainda montadas pelo wiring legado em `app.go`
- `registry.MustRegister(queue.New())` adicionado em `app.go` (antes de `notifications.New()`).
- `go build ./...` confirmado limpo após a reorganização.

### Wiring de rotas (legado) + gating ativo (C20, 2026-06-04)

Os endpoints HTTP das rotas `/v1/operations/*`, `/v1/alerts/*`, `/v1/reports/*`,
`/v1/analytics/*`, `/v1/feedback/*`, `/v1/consultants/*`, `/v1/settings/*`,
`/v1/stores/*` continuam registrados diretamente em `app.go` com `RequireAuth`,
MAS agora são **gateados por contratação** via `AccountModulesGuard.RequireModuleByPath`
(prefixo → `queue`), aplicado no `Chain`. Desabilitar o módulo `queue` de uma
account → `403 module_disabled` nessas rotas. O front envia `X-Account-Id`
automaticamente (`createApiRequest`). Ver [platform/app/AGENT.md](../../platform/app/AGENT.md)
e [platform/httpapi/AGENT.md](../../platform/httpapi/AGENT.md).

## Permissões declaradas

| Key | Scope | Descrição |
|---|---|---|
| `queue.dashboard.read` | store | Ver painel de operações em tempo real |
| `queue.operations.manage` | store | Abrir, pausar, finalizar atendimentos |
| `queue.alerts.manage` | account | Criar/editar regras de alerta |
| `queue.analytics.read` | account | Métricas e rankings de desempenho |
| `queue.reports.read` | account | Relatórios e exportações |
| `queue.feedback.read` | account | Visualizar feedbacks de atendimentos |
| `queue.settings.manage` | account | Configurações de operação e templates |
| `queue.consultants.manage` | account | Criar/editar/desativar consultores |
| `queue.communications.manage` | account | Criar/editar/publicar/excluir comunicados |

## Role templates declarados

| ID | Label | Permissões |
|---|---|---|
| `queue.supervisor` | Supervisor de Fila | Todas as 9 permissões |
| `queue.consultant` | Consultor | dashboard.read + operations.manage + analytics.read |

## Import paths

Antes da reorganização (C4):
```go
"github.com/mikewade2k16/lista-da-vez/back/internal/modules/operations"
"github.com/mikewade2k16/lista-da-vez/back/internal/modules/alerts"
// ...
```

Após (correto):
```go
"github.com/mikewade2k16/lista-da-vez/back/internal/modules/queue/operations"
"github.com/mikewade2k16/lista-da-vez/back/internal/modules/queue/alerts"
// ...
```

## Notas de deploy

Nenhuma migration nova em C4. A reorganização é apenas de pacotes Go — não
afeta banco, variáveis de ambiente ou Dockerfile.

## Próximos passos (C5)

C5 faz o mesmo para CRM:
- Mover `back/internal/modules/erp` → `back/internal/modules/crm/erp`
- Mover `back/internal/modules/catalog` → `back/internal/modules/crm/catalog`
- Criar `crm/module.go` com ID="crm", permissões e role templates
- Criar `queue/catalog_adapter.go` com fallback local quando crm não habilitado
- Registrar `crm.New()` em `app.go`

## Regras inegociáveis

- `account_id` SO vem do middleware (Principal.AccountID) — nunca do body/query.
- Queries SQL internas devem usar fontes canonicas: `queue.stores`, `queue.consultants` e `core.users`; nao usar aliases publicos `stores`, `consultants` ou `users`.
- Não introduzir FK de `queue.*` para `core.*`; abstrair via interface in-process.
- `core.account_modules` é a fonte de verdade para "módulo queue habilitado?".
- Máx 450 linhas por arquivo; quebrar em sub-arquivos se necessário.

## Gravacoes e transcricoes experimentais

- O pacote `transcriptions/` recebe blocos idempotentes, consolida o audio em
  storage privado e lista os metadados autoritativos por conta/loja.
- As tabelas `queue.attendance_recordings` e
  `queue.attendance_recording_chunks` repetem `account_id`; nao possuem FK para
  `core.*`.
- O endpoint de audio e autenticado. Ao consolidar uma gravacao, o backend
  solicita automaticamente o job duravel do Whisper local; o endpoint
  `POST .../{id}/transcribe` permanece idempotente para retry manual.

## Comunicados da operacao

- O pacote `communications/` fornece CRUD em
  `/v1/operations/communications`.
- `queue.communications` guarda conteudo, vigencia, publicacao, ordem e soft
  delete; `queue.communication_stores` guarda destinos especificos.
- A FK composta `(account_id, store_id)` impede vinculo de loja de outra conta.
- O painel operacional consulta apenas comunicados publicados e vigentes para
  a loja corrente; a aba `/comunicados` administra todos os registros da conta.

## Navegacao enxuta da Fila (2026-07-25)

- As abas `Dados`, `Inteligencia`, `Relatorios` e `Clientes` foram removidas da
  navegacao do modulo Fila.
- As antigas paginas `/clientes`, `/operacao/clientes` e `/manage/clientes`
  foram retiradas por duplicarem a administracao de contas; o alias de Manage
  aponta para `/manage/clientes-web`. Elas liam `core.accounts` por
  `/v1/tenants`; nunca houve tabela de clientes propria do schema `queue`.
- `core.accounts` permanece como fonte canonica e nao pode ser removida: o
  catalogo e compartilhado por toda a plataforma e possui dependencias
  multi-tenant fora da Fila.
- Nesta retirada, `Dados`, `Inteligencia` e `Relatorios` perderam apenas os
  atalhos da Fila; seus endpoints e superficies legadas foram preservados para
  rollback e para nao interromper consumidores existentes.

## Quando atualizar este AGENT.md

- Quando rotas migrarem do wiring legado (app.go) para `RequireModule("queue")`.
- Quando C5 adicionar crm.Resolver e o catalog_adapter for criado.
- Quando permissões ou role templates forem alterados.
