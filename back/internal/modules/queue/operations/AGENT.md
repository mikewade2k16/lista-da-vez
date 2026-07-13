# AGENT

## Escopo

Estas instrucoes valem para `back/internal/modules/operations`.

## Responsabilidade do modulo

O modulo `operations` cuida da fila operacional por loja.

Hoje ele deve responder por:

- snapshot da operacao por loja
- overview integrado da operacao para usuarios com escopo multi-loja
- entrada na fila
- pausa e retomada
- retirada da fila para tarefa ou reuniao
- inicio de atendimento
- encerramento de atendimento
- cronometro autoritativo dos alertas temporais ligados ao estado operacional aberto
- persistencia do historico operacional
- persistencia das sessoes de status dos consultores

Ele nao deve cuidar de:

- auth
- configuracoes do modal e catalogos
- busca remota de produtos para autocomplete
- campanhas como fonte de verdade
- relatorios server-side
- websocket

## Contrato minimo para plugar em outro projeto

O service de `operations` nao deve depender do projeto host inteiro.

Hoje o contrato minimo de entrada do modulo e:

- `AccessContext`
- `Repository`
- `StoreScopeProvider`
- `EventPublisher`
- `GoalProgressProvider` (opcional)

### `AccessContext`

Representa o minimo que o modulo precisa da autenticacao:

- `user_id`
- `tenant_id`
- `role`
- `store_ids[]`

O adapter HTTP atual converte `auth.Principal` para esse contrato, mas outro projeto host pode fornecer o mesmo shape a partir do seu proprio auth.

### `StoreScopeProvider`

Representa o minimo que o modulo precisa do cadastro de lojas:

- listar lojas acessiveis do usuario
- devolver `id`, `tenantId`, `code`, `name`, `city`

O modulo nao deve depender do CRUD inteiro de lojas para funcionar.

### `Repository`

Representa a persistencia operacional:

- roster
- estado corrente da fila
- atendimentos ativos
- pausas/tarefas
- status corrente
- append de historico
- append de sessoes

### `EventPublisher`

Representa a invalidacao/realtime.

Pode ser:

- websocket
- broker
- noop

Se o host nao quiser realtime, o modulo continua funcionando com publisher noop.

### `GoalStats` no snapshot — meta canonica + vendido do ERP

Cada consultor do snapshot/overview pode carregar `goalStats` (em reais):
`monthlyGoal`, `soldValue`, `remainingToGoal`, `progress` (%, pode passar de 100),
`hasGoal`. O front usa para o anel de meta no card da fila.

Fonte de cada numero (montado em `combineGoalStats`, service.go):
- a META e CANONICA de `queue.operation_goal_targets` (meta individual do consultor
  no mes; senao a meta da loja como fallback), resolvida por
  `Repository.EffectiveMonthlyGoalByConsultant(storeIDs, month)`. A coluna
  `queue.consultants.monthly_goal` NAO e a fonte (fica zerada). Por ser in-domain,
  a meta aparece ATE em loja sem ERP.
- o VENDIDO vem do ERP via `GoalProgressProvider` (abaixo). Quando o ERP ja cobre o
  consultor (meta>0), usa-se o stat inteiro do ERP (bate exatamente com `/v1/erp/crm`);
  para os demais com meta cadastrada, usa-se a meta canonica + vendido do ERP quando
  houver (senao 0). Consultor sem meta nenhuma fica fora do map (anel neutro no front).

### `GoalProgressProvider` (ponte de VENDIDO com o CRM/ERP)

Entrega o atingimento/vendido CANONICO por consultor (vindo do `goalProgress`/
`salesCents` do `/v1/erp/crm`) para o modulo cruzar com a meta no snapshot/overview.

