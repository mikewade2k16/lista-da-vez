# Plano: Fase 5.2.1 — Threads de Feedback + Notificações em Tempo Real

## Contexto

A Fase 5.2 já entregou o canal de feedback (criar / listar / atualizar status + nota interna). O usuário pediu para evoluir para:

1. **Conversa multi-mensagem** entre usuário e admin (substituindo o `admin_note` por uma thread)
2. **Notificações em tempo real via WebSocket** quando há novo feedback ou nova mensagem
3. **Sino no header** com dropdown de resumo, destaque para não lidos
4. **Admin** vê todos os feedbacks do tenant (com nome do remetente, loja, status); **usuário comum** vê apenas os próprios via sino
5. **Edição inline de status** na grade (não precisa abrir o modal só para mudar status)
6. **Filtros adicionais** na página `/feedback` (tipo, status, **loja**)
7. **Redesign** da página `/feedback` removendo o visual claro/quadrado da grade atual

Vamos reaproveitar a infra de realtime existente (`back/internal/modules/realtime`, gorilla/websocket — projeto **já usa WebSocket**, não SSE).

---

## Visão geral do fluxo

### Usuário comum
- Clica no botão flutuante 💬 → modal "Enviar Feedback" (sem mudança visual)
- Cria feedback → recebe toast de sucesso
- Quando admin responde:
  - Toast push aparece
  - Sino no header ganha badge com nº de não lidos
  - Abre o sino → vê resumo da thread → clica → modal com a conversa completa + caixa de resposta

### Admin (owner/manager/platform_admin)
- Quando qualquer usuário do tenant cria feedback ou responde:
  - Toast push
  - Sino ganha badge
- Sino mostra resumo (não lidos primeiro)
- Página `/feedback` redesenhada: filtros (tipo, status, loja), grade com inline status edit, modal com thread + caixa de resposta

---

## Parte 1 — Backend

### 1.1 Migration `0030_feedback_messages.sql` (nova)

```sql
-- Thread de mensagens
create table if not exists user_feedback_messages (
    id uuid primary key default gen_random_uuid(),
    feedback_id uuid not null references user_feedback(id) on delete cascade,
    author_id uuid not null references users(id),
    author_name text not null default '',
    author_role text not null default '',  -- 'user' ou 'admin' (drives styling)
    body text not null,
    created_at timestamptz not null default now()
);
create index if not exists user_feedback_messages_feedback_id_idx
    on user_feedback_messages (feedback_id, created_at);

-- Flags de "não lido" por lado
alter table user_feedback
    add column if not exists unread_for_admin boolean not null default true,
    add column if not exists unread_for_user boolean not null default false;

-- Backfill: qualquer feedback existente com admin_note não vazio vira primeira mensagem do admin
insert into user_feedback_messages (feedback_id, author_id, author_name, author_role, body, created_at)
select id, user_id, 'Admin', 'admin', admin_note, updated_at
from user_feedback
where coalesce(trim(admin_note), '') <> '';

-- Índices de unread para query do sino
create index if not exists user_feedback_unread_admin_idx
    on user_feedback (tenant_id, unread_for_admin) where unread_for_admin = true;
create index if not exists user_feedback_unread_user_idx
    on user_feedback (user_id, unread_for_user) where unread_for_user = true;
```

> **Decisão**: `admin_note` permanece na tabela mas vira deprecated; código novo não escreve nele. Evita refatoração ampla.

### 1.2 `back/internal/modules/feedback/model.go` — alteração

- Acrescentar campos em `Feedback`:
  ```go
  UnreadForAdmin bool
  UnreadForUser  bool
  ```
- Acrescentar struct `Message`:
  ```go
  type Message struct {
      ID         string
      FeedbackID string
      AuthorID   string
      AuthorName string
      AuthorRole string  // "user" | "admin"
      Body       string
      CreatedAt  time.Time
  }

  type MessageView struct {
      ID         string    `json:"id"`
      FeedbackID string    `json:"feedback_id"`
      AuthorID   string    `json:"author_id"`
      AuthorName string    `json:"author_name"`
      AuthorRole string    `json:"author_role"`
      Body       string    `json:"body"`
      CreatedAt  time.Time `json:"created_at"`
  }
  ```
