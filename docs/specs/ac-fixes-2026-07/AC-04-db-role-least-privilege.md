# AC-04 — Role de runtime least-privilege no Postgres (`omni_app`)

> Prioridade P1 · Esforço M · Impacto alto · Origem: diagnóstico 2026-07-02 (achado canônico AC-04)

## 1. Contexto

**O achado:** a API conecta no Postgres como `omni` = `POSTGRES_USER` do container = **SUPERUSER** do cluster. Um único SQL injetado/bug de aplicação teria poder de DDL total (DROP TABLE, ALTER ROLE, COPY TO PROGRAM em outros setups). Além disso, `docs/RLS_PLAN.md` (linha 7) declara como **pré-requisito crítico** do RLS: *"criar um role de app dedicado, SEM SUPERUSER e SEM BYPASSRLS, com os GRANTs nas tabelas/schemas que ele usa, e apontar a DATABASE_URL (dev e prod) pra esse role"* — superuser ignora RLS por completo, então qualquer policy futura é no-op enquanto isso não mudar.

**Evidências (código atual):**

- `docker-compose.yml:39` — `DATABASE_URL: postgres://${POSTGRES_USER:-omni}:...@postgres:5432/...` (api usa o superuser do container postgres).
- `docker-compose.prod.yml:53` — idem em prod.
- `back/internal/platform/database/pool.go:14-19` — `OpenPool` usa só `cfg.DatabaseURL`; não existe URL separada para runtime.
- `back/internal/platform/config/config.go:154` — `DatabaseURL: getEnv("DATABASE_URL", "")`; não existe `DATABASE_APP_URL`.
- `back/cmd/api/main.go:27` e `back/cmd/migrate/main.go:23` — api e migrator abrem o MESMO pool com a MESMA URL.
- `back/Dockerfile:26` — `CMD ["sh", "-c", "migrate up && migrate bootstrap-erp-store && api"]`: migrate e api rodam no mesmo container, em sequência — dá pra dar URLs diferentes a cada binário sem mudar a orquestração.
- `fatos.json → banco.rls`: "app conecta como omni=SUPERUSER (...) replano em docs/RLS_PLAN.md exige role não-superuser primeiro".

**Por que agora:** é o passo 1 do RLS_PLAN, destrava defesa-em-profundidade multi-tenant, e é pré-requisito de software do cenário de escala (fatos.json → vps_estimativas.cenario_escala).

**Fato importante para o design:** o migrator PRECISA continuar privilegiado (roda DDL das migrations, `create schema`, etc.). Só o binário `api` muda de role. Como o `migrate up` roda a cada boot do container (CMD do Dockerfile), ele é o lugar perfeito para **re-sincronizar os GRANTs** da role de app a cada deploy — self-healing para schemas/tabelas novos.

## 2. Objetivo e não-objetivos

**Objetivo (escopo fechado):**

1. Criar role `omni_app` (LOGIN, `NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS NOREPLICATION`, sem CREATE em schema nenhum) via script SQL idempotente — **não** via migration (role é cluster-level e a senha vem de env).
2. Nova env `DATABASE_APP_URL`: o binário `api` conecta com ela; o binário `migrate` continua com `DATABASE_URL` (privilegiada).
3. GRANTs de runtime (USAGE em schemas, SELECT/INSERT/UPDATE/DELETE em tabelas/views, USAGE/SELECT/UPDATE em sequences) sincronizados automaticamente pelo `migrate up` a cada boot + `ALTER DEFAULT PRIVILEGES` global para objetos futuros criados pelo `omni`.
4. Compose dev e prod atualizados; dev com bootstrap automático da role em volume novo (initdb.d) e comando único para volume existente.
5. Guard de produção: `Validate()` exige `DATABASE_APP_URL` quando `APP_ENV=production`.
6. Teste de integração (positivo DML + negativo DDL) e runbook de prod.

**Não-objetivos (explicitamente NÃO fazer):**

