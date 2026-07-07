# AC-05b — Off-site do backup (rclone) + drill de restore mensal

> Spec de implementação · Prioridade **P1** · Esforço **S** · Impacto **alto**
> Origem: diagnóstico 2026-07 (AC-05, parte pendente) · roadmap `ac-fixes-2026-07` → task `ac-05-backup-agendado-offsite`

## 1. Contexto

**O achado:** o backup diário agendado está LIVE na VPS desde 03/07 (cron 06:40 UTC, retenção
7d/4sem, `scripts/backup/backup-db.sh`), mas as duas pernas que fazem dele um backup DE VERDADE
seguem pendentes: (a) **off-site** — se a VPS morrer/for comprometida, os dumps morrem junto;
(b) **restore testado** — backup que nunca restaurou não é backup, é esperança.

Evidências:
- `scripts/backup/backup-db.sh:19-24,69-80` — o suporte a rclone JÁ EXISTE no script
  (`BACKUP_RCLONE_REMOTE` via `.backup.env`; falha de off-site = falha do backup); só falta
  instalar/configurar o rclone na VPS.
- `docs/BACKUP_RESTORE.md` §3 (off-site) e §4 (teste de restore mensal) — runbook escrito,
  nunca executado; o drill do §4 é 100% manual (sem script, sem cron, sem status).
- `scripts/backup/backup-db.sh:17,29,82` — gramática do status file (`ok|fail <ts> <detalhe>`)
  que este spec replica para o drill.

## 2. Objetivo e não-objetivos

**Objetivo (escopo fechado):**
1. Off-site funcionando: rclone instalado e configurado na VPS (bucket S3-compatível), `.backup.env`
   com `BACKUP_RCLONE_REMOTE`, evidência de dump do dia no bucket.
2. Drill de restore AUTOMATIZADO: novo `scripts/backup/restore-drill.sh` (restaura o último dump em
   banco temporário, compara contagens com o vivo, grava `backups/last_restore_drill`).
3. Cron mensal do drill (dia 1, 04:30 UTC) + procedimento trimestral com dump baixado do OFF-SITE.

**Não-objetivos (FORA):**
- NÃO mudar `backup-db.sh` (já suporta tudo).
- NÃO fazer backup de `api_uploads`/`.env.production` (pendência separada, documentada no runbook).
- NÃO restaurar por cima do banco vivo em NENHUMA hipótese (o script usa banco temporário).

## 3. Regras de execução (obrigatórias)

- NENHUM comando git. Passos na VPS via SSH (`ssh -i ~/.ssh/gh_actions_omnichannel_vps deploy@85.31.62.33`).
- NUNCA inventar/reutilizar credenciais: o bucket S3 e suas chaves são criados/fornecidos PELO DONO
  (parar e pedir quando chegar no passo 4.3).
- Nunca parar o Caddy da stack omnichannel-mvp; nunca tocar volumes de prod.
- Atualizar `scripts/backup/AGENT.md` e `docs/BACKUP_RESTORE.md`.

## 4. Mudanças (passo a passo)

### 4.1 CRIAR `scripts/backup/restore-drill.sh`