- assinatura: `GoalStatsByConsultant(ctx, tenantID, month) (map[string]GoalStats, error)`
- chave do map: o `consultant.ID` de PERFIL (mesmo `roster.id` do snapshot / `person.id` do front); o adapter pula consultores sem ID de perfil (`ProfileConsultantID` vazio)
- `month` no formato `"YYYY-MM"`; vazio => mes corrente (UTC)
- `GoalStats` (em reais): `monthlyGoal`, `soldValue`, `remainingToGoal`, `progress` (%, pode passar de 100), `hasGoal`
- injecao via `SetGoalProgressProvider` (opcional). Sem provider, ou em erro/nil, o snapshot degrada com `goalStats=nil` (log em WARN `operations_goal_stats_unavailable`). E enriquecimento: erro NUNCA propaga para o snapshot/overview
- o adapter concreto vive na composition root (`back/internal/platform/app/operations_goal_progress_adapter.go`) para que `operations` NAO importe `crm/erp` (sem ciclo). Ele chama o service do erp server-side (sem o gate `canViewERP`), com um principal `platform_admin` escopado ao tenant pedido — decisao de produto "todos os operadores veem a meta de todos"
- escopo tenant-wide: o adapter usa o mesmo caminho da pagina de consultores (`resolveERPScope` + `GetCRMOverview`), com janela do mes (primeiro ao ultimo dia, UTC) IGUAL ao default de `consultant-transforms.ts`, para o numero bater exatamente
- CACHE no adapter por `(tenantID, month)` com TTL de 120s (mutex + map): o snapshot e hot path realtime e nao pode rodar o overview inteiro do CRM a cada chamada

### Caches do hot path de leitura (snapshot/overview) — `service_goal_stats.go`

Snapshot e overview sao hot path: o front refaz a leitura apos CADA mutacao (start/finish/pause...) e a cada evento de realtime, entao todo clique pagava o custo das queries de enriquecimento de meta. Para nao bater no banco a cada chamada, alem do cache do adapter do ERP (acima), `operations` mantem dois caches em memoria no proprio service (`sync.Mutex` + `map`):

- **meta canonica por consultor** (`effectiveGoalsByConsultant` -> `Repository.EffectiveMonthlyGoalByConsultant`): TTL **60s**. Chave = lista de `storeIDs` (trim + dedup + ORDENADOS, juntados por `,`) + `"|"` + mes `"YYYY-MM"`. O mes na chave garante que a virada de mes nunca devolve dado stale do mes anterior (e da determinismo aos testes sem precisar de relogio injetado). A meta muda raramente; 60s reflete edicao de meta sem o operador esperar.
- **store -> tenant_id** (`storeTenantID` -> `Repository.GetStoreTenantID`): TTL **300s**, chave = `storeID`. Usado SO no fallback de escopo do ERP quando o principal nao traz account/tenant (ex.: `platform_admin` em rota `RequireAuth` sem `X-Account-Id`). store -> tenant e praticamente imutavel, por isso o TTL e generoso.

Em ambos: ERRO NUNCA e cacheado (mantem a degradacao graciosa — na proxima chamada tenta de novo e o resultado segue `nil`/`""`, snapshot com `goalStats=nil`). Nao altera os VALORES de `GoalStats` nem a logica de `combineGoalStats`; so evita o trabalho de banco repetido. `goalStatsForTenant`, `effectiveGoalsByConsultant` e `combineGoalStats` vivem em `service_goal_stats.go`; os gates de permissao (`canReadOperations`/`canMutateOperations` e os papeis) em `service_access.go`.

## Contrato atual

- `GET /v1/operations/snapshot?storeId=...`
- `GET /v1/operations/overview`
- `POST /v1/operations/queue`
- `POST /v1/operations/pause`
- `POST /v1/operations/resume`
- `POST /v1/operations/assign-task`
- `POST /v1/operations/start`
- `POST /v1/operations/finish`
- `POST /v1/operations/keep-open` (auto-encerramento 2h — "Continuar", operador)
- `POST /v1/operations/validate` (auto-encerramento 2h — validar pendencia, gerente)
- `POST /v1/operations/cancel-metric` (auto-encerramento 2h — cancelar metrica, gerente)

## Auto-encerramento de atendimento (2h)

Plano canonico: `docs/operacao/AUTO_ENCERRAMENTO_PLAN.md`; fronteira com `alerts` em
`docs/OPERATIONS_ALERTS_TIMER_FLOW.md` (§Auto-encerramento).

Encerra automaticamente atendimentos esquecidos (limite CONFIGURAVEL por tenant, default
2h), preservando a metrica de tempo e mandando o auto-fechado para uma caixa de Pendencias
onde a gestao valida (desfecho real) ou cancela (fora da metrica, mantido p/ auditoria).
SERVIDOR AUTORITATIVO: o sweep decide e fecha mesmo com a aba fechada; a barra de 1 min no
front e' so display.

- **Sweep**: `processAutoClose`/`autoCloseService`/`persistGraceState` em
  `service_autoclose.go`, dentro do `ProcessTimedAlerts` (mesmo tick de 3s do
  `long_open_service`). Le `AutoCloseEnabled/AutoCloseMinutes/AutoCloseGraceSeconds/
  SnoozeRepromptMinutes` via `AlertCoordinator.LoadOperationalRules` (config em
  `tenant_operational_alert_rules`). Um auto-close por tick (respeita o append de sessao
  unica do `persistAndAck`; o grace-only e o auto-close-not-last zeram as sessoes p/ nao
  reinserir a ultima).
