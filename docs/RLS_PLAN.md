# Plano — Row-Level Security (RLS) no Postgres (SEC-1)

> Status: **DESIGN para revisão** (2026-06-29). Decisão do usuário: design primeiro, implementar só após o ok. Nenhuma migration/código foi escrito ainda.
>
> RLS é a mudança mais perigosa do projeto: feita errada, **tranca todo o acesso** (queries voltam 0 linhas) **ou vaza cross-tenant** (policy frouxa). Por isso este doc existe antes de qualquer linha.
>
> **PRÉ-REQUISITO CRÍTICO (descoberto na implementação da fundação, wave 4):** a app conecta como `omni` = `POSTGRES_USER` = **SUPERUSER**, e no Postgres **superuser ignora RLS por completo** (nem `FORCE ROW LEVEL SECURITY` aplica). Enquanto a app conectar como superuser, **toda policy RLS é um no-op silencioso** (falsa sensação de isolamento — a proteção real continua sendo só o filtro na aplicação). **Antes de RLS ter QUALQUER efeito** é obrigatório: criar um **role de app dedicado, SEM `SUPERUSER` e SEM `BYPASSRLS`**, com os `GRANT`s nas tabelas/schemas que ele usa, e apontar a `DATABASE_URL` (dev e prod) pra esse role. Isso é mudança de infra/deploy **com risco próprio** (GRANT incompleto = lockout total da app). A fundação (Querier + middleware + migration + teste) já está implementada e **fica inerte** até esse role existir.
>
> **ATENDIDO (AC-04, 2026-07-02):** role `omni_app` criada via `scripts/db/create-app-role.sql` + `DATABASE_APP_URL`; grants sincronizados por `SyncAppRoleGrants` a cada `migrate up`. A fundação RLS pode sair do estado inerte quando for retomada.

## 1. Objetivo e princípio

Hoje o isolamento multi-tenant é **100% na aplicação** (`resolveTenantScope` no service + filtro de `account_id`/`tenant_id` no repo). Se um handler novo esquecer o escopo, **vaza** — não há rede embaixo. RLS é essa rede: uma policy no banco que, mesmo com a query sem filtro, só devolve linhas do tenant ativo.

Princípio: **defesa em profundidade**. RLS NÃO substitui o `resolveTenantScope` — fica POR BAIXO dele. O alvo de aceite: remover o `resolveTenantScope` de um service de teste e a query AINDA assim não devolver linha de outro tenant.

## 2. O desafio real — a camada de conexão (a parte difícil)

A policy precisa saber "qual o tenant ativo desta request". O padrão Postgres é um **GUC** (`current_setting('app.account_id')`) que a policy lê. O problema: **a app usa um pool de conexões** (`pgxpool`), e os repositórios hoje guardam `*pgxpool.Pool` e fazem `pool.Query(...)` direto. Uma conexão do pool é **reusada entre requests/tenants** — então não dá pra setar o GUC "por conexão" no boot.

O GUC tem que ser setado **na mesma conexão que roda a query, no escopo da request**. Duas formas:

- **(A) Conexão por request (recomendado):** um middleware (após `RequireAuth`) faz `pool.Acquire`, roda `select set_config('app.account_id', $1, false)` (nível de sessão, sem precisar de transação), guarda a conn no `context`. Os repos passam a pegar a conn do context (com fallback pro pool). No fim da request: `reset all` + release. **Custo:** os repos hoje recebem `*pgxpool.Pool`; precisam aceitar uma interface `Querier` (tanto `*pgxpool.Pool` quanto `*pgx.Conn` satisfazem `Query/Exec/QueryRow`) e resolver a conn do context. É refactor da camada de dados — espalhado, mas mecânico.
- **(B) Transação por request:** envolver cada request numa tx com `SET LOCAL app.account_id`. Mais simples de garantir o reset (a tx fecha), MAS força TUDO numa transação (muda semântica de erro/rollback de handlers que hoje fazem queries soltas) e serializa o request numa conn só. Mais invasivo no comportamento.

**Recomendação: (A).** O `set_config(..., false)` é por sessão (dura a request), o middleware controla acquire/reset, e a interface `Querier` é uma mudança de tipo de baixo risco (a maioria dos repos só chama `Query/QueryRow/Exec`). É o ponto que mais quero seu ok antes de mexer.

## 3. Mecanismo da policy

Por tabela tenant-scoped:

```sql
alter table <schema>.<tabela> enable row level security;
alter table <schema>.<tabela> force row level security;  -- aplica ao dono da tabela tb
create policy <tabela>_tenant_isolation on <schema>.<tabela>
  using (<coluna_escopo> = current_setting('app.account_id', true)::uuid);
```

- `current_setting('app.account_id', true)` — o `true` (missing_ok) evita erro quando o GUC não está setado (boot, migrations, jobs): aí retorna `NULL` e a comparação dá `false` → **fail-closed** (0 linhas), nunca fail-open.
- Policy só de leitura (`using`) cobre `SELECT/UPDATE/DELETE`. Para `INSERT`, adicionar `with check (<coluna> = current_setting(...)::uuid)` para impedir gravar em outro tenant.

## 4. platform_admin / agência / contexto sem tenant

`platform_admin` não tem um tenant único (vê todas as contas) — a policy o trancaria. Opções:

- **(rec.) GUC de bypass:** o middleware, quando o Principal é platform_admin, seta `app.bypass_rls = 'on'`; a policy vira `using (app.bypass_rls OR <coluna> = current_setting('app.account_id')::uuid)`. Simples, auditável, sem role especial.
- **Alternativa:** role Postgres com `BYPASSRLS` para a conexão do admin — mais "Postgres-puro" mas exige troca de role por request (mais complexo no pool).

agency_owner operando uma conta-cliente: o `app.account_id` é setado para a **conta-alvo** que o middleware já resolve (X-Account-Id validado contra membership) — então a policy funciona naturalmente (ele "é" aquela conta na sessão).

## 5. Escopo de tabelas (account_id × tenant_id)

~70 colunas de escopo em 30 migrations, **colunas mistas**: tabelas antigas usam `tenant_id`, as novas (core/cardapio/etc.) usam `account_id`. A policy de cada tabela usa a coluna que ela tem. Plano: gerar o conjunto a partir do catálogo (`information_schema.columns` onde column_name in ('account_id','tenant_id')) e revisar a lista antes de aplicar — algumas tabelas são globais (ex.: `core.modules`, `core.permissions`) e **não** entram. A lista revisada vai no próprio doc antes da migration.

## 6. Migration (idempotente, sem goose Down)

Seguindo a regra do projeto (migrator roda o `.sql` inteiro, sem `Down`): SQL plano e idempotente — `enable`/`force` são idempotentes; `create policy` precisa de guard (`drop policy if exists ... ; create policy ...`) ou `do $$ ... if not exists ... $$`. Índice em `account_id`/`tenant_id` confirmado em todas (RLS adiciona um predicado por query — sem índice, degrada). 

## 7. Teste de regressão (a prova)

Um teste de integração (no `migration_integration_test.go` ou novo) que:
1. Cria 2 contas + linhas em cada numa tabela tenant-scoped.
2. Seta `app.account_id` = conta A, roda `select * from <tabela>` **sem** filtro de app → só linhas de A.
3. Confirma que mesmo um repo SEM `resolveTenantScope` não vê B.
4. platform_admin (bypass GUC) vê as duas.

Sem esse teste verde, RLS não entra.

## 8. Rollout faseado, performance, rollback

- **Faseado:** habilitar primeiro num módulo isolado (ex.: `queue.feedback` ou `cardapio`) + validar no browser, depois expandir. Não ligar nas 30 tabelas de uma vez.
- **Performance:** RLS = +1 predicado por query; com índice em `account_id`/`tenant_id` (já existem na maioria) o custo é baixo. Medir num endpoint de lista grande.
- **Rollback:** `alter table ... disable row level security` (instantâneo, reversível). A camada (A) de conexão fica — é inerte sem as policies.

## 9. Decisões em aberto (preciso do seu ok)

1. **Camada de conexão: (A) conn-por-request** [recomendado] ou (B) tx-por-request? (A) é o que eu seguiria.
2. **Bypass do platform_admin: GUC `app.bypass_rls`** [recomendado] ou role `BYPASSRLS`?
3. **Rollout:** começar por 1 módulo (qual? sugiro `cardapio`, é o que vamos vender) e expandir, ok?
4. Confirmar que dá pra investir o refactor da camada de dados (interface `Querier` nos repos) — é o maior custo.

Com esses 4 respondidos, implemento: migration faseada + camada de conexão + middleware + teste de regressão + build, e valido no browser.
