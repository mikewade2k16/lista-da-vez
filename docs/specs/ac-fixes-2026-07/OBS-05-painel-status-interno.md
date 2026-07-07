# OBS-05 — Painel de status interno real (substitui o mock de /monitoramento)

> Spec de implementação · Prioridade **P2** · Esforço **M** · Impacto **médio**
> Origem: AC-16 (evolução) · roadmap `observabilidade-n8n` → task `obs-05-painel-status-interno`

## 1. Contexto

**O achado:** a página `/monitoramento` do painel é **mock de design**:
`web/app/pages/monitoramento.vue` renderiza `DemoWorkspacePage` com dados fake de
`web/app/utils/demo-pages.ts:156-187` ("Servicos OK 12", "webhook 84ms" — nada real). Não existe
backend de monitoração. Regra do projeto (princípio 4): mock visível tem que virar dado real ou ser
marcado — este spec o torna REAL.

Fontes reais já existentes para agregar:
- `alert_instances` (módulo `queue/alerts`) — alertas operacionais abertos.
- `erp_sync_runs` — última sincronização ERP.
- `cardapio.orders` — pedidos em status não-terminal parados (statuses em
  `back/internal/modules/cardapio/model_order.go:10-15`: recebido/em_preparo/pronto/saiu_entrega).
- `backups/last_backup_status` (host) — via mount read-only.
- n8n interno — `GET {AUTOMATION_N8N_INTERNAL_URL}/healthz`.

## 2. Objetivo e não-objetivos

**Objetivo (escopo fechado):**
1. Novo módulo Go `back/internal/modules/monitoring/` com `GET /v1/monitoring/status`
   (platform_admin only) e `GET /v1/runtime/monitoring/summary` (Bearer de serviço — contrato do OBS-04).
2. Front real: `/monitoramento` deixa de ser demo; workspace com cards consumindo o endpoint,
   polling 30s.
3. Status do backup legível pela api via arquivo montado read-only (prod) com fallback `unknown` (dev).

**Não-objetivos (FORA):**
- NÃO gravar nada (módulo 100% read-only; zero migration).
- NÃO série temporal/histórico (v2); NÃO WebSocket (polling 30s na v1).
- NÃO tocar o módulo `queue/alerts` (só SELECT em `alert_instances`).
- NÃO remover `demo-pages.ts` (outras páginas demo continuam) — só a chave `monitoramento` sai de uso.

## 3. Regras de execução (obrigatórias)

- NENHUM comando git. Back: `docker compose up -d --build api`. Front: `npm run dev:watch`.
- Máx 450 linhas/arquivo; handler→service→store (sem pular camada); IDs string; scan nullable `*string`.
- **Gate de front:** `has()` retorna false para platform_admin — TODO gating deve ser
  `isPlatformAdmin || has(...)` (memória do projeto; sem isso o admin não vê a página).
- Multi-tenant: este endpoint é de PLATAFORMA (agrega cross-tenant de propósito) — por isso o gate
  duro de `platform_admin` no back é OBRIGATÓRIO (403/404 para os demais).
- Atualizar AGENT.md do módulo novo + `back/internal/platform/app/AGENT.md` + `web/AGENT.md`.

## 4. Mudanças (passo a passo)

### 4.1 CRIAR `back/internal/modules/monitoring/` (4 arquivos, padrão dos módulos existentes — usar `bi/` como referência de módulo read-only pequeno)

**`model.go`** — o CONTRATO (o OBS-04 depende dele; não renomear campos):

```go
package monitoring

type StatusSummary struct {
	DB       DBStatus       `json:"db"`
	Alerts   AlertsStatus   `json:"alerts"`
	ERP      ERPStatus      `json:"erp"`
	Cardapio CardapioStatus `json:"cardapio"`
	N8N      N8NStatus      `json:"n8n"`
	Backup   BackupStatus   `json:"backup"`
	At       string         `json:"at"` // ISO8601 UTC
}

type DBStatus struct {
	Status string `json:"status"` // "ok" | "down"
	PingMs int64  `json:"pingMs"`
}
type AlertsStatus struct {
	Open   int            `json:"open"`
	ByType map[string]int `json:"byType"`
}
type ERPStatus struct {
	LastRunStatus string  `json:"lastRunStatus"` // "ok" | "failed" | "none"
	LastRunAt     *string `json:"lastRunAt"`
}
type CardapioStatus struct {
	StuckOrders     int `json:"stuckOrders"`
	StuckAfterMin   int `json:"stuckAfterMin"` // threshold usado (informativo)
}
type N8NStatus struct {
	Reachable bool `json:"reachable"`
}
type BackupStatus struct {
	Status string  `json:"status"` // "ok" | "fail" | "unknown"
	Detail *string `json:"detail"`
	At     *string `json:"at"`
}
```