- **Estado corrente** (`operation_active_services`, migration 0196): `grace_deadline`
  (epoch ms absoluto do vencimento do countdown; 0 = sem), `snoozed_until`, `snooze_count`.
  ROUND-TRIP obrigatorio em `loadActiveServices` (Scan), `replaceActiveServices` (INSERT)
  E `normalizeSnapshotState` (senao zeram a cada leitura).
- **Historico** (`operation_service_history`, migration 0196): `close_reason`
  ('manual'|'auto'), `validation_status` ('pending'|'validated'|'cancelled'),
  `validated_by`/`validated_at` (sem FK p/ core.*), `snooze_count`, `cancel_reason`.
  `finish_outcome` admite o sentinela 'auto' (`normalizeOutcome` preserva). `appendHistory`
  grava close_reason/validation_status/snooze_count.
- **Fechamento**: `autoCloseService` grava a linha PENDENTE (duration = fechamento−inicio,
  NAO capado no limite p/ contar snooze real), devolve o consultor a fila (decisao de
  produto) e resolve o alerta (`long_open_service.resolved`).
- **Encerrar pendencia (validate)**: UPDATE no historico (`store_postgres_autoclose.go`),
  nunca re-INSERT (appendHistory e `on conflict do nothing`). Gate `canValidateAutoClose`
  (gerente/owner/platform_admin). ErrPendingNotFound → 404. O `/validate` recebe o
  payload COMPLETO do modal de encerramento (mesmo shape do `/finish`) + campo
  `validationReason` OBRIGATORIO (migration 0197, coluna `validation_reason`):
  justificativa de por que o consultor nao encerrou na hora — junto de `validated_by`
  (quem)/`validated_at` (quando)/`close_reason='auto'`/`snooze_count`, e a base das
  metricas de cobranca por consultor/gerente/loja. UX: botao "Pendencias para
  encerrar (N)" no topo da Lista da vez → modal de lista → "Encerrar" abre o MESMO
  finish modal do fluxo normal (controller resolve o service-like da pendencia; o
  submit vai para `/validate`). O `/cancel-metric` continua exposto no back (sem UI
  proeminente; cancelamento de metrica fica para a fase de metricas).
- **Snapshot**: `ActiveService` ganha `graceDeadline`/`snoozedUntil`/`snoozeCount`; o
  Snapshot ganha `pendingValidations[]` (derivado do historico pending).
- **Metrica**: `reports` (via `appendHistoryFilters`) e `analytics` (via
  `excludeCancelledMetrics`) IGNORAM `validation_status='cancelled'`; pendentes/validadas
  contam.

Regra de resposta:

- `GET /v1/operations/snapshot` devolve o snapshot operacional completo da loja, incluindo `roster` (projecao ENXUTA dos consultores da loja: `id`, `storeId`, `name`, `role`, `initials`, `color`). O `roster` existe para a faixa de consultores funcionar para papeis operadores que NAO tem a permissao de gestao `/v1/consultants` (ex.: `consultant`). NUNCA inclua meta/comissao/e-mail no `roster` do snapshot — esses ficam so no endpoint de gestao
- `waitingList[].goalStats` e `activeServices[].goalStats` (opcionais, `null` quando nao ha dado ERP do consultor): o atingimento de meta CANONICO do consultor vindo do CRM/ERP (`goalProgress` do `/v1/erp/crm`), embutido server-side via `GoalProgressProvider` para que TODO operador veja o anel de meta no card da Lista da vez, sem precisar da permissao de gestao do ERP. Shape: `{ monthlyGoal, soldValue, remainingToGoal, progress, hasGoal }` em reais. No overview, `goalStats` aparece em cada `OperationOverviewPerson` (waitingList/activeServices/pausedEmployees/availableConsultants), preenchido por uma unica busca tenant-wide. Payout NAO entra neste corte
- `GET /v1/operations/overview` devolve a visao operacional integrada das lojas acessiveis da sessao autenticada
- `snapshot` e `overview` carregam `serverTime` (relogio do servidor na resposta). O front re-sincroniza o `serverClockOffsetMs` a CADA leitura ao vivo com esse campo — nao so no `savedAt` do ack de mutacao. Sem isso, a sessao que apenas OBSERVA (nao muta) nunca recaptura o offset e o cronometro drifta pelo skew API/navegador (ver nota de skew abaixo) ate o reload
- comandos `POST` devolvem apenas `ack` minimo (`ok`, `storeId`, `savedAt`, `action`, `personId`)
- o frontend deve revalidar o snapshot por `GET /v1/operations/snapshot` apos mutacao bem-sucedida
- no modo integrado, o frontend deve revalidar `GET /v1/operations/overview` apos mutacao bem-sucedida
- `POST /v1/operations/finish` deve receber apenas os campos aplicaveis ao desfecho atual; por exemplo, `lossReasons*` so sobem em `nao-compra`
- campos opcionais/default sem valor de negocio nao devem subir como string vazia, array vazio ou objeto vazio

