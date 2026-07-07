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

### [2026-06-08] Faixa de consultores sumiu para operador `consultant` (roster so vinha do endpoint de gestao)

**O que aconteceu:** Em producao, o usuario de loja que opera a fila (papel `consultant`, ex.: "Terminal Perola Riomar") via a coluna "Lista da vez" interativa, mas a **faixa de consultores** do rodape (entrar na fila, pausar, retomar, tirar da fila) vinha **vazia** — sem ela, nao da pra girar a operacao. O `platform_admin` via a faixa normal, entao parecia permissao de mutacao. O Codex tentou e nao resolveu (mexeu em `v-if`/permissoes de mutacao).

**Causa raiz:** A faixa renderiza `state.roster`. O `roster` (consultores disponiveis) so era populado por `GET /v1/consultants` (endpoint de GESTAO), gateado por `canViewConsultants`, que NAO inclui `consultant` em nenhum dos dois mundos: (a) modo legado `permissionsResolved=false` -> fallback so `owner/platform_admin/store_terminal`; (b) modo core -> `consultant` so tem `workspace.operacao.view/edit`, nao `consultor.view`/`settings.view`. A coluna da fila aparecia porque o **snapshot** (`/v1/operations/snapshot`, que o consultor PODE ler) ja embute nome/iniciais de quem esta em fila/atendimento — mas o snapshot NAO trazia os consultores **disponiveis**. Logo: fila com gente, faixa vazia.

**Correcao (dois mundos, sem vazar dado):** o snapshot da operacao passou a expor um `roster` ENXUTO (`id/storeId/name/role/initials/color` — sem meta/comissao/e-mail) em `buildSnapshotView` (back) e o front (`runtime-remote.ts`) usa esse roster como fallback quando `/v1/consultants` vem vazio. Assim qualquer papel que pode OPERAR ganha a faixa, sem abrir o endpoint de gestao. De quebra, revertida uma regressao de seguranca introduzida na tentativa anterior (mutacao liberada com permissao so de `view`, e `director`/`marketing` ganhando comando) — mutacao volta a exigir `workspace.operacao.edit`.

**Regra criada:** dado que a faixa precisa do roster, todo papel operador (`consultant`, `store_terminal`, `manager`...) precisa do roster **pela via que ele ja pode ler (o snapshot da operacao)**, nunca dependendo do endpoint de gestao de consultores. Ao diagnosticar "componente sumiu", checar PRIMEIRO se a FONTE DE DADOS dele chega aquele papel (gate de fetch), nao so o `v-if` de render. E nunca colapsar `view==edit` para resolver UI — view nao muta.

### [2026-06-13] USelect (Reka UI) explode com SelectItem de value vazio (500 na tela)

**O que aconteceu:** Ao abrir `/site/bio` como admin, a tela dava `500: A <SelectItem /> must have a value prop that is not an empty string`. O filtro de status e o de cliente (`BioListWorkspace.vue`) tinham um item placeholder `{ label: 'Todos os status', value: '' }` / `{ label: 'Todos os clientes', value: '' }`.

**Causa raiz:** O `USelect` do Nuxt UI (Reka UI por baixo) reserva o value vazio (`''`) para o estado "sem selecao" (mostra o placeholder) — entao **proibe** um `SelectItem` com `value=""`. `<select>`/`<option>` HTML nativo aceita `value=""` numa boa; por isso os componentes do cardapio (que usam select nativo) nao quebraram, so o bio (que usa `USelect`).

**Correcao:** Item "Todos/Selecione" usa um sentinela nao-vazio (`value: 'all'`); o estado do filtro inicia em `'all'`; ao montar a query pro backend converte `'all'` -> `''` (sem filtro). `model-value=""` continua valido (e o jeito de limpar e mostrar placeholder) — o proibido e o **item** ter value vazio.

**Regra criada:** Em qualquer `USelect`/`USelectMenu`, NUNCA usar `value: ''` num item (placeholder "Todos"/"Selecione"). Usar sentinela (`'all'`, `'none'`) e converter na borda, OU usar a prop `placeholder` com `model-value` vazio e sem item-placeholder. Vale para todo componente do painel.

### [2026-06-13] Sync de API externa do cliente falhava com 406 (WAF bloqueia User-Agent do Go)

**O que aconteceu:** O sync de produtos da Pérola (`POST /v1/admin/products/sync` → cliente Go lê `perolajoias.com/api/products`) retornava `406 Not Acceptable`. A API da Pérola responde 200 a `curl`, mas o WAF/host do cliente bloqueia o User-Agent padrão do Go (`Go-http-client/1.1`) — e até `Mozilla/5.0` genérico — com 406.

**Causa raiz:** `http.Client` do Go não seta User-Agent customizado; o default `Go-http-client/...` cai numa regra de WAF (ModSecurity-like) do servidor do cliente.

