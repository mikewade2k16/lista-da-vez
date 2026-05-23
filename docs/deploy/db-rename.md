# Rename do banco em producao para `omni`

Runbook da Fase 4.6 para trocar o banco de producao de `listaatendimento` para `omni` sem perder dados.

## Objetivo

- manter o conteudo do banco intacto;
- executar a troca em janela curta de manutencao;
- sair com a stack apontando para `POSTGRES_DB=omni`;
- preservar rollback objetivo caso algo bloqueie o rename.

## Pre-condicoes

- backup completo obrigatorio antes de qualquer comando de rename;
- `api` e `web` precisam ser parados antes do `ALTER DATABASE`;
- o arquivo `.env.production` do host precisa estar pronto para receber `COMPOSE_PROJECT_NAME=omni`, `APP_NAME=omni-api`, `POSTGRES_DB=omni` e, se a role tambem for renomeada na mesma janela, `POSTGRES_USER=omni`;
- o proxy externo (`omnichannel-mvp_default`, `lista-api`, `lista-web`, `/opt/omnichannel/Caddyfile`) nao entra no rename.

## Estrategia recomendada

### 1. Gerar backup antes da janela

Opcoes seguras:

- `npm run prod:deploy:vps:backup` no workspace local;
- ou workflow manual `deploy-vps.yml` com `backup_database=true`;
- ou backup manual no host:

```bash
cd /home/deploy/lista-atendimento
mkdir -p backups
backup="backups/pre_omni_rename_$(date +%Y%m%d_%H%M%S).sql.gz"
docker compose --env-file .env.production -f docker-compose.prod.yml exec -T postgres \
  sh -lc 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB"' | gzip > "$backup"
ls -lh "$backup"
```

### 2. Parar quem conecta no banco

```bash
cd /home/deploy/lista-atendimento
docker compose --env-file .env.production -f docker-compose.prod.yml stop api web
```

### 3. Conferir sessoes abertas

Se ainda houver conexoes no banco antigo, o rename vai falhar. Verifique antes:

```bash
cd /home/deploy/lista-atendimento
docker compose --env-file .env.production -f docker-compose.prod.yml exec -T postgres \
  psql -U "$POSTGRES_USER" -d postgres -v ON_ERROR_STOP=1 <<'SQL'
select pid, usename, datname, application_name, client_addr, state
from pg_stat_activity
where datname in ('listaatendimento', 'omni')
order by datname, pid;
SQL
```

Se sobrar sessao presa em `listaatendimento`, termine as conexoes antes do rename:

```bash
cd /home/deploy/lista-atendimento
docker compose --env-file .env.production -f docker-compose.prod.yml exec -T postgres \
  psql -U "$POSTGRES_USER" -d postgres -v ON_ERROR_STOP=1 <<'SQL'
select pg_terminate_backend(pid)
from pg_stat_activity
where datname = 'listaatendimento'
  and pid <> pg_backend_pid();
SQL
```

### 4. Renomear o banco

```bash
cd /home/deploy/lista-atendimento
docker compose --env-file .env.production -f docker-compose.prod.yml exec -T postgres \
  psql -U "$POSTGRES_USER" -d postgres -v ON_ERROR_STOP=1 <<'SQL'
alter database listaatendimento rename to omni;
SQL
```

### 5. Renomear a role apenas se houver sessao administrativa separada

O passo abaixo e canonico, mas nao vale forcar se o container estiver autenticado justamente como `listaatendimento` e o ambiente nao tiver outra role administrativa pronta. Se houver duvida, mantenha a role antiga no primeiro restart e faca esta troca numa janela propria.

```bash
cd /home/deploy/lista-atendimento
docker compose --env-file .env.production -f docker-compose.prod.yml exec -T postgres \
  psql -U postgres -d postgres -v ON_ERROR_STOP=1 <<'SQL'
alter role listaatendimento rename to omni;
SQL
```

### 6. Atualizar `.env.production`

Valores alvo da Fase 4:

```env
COMPOSE_PROJECT_NAME=omni
APP_NAME=omni-api
POSTGRES_DB=omni
POSTGRES_USER=omni
```

Se a role ainda nao foi renomeada na mesma janela, use transitoriamente:

```env
COMPOSE_PROJECT_NAME=omni
APP_NAME=omni-api
POSTGRES_DB=omni
POSTGRES_USER=listaatendimento
```

### 7. Subir de novo e validar migrations

```bash
cd /home/deploy/lista-atendimento
docker compose --env-file .env.production -f docker-compose.prod.yml up -d --build api web
docker compose --env-file .env.production -f docker-compose.prod.yml exec -T api migrate up
docker compose --env-file .env.production -f docker-compose.prod.yml ps
curl -I https://lista.whenthelightsdie.com
curl -I https://lista.whenthelightsdie.com/healthz
```

## Fallback seguro: dump/restore

Use este caminho se o `ALTER DATABASE` falhar repetidamente por bloqueio, permissao ou dependencia inesperada.

### 1. Exportar dump completo do banco antigo

```bash
cd /home/deploy/lista-atendimento
docker compose --env-file .env.production -f docker-compose.prod.yml exec -T postgres \
  sh -lc 'pg_dump -Fc -U "$POSTGRES_USER" -d listaatendimento' > omni-pre-rename.dump
```

### 2. Criar o banco novo

```bash
cd /home/deploy/lista-atendimento
docker compose --env-file .env.production -f docker-compose.prod.yml exec -T postgres \
  psql -U "$POSTGRES_USER" -d postgres -v ON_ERROR_STOP=1 <<'SQL'
create database omni;
SQL
```

### 3. Restaurar

```bash
cd /home/deploy/lista-atendimento
container=$(docker compose --env-file .env.production -f docker-compose.prod.yml ps -q postgres)
docker cp omni-pre-rename.dump "$container:/tmp/omni-pre-rename.dump"
docker compose --env-file .env.production -f docker-compose.prod.yml exec -T postgres \
  pg_restore -U "$POSTGRES_USER" -d omni --clean --if-exists /tmp/omni-pre-rename.dump
```

Depois, seguir os passos de atualizar `.env.production`, subir `api/web` e validar smoke.

## Rollback

Se a stack nao subir apos o rename:

1. parar `api` e `web`;
2. restaurar o backup feito antes da janela; ou, se o rename foi o unico passo concluido, inverter o nome do banco com `alter database omni rename to listaatendimento;`;
3. recolocar os valores antigos de `.env.production`;
4. subir novamente e validar `healthz`.

## Observacoes praticas

- o path remoto `/home/deploy/lista-atendimento` pode continuar como esta; ele nao afeta integridade do banco;
- `PROXY_NETWORK_NAME=omnichannel-mvp_default`, `PROXY_API_ALIAS=lista-api` e `PROXY_WEB_ALIAS=lista-web` podem permanecer para reduzir risco no proxy publico;
- se a troca da role ficar para depois, a meta de seguranca continua preservada porque o nome do banco e o contrato de app ja migram primeiro, sem jogar fora dado nenhum.