**`store_postgres.go`** — queries (todas parametrizadas, schema-qualificadas):

```sql
-- alertas abertos por tipo
select trigger_type, count(*) from queue.alert_instances
 where status = 'active' group by trigger_type;
-- ultima execucao ERP (ajustar nomes de coluna ao schema real de erp_sync_runs ao implementar)
select status, finished_at from erp_sync_runs order by started_at desc limit 1;
-- pedidos travados: nao-terminal parados ha mais de $1 minutos
select count(*) from cardapio.orders
 where status in ('recebido','em_preparo','pronto','saiu_entrega')
   and updated_at < now() - make_interval(mins => $1);
```

**ATENÇÃO ao implementar:** conferir os nomes reais de tabela/colunas com grep nas migrations
(`alert_instances` pode viver como view public.* e tabela `queue.*`; `erp_sync_runs` é public) —
usar a tabela BASE schema-qualificada, não a view.

**`service.go`** — `Status(ctx) (StatusSummary, error)`: db ping = `pool.Ping` cronometrado
(timeout 2s → `down`); n8n = GET `{cfg.AutomationN8NInternalURL}/healthz` com `http.Client{Timeout: 2s}`
(URL vazia → `Reachable:false`); backup = ler `cfg.MonitoringBackupStatusFile` (ausente/vazio →
`unknown`; 1ª palavra `ok|fail`, 2ª = ts, resto = detail — gramática do `backup-db.sh:29,82`);
threshold de pedido travado = const `stuckOrderMinutes = 45`.

**`http.go`** — duas rotas:
- `GET /v1/monitoring/status`: middleware de auth existente + guarda `principal.IsPlatformAdmin`
  (senão **404**, padrão anti-enumeração do projeto).
- `GET /v1/runtime/monitoring/summary`: Bearer == `cfg.AutomationRuntimeToken` (mesmo padrão do
  runtime-config do módulo `automation` — copiar a comparação constant-time de lá); payload =
  o MESMO StatusSummary (sem PII por construção).

**`module.go`** — registro no padrão dos demais módulos; wire em `back/internal/platform/app/app.go`.

### 4.2 EDITAR `back/internal/platform/config/config.go`

Nova env `MONITORING_BACKUP_STATUS_FILE` (string, default `""`), campo
`MonitoringBackupStatusFile`. Documentar no `back/.env.example`.

### 4.3 EDITAR `docker-compose.prod.yml` — mount read-only no serviço api

```yaml
    volumes:
      # OBS-05: status do backup (host) legivel pela api; ausente em dev => "unknown"
      - /home/deploy/lista-atendimento/backups/last_backup_status:/app/data/monitoring/last_backup_status:ro
    environment:
      MONITORING_BACKUP_STATUS_FILE: /app/data/monitoring/last_backup_status
```

**Armadilha:** se o arquivo não existir na VPS no momento do `up`, o docker cria um DIRETÓRIO no
lugar — o runbook manda garantir `touch` antes (`ssh ... "test -f .../last_backup_status || echo 'unknown' > .../last_backup_status"`).
No compose de DEV não montar nada (env fica vazia → `unknown`).

### 4.4 CRIAR front

- **`web/app/stores/monitoring.ts`** (setup-store padrão do projeto): state
  `{ summary: StatusSummary | null, loading, error }`, action `fetchStatus()` via api-client
  (`GET /v1/monitoring/status`), `startPolling()/stopPolling()` com `setInterval` 30s e clear no stop.
