#!/usr/bin/env bash
# Sobe os dados + imagens do Mostarda Bar Bistro para a PRODUCAO (VPS lista-atendimento,
# conta Crow em prod) e liga o front TAVOLA (https://mostarda.crowvisuals.com.br).
#
# Rode da maquina LOCAL no Git Bash:   bash scripts/deploy/upload-mostarda-prod.sh
# Nao builda imagem: so transfere dados/imagens e RECRIA o container api com a env
# PUBLIC_API_BASE_URL (necessaria pras imagens absolutizarem no front em outro dominio).
# Idempotente: pode rodar de novo (limpa o restaurante e recarrega; upsert no dominio).
#
# Pre-requisitos:
#   - Docker Desktop com o stack LOCAL de dev up (postgres local serve a fonte unica)
#   - Chave SSH ~/.ssh/gh_actions_omnichannel_vps
#   - Imagens em C:/tmp/mostarda_jpg (203 .jpg). Ver docs/cardapio/DEPLOY_MOSTARDA_PROD.md
set -euo pipefail

# ---------- parametros (PROD) ----------
CROW_PROD="0d88ff7b-b274-4a0f-83c0-972983e9a081"   # conta Crow Visuals em PROD
LOCAL_ACCT="80caf5d5-6e81-4763-8373-2d563e1a1988"  # conta Crow LOCAL (sera re-targetada)
REST_ID="b1b1b1b1-1111-4111-8111-111111111111"     # id do restaurante (estavel local/prod)
SLUG="mk"
HOST="mostarda.crowvisuals.com.br"                 # dominio do front TAVOLA
PUBLIC_BASE="https://omni.crowvisuals.com.br"      # host publico do api (serve /v1 e /uploads)
IMAGES_DIR="/c/tmp/mostarda_jpg"
DUMP="/c/tmp/mostarda_prod.sql"

VPS="85.31.62.33"; SSH_USER="deploy"; PORT="22"
KEY="$HOME/.ssh/gh_actions_omnichannel_vps"
REMOTE_PATH="/home/deploy/lista-atendimento"
ENV_FILE=".env.production"

SSH_EXE="/c/Windows/System32/OpenSSH/ssh.exe"
SCP_EXE="/c/Windows/System32/OpenSSH/scp.exe"
SSH_ARGS=(-i "$KEY" -o StrictHostKeyChecking=accept-new -o BatchMode=yes -o ConnectTimeout=30 -p "$PORT")
TARGET="$SSH_USER@$VPS"

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
COMPOSE_LOCAL="$REPO_DIR/docker-compose.prod.yml"
DC="docker compose --env-file $ENV_FILE -f docker-compose.prod.yml"
UPLOAD_DIR="/app/data/uploads/cardapio/$CROW_PROD/mostarda"

say(){ printf '\n\033[1;36m==> %s\033[0m\n' "$*"; }
remote(){ "$SSH_EXE" "${SSH_ARGS[@]}" "$TARGET" "$@"; }
# psql no container postgres da VPS, lendo SQL do stdin
remote_psql(){ remote "cd $REMOTE_PATH && $DC exec -T postgres sh -lc 'psql -U \$POSTGRES_USER -d \$POSTGRES_DB $*'"; }

# ---------- 0. pre-checagens ----------
[ -f "$KEY" ]          || { echo "ERRO: chave SSH nao encontrada em $KEY"; exit 1; }
[ -d "$IMAGES_DIR" ]   || { echo "ERRO: pasta de imagens nao encontrada em $IMAGES_DIR"; exit 1; }
[ -f "$COMPOSE_LOCAL" ]|| { echo "ERRO: docker-compose.prod.yml nao encontrado em $REPO_DIR"; exit 1; }
IMG_COUNT="$(ls -1 "$IMAGES_DIR"/*.jpg 2>/dev/null | wc -l | tr -d ' ')"
echo "Imagens locais: $IMG_COUNT jpg | conta prod: $CROW_PROD | restaurante: $SLUG"

# ---------- 1. dump re-targetado do banco LOCAL (fonte unica) ----------
say "1/6 Gerando dump do banco local e re-targetando ($LOCAL_ACCT -> $CROW_PROD)"
docker compose -f "$REPO_DIR/docker-compose.yml" exec -T postgres sh -lc \
  'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --data-only --no-owner --no-privileges -t cardapio.restaurants -t cardapio.categories -t cardapio.products -t cardapio.delivery_zones' \
  | sed "s/$LOCAL_ACCT/$CROW_PROD/g" > "$DUMP"
[ "$(grep -c "$LOCAL_ACCT" "$DUMP")" = "0" ] || { echo "ERRO: id local ainda presente no dump"; exit 1; }
echo "Dump: $(wc -l < "$DUMP") linhas | blocos COPY: $(grep -c '^COPY cardapio' "$DUMP")"

