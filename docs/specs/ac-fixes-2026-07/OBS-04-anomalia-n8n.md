# OBS-04 — Detecção de anomalia via n8n agendado (regras v1)

> Spec de implementação · Prioridade **P2** · Esforço **M** · Impacto **médio**
> Origem: AC-16 (evolução) · roadmap `observabilidade-n8n` → task `obs-04-deteccao-anomalia-metricas`
> **DEPENDE DE:** OBS-02 (workflow de fan-out `Omni Alerts` ativo) e OBS-05 (endpoint
> `GET /v1/runtime/monitoring/summary`). NÃO executar antes dos dois.

## 1. Contexto

**O achado:** os alertas existentes são threshold de HOST (disco/RAM/load) ou eventos pontuais.
Ninguém cruza sinais de NEGÓCIO/plataforma (ERP sync falhou, pedidos travados acumulando, alertas
operacionais abertos demais, backup falho) num verificador periódico. Decisão de design: fazer isso
**no n8n, não em Go** — thresholds mudam sem rebuild/deploy da api, e o fan-out (OBS-02) já vive lá.
v1 = **regras fixas**, não estatística (anomalia estatística é v2, só com histórico acumulado).

Fatos:
- OBS-05 cria `GET /v1/runtime/monitoring/summary` (Bearer `AUTOMATION_RUNTIME_TOKEN`, payload
  reduzido sem PII) — a fonte única deste workflow.
- Na rede interna do compose o n8n alcança a api por `http://api:8080` (mesmo padrão do
  `AUTOMATION_N8N_INTERNAL_URL` invertido — aqui é n8n→api).
- `$getWorkflowStaticData('global')` persiste estado entre execuções do workflow (usado p/ cooldown).

## 2. Objetivo e não-objetivos

**Objetivo:** workflow `Omni Anomaly` agendado (15 min) que lê o summary, aplica a tabela de regras
v1 e dispara os achados pelo fan-out do OBS-02, com cooldown de 1h por regra.

**Não-objetivos:** detecção estatística/baseline (v2); persistir histórico (o painel OBS-05 mostra o
estado atual; série temporal é futuro); alertar host metrics (já é o check-vps).

## 3. Regras de execução

- NENHUM comando git. Validar em n8n de DEV antes de prod. Tokens reais só com o dono.
- Não duplicar canal: este workflow NUNCA chama Telegram/ntfy direto — SEMPRE via `Execute Workflow → Omni Alerts`.
- Atualizar `automation/AGENT.md` + `docs/automation/SETUP.md`.

## 4. Mudanças (passo a passo)

### 4.1 CRIAR `automation/export/workflow-omni-anomaly.json`

Nodes (envelope igual aos exports existentes; `"active": false` no arquivo):

1. **Schedule Trigger** (`n8n-nodes-base.scheduleTrigger`): a cada 15 minutos.
2. **HTTP Request** `Get summary`: `GET http://api:8080/v1/runtime/monitoring/summary`, header
   `Authorization: Bearer {{ $env.AUTOMATION_RUNTIME_TOKEN }}`, timeout 10s,
   `onError: continueRegularOutput` (api fora → o próprio Code node alerta "summary inacessível").
3. **Code** `Aplicar regras v1` (JavaScript). Implementar EXATAMENTE:

```javascript
const staticData = $getWorkflowStaticData('global');
staticData.lastFired = staticData.lastFired || {};
const now = Date.now();
const COOLDOWN_MS = 60 * 60 * 1000; // 1h por regra

const resp = $input.first().json;
const s = resp.body ?? resp; // summary
const findings = [];
const add = (key, severity, msg) => {
  const last = staticData.lastFired[key] || 0;
  if (now - last < COOLDOWN_MS) return;
  staticData.lastFired[key] = now;
  findings.push({ host: 'omni-api', key, severity, msg, ts: new Date().toISOString() });
};

// R0 — summary inacessível (a chamada HTTP falhou/veio vazia)
if (!s || s.error || typeof s !== 'object' || (!('db' in s) && !('alerts' in s))) {
  add('anomaly_summary_unreachable', 'critical', 'GET /v1/runtime/monitoring/summary inacessivel ou invalido.');
  return findings.map(f => ({ json: f }));
}
// R1 — banco: latencia do ping acima de 500ms ou down
if (s.db && s.db.status !== 'ok') add('anomaly_db', 'critical', `db status=${s.db.status}`);
else if (s.db && s.db.pingMs > 500) add('anomaly_db_slow', 'warning', `ping do banco em ${s.db.pingMs}ms (>500ms)`);
// R2 — ERP sync: ultima execucao falhou
if (s.erp && s.erp.lastRunStatus === 'failed') add('anomaly_erp_sync', 'warning', `ultimo erp_sync falhou em ${s.erp.lastRunAt}`);
// R3 — pedidos travados (cardapio) > 0
if (s.cardapio && s.cardapio.stuckOrders > 0) add('anomaly_stuck_orders', 'warning', `${s.cardapio.stuckOrders} pedido(s) parados alem do limite`);
// R4 — alertas operacionais abertos demais
if (s.alerts && s.alerts.open >= 10) add('anomaly_open_alerts', 'warning', `${s.alerts.open} alertas operacionais abertos (>=10)`);
// R5 — backup: status fail ou desconhecido em prod
if (s.backup && s.backup.status === 'fail') add('anomaly_backup', 'critical', `backup: ${s.backup.detail || 'fail'}`);

return findings.map(f => ({ json: f }));
```

4. **IF** `tem achados?` (items > 0) → TRUE → **Execute Workflow** apontando para **`Omni Alerts`**
   (OBS-02), passando cada item (o shape `{host,key,msg,severity,ts}` é o MESMO contrato do
   webhook — o workflow de fan-out precisa aceitar entrada via Execute Workflow além do webhook;
   se o nó Webhook não aceitar, adaptar `Omni Alerts` com um node `Execute Workflow Trigger`
   paralelo ao Webhook, convergindo no mesmo Format).

### 4.2 Tabela de regras v1 (documentar na spec/AGENT.md; thresholds ajustáveis SÓ no Code node)

| Regra | Fonte (summary) | Condição | Severidade |
|---|---|---|---|
| R0 summary inacessível | — | HTTP falhou/shape inválido | critical |
| R1 banco | `db.status` / `db.pingMs` | ≠ ok / > 500ms | critical / warning |
| R2 ERP sync | `erp.lastRunStatus` | `failed` | warning |
| R3 pedidos travados | `cardapio.stuckOrders` | > 0 | warning |
| R4 alertas abertos | `alerts.open` | ≥ 10 | warning |
| R5 backup | `backup.status` | `fail` | critical |

### 4.3 Runbook (dev → prod)

Igual ao OBS-02 §4.3: import no dev, ativar, esperar 1 ciclo (15 min) ou "Execute workflow" manual,
conferir que um achado forjado (ex.: derrubar o postgres de DEV por 1 min) chega pelos canais.
Prod: dono importa/ativa no `omni-n8n-1`.

### 4.4 EDITAR docs

`automation/AGENT.md` (workflow + tabela de regras + "thresholds moram no Code node") e
`docs/automation/SETUP.md` (lista de imports).

## 5. Critérios de aceite

1. Import limpo em n8n dev; execução manual sem achados retorna vazio e NÃO alerta.
2. Cenário forjado por regra (dev): cada Rx dispara 1 alerta com a chave/severidade da tabela.
3. Cooldown: repetir a execução em <1h NÃO re-alerta a mesma regra.
4. api de dev fora do ar → `anomaly_summary_unreachable` critical (R0).
5. Nenhuma credencial no JSON exportado (só `$env.*`).

## 6. Validação

Passo 4.3. `docker compose exec n8n n8n list:workflow` confirma os 2 workflows OBS ativos.

## 7. Notas de Deploy

Nenhuma migration/env NOVA (reusa `AUTOMATION_RUNTIME_TOKEN` já presente). Import + ativação no n8n
de prod. Rollback: desativar o workflow.

## 8. Arquivos tocados

| Arquivo | Ação |
|---|---|
| `automation/export/workflow-omni-anomaly.json` | criar |
| `automation/AGENT.md` / `docs/automation/SETUP.md` | editar |

**Conflitos potenciais:** contrato do payload com OBS-02 (`Omni Alerts`) e do summary com OBS-05
(`db/erp/cardapio/alerts/backup` — se o shape do summary mudar, atualizar o Code node junto).