- NÃO implementar RLS/policies (isso é o RLS_PLAN inteiro; aqui é só o pré-requisito).
- NÃO mexer no `Querier`/camada de conexão por request do RLS_PLAN §2.
- NÃO trocar a role dos comandos `migrate up|bootstrap-owner|bootstrap-erp-store` (continuam com `DATABASE_URL`).
- NÃO tocar em `password_hash` nem em nenhum dado de usuário (a role nova é do Postgres, não de `core.users`).
- NÃO mudar portas (api 9091 host, web 3003, postgres 5432), nem `POSTGRES_USER`/`POSTGRES_PASSWORD` existentes.
- NÃO conceder TRUNCATE/EXECUTE extra: grep confirmou zero `TRUNCATE`, zero `REFRESH MATERIALIZED VIEW`, zero `LISTEN` no runtime Go; funções pgcrypto têm EXECUTE para PUBLIC por default.
- NÃO rodar nenhum comando git; NÃO rodar npm/generate/build do web.

## 3. Regras de execução (obrigatórias para o implementador)

- **NENHUM comando git** (sessão multi-agente — só o usuário roda git).
- Validação do back: **rodar** `docker compose up -d --build api` quando `back/` mudar (build local, não é deploy). NÃO rodar npm/vitest/generate — deixar listado para o usuário aprovar.
- Máx **450 linhas** por arquivo novo/refatorado.
- Não remover funcionalidade existente; `OpenPool` continua existindo e funcionando como hoje.
- Zero mock/legado novo; nada de senha hardcoded — senha da role vem de env (`APP_DB_ROLE_PASSWORD`).
- Go: **sem lib uuid externa**; scan nullable com `*string` (não se aplica aqui, mas vale para qualquer ajuste).
- **NUNCA** sobrescrever `password_hash`/dados de usuário; nenhum passo deste AC toca dados.
- Atualizar os AGENT.md dos módulos tocados ao final (lista na seção 8).
- Migrations: este AC **não cria migration** (decisão de design — ver 4.1); se o implementador julgar necessário criar alguma, a numeração seguinte é 0187, SQL plano idempotente, SEM `-- +goose Down`.

## 4. Mudanças (passo a passo)

### 4.1 Criar `scripts/db/create-app-role.sql` (NOVO)

Script SQL **idempotente**, cluster-level, executado por psql com variáveis (`-v role=... -v pw=...`). Ele SÓ cria/atualiza a role e o CONNECT — os GRANTs de objetos ficam no Go (4.3), fonte única, re-sincronizada a cada boot. Usa o padrão `format(...) \gexec` porque psql **não** interpola `:'var'` dentro de dollar-quoting de bloco `DO`.

```sql
-- scripts/db/create-app-role.sql — AC-04: role de RUNTIME least-privilege da api.
-- Idempotente. NAO e migration (role e cluster-level; senha vem de env).
-- Uso: psql -v ON_ERROR_STOP=1 -U <superuser> -d <db> -v role=omni_app -v pw='<senha>' -f scripts/db/create-app-role.sql
-- Os GRANTs de tabelas/sequences sao aplicados pelo `migrate up` (SyncAppRoleGrants)
-- a cada boot da api — este arquivo cuida apenas de: existencia, senha, atributos, CONNECT.

\set ON_ERROR_STOP on

select format('create role %I login', :'role')
where not exists (select 1 from pg_roles where rolname = :'role')
\gexec

select format(
  'alter role %I with login password %L nosuperuser nocreatedb nocreaterole nobypassrls noreplication',
  :'role', :'pw')
\gexec

select format('grant connect on database %I to %I', current_database(), :'role')
\gexec
```

Decisões já tomadas: nome fixo `omni_app` (default; configurável por `APP_DB_ROLE`); sem `NOINHERIT` (a role não é membro de nada); sem `GRANT CREATE` em schema algum (PG16 já não dá CREATE em `public` para não-owner — o teste negativo prova).

### 4.2 Criar `scripts/db/postgres-init/10-app-role.sh` (NOVO)

Wrapper para o `docker-entrypoint-initdb.d` do postgres dev (roda no primeiro init de volume novo) e reutilizável via `docker compose exec` em volume existente:

```sh
#!/bin/sh
# AC-04: cria a role de runtime da api no postgres do compose dev.
# Roda automatico no primeiro init do volume (docker-entrypoint-initdb.d)
# e manualmente em volume existente:
#   docker compose exec -T postgres sh /docker-entrypoint-initdb.d/10-app-role.sh
set -eu
: "${APP_DB_ROLE:=omni_app}"
: "${APP_DB_ROLE_PASSWORD:?APP_DB_ROLE_PASSWORD obrigatoria}"
psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" \
  -v role="$APP_DB_ROLE" -v pw="$APP_DB_ROLE_PASSWORD" \
  -f /scripts/db/create-app-role.sql
```

### 4.3 Criar `back/internal/platform/database/app_role_grants.go` (NOVO, ~120 linhas)

Sincroniza GRANTs para a role de app. Roda como a role privilegiada (pool do migrate). Assinatura:

```go
// SyncAppRoleGrants garante os GRANTs de runtime da role da app (AC-04):
// USAGE nos schemas, SELECT/INSERT/UPDATE/DELETE em tabelas/views,
// USAGE/SELECT/UPDATE em sequences, e ALTER DEFAULT PRIVILEGES global para
// objetos futuros criados pela role de migration. Retorna (false, nil) quando
// nao ha o que fazer: appDatabaseURL vazia, mesma role do pool, ou role
// inexistente no cluster (o caller loga e segue).
func SyncAppRoleGrants(ctx context.Context, pool *pgxpool.Pool, appDatabaseURL string) (bool, error)
```

Implementação (decisões fechadas):

1. `appDatabaseURL == ""` → `return false, nil`.
2. `appCfg, err := pgxpool.ParseConfig(appDatabaseURL)` → erro se inválida. `roleName := appCfg.ConnConfig.User`.
3. Se `roleName == pool.Config().ConnConfig.User` → `return false, nil` (app e migrate na mesma role; nada a conceder).
4. Validar `roleName` com regex `^[a-z_][a-z0-9_]*$` (sem uuid lib, sem identifier injection) → erro se não casar.
5. `select exists(select 1 from pg_roles where rolname = $1)` → se `false`, `return false, nil`.
6. Listar schemas: `select nspname from pg_namespace where nspname not in ('information_schema') and nspname not like 'pg\_%' escape '\' order by nspname` (pega `public`, `core`, `queue`, `tasks`, `notifications`, `roadmap`, `site`, `automation`, `bio`, `meta_ads`, `cardapio`, `calendar` e qualquer schema futuro — NÃO hardcodar a lista).
7. Para cada schema `s`, com `roleIdent := pgx.Identifier{roleName}.Sanitize()` e `schemaIdent := pgx.Identifier{s}.Sanitize()`, executar via `pool.Exec` (fmt.Sprintf com os identifiers sanitizados — não há placeholder para identifier):
   - `GRANT USAGE ON SCHEMA <s> TO <role>`
   - `GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA <s> TO <role>` (inclui as views legadas de `public.*` — view entra em ALL TABLES)
   - `GRANT USAGE, SELECT, UPDATE ON ALL SEQUENCES IN SCHEMA <s> TO <role>`
8. Uma vez (fora do loop), default privileges **globais** da role corrente (o migrator), cobrindo tabelas/sequences futuras em QUALQUER schema criado por migrations:
   - `ALTER DEFAULT PRIVILEGES GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO <role>`
   - `ALTER DEFAULT PRIVILEGES GRANT USAGE, SELECT, UPDATE ON SEQUENCES TO <role>`
   (sem `FOR ROLE` e sem `IN SCHEMA` de propósito: aplica ao `current_user` = role do migrate, em todos os schemas.)
9. `return true, nil`. **NÃO** conceder: CREATE, TRUNCATE, REFERENCES, TRIGGER, EXECUTE extra.

Nota: schema novo criado por migration futura é coberto porque o sync roda DEPOIS de `ApplyMigrations` em todo boot (4.4) — o `GRANT USAGE` do schema novo entra no próximo passo do mesmo `migrate up`.

### 4.4 Editar `back/cmd/migrate/main.go`

No case `"up"`, após o `logger.Info("migration_up_ok")` (linha 44):

```go
granted, err := database.SyncAppRoleGrants(ctx, pool, cfg.DatabaseAppURL)
if err != nil {
    logger.Error("app_role_grants_failed", slog.Any("error", err))
    os.Exit(1)
}
if granted {
    logger.Info("app_role_grants_ok")
} else {
    logger.Info("app_role_grants_skipped") // sem DATABASE_APP_URL, mesma role, ou role ausente
}
```

