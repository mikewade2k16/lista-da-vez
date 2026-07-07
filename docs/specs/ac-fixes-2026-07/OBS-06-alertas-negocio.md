# OBS-06 — Alertas de negócio, Fase 1: `long_queue_wait` + `long_pause`

> Spec de implementação · Prioridade **P2** · Esforço **M** · Impacto **médio**
> Origem: AC-16 (evolução) · roadmap `observabilidade-n8n` → task `obs-06-alertas-de-negocio`

## 1. Contexto

**O achado:** o sistema de alertas operacionais só processa UM sinal (`long_open_service` —
atendimento aberto demais). Os sinais `long_queue_wait` (cliente esperando demais na fila),
`long_pause` (consultor pausado demais) e `idle_store` têm constantes DEFINIDAS mas:

- **(a) nada os emite** — só `long_open_service` é emitido no scan periódico
  (`back/internal/modules/queue/operations/service_alerts.go`, emissão ~linha 106);
- **(b) o processador os DESCARTA** — o switch de
  `back/internal/modules/queue/alerts/store_postgres_signals.go` (~linha 98) só trata
  `long_open_service.triggered/.resolved` e cai em `default: continue` para o resto.

Constantes existentes: `back/internal/modules/queue/operations/alerts.go:14-24`
(`TriggerLongOpenService`, `TriggerLongQueueWait`, `TriggerLongPause`, `TriggerIdleStore`, struct
`OperationalAlertSignal`, interface `ReceiveOperationalSignals`). O ticker do scan já roda
(`back/internal/platform/app/app.go:113`, `ProcessTimedAlerts`).

**Fase 1 (esta spec):** `long_queue_wait` + `long_pause` — dados já existem
(`operation_queue_entries.joined_at`, `operation_paused_consultants.paused_at`).
**Fase 2 (FORA):** `idle_store` (depende de business-hours configurado por loja).

## 2. Objetivo e não-objetivos

**Objetivo (escopo fechado):**
1. Migration: colunas de threshold `long_queue_wait_minutes` e `long_pause_minutes` em
   `tenant_operational_alert_rules` (+ seed das rule definitions, padrão existente).
2. Emissão dos 2 sinais no scan periódico existente + emissão dos `.resolved` (entrar em
   atendimento resolve o queue_wait; despausar resolve o pause).
3. Processamento: 2 novos cases no switch + funções `process*Tx` clonadas do molde
   `long_open_service`, materializando em `alert_instances` com dedupe.
4. Alertas visíveis onde os `long_open_service` já aparecem (workspace de alertas + sino) — SEM
   nova UI: o front já renderiza `alert_instances` genericamente por tipo.

**Não-objetivos (FORA):**
- `idle_store` (fase 2); notificação externa direta (o caminho externo é OBS-04/05 lendo
  `alert_instances`); mudanças de UI além de labels; pedido travado do cardápio (é read-time no OBS-05).

## 3. Regras de execução (obrigatórias)

- NENHUM comando git. Validação = `docker compose up -d --build api`.
- **RE-LER os arquivos-molde antes de codar** (as linhas citadas são de 03/07):
  `service_alerts.go`, `store_postgres_signals.go`, `store_postgres_rules.go`, `alerts/model.go`,
  migration `0056` (rule definitions) e `0044/0106` (colunas/view de rules).
- Migration: **próximo número livre** (`ls back/internal/platform/database/migrations/ | tail -3`;
  em 03/07 o último era `0187` → provável `0188`). SQL plano, idempotente
  (`add column if not exists`, `on conflict do nothing`), SEM goose Down. Se a rules for exposta por
  view `public.tenant_operational_alert_rules`, recriar a view com `create or replace view` no MESMO
  arquivo (padrão das migrations 0113+; conferir `relkind` antes de qualquer drop).
- Defesa multi-tenant em TODA query nova (tenant_id no WHERE mesmo com service validando).
- Máx 450 linhas/arquivo (se `store_postgres_signals.go` estourar, extrair os novos processors para
  `store_postgres_signals_wait_pause.go` no mesmo pacote).
- Atualizar AGENT.md de `operations` e `alerts`.

## 4. Mudanças (passo a passo)