- **`web/app/components/monitoring/MonitoringWorkspace.vue`** (+ subcomponentes se passar de 450):
  cards Banco (status+ping), Alertas abertos (total + por tipo), ERP (última sync), Pedidos
  travados, n8n, Backup (status+quando). Estados de loading/erro explícitos (princípio 5 — nada de
  default silencioso: backup `unknown` mostra "sem dado no dev" em texto).
- **EDITAR `web/app/pages/monitoramento.vue`**: remover `DemoWorkspacePage`, renderizar o
  workspace novo; `onMounted → startPolling`, `onUnmounted → stopPolling`.
- **Gate:** página/menu com `isPlatformAdmin || has('monitoring.view')` (front); o back já força
  platform_admin.

### 4.5 Testes

- Go: `service_test.go` com stores fake (db ok/down, backup ok/fail/unknown por arquivo temporário,
  n8n reachable via `httptest.Server`). Padrão dos services testáveis existentes.
- Front: `web/app/stores/monitoring.test.ts` (padrão onda 1: estado inicial, fetch feliz via mock
  `$fetch`, erro de fetch, stopPolling limpa o interval).

### 4.6 EDITAR docs

`back/internal/modules/monitoring/AGENT.md` (novo — contrato do summary, gates, envs),
`back/internal/platform/app/AGENT.md` (módulo registrado), `web/AGENT.md` (página real, store),
e REMOVER a chave `monitoramento` de `web/app/utils/demo-pages.ts` (a página não é mais demo).

## 5. Critérios de aceite

1. Login como platform_admin em dev → `/monitoramento` mostra dados REAIS (alertas do banco local,
   erp `none/ok`, backup `unknown`, n8n reachable se profile automation up).
2. Usuário NÃO-platform_admin: `GET /v1/monitoring/status` → 404; página fora do menu.
3. `curl -H "Authorization: Bearer dev-automation-runtime-token" http://localhost:9091/v1/runtime/monitoring/summary`
   → 200 com o shape do contrato; Bearer errado → 401.
4. Pedido de cardápio parado >45min em dev aparece em `cardapio.stuckOrders`.
5. `go test ./internal/modules/monitoring/...` e `vitest run` (container) verdes.
6. Nenhum arquivo >450 linhas; demo-pages sem a chave morta.

## 6. Validação

```bash
docker compose up -d --build api && docker compose logs api --tail 20
curl -s http://localhost:9091/v1/runtime/monitoring/summary -H "Authorization: Bearer dev-automation-runtime-token" | head -c 400
npm run dev:watch   # conferir /monitoramento no browser (platform_admin)
```

Smoke autenticado no browser fica com o dono (credenciais dele).

## 7. Notas de Deploy

- **Migrations:** nenhuma. **Env nova:** `MONITORING_BACKUP_STATUS_FILE` (só prod, via compose).
- **Compose prod:** mount ro novo no api (recreate do api na próxima subida) — garantir o `touch`
  do arquivo ANTES (armadilha do diretório).
- **Rebuild:** api (back mudou) + web (front mudou) — caminho GHCR normal.
- Rollback: voltar imagem; o mount é inofensivo para imagens antigas.

## 8. Arquivos tocados

| Arquivo | Ação |
|---|---|
| `back/internal/modules/monitoring/{module,http,service,store_postgres,model}.go` | criar |
| `back/internal/modules/monitoring/service_test.go` + `AGENT.md` | criar |
| `back/internal/platform/app/app.go` / `config/config.go` / `back/.env.example` | editar |
| `docker-compose.prod.yml` | editar (mount + env api) |
| `web/app/stores/monitoring.ts` (+`.test.ts`) | criar |
| `web/app/components/monitoring/MonitoringWorkspace.vue` | criar |
| `web/app/pages/monitoramento.vue` | editar (sai o demo) |
| `web/app/utils/demo-pages.ts` | editar (remove chave) |
| AGENT.md (monitoring, app, web) | editar/criar |

**Conflitos potenciais:** OBS-04 consome o summary (contrato `db/erp/cardapio/alerts/backup` —
congelar nomes); OBS-06 adiciona novos trigger_types que aparecem automaticamente em
`alerts.byType` (sem mudança aqui).
