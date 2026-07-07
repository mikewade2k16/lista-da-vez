# AC-05 — Backup agendado do Postgres + retenção + off-site + teste de restore

> Spec de implementação. Prioridade **P1**, esforço S, impacto alto.
> Achado canônico AC-05 do diagnóstico 2026-07-02 (fatos.json → achados_canonicos.AC-05).

## 1. Contexto

**O achado:** o backup do banco de produção existe SÓ on-demand, acoplado ao deploy:

- `scripts/deploy/deploy-ship.ps1:87` — monta `pg_dump | gzip` dentro do container postgres
  apenas quando o operador passa `-BackupDatabase`;
- `.github/workflows/deploy-vps.yml:131-145` — step "Backup remote database" só roda no
  `workflow_dispatch` com input `backup_database=true`;
- `docs/DEPLOY_VPS.md:252-258` — seção "Backup minimo" apenas lista o que deveria ser
  protegido (dump + volume `api_uploads` + `.env.production`), sem procedimento agendado;
- dumps ficam em `/home/deploy/lista-atendimento/backups/` **na mesma VPS** (85.31.62.33),
  sem retenção (crescem para sempre), sem cópia off-site e sem teste de restore.

**Por que importa:** se não houver deploy, não há backup — dias de dados de produção
(operação de fila, CRM, cardápio com pedidos reais da Mostarda, calendário) podem se perder.
Se o disco/VPS morrer, TODOS os backups morrem junto (mesmo host). Nunca foi validado que
um dump restaura de fato.

**Padrão atual de dump a reaproveitar** (deploy-ship.ps1:87 / deploy-vps.yml:139-141):

```bash
docker compose --env-file .env.production -f docker-compose.prod.yml exec -T postgres \
  sh -lc 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB"' | gzip > backups/backup_$(date +%Y%m%d_%H%M%S).sql.gz
```

Fatos da infra que a solução respeita: Postgres 16-alpine no serviço `postgres` do
`docker-compose.prod.yml` (sem porta no host, credenciais só via env do container — o script
NÃO precisa de senha); prod em `/home/deploy/lista-atendimento` com `.env.production`;
user SSH `deploy`; a VPS nunca compila nada; secret `DEPLOY_VPS_SSH_KEY` já existe no GitHub
(usado por `deploy-vps.yml:94`); ERP sync automático às 04:00 UTC (`ERP_SYNC_HOUR_UTC=4`).

## 2. Objetivo e não-objetivos

**Objetivo (escopo fechado):**

1. Script bash versionado `scripts/backup/backup-db.sh` que roda NA VPS: pg_dump via
   `docker compose exec` + gzip + verificação de integridade + retenção local
   (7 diários / 4 semanais) + off-site opcional via rclone + arquivo de status.
2. Agendamento primário: **cron do host da VPS** (user `deploy`) — funciona sem GitHub.
3. Camada extra + alerta: workflow agendado `.github/workflows/backup-check.yml` que SSHa,
   sincroniza o script, confere o status do último backup e roda fallback se estiver
   ausente/velho/falho; workflow vermelho = e-mail automático do GitHub (alerta mínimo viável).
4. Off-site: rclone para bucket S3-compatível — opcional-mas-recomendado, credenciais FORA
   do repo (arquivo `.backup.env` na VPS, chmod 600).
5. Runbook `docs/BACKUP_RESTORE.md` com instalação, teste de restore mensal (banco
   temporário + contagem de tabelas) e procedimento de restore real.

**Não-objetivos (explicitamente NÃO fazer):**

- NÃO remover o backup on-demand dos scripts de deploy (`-BackupDatabase` /
  `backup_database`) — features coexistem; o backup pré-deploy continua sendo a proteção
  de release de schema.
- NÃO mexer no Go/`healthz` para expor status de backup — o container da api não enxerga o
  filesystem do host; o alerta mínimo viável decidido é status-file + workflow agendado
  (decisão fechada, não reabrir).
- NÃO fazer backup do volume `api_uploads` neste AC (fica documentado como pendência no
  runbook; escopo aqui é o banco).
