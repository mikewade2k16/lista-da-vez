# AC-04b — Auto-provisão da role `omni_app` no `migrate up` (deploy self-healing)

> Spec de implementação · Prioridade **P1** · Esforço **S** · Impacto **alto**
> Origem: incidente de produção 2026-07-03 (registro de falhas nº 13 do ENGINEERING_PRINCIPLES) · roadmap `ac-fixes-2026-07` → task `ac-04b-migrate-auto-provision-role`

## 1. Contexto

**O achado:** o AC-04 fez a api conectar como a role least-privilege `omni_app`, mas a CRIAÇÃO da
role é passo manual de runbook (`scripts/db/create-app-role.sql` + envs na VPS). Em 2026-07-03 um
`deploy:fast:prod` subiu SEM esse passo → api em crash-loop `28P01` (o mock SCRAM do Postgres
devolve "password authentication failed" até para role INEXISTENTE), web preso em `Created`,
**~1h de 502 em produção**.

Evidências:
- `back/cmd/migrate/main.go:46-55` — o case `"up"` chama `database.SyncAppRoleGrants(...)`, que
  APENAS aplica grants; com role ausente retorna `(false, nil)` e loga `app_role_grants_skipped`.
- `back/internal/platform/database/app_role_grants.go:48-56` — o ponto exato onde a role ausente é
  detectada (`select exists(select 1 from pg_roles where rolname = $1)`) e o Sync desiste.
- `scripts/deploy/deploy-pull.ps1:158-162` — comentário registrando o incidente e apontando esta
  task como prevenção.
- `scripts/db/create-app-role.sql` — o SQL manual de criação (idempotente, via `format + \gexec`).

**Por que agora:** o migrate roda como superuser (`DATABASE_URL`) ANTES da api subir — ele tem tudo
que precisa para criar/curar a role sozinho. Com isso o deploy vira self-healing: nenhum ambiente
novo (prod, staging D3, dev com volume limpo) pode mais cair no 28P01.

## 2. Objetivo e não-objetivos

**Objetivo (escopo fechado):**
1. O `migrate up` passa a garantir a existência da role de app (nome+senha extraídos de
   `DATABASE_APP_URL`) ANTES de sincronizar os grants: cria se não existe, converge senha e
   atributos sempre (idempotente), concede `CONNECT`.
2. Em `production`, `DATABASE_APP_URL` sem senha (ou vazia) derruba o migrate ALTO E CEDO
   (`os.Exit(1)`) — nunca mais crash-loop silencioso da api.
3. Em dev sem role dedicada (app e migrate na mesma role), comportamento atual preservado (skip).

**Não-objetivos (explicitamente FORA):**
- NÃO alterar `SyncAppRoleGrants` (contrato intacto — continua skip se role ausente; é defesa em profundidade).
- NÃO remover `scripts/db/create-app-role.sql` nem `scripts/db/postgres-init/10-app-role.sh`
  (continuam como fallback manual e caminho do initdb).
- NÃO mexer em `DATABASE_URL`/pool da api, nem em RLS.

## 3. Regras de execução (obrigatórias para o implementador)

- NENHUM comando git (o dono commita).
- Validação back = `docker compose up -d --build api` (build local; nunca deploy).
- Máx 450 linhas por arquivo; Go sem lib uuid externa; scan nullable com `*string`.
- Nunca tocar `password_hash` de usuário; portas api 9091 / web 3003 / postgres 5432.
- Atualizar o AGENT.md do módulo tocado (`back/internal/platform/database/AGENT.md`).

## 4. Mudanças (passo a passo)

### 4.1 CRIAR `back/internal/platform/database/app_role_ensure.go`