## Carregamento do snapshot e janela de historico (`store_postgres_history.go`)

- `LoadSnapshot` e `LoadSnapshotWithHistorySince` vivem em
  `store_postgres_history.go` (movidos de `store_postgres.go`, junto de
  `loadServiceHistory` e do helper puro `buildServiceHistoryQuery`).
- `LoadSnapshot(ctx, storeID)` mantem o contrato atual: historico COMPLETO, SEM
  janela (delega para `LoadSnapshotWithHistorySince(ctx, storeID, 0)`). E' o que a
  operacao ao vivo usa (`service.go` -> `GET /v1/operations/snapshot`), entao esse
  payload permanece byte-identico.
- `LoadSnapshotWithHistorySince(ctx, storeID, historySinceMillis)` janela o
  historico por `finished_at >= historySinceMillis` (0 = sem janela). Consumido
  pelo `analytics`, que so agrega — carregar todo o historico da loja a cada
  request era o gargalo.
- `buildServiceHistoryQuery(storeID, sinceMillis)` e' pura (sem banco) para ser
  testavel: adiciona `and finished_at >= $2` so quando `sinceMillis > 0` e mantem
  `order by started_at asc, created_at asc`.
- A interface `Repository` (model.go) NAO muda: continua declarando so
  `LoadSnapshot`. `LoadSnapshotWithHistorySince` e' metodo concreto do
  `PostgresRepository`, consumido pela interface propria do `analytics`.
- Follow-up (fora deste corte): janelar o `serviceHistory` que o snapshot ao vivo
  envia ao navegador (`snapshot.go`) e a janela em `loadSessions`.

## Regras de escopo

- leitura: `consultant`, `store_terminal`, `manager`, `marketing`, `director`, `owner` e `platform_admin`
- comando (mutacao): `consultant`, `store_terminal`, `manager`, `owner` e `platform_admin` — `marketing` e `director` sao read-only (acompanham, nao operam)
- quando as permissoes ja vierem resolvidas: `workspace.operacao.view` libera LEITURA; `workspace.operacao.edit` libera COMANDOS. View sozinho NAO muta (least-privilege)
- leitura integrada multi-loja: qualquer sessao com mais de uma loja acessivel
- sempre validar `store_id` contra o principal autenticado

## Regra de persistencia

- o estado corrente vive em tabelas correntes por loja:
  - `operation_queue_entries`
  - `operation_active_services`
  - `operation_paused_consultants`
  - `operation_current_status`
- `operation_paused_consultants.kind` diferencia pausa comum de deslocamento operacional:
  - `pause`
  - `assignment`
- auditoria vive em tabelas append-only:
  - `operation_status_sessions`
  - `operation_service_history`
- `operation_status_sessions` agora tem `reason` e `kind` (nullable, migration 0159): quando a sessao fechada e de pausa (`status='paused'`), `applyStatusTransitions` anexa o motivo e o tipo (`pause`/`assignment`) capturados no `Resume` ANTES do `filterPaused` (o `operation_paused_consultants` e apagado no resume, entao a metrica precisa do dado na sessao). Isso alimenta o relatorio de pausas (`GET /v1/reports/pauses`)
- o snapshot enviado ao Nuxt deve manter compatibilidade com o shape atual do runtime, para reduzir retrabalho no frontend
- comandos nao devem devolver o snapshot inteiro da loja; isso aumenta payload, confunde debug e mistura leitura com mutacao
- o modulo ja esta integrado ao Nuxt via `web/app/stores/operations.ts` e `web/app/utils/runtime-remote.ts`
- a busca dinamica de produtos do modal deve consumir o modulo `catalog`; `operations` nao deve conhecer tabela ERP nem catalogo manual de settings
- a source `erp_current` do `catalog` esta tenant-scoped neste momento, porque os dados importados do ERP ainda vivem apenas na loja `184`; a Operacao continua enviando `storeId` para controle de acesso, nao para escolher tabela/coluna
- no modo novo de fechamento (`erp-reconciliation`), `operations` deve apenas persistir `purchaseCode` como referencia de conciliacao para compras; nao tentar buscar ERP em tempo real dentro do fechamento

