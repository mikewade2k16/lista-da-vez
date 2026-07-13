# Plano — Auto-encerramento de atendimento (2h) na Fila/Operação

> Documento canônico desta entrega. Espelhado em `roadmap-data.ts` (grupo
> `fila-operacao`, fase `operacao-auto-encerramento`) e nos `AGENT.md` dos módulos
> tocados (`queue/operations`, `queue/alerts`). Contrato de fronteira operations↔alerts
> em `docs/OPERATIONS_ALERTS_TIMER_FLOW.md` (§Auto-encerramento).
> Status: **CÓDIGO LOCAL CONCLUÍDO (2026-07-09)** — 7 fases implementadas inline pelo
> orquestrador. Migration 0196 aplicada no dev (colunas confirmadas nas views public.*);
> api/web rebuildados. Gates locais OK: go build/vet/gofmt + go test (operations/alerts/
> reports/analytics) + eslint 0 erros. **FALTA (dono):** validar no browser (ligar a config
> com limites baixos) + aplicar a migration 0196 na VPS no deploy.
> Ajustes de escopo vs. plano original: (1) o "grace" é gravado como `grace_deadline`
> ABSOLUTO (não `grace_started_at`), evitando carregar config no hot path do snapshot; (2)
> o gate de gerente é por PAPEL (permissão dinâmica `queue.operations.validate` fica como
> refino futuro).
> **Redesenho da UX de encerramento (2026-07-10, decisão do dono):** a caixa lateral e o
> diálogo dedicado foram REMOVIDOS. O fluxo agora é: botão "Pendencias para encerrar (N)"
> (badge de contagem) no topo da Lista da vez, acima do "Atender primeiro da fila" → modal
> de LISTA → "Encerrar" abre o MESMO modal de encerramento do fluxo normal. O submit vai
> para `POST /v1/operations/validate` com o payload completo do modal + `validationReason`
> OBRIGATÓRIA (migration 0197): por que o consultor não encerrou na hora. Fica registrado
> quem encerrou (`validated_by`), quando (`validated_at`), que foi auto-encerrado
> (`close_reason='auto'`) e quantos adiamentos houve (`snooze_count`) — base das métricas
> futuras de cobrança (quantos/quais/porquês por consultor/gerente/loja). O
> `/cancel-metric` permanece no back sem UI (cancelamento de métrica fica para a fase de
> métricas).
> Criado em 2026-07-09.

## Contexto (por que)

Hoje um atendimento aberto na Operação conta tempo indefinidamente. Quando o operador
esquece de encerrar (foi embora, fechou a aba, PC desligado), o cronômetro segue correndo
e **contamina a métrica de tempo de atendimento** — não dá para distinguir um atendimento
real de 2h de um esquecido de 2h.

O módulo já tem o alerta `long_open_service` (avisa quando passa do limite), mas ele só
**avisa**, não **encerra**, e depende de alguém olhando a tela — exatamente o que falta
num atendimento esquecido.

Objetivo do dono: **encerrar automaticamente** atendimentos a partir de um limite (2h),
dando ao operador a chance de continuar (para não fechar um atendimento legítimo), e mandar
o que foi auto-encerrado para uma **fila de pendências** onde o gerente **valida** (era
real) ou **cancela** (foi erro/esquecido). Assim a métrica de tempo é preservada e os erros
reais ficam identificados.

## Decisões de produto (confirmadas com o dono)

1. **Limites configuráveis por tenant** (não fixos): 2h de atendimento / 1min de barra /
   30min de re-pergunta viram config editável no painel, com flag liga/desliga.
2. **Consultor volta para a fila na hora** do auto-encerramento (a pendência é sobre a
   métrica, não trava o consultor).
3. **Sai do board → caixa "Pendências" vermelha**: o card sai de "Em atendimento" e
   aparece VERMELHO só na caixa lateral. Sem card vermelho persistente no board.
4. **Cancelar = manter a linha para auditoria + motivo obrigatório**
   (`validation_status='cancelled'`, fora da métrica, preservada; reusa a coluna
   `cancel_reason` existente).

## Arquitetura central — SERVIDOR AUTORITATIVO