# ---------- 2. carregar no prod (limpa + COPY, atomico e re-rodavel) ----------
say "2/6 Carregando dados no prod (restore atomico)"
{
  echo "delete from cardapio.products      where restaurant_id='$REST_ID';"
  echo "delete from cardapio.delivery_zones where restaurant_id='$REST_ID';"
  echo "delete from cardapio.categories     where restaurant_id='$REST_ID';"
  echo "delete from cardapio.restaurants    where id='$REST_ID';"
  cat "$DUMP"
} | remote_psql "--single-transaction -v ON_ERROR_STOP=1"
echo "OK restore."

say "Contagens em prod"
printf '%s\n' "select 'restaurants', count(*) from cardapio.restaurants where id='$REST_ID' \
  union all select 'categories', count(*) from cardapio.categories where restaurant_id='$REST_ID' \
  union all select 'zones', count(*) from cardapio.delivery_zones where restaurant_id='$REST_ID' \
  union all select 'products', count(*) from cardapio.products where restaurant_id='$REST_ID' \
  union all select 'sem_img', count(*) from cardapio.products where restaurant_id='$REST_ID' and coalesce(image_url,'')='';" \
  | remote_psql "-tA"

# ---------- 3. copiar imagens pro volume api_uploads ----------
say "3/6 Copiando $IMG_COUNT imagens -> $UPLOAD_DIR"
tar -cf - -C "$IMAGES_DIR" . \
  | remote "cd $REMOTE_PATH && $DC exec -T api sh -c 'mkdir -p $UPLOAD_DIR && tar -xf - -C $UPLOAD_DIR && echo no-volume: \$(ls -1 $UPLOAD_DIR | wc -l) arquivos'"

# ---------- 4. PUBLIC_API_BASE_URL + recriar api ----------
say "4/6 Setando PUBLIC_API_BASE_URL e recriando o api"
"$SCP_EXE" -i "$KEY" -o StrictHostKeyChecking=accept-new -P "$PORT" "$COMPOSE_LOCAL" "$TARGET:$REMOTE_PATH/docker-compose.prod.yml"
remote "bash -s" <<REMOTE
set -e
cd $REMOTE_PATH
if grep -q '^PUBLIC_API_BASE_URL=' $ENV_FILE; then
  sed -i 's|^PUBLIC_API_BASE_URL=.*|PUBLIC_API_BASE_URL=$PUBLIC_BASE|' $ENV_FILE
else
  printf 'PUBLIC_API_BASE_URL=%s\n' '$PUBLIC_BASE' >> $ENV_FILE
fi
echo "env -> \$(grep '^PUBLIC_API_BASE_URL=' $ENV_FILE)"
$DC up -d --no-build api
$DC ps --format '{{.Name}}  {{.State}}' | grep -i api || true
REMOTE

# ---------- 5. registrar dominio (idempotente) ----------
say "5/6 Registrando dominio $HOST (primario)"
printf '%s\n' "insert into cardapio.restaurant_domains (host, restaurant_id, account_id, is_primary, created_at) \
  values ('$HOST','$REST_ID','$CROW_PROD',true,now()) \
  on conflict (host) do update set restaurant_id=excluded.restaurant_id, account_id=excluded.account_id, is_primary=excluded.is_primary;" \
  | remote_psql "-v ON_ERROR_STOP=1"

# ---------- 6. smoke ----------
say "6/6 Smoke (espera o api subir, depois testa publico + 1 imagem)"
printf 'aguardando healthz'
for _ in $(seq 1 20); do
  c="$(curl -sS -o /dev/null -w '%{http_code}' --max-time 8 "$PUBLIC_BASE/healthz" 2>/dev/null || true)"
  [ "$c" = "200" ] && { printf ' OK\n'; break; }
  printf '.'; sleep 3
done
IMG_URL="$(curl -sS --max-time 15 "$PUBLIC_BASE/v1/public/restaurants/$SLUG" | grep -oE 'https://[^"]+/uploads/cardapio/[^"]+\.(jpg|jpeg|png|webp)' | head -1 || true)"
echo "1a imagem do publico: ${IMG_URL:-(nenhuma — confira PUBLIC_API_BASE_URL/restart)}"
[ -n "${IMG_URL:-}" ] && { printf 'GET imagem -> '; curl -sS -o /dev/null -w '%{http_code}\n' --max-time 15 "$IMG_URL"; }

say "Concluido."
echo "  publico : $PUBLIC_BASE/v1/public/restaurants/$SLUG"
echo "  front   : https://$HOST/   (ou https://$HOST/?slug=$SLUG sem depender do dominio)"