**Correção:** `req.Header.Set("User-Agent", "OmniSync/1.0 (+https://omni)")` (descritivo, honesto) no `perola_client.go`. UA próprio passou (200), assim como `curl/x`; só `Go-http-client` e `Mozilla/5.0` genérico davam 406.

**Regra criada:** Todo cliente HTTP Go que consome API de terceiro (integração de cliente) deve setar um `User-Agent` descritivo próprio (`OmniSync/1.0 (+url)`) + `Accept`. Não confiar no UA default do Go — WAFs o bloqueiam. Ao integrar um cliente novo, testar o endpoint com `curl -A` antes (UA é a causa nº1 de 403/406 em sync externo).

### [2026-06-15] Visibilidade de account em DOIS caminhos que podem divergir (org-aware)

**O que aconteceu:** Ao tornar o acesso da agência org-aware (AGENCY_TENANT plan, Etapa 3), o spec
cobriu só a LEITURA (`core.ListAccountsForUser`/`FindAccountIfMember`, que alimenta `/v2/me/accounts`
= o que o switcher lista). Faltou o ENFORCEMENT: `auth.PostgresAccountMemberChecker.IsMember`, o
portão de `RequireAuthWithAccount` que valida `X-Account-Id` em TODA rota de módulo (queue/crm/tasks),
ainda consultava só `core.account_users`. Resultado seria: o switcher LISTA a conta-agência Crow, mas
ao usá-la (carregar o board Tasks com `X-Account-Id=crow`) o login-agência levaria `403
account_not_member` — o board "some" ao trocar de conta, repetindo o incidente de 2026-06-10 por outro
motivo. Pego no Gate 1 (antes de mover dado), não em produção.

**Causa raiz:** duas fontes de verdade para "quais accounts o user acessa" — uma de leitura (módulo
`core`) e uma de enforcement (`auth` middleware) — em pacotes diferentes (`auth` não importa `core`,
para evitar ciclo). Mudar uma sem a outra cria drift silencioso (compila, passa nos testes do pacote).

**Correção:** `IsMember` recebeu a MESMA regra org-aware (platform_admin / agency_owner / membership),
replicando a cláusula SQL em `accountAccessibleQuery` (const, com teste de contrato). Provado no banco:
`platform_admin→qualquer conta=true`; `cliente→conta de outra org=false`.

**Regra criada:** ao mudar a regra de visibilidade/escopo de account, alterar OS DOIS caminhos juntos —
`core.ListAccountsForUser`/`FindAccountIfMember` (leitura) E `auth.PostgresAccountMemberChecker.IsMember`
(enforcement do middleware). Um teste de contrato em cada lado documenta que a regra existe. Antes de
mover dado de tenant entre accounts, validar no Gate que o portão de membership aceita o novo dono.

### [2026-06-17] store_type do multiloja "revertia" no reload (front nao re-hidratava do banco)

**O que aconteceu:** Em Configuracoes > Multi-loja, trocar o "Tipo de loja" de Bairro para Shopping
parecia nao salvar: ao recarregar a pagina, o select voltava para Bairro. Suspeita inicial de bug de
persistencia (banco/back).

**Causa raiz:** o dado JA persistia certo — `queue.stores.store_type` tinha `shopping` para as lojas
editadas e a API `/v1/stores` devolvia `storeType`. O bug era 100% de leitura no front: o
`MultiStoreLojasSection.vue` montava o "draft" de cada linha com `drafts[id] ?? createDraftFromStore(store)`.
No reload, `state.managedStores` chegava PRIMEIRO pelo fallback `auth.storeContext`/`runtimeState.stores`
(contexto do switcher, SEM `storeType`) — o draft era semeado com o default `'bairro'`. Quando o
`/v1/stores` chegava depois com `'shopping'`, o `??` preservava o draft velho e ignorava o valor real do
banco. Fonte parcial (contexto) vencia a fonte autoritativa (endpoint do recurso).

**Correcao:** o draft passa a ser SEMPRE re-hidratado a partir do servidor; so se preserva enquanto a
linha tem edicao pendente (`touched`) ou esta salvando (`rowBusy`). `touched[id]` e' marcado ao editar e
limpo no save bem-sucedido. Assim o valor do `/v1/stores` (banco) sempre vence o default/contexto.

**Regra criada:** ver AGENT_RULES "Nada hardcoded — toda informacao vem do banco". Front nunca renderiza
dado real a partir de fonte parcial/fallback; draft re-hidrata do back assim que ele responde. Ao depurar
"nao salvou/reverteu", checar PRIMEIRO se o dado esta no banco (`psql`) e se a API o devolve, antes de
mexer no back.

### [2026-07-03] Dev "lento normal do Vite" era o bind mount Windows→container (meses de misdiagnóstico)