- Acrescentar em `FeedbackView`:
  ```go
  StoreName       string        `json:"store_name"`
  Messages        []MessageView `json:"messages"`
  UnreadForAdmin  bool          `json:"unread_for_admin"`
  UnreadForUser   bool          `json:"unread_for_user"`
  ```
- Atualizar `ListInput`:
  ```go
  type ListInput struct {
      Kind    string
      Status  string
      StoreID string
      OnlyOwn bool   // service seta true quando principal não é admin
      UserID  string // service seta para principal.UserID quando OnlyOwn = true
  }
  ```
- Acrescentar `ReplyInput`:
  ```go
  type ReplyInput struct {
      Body string `json:"body"`
  }
  ```
- Acrescentar `PublishedEvent` (para realtime, evita ciclo de import):
  ```go
  type PublishedEvent struct {
      Type            string
      TenantID        string
      RecipientUserID string
      FeedbackID      string
      Subject         string
      Kind            string
      Status          string
      Preview         string
      AuthorName      string
      AuthorRole      string
      SavedAt         time.Time
  }
  ```
- Estender interface `Repository`:
  ```go
  type Repository interface {
      Create(feedback *Feedback) (*Feedback, error)
      GetByID(id string) (*Feedback, error)
      List(input ListInput) ([]Feedback, error)
      Update(feedback *Feedback) error

      AddMessage(message *Message) (*Message, error)
      ListMessages(feedbackID string) ([]Message, error)
      MarkRead(feedbackID string, side string) error  // side: "admin" | "user"
      CountUnread(side string, scopeID string) (int, error)
  }
  ```

### 1.3 `back/internal/modules/feedback/store_postgres.go` — alteração

Pontos críticos para **evitar erros antigos**:

- **Sempre** usar `*string` para colunas que podem ser NULL (`tenant_id`, `store_id`, `s.name` no LEFT JOIN). Helper `scanFeedback` já existe — estender para incluir os novos campos.
- **Sempre** usar `fmt.Sprintf("$%d", argCount)` em filtros condicionais (já correto).
- **Sempre** usar `nullableUUID()` para evitar cast `''::uuid`.
- **Sempre** usar `WHERE 1=1` + filtros condicionais (já correto).

Mudanças:

- `List` passa a receber `ListInput` completo (ao invés de tenantID separado):
  ```sql
  select f.id::text, f.tenant_id::text, f.store_id::text, f.user_id::text, f.user_name,
         f.kind, f.status, f.subject, f.body, f.admin_note,
         f.unread_for_admin, f.unread_for_user,
         f.created_at, f.updated_at,
         s.name
  from user_feedback f
  left join stores s on s.id = f.store_id
  where 1=1
  ```
  Adiciona condicionalmente `and f.tenant_id = $N::uuid`, `and f.user_id = $N::uuid`, `and f.kind = $N`, `and f.status = $N`, `and f.store_id = $N::uuid`.
- `scanFeedback` reescrito para receber também `*storeName` e dois bools de unread.
- `Create` retorna também os novos flags via RETURNING.
- Implementar `AddMessage`:
  ```sql
  insert into user_feedback_messages (feedback_id, author_id, author_name, author_role, body)
  values ($1::uuid, $2::uuid, $3, $4, $5)
  returning id::text, created_at;
  ```
  Após inserir, atualizar flags na `user_feedback`:
  - se `author_role = 'admin'` → `set unread_for_user = true, unread_for_admin = false, updated_at = now()`
  - se `author_role = 'user'` → `set unread_for_admin = true, unread_for_user = false, updated_at = now()`
  - Operações em transação (`pgx.Tx`).
- Implementar `ListMessages`:
  ```sql
  select id::text, feedback_id::text, author_id::text, author_name, author_role, body, created_at
  from user_feedback_messages
  where feedback_id = $1::uuid
  order by created_at asc;
  ```
