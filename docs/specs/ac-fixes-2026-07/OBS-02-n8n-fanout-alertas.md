# OBS-02 — Workflow n8n de fan-out de alertas (e-mail + Telegram + ntfy)

> Spec de implementação · Prioridade **P1** · Esforço **S** · Impacto **alto**
> Origem: evolução do AC-16 · roadmap `observabilidade-n8n` → task `obs-02-fanout-email-telegram-ntfy`
> **Depende de/contrato com:** OBS-01 (emissor; payload `{host,key,msg,severity,ts}`)

## 1. Contexto

**O achado:** os alertas do host saem hoje por canais hardcoded no `check-vps.sh` (Telegram/ntfy
direto). Não há fan-out central: adicionar e-mail, mudar destinatário ou formatar mensagem exige
editar script bash na VPS. O n8n de prod (`omni-n8n-1`, `n8nio/n8n:2.23.2`) já roda 24/7 na stack
automation e é o lugar natural desse roteamento.

Evidências:
- `docker-compose.yml:292-322` (dev) e serviço n8n no `docker-compose.prod.yml` — envs
  `AUTOMATION_RUNTIME_TOKEN`, `N8N_BLOCK_ENV_ACCESS_IN_NODE: "false"` (o `$env.*` funciona em
  expression), porta host `127.0.0.1:15680` em prod (`AUTOMATION_N8N_PORT`).
- `automation/export/*.json` — padrão de versionamento dos workflows (array com
  `{id,name,nodes,connections,settings,...}`); import via
  `docker compose cp <file> n8n:/tmp/wf.json && docker compose exec n8n n8n import:workflow --input=/tmp/wf.json`
  (runbook `docs/automation/SETUP.md` §3.4).
- SMTP: as vars `SMTP_HOST/PORT/USERNAME/PASSWORD/FROM_*` já existem no `.env.production` (a api
  usa); o n8n usará os MESMOS valores numa credencial própria criada 1x no editor.

## 2. Objetivo e não-objetivos

**Objetivo (escopo fechado):**
1. Workflow `Omni Alerts` versionado em `automation/export/workflow-omni-alerts.json`: webhook
   `POST /webhook/omni-monitoring` autenticado por Bearer → responde 200 imediato → roteia por
   severidade → **critical** = E-mail + Telegram + ntfy; **warning** = Telegram + ntfy.
2. Envs de canal no serviço n8n dos dois compose (defaults vazios em dev).
3. Runbook de import/ativação + criação da credencial SMTP.

**Não-objetivos (FORA):**
- NÃO fazer dedup/cooldown no workflow (fica no emissor — check-vps.sh já tem cooldown 1h por chave).
- NÃO expor o webhook publicamente (o emissor usa loopback `127.0.0.1:15680`; o subdomínio
  n8n.* com basic_auth não é tocado).
- NÃO mexer nos workflows existentes (whatsapp, omni-chat, calendar).

## 3. Regras de execução (obrigatórias)

- NENHUM comando git. NUNCA inventar tokens/chats: `AUTOMATION_RUNTIME_TOKEN` e os valores de
  Telegram/ntfy/SMTP são os que o DONO já tem (pedir quando precisar testar de verdade).
- Validar o JSON importando num n8n de DEV primeiro (`docker compose --profile automation up -d n8n`),
  nunca direto em prod.
- Manter mensagem SEM dados sensíveis (o payload só carrega host/key/msg/severity/ts).
- Atualizar `docs/automation/SETUP.md` + `automation/AGENT.md`.

## 4. Mudanças (passo a passo)

### 4.1 CRIAR `automation/export/workflow-omni-alerts.json`

Estrutura obrigatória (nodes e fluxo; ao montar o JSON final, seguir o envelope dos exports
existentes — array raiz com um objeto de workflow, `nodes[]` + `connections{}`):

1. **Webhook** (`n8n-nodes-base.webhook`, typeVersion 2.1): `httpMethod: POST`,
   `path: omni-monitoring`, `responseMode: responseNode`.
2. **IF Auth** (`n8n-nodes-base.if`): condição string equals —
   `={{ $json.headers.authorization }}` == `={{ 'Bearer ' + $env.AUTOMATION_RUNTIME_TOKEN }}`.
   FALSE → **Respond 401** (`n8n-nodes-base.respondToWebhook`, responseCode 401, body `{"error":"unauthorized"}`).
3. TRUE → **Respond 200** (`respondToWebhook`, responseCode 200, body `{"ok":true}`) — responder
   ANTES do fan-out (não segurar o curl do cron).
4. Após o Respond 200 → **Format** (`n8n-nodes-base.set` ou Code): monta
   `text = "[Omni " + $json.body.host + "][" + $json.body.severity + "][" + $json.body.key + "] " + $json.body.msg + " (" + $json.body.ts + ")"`
   e propaga `severity`.
