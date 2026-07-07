# OBS-01 — check-vps.sh dispara webhook n8n (+ severity + check de backup)

> Spec de implementação · Prioridade **P1** · Esforço **S** · Impacto **alto**
> Origem: evolução do AC-16 (diagnóstico 2026-07) · roadmap `observabilidade-n8n` → task `obs-01-check-vps-webhook-n8n`

## 1. Contexto

**O achado:** o `scripts/monitoring/check-vps.sh` (cron */5 na VPS) alerta hoje por 2 canais
diretos (Telegram/ntfy) configurados em `/home/deploy/.omni-monitoring.env`. Não existe canal n8n —
o fan-out inteligente (OBS-02) não tem como receber os eventos. Além disso o script não vigia o
resultado do backup (`backups/last_backup_status`, escrito pelo `backup-db.sh`).

Evidências (arquivo lido em 03/07):
- `scripts/monitoring/check-vps.sh:31-53` — `send_alert <chave> <mensagem>`: cooldown 1h por chave
  (`$STATE_DIR/$key.last`), echo no stdout, Telegram (`curl api.telegram.org`, linhas 41-47) e ntfy
  (linhas 48-52). Sem severidade, sem n8n.
- `scripts/monitoring/check-vps.sh:55-92` — 6 checks: disk/mem/load/unhealthy/restarting/healthz.
- `docker-compose.prod.yml` serviço n8n publica porta no HOST em loopback
  (`127.0.0.1:${AUTOMATION_N8N_PORT:-15680}:5678`) — o script pode POSTar sem passar pelo
  Caddy/basic_auth. **CONFERIR o valor real na VPS antes: `grep AUTOMATION_N8N_PORT .env.production`
  (default 15680; se o env define outro, usar o definido).**
- `scripts/backup/backup-db.sh:17,29,82` — status file `ok|fail <ISO-ts> <detalhe>`.

## 2. Objetivo e não-objetivos

**Objetivo (escopo fechado):**
1. `send_alert` ganha um 3º canal: POST JSON no webhook n8n (`ALERT_N8N_WEBHOOK_URL`) com Bearer
   (`ALERT_N8N_TOKEN`). Se o n8n aceitar (2xx), os canais diretos NÃO disparam (o fan-out passa a
   ser do n8n — OBS-02); se falhar, fallback nos canais atuais (resiliência: o n8n pode estar caído
   junto com o incidente).
2. `send_alert` ganha severidade (`warning` default / `critical`), enviada no payload.
3. Check novo #7: status + frescor do backup (`last_backup_status`) e do drill (`last_restore_drill`, AC-05b).

**Não-objetivos (FORA):**
- NÃO criar o workflow n8n (é o OBS-02).
- NÃO mudar cooldown/checks existentes além de adicionar o argumento de severidade.
- NÃO colocar secrets no repo (config continua em `.omni-monitoring.env` na VPS).

## 3. Regras de execução (obrigatórias)

- NENHUM comando git. Editar o script no repo; a instalação na VPS é `scp` + teste (runbook §6).
- Bash puro POSIX-ish (o script roda em Ubuntu 24.04; manter o estilo atual, `set -u`).
- Nunca inventar tokens: `ALERT_N8N_TOKEN` = o `AUTOMATION_RUNTIME_TOKEN` que JÁ existe no
  `.env.production` da VPS (o dono cola o valor no `.omni-monitoring.env`).
- Atualizar `scripts/monitoring/AGENT.md` + `docs/DEPLOY_VPS.md → Monitoração`.

## 4. Mudanças (passo a passo)

### 4.1 EDITAR `scripts/monitoring/check-vps.sh` — header de config

Adicionar às linhas de exemplo do header (após `ALERT_TELEGRAM_CHAT_ID`):

```bash
#   ALERT_N8N_WEBHOOK_URL=http://127.0.0.1:15680/webhook/omni-monitoring   (porta host do n8n; ver AUTOMATION_N8N_PORT)
#   ALERT_N8N_TOKEN=<AUTOMATION_RUNTIME_TOKEN do .env.production>
#   BACKUP_STATUS_FILE=/home/deploy/lista-atendimento/backups/last_backup_status
```

### 4.2 EDITAR `scripts/monitoring/check-vps.sh` — `json_escape` + novo `send_alert`