- Implementar `MarkRead(feedbackID, side)`:
  ```sql
  -- side = "admin": update user_feedback set unread_for_admin = false where id = $1::uuid;
  -- side = "user":  update user_feedback set unread_for_user  = false where id = $1::uuid;
  ```
- Implementar `CountUnread`:
  ```sql
  -- admin: where tenant_id = $1::uuid and unread_for_admin = true
  -- user:  where user_id = $1::uuid and unread_for_user = true
  select count(*) from user_feedback where ...
  ```

### 1.4 `back/internal/modules/feedback/service.go` — alteração

- Adicionar interface `RealtimePublisher`:
  ```go
  type RealtimePublisher interface {
      PublishFeedbackEvent(ctx context.Context, event PublishedEvent)
  }
  ```
- `NewService(repository, publisher)` — publisher pode ser `nil`.
- `Create` — após criar, publica `feedback.created` com `RecipientUserID = principal.UserID` (echo) + `TenantID = principal.TenantID`.
- `List(ctx, principal, input)`:
  - Se `canManageFeedback(principal)` → `input.TenantID = principal.TenantID`, sem `OnlyOwn`.
  - Senão → `input.OnlyOwn = true`, `input.UserID = principal.UserID`, `input.TenantID = principal.TenantID`.
  - Para a lista, **não** carrega messages (payload menor); messages só em `GetByID`.
- Adicionar `GetByID(ctx, principal, id)`:
  - Carrega feedback + messages
  - Verifica acesso: admin do mesmo tenant **ou** `feedback.UserID == principal.UserID`
- Adicionar `Reply(ctx, principal, id, input)`:
  - Carrega feedback, valida acesso
  - `authorRole`: "admin" se `canManageFeedback(principal)`, senão "user"
  - Insere mensagem (repo dispara update das flags)
  - Publica `feedback.replied`
  - Retorna feedback completo (com messages atualizadas)
- Manter `Update(ctx, principal, id, input)` apenas para mudança de status (admin only). Publica `feedback.updated`.
- Adicionar `MarkRead(ctx, principal, id)`:
  - Carrega feedback, valida acesso
  - Se principal é admin no tenant → `MarkRead(id, "admin")`
  - Senão se é o dono → `MarkRead(id, "user")`
  - **Não publica evento de WS** (só ajusta o próprio badge — outros lados não precisam saber).
- Adicionar `Summary(ctx, principal)`:
  - Retorna `{ unread_count, recent: [FeedbackView (sem messages)] }` (top 10 mais recentes do escopo do principal)

### 1.5 `back/internal/modules/feedback/http.go` — alteração

Rotas:
- `POST /v1/feedback` — qualquer usuário autenticado (cria)
- `GET /v1/feedback?kind=&status=&store_id=` — qualquer usuário autenticado (service decide escopo)
- `GET /v1/feedback/{id}` — qualquer usuário autenticado com acesso
- `PATCH /v1/feedback/{id}` — admin (mudança de status)
- `POST /v1/feedback/{id}/messages` — qualquer usuário com acesso (reply)
- `POST /v1/feedback/{id}/read` — qualquer usuário com acesso (marcar como lido)
- `GET /v1/feedback/summary` — qualquer usuário autenticado (resumo do sino)

`writeServiceError` cobre `ErrForbidden` (já existe).

### 1.6 `back/internal/modules/realtime/model.go` — alteração

```go
const (
    EventTypeConnected         = "realtime.connected"
    EventTypeOperationUpdated  = "operation.updated"
    EventTypeContextUpdated    = "context.updated"
    EventTypeFeedbackCreated   = "feedback.created"   // novo
    EventTypeFeedbackReplied   = "feedback.replied"   // novo
    EventTypeFeedbackUpdated   = "feedback.updated"   // novo (mudança de status)
)
```