### 4.1 CRIAR migration `back/internal/platform/database/migrations/<NNNN>_alert_rules_queue_wait_pause.sql`

```sql
-- OBS-06 fase 1: thresholds de long_queue_wait e long_pause por tenant.
-- Idempotente; espelha o padrao de long_open_service_minutes (0044/0106).
alter table queue.tenant_operational_alert_rules
  add column if not exists long_queue_wait_minutes integer not null default 20,
  add column if not exists long_pause_minutes integer not null default 30;

-- Recriar a view de compat public.* SE ela existir (conferir na implementacao:
-- select relkind from pg_class where relname='tenant_operational_alert_rules').
-- create or replace view public.tenant_operational_alert_rules as select * from queue.tenant_operational_alert_rules;

-- Rule definitions (padrao da 0056) — nomes/estrutura da tabela de definitions
-- devem ser confirmados na 0056 antes de finalizar este bloco.
```

**Nota ao implementador:** o esqueleto acima assume `queue.tenant_operational_alert_rules` como
tabela base; CONFIRMAR schema/nome reais via grep nas migrations antes de finalizar (a evidência de
03/07 aponta a view criada na `0106`). Defaults 20/30 min são decisão fechada (alinhados ao
`defaultLongOpenMinutes` existente — conferir o valor no código e manter proporção).

### 4.2 EDITAR `back/internal/modules/queue/operations/store_postgres.go` (ou arquivo do scan)

Novas queries do scan (usar as TABELAS base schema-qualificadas, com tenant/store no WHERE):

```sql
-- entradas de fila esperando alem do threshold do tenant
select e.id, e.tenant_id, e.store_id, e.joined_at
  from queue.operation_queue_entries e
  join queue.tenant_operational_alert_rules r on r.tenant_id = e.tenant_id
 where e.status = 'waiting'
   and e.joined_at < now() - make_interval(mins => r.long_queue_wait_minutes);

-- pausas alem do threshold
select p.consultant_id, p.tenant_id, p.store_id, p.paused_at
  from queue.operation_paused_consultants p
  join queue.tenant_operational_alert_rules r on r.tenant_id = p.tenant_id
 where p.paused_at < now() - make_interval(mins => r.long_pause_minutes);
```

(Nomes de coluna/status: CONFIRMAR no schema real — `operation_queue_entries` pode usar outro campo
de estado; espelhar o que o scan de `long_open_service` já faz.)

### 4.3 EDITAR `back/internal/modules/queue/operations/service_alerts.go` — emissores

No `ProcessTimedAlerts` (mesmo ciclo do long_open_service): para cada linha das queries do 4.2,
emitir `OperationalAlertSignal{SignalType: TriggerLongQueueWait + ".triggered", ElapsedMinutes: ...}`
(idem pause). Emissão de `.resolved`:
- `long_queue_wait.resolved` — no ponto onde a entrada de fila vira atendimento (seguir o fluxo de
  start-service; mesmo lugar que já mexe em `operation_queue_entries`).
- `long_pause.resolved` — no despausar (fluxo resume; espelhar como o finish emite
  `long_open_service.resolved` em `service_finish.go`).

O dedupe do processador (4.4) torna re-emissões inofensivas — emitir resolved SEM checar se havia
triggered é aceitável e mais simples (o processor ignora resolved sem instância ativa, como o molde
long_open_service já faz).

### 4.4 EDITAR `back/internal/modules/queue/alerts/store_postgres_signals.go` — processadores

No switch (~linha 98), adicionar ANTES do `default`:

```go
case operations.TriggerLongQueueWait + ".triggered":
	err = s.processLongQueueWaitTriggeredTx(ctx, tx, signal)
case operations.TriggerLongQueueWait + ".resolved":
	err = s.processLongQueueWaitResolvedTx(ctx, tx, signal)
case operations.TriggerLongPause + ".triggered":
	err = s.processLongPauseTriggeredTx(ctx, tx, signal)
case operations.TriggerLongPause + ".resolved":
	err = s.processLongPauseResolvedTx(ctx, tx, signal)
```

(Adaptar à forma EXATA do switch atual — pode casar por campo `SignalType` já separado em
tipo+evento; espelhar o long_open_service.)