Inserir ANTES de `send_alert` (linha ~29):

```bash
# json_escape <str> — escapa \ e " e achata quebras de linha (payload de 1 linha).
json_escape() {
  local s="$1"
  s="${s//\\/\\\\}"
  s="${s//\"/\\\"}"
  s="${s//$'\n'/ }"
  printf '%s' "$s"
}
```

Substituir a função `send_alert` inteira (linhas 31-53 atuais) por:

```bash
# send_alert <chave> <mensagem> [severity] — cooldown por chave; tenta o webhook
# n8n primeiro (fan-out centralizado, OBS-02); se o n8n nao aceitar, cai nos
# canais diretos (telegram/ntfy) como fallback. severity: warning (default) | critical.
send_alert() {
  local key="$1" msg="$2" severity="${3:-warning}" stamp now last payload n8n_code
  stamp="$STATE_DIR/$key.last"
  now=$(date +%s)
  if [ -f "$stamp" ]; then
    last=$(cat "$stamp" 2>/dev/null || echo 0)
    [ $((now - last)) -lt "$ALERT_COOLDOWN_SECONDS" ] && return 0
  fi
  echo "$now" > "$stamp"
  echo "[ALERTA][$HOST_LABEL][$severity][$key] $msg"
  if [ -n "${ALERT_N8N_WEBHOOK_URL:-}" ]; then
    payload=$(printf '{"host":"%s","key":"%s","msg":"%s","severity":"%s","ts":"%s"}' \
      "$(json_escape "$HOST_LABEL")" "$(json_escape "$key")" "$(json_escape "$msg")" \
      "$severity" "$(date -u +%Y-%m-%dT%H:%M:%SZ)")
    n8n_code=$(curl -sS -m 10 -o /dev/null -w '%{http_code}' \
      -H "Content-Type: application/json" \
      -H "Authorization: Bearer ${ALERT_N8N_TOKEN:-}" \
      -d "$payload" "$ALERT_N8N_WEBHOOK_URL" 2>/dev/null || echo 000)
    case "$n8n_code" in
      2*) return 0 ;;  # n8n recebeu: fan-out e' dele (OBS-02); nao duplicar
      *)  echo "[erro] webhook n8n (${key}) => ${n8n_code}; usando canais diretos" ;;
    esac
  fi
  if [ -n "${ALERT_TELEGRAM_BOT_TOKEN:-}" ] && [ -n "${ALERT_TELEGRAM_CHAT_ID:-}" ]; then
    curl -sS -m 10 -o /dev/null \
      "https://api.telegram.org/bot${ALERT_TELEGRAM_BOT_TOKEN}/sendMessage" \
      --data-urlencode "chat_id=${ALERT_TELEGRAM_CHAT_ID}" \
      --data-urlencode "text=[Omni VPS ${HOST_LABEL}][${severity}] ${msg}" \
      || echo "[erro] falha ao enviar alerta telegram (${key})"
  fi
  if [ -n "${ALERT_NTFY_URL:-}" ]; then
    curl -sS -m 10 -o /dev/null -H "Title: Omni VPS ${HOST_LABEL} [${severity}]" \
      -d "${msg}" "$ALERT_NTFY_URL" \
      || echo "[erro] falha ao enviar alerta ntfy (${key})"
  fi
}
```

### 4.3 EDITAR os 6 checks — severidade explícita

Trocar as chamadas (mantendo as mensagens atuais): `disk`/`mem`/`load`/`unhealthy` ganham 3º arg
`warning`; `restarting` e `healthz` ganham `critical`.

### 4.4 EDITAR — check #7 (backup + drill), antes do `exit 0` final