5. **Switch severity** (`n8n-nodes-base.switch`): output `critical` / fallback `warning`.
6. Canais (todos com `onError: continueRegularOutput` para um canal caído não matar os outros):
   - **Telegram** (`n8n-nodes-base.httpRequest` POST):
     `https://api.telegram.org/bot{{ $env.ALERT_TELEGRAM_BOT_TOKEN }}/sendMessage`,
     body JSON `{"chat_id": "{{ $env.ALERT_TELEGRAM_CHAT_ID }}", "text": "{{ $json.text }}"}` —
     ligado nas DUAS saídas do switch.
   - **ntfy** (`httpRequest` POST): URL `={{ $env.ALERT_NTFY_URL }}`, header
     `Title: Omni Alerts`, body raw = `={{ $json.text }}` — nas DUAS saídas.
   - **E-mail** (`n8n-nodes-base.emailSend`, credencial SMTP `Omni SMTP`): to
     `={{ $env.MONITORING_ALERT_EMAIL_TO }}`, subject
     `[Omni][{{ $json.severity }}] {{ $json.key }}`, text = `={{ $json.text }}` — SÓ na saída `critical`.

O JSON deve ter `"active": false` no export (ativação é passo do runbook) e ids de node fixos
(uuid literais) para o import ser reprodutível.

### 4.2 EDITAR `docker-compose.prod.yml` e `docker-compose.yml` — envs do serviço n8n

Adicionar ao `environment:` do serviço `n8n` (prod usa `${VAR}` do `.env.production`; dev usa
default vazio):

```yaml
      # OBS-02: canais do fan-out de alertas (workflow-omni-alerts). Vazio = canal desligado.
      ALERT_TELEGRAM_BOT_TOKEN: ${ALERT_TELEGRAM_BOT_TOKEN:-}
      ALERT_TELEGRAM_CHAT_ID: ${ALERT_TELEGRAM_CHAT_ID:-}
      ALERT_NTFY_URL: ${ALERT_NTFY_URL:-}
      MONITORING_ALERT_EMAIL_TO: ${MONITORING_ALERT_EMAIL_TO:-}
```

Adicionar as 4 vars (comentadas/vazias) em `.env.docker.example` e `.env.production.example`.

### 4.3 Runbook de import (dev primeiro, depois prod)

```bash
# DEV
docker compose --profile automation up -d n8n
docker compose cp automation/export/workflow-omni-alerts.json n8n:/tmp/wf.json
docker compose exec n8n n8n import:workflow --input=/tmp/wf.json
# criar credencial SMTP "Omni SMTP" no editor (http://localhost:5680) com os SMTP_* — 1x
# ativar o workflow no editor e testar:
curl -sS -X POST http://127.0.0.1:5680/webhook/omni-monitoring \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer dev-automation-runtime-token" \
  -d '{"host":"dev","key":"teste","msg":"alerta de teste OBS-02","severity":"critical","ts":"2026-07-03T00:00:00Z"}'
# => {"ok":true} e mensagens nos canais configurados; sem/errado o Bearer => 401
```

Prod (dono roda, na VPS): mesmo import no container `omni-n8n-1` com
`docker compose --env-file .env.production -f docker-compose.prod.yml`, adicionar as 4 vars no
`.env.production`, `up -d n8n` (recria com as envs), criar credencial SMTP no editor de prod,
ativar, testar com o Bearer real.

### 4.4 EDITAR docs

- `docs/automation/SETUP.md`: workflow novo na lista de imports (§3.4) + credencial SMTP.
- `automation/AGENT.md`: registrar o workflow, o contrato do payload (OBS-01) e as envs.

## 5. Critérios de aceite

1. Import limpo num n8n dev zerado (sem node faltando/versão incompatível).
2. `curl` SEM Bearer → 401; com Bearer certo → `{"ok":true}` em <2s (fan-out não bloqueia a resposta).
3. `severity=critical` → e-mail + telegram + ntfy; `severity=warning` → telegram + ntfy (sem e-mail).
4. Canal com env vazia não derruba os demais (onError continue).
5. `check-vps.sh` (OBS-01) apontado para o webhook entrega alerta ponta-a-ponta na VPS.
6. Workflows existentes intocados (`n8n list:workflow` mostra os antigos ativos como estavam).

## 6. Validação

Passo 4.3 dev é a validação executável. `docker compose config --quiet` valida o YAML dos compose.
Teste de prod fica com o dono (tokens reais).

## 7. Notas de Deploy

- **Migrations:** nenhuma. **Env vars novas (VPS, `.env.production`):** `ALERT_TELEGRAM_BOT_TOKEN`,
  `ALERT_TELEGRAM_CHAT_ID`, `ALERT_NTFY_URL`, `MONITORING_ALERT_EMAIL_TO`. **Rebuild:** nenhum —
  só `up -d n8n` para recriar com as envs.
- Ordem: 4.2 (envs) → `up -d n8n` → import 4.3 → credencial SMTP → ativar → só então configurar a
  URL no `.omni-monitoring.env` (OBS-01).
- Rollback: desativar o workflow no editor (emissor cai no fallback direto automaticamente).

## 8. Arquivos tocados

| Arquivo | Ação |
|---|---|
| `automation/export/workflow-omni-alerts.json` | criar |
| `docker-compose.prod.yml` | editar (envs n8n) |
| `docker-compose.yml` | editar (envs n8n, defaults vazios) |
| `.env.docker.example` / `.env.production.example` | editar |
| `docs/automation/SETUP.md` / `automation/AGENT.md` | editar |

**Conflitos potenciais:** OBS-01 (contrato do payload); OBS-04 (reusa este fan-out via Execute
Workflow — manter o nome do workflow `Omni Alerts` estável).
