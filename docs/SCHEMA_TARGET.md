# Schema Target — Postgres pós-reestruturação

Branch: `refactor/multi-tenant-core`
Plano: `~/.claude/plans/preciso-que-analise-nosso-ancient-orbit.md` (seção A.2 e B.3)

Estado-alvo do banco PostgreSQL após a Fase 6. Estado intermediário em cada fase está descrito ao final.

---

## 1. Visão geral dos schemas

```
PostgreSQL
├── public/                       # legado em transição (vazio ao fim da Fase 6)
│
├── core/                         # plataforma: identidade, multi-tenant, RBAC, modules
│   ├── organizations             # agência (opcional)
│   ├── accounts                  # cliente (substitui public.tenants)
│   ├── users                     # identidade global (1 e-mail = 1 user)
│   ├── account_users             # membership user↔account
│   ├── organization_users        # membership user↔organization
│   ├── user_sessions             # JWT sessionId → revogação granular
│   ├── modules                   # catálogo (preenchido no boot)
│   ├── account_modules           # módulos habilitados por Account
│   ├── permissions               # catálogo declarativo
│   ├── role_templates            # templates declarados pelos módulos
│   ├── role_template_permissions # matriz template → permissões
│   ├── roles                     # cargos efetivos da Account (clones de templates)
│   ├── role_permissions          # permissões por role
│   ├── user_role_assignments     # atribuição user↔role na Account
│   ├── user_permission_overrides # allow/deny granular
│   └── event_outbox              # opcional, para handlers críticos
│
├── queue/                        # ex-fila-atendimento (atual produto único)
│   ├── stores                    # lojas físicas
│   ├── consultants               # roster de consultores
│   ├── settings                  # config operacional por tenant
│   ├── catalog                   # produtos/categorias para fila
│   ├── operations_*              # fila operacional
│   ├── feedback_*                # canal de feedback dos usuários
│   ├── alerts_*                  # regras e incidentes
│   ├── analytics_*               # leituras gerenciais consolidadas
│   ├── reports_*                 # leituras analíticas históricas
│   └── erp_*                     # ingestão FTP/ERP
│
├── contacts/                     # core opcional — fonte de verdade de "cliente do cliente"
│   ├── contacts                  # pessoas/empresas
│   └── contact_*                 # tags, fields custom, etc.
│
├── finance/                      # módulo satélite
│   ├── customers                 # local fallback se contacts não habilitado
│   ├── invoices
│   ├── payments
│   └── ...
│
├── tasks/                        # módulo satélite (notion-like)
│   ├── boards
│   ├── tasks
│   └── ...
│
├── omni/                         # módulo satélite (WhatsApp/Instagram)
│   ├── channels
│   ├── conversations
│   └── ...
│
├── site/                         # módulo satélite
└── bio/                          # módulo satélite
```

---

## 2. Regras de integridade entre schemas

### 2.1 FKs cross-schema permitidas

```
queue.* ────► core.accounts(id)        ✓
queue.* ────► core.users(id)           ✓
finance.* ──► core.accounts(id)        ✓
finance.* ──► core.users(id)           ✓
contacts.* ─► core.accounts(id)        ✓
tasks.* ────► core.accounts(id)        ✓
... etc
```

Todo módulo satélite "depende de" `core` (identidade + multi-tenant). FKs nesse sentido são esperadas.

### 2.2 FKs cross-schema PROIBIDAS

```
finance.* ──► contacts.contacts(id)    ✗  (usar UUID livre + Resolver in-process)
finance.* ──► queue.stores(id)         ✗  (usar UUID livre)
omni.* ─────► contacts.contacts(id)    ✗  (usar UUID livre + Resolver in-process)
queue.* ────► contacts.contacts(id)    ✗  (usar UUID livre)
```

**Por quê**: extrair um módulo para microserviço fica trivial — drop schema não trava por FK.

