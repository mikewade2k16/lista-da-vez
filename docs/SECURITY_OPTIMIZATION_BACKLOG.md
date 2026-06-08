# Backlog — Segurança & Otimização Multi-Tenant

> Origem: auditoria 2026-06-07 ([ENGINEERING_PRINCIPLES.md §10](ENGINEERING_PRINCIPLES.md)). Objetivo: isolamento à prova de bypass + sistema rápido que pede só o necessário.
> Cada ticket é despachável de forma independente (engenheiro/programador). O **Mapa de paralelismo** no fim diz o que roda junto sem conflito de arquivo.

Severidade: 🔴 alta · 🟡 média · 🟢 baixa.

---

## Segurança

### SEC-1 🔴 RLS no Postgres (rede de segurança final)
- **Problema:** isolamento é 100% na aplicação (`resolveTenantScope`). Um handler novo que esqueça o scope **vaza dado cross-tenant**. Não há rede embaixo.
- **Solução:** habilitar Row-Level Security nas tabelas tenant-scoped: `ENABLE/FORCE ROW LEVEL SECURITY` + policy `USING (tenant_id = current_setting('app.account_id')::uuid)`. Middleware/transação seta `SET LOCAL app.account_id` a partir do `Principal.AccountID` em cada request.
- **Arquivos:** migration nova (todas as tabelas tenant-scoped de `queue.*`/`crm`/etc.), camada de conexão (`platform/database` — setter do GUC por tx), middleware no Chain.
- **Aceite:** com RLS ativo, uma query SEM o `SET app.account_id` correto retorna zero linhas de outro tenant, mesmo removendo o `resolveTenantScope` do service (teste de regressão).
- **Risco:** alto (toca toda tabela tenant-scoped + pool de conexão). Exige design dedicado. platform_admin precisa de bypass (`SET app.account_id` especial ou role `BYPASSRLS`).

### SEC-2 🟡 Erro uniforme 404 (não vazar existência)
- **Problema:** recurso de outro tenant pode retornar `403` (revela que existe) em vez de `404`. Enumeration.
- **Solução:** fora do escopo → `404 not_found`. Padronizar o mapeamento de erro nos services/handlers (`ErrForbidden` por escopo vira `404`; `ErrForbidden` por permissão RBAC continua `403`).
- **Arquivos:** mapeadores de erro de cada módulo (`*/http.go` `writeError`, `errors.go`). Espalhado.
- **Aceite:** GET de id de outro tenant → `404` idêntico ao de id inexistente.

### SEC-3 🟡 Rate limit por tenant (anti noisy-neighbor)
- **Problema:** rate limit é por `userID`. Um tenant com muitos users degrada vizinhos.
- **Solução:** adicionar quota por `account_id` além da de user.
- **Arquivos:** `httpapi/ratelimit.go` (+ resolver), `platform/app/app.go` (Chain).
- **Aceite:** estouro de quota de uma account não afeta requests de outra.

### SEC-4 🟡 Middleware de security headers
- **Problema:** sem `HSTS`, `X-Content-Type-Options`, `X-Frame-Options`, CSP no Chain.
- **Solução:** `httpapi.SecurityHeaders` middleware aplicado no Chain.
- **Arquivos:** `httpapi/security_headers.go` (novo), `platform/app/app.go` (Chain).
- **Aceite:** resposta traz os headers; CSP não quebra o front (testar em dev).

### SEC-5 🟢 Auditar tenant_id em cache keys e paths de upload
- **Problema:** cache key/arquivo sem `tenant_id` pode servir conteúdo cross-tenant.
- **Solução:** auditar `AccountModulesGuard` cache (já por accountID ✓), avatar/upload storage paths, qualquer cache novo. Garantir prefixo de tenant.
- **Arquivos:** `auth/*avatar*`, `platform/*storage*`, qualquer cache.
- **Aceite:** todo path/cache key inclui o id do tenant.

---

## Otimização

### OPT-1 🔴 Front não pede o que a role não vê
- **Problema:** bootstrap dispara `/v1/alerts`, `/v1/alerts/overview`, `/v1/consultants` para roles sem acesso → 403 + round-trip desperdiçado.
- **Solução:** helper `canViewAlerts`/`canViewConsultants` no front (espelha o back); gatear o fetch ANTES de disparar.
- **Arquivos:** `web/app/domain/utils/permissions.ts` (helpers), `web/app/pages/operacao/index.vue` (gate do `refreshOperationAlerts`), `web/app/utils/runtime-remote.ts` (gate de consultants).
- **Aceite:** logado como marketing, zero request a `/v1/alerts`/`/v1/consultants` no boot; sem 403 no console.