- NÃO criar migration, NÃO tocar em `back/`, `web/`, compose files.
- NÃO instalar nada na VPS nesta tarefa (instalação é do usuário — ver Notas de Deploy).
- NÃO colocar credencial nenhuma no repo (nem exemplo com valor real).
- NÃO tocar em `roadmap-data.ts` (evita conflito com outros ACs em paralelo).

## 3. Regras de execução (obrigatórias para o implementador)

- **NENHUM comando git** (sessão multi-agente — só o usuário roda git).
- **NÃO rodar npm/build/generate.** Este AC não toca `back/` → NÃO precisa de
  `docker compose up -d --build api`. Validação é só sintática (seção 6).
- **NÃO executar nada na VPS** (sem ssh/scp reais). Os comandos remotos vão para docs/Notas
  de Deploy, para o usuário rodar.
- Máx 450 linhas por arquivo novo.
- Não remover funcionalidade existente (backup on-demand do deploy fica intacto).
- Zero mock/legado novo; nada a registrar em `docs/LEGADO.md`.
- NUNCA sobrescrever password_hash/dados de usuário; o runbook de restore restaura SEMPRE
  em banco temporário, nunca por cima do banco `omni` sem ordem explícita do usuário.
- Atualizar AGENT.md da pasta tocada (criar `scripts/backup/AGENT.md`) e referenciar no
  `scripts/deploy/AGENT.md` (seção Referencias).
- Portas fixas intocadas (api 9091 host dev, web 3003, postgres 5432).

## 4. Mudanças

### 4.1 CRIAR `scripts/backup/backup-db.sh` (novo, ~90 linhas)

Bash para Ubuntu 24.04 (host da VPS). Conteúdo exato:

```bash
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
```

Decisões já tomadas (não reabrir):

- **Formato do dump:** SQL plano gzipado (`pg_dump | gzip`), idêntico ao fluxo de deploy —
  restaura com `psql`, sem depender de versão de `pg_restore`.
- **Status file:** 1 linha `ok|fail <UTC ISO8601> <arquivo-ou-motivo> [bytes]` em
  `backups/last_backup_status`. É a interface do alerta (4.2).
- **Falha de off-site = falha do backup** (status `fail`, exit 1) mas o arquivo local é
  preservado — a mensagem diz "backup local OK".
- **`flock`** impede sobreposição cron × workflow fallback.
- **Sem credencial no script**: Postgres via env do container; S3 via config do rclone na VPS.

### 4.2 CRIAR `.github/workflows/backup-check.yml` (novo)

Camada extra de agendamento + alerta. Segue o padrão SSH de `deploy-vps.yml:92-104`
(mesmo secret `DEPLOY_VPS_SSH_KEY`, mesmo host/user). Conteúdo exato:

```yaml
name: Backup Check

# Camada EXTRA do backup diario (o primario e' o cron do host da VPS — ver
# docs/BACKUP_RESTORE.md). Este workflow: (1) sincroniza scripts/backup/backup-db.sh
# do repo pra VPS; (2) confere o status do ultimo backup; (3) se ausente/velho/falho,
# roda o script como fallback; (4) se ainda assim falhar, o job fica vermelho e o
# GitHub notifica por e-mail (alerta minimo viavel — sem stack de monitoracao).
# ATENCAO: `schedule` so dispara na branch default (main); antes do merge, usar
# workflow_dispatch manual.

on:
  schedule:
    - cron: "0 9 * * *" # 09:00 UTC, ~2h20 depois do cron da VPS (06:40 UTC)
  workflow_dispatch: {}

permissions:
  contents: read

concurrency:
  group: backup-check-lista-atendimento
  cancel-in-progress: false

jobs:
  check:
    runs-on: ubuntu-latest
    env:
      DEPLOY_HOST: 85.31.62.33
      DEPLOY_PORT: "22"
      DEPLOY_USER: deploy
      REMOTE_PATH: /home/deploy/lista-atendimento
    steps:
      - name: Checkout
        uses: actions/checkout@v5

      - name: Configure SSH access
        env:
          DEPLOY_VPS_SSH_KEY: ${{ secrets.DEPLOY_VPS_SSH_KEY }}
        run: |
          set -euo pipefail
          if [[ -z "$DEPLOY_VPS_SSH_KEY" ]]; then
            echo "Secret DEPLOY_VPS_SSH_KEY ausente." >&2
            exit 1
          fi
          mkdir -p ~/.ssh
          printf '%s\n' "$DEPLOY_VPS_SSH_KEY" > ~/.ssh/deploy_vps
          chmod 600 ~/.ssh/deploy_vps
          ssh-keyscan -p "$DEPLOY_PORT" -H "$DEPLOY_HOST" >> ~/.ssh/known_hosts

      - name: Sync backup script to VPS
        run: |
          set -euo pipefail
          ssh -i ~/.ssh/deploy_vps -o StrictHostKeyChecking=yes -p "$DEPLOY_PORT" \
            "$DEPLOY_USER@$DEPLOY_HOST" "mkdir -p '$REMOTE_PATH/scripts'"
          scp -i ~/.ssh/deploy_vps -o StrictHostKeyChecking=yes -P "$DEPLOY_PORT" \
            scripts/backup/backup-db.sh "$DEPLOY_USER@$DEPLOY_HOST:$REMOTE_PATH/scripts/backup-db.sh"
          ssh -i ~/.ssh/deploy_vps -o StrictHostKeyChecking=yes -p "$DEPLOY_PORT" \
            "$DEPLOY_USER@$DEPLOY_HOST" "chmod +x '$REMOTE_PATH/scripts/backup-db.sh'"

      - name: Check last backup (fallback roda o script)
        run: |
          set -euo pipefail
          cat > /tmp/check.sh << 'REMOTE_SCRIPT'
          set -euo pipefail
          REMOTE_PATH=/home/deploy/lista-atendimento
          STATUS="$REMOTE_PATH/backups/last_backup_status"
          MAX_AGE_S=93600 # 26h

          is_fresh_ok() {
            [ -f "$STATUS" ] || return 1
            [ "$(awk '{print $1}' "$STATUS")" = "ok" ] || return 1
            age=$(( $(date +%s) - $(stat -c %Y "$STATUS") ))
            [ "$age" -lt "$MAX_AGE_S" ]
          }

          if is_fresh_ok; then
            echo "backup em dia:"; cat "$STATUS"; exit 0
          fi
          echo "backup ausente/velho/falho — rodando fallback agora"
          [ -f "$STATUS" ] && cat "$STATUS" || echo "(sem status file)"
          "$REMOTE_PATH/scripts/backup-db.sh"
          is_fresh_ok || { echo "fallback tambem falhou" >&2; exit 1; }
          echo "fallback OK:"; cat "$STATUS"
          REMOTE_SCRIPT
          ssh -i ~/.ssh/deploy_vps -o StrictHostKeyChecking=yes -p "$DEPLOY_PORT" \
            "$DEPLOY_USER@$DEPLOY_HOST" 'bash -s' < /tmp/check.sh

      - name: Publish summary
        if: ${{ always() }}
        run: |
          {
            echo "## Backup Check"
            echo ""
            echo "- Host: \`$DEPLOY_HOST\` | path: \`$REMOTE_PATH/backups\`"
            echo "- Primario: cron do host (06:40 UTC). Este job e' a camada extra + alerta."
            echo "- Job vermelho = backup do dia falhou nas DUAS camadas."
          } >> "$GITHUB_STEP_SUMMARY"
```

### 4.3 CRIAR `docs/BACKUP_RESTORE.md` (novo, ~120 linhas)

Runbook com EXATAMENTE estas seções (redigir em pt-BR, tom dos docs existentes):

1. **Visão geral** — arquitetura em camadas: cron do host (primário, funciona sem GitHub)
   → workflow `backup-check.yml` (camada extra + alerta por e-mail) → off-site rclone
   (sobrevive à perda da VPS). Layout: `backups/daily/` (7), `backups/weekly/` (4),
   `backups/last_backup_status`, `backups/backup.log`. O backup on-demand do deploy
   (`-BackupDatabase`) continua existindo e é complementar (proteção de release de schema).