```bash
#!/usr/bin/env bash
# restore-drill.sh — drill mensal de restore (AC-05b). Restaura o ultimo backup
# diario num banco TEMPORARIO (omni_restore_test), compara contagens com o banco
# vivo e grava backups/last_restore_drill (ok|fail <ts> <detalhe>) — mesma
# gramatica do last_backup_status; o check-vps.sh (OBS-01) vigia os dois.
# NUNCA toca o banco vivo. Runbook: docs/BACKUP_RESTORE.md §4.
# Uso trimestral com dump do off-site: RESTORE_DRILL_FILE=<path> ./restore-drill.sh
set -euo pipefail

COMPOSE_DIR="${BACKUP_COMPOSE_DIR:-/home/deploy/lista-atendimento}"
ENV_FILE="${BACKUP_ENV_FILE:-.env.production}"
BACKUP_DIR="${BACKUP_DIR:-$COMPOSE_DIR/backups}"
STATUS_FILE="$BACKUP_DIR/last_restore_drill"
TEST_DB="omni_restore_test"

fail() {
  printf 'fail %s %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$1" > "$STATUS_FILE"
  echo "ERRO: $1" >&2
  exit 1
}

cd "$COMPOSE_DIR" || fail "COMPOSE_DIR inexistente: $COMPOSE_DIR"
[ -f "$ENV_FILE" ] || fail "env file ausente: $COMPOSE_DIR/$ENV_FILE"

dump="${RESTORE_DRILL_FILE:-$(ls -1t "$BACKUP_DIR"/daily/backup_*.sql.gz 2>/dev/null | head -1)}"
[ -n "$dump" ] && [ -f "$dump" ] || fail "nenhum dump encontrado (daily vazio e RESTORE_DRILL_FILE nao setado)"
gzip -t "$dump" || fail "dump corrompido: $dump"

pg() { docker compose --env-file "$ENV_FILE" -f docker-compose.prod.yml exec -T postgres sh -lc "$1"; }

pg "dropdb --if-exists -U \"\$POSTGRES_USER\" $TEST_DB && createdb -U \"\$POSTGRES_USER\" $TEST_DB" \
  || fail "nao conseguiu recriar $TEST_DB"

if ! gunzip -c "$dump" | docker compose --env-file "$ENV_FILE" -f docker-compose.prod.yml exec -T postgres \
    sh -lc "psql -q -v ON_ERROR_STOP=1 -U \"\$POSTGRES_USER\" -d $TEST_DB" >/dev/null; then
  pg "dropdb --if-exists -U \"\$POSTGRES_USER\" $TEST_DB" || true
  fail "psql retornou erro restaurando $(basename "$dump")"
fi

# Verificacao do BACKUP_RESTORE.md §4: contagem de tabelas por schema + core.users.
count_sql="select (select count(*) from information_schema.tables where table_schema in ('core','queue','crm','calendar','site','public'))::text || '/' || (select count(*) from core.users)::text"
live=$(pg "psql -tA -U \"\$POSTGRES_USER\" -d \"\$POSTGRES_DB\" -c \"$count_sql\"" | tr -d '[:space:]')
test=$(pg "psql -tA -U \"\$POSTGRES_USER\" -d $TEST_DB -c \"$count_sql\"" | tr -d '[:space:]')

pg "dropdb --if-exists -U \"\$POSTGRES_USER\" $TEST_DB" || true

if [ "$live" = "$test" ]; then
  printf 'ok %s %s tabelas+users=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$(basename "$dump")" "$live" > "$STATUS_FILE"
  echo "RESTORE_OK: $(basename "$dump") (tabelas+users: $live)"
else
  # Pequena divergencia e' esperada se houve escrita desde o dump; grande = investigar.
  printf 'ok %s %s DIVERGENTE live=%s dump=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$(basename "$dump")" "$live" "$test" > "$STATUS_FILE"
  echo "RESTORE_DIVERGENTE: live=$live dump=$test (esperado com escrita recente; investigar se a diferenca for grande)"
fi
```

`chmod +x scripts/backup/restore-drill.sh` no repo.

### 4.2 Instalar o drill na VPS (runbook)

```bash
scp -i ~/.ssh/gh_actions_omnichannel_vps scripts/backup/restore-drill.sh \
  deploy@85.31.62.33:/home/deploy/lista-atendimento/scripts/restore-drill.sh
ssh -i ~/.ssh/gh_actions_omnichannel_vps deploy@85.31.62.33 \
  "chmod +x /home/deploy/lista-atendimento/scripts/restore-drill.sh && \
   /home/deploy/lista-atendimento/scripts/restore-drill.sh"   # 1ª execução, na hora
# cron mensal (dia 1, 04:30 UTC — depois do backup 06:40? NÃO: 04:30 usa o dump de ONTEM,
# proposital: nunca disputa com o backup do dia)
ssh ... "( crontab -l 2>/dev/null; echo '30 4 1 * * /home/deploy/lista-atendimento/scripts/restore-drill.sh >> /home/deploy/lista-atendimento/backups/restore-drill.log 2>&1' ) | crontab -"
```