**Como integrar então**: módulo declara `OptionalDependency("contacts")` no `Build()`. Recebe `contacts.Resolver` injetado. Se o Account não tem `contacts` habilitado, Resolver retorna `ErrNotEnabled` e o módulo cai no fallback (ex: tabela local `finance.customers`).

### 2.3 FKs internas livres

Dentro do MESMO schema, FKs normais. Ex: `finance.invoice_items.invoice_id` → `finance.invoices(id)` ON DELETE CASCADE.

---

## 3. Schema `core` em detalhe

| Tabela | Propósito | FKs principais |
|---|---|---|
| `organizations` | Agência (opcional). Agrupa accounts. | — |
| `accounts` | Cliente do SaaS. Pode ou não pertencer a uma organization. | `organization_id → organizations(id)` (nullable) |
| `users` | Identidade global. 1 e-mail = 1 user, vive sozinho. | — |
| `account_users` | Membership user↔account. | `account_id → accounts(id)`, `user_id → users(id)` |
| `organization_users` | Membership user↔organization (modo agência). | `organization_id → organizations(id)`, `user_id → users(id)` |
| `user_sessions` | Sessões ativas. JWT carrega `sessionId`. `revoked_at` faz logout funcionar. | `user_id → users(id)` |
| `modules` | Catálogo populado no boot pelo Module Registry. | — |
| `account_modules` | Quais módulos cada Account tem habilitados. Carrega `config jsonb` por módulo. | `account_id → accounts(id)`, `module_id → modules(id)` |
| `permissions` | Catálogo declarativo de permissões (declaradas pelos módulos). `deprecated_at` quando módulo remove a key. | `module_id → modules(id)` |
| `role_templates` | Templates de cargo declarados pelos módulos. | `module_id → modules(id)` |
| `role_template_permissions` | Matriz template→permissões. Imutável após template criado. | `role_template_id`, `permission_key` |
| `roles` | Cargos efetivos da Account. Clones editáveis dos templates. `is_locked` para Owner. | `account_id → accounts(id)`, `cloned_from_template_id → role_templates(id)` |
| `role_permissions` | Permissões efetivas por role. Editado pelo cliente. | `role_id → roles(id)`, `permission_key → permissions(key)` |
| `user_role_assignments` | Quem tem qual cargo em qual Account. | `account_id`, `user_id`, `role_id` |
| `user_permission_overrides` | Allow/deny granular por usuário. Convive com role. | `account_id`, `user_id`, `permission_key` |
| `event_outbox` | Opcional. Eventos críticos persistidos antes de publicar (outbox pattern). | — |

---

## 4. Mapa de migração `public.*` → `queue.*`

A partir da Fase 4A (escalonada):

| Tabela atual (`public.*`) | Schema alvo | Sub-fase |
|---|---|---|
| `tenants` | `core.accounts` (mesmo `id`) | Fase 1 (cópia) + Fase 4 (deprecação) |
| `stores`, `consultants`, `settings_*`, `catalog_*` | `queue.*` | Fase 4A |
| `operations_*`, `feedback_*` | `queue.*` | Fase 4B |
| `alerts_*`, `analytics_*`, `reports_*`, `erp_*` | `queue.*` | Fase 4C |
| `users` (atual) | `core.users` (cópia, `core.users.id` igual) + `core.account_users` | Fase 1 |
| `access_permissions`, `access_role_permissions`, `user_access_overrides` | `core.permissions` + `core.role_permissions` (por account) + `core.user_permission_overrides` | Fase 3 |
| `user_platform_roles`, `user_tenant_roles`, `user_store_roles` | `core.user_role_assignments` (após criação dos roles efetivos por account) | Fase 3 |

Durante a transição, **views compatíveis** em `public.*` apontam para os novos schemas para que código não-migrado continue lendo (Fase 4A explícita: views).

---

## 5. Estado em cada fase

### Após Fase 0 (atual)
Apenas docs e feature-flag. Banco intocado.

### Após Fase 1
- Schema `core` existe com todas as tabelas.
- `core.accounts` tem cópia de `public.tenants`.
- `core.users` tem cópia de `public.users`.
- `core.account_users` populada.
- `public.*` continua intocado e em uso ativo.