2. **Instalação na VPS (uma vez)** — copiar o bloco pronto das Notas de Deploy desta spec
   (seção 7), incluindo a linha de crontab:
   `40 6 * * * /home/deploy/lista-atendimento/scripts/backup-db.sh >> /home/deploy/lista-atendimento/backups/backup.log 2>&1`
   (06:40 UTC = 03:40 America/Sao_Paulo, DEPOIS do ERP sync das 04:00 UTC, para o dump já
   capturar os dados sincronizados do dia).
3. **Off-site (recomendado)** — instalar rclone (`sudo apt-get install -y rclone`), rodar
   `rclone config` criando remote `offsite` tipo S3-compatível (Cloudflare R2 / Backblaze B2
   / Wasabi — qualquer um serve; credenciais ficam em `~/.config/rclone/rclone.conf` da VPS,
   NUNCA no repo), criar `/home/deploy/lista-atendimento/.backup.env` com
   `BACKUP_RCLONE_REMOTE=offsite:omni-db-backups` (e opcional `BACKUP_ALERT_URL`), rodar
   `chmod 600 .backup.env`, testar com `rclone lsd offsite:` e uma execução manual do script.
4. **Teste de restore mensal (runbook)** — todo dia 1º do mês, na VPS:

   ```bash
   cd /home/deploy/lista-atendimento
   latest=$(ls -t backups/daily/backup_*.sql.gz | head -n 1)
   echo "restaurando $latest em banco temporario"

   # 1. banco temporario (NUNCA restaurar por cima do banco omni)
   docker compose --env-file .env.production -f docker-compose.prod.yml exec -T postgres \
     sh -lc 'dropdb -U "$POSTGRES_USER" --if-exists omni_restore_test && createdb -U "$POSTGRES_USER" omni_restore_test'

   # 2. restore
   gunzip -c "$latest" | docker compose --env-file .env.production -f docker-compose.prod.yml \
     exec -T postgres sh -lc 'psql -q -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d omni_restore_test'

   # 3. verificacao: contagem de tabelas e de usuarios igual ao banco vivo
   docker compose --env-file .env.production -f docker-compose.prod.yml exec -T postgres sh -lc '
     q="select count(*) from information_schema.tables where table_schema in ('"'"'core'"'"','"'"'queue'"'"','"'"'crm'"'"','"'"'calendar'"'"','"'"'site'"'"','"'"'public'"'"')";
     a=$(psql -tA -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "$q");
     b=$(psql -tA -U "$POSTGRES_USER" -d omni_restore_test -c "$q");
     u1=$(psql -tA -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "select count(*) from core.users");
     u2=$(psql -tA -U "$POSTGRES_USER" -d omni_restore_test -c "select count(*) from core.users");
     echo "tabelas vivo=$a restore=$b | core.users vivo=$u1 restore=$u2";
     [ "$a" = "$b" ] && [ "$u1" = "$u2" ] && echo RESTORE_OK || echo RESTORE_DIVERGENTE'

   # 4. limpar
   docker compose --env-file .env.production -f docker-compose.prod.yml exec -T postgres \
     sh -lc 'dropdb -U "$POSTGRES_USER" omni_restore_test'
   ```

   Nota no runbook: pequena divergência de contagem é esperada se houver escrita entre o
   dump e o teste; divergência grande ou erro de `psql` = investigar antes de confiar no backup.
5. **Restore real (desastre)** — procedimento de referência: parar `api` e `web`
   (`docker compose ... stop api web`), NUNCA dropar `postgres_data` sem backup adicional,
   restaurar em banco novo (`omni_restored`), validar como no teste mensal e só então
   renomear bancos (`ALTER DATABASE ... RENAME`) — decisão consciente do usuário, nunca
   automatizada. Reforçar: dados de usuário/password_hash jamais são sobrescritos fora
   desse fluxo explícito.
