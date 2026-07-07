# Backup & Restore do Postgres (Omni)

Runbook do backup diario AGENDADO do banco de producao e do procedimento de
restore. Fonte do script: `scripts/backup/backup-db.sh` no repo. Instalacao e
operacao acontecem NA VPS (host `85.31.62.33`, user `deploy`, prod em
`/home/deploy/lista-atendimento`).

> Complementa o `docs/DEPLOY_VPS.md` (secao "Backup"). O backup on-demand dos
> deploys (`-BackupDatabase` / input `backup_database`) continua existindo e e'
> complementar — protege release de schema, nao substitui este backup diario.

---

## 1. Visao geral (arquitetura em camadas)

O backup e' redundante de proposito, em tres camadas:

1. **Cron do host da VPS (primario)** — roda `backup-db.sh` todo dia as 06:40 UTC.
   Funciona mesmo sem GitHub, sem CI, sem rede externa. E' a camada que garante o
   dump diario.
2. **Workflow `backup-check.yml` (camada extra + alerta)** — todo dia as 09:00 UTC
   o GitHub Actions SSHa na VPS, sincroniza o script do repo, confere o status do
   ultimo backup e roda um fallback se estiver ausente/velho/falho. Se o fallback
   tambem falhar, o job fica vermelho e o GitHub manda e-mail ao dono do repo
   (alerta minimo viavel — sem stack de monitoracao).
3. **Off-site rclone (sobrevive a perda da VPS)** — cada dump e' copiado para um
   bucket S3-compativel (Cloudflare R2 / Backblaze B2 / Wasabi). Se o disco ou a
   VPS inteira morrer, o backup continua fora do host. Opcional-mas-recomendado.

**Layout em `backups/`:**

```
backups/
  daily/                     7 dumps diarios (backup_YYYYMMDD_HHMMSS.sql.gz)
  weekly/                    ~4 copias semanais (feitas aos domingos)
  last_backup_status         1 linha: ok|fail <UTC ISO8601> <arquivo|motivo> [bytes]
  backup.log                 saida do cron (append)
  .backup.lock               lock do flock (nao versionar)
```

**Formato do dump:** SQL plano gzipado (`pg_dump | gzip`), identico ao fluxo de
deploy — restaura com `psql`, sem depender de versao de `pg_restore`.

**Status file** e' a interface do alerta: a primeira palavra e' `ok` ou `fail`.
Falha de off-site conta como `fail` (o arquivo local e' preservado; a mensagem
diz "backup local OK").

---

## 2. Instalacao na VPS (uma vez)

Executar pelo operador, na ordem. Os mesmos comandos estao nas "Notas de Deploy"
da spec AC-05.

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
```

O horario **06:40 UTC = 03:40 America/Sao_Paulo** e' proposital: cai DEPOIS do
ERP sync das 04:00 UTC (`ERP_SYNC_HOUR_UTC=4`), entao o dump ja captura os dados
sincronizados do dia.

---

## 3. Off-site (recomendado)

O dump local sobrevive a um deploy ruim, mas nao a perda do disco/VPS. Para isso,
configurar o rclone:

```bash
# NA VPS:
sudo apt-get update && sudo apt-get install -y rclone
rclone config     # criar remote "offsite" tipo S3-compativel (R2 / B2 / Wasabi)
                  # credenciais ficam em ~/.config/rclone/rclone.conf da VPS,
                  # NUNCA no repo.

# apontar o script pro remote (arquivo fora do repo, chmod 600):
printf 'BACKUP_RCLONE_REMOTE=offsite:omni-db-backups\n' > /home/deploy/lista-atendimento/.backup.env
# (opcional) alerta push imediato em falha:
# printf 'BACKUP_ALERT_URL=https://ntfy.sh/<seu-topico>\n' >> /home/deploy/lista-atendimento/.backup.env
chmod 600 /home/deploy/lista-atendimento/.backup.env