```go
package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// EnsureAppRoleResult descreve o resultado da auto-provisão da role de app.
type EnsureAppRoleResult struct {
	Created    bool   // role não existia e foi criada nesta execução
	Synced     bool   // senha/atributos/CONNECT convergidos
	SkipReason string // "empty_url" | "same_role" | "empty_password" | "" (não pulou)
}

// EnsureAppRole garante que a role de runtime da app (AC-04b) exista com a
// senha e os atributos esperados, extraídos de DATABASE_APP_URL. Roda como a
// role privilegiada do migrate ANTES de SyncAppRoleGrants — juntas elas tornam
// o deploy self-healing (incidente 2026-07-03: role ausente = api em
// crash-loop 28P01). Não decide política de erro: skips voltam em SkipReason
// e o caller (cmd/migrate) decide se falha (production) ou segue (dev).
func EnsureAppRole(ctx context.Context, pool *pgxpool.Pool, appDatabaseURL string) (EnsureAppRoleResult, error) {
	if appDatabaseURL == "" {
		return EnsureAppRoleResult{SkipReason: "empty_url"}, nil
	}

	appCfg, err := pgxpool.ParseConfig(appDatabaseURL)
	if err != nil {
		return EnsureAppRoleResult{}, fmt.Errorf("parse app database url: %w", err)
	}
	roleName := appCfg.ConnConfig.User
	password := appCfg.ConnConfig.Password

	// App e migrate na mesma role (dev local sem role dedicada): nada a criar.
	if roleName == pool.Config().ConnConfig.User {
		return EnsureAppRoleResult{SkipReason: "same_role"}, nil
	}
	if !roleNamePattern.MatchString(roleName) {
		return EnsureAppRoleResult{}, fmt.Errorf("app role name invalido: %q", roleName)
	}
	if password == "" {
		return EnsureAppRoleResult{SkipReason: "empty_password"}, nil
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return EnsureAppRoleResult{}, fmt.Errorf("begin ensure app role: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// A senha entra no DDL montado (CREATE/ALTER ROLE não aceita bind param);
	// silenciar o statement log desta tx para ela não vazar no log do Postgres.
	if _, err := tx.Exec(ctx, "set local log_statement = 'none'"); err != nil {
		return EnsureAppRoleResult{}, fmt.Errorf("silence statement log: %w", err)
	}

	var exists bool
	if err := tx.QueryRow(ctx,
		`select exists(select 1 from pg_roles where rolname = $1)`, roleName,
	).Scan(&exists); err != nil {
		return EnsureAppRoleResult{}, fmt.Errorf("check app role exists: %w", err)
	}

	// Quoting seguro delegado ao próprio Postgres via format(%I/%L) — mesmo
	// padrão do scripts/db/create-app-role.sql, sem montar literal em Go.
	var ddl string
	if !exists {
		if err := tx.QueryRow(ctx,
			`select format('create role %I login', $1::text)`, roleName,
		).Scan(&ddl); err != nil {
			return EnsureAppRoleResult{}, fmt.Errorf("format create role: %w", err)
		}
		if _, err := tx.Exec(ctx, ddl); err != nil {
			return EnsureAppRoleResult{}, fmt.Errorf("create app role: %w", err)
		}
	}

	// SEMPRE convergir senha + atributos (cura rotação de APP_DB_ROLE_PASSWORD
	// e role pré-existente com atributos errados). Idempotente, 1x por boot.
	if err := tx.QueryRow(ctx,
		`select format('alter role %I with login password %L nosuperuser nocreatedb nocreaterole nobypassrls noreplication', $1::text, $2::text)`,
		roleName, password,
	).Scan(&ddl); err != nil {
		return EnsureAppRoleResult{}, fmt.Errorf("format alter role: %w", err)
	}
	if _, err := tx.Exec(ctx, ddl); err != nil {
		return EnsureAppRoleResult{}, fmt.Errorf("alter app role: %w", err)
	}

	if err := tx.QueryRow(ctx,
		`select format('grant connect on database %I to %I', current_database(), $1::text)`,
		roleName,
	).Scan(&ddl); err != nil {
		return EnsureAppRoleResult{}, fmt.Errorf("format grant connect: %w", err)
	}
	if _, err := tx.Exec(ctx, ddl); err != nil {
		return EnsureAppRoleResult{}, fmt.Errorf("grant connect: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return EnsureAppRoleResult{}, fmt.Errorf("commit ensure app role: %w", err)
	}

	return EnsureAppRoleResult{Created: !exists, Synced: true}, nil
}
```

### 4.2 EDITAR `back/cmd/migrate/main.go` — case `"up"`

Inserir o bloco abaixo IMEDIATAMENTE ANTES da chamada existente
`database.SyncAppRoleGrants` (hoje linha 46; re-ler o arquivo antes):

```go
		ensured, err := database.EnsureAppRole(ctx, pool, cfg.DatabaseAppURL)
		if err != nil {
			logger.Error("app_role_ensure_failed", slog.Any("error", err))
			os.Exit(1)
		}
		switch {
		case ensured.SkipReason == "":
			logger.Info("app_role_ensure_ok", slog.Bool("created", ensured.Created))
		case strings.EqualFold(cfg.Env, "production") &&
			(ensured.SkipReason == "empty_password" || ensured.SkipReason == "empty_url"):
			// Fail-fast: subir a api sem role utilizável = crash-loop 28P01
			// (incidente 2026-07-03). Melhor derrubar o migrate com motivo claro.
			logger.Error("app_role_ensure_failed", slog.String("reason", ensured.SkipReason))
			os.Exit(1)
		default:
			logger.Info("app_role_ensure_skipped", slog.String("reason", ensured.SkipReason))
		}
```

Os imports (`strings`, `slog`, `os`) já existem no arquivo. O bloco de
`SyncAppRoleGrants` que vem depois NÃO muda.

### 4.3 EDITAR `scripts/db/create-app-role.sql` — só o comentário do header

Adicionar ao topo (mantendo o SQL intacto):

