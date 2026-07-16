#!/usr/bin/env bash
# check-vps.sh — monitoracao minima do host da VPS Omni (AC-16).
# Roda via cron (*/5) no host. Quiet: so produz output quando ha alerta/erro.
# Config opcional: /home/deploy/.omni-monitoring.env (chmod 600, NUNCA no repo):
#   ALERT_NTFY_URL=https://ntfy.sh/<topico-secreto>
#   ALERT_TELEGRAM_BOT_TOKEN=123456:ABC...
#   ALERT_TELEGRAM_CHAT_ID=123456789
#   DISK_USAGE_MAX=85  MEM_AVAILABLE_MIN_PCT=10  LOAD_PER_CORE_MAX=2
#   ALERT_COOLDOWN_SECONDS=3600  OMNI_API_PORT=18080
#   N8N_COMPOSE_DIR=/home/deploy/lista-atendimento         (onde vive o docker-compose.prod.yml + .env.production)
#   N8N_ENV_FILE=.env.production                            (para `docker compose --env-file`)
#   N8N_CRITICAL_IDS="calendaromni0001 calendarchat0001 calendartrans001 omnichatmvp00001"  (default; = deploy-pull.ps1)
set -u

ENV_FILE="${OMNI_MONITORING_ENV:-/home/deploy/.omni-monitoring.env}"
# shellcheck disable=SC1090
[ -f "$ENV_FILE" ] && . "$ENV_FILE"

DISK_USAGE_MAX="${DISK_USAGE_MAX:-85}"
MEM_AVAILABLE_MIN_PCT="${MEM_AVAILABLE_MIN_PCT:-10}"
LOAD_PER_CORE_MAX="${LOAD_PER_CORE_MAX:-2}"
ALERT_COOLDOWN_SECONDS="${ALERT_COOLDOWN_SECONDS:-3600}"
STATE_DIR="${STATE_DIR:-/home/deploy/.omni-monitoring-state}"
OMNI_API_PORT="${OMNI_API_PORT:-18080}"
HOST_LABEL="${HOST_LABEL:-$(hostname)}"

if ! mkdir -p "$STATE_DIR" 2>/dev/null; then
  STATE_DIR="/tmp/omni-monitoring-state"
  mkdir -p "$STATE_DIR"
fi

# send_alert <chave> <mensagem> — respeita cooldown por chave; envia p/ todos os
# canais configurados; sempre ecoa no stdout (vira log do cron se redirecionado).
send_alert() {
  local key="$1" msg="$2" stamp now last
  stamp="$STATE_DIR/$key.last"
  now=$(date +%s)
  if [ -f "$stamp" ]; then
    last=$(cat "$stamp" 2>/dev/null || echo 0)
    [ $((now - last)) -lt "$ALERT_COOLDOWN_SECONDS" ] && return 0
  fi
  echo "$now" > "$stamp"
  echo "[ALERTA][$HOST_LABEL][$key] $msg"
  if [ -n "${ALERT_TELEGRAM_BOT_TOKEN:-}" ] && [ -n "${ALERT_TELEGRAM_CHAT_ID:-}" ]; then
    curl -sS -m 10 -o /dev/null \
      "https://api.telegram.org/bot${ALERT_TELEGRAM_BOT_TOKEN}/sendMessage" \
      --data-urlencode "chat_id=${ALERT_TELEGRAM_CHAT_ID}" \
      --data-urlencode "text=[Omni VPS ${HOST_LABEL}] ${msg}" \
      || echo "[erro] falha ao enviar alerta telegram (${key})"
  fi
  if [ -n "${ALERT_NTFY_URL:-}" ]; then
    curl -sS -m 10 -o /dev/null -H "Title: Omni VPS ${HOST_LABEL}" \
      -d "${msg}" "$ALERT_NTFY_URL" \
      || echo "[erro] falha ao enviar alerta ntfy (${key})"
  fi
}

# 1) Disco (particao raiz)
disk_pct=$(df -P / | awk 'NR==2 {gsub("%","",$5); print $5}')
if [ "${disk_pct:-0}" -ge "$DISK_USAGE_MAX" ]; then
  send_alert disk "Disco em ${disk_pct}% (limite ${DISK_USAGE_MAX}%). Checar: docker system df; backups antigos em /home/deploy/lista-atendimento/backups."
fi

# 2) RAM disponivel (% de MemAvailable sobre MemTotal)
mem_pct=$(awk '/MemTotal/ {t=$2} /MemAvailable/ {a=$2} END {if (t>0) printf "%d", a*100/t}' /proc/meminfo)
if [ "${mem_pct:-100}" -le "$MEM_AVAILABLE_MIN_PCT" ]; then
  send_alert mem "MemAvailable em ${mem_pct}% (minimo ${MEM_AVAILABLE_MIN_PCT}%). Checar: docker stats --no-stream."
fi

# 3) Load medio 1min por core
cores=$(nproc 2>/dev/null || echo 1)
load1=$(awk '{print $1}' /proc/loadavg)
load_max=$(awk -v c="$cores" -v m="$LOAD_PER_CORE_MAX" 'BEGIN {printf "%.2f", c*m}')
if [ "$(awk -v l="$load1" -v m="$load_max" 'BEGIN {print (l>m) ? 1 : 0}')" = "1" ]; then
  send_alert load "load1=${load1} acima de ${load_max} (${cores} cores x ${LOAD_PER_CORE_MAX}). Checar: top; docker stats."