# testar
rclone lsd offsite:                                       # lista os buckets
/home/deploy/lista-atendimento/scripts/backup-db.sh       # re-executa com off-site ligado
```

Qualquer provedor S3-compativel serve. As credenciais vivem SO na VPS
(`rclone.conf` + `.backup.env`), nunca no repositorio.

---

## 4. Teste de restore mensal (runbook)

Um backup nunca testado nao e' backup. Todo dia 1o do mes, na VPS, restaurar o
ultimo dump em um banco **temporario** e conferir. NUNCA restaurar por cima do
banco `omni` vivo.

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

Pequena divergencia de contagem e' esperada se houve escrita entre o dump e o
teste. **Divergencia grande** ou **erro de `psql`** = investigar antes de confiar
no backup — nao marcar o teste como passado.

---

## 5. Restore real (desastre)

Procedimento de referencia para recuperar producao de um dump. E' uma decisao
consciente do operador, **nunca automatizada** — dados de usuario/password_hash
jamais sao sobrescritos fora deste fluxo explicito.

```bash
cd /home/deploy/lista-atendimento

# 1. parar quem escreve no banco (Postgres continua no ar)
docker compose --env-file .env.production -f docker-compose.prod.yml stop api web

# 2. escolher o dump (local mais recente OU baixar do off-site com rclone)
latest=$(ls -t backups/daily/backup_*.sql.gz | head -n 1)
# off-site:  rclone copy offsite:omni-db-backups/daily/<arquivo> backups/daily/

# 3. restaurar em banco NOVO (nunca dropar postgres_data sem backup adicional)
docker compose --env-file .env.production -f docker-compose.prod.yml exec -T postgres \
  sh -lc 'dropdb -U "$POSTGRES_USER" --if-exists omni_restored && createdb -U "$POSTGRES_USER" omni_restored'
gunzip -c "$latest" | docker compose --env-file .env.production -f docker-compose.prod.yml \
  exec -T postgres sh -lc 'psql -q -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d omni_restored'

# 4. validar omni_restored como no teste mensal (secao 4, apontando pra omni_restored)

# 5. SO ENTAO, com o operador certo, trocar os bancos (decisao explicita):
#    docker compose ... exec -T postgres sh -lc '
#      psql -U "$POSTGRES_USER" -d postgres -c "ALTER DATABASE \"$POSTGRES_DB\" RENAME TO omni_old;
#                                               ALTER DATABASE omni_restored RENAME TO \"$POSTGRES_DB\";"'

# 6. subir api/web de volta
docker compose --env-file .env.production -f docker-compose.prod.yml up -d api web
```

Regra dura: **nunca** dropar o volume `postgres_data` nem o banco `omni` sem um
backup adicional na mao. Restaura-se sempre em banco novo, valida-se, e so entao
se decide trocar.

---

## 6. Alerta

Como o time descobre que um backup falhou:

- **Status file + backup-check vermelho ⇒ e-mail do GitHub.** Se o
  `backup-check.yml` nao consegue confirmar um `ok` fresco (<26h) nem rodando o
  fallback, o job falha e o GitHub notifica por e-mail o dono do repo. E' o
  alerta minimo viavel, sem stack de monitoracao dedicada.
- **`BACKUP_ALERT_URL` (opcional)** — se definido em `.backup.env` (ntfy.sh ou um
  webhook qualquer), o proprio script faz um POST imediato na hora da falha, para
  push instantaneo no celular.

Checagem manual rapida a qualquer momento:

```bash
cat /home/deploy/lista-atendimento/backups/last_backup_status
gh workflow run backup-check.yml --repo mikewade2k16/lista-da-vez   # forcar a camada extra
```

---

## 7. Pendencias fora deste runbook

Cobertas por AC futuro, ainda **manuais** (citadas em `docs/DEPLOY_VPS.md`):

- **Volume `api_uploads`** — midias/uploads da api nao entram neste backup (escopo
  aqui e' so o banco). Backup manual do volume Docker por enquanto.
- **Arquivo `.env.production`** — segredos de ambiente da VPS; guardar copia
  segura fora do host (nunca no repo).