```sql
-- DESDE O AC-04b o `migrate up` auto-provisiona esta role no boot (cria +
-- converge senha/atributos + grant connect, a partir de DATABASE_APP_URL).
-- Este script permanece como FALLBACK MANUAL e como caminho do initdb
-- (scripts/db/postgres-init/10-app-role.sh) para volumes novos.
```

### 4.4 CRIAR `back/internal/platform/database/app_role_ensure_test.go`

Teste de integração no padrão de `app_role_grants_test.go` (guardado por
`TEST_DATABASE_URL`; reusar os helpers de lá — ex.: montagem de URL com outra
role e limpeza de role de teste). Casos:

1. **Cria role nova:** URL com role inexistente + senha → `Created=true, Synced=true`;
   conectar com `pgx.Connect` usando a URL da app FUNCIONA.
2. **Idempotência:** 2ª chamada → `Created=false, Synced=true`; sem erro.
3. **Rotação de senha:** mesma role, senha nova na URL → conexão com a senha NOVA funciona
   e com a antiga falha.
4. **Skips (sem tocar DDL):** URL vazia → `empty_url`; URL com a MESMA role do pool → `same_role`;
   URL sem senha → `empty_password`. Em todos, `pg_roles` não ganha linha nova.
5. **Nome inválido:** role `evil"role` na URL → erro (roleNamePattern).

Cleanup: `drop role if exists <role de teste>` em `t.Cleanup`.

### 4.5 EDITAR docs

- `back/internal/platform/database/AGENT.md`: na seção dos dois pools/AC-04, registrar que o
  `migrate up` agora auto-provisiona a role (EnsureAppRole → SyncAppRoleGrants) e a política
  fail-fast em production.
- `docs/MULTITENANT_COMPLETION_PLAN.md` § AC-04: no bloco de Status, acrescentar
  "ac-04b IMPLEMENTADO: o migrate auto-provisiona a role; os passos manuais 2–4 viram fallback".
- `scripts/deploy/deploy-pull.ps1:158-162`: atualizar o comentário — o pré-requisito manual deixou
  de existir a partir da imagem que contém o AC-04b (manter o aviso para imagens antigas).

## 5. Critérios de aceite

1. `docker compose up -d --build api` com volume postgres LIMPO (`docker compose down -v` antes) e
   SEM rodar `create-app-role.sql`: api sobe saudável, sem `28P01`, logs mostram
   `app_role_ensure_ok created=true` seguido de `app_role_grants_ok`.
2. Segundo boot: `app_role_ensure_ok created=false` (idempotente).
3. Trocar `APP_DB_ROLE_PASSWORD` no `.env` + recriar api → api conecta com a senha nova sem passo manual.
4. Simular production sem senha (`APP_ENV=production` + `DATABASE_APP_URL` sem senha, via
   `docker compose run --rm api sh -lc 'APP_ENV=production DATABASE_APP_URL=postgres://omni_app@postgres:5432/omni migrate up'`):
   migrate sai com código ≠ 0 e loga `app_role_ensure_failed reason=empty_password`.
5. `go test ./internal/platform/database/...` verde (com `TEST_DATABASE_URL` apontando pro postgres local).
6. Nenhuma senha aparece no log do Postgres (`docker compose logs postgres | grep -i "alter role"` vazio).

## 6. Validação

```bash
docker compose down -v && docker compose up -d --build api   # cenário volume limpo
docker compose logs api | grep -E "app_role_ensure|app_role_grants|28P01"
docker compose exec -T postgres psql -U omni -d omni -tc "select rolname, rolsuper, rolbypassrls from pg_roles where rolname='omni_app'"
# esperado: omni_app | f | f
cd back && go build ./... && go vet ./...
```

Smoke autenticado (login no painel) fica com o dono.

## 7. Notas de Deploy

- **Migrations:** nenhuma. **Env vars novas:** nenhuma. **Rebuild:** api (`back/` mudou) —
  em prod o caminho normal GHCR (`deploy:fast:prod` ou CI).
- A partir desta imagem, criar a role deixa de ser pré-requisito manual em QUALQUER ambiente
  (prod/staging/dev). O runbook AC-04 passos 2–4 vira fallback.
- Rollback: voltar a imagem anterior (a role já criada permanece — nada a desfazer no banco).

## 8. Arquivos tocados

| Arquivo | Ação |
|---|---|
| `back/internal/platform/database/app_role_ensure.go` | criar |
| `back/internal/platform/database/app_role_ensure_test.go` | criar |
| `back/cmd/migrate/main.go` | editar (case "up") |
| `scripts/db/create-app-role.sql` | editar (comentário) |
| `back/internal/platform/database/AGENT.md` | editar |
| `docs/MULTITENANT_COMPLETION_PLAN.md` | editar (§ AC-04 status) |
| `scripts/deploy/deploy-pull.ps1` | editar (comentário 158-162) |

**Conflitos potenciais:** nenhum com as demais specs da rodada (D3 apenas se BENEFICIA — staging
deixa de precisar do passo manual).