Acrescentar campos no `Event`:
```go
UserID     string `json:"userId,omitempty"`
FeedbackID string `json:"feedbackId,omitempty"`
Subject    string `json:"subject,omitempty"`
Kind       string `json:"kind,omitempty"`
Status     string `json:"status,omitempty"`
Preview    string `json:"preview,omitempty"`
AuthorName string `json:"authorName,omitempty"`
AuthorRole string `json:"authorRole,omitempty"`
```

Helpers:
```go
func feedbackTenantTopic(tenantID string) string { return "feedback:tenant:" + tenantID }
func feedbackUserTopic(userID string) string     { return "feedback:user:" + userID }
```

### 1.7 `back/internal/modules/realtime/service.go` — alteração

Adicionar:
```go
func (service *Service) PublishFeedbackEvent(ctx context.Context, event feedback.PublishedEvent) {
    rtEvent := Event{
        Type:       event.Type,
        TenantID:   strings.TrimSpace(event.TenantID),
        UserID:     strings.TrimSpace(event.RecipientUserID),
        FeedbackID: strings.TrimSpace(event.FeedbackID),
        Subject:    event.Subject,
        Kind:       event.Kind,
        Status:     event.Status,
        Preview:    event.Preview,
        AuthorName: event.AuthorName,
        AuthorRole: event.AuthorRole,
        SavedAt:    event.SavedAt.UTC(),
    }

    if rtEvent.TenantID != "" {
        service.hub.Publish(feedbackTenantTopic(rtEvent.TenantID), rtEvent)
    }
    if rtEvent.UserID != "" {
        service.hub.Publish(feedbackUserTopic(rtEvent.UserID), rtEvent)
    }
}
```

> **Anti-ciclo de import**: `realtime` importa `feedback` (mesmo padrão que faz com `operations`). `feedback.Service` usa apenas a interface `RealtimePublisher` declarada no próprio pacote `feedback` — o injeção é feita em `app.go`.

Adicionar handler WebSocket para feedback:

```go
func (service *Service) HandleFeedbackSocket(w http.ResponseWriter, r *http.Request) {
    // mesma autenticação dos outros sockets (token via query/header)
    // Subscreve em:
    //   - feedbackUserTopic(principal.UserID)  -- sempre
    //   - feedbackTenantTopic(principal.TenantID) -- só se canManageFeedback E tenantID != ""
    // Loop de pump idêntico aos outros sockets
}
```

`canManageFeedback` aqui é uma checagem local de roles (`principal.Role == auth.RoleOwner || principal.Role == auth.RoleManager || principal.Role == auth.RolePlatformAdmin`) — sem importar `feedback` para evitar acoplamento.

### 1.8 `back/internal/modules/realtime/http.go` — alteração

```go
mux.HandleFunc("GET /v1/realtime/feedback", service.HandleFeedbackSocket)
```

### 1.9 `back/internal/platform/app/app.go` — alteração

```go
feedbackRepository := feedback.NewPostgresRepository(pool)
feedbackService := feedback.NewService(feedbackRepository, realtimeService) // <-- passa realtime
```

### 1.10 `back/internal/modules/feedback/AGENT.md` — alteração

Atualizar para descrever:
- Tabela `user_feedback_messages` + flags `unread_for_admin/user`
- Endpoints de threads e read
- Eventos publicados (`feedback.created`, `feedback.replied`, `feedback.updated`)
- Topics WS: `feedback:tenant:{id}`, `feedback:user:{id}`

### 1.11 `back/internal/modules/realtime/AGENT.md` — alteração

Adicionar seção mencionando os novos topics e endpoint `/v1/realtime/feedback`.

### 1.12 `back/database/ERD.md` — alteração

Adicionar tabela `USER_FEEDBACK_MESSAGES` ao Mermaid + relação `USER_FEEDBACK ||--o{ USER_FEEDBACK_MESSAGES : threads`. Adicionar colunas `unread_for_admin` / `unread_for_user` em `USER_FEEDBACK`.

---

## Parte 2 — Frontend

### 2.1 `web/app/composables/useFeedbackRealtime.ts` (novo)