## Alertas recentes do fluxo

- o nome legado `atendimento paralelo` continua aparecendo em partes do codigo, mas a regra operacional esperada e permitir mais de um atendimento em aberto para o mesmo consultor, mantendo encerramento individual posterior por `serviceId`
- cada atendimento em aberto precisa manter cronometro, historico e fechamento proprios; o consultor so volta para a fila ao encerrar o ultimo atendimento ativo dele
- `POST /v1/operations/finish` e identificado por `serviceId`; qualquer cache ou draft do frontend precisa ser invalidado se esse `serviceId` nao existir mais no snapshot atual
- incidente recente: o erro de encerramento reportado passou a aparecer quando o modal foi reaberto com draft restaurado; o frontend agora invalida rascunho stale por `storeId + serviceId + serviceStartedAt` antes de reaproveitar ou submeter esse payload
- atendimentos abertos `na sequencia` precisam herdar do primeiro atendimento do grupo `queueJoinedAt`, `queueWaitMs`, `queuePositionAtStart` e `skippedPeople`; perder esses metadados no backend volta a quebrar o insert em `operation_service_history` ou distorce o historico
- a duracao efetiva de um atendimento em aberto na sequencia nao vai ate o momento do fechamento manual quando ja existe um proximo atendimento do mesmo grupo; ela deve ser truncada no `startedAt` do proximo `serviceId` do grupo
- no modo integrado, mutacoes e fechamento precisam usar a `storeId` do proprio servico; depender apenas de `activeStoreId` reintroduz erro silencioso em `Todas as lojas`
- houve incidente de ambiente com a API cerca de 4,5s a frente do navegador/host; enquanto esse skew existir, os timestamps persistidos no backend continuam corretos para auditoria, mas a UI precisa compensar `savedAt -> Date.now()` para o cronometro nao parecer atrasado ao iniciar atendimento
- `POST /v1/operations/finish` com `action=cancel` reinsere o consultor na fila usando o `QueueJoinedAt` original como chave de ordenacao: percorre a fila atual e insere antes da primeira pessoa cujo `QueueJoinedAt` seja maior; isso preserva a ordem relativa corretamente mesmo quando a fila encolheu (ex.: era o 2o, o 1o foi para atendimento, o 10o ficou na fila — volta como 1o porque entrou antes do 10o)
- `POST /v1/operations/finish` com `action=stop` nao exige `stopReason`; o campo e opcional e gravado se vier preenchido; sem justificativa obrigatoria nos dois modais (cancel e stop)
- `operations` continua como fonte de verdade dos alertas temporais operacionais: ele le os thresholds configurados em `alerts`, roda a reavaliacao temporal no backend e emite sinais leves para `alerts` abrir, atualizar ou resolver a instancia correspondente
- `alerts` nao deve ter timer proprio para `long_open_service`; quando o tempo vence, e `operations` quem dispara o sinal `long_open_service.triggered`, e quando o atendimento termina, cancela ou para, e `operations` quem dispara `long_open_service.resolved`
- atendimento com `StoppedAt` preenchido continua visivel no snapshot como atendimento parado, mas sai do cronometro de `long_open_service`; parar ou cancelar atendimento deve resolver qualquer alerta operacional aberto daquele `serviceId`
- `readJSONLenient` agora carrega o body antes de decodificar e devolve preview do payload + `Content-Type` no campo `details.cause` quando o JSON falha; manter esse comportamento facilita debug de payload mismatch via toast no frontend

## Estado atual

Hoje este modulo ja sustenta:

- fila por loja em PostgreSQL
- atendimentos ativos
- pausas e retomadas
- designacao de tarefa/reuniao com retirada controlada da fila
- historico de atendimento
- sessoes de status
- hidratacao do frontend no login/troca de loja
- visao integrada da operacao para sessoes com mais de uma loja acessivel
- cards operacionais com identificacao visual da loja de origem

## Regra de acoplamento

- qualquer dependencia com `auth`, `stores` ou outro modulo host deve entrar por adapter pequeno na borda
- a regra de negocio do service deve falar a linguagem do proprio modulo
- este modulo ja usa `AccessContext` no service; nao voltar a passar `auth.Principal` direto para a regra de negocio