**O que aconteceu:** o dev do painel foi ficando inutilizável — troca de página levava minutos e, no
fim, o `nuxt dev` nem terminava o boot (localhost:3003 recusava conexão). Durante semanas o sintoma foi
diagnosticado como "compile on-demand do Vite, normal em dev", e as tentativas de correção (polling do
watcher, warmup de todas as páginas) PIORARAM o problema.

**Causa raiz:** infraestrutura, não framework. O bind mount `./web:/app` montava pasta do WINDOWS num
container Linux via a ponte 9P/gRPC-FUSE do Docker Desktop/WSL2 — cada stat/read fica ~100x mais lento,
e o nuxt dev toca milhares de arquivos. Os "paliativos" multiplicaram o I/O exatamente na ponte lenta:
polling varria a árvore toda a cada 350ms e o warmup pré-compilava o grafo INTEIRO no boot, tudo
atravessando a ponte. O Vite era inocente: os mesmos comandos em FS nativo (VPS, container sem bind
mount) sempre foram rápidos.

**Correção:** bind mount removido; o código é copiado no build da imagem (target dev, overlayfs nativo)
e `docker compose watch` (develop.watch, action sync) sincroniza as edições host→container; inotify real
dentro do container, polling desligado. Fluxo: `npm run dev` (= `up --build --watch`) ou
`npm run dev:watch` (= scripts/dev/watch-web.ps1: `up -d --build web` + `watch --no-up web`). Medido:
boot 5+min-sem-terminar → ~60s; GET / em 0.04s; edição chega em ~2s. Detalhe completo em
[OPERATION_DOCKER_BUG_LOG.md](OPERATION_DOCKER_BUG_LOG.md) §7.

**Regra criada:** lentidão PATOLÓGICA em dev (minutos, não segundos) é infraestrutura até prova em
contrário — medir ONDE o tempo é gasto (FS? CPU? rede?) antes de aceitar "é normal do framework".
Nunca montar caminho Windows como bind mount de fonte de um dev server que varre muitos arquivos;
usar compose watch/sync (código em FS nativo do container). E nunca empilhar paliativos (polling,
warmup, cache) sem confirmar a causa raiz: paliativo em cima de diagnóstico errado multiplica o problema.
Armadilhas do fluxo novo: o watch NÃO faz sync inicial (edição com watch desligado exige rebuild);
`docker compose up -d web` sozinho deixa o código congelado no último build; só 1 watch por projeto
(lock exclusivo — watch zumbi segura o lock).

### [2026-07-03] deploy:fast:prod derrubou produção — runbook AC-04 pulado (~1h de 502)

**O que aconteceu:** logo após um `npm run deploy:fast:prod`, o painel inteiro caiu (502 no Caddy).
Na VPS: api em crash-loop, web preso em `Created` (depends_on api healthy), postgres saudável.

**Causa raiz:** o `deploy:fast` builda do WORKING TREE local — embarcou o AC-04 (role de runtime
`omni_app` least-privilege), que exige passos manuais na VPS ANTES da imagem nova: criar a role no
Postgres (`scripts/db/create-app-role.sql`) e adicionar `APP_DB_ROLE`/`APP_DB_ROLE_PASSWORD` no
`.env.production`. Nada disso tinha sido feito. O compose interpolou senha em branco para role
inexistente → SASL 28P01 a cada boot (o mock SCRAM do Postgres reporta "password authentication
failed" mesmo para role ausente — não confunda com senha errada). O runbook existia e previa a ordem
(doc canônico, Notas de Deploy AC-04); o deploy foi rodado sem conferi-lo.

**Correção:** criar a role + envs na VPS e recriar api/web (runbook passos 2–6); serviço voltou.
Rollback alternativo sem trocar imagem (passo 8): apontar `APP_DB_ROLE` para o superuser
temporariamente. Detalhe em [MULTITENANT_COMPLETION_PLAN.md](MULTITENANT_COMPLETION_PLAN.md) § AC-04.

**Regra criada:** antes de QUALQUER `deploy:fast`, conferir as "Notas de Deploy" do doc canônico por
passos manuais pendentes — o deploy:fast embarca TUDO que está no working tree, inclusive mudança que
exige env var/role/script na VPS (env var nova NUNCA viaja na imagem). Mudança de infra de banco
(role, extensão, grant) deve preferir auto-provisão idempotente no migrate (proposta `ac-04b`) a
passo manual. E crash-loop com `28P01` logo após deploy = checar PRIMEIRO se role/env do banco
existem no ambiente, antes de suspeitar do código.

## Referência cruzada

- Plano canônico da branch atual → [MULTITENANT_COMPLETION_PLAN.md](MULTITENANT_COMPLETION_PLAN.md)
- Roadmap geral → [ROADMAP.md](ROADMAP.md)
- Contratos invariantes → [CONTRACT_FREEZE.md](CONTRACT_FREEZE.md)
- Schema-alvo → [SCHEMA_TARGET.md](SCHEMA_TARGET.md)