Espelha `useContextRealtime.ts`:
- Conecta em `/v1/realtime/feedback?access_token=...`
- Watcher em `auth.isAuthenticated && auth.accessToken`
- Reconnect com backoff exponencial (mesmo padrão)
- `onmessage`:
  - `feedback.created`: toast "Novo feedback de {authorName}: {subject}"; refresh summary
  - `feedback.replied`:
    - se admin e author é user → toast "Nova resposta em {subject}"
    - se user comum (recipient é ele e author é admin) → toast "Você tem uma resposta em {subject}"
    - refresh summary
  - `feedback.updated`: refresh summary; se há thread modal aberto com aquele id, recarrega
- Despacha eventos para o store via `feedbackStore.applyRealtimeEvent(payload)`

### 2.2 `web/app/stores/feedback.ts` — reescrita

Tipos:
```ts
interface FeedbackMessage {
  id: string;
  feedback_id: string;
  author_id: string;
  author_name: string;
  author_role: "user" | "admin";
  body: string;
  created_at: string;
}

interface FeedbackItem {
  id: string;
  tenant_id: string;
  store_id: string;
  store_name: string;
  user_id: string;
  user_name: string;
  kind: string;
  status: string;
  subject: string;
  body: string;
  admin_note: string;             // legacy
  messages: FeedbackMessage[];
  unread_for_admin: boolean;
  unread_for_user: boolean;
  created_at: string;
  updated_at: string;
}

interface FeedbackSummary {
  unread_count: number;
  recent: FeedbackItem[];
}
```

State:
- `items: FeedbackItem[]` — lista da página `/feedback`
- `summary: FeedbackSummary` — para o sino
- `loading`, `error`

Actions:
- `submitFeedback(input)` — POST `/v1/feedback`
- `fetchFeedbacks(filters?)` — GET `/v1/feedback` (filtros: kind, status, store_id)
- `getFeedback(id)` — GET `/v1/feedback/{id}` (com messages)
- `replyToFeedback(id, body)` — POST `/v1/feedback/{id}/messages`
- `markFeedbackRead(id)` — POST `/v1/feedback/{id}/read`
- `updateFeedback(id, { status })` — PATCH `/v1/feedback/{id}` (apenas status — para inline edit)
- `fetchSummary()` — GET `/v1/feedback/summary`
- `applyRealtimeEvent(payload)` — chamado pelo composable
  - Se há item em `items` com `id === payload.feedbackId` → atualiza inline
  - Sempre chama `fetchSummary()` (cheap)

### 2.3 `web/app/components/dashboard/DashboardHeaderNotifications.vue` (novo)

UI:
- Botão sino (`lucide-vue-next` `Bell`) com badge superior-direita mostrando número de não lidos (oculto se 0)
- Click → toggle dropdown
- Dropdown:
  - Header: "Notificações" + ação "Ver todas" (apenas admin → vai para `/feedback`)
  - Lista das `summary.recent` (top 10):
    - Linha por feedback: ícone do kind, subject (1 linha), nome do remetente + loja (admin) ou "Você" (usuário), última msg preview, "há X min", badge de status, indicador unread (bolinha azul)
  - Estado vazio: "Nenhuma mensagem ainda"
- Click em linha:
  - Admin → `navigateTo('/feedback?focus=' + id)`
  - Usuário comum → emite `open-feedback` para o layout, que abre `FeedbackThreadModal` com aquele id
- Estilo: idêntico ao `dashboard-header__profile-dropdown` (dark gradient, blur, transitions)

Lifecycle:
- `onMounted` → `feedbackStore.fetchSummary()`
- Pointerdown global fecha dropdown
- ESC fecha dropdown
- Watch route → fecha dropdown

### 2.4 `web/app/components/dashboard/DashboardHeader.vue` — alteração

Importar e usar `<DashboardHeaderNotifications />` antes do `dashboard-header__profile-menu`. Re-emitir `open-feedback` para o layout.