Funções novas = CLONES ADAPTADOS de `processLongOpenService{Triggered,Resolved}Tx`, mudando:
- `dedupe_key`: `operations:long_queue_wait:<storeID>:<entryID>` e
  `operations:long_pause:<storeID>:<consultantID>` (mesmo prefixo/formato do molde).
- `type`: `long_queue_wait` / `long_pause`; `category: operational`; `severity: warning`
  (long_open é critical; espera/pausa longa é warning — decisão fechada).
- `headline/body`: "Cliente aguardando ha X min na fila" / "Consultor pausado ha X min"
  (seguir o formato/i18n do molde).
- Threshold para o corpo: `ElapsedMinutes` do sinal.

Se o arquivo passar de 450 linhas → extrair para `store_postgres_signals_wait_pause.go`.

### 4.5 EDITAR `back/internal/modules/queue/alerts/store_postgres_rules.go` + `model.go`

- `LoadOperationalRules`: scan das 2 colunas novas (com default se NULL — `*int` + fallback).
- `model.go`: os 2 tipos novos com `interaction_kind` PASSIVO (sem response_required — igual ao
  comportamento de exibição do long_open; conferir o enum existente).

### 4.6 Testes

- `operations`: teste do emissor (fixture com entrada de fila velha → sinal emitido; recente → não).
- `alerts`: estender o teste de processamento (triggered cria instância com dedupe; re-triggered não
  duplica; resolved fecha) e `cross_tenant_test.go` (sinal do tenant A nunca cria/fecha instância do B).

### 4.7 EDITAR AGENT.md de `queue/operations` e `queue/alerts` (sinais novos, thresholds, dedupe keys).

## 5. Critérios de aceite

1. Migration aplica limpa em banco dev existente E em volume zerado (idempotente, 2ª aplicação no-op).
2. Dev: entrada de fila com `joined_at` retroagido além do threshold → na próxima varredura do
   ticker existe `alert_instances` `type=long_queue_wait, status=active`; iniciar o atendimento →
   instância `resolved`.
3. Idem pause: pausar consultor com `paused_at` retroagido → alerta; despausar → resolved.
4. Sem duplicação: 3 varreduras seguidas = 1 instância ativa por dedupe_key.
5. Alerta aparece no workspace de alertas do painel (front genérico por tipo) sem mudança de UI.
6. `go test ./internal/modules/queue/...` verde; nenhum arquivo >450; cross-tenant test verde.
7. `GET /v1/monitoring/status` (OBS-05, se já aplicado) mostra os tipos novos em `alerts.byType`.

## 6. Validação

```bash
docker compose up -d --build api
# retroagir uma entrada de fila em dev (banco local, NUNCA prod):
docker compose exec -T postgres psql -U omni -d omni -c \
  "update queue.operation_queue_entries set joined_at = now() - interval '90 minutes' where id = '<id de teste>'"
docker compose logs api -f | grep -E "long_queue_wait|long_pause"
```

Smoke no painel (workspace Alertas) fica com o dono.

## 7. Notas de Deploy

- **Migration nova** (roda no boot da api — `migrate up && api`). **Env vars:** nenhuma.
  **Rebuild:** api. Thresholds default 20/30 min entram para todos os tenants; ajuste por tenant
  via a tela de regras existente (se a tela expõe colunas dinamicamente) ou SQL manual documentado.
- Rollback: voltar imagem (colunas novas ficam inertes — código antigo as ignora).

## 8. Arquivos tocados

| Arquivo | Ação |
|---|---|
| `back/internal/platform/database/migrations/<NNNN>_alert_rules_queue_wait_pause.sql` | criar |
| `back/internal/modules/queue/operations/{service_alerts,store_postgres,service_finish}.go` (+ ponto de start-service) | editar |
| `back/internal/modules/queue/alerts/{store_postgres_signals,store_postgres_rules,model}.go` | editar |
| `back/internal/modules/queue/alerts/store_postgres_signals_wait_pause.go` | criar (se >450) |
| testes dos 2 módulos | criar/editar |
| AGENT.md dos 2 módulos | editar |

**Conflitos potenciais:** OBS-05 (os tipos novos aparecem no summary — nenhum código extra, mas o
OBS-04 R4 conta `alerts.open` incluindo-os: thresholds do R4 podem precisar de ajuste depois).