6. **Alerta** — como funciona: status file + backup-check vermelho ⇒ e-mail do GitHub ao
   dono do repo; opcional `BACKUP_ALERT_URL` (ntfy.sh ou webhook) para push imediato.
7. **Pendências fora deste runbook** — backup do volume `api_uploads` e do `.env.production`
   (hoje manual, citado em DEPLOY_VPS.md; candidato a AC futuro).

### 4.4 EDITAR `docs/DEPLOY_VPS.md` — seção "Backup minimo" (linhas 252-258)

Substituir o conteúdo atual:

```markdown
## Backup minimo

- dump do PostgreSQL desta stack (os scripts de deploy fazem com `-BackupDatabase`; ficam em
  `/home/deploy/lista-atendimento/backups/`)
- volume `api_uploads`
- arquivo `.env.production`
```

por:

```markdown
## Backup

Backup diario AGENDADO do Postgres: cron do host (06:40 UTC) roda
`/home/deploy/lista-atendimento/scripts/backup-db.sh` (fonte: `scripts/backup/backup-db.sh`
no repo) — retencao 7 diarios + 4 semanais em `backups/daily|weekly/`, off-site opcional via
rclone, status em `backups/last_backup_status`. O workflow `backup-check.yml` confere/roda
fallback todo dia e alerta por e-mail se falhar. Instalacao, teste de restore mensal e
restore real: [BACKUP_RESTORE.md](BACKUP_RESTORE.md).

O backup on-demand dos deploys (`-BackupDatabase` / input `backup_database`) continua
existindo e segue OBRIGATORIO em release que toca schema.

Ainda manual (pendente): volume `api_uploads` e arquivo `.env.production`.
```

### 4.5 CRIAR `scripts/backup/AGENT.md` (novo, curto)

No formato do `scripts/deploy/AGENT.md`: escopo (`scripts/backup`), objetivo (backup
agendado do Postgres de produção), regras — o script roda NA VPS e é sincronizado pelo
`backup-check.yml` (repo é a fonte de verdade); nunca colocar credencial; falha de off-site
é falha do backup; `flock` contra sobreposição; retenção 7/4; formato do status file;
validação mínima (`bash -n`); referências (`docs/BACKUP_RESTORE.md`, `docs/DEPLOY_VPS.md`,
`.github/workflows/backup-check.yml`, `../deploy/AGENT.md`).

### 4.6 EDITAR `scripts/deploy/AGENT.md` — seção "Referencias" (linhas 96-103)

Adicionar duas linhas à lista existente:

```markdown
- `../backup/AGENT.md` (backup agendado do Postgres — complementa o `-BackupDatabase` on-demand)
- `../../docs/BACKUP_RESTORE.md` (runbook de backup/restore)
```

## 5. Critérios de aceite

1. `scripts/backup/backup-db.sh` existe, com `set -euo pipefail`, lock via `flock`,
   `gzip -t` + checagem de tamanho mínimo, retenção 7/4, off-site condicional a
   `BACKUP_RCLONE_REMOTE`, status file `ok|fail ...` e ZERO credencial hardcoded.
2. `bash -n scripts/backup/backup-db.sh` sai com código 0.
3. `.github/workflows/backup-check.yml` existe com `schedule` 09:00 UTC + `workflow_dispatch`,
   usa o secret `DEPLOY_VPS_SSH_KEY` já existente, sincroniza o script para a VPS e falha o
   job quando o status não é `ok` fresco (<26h) mesmo após fallback.
4. `docs/BACKUP_RESTORE.md` existe com as 7 seções da 4.3, incluindo a linha de crontab
   pronta e o teste de restore mensal com contagem de tabelas + `core.users`.
5. `docs/DEPLOY_VPS.md` tem a seção "Backup" reescrita apontando para o runbook, mantendo a
   menção ao on-demand de deploy e às pendências (`api_uploads`, `.env.production`).
6. `scripts/backup/AGENT.md` criado; `scripts/deploy/AGENT.md` referencia a nova pasta.
7. Nada foi removido dos fluxos existentes (`deploy-ship.ps1`, `deploy-vps.yml`,
   `deploy-pull.ps1` intactos); nenhum arquivo de código Go/TS tocado; nenhuma migration.