fi

# 4) Containers unhealthy (inclui a api quando o /healthz devolve 503)
unhealthy=$(docker ps --filter health=unhealthy --format '{{.Names}}' 2>/dev/null | paste -sd' ' -)
if [ -n "${unhealthy:-}" ]; then
  send_alert unhealthy "Containers unhealthy: ${unhealthy}"
fi

# 5) Containers em crash-loop
restarting=$(docker ps --filter status=restarting --format '{{.Names}}' 2>/dev/null | paste -sd' ' -)
if [ -n "${restarting:-}" ]; then
  send_alert restarting "Containers em restart-loop: ${restarting}"
fi

# 6) healthz da api direto na porta local (nao depende do Caddy compartilhado)
api_code=$(curl -sS -m 10 -o /dev/null -w '%{http_code}' \
  "http://127.0.0.1:${OMNI_API_PORT}/healthz" 2>/dev/null || echo 000)
if [ "$api_code" != "200" ]; then
  send_alert healthz "GET 127.0.0.1:${OMNI_API_PORT}/healthz => ${api_code} (200=ok; 503=banco fora; 000=api fora)."
fi

# 8) Saude do n8n: container no ar + workflows criticos ATIVOS.
#    A lista N8N_CRITICAL_IDS e a MESMA de scripts/deploy/deploy-pull.ps1:246
#    (contrato: mudou uma, muda a outra). Le o estado real via `n8n export:workflow`
#    (respeita o WAL do SQLite; `docker cp database.sqlite` NAO leva writes recentes).
#    O 3o arg de send_alert (critical/warning) e a severidade do OBS-01; enquanto
#    OBS-01 nao entrar, o send_alert atual (2 args) so ignora o 3o, sem quebrar.
N8N_COMPOSE_DIR="${N8N_COMPOSE_DIR:-/home/deploy/lista-atendimento}"
N8N_ENV_FILE="${N8N_ENV_FILE:-.env.production}"
N8N_CRITICAL_IDS="${N8N_CRITICAL_IDS:-calendaromni0001 calendarchat0001 calendartrans001 omnichatmvp00001}"
n8n_compose="docker compose --env-file $N8N_ENV_FILE -f docker-compose.prod.yml --profile automation"

if [ -d "$N8N_COMPOSE_DIR" ]; then
  # 8a) container rodando?
  n8n_state=$(cd "$N8N_COMPOSE_DIR" && $n8n_compose ps -q n8n 2>/dev/null)
  if [ -z "$n8n_state" ]; then
    send_alert n8n "Container n8n NAO esta rodando (profile automation). Calendario/Omni Chat parados. Checar: cd $N8N_COMPOSE_DIR && $n8n_compose ps n8n; logs n8n." critical
  else
    n8n_health=$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "$n8n_state" 2>/dev/null || echo unknown)
    if [ "$n8n_health" = "unhealthy" ]; then
      send_alert n8n "Container n8n UNHEALTHY. Checar: docker logs $n8n_state --tail=50." critical
    else
      # 8b) workflows criticos ativos? (so checa se o container respondeu)
      #     WANT vai via `exec -e` (env DENTRO do container). NAO colocar WANT=... depois
      #     da string do sh -lc: viraria $0 do shell interno e nunca chegaria ao node
      #     (bug: process.env.WANT undefined => .split() TypeError => sempre __EXPORT_FAIL__).
      inactive=$(cd "$N8N_COMPOSE_DIR" && $n8n_compose exec -T -e "WANT=$N8N_CRITICAL_IDS" n8n sh -lc \
        "n8n export:workflow --all --output=/tmp/obs7_wf.json >/dev/null 2>&1 && node -e '
          const fs=require(\"fs\");
          const want=(process.env.WANT||\"\").split(/\s+/).filter(Boolean);
          let list=[]; try{const r=JSON.parse(fs.readFileSync(\"/tmp/obs7_wf.json\",\"utf8\"));list=Array.isArray(r)?r:(r.data||[]);}catch(e){process.exit(3);}
          const byId=Object.fromEntries(list.map(w=>[String(w.id),w]));
          const bad=want.filter(id=>!byId[id] || byId[id].active!==true);
          process.stdout.write(bad.join(\" \"));
        '" 2>/dev/null || echo "__EXPORT_FAIL__")
      if [ "$inactive" = "__EXPORT_FAIL__" ]; then
        send_alert n8n "Nao consegui ler os workflows do n8n (export falhou). Container up mas CLI/Node com problema? Checar: $n8n_compose exec n8n n8n export:workflow --all." warning
      elif [ -n "$inactive" ]; then
        send_alert n8n "Workflow(s) critico(s) INATIVO(s) no n8n: ${inactive}. Reativar: $n8n_compose exec n8n n8n update:workflow --id=<id> --active=true (ou re-deploy com -DeployAutomation)." warning
      fi
    fi
  fi
fi

exit 0