O skip é `Info` (não `Warn`): dev local sem a role continua funcionando; quem trava produção sem a env é o `Validate()` da api (4.6).

### 4.5 Editar `back/internal/platform/database/pool.go`

Refatorar mantendo `OpenPool` intacto para os callers atuais e adicionando o pool de runtime:

```go
func OpenPool(ctx context.Context, cfg config.Config) (*pgxpool.Pool, error) {
    return openPoolWithURL(ctx, cfg, cfg.DatabaseURL)
}

// OpenAppPool abre o pool de RUNTIME da api com DATABASE_APP_URL (role
// least-privilege omni_app, AC-04). Fallback para DATABASE_URL quando a app
// URL nao esta definida (dev local sem a role — em production o Validate()
// da config impede esse fallback).
func OpenAppPool(ctx context.Context, cfg config.Config) (*pgxpool.Pool, error) {
    url := cfg.DatabaseAppURL
    if url == "" {
        url = cfg.DatabaseURL
    }
    return openPoolWithURL(ctx, cfg, url)
}

func openPoolWithURL(ctx context.Context, cfg config.Config, url string) (*pgxpool.Pool, error) {
    // corpo IGUAL ao OpenPool atual (pool.go:15-46), trocando cfg.DatabaseURL por url
}
```

Tuning existente (MinConns/MaxConns/idle/lifetime/healthcheck, pool.go:24-34) permanece idêntico e vale para os dois pools.

### 4.6 Editar `back/internal/platform/config/config.go`

1. Struct (após `DatabaseURL`, linha 60): `DatabaseAppURL string`.
2. `Load()` (após linha 154): `DatabaseAppURL: getEnv("DATABASE_APP_URL", ""),`.
3. `Validate()` (dentro do bloco production, junto dos checks das linhas 199-204):

```go
if strings.TrimSpace(cfg.DatabaseAppURL) == "" {
    problems = append(problems, "DATABASE_APP_URL ausente (AC-04: a api deve conectar com a role least-privilege omni_app, nunca com o superuser)")
}
```

Não alterar a semântica `Env == "production"` do guard existente (isso é escopo do AC-09 — não conflitar).

### 4.7 Editar `back/cmd/api/main.go`

- Linha 27: trocar `database.OpenPool(ctx, cfg)` por `database.OpenAppPool(ctx, cfg)`.
- Após abrir o pool, logar a role efetiva (nunca a senha): `logger.Info("database_connected", slog.String("db_user", pool.Config().ConnConfig.User))`.

### 4.8 Editar `docker-compose.yml` (dev)

Serviço `postgres` — adicionar env + mounts (manter healthcheck/portas como estão):

```yaml
    environment:
      POSTGRES_DB: ${POSTGRES_DB:-omni}
      POSTGRES_USER: ${POSTGRES_USER:-omni}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-omni_dev}
      APP_DB_ROLE: ${APP_DB_ROLE:-omni_app}
      APP_DB_ROLE_PASSWORD: ${APP_DB_ROLE_PASSWORD:-omni_app_dev}
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./scripts/db:/scripts/db:ro
      - ./scripts/db/postgres-init:/docker-entrypoint-initdb.d:ro
```

Serviço `api` — adicionar logo abaixo de `DATABASE_URL` (linha 39):

```yaml
      # AC-04: URL de RUNTIME da api (role least-privilege omni_app; sem DDL).
      # O binario migrate continua com DATABASE_URL (privilegiada, roda DDL).
      DATABASE_APP_URL: postgres://${APP_DB_ROLE:-omni_app}:${APP_DB_ROLE_PASSWORD:-omni_app_dev}@postgres:5432/${POSTGRES_DB:-omni}?sslmode=disable
```

### 4.9 Editar `docker-compose.prod.yml`

Serviço `api` — abaixo de `DATABASE_URL` (linha 53):

```yaml
      # AC-04: role de runtime least-privilege. Criar a role ANTES do deploy
      # (runbook: docs/MULTITENANT_COMPLETION_PLAN.md, Notas de Deploy AC-04).
      DATABASE_APP_URL: postgres://${APP_DB_ROLE:-omni_app}:${APP_DB_ROLE_PASSWORD}@postgres:5432/${POSTGRES_DB}?sslmode=disable
```

