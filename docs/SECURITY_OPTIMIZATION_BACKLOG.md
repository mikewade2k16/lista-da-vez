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

## Revisão 2026-06-29 (revisão tripla adversarial)

> 9 achados de segurança da revisão tripla de 2026-06-29 (todos verificados adversarialmente). A maioria nasceu no módulo `cardapio` (gating fino decorativo) e foi tratada na **wave 1** (correção imediata). Numeração continua a sequência SEC-*. Cada ticket traz `arquivo:linha` e o conserto.

### SEC-6 🔴 [P1] Cardápio sem permissão fina (RequireAuth-only)
- **Problema:** as rotas `/v1/cardapio/*` do painel usam só `RequireAuth` — qualquer usuário autenticado de qualquer papel chega no handler. As permissões `cardapio.view`/`cardapio.manage` existiam no registry mas eram **decorativas** (nada as exigia).
- **Arquivo:** `back/internal/modules/cardapio/http.go:17-39` (`RegisterRoutes` envolve tudo em `middleware.RequireAuth`).
- **Conserto (wave 1):** exigir `cardapio.view` no GET e `cardapio.manage` no POST/PATCH/DELETE/duplicate/media (platform_admin e agency_owner em curto-circuito). O gating por módulo (`account_modules`) já vem do `RequireModuleByPath` no Chain; a permissão fina passa a ser checada no handler/wrap.
- **Aceite:** usuário sem `cardapio.view` → 404 no GET; sem `cardapio.manage` → 403 na escrita; escopo de outra account → 404.
- **Status:** em correção wave 1.

### SEC-7 🔴 [P1] assignRole sem CheckMembership (concessão de papel cross-tenant)
- **Problema:** `AssignRoleToUser` atribuía um role na account sem validar que o usuário-alvo é **membro** daquela account — permitia conceder papel a usuário de outro tenant (escalonamento cross-tenant), divergindo do `SetUserRoles` que já validava.
- **Arquivo:** `back/internal/modules/core/rbac_service.go:179-193` (`AssignRoleToUser`).
- **Conserto (wave 1):** chamar `s.rbac.CheckMembership(ctx, accountID, userID)` antes do `FindRole`/assign; `ErrAccountNotMember` → `ErrNotMember` → 404 (escopo uniforme, igual ao `SetUserRoles`).
- **Aceite:** atribuir papel a usuário não-membro da account → 404; membro → idempotente.
- **Status:** em correção wave 1.

### SEC-8 🔴 [P1] Front cardapio_web sem viewPermission/editPermission
- **Problema:** o workspace `cardapio_web` não declara `viewPermission`/`editPermission` — o menu/tela do cardápio aparece sem espelhar o gating fino do back (SEC-6), inconsistente com os demais workspaces.
- **Arquivo:** `web/app/utils/workspaces.ts:34-39` (entry `cardapio_web`, sem chaves de permissão).
- **Conserto (wave 1):** declarar `viewPermission: 'cardapio.view'` e `editPermission: 'cardapio.manage'` (espelhando o back), mantendo platform_admin/agency_owner com bypass via `isPlatformAdmin || has(...)`.
- **Aceite:** papel sem `cardapio.view` não vê o item no menu nem acessa por URL; admin/agência continua vendo.
- **Status:** em correção wave 1.

### SEC-9 🟡 [P2] Feedback retorna 403 em vez de 404 (SEC-2)
- **Problema:** acesso a feedback fora do escopo do tenant devolve `403 forbidden` (revela existência) em vez de `404` — enumeration, mesma classe do SEC-2.
- **Arquivo:** `back/internal/modules/queue/feedback/http.go:355-356` (`writeServiceError` mapeia `ErrForbidden` → `http.StatusForbidden`); origem em `back/internal/modules/queue/feedback/service.go:65,109,127,154,199,208,211`.
- **Conserto (wave 1):** o `ErrForbidden` **por escopo de tenant** vira 404 (`feedback_not_found`); manter 403 só para negação por permissão RBAC genuína (distinguir os dois caminhos no service).
- **Aceite:** GET/POST em feedback de outro tenant → 404 idêntico ao de id inexistente.
- **Status:** wave 1.

### SEC-10 🟡 [P2] Queries de feedback sem filtro tenant_id (defesa em profundidade)
- **Problema:** parte das queries de feedback resolve o escopo só na aplicação (service), sem `where tenant_id = $` na própria query — um handler novo que esqueça o scope vaza cross-tenant (sem rede embaixo, mesma raiz do SEC-1).
- **Arquivo:** `back/internal/modules/queue/feedback/store_postgres.go:138,157,217` (selects de feedback/mensagens sem cláusula `tenant_id` consistente; o filtro só aparece no list em :237).
- **Conserto (wave 1):** adicionar `and tenant_id = $::uuid` (defesa em profundidade) nos selects de feedback e mensagens, alimentado pelo `Principal.TenantID`/`AccountID`.
- **Aceite:** query de feedback sempre carrega o predicado de tenant; teste de regressão IDOR cross-tenant verde.
- **Status:** wave 1.