8. Nenhum arquivo novo passa de 450 linhas.
9. Nenhum comando git executado; nada executado na VPS.

## 6. Validação (local, sem VPS)

```bash
# 1. sintaxe do script bash (Git Bash no Windows)
bash -n scripts/backup/backup-db.sh

# 2. sanidade do YAML do workflow (parser Node já disponível no repo)
node -e "const y=require('web/node_modules/yaml'); y.parse(require('fs').readFileSync('.github/workflows/backup-check.yml','utf8')); console.log('yaml ok')"
#    (se o pacote 'yaml' não estiver em web/node_modules, deixar listado para o usuário
#     validar com actionlint/editor — NÃO rodar npm install)

# 3. dry-run lógico do script SEM tocar banco: conferir que aborta cedo com env file ausente
BACKUP_COMPOSE_DIR="$PWD/tmp-backup-test" bash scripts/backup/backup-db.sh || echo "abortou como esperado (exit != 0)"
rm -rf tmp-backup-test
#    (o script cria tmp-backup-test/backups/* e falha em "env file ausente" — comportamento correto)
```

Validação real na VPS (execução manual do script + workflow_dispatch) fica para o usuário —
listada nas Notas de Deploy.

## 7. Notas de Deploy (ordem exata — executar pelo USUÁRIO na VPS)

Sem migration, sem env var de container, sem rebuild de imagem (não toca `back/` nem `web/`).
Instalação one-time, nesta ordem:

```bash
# 0. (da maquina local) copiar o script pra VPS
scp -i ~/.ssh/gh_actions_omnichannel_vps scripts/backup/backup-db.sh \
  deploy@85.31.62.33:/home/deploy/lista-atendimento/scripts/backup-db.sh

# NA VPS (ssh deploy@85.31.62.33):
chmod +x /home/deploy/lista-atendimento/scripts/backup-db.sh

# 1. primeira execucao manual (valida dump + status file)
/home/deploy/lista-atendimento/scripts/backup-db.sh
cat /home/deploy/lista-atendimento/backups/last_backup_status   # deve comecar com "ok"

# 2. cron diario (primario) — crontab do user deploy
crontab -l 2>/dev/null | { cat; echo '40 6 * * * /home/deploy/lista-atendimento/scripts/backup-db.sh >> /home/deploy/lista-atendimento/backups/backup.log 2>&1'; } | crontab -
crontab -l   # conferir a linha

# 3. (recomendado) off-site: instalar rclone + configurar remote S3-compativel
sudo apt-get update && sudo apt-get install -y rclone
rclone config   # criar remote "offsite" (R2/B2/Wasabi); credenciais SO na VPS
printf 'BACKUP_RCLONE_REMOTE=offsite:omni-db-backups\n' > /home/deploy/lista-atendimento/.backup.env
chmod 600 /home/deploy/lista-atendimento/.backup.env
/home/deploy/lista-atendimento/scripts/backup-db.sh   # re-testar com off-site ligado

# 4. camada extra GitHub: apos merge na main, o schedule do backup-check.yml ativa sozinho
#    (schedule so roda na branch default). Antes do merge: rodar manual p/ validar:
#    gh workflow run backup-check.yml --repo mikewade2k16/lista-da-vez

# 5. agendar no calendario do time o teste de restore mensal (docs/BACKUP_RESTORE.md secao 4)
```

Dependência de secret: `DEPLOY_VPS_SSH_KEY` já existe (usado pelo deploy-vps.yml) — nada novo.

## 8. Arquivos tocados

| Arquivo | Ação |
|---|---|
| `scripts/backup/backup-db.sh` | criar |
| `scripts/backup/AGENT.md` | criar |
| `.github/workflows/backup-check.yml` | criar |
| `docs/BACKUP_RESTORE.md` | criar |
| `docs/DEPLOY_VPS.md` | editar (seção "Backup minimo", ~linhas 252-258) |
| `scripts/deploy/AGENT.md` | editar (seção "Referencias", acrescentar 2 linhas) |