NÃO montar initdb.d no postgres de prod (criação da role em prod é manual, via runbook — evita script auto-executando em banco de produção).

### 4.10 Editar os `.env` examples

- `.env.docker.example` — após o bloco `POSTGRES_*` (linhas 1-4):
  ```
  # AC-04: role de RUNTIME da api (least-privilege, sem DDL). O migrate continua
  # usando POSTGRES_USER. Senha alfanumerica (evita urlencode na DATABASE_APP_URL).
  APP_DB_ROLE=omni_app
  APP_DB_ROLE_PASSWORD=omni_app_dev
  ```
- `.env.production.example` — após `POSTGRES_PASSWORD` (linha 19): mesmas 2 chaves, com `APP_DB_ROLE_PASSWORD=troque-essa-senha-forte-alfanumerica` e o mesmo comentário + "criar a role antes do deploy (runbook AC-04)".
- `.env.staging.example` — idem no bloco de banco (segredo próprio do staging, nunca o de prod).
- `back/.env.example` — após `DATABASE_URL` (linha 6): `# DATABASE_APP_URL=postgres://omni_app:omni_app_dev@localhost:5432/omni?sslmode=disable` comentada + 1 linha explicando (dev local fora do docker funciona sem ela via fallback).

### 4.11 Criar `back/internal/platform/database/app_role_grants_test.go` (NOVO)

Teste de integração no padrão de `migration_integration_test.go:21-25` (skip sem `TEST_DATABASE_URL`):

```go
func TestAppRoleGrantsLeastPrivilege(t *testing.T) {
    // 1. abre pool com TEST_DATABASE_URL (superuser do CI/dev) e roda ApplyMigrations
    //    (idempotente — mesmo padrao do TestAllMigrationsApply).
    // 2. cria role de teste: drop owned/if exists + create role ac04_test_app login password 'ac04-test-pw'
    //    (defer: DROP OWNED BY ac04_test_app; DROP ROLE ac04_test_app).
    // 3. monta appURL trocando user/password da TEST_DATABASE_URL e chama
    //    SyncAppRoleGrants(ctx, pool, appURL) → espera (true, nil).
    // 4. conecta como ac04_test_app (pgx.Connect com a appURL):
    //    a. POSITIVO: `select count(*) from core.users` → sem erro.
    //    b. NEGATIVO: `create table public.ac04_should_fail(x int)` → espera erro
    //       SQLSTATE 42501 (insufficient_privilege).
    //    c. NEGATIVO: `create schema ac04_hack` → espera 42501.
    // 5. chama SyncAppRoleGrants de novo → (true, nil) (idempotencia).
}
```

Asserção do SQLSTATE via `pgconn.PgError` (`errors.As`) — sem lib nova. O CI (`build-images.yml`, job test) já exporta `TEST_DATABASE_URL` com service Postgres 16 superuser, então o teste roda no gate sem mudança de workflow.

### 4.12 Documentação (mesmo PR de implementação)

1. `back/internal/platform/database/AGENT.md` — nova seção curta "Role de runtime (AC-04)": api = `OpenAppPool`/`DATABASE_APP_URL`/`omni_app` sem DDL; migrate = `OpenPool`/`DATABASE_URL` privilegiada; GRANTs auto-sincronizados por `SyncAppRoleGrants` em todo `migrate up`; script canônico `scripts/db/create-app-role.sql`.
2. `back/internal/platform/config/AGENT.md` — documentar `DATABASE_APP_URL` (e o check de `Validate()` em production).
3. `docs/RLS_PLAN.md` — no bloco "PRÉ-REQUISITO CRÍTICO" (linha 7), acrescentar: `**ATENDIDO (AC-04, 2026-07-02):** role omni_app criada via scripts/db/create-app-role.sql + DATABASE_APP_URL; grants sincronizados por SyncAppRoleGrants a cada migrate up. A fundação RLS pode sair do estado inerte quando for retomada.`
4. `docs/MULTITENANT_COMPLETION_PLAN.md` — apêndice na seção `## Notas de Deploy` (linha 587): subseção `### AC-04 (2026-07-02) — role de runtime least-privilege` com o runbook da seção 7 desta spec (ordem exata).

