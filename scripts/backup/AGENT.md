# AGENT

## Escopo

Estas instrucoes valem para `scripts/backup`.

## Objetivo

Backup diario AGENDADO do Postgres de producao do Omni. Complementa (nao
substitui) o backup on-demand dos deploys (`-BackupDatabase` / input
`backup_database`), que protege release de schema.

## Regras

- o script `backup-db.sh` roda **NA VPS** (host `85.31.62.33`, user `deploy`,
  prod em `/home/deploy/lista-atendimento`), nunca na maquina local.
- **o repo e' a fonte de verdade**: o `backup-check.yml` sincroniza
  `scripts/backup/backup-db.sh` do repo para
  `/home/deploy/lista-atendimento/scripts/backup-db.sh` a cada execucao. Editar
  aqui, nunca so na VPS.
- **nunca colocar credencial no script.** Postgres via env do proprio container
  (`$POSTGRES_USER`/`$POSTGRES_DB`); off-site S3 via config do rclone na VPS
  (`~/.config/rclone/rclone.conf` + `.backup.env`, ambos fora do repo, chmod 600).
- `set -euo pipefail` obrigatorio: sem `pipefail` o gzip mascara um `pg_dump` que
  falhou e o backup sai vazio.
- **falha de off-site e' falha do backup** (status `fail`, exit 1) — mas o arquivo
  local e' preservado.
- **`flock`** contra sobreposicao cron x fallback do workflow.
- **retencao 7/4**: 7 diarios em `backups/daily/`, ~4 semanais em
  `backups/weekly/` (copia aos domingos). Tambem poda dumps soltos do fluxo
  on-demand de deploy em `backups/`.
- **status file** `backups/last_backup_status`: 1 linha
  `ok|fail <UTC ISO8601> <arquivo|motivo> [bytes]`. E' a interface do alerta.
- validacao minima ao alterar o script: `bash -n scripts/backup/backup-db.sh`.

## Referencias

- `../../docs/BACKUP_RESTORE.md` (runbook: instalacao, restore mensal, desastre)
- `../../docs/DEPLOY_VPS.md` (secao "Backup")
- `../../.github/workflows/backup-check.yml` (camada extra + alerta)
- `../deploy/AGENT.md` (deploy e backup on-demand `-BackupDatabase`)
