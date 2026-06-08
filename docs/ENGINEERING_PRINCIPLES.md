# Princípios de Engenharia — Omni

> Documento vivo. Toda falha nova vai em [#Registro de falhas e aprendizados](#registro-de-falhas-e-aprendizados).
> Toda regra aqui se aplica a **qualquer** arquivo novo ou editado, em qualquer branch.

---

## 1. Código limpo e organizado

- **Máximo 450 linhas por arquivo.** Se ultrapassar, dividir por responsabilidade.
- Preferir **menos arquivos mais coesos** a muitos arquivos rasos. 20 arquivos × 400 linhas > 50 arquivos × 300 linhas — desde que cada arquivo tenha uma responsabilidade clara.
- Nomes descritivos. Código que explica a si mesmo, sem precisar de comentário.
- Sem repetição. Se algo aparece duas vezes, extrai função/composable/helper.
- Sem código morto. Remove o que não é usado; não comenta pra "deixar pra depois".

## 2. Padrões da linguagem e da comunidade

### Go (backend)
- `gofmt` sempre. Nenhum arquivo foge do formatter.
- Erros tratados no ponto onde ocorrem; nunca ignorados com `_` (exceto onde é explicitamente justificado).
- Interfaces pequenas e focadas. Uma interface de 10 métodos é um sinal de problema.
- Injeção de dependência via construtor — sem globais, sem singletons implícitos.
- IDs sempre `uuid` como `string` internamente; cast explícito na borda (scan nullable com `*string`).
- Permissões declaradas no banco (`core.permissions`), nunca hardcoded em `if`.

### TypeScript / Vue (frontend)
- `<script setup lang="ts">` em todo componente Vue. Sem Options API.
- Props tipadas com `defineProps<{}>()`. Sem `any`.
- Composables (`use*`) para lógica reutilizável; componentes só orquestram UI.
- Sem `v-html` exceto conteúdo sanitizado. Nunca interpolar input do usuário direto no DOM.
- Imports com `~/` (alias raiz), nunca `../../../`.

## 3. Separação de responsabilidades

- **Cada função sabe só o que precisa saber.** Handler HTTP não acessa banco diretamente; repository não conhece HTTP.
- Camadas: `handler → service → repository`. Sem pular.
- No front: `page → composable → store/fetch`. Pages são orquestradores, não lógica.
- Componentes atômicos: um componente faz uma coisa. Componente de lista não faz fetch de detalhe.
- Mesmo dentro de um arquivo com 400 linhas: funções separadas com nome claro, não bloco de 200 linhas inline.

## 4. Componentização

- Frontend: qualquer UI que aparece em 2+ lugares vira componente.
- Backend: qualquer query ou lógica usada em 2+ handlers vira método no repository/service.
- **Modal e board card são sempre espelhados** — mudança em um aplica no outro (regra de UX do produto).
- Layers Nuxt (`layers/queue`, `layers/tasks`, etc.) isolam domínios. Componente de queue não importa de tasks e vice-versa.

## 5. Segurança (pilar 1)

- `account_id` **nunca** vem do body/query do request. Sempre do `Principal.AccountID` resolvido pelo middleware.
- Validação de input na borda do sistema (handlers), nunca assumir que dados internos são limpos.
- Sem SQL dinâmico concatenado. Sempre parâmetros posicionais (`$1`, `$2`, ...).
- Tokens e chaves sensíveis nunca em logs. `slog` com campos explícitos, não `fmt.Sprintf` com structs inteiras.
- Sessões têm `revoked_at`; middleware checa validade no mesmo lookup do Principal (sem query extra quando cache hit).
- Webhook keys rotacionadas via endpoint, nunca expostas em listagem.
- CSP, CORS configurados. Sem `Access-Control-Allow-Origin: *` em produção.

## 6. Performance (pilar 2)

- **Usuário clicou → sistema responde imediatamente.** Se vai demorar, skeleton/loading aparece em < 100ms. Sem tela branca, sem delay perceptível.
- Requests da UI são cancelados se o usuário navegar antes de completar (AbortController).
- Paginação server-side em qualquer lista com > 50 itens potenciais.
- Índices no banco para qualquer coluna usada em `WHERE`, `JOIN` ou `ORDER BY` em hot path.
- `PrincipalCache` com TTL 2-5 min + invalidação por evento (não polling).
- WebSocket para atualizações em tempo real — não polling de 5 em 5 segundos.
- Bundle Nuxt: imports dinâmicos para módulos grandes. Nenhuma página carrega código que não usa.
- Imagens com lazy loading; avatares com dimensão máxima definida.

## 7. UX — Experiência é determinante (pilar 3)

- **Mínimo de cliques.** Ação que pode ser feita em 1 clique não deve exigir 2.
- Feedback visual imediato: botão desabilita + spinner no clique, não espera o servidor responder.
- Erros devem dizer o que fazer, não só o que deu errado. "E-mail inválido" > "Erro 422".
- Layout consistente em todo o painel. Mesma paleta, mesmos espaçamentos, mesmos componentes base (UnaUI / Nuxt UI).
- Mobile-first: painel deve ser usável em tela pequena (gestores acessam pelo celular).
- Estados vazios com mensagem orientativa — nunca tabela em branco sem contexto.
- Toasts para feedback de ação (sucesso/erro), não alert() do browser.

## 8. Observabilidade

- Logs estruturados com `slog` no backend. Campos: `op` (operação), `account_id`, `user_id`, `error`.
- Nível `INFO` para fluxos normais; `WARN` para degradações recuperáveis; `ERROR` para falhas que precisam de atenção.
- Nunca logar senha, token, `password_hash` ou PII além do necessário.
- Métricas de hot path via `perf_hotpath_stats` (schema `queue.*`) já existentes — expandir para `core.*` quando relevante.

## 9. Regras para agentes Claude nessa codebase

- Máximo 1 tarefa `in_progress` por vez no TodoWrite.
- Ao finalizar qualquer entrega: atualizar **3 lugares** — doc canônico + AGENT.md do módulo + `roadmap-data.ts`. Ver [[three-docs-sync]].
- Anotar mudanças de deploy em "Notas de Deploy" do doc canônico.
- Sempre verificar porta do banco antes de rodar migrate: `localhost:5433` = omni (certo); `localhost:5432` = postgres nativo Windows (errado).
- Nunca commitar/push/deploy — devolver comandos para o usuário rodar.
- Nunca Co-Authored-By Claude em commits.
- Documentar (plano + roadmap-data.ts) antes de codar.

---

## 10. Auditoria de Segurança e Otimização Multi-Tenant (2026-06-07)

> Auditoria do módulo multi-tenant ao fechar a fase. Objetivo do produto: **um usuário NUNCA vê dado de outra account/loja**, e nenhum payload/bypass/parâmetro forjado contorna isso. Fundamentado em [OWASP Multi-Tenant Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Multi_Tenant_Security_Cheat_Sheet.html) e práticas de SaaS em escala.

### 10.1 Medidas de segurança ATIVAS (inventário — defesa em profundidade)

Camadas que hoje protegem o isolamento (cada uma é uma rede; nenhuma é única):

1. **Auth por JWT assinado (HMAC)** — `auth/tokens.go`. Papel, tenant e claims vêm do token assinado; cliente não forja `role`/`tenantId`. Bearer obrigatório.
2. **Revogação de sessão** — claim `sid` + `core.user_sessions.revoked_at` checado no mesmo lookup do Principal. Logout/kill invalida na hora.
3. **Membership obrigatória por account** — `RequireAuthWithAccount` valida `X-Account-Id` contra `core.account_users` (`IsMember`) ANTES de injetar `AccountID` no Principal. Header forjado → `403 account_not_member`.
4. **Gating de módulo por rota** — `RequireModuleByPath` (back) + `module-enabled.global.ts` (front): rota de um módulo só abre se a account contratou (`core.account_modules`). platform_admin isento (gerencia todas).
5. **Escopo de tenant validado no service** — `resolveTenantScope(principal, requestedTenantID)`: não-admin pedindo tenant ≠ do seu → `ErrForbidden`. O `tenantId` do query é **filtro dentro do permitido**, nunca bypass. Padrão replicado em alerts/erp/settings/analytics/stores (70 checagens em 29 arquivos).
6. **Escopo de loja validado** — `resolveStoreScope`: storeId fora de `principal.StoreIDs` → `ErrForbidden`.
7. **Defesa em profundidade na query** — mesmo se o escopo passasse, o repo filtra por `tenant_id = $1` sempre. Store de outro tenant → query retorna vazio (não vaza).
8. **RBAC por permissão** — `canViewAlerts`/`canManage*` checam `core.role_permissions` (ou papel coarse no fallback). 403 de role sem permissão é **comportamento correto**, não bug.
9. **SQL 100% parametrizado** — `$1,$2,...`, zero concatenação. Sem superfície de SQLi.
10. **Rate limit** — `httpapi.RateLimit` por `userID`.
11. **CORS allowlist** — `httpapi.CORS(cfg.CORSAllowedOrigins)`, sem `*` em prod.

### 10.2 Gaps de segurança (priorizados)

- **[Alto] Sem RLS no Postgres.** O isolamento é 100% na aplicação (service layer). A prática de escala recomenda **Row-Level Security** como rede final: se um handler novo esquecer o `resolveTenantScope`, hoje vaza. RLS com `USING (tenant_id = current_setting('app.account_id'))` fecharia por baixo. → avaliar como C-segurança.
- **[Médio] Paridade de erro not-found × forbidden.** Retornar `404` vs `403` diferentes vaza a existência de um recurso de outro tenant ("enumeration"). Padronizar: recurso fora do escopo → `404 not_found` (não `403`).
- **[Médio] Rate limit por usuário, não por tenant.** Um tenant com muitos usuários pode degradar vizinhos ("noisy neighbor"). Adicionar quota por `account_id`.
- **[Médio] Sem middleware de security headers.** Falta `HSTS`, `X-Content-Type-Options`, `X-Frame-Options`, CSP no Chain. Adicionar `httpapi.SecurityHeaders`.
- **[Baixo] Escopo em cache keys / storage.** Confirmar que toda cache key e todo path de upload inclui `tenant_id`/`account_id` (evita cross-tenant em cache/arquivos).

### 10.3 Otimização — "pedir só o necessário" (UX de resposta imediata)

Princípio: **carregar primeiro o essencial da tela, lazy-load do resto; payload mínimo; nada de buscar o que não se usa.**

- **[Alto] Não pedir o que a role não vê.** Front dispara `/v1/alerts`, `/v1/consultants` no bootstrap mesmo para roles sem acesso → 403 + request desperdiçado. Gatear o fetch por permissão ANTES de disparar (`canViewAlerts` no front, espelhando o back). Remove ruído e economiza round-trip.
- **[Alto] N+1 em `MeAccounts`.** `for account { ListEnabledModuleIDs(account.id) }` faz 1 query por account. Trocar por 1 query agregada (`WHERE account_id = ANY($1)`). Mesmo no `MeContext`.
- **[Médio] Sem compressão.** Nenhum middleware gzip/brotli. Respostas JSON grandes (listas) trafegam cru. Adicionar `Content-Encoding: gzip`.
- **[Médio] Field selection / sparse fieldset.** Listas retornam o objeto inteiro quando a tabela usa poucos campos. Suportar `?fields=` ou projeções lean por endpoint (ex.: `AccountSummary` já é lean — replicar). Reduz payload e serialização.
- **[Médio] Paginação cursor em listas grandes.** Hoje offset (`page/perPage`). Para listas que crescem (users, leads, operações), cursor (`?after=`) evita `OFFSET` caro e `COUNT` em cada página.
- **[Baixo] Prefetch só do above-the-fold; resto sob interação.** Detalhes de linha (memberships, stores) só ao abrir o modal, não na listagem.
- **UI:** skeleton < 100ms no clique, `AbortController` ao trocar de rota, otimismo no toggle (já praticado nos switches).

### 10.4 Como big techs fazem (referência)

- **Isolamento:** tenant_id em TODA query + RLS no banco como rede final; nunca confiar só na aplicação ([Security Boulevard, 2025](https://securityboulevard.com/2025/12/tenant-isolation-in-multi-tenant-systems-architecture-identity-and-security/), [Redis](https://redis.io/blog/data-isolation-multi-tenant-saas/)). Erros uniformes (não vazar existência).
- **Performance:** paginação cursor > offset, field selection (sparse fieldset/GraphQL), compressão gzip/brotli, evitar N+1 com eager-load/batch, cache (Redis/CDN) com chave por tenant, HTTP/2 keep-alive ([Speakeasy pagination](https://www.speakeasy.com/api-design/pagination), [10x API performance](https://medium.com/@crok07.benahmed/top-7-ways-to-10x-your-api-performance-caching-connection-pooling-avoiding-n-1-problem-33516b657af2)).

---

## Registro de falhas e aprendizados

> Toda entrada aqui evita que o mesmo erro aconteça de novo.

### [2026-05-28] Migrate up no banco errado (postgres host 5432 vs container 5433)

**O que aconteceu:** Rodei `go run ./cmd/migrate up` sem verificar que `back/.env` apontava para `localhost:5432` (postgres nativo Windows) em vez de `localhost:5433` (omni-postgres-1 Docker). As migrations 0118-0125 foram aplicadas no banco errado. As C1 (0123/0124/0125) foram revertidas manualmente; 0118-0122 ficaram no banco errado (risco baixo, esse banco não é usado em produção).

**Causa raiz:** Dois servidores PostgreSQL na mesma máquina em portas diferentes (`5432` nativo, `5433` container). O `.env` apontava para a porta errada.

**Correção:** `.env` atualizado para `localhost:5433`. Migrations C1 re-aplicadas no banco certo via `docker exec psql` + `migrate up` apontando pra 5433.

**Regra criada:** Antes de rodar qualquer `migrate up` local, verificar `echo $DATABASE_URL` — deve conter `:5433`. Ver aviso no `back/internal/platform/database/AGENT.md`.

### [2026-05-29] Panic no boot por conflito de rota `/v1/me/context` (módulo core × legado)

**O que aconteceu:** Após rebuild do container api (C9), o app crashou com `panic: pattern "GET /v1/me/context" ... conflicts with pattern "GET /v1/me/context"`. O módulo core (`modules/core/http.go`) registrava `/v1/me/context` como alias v1, e `platform/app/context_http.go` (legado) também registrava o mesmo endpoint. Conflito só apareceu agora porque o container anterior rodava com binário antigo, sem o módulo core compilado.

**Causa raiz:** Duas implementações concorrentes da mesma rota com shapes incompatíveis: legacy retorna `{user, principal, context: {tenants, stores}}`; módulo core exige `accountId` na query string e retorna `{user, account, roles, permissions}`. O frontend usa o shape legacy. Mesmo se o panic fosse evitado, o frontend quebraria com `400 missing_account_id`.

**Correção:** Aliases v1 removidos do módulo core (`/v1/me/accounts` e `/v1/me/context`). Módulo core fica só com `/v2/me/*`. Legacy continua servindo `/v1/me/context` enquanto o frontend não migra para o shape v2.

**Regra criada:** Antes de adicionar alias v1 de uma rota nova no módulo (core ou satélites), grep `mux.Handle("GET /v1/<path>"` no projeto inteiro. Conflito de rota é detectado só no boot — não no `go build`. Documentar shape esperado pelo frontend antes de "modernizar" rota legada.

### [2026-05-29] Porta do api perdida ao recriar container

**O que aconteceu:** Após `docker compose up -d --build api`, o frontend deu "Failed to fetch" porque o api subiu na porta 8080 (default do compose) em vez de 8883 (esperado pelo `web/.env`). O `.env` da raiz não existia, então `${API_PORT:-8080}` caiu no default. A porta 8883 vinha de algum shell export anterior que se perdeu no rebuild.

**Causa raiz:** Config implícita via variável de ambiente exportada em shell, sem persistência em arquivo. Qualquer rebuild que crie um shell novo (CI, container, novo terminal) perde a config.

**Correção:** Criado `.env` na raiz com `API_PORT=8883` + `NUXT_PUBLIC_API_BASE=http://localhost:8883` espelhando o `web/.env`. Container recriado com `docker compose up -d api` (sem `--build`).

**Regra criada:** Toda variável de ambiente que afeta network (`*_PORT`, `*_URL`, `*_BASE`) deve viver em `.env` versionado por `.env.example`, nunca em shell export. Em PR que adiciona variável nova, marcar em "Notas de Deploy" do doc canônico que o `.env` precisa ser atualizado.

### [2026-05-29] WorkspaceId novo não aparece no menu por falta de wire em 3 lugares

**O que aconteceu:** Criei page `/manage/users.vue` + entry no `nav.config.ts` com `workspaceId: 'usuarios_admin'`. Página existia, build OK, mas o item NÃO aparecia no menu lateral. Usuário (platform_admin) testou e reportou "página não existe ou não está no menu".

**Causa raiz:** `useDashboardNav` filtra cada item via `allowedWorkspaceSet.has(workspaceId)`. O set vem de `getAllowedWorkspaces(role, ...)` em `permissions.ts`, que para `platform_admin` retorna a lista de `ROLE_WORKSPACES.platform_admin`. Sem o ID lá, o item é silenciosamente filtrado — nenhum log, nenhum erro.

**Correção:** Ao adicionar um `workspaceId` novo, **atualizar 3 arquivos**:
1. `web/app/utils/workspaces.ts` → entry no `WORKSPACES` array (id, label, icon, path).
2. `web/app/domain/utils/permissions.ts` → entry em `WORKSPACE_ACCESS_DEFINITIONS` (id, label, viewPermission, editPermission) + adicionar `id` em cada `ROLE_WORKSPACES[role]` que deve ter acesso.
3. `web/layers/queue/nav.config.ts` (ou `web/app/utils/sidebar-nav.ts` se for sidebar legada) → item de menu com `workspaceId` igual.

**Regra criada:** PR que adiciona página nova com `workspaceId` deve ser auditado contra os 3 arquivos acima. Verificável: grep o ID em todos os 3. Falha silenciosa é a pior — type-checker não pega porque os IDs são string. Em PR review: confirmar item visível no menu pelo papel-alvo antes de aprovar.

---

## Referência cruzada

- Plano canônico da branch atual → [MULTITENANT_COMPLETION_PLAN.md](MULTITENANT_COMPLETION_PLAN.md)
- Roadmap geral → [ROADMAP.md](ROADMAP.md)
- Contratos invariantes → [CONTRACT_FREEZE.md](CONTRACT_FREEZE.md)
- Schema-alvo → [SCHEMA_TARGET.md](SCHEMA_TARGET.md)