## 5. Critérios de aceite (verificar um a um)

1. `scripts/db/create-app-role.sql` existe, é idempotente (rodar 2× não dá erro) e não contém senha hardcoded.
2. Volume dev NOVO: `docker compose up -d` cria `omni_app` automaticamente (initdb.d). Volume dev EXISTENTE: `docker compose exec -T postgres sh /docker-entrypoint-initdb.d/10-app-role.sh` cria/atualiza a role.
3. `docker compose logs api` mostra, nesta ordem: `migration_up_ok` → `app_role_grants_ok` → `database_connected db_user=omni_app` → `api_listening`.
4. Como `omni_app`: `select count(*) from core.users` funciona; `create table public.ac04_negative(x int)` e `create schema ac04_hack` retornam `permission denied` (42501).
5. `pg_roles` confirma: `rolsuper=f, rolcreatedb=f, rolcreaterole=f, rolbypassrls=f, rolreplication=f` para `omni_app`.
6. `migrate up` continua rodando como `omni` (DDL das migrations intacto); `migrate bootstrap-erp-store` inalterado.
7. Sem `DATABASE_APP_URL` e `APP_ENV=production`, a api aborta o boot com `config_invalid` citando AC-04; em dev/docker sem a env, a api sobe com fallback para `DATABASE_URL` (comportamento atual preservado — feature coexiste).
8. `TestAppRoleGrantsLeastPrivilege` passa com `TEST_DATABASE_URL` apontando para o postgres dev.
9. Painel dev funciona de ponta a ponta com a role nova (login, operação, uma listagem CRM) — smoke visual pelo usuário.
10. Nenhum arquivo novo/refatorado passa de 450 linhas; AGENT.md e docs da seção 4.12 atualizados.

## 6. Validação (comandos exatos, dev local)

```bash
# 0) conferir a porta host do postgres (memoria: 5432 pode ser o postgres nativo do Windows)
docker compose port postgres 5432

# 1) recriar postgres com os mounts novos (volume/dados preservados) e criar a role
docker compose up -d postgres
docker compose exec -T postgres sh /docker-entrypoint-initdb.d/10-app-role.sh
# idempotencia: rodar de novo, deve terminar OK
docker compose exec -T postgres sh /docker-entrypoint-initdb.d/10-app-role.sh

# 2) rebuild da api (back/ mudou — regra do projeto: PODE e DEVE rodar)
docker compose up -d --build api
docker compose logs api --tail 50   # esperar: migration_up_ok, app_role_grants_ok, database_connected db_user=omni_app, api_listening

# 3) smoke http (porta host da api = 9091)
curl -s http://localhost:9091/healthz

# 4) atributos da role + teste negativo (DDL negado) + positivo (DML ok)
docker compose exec -T postgres psql -U omni -d omni -c "select rolname, rolsuper, rolcreatedb, rolcreaterole, rolbypassrls from pg_roles where rolname='omni_app';"
docker compose exec -T postgres psql -U omni_app -d omni -c "create table public.ac04_negative(x int);"   # ESPERADO: ERROR permission denied for schema public
docker compose exec -T postgres psql -U omni_app -d omni -c "create schema ac04_hack;"                     # ESPERADO: ERROR permission denied for database omni
docker compose exec -T postgres psql -U omni_app -d omni -c "select count(*) from core.users;"             # ESPERADO: numero, sem erro

# 5) teste de integracao Go (ajustar a porta conforme o passo 0)
cd back && TEST_DATABASE_URL="postgres://omni:omni_dev@localhost:5432/omni?sslmode=disable" go test ./internal/platform/database/ -run TestAppRoleGrantsLeastPrivilege -v
```