### 2.5 `web/app/layouts/dashboard.vue` — alteração

- Mountar `useFeedbackRealtime()` ao lado de `useContextRealtime()`
- Adicionar `ref` `threadModalOpen` + `threadModalId`
- Capturar `@open-feedback` do header e abrir `<FeedbackThreadModal>`
- Renderizar `<FeedbackThreadModal v-model="threadModalOpen" :feedback-id="threadModalId" />`

### 2.6 `web/app/components/feedback/FeedbackThread.vue` (novo, reusável)

Componente puro de UI:
- Props: `messages: FeedbackMessage[]`, `currentUserId: string`, `loading: boolean`
- Renderiza bolhas de mensagem com:
  - Avatar/iniciais
  - Nome + role badge ("Admin" ou "Você"/"Usuário")
  - Body com `white-space: pre-wrap`
  - Timestamp relativo
  - Bolhas do `currentUserId` à direita (azul), demais à esquerda (cinza escuro)
- Auto-scroll ao final em mount/quando messages muda
- Estilo: dark, gradient, fonte do design system

### 2.7 `web/app/components/feedback/FeedbackThreadModal.vue` (novo)

Wrapper modal:
- Props: `modelValue: boolean`, `feedbackId: string`
- Em mount/watch `feedbackId`: chama `feedbackStore.getFeedback(id)` e `markFeedbackRead(id)`
- Cabeçalho: subject, kind badge, status badge, autor + loja, data
- `<FeedbackThread :messages="..." :current-user-id="auth.userId" />`
- Caixa de reply na parte inferior — textarea + botão "Enviar"
- Ao enviar: `replyToFeedback(id, body)` → atualiza messages localmente
- Estilo idêntico a `FeedbackFormModal.vue` (dark, gradient, transitions, body scroll lock)
- `Teleport to="body"`, `Transition` slideUp/fadeIn

### 2.8 `web/app/components/feedback/FeedbackWorkspace.vue` — reescrita visual

Mudanças:
- **Remover todo o styling claro** (`#dbeafe`, `#fce7f3`, etc) e adotar paleta dark do `AppDetailDialog` e do `dashboard-header`
- Toolbar redesenhada: campo de busca + filtros (kind, status, **loja**), botão "Atualizar"
- Filtro de loja: opções vindas das lojas acessíveis no estado (via `props`/store)
- Grade: dark headers, dark rows, hover sutil, status como **badge clicável** (popover de seleção de status para edição inline)
- Linha unread (`unread_for_admin === true`) ganha indicador lateral (barra azul) e fundo levemente destacado
- Coluna "Loja" exibindo `store_name` (ou "—" se vazio)
- Coluna "Remetente" exibindo `user_name`
- Click em linha → abre `<FeedbackThreadModal>` (substitui o `AppDetailDialog` atual)
- Suporte a `?focus={id}` na rota: ao montar, ler `route.query.focus` e abrir modal com aquele id; remover query depois com `router.replace`

Inline status edit:
- Pequeno popover ancorado na badge de status com 4 opções
- Ao escolher → `feedbackStore.updateFeedback(id, { status })`
- Toast de confirmação
- Fechar popover

### 2.9 `web/app/components/feedback/FeedbackFormModal.vue` — alteração mínima

Após sucesso do submit, chamar `feedbackStore.fetchSummary()` para atualizar o sino imediatamente.

### 2.10 `web/app/pages/feedback.vue` — sem mudança

Continua só renderizando `<FeedbackWorkspace />`.

### 2.11 `web/app/components/AGENTS.md` — alteração

Atualizar seção `feedback`:
- Acrescentar `FeedbackThread.vue` e `FeedbackThreadModal.vue`
- Mencionar que `FeedbackWorkspace.vue` foi redesenhado e usa `?focus={id}`
- Acrescentar bloco em `dashboard` para `DashboardHeaderNotifications.vue`

### 2.12 `web/AGENTS.md` — alteração

- Acrescentar `useFeedbackRealtime` na lista de composables realtime
- Acrescentar fluxo de bell + thread