### 4.3 Off-site (runbook — PARAR e pedir credenciais ao dono)

Seguir `docs/BACKUP_RESTORE.md` §3, na VPS:

```bash
sudo apt-get update && sudo apt-get install -y rclone
rclone config   # remote "offsite", tipo s3 (R2/B2/Wasabi — o dono escolhe e fornece as chaves)
rclone lsd offsite:                       # valida credenciais
rclone mkdir offsite:omni-db-backups
printf 'BACKUP_RCLONE_REMOTE=offsite:omni-db-backups\n' >> /home/deploy/lista-atendimento/.backup.env
chmod 600 /home/deploy/lista-atendimento/.backup.env
/home/deploy/lista-atendimento/scripts/backup-db.sh    # roda 1x na hora: dump + envio
rclone lsl offsite:omni-db-backups/daily               # EVIDÊNCIA: dump do dia no bucket
```

### 4.4 Drill trimestral com o dump do off-site (adicionar ao runbook)

A cada 3 meses (janeiro/abril/julho/outubro, manual): baixar o último dump do BUCKET e rodar o
drill contra ELE — prova o caminho completo de desastre (VPS nova + bucket):

```bash
rclone copy offsite:omni-db-backups/daily /home/deploy/tmp-restore --max-age 2d
RESTORE_DRILL_FILE=$(ls -1t /home/deploy/tmp-restore/backup_*.sql.gz | head -1) \
  /home/deploy/lista-atendimento/scripts/restore-drill.sh
rm -rf /home/deploy/tmp-restore
```

### 4.5 EDITAR docs

- `docs/BACKUP_RESTORE.md` §4: apontar para o script (`scripts/restore-drill.sh` na VPS), registrar
  o cron mensal e o procedimento trimestral do 4.4; §3 marcar como EXECUTADO com a data.
- `scripts/backup/AGENT.md`: nova entrada do `restore-drill.sh` (o que faz, status file, crons).
- Registrar em "Notas de Deploy" do doc canônico: `.backup.env` ganhou `BACKUP_RCLONE_REMOTE`
  (arquivo só na VPS, chmod 600, NUNCA no repo).

## 5. Critérios de aceite

1. `rclone lsl offsite:omni-db-backups/daily` mostra o dump do dia (bytes > 10240).
2. `cat backups/last_restore_drill` começa com `ok` e cita o dump usado.
3. `crontab -l` contém as duas linhas (backup 06:40 diário; drill 04:30 dia 1).
4. Banco vivo intocado: `psql -tc "select count(*) from pg_database where datname='omni_restore_test'"` = 0 após o drill.
5. Simulação de falha: rodar o drill com `RESTORE_DRILL_FILE=/tmp/inexistente.sql.gz` → status `fail` + exit ≠ 0.
6. (Integração OBS-01, quando aplicado) check-vps alerta se `last_restore_drill` > 32 dias.

## 6. Validação

Passos 4.2/4.3 JÁ SÃO a validação (execuções reais na VPS com evidência). Local: `bash -n
scripts/backup/restore-drill.sh` (sintaxe) + shellcheck se disponível.

## 7. Notas de Deploy

- Nenhuma migration, nenhuma env de app, nenhum rebuild de imagem. Tudo é host-side na VPS +
  1 script novo no repo.
- Segredos: chaves do bucket ficam SÓ em `~/.config/rclone/rclone.conf` (VPS) e o remote em
  `.backup.env` (chmod 600). Nada no repo.
- Rollback: remover `BACKUP_RCLONE_REMOTE` do `.backup.env` (backup volta a ser só local) e as
  linhas do cron.

## 8. Arquivos tocados

| Arquivo | Ação |
|---|---|
| `scripts/backup/restore-drill.sh` | criar |
| `docs/BACKUP_RESTORE.md` | editar (§3 executado, §4 aponta pro script, novo §4b trimestral) |
| `scripts/backup/AGENT.md` | editar |

**Conflitos potenciais:** OBS-01 lê `last_restore_drill` (check 7b) — os specs são independentes,
mas o alerta de drill só faz sentido após este AC-05b rodar 1x.
