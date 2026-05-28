# Plano: Sistema de Alertas Operacionais Dinâmicos e Customizáveis

## Diagnostico local - 2026-05-02

Escopo desta analise: somente o ambiente local. A VPS nao foi usada como referencia porque o deploy de la esta atrasado em relacao a este trabalho.

### Veredito

A arquitetura pensada estava correta: `operations` detecta condicoes operacionais, `alerts` materializa o incidente com snapshot da regra, e o front renderiza por `displayKind`. O problema foi execucao incompleta entre as camadas. Existiam tabelas, endpoints e componentes, mas alguns contratos nao batiam; por isso a tela passava a sensacao de "tem tudo", enquanto o fluxo real quebrava antes de materializar ou exibir.

### O que deu errado

- O scanner retroativo de `operations.ScanForRule` estava como stub/no-op; entao "Aplicar agora" sempre tendia a gerar zero alertas.
- `alerts.ApplyRuleNow` e o scanner nao tinham contrato forte entre si; havia conversao fraca de retorno e sinais podiam ser descartados.
- A injecao `alertsService.SetOperationsScanner(operationsService)` nao estava conectada inicialmente; sem isso, retroatividade nao tinha como chamar operations.
- O scheduler ainda lia o threshold legado de `tenant_operational_alert_rules` e ignorava as regras dinamicas de `alert_rule_definitions`.
- `ProcessOperationalSignals` descartava campos importantes do sinal (`ConsultantName`, `ElapsedMinutes`, `TriggerType`), quebrando texto do alerta e escolha da regra.
- O materializador esperava metadados numericos em formato de JSON (`float64`), mas os sinais Go chegavam como `int/int64`; com isso `{elapsed}` podia virar `0 min`.
- A migration/backfill de regras usava `on conflict do nothing` sem chave unica; em reset/renumeracao local isso permitiu duplicar a regra padrao, como apareceu na lista.
- O modal de regra enviava `isActive` no create e, ao editar, tambem mandava campos como `id`, `tenantId`, `createdAt`, `updatedAt`; a API usa `DisallowUnknownFields`, entao rejeitava o payload antes de criar/atualizar.
- `applyRuleNow` no front retornava numero puro, mas a tela esperava `{ appliedCount }`.
- Componentes de display tentavam usar `titleTemplate/bodyTemplate` em alertas materializados, mas a API retorna o snapshot renderizado como `headline/body`.
- O card de atendimento tinha badge hardcoded "Atendimento longo" e nao usava `colorTheme/headline/body` do alerta.
- Toast aparecia para alerta de resposta obrigatoria, mesmo quando o `displayKind` nao era `toast`.
- Botoes de resposta em popup/banner existiam visualmente, mas parte deles nao chamava o endpoint de resposta corretamente.

### Correcoes aplicadas localmente

- `operations.ScanForRule` agora faz scan real para `long_open_service` em atendimentos ativos do tenant e retorna sinais retroativos.
- `alerts.ApplyRuleNow` agora chama o scanner injetado, transforma sinais de operations e publica invalidacao depois de materializar.
- O app local conecta `alertsService` e `operationsService` nos dois sentidos necessarios.
- O scheduler passa a considerar a menor regra dinamica ativa de `long_open_service`; se nao houver regra dinamica ativa, nao gera novo sinal temporizado para esse trigger.
- O materializador preserva consultor, tempo decorrido, trigger e snapshot da regra (`displayKind`, `colorTheme`, `responseOptions`, `isMandatory`, `ruleDefinitionId`).
- A renderizacao de template no backend ficou resiliente para metadados `int`, `int64`, `float64`, `json.Number` e string.
- O front normaliza os campos novos do alerta e usa `headline/body` como fallback dos templates.
- O store de alertas busca `format=definitions`, atualiza a lista local apos criar/editar/excluir e retorna `{ appliedCount }` em `applyRuleNow`.
- O payload de criar/editar regra agora e sanitizado no front antes de ir para API; create tambem envia `tenantId` local.
- A API passou a aceitar `tenantId` e `isActive` no create, e `triggerType` no update.
- O editor nao fecha mais sozinho antes da API confirmar sucesso; se salvar falhar, o usuario continua no modal.
- O card de atendimento agora mostra o texto real do alerta e aplica a cor vinda de `colorTheme`.
- Foi adicionada a migration `0050_deduplicate_alert_rule_definitions.sql` para remover duplicatas locais e criar indice unico em `(tenant_id, trigger_type, name)`. A migration `0047` tambem foi ajustada para fresh install nao recriar duplicatas.

### O que ainda nao esta pronto

- O unico trigger operacional funcionando de ponta a ponta agora e `long_open_service`.
- `long_queue_wait`, `long_pause`, `idle_store` e `outside_business_hours` ainda estao como stubs/no-op em `operations`.
- Multiplas regras ativas para o mesmo trigger ainda nao sao um caso bem fechado: o scheduler usa o menor threshold e o materializador escolhe uma regra ativa. O MVP deveria restringir uma regra por trigger ou implementar dedupe por regra.
- `UpdateRule` ainda precisa de validacao completa igual ao create antes de considerarmos o CRUD blindado.
- Excluir regra hoje remove fisicamente; se quisermos historico/auditoria melhor, o ideal e soft delete/desativacao.
- O filtro por consultor no consumo de alertas ainda precisa ser revisado se o consultor deve ver apenas os proprios alertas.