O auto-encerramento é **decidido e executado no backend**; a barra de 1 min no front é só
aviso de UI. Um atendimento esquecido é o caso em que ninguém está olhando — se o
fechamento dependesse de `setInterval` no Vue, nunca fecharia no cenário-alvo. Isso respeita
a regra canônica de `OPERATIONS_ALERTS_TIMER_FLOW.md` ("operations é dono do cronômetro; o
front exibe e reage à invalidação do snapshot") e o timer server-anchored (`elapsed` =
âncora de servidor, nunca `Date.now` de parede).

- O **sweep de 3s** (`ProcessTimedAlerts`, já existente) detecta `elapsed >= autoClose` e
  grava `grace_started_at`; ao detectar `now >= graceDeadline` sem snooze, executa o
  fechamento autoritativo.
- O front desenha a barra encolhendo de `graceDeadline − adjustedNow` (ambos relógio de
  servidor). É pura animação; nunca chama API para fechar. O estado real chega pelo WS
  (`operation.updated` → refetch do snapshot).
- **"Continuar atendimento"** é uma mutação real (POST) que grava `snoozed_until = now +
  30min` no banco — durável, sobrevive a restart da API.

## Reúso vs. novo

**Reusar:** o scheduler de 3s (`app.go`, `operationsAlertMonitorInterval`); a varredura
`ProcessTimedAlerts`/`buildLongOpenSignals` (`operations/service_alerts.go`); o gate
`shouldMonitorLongOpenAlert`; a config por tenant `LoadOperationalRules` (`alerts/service.go`
→ `alerts/store_postgres_signals.go`, tabela `tenant_operational_alert_rules`); o caminho
único de escrita `persistAndAck` (`operations/service.go`) — o sweep DEVE passar por ele
(senão não há WS); o primitivo "congela sem histórico" do `action=stop`
(`service_finish.go`); a resolução do alerta `long_open_service` via `emitAlertSignals`.

**Novo (decisão):** o "continuar + re-perguntar a cada 30min" é mecanismo **novo em
operations, ancorado em coluna de DB** — NÃO reusa o `still_happening` do alerts (que marca
`acknowledged` permanente, nunca reabre, e cujo dedup in-memory reseta no restart).
"Continuar" precisa impactar o timer de fechamento (conceito de operations, dono do timer).

## Modelo de dados — migration `0196_operation_auto_close.sql`

SQL plano idempotente, sem goose Down (`add column if not exists`, `drop constraint if
exists` antes de re-add, `create index if not exists`). Padrão a copiar: `0031` (add bigint)
+ `0113` (recriar view). Próximo número livre = 0196 (última = 0195).

**`queue.operation_active_services`** (timer/grace/snooze — bigint epoch ms / int, default 0):
```
grace_started_at bigint  not null default 0
snoozed_until    bigint  not null default 0
snooze_count     integer not null default 0
```

**`queue.operation_service_history`** (métrica + validação):
```
close_reason      text   not null default 'manual'    -- 'manual' | 'auto'
validation_status text   not null default 'validated' -- 'pending' | 'validated' | 'cancelled'
validated_by      uuid                                 -- FK public.users(id) on delete set null
validated_at      bigint not null default 0
-- cancel_reason JÁ EXISTE (reusar para o motivo obrigatório do cancelamento)
-- CHECKs idempotentes (drop if exists → add) para close_reason e validation_status
-- índice parcial: (store_id, finished_at desc) where validation_status = 'pending'
```