```bash
# 7) Backup diario: falha registrada OU stale (>26h sem rodar = cron parado).
BACKUP_STATUS_FILE="${BACKUP_STATUS_FILE:-/home/deploy/lista-atendimento/backups/last_backup_status}"
if [ -f "$BACKUP_STATUS_FILE" ]; then
  b_word=$(awk '{print $1}' "$BACKUP_STATUS_FILE" 2>/dev/null || echo "")
  b_age=$(( $(date +%s) - $(stat -c %Y "$BACKUP_STATUS_FILE" 2>/dev/null || echo 0) ))
  if [ "$b_word" = "fail" ]; then
    send_alert backup "Backup do banco FALHOU: $(head -c 300 "$BACKUP_STATUS_FILE")" critical
  elif [ "$b_age" -gt $((26 * 3600)) ]; then
    send_alert backup "Backup do banco nao roda ha $((b_age / 3600))h (cron parado?)." critical
  fi
else
  send_alert backup "Status do backup ausente (${BACKUP_STATUS_FILE}) — backup nunca rodou?" critical
fi

# 7b) Drill de restore (AC-05b): so vigia DEPOIS que o drill existir (arquivo presente).
DRILL_STATUS_FILE="${DRILL_STATUS_FILE:-/home/deploy/lista-atendimento/backups/last_restore_drill}"
if [ -f "$DRILL_STATUS_FILE" ]; then
  d_word=$(awk '{print $1}' "$DRILL_STATUS_FILE" 2>/dev/null || echo "")
  d_age=$(( $(date +%s) - $(stat -c %Y "$DRILL_STATUS_FILE" 2>/dev/null || echo 0) ))
  if [ "$d_word" = "fail" ]; then
    send_alert restore_drill "Drill de restore FALHOU: $(head -c 300 "$DRILL_STATUS_FILE")" critical
  elif [ "$d_age" -gt $((32 * 24 * 3600)) ]; then
    send_alert restore_drill "Drill de restore nao roda ha $((d_age / 86400)) dias." warning
  fi
fi
```

### 4.5 EDITAR docs

- `scripts/monitoring/AGENT.md`: 3º canal (n8n-first com fallback), severidades, checks 7/7b, envs novas.
- `docs/DEPLOY_VPS.md → Monitoração` passo 3: as 3 linhas novas de env no `.omni-monitoring.env`.

## 5. Critérios de aceite

1. `bash -n scripts/monitoring/check-vps.sh` limpo (sintaxe).
2. Sem `ALERT_N8N_WEBHOOK_URL` definido → comportamento IDÊNTICO ao atual (canais diretos), com
   `[severity]` no texto.
3. Com URL válida (OBS-02 no ar): alerta chega SÓ via n8n (telegram direto não dispara em 2xx).
4. Com URL inválida: `[erro] webhook n8n ... usando canais diretos` no stdout + alerta chega pelo fallback.
5. `last_backup_status` com `fail` na 1ª palavra → alerta `backup` critical; arquivo com mtime
   envelhecido artificialmente (`touch -d '30 hours ago'`) → alerta de staleness.
6. Cooldown continua funcionando por chave (2ª execução em <1h não re-alerta).

## 6. Validação

Local (Git Bash): `bash -n` + rodar com env fake:

```bash
STATE_DIR=/tmp/omni-mon-test OMNI_MONITORING_ENV=/tmp/omni-mon.env \
BACKUP_STATUS_FILE=/tmp/fake_backup_status bash scripts/monitoring/check-vps.sh
```

Na VPS (dono roda): scp para `/home/deploy/monitoring/check-vps.sh`, editar
`.omni-monitoring.env`, executar 1x manual e conferir os cenários 3/4/5 acima.

## 7. Notas de Deploy

- Nenhuma migration/rebuild. Host-side: scp do script + 3 linhas no `.omni-monitoring.env`
  (`ALERT_N8N_WEBHOOK_URL`, `ALERT_N8N_TOKEN`, opcional `BACKUP_STATUS_FILE`).
- Ordem: pode subir ANTES do OBS-02 (sem URL configurada nada muda); a URL só entra no env quando o
  workflow do OBS-02 estiver importado e ativo.
- Rollback: remover as linhas do env (canal n8n desliga sozinho).

## 8. Arquivos tocados

| Arquivo | Ação |
|---|---|
| `scripts/monitoring/check-vps.sh` | editar (send_alert, severities, checks 7/7b) |
| `scripts/monitoring/AGENT.md` | editar |
| `docs/DEPLOY_VPS.md` | editar (§ Monitoração) |

**Conflitos potenciais:** OBS-02 (consome o payload — o shape
`{host,key,msg,severity,ts}` é contrato entre os dois; mudar um = mudar o outro). AC-05b (check 7b
só alerta após o primeiro drill).