### Validacao local executada

- `go test ./internal/modules/alerts/...`
- `go test ./internal/modules/operations/...`
- `go test ./internal/platform/app`
- `go test ./...`
- `npm --prefix web run build`

## Contexto

O sistema atual tem regras de alerta fixas (1 linha por tenant em `tenant_operational_alert_rules` com 3 thresholds e 3 toggles) e só dispara `long_open_service`. Front renderiza apenas card badge + banner. O usuário pediu:

- 5+ tipos de display além do card (banner, toast, corner-popup, center-modal, fullscreen)
- Configuração 100% dinâmica na página `/alertas` (cor, texto, obrigatoriedade, tipo de interação)
- Triggers além de tempo de atendimento: tempo na fila, tempo em pausa, fora do horário comercial
- **Retroatividade**: ao salvar uma regra, atendimentos já em andamento que se enquadram disparam alerta imediatamente
- Identificação do consultor no texto do alerta (ou "Loja" quando geral)
- Manter placeholder para WhatsApp (sem integração real ainda)
- Sem ENUM nativo do Postgres — `varchar` + CHECK constraint (já é o padrão atual)

## Estado mapeado

- `back/internal/platform/database/migrations/0044_alerts_foundation.sql` — `tenant_operational_alert_rules` + `alert_instances` + `alert_actions`
- `back/internal/platform/database/migrations/0044_alert_interaction.sql` — adicionou `interaction_kind`, `interaction_response`, `responded_at`, `external_notified_at` em `alert_instances`
- Scheduler `ProcessTimedAlerts` roda a cada 15s em [back/internal/platform/app/app.go:88](back/internal/platform/app/app.go#L88)
- Operations já tem timestamps disponíveis: `ActiveServiceState.ServiceStartedAt`, `QueueStateItem.QueueJoinedAt`, `PausedStateItem.StartedAt` — não precisamos novas colunas em operations
- Página `/alertas` é renderizada por [web/app/components/alerts/AlertsWorkspace.vue](web/app/components/alerts/AlertsWorkspace.vue) com formulário fixo
- Banner já existe ([web/app/features/operation/components/OperationAlertBanner.vue](web/app/features/operation/components/OperationAlertBanner.vue)) e foi corrigido nesta sessão
- Backend hoje só conhece `SignalLongOpenServiceTriggered` / `SignalLongOpenServiceResolved` ([back/internal/modules/operations/alerts.go](back/internal/modules/operations/alerts.go))

## Decisões arquiteturais

1. **Regras dinâmicas substituem regras fixas**. A tabela `tenant_operational_alert_rules` continua existindo apenas para os 3 toggles globais (`notify_dashboard`, `notify_operation_context`, `notify_external`); os thresholds saem dela e migram para a nova tabela `alert_rule_definitions` (N regras por tenant, uma por tipo+escopo). Migration faz backfill automático criando 1 regra `long_open_service` para cada tenant existente.

2. **Display kind e interação ficam na regra, não hardcoded por tipo**. O alerta materializado em `alert_instances` armazena snapshot dos campos da regra (display_kind, color_theme, response_options, is_mandatory) — assim, alterar regra não muda alertas já criados; resolve corrida e simplifica frontend.

3. **Nome do consultor é denormalizado** em `alert_instances.consultant_name` (snapshot no momento da criação), igual já é feito em `service_history.person_name`. Quando não há consultor (ex: idle_store), fica vazio e o front renderiza "Loja".

4. **Retroatividade é síncrona via endpoint dedicado** `POST /v1/alerts/rules/{id}/apply-now` que faz scan imediato. O scheduler de 15s continua sendo a fonte primária; o apply-now é só para feedback imediato pós-edição da regra.

5. **Triggers adicionais reutilizam `OperationalAlertSignal`** com novos `SignalType` constants. A lógica de detecção fica em `operations/alerts.go` (build*Signals) e o módulo `alerts` consome.

6. **Sem ENUM nativo Postgres** — todos os campos novos são `varchar` com `CHECK (col in ('a','b','c'))`. Padrão já adotado no projeto.

## Arquivos críticos

**Migrations**
- `back/internal/platform/database/migrations/0046_alert_rule_definitions.sql` (novo)
- `back/internal/platform/database/migrations/0047_alert_instances_display_snapshot.sql` (novo)
- `back/internal/platform/database/migrations/0048_alert_instances_consultant_name.sql` (novo)

**Backend — alerts module**
- `back/internal/modules/alerts/model.go`
- `back/internal/modules/alerts/store_postgres.go`
- `back/internal/modules/alerts/service.go`
- `back/internal/modules/alerts/http.go`
- `back/internal/modules/alerts/service_test.go`
- `back/internal/modules/alerts/AGENT.md`

**Backend — operations module**
- `back/internal/modules/operations/alerts.go` (novos signal types)
- `back/internal/modules/operations/service.go` (build*Signals para queue/pause/outside-hours)

**Backend — orquestração**
- `back/internal/platform/app/app.go` (passar relógio/horário para coordinator)

**Frontend — store e página de alertas**
- `web/app/stores/alerts.ts`
- `web/app/components/alerts/AlertsWorkspace.vue`
- `web/app/components/alerts/AlertRuleEditor.vue` (novo)
- `web/app/components/alerts/AlertRuleList.vue` (novo)

**Frontend — componentes de display**
- `web/app/features/operation/components/OperationAlertBanner.vue` (refatorar para receber alerts via prop)
- `web/app/features/operation/components/AlertDisplayCornerPopup.vue` (novo)
- `web/app/features/operation/components/AlertDisplayCenterModal.vue` (novo)
- `web/app/features/operation/components/AlertDisplayFullscreen.vue` (novo)
- `web/app/features/operation/components/AlertDisplayHost.vue` (novo — orquestra todos)
- `web/app/features/operation/components/OperationActiveServiceCard.vue` (lê color_theme do alerta)
- `web/app/pages/operacao/index.vue` (substitui banner pelo host)
- `web/app/composables/useContextRealtime.ts` (toast respeita display_kind)
- `web/app/features/operation/components/AGENTS.md`

---

## Fase 1 — Banco de dados (3 migrations)

### 1.1 — `0046_alert_rule_definitions.sql`

Cria a tabela primária de regras dinâmicas. Faz backfill da regra `long_open_service` para tenants existentes a partir de `tenant_operational_alert_rules.long_open_service_minutes`.

```sql
create table if not exists alert_rule_definitions (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id) on delete cascade,
    name text not null,
    description text not null default '',
    is_active boolean not null default true,

    -- Trigger
    trigger_type varchar(40) not null check (trigger_type in (
        'long_open_service',
        'long_queue_wait',
        'long_pause',
        'idle_store',
        'outside_business_hours'
    )),
    threshold_minutes integer not null check (threshold_minutes > 0),
    severity varchar(20) not null default 'attention' check (severity in ('info','attention','critical')),

    -- Display
    display_kind varchar(30) not null default 'banner' check (display_kind in (
        'card_badge','banner','toast','corner_popup','center_modal','fullscreen'
    )),
    color_theme varchar(20) not null default 'amber' check (color_theme in (
        'amber','red','blue','green','purple','slate'
    )),
    title_template text not null,
    body_template text not null default '',

    -- Interaction
    interaction_kind varchar(30) not null default 'none' check (interaction_kind in (
        'none','dismiss','confirm_choice','select_option'
    )),
    response_options jsonb not null default '[]'::jsonb,
    is_mandatory boolean not null default false,

    -- Notification channels
    notify_dashboard boolean not null default true,
    notify_operation_context boolean not null default true,
    notify_external boolean not null default false,
    external_channel varchar(20) not null default 'none' check (external_channel in ('none','whatsapp','email')),

    -- Audit
    created_by uuid references users(id) on delete set null,
    updated_by uuid references users(id) on delete set null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create index if not exists alert_rule_definitions_tenant_active_idx
    on alert_rule_definitions (tenant_id, is_active, trigger_type);

-- Backfill: cria regra long_open_service para cada tenant existente
insert into alert_rule_definitions (
    tenant_id, name, trigger_type, threshold_minutes, severity,
    display_kind, color_theme, title_template, body_template,
    interaction_kind, response_options, is_mandatory,
    notify_dashboard, notify_operation_context, notify_external
)
select
    tenant_id,
    'Atendimento longo (padrão)',
    'long_open_service',
    long_open_service_minutes,
    'critical',
    'banner',
    'amber',
    'Atendimento aberto há {elapsed}',
    'O atendimento de {consultant} segue aberto acima do tempo configurado.',
    'confirm_choice',
    '[{"value":"still_happening","label":"Ainda está acontecendo"},{"value":"forgotten","label":"Esqueci de fechar"}]'::jsonb,
    false,
    notify_dashboard, notify_operation_context, notify_external
from tenant_operational_alert_rules
on conflict do nothing;

-- Rollback:
-- drop table if exists alert_rule_definitions;
```

**Variáveis de template suportadas** (resolvidas no backend ao materializar o alerta):
- `{elapsed}` — duração formatada (ex: "1h17min")
- `{consultant}` — nome do consultor (ou "Loja" se vazio)
- `{store}` — nome da loja
- `{threshold}` — valor do threshold (minutos)

### 1.2 — `0047_alert_instances_display_snapshot.sql`

Adiciona snapshot dos campos da regra na instância do alerta. Garante que mudar a regra não afeta alertas já criados.

```sql
alter table alert_instances
    add column if not exists rule_definition_id uuid references alert_rule_definitions(id) on delete set null,
    add column if not exists display_kind varchar(30) not null default 'banner' check (display_kind in (
        'card_badge','banner','toast','corner_popup','center_modal','fullscreen'
    )),
    add column if not exists color_theme varchar(20) not null default 'amber' check (color_theme in (
        'amber','red','blue','green','purple','slate'
    )),
    add column if not exists response_options jsonb not null default '[]'::jsonb,
    add column if not exists is_mandatory boolean not null default false;

-- Expande CHECK de interaction_kind para os novos valores
alter table alert_instances
    drop constraint if exists alert_instances_interaction_kind_check;
alter table alert_instances
    add constraint alert_instances_interaction_kind_check
    check (interaction_kind in ('none','reminder','response_required','dismiss','confirm_choice','select_option'));

-- Backfill: alertas long_open_service existentes ganham o display do banner clássico
update alert_instances
set display_kind = 'banner',
    color_theme = 'amber',
    response_options = '[{"value":"still_happening","label":"Ainda está acontecendo"},{"value":"forgotten","label":"Esqueci de fechar"}]'::jsonb,
    interaction_kind = 'confirm_choice'
where type = 'long_open_service';

-- Rollback:
-- alter table alert_instances drop column if exists rule_definition_id, display_kind, color_theme, response_options, is_mandatory;
```

### 1.3 — `0048_alert_instances_consultant_name.sql`

Denormaliza nome do consultor no alerta (snapshot). Backfill via join com `consultants` para alertas existentes.

```sql
alter table alert_instances
    add column if not exists consultant_name text not null default '';

update alert_instances ai
set consultant_name = c.name
from consultants c
where ai.consultant_id = c.id
  and ai.consultant_name = '';

-- Rollback:
-- alter table alert_instances drop column if exists consultant_name;
```

---

## Fase 2 — Backend: model + store

### 2.1 — `model.go`

**Novas constantes:**

```go
const (
    TriggerLongOpenService      = "long_open_service"
    TriggerLongQueueWait        = "long_queue_wait"
    TriggerLongPause            = "long_pause"
    TriggerIdleStore            = "idle_store"
    TriggerOutsideBusinessHours = "outside_business_hours"

    DisplayKindCardBadge   = "card_badge"
    DisplayKindBanner      = "banner"
    DisplayKindToast       = "toast"
    DisplayKindCornerPopup = "corner_popup"
    DisplayKindCenterModal = "center_modal"
    DisplayKindFullscreen  = "fullscreen"

    ColorThemeAmber  = "amber"
    ColorThemeRed    = "red"
    ColorThemeBlue   = "blue"
    ColorThemeGreen  = "green"
    ColorThemePurple = "purple"
    ColorThemeSlate  = "slate"

    InteractionKindDismiss       = "dismiss"
    InteractionKindConfirmChoice = "confirm_choice"
    InteractionKindSelectOption  = "select_option"

    ExternalChannelNone     = "none"
    ExternalChannelWhatsapp = "whatsapp"
    ExternalChannelEmail    = "email"
)
```

**Novo struct `RuleDefinition`:**

```go
type ResponseOption struct {
    Value string `json:"value"`
    Label string `json:"label"`
}

type RuleDefinition struct {
    ID               string
    TenantID         string
    Name             string
    Description      string
    IsActive         bool
    TriggerType      string
    ThresholdMinutes int
    Severity         string
    DisplayKind      string
    ColorTheme       string
    TitleTemplate    string
    BodyTemplate     string
    InteractionKind  string
    ResponseOptions  []ResponseOption
    IsMandatory      bool
    NotifyDashboard        bool
    NotifyOperationContext bool
    NotifyExternal         bool
    ExternalChannel        string
    CreatedAt              time.Time
    UpdatedAt              time.Time
}

type RuleDefinitionView struct {
    ID               string           `json:"id"`
    TenantID         string           `json:"tenantId"`
    Name             string           `json:"name"`
    Description      string           `json:"description"`
    IsActive         bool             `json:"isActive"`
    TriggerType      string           `json:"triggerType"`
    ThresholdMinutes int              `json:"thresholdMinutes"`
    Severity         string           `json:"severity"`
    DisplayKind      string           `json:"displayKind"`
    ColorTheme       string           `json:"colorTheme"`
    TitleTemplate    string           `json:"titleTemplate"`
    BodyTemplate     string           `json:"bodyTemplate"`
    InteractionKind  string           `json:"interactionKind"`
    ResponseOptions  []ResponseOption `json:"responseOptions"`
    IsMandatory      bool             `json:"isMandatory"`
    NotifyDashboard        bool       `json:"notifyDashboard"`
    NotifyOperationContext bool       `json:"notifyOperationContext"`
    NotifyExternal         bool       `json:"notifyExternal"`
    ExternalChannel        string     `json:"externalChannel"`
    CreatedAt              time.Time  `json:"createdAt"`
    UpdatedAt              time.Time  `json:"updatedAt"`
}

func (rule RuleDefinition) View() RuleDefinitionView { /* mapping direto */ }
```

**Inputs CRUD:**

```go
type CreateRuleInput struct {
    TenantID         string
    Name             string
    Description      string
    TriggerType      string
    ThresholdMinutes int
    Severity         string
    DisplayKind      string
    ColorTheme       string
    TitleTemplate    string
    BodyTemplate     string
    InteractionKind  string
    ResponseOptions  []ResponseOption
    IsMandatory      bool
    NotifyDashboard        bool
    NotifyOperationContext bool
    NotifyExternal         bool
    ExternalChannel        string
}

type UpdateRuleInput struct {
    // mesmos campos de CreateRuleInput, todos opcionais (ponteiros)
}

type ListRulesInput struct {
    TenantID    string
    TriggerType string
    OnlyActive  bool
}
```

**Adicionar campos ao struct `Alert`:**

```go
type Alert struct {
    // ... campos existentes
    RuleDefinitionID string
    DisplayKind      string
    ColorTheme       string
    ResponseOptions  []ResponseOption
    IsMandatory      bool
    ConsultantName   string
}
```

**Atualizar `AlertView` com os novos campos** (todos com tag JSON correspondente).

**Atualizar `View()` para mapear os novos campos.**

**Atualizar `OperationalSignalInput`** para suportar diferentes tipos:

```go
type OperationalSignalInput struct {
    // ... existentes
    ConsultantName string
    ElapsedMinutes int
    TriggerType    string  // identifica qual regra carregar
}
```

**Estender interface `Repository`:**

```go
type Repository interface {
    // ... métodos existentes
    ListRules(ctx context.Context, input ListRulesInput) ([]RuleDefinition, error)
    GetRule(ctx context.Context, ruleID string) (*RuleDefinition, error)
    CreateRule(ctx context.Context, input CreateRuleInput, actor Actor) (*RuleDefinition, error)
    UpdateRule(ctx context.Context, ruleID string, input UpdateRuleInput, actor Actor) (*RuleDefinition, error)
    DeleteRule(ctx context.Context, ruleID string) error
    LoadActiveRulesForTrigger(ctx context.Context, tenantID string, triggerType string) ([]RuleDefinition, error)
}
```

### 2.2 — `store_postgres.go`

**Implementar todos os métodos novos** (`ListRules`, `GetRule`, `CreateRule`, `UpdateRule`, `DeleteRule`, `LoadActiveRulesForTrigger`) com queries diretas. `LoadActiveRulesForTrigger` é o quente — usado pelo scheduler a cada 15s.

**Atualizar `processLongOpenTriggeredTx` (e novos `process*Tx`):**
- Antes do INSERT: chamar `LoadActiveRulesForTrigger(ctx, tenantID, triggerType)` para encontrar a regra que casa
- Pegar a primeira regra ativa do tipo (no MVP, 1 regra por trigger; futuro: múltiplas com filtros)
- Renderizar `title_template` e `body_template` substituindo `{elapsed}`, `{consultant}`, `{store}`, `{threshold}`
- INSERT em `alert_instances` com snapshot da regra: `rule_definition_id`, `display_kind`, `color_theme`, `response_options`, `is_mandatory`, `interaction_kind`, `consultant_name`
- Severidade vem da regra, não mais hardcoded `SeverityCritical`

**Atualizar todos os SELECTs de `alert_instances`** para incluir as novas colunas (`rule_definition_id`, `display_kind`, `color_theme`, `response_options`, `is_mandatory`, `consultant_name`). Atualizar `scanAlert`.

**Atualizar UPDATE do caminho de re-trigger** para também atualizar `display_kind`, `color_theme`, etc., caso a regra tenha mudado entre triggers.

### 2.3 — Renderização de template (utility)

Função pura no `model.go` ou helper:

```go
func renderTemplate(tmpl string, vars map[string]string) string {
    out := tmpl
    for k, v := range vars {
        out = strings.ReplaceAll(out, "{"+k+"}", v)
    }
    return out
}

func formatElapsed(d time.Duration) string {
    minutes := int(d.Minutes())
    if minutes < 60 {
        return fmt.Sprintf("%d min", minutes)
    }
    hours := minutes / 60
    rem := minutes % 60
    if rem == 0 {
        return fmt.Sprintf("%dh", hours)
    }
    return fmt.Sprintf("%dh%dmin", hours, rem)
}
```

---

## Fase 3 — Backend: service + http + retroatividade

### 3.1 — `service.go`

**Novos métodos no `Service`:**

```go
func (s *Service) ListRules(ctx, principal, tenantID, filters) ([]RuleDefinitionView, error)
func (s *Service) GetRule(ctx, principal, ruleID) (*RuleDefinitionView, error)
func (s *Service) CreateRule(ctx, principal, input CreateRuleInput) (*RuleDefinitionView, error)
func (s *Service) UpdateRule(ctx, principal, ruleID, input UpdateRuleInput) (*RuleDefinitionView, error)
func (s *Service) DeleteRule(ctx, principal, ruleID) error
func (s *Service) ApplyRuleNow(ctx, principal, ruleID) (appliedCount int, err error)
```

**Validações em `Create`/`Update`:**
- `trigger_type` ∈ lista permitida
- `threshold_minutes > 0`
- `display_kind` ∈ lista permitida
- `color_theme` ∈ lista permitida
- `interaction_kind` ∈ lista permitida
- Se `interaction_kind` ∈ {`confirm_choice`, `select_option`} → `response_options` deve ter ≥ 2 itens
- Se `is_mandatory == true` → `interaction_kind != none` (mandatório precisa de algum input)

**Permissões:**
- CRUD de regras: `alerts.rules.manage` (já existe) ou `workspace.alertas.edit`
- `ApplyRuleNow`: mesma permissão de CRUD

**`ApplyRuleNow` (retroatividade):**

```go
func (s *Service) ApplyRuleNow(ctx, principal, ruleID) (int, error) {
    rule, err := s.repo.GetRule(ctx, ruleID)
    if err != nil { return 0, err }
    if s.operationsScanner == nil { return 0, ErrNotImplemented }

    signals, err := s.operationsScanner.ScanForRule(ctx, *rule)
    if err != nil { return 0, err }

    mutations, err := s.repo.ProcessOperationalSignals(ctx, signals)
    if err != nil { return 0, err }

    s.publishContextForMutations(ctx, mutations)
    return len(mutations), nil
}
```

Isso requer expor uma interface `OperationsScanner` no service:

```go
type OperationsScanner interface {
    ScanForRule(ctx context.Context, rule RuleDefinition) ([]OperationalSignalInput, error)
}

func (s *Service) SetOperationsScanner(scanner OperationsScanner) { ... }
```

A implementação vive em `operations` module (Fase 4).

### 3.2 — `http.go`

**Novos endpoints:**

```
GET    /v1/alerts/rules                    Lista regras (filtros: tenantId, triggerType, onlyActive)
POST   /v1/alerts/rules                    Cria nova regra
GET    /v1/alerts/rules/{id}               Detalhe de uma regra
PATCH  /v1/alerts/rules/{id}               Atualiza regra
DELETE /v1/alerts/rules/{id}               Remove regra
POST   /v1/alerts/rules/{id}/apply-now     Dispara scan retroativo
```

**Manter compatibilidade temporária** com `GET /v1/alerts/rules?tenantId=X` que hoje retorna `RulesView` — adicionar query param `?format=definitions` para o novo retorno; o front começa a usar `?format=definitions` na nova UI. Após migração completa do front, remover o formato antigo em PR posterior.

### 3.3 — `service_test.go`

- Atualizar `fakeRepository` com os novos métodos da interface
- Adicionar testes:
  - `TestCreateRuleValidatesInteractionKind` (response_options obrigatório p/ confirm_choice)
  - `TestApplyRuleNowGeneratesSignals` (scanner mockado retorna 3 atendimentos > threshold)
  - `TestUpdateRuleSnapshotDoesNotMutateExistingAlerts` (alert criado antes da edição mantém display_kind antigo)

---

## Fase 4 — Operations: novos triggers

### 4.1 — `operations/alerts.go`

Adicionar constantes:

```go
const (
    SignalLongOpenServiceTriggered = "long_open_service.triggered"
    SignalLongOpenServiceResolved  = "long_open_service.resolved"

    SignalLongQueueWaitTriggered  = "long_queue_wait.triggered"
    SignalLongQueueWaitResolved   = "long_queue_wait.resolved"
    SignalLongPauseTriggered      = "long_pause.triggered"
    SignalLongPauseResolved       = "long_pause.resolved"
    SignalIdleStoreTriggered      = "idle_store.triggered"
    SignalIdleStoreResolved       = "idle_store.resolved"
    SignalOutsideHoursTriggered   = "outside_business_hours.triggered"
)
```

Adicionar campo na `OperationalAlertSignal`:

```go
type OperationalAlertSignal struct {
    // ... existentes
    ConsultantName string
    ElapsedMinutes int
    TriggerType    string
}
```

### 4.2 — `operations/service.go`

Refatorar interface `AlertCoordinator` para regras dinâmicas:

```go
type AlertCoordinator interface {
    LoadActiveRules(ctx context.Context, storeID string) (map[string][]RuleSummary, error)
    ReceiveOperationalSignals(ctx context.Context, signals []OperationalAlertSignal) error
}

type RuleSummary struct {
    ID               string
    TriggerType      string
    ThresholdMinutes int
}
```

Service itera por cada `triggerType` presente no map e roda o builder correspondente.

**Novos builders:**

```go
func (s *Service) buildLongQueueWaitSignals(ctx, storeID, snapshot, rules, now) []OperationalAlertSignal {
    // Para cada item em snapshot.WaitingList:
    //   se now - QueueJoinedAt > threshold → emit signal
    //   ConsultantName resolvido por lookup
}

func (s *Service) buildLongPauseSignals(ctx, storeID, snapshot, rules, now) []OperationalAlertSignal {
    // Para cada item em snapshot.PausedEmployees:
    //   se now - StartedAt > threshold → emit signal
}

func (s *Service) buildIdleStoreSignals(ctx, storeID, snapshot, rules, now) []OperationalAlertSignal {
    // Se WaitingList vazia + ActiveServices vazio + sem service finalizado nos últimos N min → emit
    // ConsultantID e ServiceID vazios; alerta "geral da loja"
}

func (s *Service) buildOutsideHoursSignals(ctx, storeID, snapshot, rules, now) []OperationalAlertSignal {
    // Para cada ActiveService:
    //   se started_at fora do horário comercial da loja → emit
    // Requer integração com store hours (já existe? verificar settings module)
}
```

**Resolução de nome do consultor:** adicionar método em `repository.LoadConsultantNamesForStore(ctx, storeID) map[string]string` (cache em memória por 5 min é otimização futura).

**Implementar `OperationsScanner` para retroatividade:**

```go
func (s *Service) ScanForRule(ctx context.Context, rule alerts.RuleDefinition) ([]alerts.OperationalSignalInput, error) {
    // Carrega snapshot atual de todas as lojas do tenant
    // Roda o builder específico para rule.TriggerType
    // Retorna signals
}
```

E injetar no app.go:

```go
alertsService.SetOperationsScanner(operationsService)
```

### 4.3 — `operations/service_alerts_test.go`

Adicionar testes para os novos builders (long_queue_wait, long_pause). idle_store e outside_hours podem ficar como TODO no MVP se prazo apertar — mas a estrutura deve estar pronta.

---

## Fase 5 — Frontend: store + página de alertas

### 5.1 — `web/app/stores/alerts.ts`

**Novo estado:**

```ts
const ruleDefinitions = ref<RuleDefinitionView[]>([])
const rulesPending = ref(false)
```

**Novos métodos:**

```ts
async function fetchRuleDefinitions(filters?: { triggerType?: string; onlyActive?: boolean }): Promise<RuleDefinitionView[]>
async function createRule(input: CreateRuleInput): Promise<RuleDefinitionView>
async function updateRule(ruleId: string, input: UpdateRuleInput): Promise<RuleDefinitionView>
async function deleteRule(ruleId: string): Promise<void>
async function applyRuleNow(ruleId: string): Promise<{ appliedCount: number }>
```

**Adicionar campos ao `normalizeAlert`:**

```ts
displayKind: normalizeText(alert.displayKind) || "banner",
colorTheme: normalizeText(alert.colorTheme) || "amber",
responseOptions: Array.isArray(alert.responseOptions) ? alert.responseOptions : [],
isMandatory: normalizeBoolean(alert.isMandatory, false),
consultantName: normalizeText(alert.consultantName),
ruleDefinitionId: normalizeText(alert.ruleDefinitionId),
```

**Manter `updateRules()` antigo** funcionando por compatibilidade até a UI ser totalmente migrada (os 3 toggles globais continuam em `tenant_operational_alert_rules`).

### 5.2 — `AlertsWorkspace.vue`

Refatorar para 2 abas/seções:
- **Regras** (novo): lista `ruleDefinitions` em uma tabela; botão "Nova regra" abre `AlertRuleEditor`; ações por linha: editar, ativar/desativar, aplicar agora, excluir
- **Histórico** (atual): grid de `alert_instances` com filtros

Manter card de overview no topo.

Os 3 toggles globais (`notify_dashboard`, `notify_operation_context`, `notify_external`) ficam em uma seção colapsável "Configurações globais" abaixo das regras.

### 5.3 — `AlertRuleEditor.vue` (novo)

Modal/drawer de edição com seções:

**Seção 1 — Identificação**
- Nome (obrigatório)
- Descrição (opcional)
- Toggle Ativa/Inativa

**Seção 2 — Trigger**
- Select `triggerType` (5 opções com descrição)
- Input `thresholdMinutes`
- Select `severity` (info / atenção / crítica)

**Seção 3 — Apresentação**
- Select `displayKind` (6 opções com preview pictográfico)
- Select `colorTheme` (6 opções com swatch)
- Textarea `titleTemplate` com hint mostrando variáveis disponíveis (`{elapsed}`, `{consultant}`, `{store}`, `{threshold}`)
- Textarea `bodyTemplate` (mesma hint)

**Seção 4 — Interação**
- Select `interactionKind` (none / dismiss / confirm_choice / select_option)
- Quando `confirm_choice` ou `select_option`: editor dinâmico de `responseOptions` (lista value/label, mín 2)
- Toggle `isMandatory` (desabilitado se interaction_kind = none)

**Seção 5 — Notificação**
- Toggle `notifyDashboard`
- Toggle `notifyOperationContext`
- Toggle `notifyExternal`
- Quando `notifyExternal`: select `externalChannel` (none / whatsapp / email)

**Footer:** botões Salvar / Cancelar / "Salvar e aplicar agora" (chama `applyRuleNow` após o save).

### 5.4 — `AlertRuleList.vue` (novo)

Tabela compacta com colunas: nome, trigger, threshold, display, status (ativa/inativa), última atualização, ações.

---

## Fase 6 — Frontend: componentes de display dinâmicos

### 6.1 — `AlertDisplayHost.vue` (novo)

Componente único na página de operação que substitui o uso direto do `OperationAlertBanner`. Recebe `storeId`; consulta `alertsStore.activeAlertsForStore(storeId)` filtrado por role; agrupa por `displayKind`; renderiza cada subgrupo com o componente correto:

```vue
<template>
  <div>
    <OperationAlertBanner :alerts="byKind.banner" v-if="byKind.banner.length" />
    <AlertDisplayCornerPopup :alerts="byKind.corner_popup" v-if="byKind.corner_popup.length" />
    <AlertDisplayCenterModal :alerts="byKind.center_modal" v-if="byKind.center_modal.length" />
    <AlertDisplayFullscreen :alerts="byKind.fullscreen" v-if="byKind.fullscreen.length" />
    <!-- toast e card_badge ficam responsabilidade do toast system / card -->
  </div>
</template>
```

### 6.2 — `OperationAlertBanner.vue`

Refatorar para receber `:alerts="banners"` como prop em vez de buscar do store. Mantém o visual atual (ok confirmado pelo usuário). Usa `alert.colorTheme` para o gradiente (mapa `amber→#78350f`, `red→#7f1d1d`, etc.). Renderiza `alert.responseOptions` em vez de hardcoded "Ainda está acontecendo / Esqueci de fechar". Inclui `alert.consultantName` no título quando não vazio.

### 6.3 — `AlertDisplayCornerPopup.vue` (novo)

Card flutuante no canto inferior direito (mais persistente que toast, menos invasivo que modal). Empilhável. Botão de fechar se `interactionKind == dismiss`. Não bloqueia interação com a tela.

### 6.4 — `AlertDisplayCenterModal.vue` (novo)

Modal centralizado com backdrop. Se `isMandatory`, backdrop não fecha por click-fora. Header com cor do tema, body com `alert.body`, footer com `responseOptions` como botões (ou X de dismiss).

### 6.5 — `AlertDisplayFullscreen.vue` (novo)

Overlay tela inteira (`position: fixed; inset: 0`), cor de fundo intensa (gradiente do tema), título XL, body, botões grandes. Sempre `isMandatory` — não fecha sem responder. Para alertas críticos.

### 6.6 — `OperationActiveServiceCard.vue`

Atualizar para usar `alert.colorTheme` em vez de hardcoded amber. Adicionar mapa `colorThemeToCardClass` em utility.

### 6.7 — `useContextRealtime.ts`

Toast só aparece para alertas com `displayKind === 'toast'`. Outros tipos são consumidos pelo `AlertDisplayHost`.

### 6.8 — `pages/operacao/index.vue`

Trocar `<OperationAlertBanner :store-id="bannerStoreId" />` por `<AlertDisplayHost :store-id="bannerStoreId" />`.

---

## Fase 7 — Validação + AGENT.md

### 7.1 — Testes
- `cd back && go test ./internal/modules/alerts/...` — todos verdes
- `cd back && go test ./internal/modules/operations/...` — sem regressão
- `cd back && go test ./...` — geral verde
- `npm --prefix web run build` — build limpo

### 7.2 — AGENT.md updates
- `back/internal/modules/alerts/AGENT.md`: documentar nova arquitetura (rules dinâmicas, snapshot na instância, retroatividade), endpoints `/rules*`, lista de display_kinds, color_themes, interaction_kinds, variáveis de template
- `back/internal/modules/operations/AGENT.md`: documentar novos signal types e responsabilidade de scan retroativo
- `web/app/features/operation/components/AGENTS.md`: documentar `AlertDisplayHost` e os 3 novos componentes; explicar que o display_kind do alerta dita qual componente renderiza
- `web/AGENTS.md`: nota sobre `useContextRealtime` filtrar toast só para `displayKind=toast`

### 7.3 — Smoke manual
- Criar regra `Atendimento longo` com display=banner, threshold=2 min → atendimento já em curso há 5 min dispara o banner em ≤15s (ou imediato se aplicado via "Salvar e aplicar agora")
- Criar regra `Pausa longa` com display=corner_popup, threshold=10 min → pausar consultor e aguardar
- Criar regra `Loja parada` com display=center_modal + isMandatory → ficar 5 min sem atendimento
- Criar regra `Atendimento crítico` com display=fullscreen + isMandatory + interaction=confirm_choice → atendimento > 30 min
- Editar regra (mudar cor/threshold) → alertas existentes mantêm valores antigos; novos triggers usam novos valores
- Excluir regra ativa → para de gerar novos alertas; existentes seguem ativos até resolvidos

---

## O que fica fora deste plano

- Integração real com WhatsApp (placeholder mantido — `external_channel` + `MarkExternalNotified`)
- Filtros de regras por loja (MVP: regra é tenant-wide; futuro: `store_ids[]` na regra para escopo)
- Múltiplas regras concorrentes do mesmo trigger (MVP: 1 regra por trigger; segunda regra causaria 2 alertas)
- Templates com lógica condicional (`{#if consultant}...{/if}`) — só substituição simples
- Auditoria de quem editou cada regra além de `updated_by` (sem histórico)
- Triggers compostos (ex: "atendimento longo + consultor X") — no MVP cada regra é 1 trigger só
- Internacionalização dos templates — strings em pt-BR

## Estimativa de esforço

- Fase 1 (migrations): 30min
- Fase 2 (model + store): 3h
- Fase 3 (service + http + retroatividade): 3h
- Fase 4 (operations triggers): 4h
- Fase 5 (store + página alertas): 4h
- Fase 6 (display components): 5h
- Fase 7 (validação + docs): 1.5h

**Total: ~21h** distribuído em 2-3 dias de trabalho focado.