### 2.13 `web/app/utils/workspaces.ts` — sem mudança

Workspace `feedback` continua sendo apenas para admin (já validado). Usuário comum acessa via sino.

### 2.14 `web/app/domain/utils/permissions.ts` — sem mudança

Mesmas permissões. Backend já filtra por escopo.

---

## Parte 3 — Documentação

### 3.1 `docs/review-feedback.md` — alteração

Adicionar nova seção ao final:

```markdown
---

## Fase 5.2.1 — Threads + Notificações (DATA)

### Mudanças
- Tabela user_feedback_messages com thread completa
- Flags unread_for_admin / unread_for_user no user_feedback
- Endpoints: GET /v1/feedback/{id}, POST /v1/feedback/{id}/messages,
  POST /v1/feedback/{id}/read, GET /v1/feedback/summary
- WebSocket /v1/realtime/feedback com topics feedback:tenant:* e feedback:user:*
- Sino no header (DashboardHeaderNotifications.vue)
- FeedbackThread.vue + FeedbackThreadModal.vue para conversa
- FeedbackWorkspace.vue redesenhado (dark + filtros + inline status edit)

### Decisões
- admin_note legado mantido (sem uso novo) para evitar refatoração ampla
- platform_admin sem tenantID: realtime tenant-scope é skipado;
  ele recebe via user-scope quando é dono

### Como testar
[lista de fluxos]
```

### 3.2 Memória do projeto

Salvar em `memory/feedback_realtime_pattern.md` o padrão de:
- `realtime.PublishXxxEvent` recebendo struct do módulo dono (evita ciclo de import)
- WebSocket topic compostos: `xxx:tenant:{id}` + `xxx:user:{id}`
- `unread_for_admin` / `unread_for_user` flags como modelo de "read receipt"

---

## Lista consolidada de arquivos

### Novos
| Arquivo | Tipo |
|---|---|
| `back/internal/platform/database/migrations/0030_feedback_messages.sql` | migration |
| `web/app/composables/useFeedbackRealtime.ts` | composable |
| `web/app/components/dashboard/DashboardHeaderNotifications.vue` | componente |
| `web/app/components/feedback/FeedbackThread.vue` | componente |
| `web/app/components/feedback/FeedbackThreadModal.vue` | componente |

### Alterados (backend)
| Arquivo | Ação |
|---|---|
| `back/internal/modules/feedback/model.go` | adicionar Message, expandir Feedback/View, novos inputs, PublishedEvent |
| `back/internal/modules/feedback/store_postgres.go` | scan com store_name, novos métodos thread+unread |
| `back/internal/modules/feedback/service.go` | reescrever List/Update + novos métodos Reply/Get/MarkRead/Summary |
| `back/internal/modules/feedback/http.go` | rotas extras |
| `back/internal/modules/feedback/AGENT.md` | descrever threads + realtime |
| `back/internal/modules/realtime/model.go` | novos event types + campos |
| `back/internal/modules/realtime/service.go` | PublishFeedbackEvent + HandleFeedbackSocket |
| `back/internal/modules/realtime/http.go` | rota /v1/realtime/feedback |
| `back/internal/modules/realtime/AGENT.md` | descrever feedback topics |
| `back/internal/platform/app/app.go` | passar realtimeService p/ feedback.NewService |
| `back/database/ERD.md` | adicionar tabela messages |

### Alterados (frontend)
| Arquivo | Ação |
|---|---|
| `web/app/stores/feedback.ts` | tipos novos, novas actions, applyRealtimeEvent |
| `web/app/components/dashboard/DashboardHeader.vue` | inserir <DashboardHeaderNotifications/> + emit open-feedback |
| `web/app/layouts/dashboard.vue` | mountar useFeedbackRealtime, capturar open-feedback, abrir thread modal |
| `web/app/components/feedback/FeedbackWorkspace.vue` | redesign dark + inline status + filtro de loja + ?focus |
| `web/app/components/feedback/FeedbackFormModal.vue` | chamar fetchSummary após submit |
| `web/app/components/AGENTS.md` | atualizar seções feedback + dashboard |
| `web/AGENTS.md` | mencionar useFeedbackRealtime |

