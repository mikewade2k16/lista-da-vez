#!/usr/bin/env bash
# backup-db.sh — backup diario do Postgres do Omni (roda NA VPS, user deploy).
# Fonte de verdade: repo scripts/backup/backup-db.sh; instalado em
# /home/deploy/lista-atendimento/scripts/backup-db.sh (sincronizado pelo
# workflow backup-check.yml a cada execucao). Runbook: docs/BACKUP_RESTORE.md.
#
# `set -o pipefail` (via -euo pipefail) e' ESSENCIAL: sem ele o gzip mascara um
# pg_dump que falhou e o "backup" sai vazio (mesma licao do deploy-ship.ps1).
set -euo pipefail

COMPOSE_DIR="${BACKUP_COMPOSE_DIR:-/home/deploy/lista-atendimento}"
ENV_FILE="${BACKUP_ENV_FILE:-.env.production}"
BACKUP_DIR="${BACKUP_DIR:-$COMPOSE_DIR/backups}"
KEEP_DAILY_DAYS="${BACKUP_KEEP_DAILY_DAYS:-7}"    # 7 diarios
KEEP_WEEKLY_DAYS="${BACKUP_KEEP_WEEKLY_DAYS:-27}" # ~4 semanais
MIN_BYTES="${BACKUP_MIN_BYTES:-10240}"            # dump gz < 10KB = suspeito de vazio
STATUS_FILE="$BACKUP_DIR/last_backup_status"

# Config off-site/alerta fora do repo (na VPS, chmod 600). Pode definir:
#   BACKUP_RCLONE_REMOTE=offsite:omni-db-backups   (remote do rclone; vazio = pula off-site)
#   BACKUP_ALERT_URL=https://ntfy.sh/<topico>      (POST simples em falha; opcional)
[ -f "$COMPOSE_DIR/.backup.env" ] && . "$COMPOSE_DIR/.backup.env"
RCLONE_REMOTE="${BACKUP_RCLONE_REMOTE:-}"
ALERT_URL="${BACKUP_ALERT_URL:-}"

mkdir -p "$BACKUP_DIR/daily" "$BACKUP_DIR/weekly"

fail() {
  printf 'fail %s %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$1" > "$STATUS_FILE"
  if [ -n "$ALERT_URL" ]; then
    curl -fsS -m 10 -d "Omni backup FALHOU: $1" "$ALERT_URL" >/dev/null 2>&1 || true
  fi
  echo "ERRO: $1" >&2
  exit 1
}

exec 9>"$BACKUP_DIR/.backup.lock"
flock -n 9 || fail "outra execucao em andamento (lock)"

cd "$COMPOSE_DIR" || fail "COMPOSE_DIR inexistente: $COMPOSE_DIR"
[ -f "$ENV_FILE" ] || fail "env file ausente: $COMPOSE_DIR/$ENV_FILE"

stamp="$(date +%Y%m%d_%H%M%S)"
out="$BACKUP_DIR/daily/backup_${stamp}.sql.gz"

# Mesmo padrao de dump do deploy (deploy-ship.ps1:87): credenciais vem do env
# do proprio container postgres — nada de senha neste script.
if ! docker compose --env-file "$ENV_FILE" -f docker-compose.prod.yml exec -T postgres \
    sh -lc 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB"' | gzip > "$out"; then
  rm -f "$out"
  fail "pg_dump/gzip retornou erro"
fi

gzip -t "$out" || { rm -f "$out"; fail "arquivo gzip corrompido: $out"; }
bytes="$(stat -c%s "$out")"
[ "$bytes" -ge "$MIN_BYTES" ] || { rm -f "$out"; fail "dump suspeito de vazio (${bytes}B < ${MIN_BYTES}B)"; }

# Copia semanal aos domingos (date +%u: 7 = domingo)
if [ "$(date +%u)" = "7" ]; then
  cp "$out" "$BACKUP_DIR/weekly/backup_${stamp}.sql.gz"
fi

# Retencao local: 7 diarios, 4 semanais; tambem poda dumps antigos do fluxo
# on-demand de deploy que caem soltos em backups/ (mantidos 7 dias).
find "$BACKUP_DIR/daily"  -name 'backup_*.sql.gz' -mtime +"$KEEP_DAILY_DAYS"  -delete
find "$BACKUP_DIR/weekly" -name 'backup_*.sql.gz' -mtime +"$KEEP_WEEKLY_DAYS" -delete
find "$BACKUP_DIR" -maxdepth 1 -name 'backup_*.sql.gz' -mtime +"$KEEP_DAILY_DAYS" -delete

# Off-site (opcional-mas-recomendado): bucket S3-compativel via rclone.
if [ -n "$RCLONE_REMOTE" ]; then
  command -v rclone >/dev/null 2>&1 || fail "BACKUP_RCLONE_REMOTE definido mas rclone nao instalado"
  rclone copyto "$out" "$RCLONE_REMOTE/daily/$(basename "$out")" \
    || fail "rclone daily falhou (backup local OK em $out)"
  if [ "$(date +%u)" = "7" ]; then
    rclone copyto "$out" "$RCLONE_REMOTE/weekly/backup_${stamp}.sql.gz" \
      || fail "rclone weekly falhou (backup local OK em $out)"
  fi
  rclone delete --min-age "$((KEEP_DAILY_DAYS + 1))d" "$RCLONE_REMOTE/daily"  || true
  rclone delete --min-age 60d                          "$RCLONE_REMOTE/weekly" || true
fi

printf 'ok %s %s %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$out" "$bytes" > "$STATUS_FILE"
echo "backup OK: $out (${bytes} bytes)"