### SEC-11 🟡 [P2] Rate-limit público do cardápio é global por IP (SEC-3)
- **Problema:** o limitador público do cardápio é por `(scope, IP)` em memória, **global** — não tem dimensão de tenant/restaurante, então um IP barulhento afeta todos os restaurantes (noisy-neighbor, mesma classe do SEC-3).
- **Arquivo:** `back/internal/modules/cardapio/rate_limit.go:36-40` (chave = `scope + "|" + ip`, sem restaurante/account).
- **Conserto (wave 1):** incluir o slug/restaurantID na chave do bucket (`scope|restaurant|ip`) para isolar quota por restaurante; manter o teto por IP.
- **Aceite:** estouro de quota num restaurante não derruba a ingestão pública de outro.
- **Status:** wave 1.

### SEC-12 🟡 [P2] clientIP confia no primeiro X-Forwarded-For
- **Problema:** `clientIP` usa o **primeiro** host do `X-Forwarded-For`, que é forjável pelo cliente (spoof do IP → driblar o rate-limit e poluir `ip_hash` da telemetria).
- **Arquivo:** `back/internal/modules/cardapio/rate_limit.go:65-77` (`clientIP` pega o 1º host do XFF antes do `RemoteAddr`).
- **Conserto (wave 1):** confiar só no proxy conhecido — pegar o IP **mais à direita** do XFF (o que o proxy de borda anexou) ou cair direto no `RemoteAddr`; documentar a premissa de proxy reverso confiável.
- **Aceite:** XFF arbitrário do cliente não muda o IP efetivo do rate-limit/telemetria.
- **Status:** wave 1.

### SEC-13 🟡 [P2] CARDAPIO_TELEMETRY_SALT "obrigatório" não é enforçado
- **Problema:** o `AGENT.md` chama `CARDAPIO_TELEMETRY_SALT` de obrigatório em produção, mas o boot só loga um `Warn` e segue com `ip_hash` vazio — divergência doc×código (telemetria sem salt = `ip_hash` previsível/vazio).
- **Arquivo:** `back/internal/modules/cardapio/module.go:72-83` (só `Logger.Warn` quando vazio); doc em `back/internal/modules/cardapio/AGENT.md:367,399`.
- **Conserto (wave 1):** alinhar a doc ao comportamento real (warn + `ip_hash` vazio, não fatal) **e** registrar como item de deploy: provisionar o salt no `.env.production` da VPS (ver roadmap fase de hardening). Endurecer para fatal-em-prod fica como follow-up.
- **Aceite:** doc descreve o comportamento real; salt provisionado em prod antes de vender acesso.
- **Status:** doc alinhada wave 1 (enforcement fatal = follow-up).

### SEC-14 🟢 [P2] AGENT.md do cardápio afirmava RBAC que não existia
- **Problema:** o `AGENT.md` do módulo `cardapio` descrevia gating por `cardapio.view`/`cardapio.manage` que de fato **não era aplicado** (só `RequireAuth`, ver SEC-6) — documentação enganosa que mascarava o buraco de permissão.
- **Arquivo:** `back/internal/modules/cardapio/AGENT.md` (seção de rotas do painel) + `back/internal/modules/cardapio/http.go:14-16` (comentário dizia que o gating vinha do Chain, sem permissão fina).
- **Conserto (wave 1):** corrigir o AGENT.md para refletir o gating fino real introduzido no SEC-6 (`cardapio.view`/`cardapio.manage` no handler) — doc volta a casar com o código.
- **Aceite:** AGENT.md do cardápio descreve exatamente o que o código exige; sem afirmação de RBAC fantasma.
- **Status:** corrigido wave 1.

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
Backlog criado em 2026-06-07. Ampliado em 2026-06-29 com a seção "Revisão 2026-06-29" (SEC-6 a SEC-14, 9 achados; foco em `cardapio` + `assignRole`, maioria em correção/feito na wave 1).

**Concluídos:**
- SEC-4 ✅ security headers (P1·10)
- OPT-1 ✅ front não pede o que a role não vê (P1·15)
- OPT-3 ✅ gzip (P1·16)
- SEC-2 parcial ✅ — testes IDOR cross-tenant em `alerts` e `settings` (P1·8, 2026-06-07)
- SEC-3 parcial ✅ — rate limit por tenant/account via `X-Account-Id` em memória (P1·11, 2026-06-07)
- SEC-5 parcial ✅ — token WS saiu da query string; ticket efêmero TTL 30s single-use (P1·12, Codex, 2026-06-07)

O panorama consolidado e atualizado vive em [ARQUITETURA_PANORAMA_2026-06-07.html](ARQUITETURA_PANORAMA_2026-06-07.html) (aba Plano) — manter ESSE como fonte visual de status; espelhar aqui e em `roadmap-data.ts` quando virar fase.

> Nota de processo: o panorama HTML é a fonte visual de status — atualizar a cada item concluído (não deixar acumular).
