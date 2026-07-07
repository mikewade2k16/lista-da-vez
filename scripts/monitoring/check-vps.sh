#!/usr/bin/env bash
# check-vps.sh — monitoracao minima do host da VPS Omni (AC-16).
# Roda via cron (*/5) no host. Quiet: so produz output quando ha alerta/erro.
# Config opcional: /home/deploy/.omni-monitoring.env (chmod 600, NUNCA no repo):
#   ALERT_NTFY_URL=https://ntfy.sh/<topico-secreto>
#   ALERT_TELEGRAM_BOT_TOKEN=123456:ABC...
#   ALERT_TELEGRAM_CHAT_ID=123456789
#   DISK_USAGE_MAX=85  MEM_AVAILABLE_MIN_PCT=10  LOAD_PER_CORE_MAX=2
#   ALERT_COOLDOWN_SECONDS=3600  OMNI_API_PORT=18080
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

exit 0