### Após Fase 2
- `core.modules`, `core.permissions`, `core.role_templates` populados pelo `SyncCatalog` no boot (por enquanto, listando os módulos atuais como satélites).
- `core.account_modules` tem todos os módulos atuais habilitados para todos os accounts existentes (para não quebrar nada).

### Após Fase 3
- `core.roles` por account com cópias dos roles fixos atuais (Owner, Manager, etc.).
- `core.role_permissions` populadas.
- `core.user_role_assignments` populadas a partir das tabelas atuais `user_*_roles`.
- Tabelas atuais `access_*` continuam intocadas (fallback).

### Após Fase 4A
- Schema `queue` existe. `stores`, `consultants`, `settings_*`, `catalog_*` movidas para `queue.*`.
- Views `public.stores`, `public.consultants`, etc. apontam para `queue.*`.

### Após Fase 4B
- `operations_*`, `feedback_*` movidas para `queue.*`.

### Após Fase 4C
- `alerts_*`, `analytics_*`, `reports_*`, `erp_*` movidas para `queue.*`.
- Views `public.*` ainda existem para compatibilidade até Fase 6.

### Após Fase 6 (estado-alvo)
- Schemas: `core`, `queue`, `contacts` (se trazido), `finance`, `tasks`, `omni`, `site`, `bio`.
- `public` vazio (ou apenas com tabelas de migrations da plataforma de migration tooling).
- Tabelas legadas `tenants`, `stores`, etc. dropadas em `public`.

---

## 6. Convenções de migration

- **Numeração**: continuamos a sequência atual (`0058_*`, `0059_*`, ... atualmente). Migrations da reestruturação começam em `0100_*` para deixar gap visual entre legado e novo.
- **Nome**: `NNNN_<schema>_<descricao>.sql`. Ex: `0100_core_schema.sql`, `0110_queue_schema_init.sql`, `0111_queue_move_stores.sql`.
- **Idempotência**: toda migration deve poder rodar 2× sem quebrar (`IF NOT EXISTS`, `CREATE OR REPLACE`, etc.) — facilita ambientes de dev/staging.
- **Reversão**: para mudanças destrutivas (drop tabela, drop coluna), criar migration de rollback comentada no mesmo arquivo, ou em arquivo `NNNN_rollback_*.sql` separado.
- **Backups**: antes da Fase 4 em produção, snapshot completo do banco. Antes da Fase 6 idem para cada módulo.

---

## 7. Diagrama ASCII do core (referência rápida)

```
┌─────────────────┐     ┌──────────────────┐
│  organizations  │◄────│    accounts      │
└─────────────────┘     │ organization_id? │
        │               └──────────────────┘
        │                      │
        │                      │
        │               ┌──────────────────┐
        │               │  account_users   │◄──────┐
        │               └──────────────────┘       │
        │                      │                   │
┌───────────────────┐          │            ┌──────────────┐
│ organization_users│──────────┼───────────►│    users     │
└───────────────────┘          │            └──────────────┘
                               │                   ▲
                               │                   │
                        ┌──────────────┐           │
                        │    roles     │           │
                        │ account_id   │           │
                        └──────────────┘           │
                               │                   │
                               │           ┌─────────────────┐
                               └──────────►│ user_role_      │
                                           │   assignments   │
                                           └─────────────────┘
                                                   │
                                                   │
                                           ┌────────────────────┐
                                           │ user_permission_   │
                                           │     overrides      │
                                           └────────────────────┘

                        ┌──────────────┐  ┌──────────────────┐
                        │   modules    │◄─│ account_modules  │
                        └──────────────┘  └──────────────────┘
                               │
                               │
                        ┌──────────────┐  ┌─────────────────────────────┐
                        │ permissions  │◄─│ role_template_permissions   │
                        └──────────────┘  └─────────────────────────────┘
                               │                       │
                               │                ┌──────────────────┐
                               └───────────────►│ role_permissions │
                                                └──────────────────┘
```
