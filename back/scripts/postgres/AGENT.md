# AGENT

## Escopo

Estas instrucoes valem para os scripts de apoio ao PostgreSQL local em `back/scripts/postgres/`.

## Objetivo

Padronizar o setup local do banco no Windows sem depender de Docker.

## Scripts atuais

- `init-local.ps1`
  - detecta instalacao por servico do PostgreSQL, cria role/database da aplicacao e roda migrations
- `start-local.ps1`
  - sobe o servico `postgresql-x64-16` quando ele existir, ou cai para `pg_ctl` no modo manual
- `stop-local.ps1`
  - para o servico `postgresql-x64-16` quando ele existir, ou cai para `pg_ctl` no modo manual
- `status-local.ps1`
  - mostra modo atual, caminho de dados, `pgAdmin 4` e resposta do `pg_isready`
- `open-pgadmin.ps1`
  - abre o `pgAdmin 4` local quando ele estiver instalado com o PostgreSQL

## Convencoes usadas

- binario esperado:
  - `C:\Program Files\PostgreSQL\16\bin`
- servico padrao no Windows:
  - `postgresql-x64-16`
- data dir padrao:
  - `%LOCALAPPDATA%\lista-da-vez\postgres16\data`
- data dir do modo service:
  - `C:\Program Files\PostgreSQL\16\data`
- log padrao:
  - `%LOCALAPPDATA%\lista-da-vez\postgres16\postgres.log`
- usuario da app:
  - `lista_da_vez`
- senha da app:
  - `lista_da_vez_dev`
- banco da app:
  - `lista_da_vez`

## Variaveis opcionais

- `PG_BIN`
- `PG_SERVICE`
- `PGDATA`
- `PGPORT`
- `PG_SUPERUSER`
- `PG_SUPERPASSWORD`
- `APP_DB_USER`
- `APP_DB_PASSWORD`
- `APP_DB_NAME`

## Relacao com o backend

Depois de subir o banco local, a API Go espera `DATABASE_URL` apontando para a role/database da aplicacao, e nao para o superuser.

## Modo preferido

Sempre preferir o modo `service` quando o PostgreSQL 16 estiver instalado pelo instalador do Windows. O modo `manual` continua como fallback para maquinas sem servico registrado.