**`tenant_operational_alert_rules`** (config por tenant — decisão #1):
```
auto_close_enabled       boolean not null default false
auto_close_minutes       integer not null default 120
auto_close_grace_seconds integer not null default 60
snooze_reprompt_minutes  integer not null default 30
```

**Relaxar o CHECK `finish_outcome`**: adicionar `'auto'` a
`('reserva','compra','nao-compra','auto')`. A linha auto-fechada nasce
`finish_outcome='auto'`, `close_reason='auto'`, `validation_status='pending'`; ao validar, o
gerente substitui pelo outcome real (UPDATE).

**Recriar as views (OBRIGATÓRIO — trap do 0105/0113: `select *` congela colunas):**
```
create or replace view public.operation_active_services as select * from queue.operation_active_services;
create or replace view public.operation_service_history as select * from queue.operation_service_history;
```

**Métrica de tempo:** `duration_ms = effectiveFinishedAt − ServiceStartedAt`, com
`effectiveFinishedAt = o instante do fechamento` (expiração do grace). **NÃO fixar em
start+2h** — isso subcontaria um atendimento legitimamente continuado via snooze até 3h+ (o
operador clicou "Continuar", o tempo é real). O +1min de grace é desprezível e honesto.

**Idempotência:** `appendHistory` usa `on conflict (store_id,service_id) do nothing` — a
linha pendente já existe; validar/cancelar é **UPDATE**, nunca re-INSERT.

## Fluxo de estados

```
ABERTO ──sweep: elapsed>=autoClose & snoozed_until<=now──▶ GRACE (barra 1min)
GRACE ──operador "Continuar" (POST keep-open)──▶ SNOOZE (snoozed_until=now+30min, count++)
GRACE ──sweep: now>=graceDeadline, sem keep-open──▶ AUTO_CLOSE
SNOOZE ──sweep: now>=snoozed_until──▶ GRACE (re-pergunta)  [loop enquanto continuar clicando]
AUTO_CLOSE: remove de active_services · consultor volta à fila (decisão #2) ·
            INSERT history (close_reason=auto, validation_status=pending, finish_outcome=auto,
            duration=fechamento−start) · persistAndAck (WS + resolve alerta long_open_service)
PENDENTE (history pending, vermelho na caixa, timer parado)
   ──gerente "Validar" (POST validate + finish modal)──▶ VALIDADO (finish_outcome real, status=validated)
   ──gerente "Cancelar" (POST cancel-metric + motivo)──▶ CANCELADO (status=cancelled, fora da métrica, mantida)
```

Quem dispara: ABERTO→GRACE, GRACE→AUTO_CLOSE, SNOOZE→GRACE = **scheduler**; GRACE→SNOOZE =
**operador**; PENDENTE→VALIDADO/CANCELADO = **gerente**.

## Contrato de API

**Campos novos no snapshot** (`ActiveService`, `operations/model.go`, emitidos por
`snapshot.go`): `graceDeadline` (epoch ms; >0 = barra ativa), `snoozedUntil`, `snoozeCount`.
**Novo array** `pendingValidations[]` (derivado do history `validation_status='pending'`):
`{ serviceId, personName, startedAt, finishedAt, durationMs, autoClosedAt, storeId,
snoozeCount }`.

**Endpoints novos:**
- `POST /v1/operations/keep-open` — "Continuar". Body `{ storeId, serviceId }`. Efeito:
  `snoozed_until=now+30min`, `grace_started_at=0`, `snooze_count++`. **Gating: operador**.
- `POST /v1/operations/validate` — reusa o finish modal; gerente escolhe o outcome real;
  UPDATE do history (`finish_outcome=real`, `validation_status=validated`, `validated_by`,
  `validated_at`). Body = `FinishCommandInput` + `serviceId` da pendência. **Gating: gerente**.
- `POST /v1/operations/cancel-metric` — Body `{ storeId, serviceId, reason }` (**reason
  obrigatório**, decisão #4). UPDATE `validation_status=cancelled`, `cancel_reason`,
  `validated_by`, `validated_at`. **Gating: gerente**.

**Permissão nova** `queue.operations.validate` (scope store/account): concedida a
supervisor/manager/owner, NÃO ao consultor. Front gateia com `isPlatformAdmin ||
has('queue.operations.validate')`.

**Sweep NÃO chama o `Finish` HTTP** (exige `AccessContext`). Extrair o "close-core" de
`Finish` num helper interno `autoCloseService` chamável sem HTTP (novo arquivo
`service_autoclose.go`). Const nova `actionAutoClose = "auto_close"`.

## Frontend

- **Barra de countdown + botão Continuar → no card** (`OperationActiveServiceCard.vue`).
  Reusa `.service-card__action-progress` (width%). `remaining = graceDeadline − props.now`
  (`props.now` = `adjustedNow` server-anchored; **nunca `Date.now`**). Card já tem 355 linhas
  → extrair `OperationServiceCountdownBar.vue`.
- **Caixa "Pendências" vermelha** → nova `<section>` no topo de `OperationSidePanel.vue`.
  `durationMs` FIXO do history (sem cronômetro). Ações: "Validar" → finish modal →
  `/validate`; "Cancelar" → dialog de motivo → `/cancel-metric`.
- **Threading dos campos**: `runtime-remote-normalize.ts` `normalizeOperationSnapshot` +
  `state.ts` `normalizeActiveServicesList` para `graceDeadline/snoozedUntil/snoozeCount`;
  `pendingValidations` como array store-scoped.
- **"Validar" acha a pendência:** `useFinishModalController.js` hoje busca em
  `activeServices.find(serviceId)`; precisa cair também em `pendingValidations`. Helper à
  parte (não inflar o arquivo de 2118 linhas).
- **Config (decisão #1):** bloco "Auto-encerramento" na workspace de Alertas
  (`AlertsWorkspace.vue`), ligado ao GET/PUT das operational-rules. Gate `queue.alerts.manage`.
- **Métrica:** analytics/reports que leem `operation_service_history` filtram
  `validation_status <> 'cancelled'`. Tratar `finish_outcome='auto'` nos breakdowns por desfecho.

## Sequência de implementação (7 fases)

- **Fase 0** — Doc-first: este doc + roadmap (pending) + seção no OPERATIONS_ALERTS_TIMER_FLOW.
- **Fase 1** — Migration 0196 + schema round-trip + config em tenant_operational_alert_rules.
- **Fase 2** — Close-core refatorado + helper `autoCloseService` (sem AccessContext).
- **Fase 3** — Sweep de decisão (`service_autoclose.go`) no ProcessTimedAlerts.
- **Fase 4** — Snapshot fields + endpoints keep-open/validate/cancel-metric + permissão.
- **Fase 5** (front) — barra countdown + botão Continuar (card).
- **Fase 6** (front) — caixa Pendências + validar/cancelar + config UI.
- **Fase 7** — Sincronizar 3 docs + panorama + Notas de Deploy.

Back 1→2→3→4 sequencial; front 5,6 começam com o contrato do snapshot da Fase 4 fechado.

## Riscos e armadilhas

1. Timer server-anchored: `graceDeadline`/`snoozedUntil` = epoch ms de servidor, comparado
   só com `adjustedNow`. Nunca `Date.now`.
2. View trap (0105/0113): recriar `public.*` após ALTER.
3. Migration idempotente sem goose Down.
4. `replaceActiveServices` é delete+reinsert total por loja: round-trip de
   `grace_started_at/snoozed_until/snooze_count` no Scan E no INSERT.
5. Multi-tenant: tabelas sem `tenant_id`; escopo por `store_id → queue.stores.tenant_id`.
6. Dedup in-memory reseta no restart: idempotência vem do estado no DB.
7. Validar/cancelar é UPDATE, nunca re-INSERT.
8. Modal e card espelhados.
9. Limite 450 linhas: lógica nova em `service_autoclose.go` + `OperationServiceCountdownBar.vue`.
10. `platform_admin` no front: todo gate precisa `isPlatformAdmin || has(...)`.

## Verificação end-to-end

1. Config: ligar auto-encerramento e setar limites baixos (ex.: 2min / 20s / 1min) para teste.
2. Abrir atendimento; após o limite, a barra de contagem aparece no card.
3. "Continuar" → barra some; após o snooze, reaparece (re-pergunta).
4. Deixar expirar → serviço some do board, consultor volta à fila, aparece vermelho na caixa
   com tempo congelado; alerta `long_open_service` resolve.
5. Fechar a aba antes do fim: o serviço ainda auto-fecha (prova server-authoritative).
6. Gerente: "Validar" → finish modal → some das pendências, history validated com outcome
   real. "Cancelar" sem motivo → bloqueado; com motivo → some da pendência, fica na auditoria,
   sai da métrica.
7. Analytics/reports: cancelado não conta; validado/pendente conta o tempo.

## Notas de Deploy

- Migrations `0196_operation_auto_close.sql` E `0197_operation_validation_reason.sql`
  rodam ANTES do rebuild da api.
- Mudança em `back/` → `docker compose up -d --build api`.
- Nova permissão `queue.operations.validate` (backfill nos role templates supervisor/owner/
  manager via a própria migration ou seed de RBAC).
- Sem novas env vars, sem mudança de porta.