Proximo passo natural:

- filtros administrativos mais ricos sobre historico operacional e ultimos atendimentos
- notificacao operacional estruturada para tarefa/reuniao
- refinamentos de auditoria cross-store

## Registro de implementacao (Fase 5-6: Novos signal types e builders)

### Novos signal types em alerts.go

Adicionados 8 constantes para 4 novos tipos de trigger:

- `SignalLongQueueWaitTriggered`: fila > threshold
- `SignalLongQueueWaitResolved`: fila normalizada
- `SignalLongPauseTriggered`: pausa > threshold
- `SignalLongPauseResolved`: pausa retomada
- `SignalIdleStoreTriggered`: loja sem atividade em horário comercial
- `SignalIdleStoreResolved`: atividade retomada
- `SignalOutsideBusinessHoursTriggered`: atendimento fora do horário
- `SignalOutsideBusinessHoursResolved`: atendimento encerrado ou em horário normal

### Extensão de OperationalAlertSignal

Novos campos adicionados ao struct `OperationalAlertSignal`:

- `ConsultantName string`: nome do consultor (denormalizando para o alerta)
- `ElapsedMinutes int`: minutos decorridos (para template `{elapsed}`)
- `TriggerType string`: qual trigger gerou o sinal (para identificar a regra a carregar)

### Builders em operations/service.go (stubs para MVP)

```go
func (s *Service) buildLongQueueWaitSignals(ctx, storeId, snapshot, rules, now) []OperationalAlertSignal {
    // Para cada item em snapshot.WaitingList:
    // if now - QueueJoinedAt > rule.threshold → emit signal
    // Stub retorna []
}

func (s *Service) buildLongPauseSignals(ctx, storeId, snapshot, rules, now) []OperationalAlertSignal {
    // Para cada item em snapshot.PausedEmployees:
    // if now - StartedAt > rule.threshold → emit signal
    // Stub retorna []
}

func (s *Service) buildIdleStoreSignals(ctx, storeId, snapshot, rules, now) []OperationalAlertSignal {
    // if (WaitingList.empty && ActiveServices.empty) && !finalizadoRecentemente → emit
    // ConsultantID vazio (alerta geral da loja)
    // Stub retorna []
}

func (s *Service) buildOutsideBusinessHoursSignals(ctx, storeId, snapshot, rules, now) []OperationalAlertSignal {
    // Para cada ActiveService:
    // if started_at fora do horário comercial da loja → emit
    // Requer integração com store hours de `stores` module
    // Stub retorna []
}
```

### OperationsScanner interface (para retroatividade)

Implementação em operations/service.go:

```go
func (s *Service) ScanForRule(ctx context.Context, rule alerts.RuleDefinition) (interface{}, error) {
    // Carrega snapshot atual de todas as lojas do tenant
    // Roda o builder específico para rule.TriggerType
    // Retorna signals que casam com a regra
    // Retorna interface{} para evitar import cycle
}
```

Injeção em app.go:
```go
alertsService.SetOperationsScanner(operationsService)
```

### Comportamento esperado

1. **Materializacao:** Quando um sinal chega ao `alerts`, o módulo carrega a regra ativa do tipo de trigger correspondente e cria a instância com snapshot da regra (cores, templates, interação).

2. **Retroatividade:** Quando o usuário salva uma nova regra via API `POST /v1/alerts/rules`, o endpoint chamado `POST /v1/alerts/rules/{id}/apply-now` faz:
   - Carrega a regra
   - Chama `operationsScanner.ScanForRule(ctx, rule)`
   - Processa sinais retornados
   - Cria alertas para atendimentos/contextos já em andamento que se enquadram na regra

3. **Scheduler (15s):** O `ProcessTimedAlerts` em `app.go` continua rodando, carregando as regras ativas de todos os triggers e materializando alertas para todos os contextos vivos que as acionam.

### Regra crítica: Evitar import cycle

Para que `operations` não dependa de `alerts` (e vice-versa), a interface `OperationsScanner` tem método que retorna `interface{}`, e `alerts` faz type assertion após receber:

```go
// Em alerts/service.go
signals := s.operationsScanner.ScanForRule(ctx, *rule)
typedSignals, ok := signals.([]alerts.OperationalSignalInput)
if !ok {
    return 0, errors.New("scanner retornou tipo inesperado")
}
// processa typedSignals...
```