### Documentação
| Arquivo | Ação |
|---|---|
| `docs/review-feedback.md` | nova seção 5.2.1 |
| `memory/feedback_realtime_pattern.md` | salvar padrão |

---

## Cuidados para evitar erros anteriores

1. **Sem `github.com/google/uuid`**: todos os IDs como `string`, `gen_random_uuid()` no banco, `RETURNING id::text`.
2. **`*string` em colunas nulláveis**: `tenant_id`, `store_id`, `s.name` no JOIN. Helper `scanFeedback` reescrito para incluir todos os novos campos consistentemente.
3. **`fmt.Sprintf("$%d", argCount)`** sempre. Nunca `string(rune(...))`.
4. **`WHERE 1=1`** + filtros condicionais em todas as queries com filtros opcionais.
5. **`nullableUUID()`** em qualquer insert que receba UUID que possa estar vazio.
6. **Sem ciclo de import**: `realtime` importa `feedback` (declarando struct `feedback.PublishedEvent`); `feedback` declara apenas a interface `RealtimePublisher` e injeta. Mesmo padrão que `operations`.
7. **Topic vazio no Hub**: pular explicitamente se tenantID/userID vazio para evitar `feedback:tenant:` solto.
8. **WebSocket auth via query**: `?access_token=...` (mesmo padrão).
9. **Migration aplicada via docker**: rodar `docker compose exec postgres psql ...` aplicando a 0030 após escrever.
10. **Build do api**: `docker compose up -d --build api` ao final, e validar `/healthz` antes de testar UI.
11. **Permissões DB**: NÃO precisa nova migration — endpoints novos protegidos por verificação de propriedade no service.
12. **`platform_admin` sem tenant**: `Summary()` retorna escopo só por user quando `principal.TenantID == ""`.
13. **Transação em `AddMessage`**: insert mensagem + update flags em uma única `pgx.Tx` para evitar inconsistência.

---

## Como testar (ao final)

1. `docker compose up -d --build api` + aplicar migration 0030
2. Logar como **owner** (tenant ativo, com lojas)
3. Logar como **consultor** (mesmo tenant) em outra janela/browser
4. Consultor envia feedback → owner deve ver toast push + sino com badge
5. Owner abre sino → vê resumo → clica → modal com thread → responde
6. Consultor recebe toast push + sino com badge → abre thread modal → responde
7. Owner muda status pelo inline edit na grade `/feedback` → consultor vê toast `feedback.updated`
8. Logar como **platform_admin** → confirmar que sino funciona quando ele é dono de algum feedback (criar via FAB)
9. Validar `?focus={id}` (clicar em item do sino sendo admin → abre `/feedback` com modal aberto)
10. Validar reconexão: derrubar `api` por 30s, subir novamente, ver WS reconectar

---

## Definition of done

- [ ] Migration 0030 aplicada
- [ ] `docker compose up -d --build api` retorna `/healthz` ok
- [ ] Backend: build sem erro, tests passando
- [ ] Endpoints novos respondem: GET summary, GET id, POST messages, POST read
- [ ] WS `/v1/realtime/feedback` aceita conexão e envia eventos
- [ ] Sino aparece para todos os usuários autenticados
- [ ] Toast push funciona em ambos os lados
- [ ] Badge de unread atualiza ao receber WS e ao marcar como lido
- [ ] Página `/feedback` redesenhada (dark, sem cores claras), filtros funcionais (incluindo loja)
- [ ] Inline status edit funciona na grade
- [ ] Thread modal: scroll automático, reply, status no header
- [ ] FeedbackFormModal mantido (sem regressão)
- [ ] AGENTS.md / AGENT.md / ERD.md atualizados
- [ ] `docs/review-feedback.md` com seção 5.2.1
- [ ] Memória `feedback_realtime_pattern.md` salva