Smoke autenticado (login no painel http://localhost:3003, abrir Operação e uma lista do CRM): **deixar para o usuário** — nunca inventar credenciais. Validação web via npm/vitest: nenhuma necessária (zero mudança em `web/`).

## 7. Notas de Deploy (runbook de produção — ordem importa)

Replicar esta subseção em `docs/MULTITENANT_COMPLETION_PLAN.md → ## Notas de Deploy` (item 4.12.4).

1. **Backup** (padrão existente): `pg_dump` via deploy-ship/deploy-vps antes de qualquer coisa.
2. **Env na VPS** (`/home/deploy/lista-atendimento/.env`): adicionar `APP_DB_ROLE=omni_app` e `APP_DB_ROLE_PASSWORD=<senha forte ALFANUMÉRICA>` (evita urlencode na URL). Nunca reutilizar `POSTGRES_PASSWORD`.
3. **Copiar o script** para a VPS (ele não está na imagem): `scp scripts/db/create-app-role.sql deploy@85.31.62.33:/home/deploy/lista-atendimento/scripts/db/` (criar a pasta se não existir).
4. **Criar a role ANTES do deploy da imagem nova** (idempotente; re-rodável):
   ```bash
   cd /home/deploy/lista-atendimento && set -a && . ./.env && set +a
   docker compose -f docker-compose.prod.yml exec -T postgres \
     sh -c "psql -v ON_ERROR_STOP=1 -U \$POSTGRES_USER -d \$POSTGRES_DB -v role=omni_app -v pw='$APP_DB_ROLE_PASSWORD'" \
     < scripts/db/create-app-role.sql
   ```
5. **Deploy da imagem nova** (fluxo normal GHCR pull + `up -d --no-build`). O `migrate up` do boot sincroniza os GRANTs (`app_role_grants_ok` no log) e a api sobe como `omni_app`. Nota: a imagem nova com `Validate()` **não sobe** em production sem `DATABASE_APP_URL` — por isso os passos 2-4 vêm antes.
6. **Validar**: `docker compose -f docker-compose.prod.yml logs api | grep -E "app_role_grants|database_connected|api_listening"` (esperar `db_user=omni_app`); smoke `curl -fsS https://omni.crowvisuals.com.br/healthz`; login no painel.
7. **Staging** (volume novo a cada subida): subir `postgres`, rodar o passo 4 com o env de staging, depois subir a api. Se a api subir antes da role, ela entra em crash-loop e se recupera sozinha no restart seguinte à criação da role (o `migrate up` do boot re-sincroniza).
8. **Rollback sem trocar imagem**: setar `APP_DB_ROLE=omni` e `APP_DB_ROLE_PASSWORD=$POSTGRES_PASSWORD` no `.env` + `up -d` (a app volta a conectar como superuser, temporário). Rollback completo: voltar a imagem anterior.
9. **Migrations novas**: nenhuma. **Env vars novas**: `DATABASE_APP_URL` (montada no compose), `APP_DB_ROLE`, `APP_DB_ROLE_PASSWORD`. **Rebuild**: `docker compose up -d --build api` no dev (obrigatório — back/ mudou).

## 8. Arquivos tocados (gestão de conflito)

**Criar:**
- `scripts/db/create-app-role.sql`
- `scripts/db/postgres-init/10-app-role.sh`
- `back/internal/platform/database/app_role_grants.go`
- `back/internal/platform/database/app_role_grants_test.go`

**Editar:**
- `back/internal/platform/database/pool.go`
- `back/internal/platform/config/config.go`
- `back/cmd/api/main.go`
- `back/cmd/migrate/main.go`
- `docker-compose.yml`
- `docker-compose.prod.yml`
- `.env.docker.example`
- `.env.production.example`
- `.env.staging.example`
- `back/.env.example`
- `back/internal/platform/database/AGENT.md`
- `back/internal/platform/config/AGENT.md`
- `docs/RLS_PLAN.md`
- `docs/MULTITENANT_COMPLETION_PLAN.md` (só a seção "Notas de Deploy")

**Conflitos potenciais com outros ACs:** AC-09 edita o MESMO `Validate()` de `config.go` (guard do dev secret — aqui só ADICIONAR o check de `DATABASE_APP_URL`, não mexer na condição `Env == "production"`); AC-11 edita os MESMOS `docker-compose*.yml` (mem limits/healthchecks — mudanças em chaves diferentes do mesmo serviço); AC-19 mexe em envs de pool (`DATABASE_MAX_CONNS`) nos mesmos composes. Coordenar merge por seção.