### OPT-2 🔴 N+1 em MeAccounts / MeContext
- **Problema:** `for account { ListEnabledModuleIDs(account.id) }` = 1 query por account.
- **Solução:** 1 query agregada `WHERE account_id = ANY($1)` → map em memória.
- **Arquivos:** `core/service.go` (MeAccounts/MeContext), `core/store_postgres.go` (novo `ListEnabledModuleIDsForAccounts`).
- **Aceite:** `/v2/me/accounts` faz nº de queries constante (não cresce com nº de accounts).

### OPT-3 🟡 Compressão gzip
- **Problema:** respostas JSON trafegam cruas.
- **Solução:** middleware gzip (negocia `Accept-Encoding`).
- **Arquivos:** `httpapi/compress.go` (novo), `platform/app/app.go` (Chain).
- **Aceite:** resposta de lista grande vem com `Content-Encoding: gzip`.

### OPT-4 🟡 Field selection / projeção lean
- **Problema:** listas devolvem o objeto inteiro quando a tela usa poucos campos.
- **Solução:** projeção lean por endpoint (modelo do `AccountSummary`) ou `?fields=`.
- **Arquivos:** services/repos de listagem (users, accounts, leads, operações). Espalhado.
- **Aceite:** payload da listagem só com os campos que a tabela usa.

### OPT-5 🟡 Paginação cursor nas listas que crescem
- **Problema:** offset (`page/perPage`) fica caro (OFFSET + COUNT) conforme cresce. `/manage/users` hoje busca todas as páginas (workaround).
- **Solução:** cursor (`?after=`) em users, leads, operações; UI de paginação real.
- **Arquivos:** handlers/repos das listas + composables/telas do front.
- **Aceite:** listar página N não fica mais lento conforme o total cresce.

### OPT-6 🟢 Lazy-load de detalhe
- **Problema:** detalhe (memberships, stores) carregado junto da listagem.
- **Solução:** carregar só ao abrir o modal/linha.
- **Arquivos:** composables/modais de detalhe (accounts, users).
- **Aceite:** listagem não dispara fetch de detalhe; detalhe carrega no clique.

---

## Mapa de paralelismo (o que roda junto SEM conflito de arquivo)

| Track | Tickets | Arquivos | Conflita com |
|---|---|---|---|
| **A — Chain/infra (back)** | SEC-3, SEC-4, OPT-3 | `httpapi/*` + `app.go` (slice de middlewares) | entre si (mesmo `app.go`) → **1 agente, sequencial** |
| **B — Core/identidade** | OPT-2 | `core/service.go`, `core/store_postgres.go` | nenhum |
| **C — Front** | OPT-1, OPT-6 | `permissions.ts`, `operacao/index.vue`, `runtime-remote.ts`, modais | nenhum (back) |
| **D — DB/RLS** | SEC-1 | migration + `platform/database` + 1 middleware | toca `app.go` (coordenar com Track A) |
| **E — Cross-cutting (cuidado)** | SEC-2, OPT-4, OPT-5 | `*/http.go`, `errors.go`, repos de vários módulos | entre si e com tudo → **fazer por módulo, um de cada vez** |

**Sugestão de 1ª onda paralela (zero conflito):** Track B (OPT-2) + Track C (OPT-1/OPT-6) + Track A começando por SEC-4+OPT-3. Track D (RLS) precisa de design antes — abrir sozinho. Track E por último (coordenado por módulo).

---

## Status
Backlog criado em 2026-06-07.

**Concluídos:**
- SEC-4 ✅ security headers (P1·10)
- OPT-1 ✅ front não pede o que a role não vê (P1·15)
- OPT-3 ✅ gzip (P1·16)
- SEC-2 parcial ✅ — testes IDOR cross-tenant em `alerts` e `settings` (P1·8, 2026-06-07)
- SEC-3 parcial ✅ — rate limit por tenant/account via `X-Account-Id` em memória (P1·11, 2026-06-07)
- SEC-5 parcial ✅ — token WS saiu da query string; ticket efêmero TTL 30s single-use (P1·12, Codex, 2026-06-07)

O panorama consolidado e atualizado vive em [ARQUITETURA_PANORAMA_2026-06-07.html](ARQUITETURA_PANORAMA_2026-06-07.html) (aba Plano) — manter ESSE como fonte visual de status; espelhar aqui e em `roadmap-data.ts` quando virar fase.

> Nota de processo: o panorama HTML é a fonte visual de status — atualizar a cada item concluído (não deixar acumular).
