# AGENTS

## Escopo

Estas instrucoes valem para todo o frontend dentro de `web/`.

## Estado atual — branch refactor/multi-tenant-complete (2026-06-01)

Migracao multi-tenant concluida em `refactor/multi-tenant-complete`:

- `web/server/` (Nitro BFF mock) **apagado** — C7.
- Store de simulacao de sessao (lista hardcoded de 6 clientes com IDs fictícios) **apagado** — C8. Tasks-layer migrou para `tasks-client.ts`.
- `web/app/composables/useBffFetch.ts` **apagado** — C7.
- Stores de tasks-layer migrados: `web/layers/tasks/stores/tasks-client.ts` substitui o antigo store de simulacao; campo `clientId` em tasks ainda e integer legado (requer tasks-refactor-v2 para usar UUID).
- `/manage/clientes-web`, `/manage/leads-web`, `/manage/produtos-web`, `/manage/users`, `/manage/organizations` consomem API Go real (`/v1/admin/*`).
- `/site/leads`, `/site/produtos` e `/site/tracking` sao as superficies canonicas do menu `Site`; os atalhos equivalentes em `/manage/*-web` ficam apenas como compatibilidade tecnica e nao como navegacao principal.
- `web/server/` nao existe mais. Nao recriar BFF Nitro — decisao arquitetural documentada em `docs/adr/0002-remove-bff-nitro-mock.md`.

Plano canonico: [docs/MULTITENANT_COMPLETION_PLAN.md](../docs/MULTITENANT_COMPLETION_PLAN.md)

## Stack atual

- Nuxt 4
- Vue 3
- Pinia
- Node 24.11.1 em container
- CSS local em `app/assets/styles/*`
- auth real via API Go + PostgreSQL

## Workflow oficial

Dev do frontend roda NO DOCKER via `docker compose watch` desde 2026-07-03;
producao continua 100% Docker (build multi-stage -> GHCR). O bind mount
`./web:/app` FOI REMOVIDO do compose: caminho Windows atravessa a ponte 9P do
WSL2 (~100x mais lento por arquivo) e o nuxt dev nao terminava nem o boot. O
codigo agora e copiado no build da imagem (target dev) e o watch sincroniza as
edicoes host -> container; o watcher usa inotify real (polling DESLIGADO).

Dia a dia (duas formas equivalentes):

```powershell
# A) stack completa + watch, attached:
npm run dev

# B) stack ja de pe — reconcilia o web (up -d --build) e liga so o watch:
npm run dev:watch     # = scripts/dev/watch-web.ps1
```

IMPORTANTE:

- O watch NAO faz sync inicial (verificado no Compose v2.38): edicao feita com
  o watch DESLIGADO so chega ao container com rebuild e recriacao — e por isso
  que o dev:watch roda `docker compose up -d --build --force-recreate --no-deps web`
  antes (BuildKit reenvia so o que mudou; custa segundos). Sem `--force-recreate`,
  o Compose pode manter o container antigo mesmo com uma imagem nova.
- `docker compose up -d web` sozinho sobe o servidor mas NAO sincroniza
  edicoes — a pagina "congela" no codigo do ultimo build.
- Mudanca em package.json/package-lock.json: o watch faz sync+restart do
  container e o ensure-node-modules roda `npm ci` no volume. NUNCA rode
  `npm install` no host (reescreve o package-lock e quebra o `npm ci` do
  container; ja quebrou).
- Fallback nativo (fora do Docker) continua existindo se um dia precisar:
  `scripts/dev/start-web-native.ps1` (usa `dev:native`, --host :: dual-stack —
  com `--host 0.0.0.0` o browser cai num 426 Upgrade Required no Windows).

No browser:

- `web` responde em `http://localhost:3003`
- a API publica fica em `http://localhost:9091` (API_PORT do .env)

No SSR/container:

- o frontend usa `NUXT_API_INTERNAL_BASE`
- por padrao no Compose: `http://api:8080`

## Objetivo da fase atual

O legado de `core/` ja foi removido do frontend.

Toda implementacao nova em `web/` deve:

- nascer no Nuxt;
- usar stores por dominio em `app/stores`;
- evitar reintroduzir qualquer acoplamento legado no frontend;
- preparar a camada para futura integracao com API.

## Regras de arquitetura

### 1. Stores

- Cada area deve consumir seu store de dominio.
- Nao importar `useDashboardStore()` direto em paginas, components ou composables de tela.
- `web/app/stores/app-runtime.ts` e a ponte de infraestrutura do runtime atual.
- `web/app/stores/auth.ts` e a fonte de verdade da sessao autenticada.
- `web/app/stores/reports.ts` ja consome `GET /v1/reports/overview` e `GET /v1/reports/results` como fonte principal de `/relatorios`.
- `web/app/stores/analytics.ts` ja consome `GET /v1/analytics/ranking`, `GET /v1/analytics/data` e `GET /v1/analytics/intelligence`.
- `web/app/stores/multistore.ts` ja consome o CRUD real de lojas via API.
- `web/app/stores/multistore.ts` ja consome `GET /v1/reports/multistore-overview` como fonte principal do comparativo gerencial.
- `web/app/stores/users.ts` ja consome papeis e usuarios reais via API para a area de acessos.
- `web/app/stores/users.ts` ja trabalha com onboarding por convite e link de aceite.
- `web/app/stores/users.ts` ja expõe o papel `store_terminal` para o acesso fixo da unidade.
- `web/app/stores/access-control.ts` consome a matriz de acessos por perfil e os overrides individuais de usuario via `/v1/access/*`.
- `web/app/stores/feedback.ts` gerencia o fluxo de feedback dos usuarios: envio por qualquer usuario autenticado e gestao (listar, filtrar, atualizar) por administradores via `/v1/feedback`. Feedback e feature do modulo `queue` (gating em `module-enabled.global.ts`); o `FeedbackNotificationsDropdown` so busca e so mostra o sino quando `enabledModules.includes('queue')` — conta sem o modulo (cardapio/bio-only) nao dispara `/v1/feedback/*` (evita 403).
- O feedback de desempenho vive dentro de `/consultor`: o card abre
  `PerformanceFeedbackModal`, que usa `/v1/performance-feedback`, snapshots de KPI e
  blocos livres de `OmniEditor`. `/feedback-desempenho` apenas redireciona por
  compatibilidade e nao possui item de menu ou workspace propria. O dominio continua
  separado do canal publico `/suporte` acima.
- `store_terminal` deve permanecer com workspace enxuta, operacao completa da propria loja e acesso apenas a telas seguras de leitura como `consultor`, `ranking`, `dados`, `inteligencia` e `relatorios`.
- `web/app/components/users/UsersAccessManager.vue` e a referencia atual de grade administrativa sem `<table>`, com filtros locais, colunas configuraveis, detalhes e edicao inline.
- `web/app/components/users/UsersRoleMatrixManager.vue` concentra a edicao do padrao de visibilidade/edicao por papel dentro da workspace de usuarios.
- `web/app/stores/consultants.ts` agora cria consultores ja com conta autenticada vinculada.
- `web/app/composables/useContextRealtime.ts` cuida da sincronizacao administrativa por tenant para atualizar lojas, usuarios e header entre instancias.
- `web/app/composables/useContextRealtime.ts` tambem precisa revalidar `web/app/stores/access-control.ts` quando chegar `context.updated` com `resource=access` ou `resource=user`, senao a workspace de Usuarios e acessos fica stale ate reload.
- `web/app/composables/useContextRealtime.ts` tambem e o caminho oficial para revalidar `web/app/stores/alerts.ts` quando chegar `context.updated` com `resource=alerts`; a UI de alertas nao deve abrir timer proprio para decidir vencimento de `long_open_service`.
- `web/app/utils/api-client.ts` concentra o client HTTP criado dentro do contexto do store.
- `web/app/composables/useOperationsRealtime.ts` cuida da assinatura WebSocket da operacao e revalidacao do snapshot em tempo real.
- `web/app/composables/useOperationsRealtime.ts` agora tambem cuida do modo integrado multi-loja de `/operacao` quando a sessao tiver mais de uma loja acessivel.
- os cronometros exibidos em `/operacao` continuam sendo apenas representacao visual do estado atual; a decisao de disparar ou resolver alerta temporal pertence ao backend em `operations`.
- mudancas em `settings` devem continuar propagando sem refresh via `context.updated` com `resource=settings` e `resourceId={tenantId}`.
- `web/app/utils/runtime-remote.ts` hidrata consultores, settings e operations remotos para a loja ativa.
- `web/app/plugins/account-id-bridge.client.ts` injeta `X-Account-Id` a partir de `useCoreAccountStore().activeAccountId`; ha fallback temporario para `auth.activeTenantId` apenas durante o boot antes da account ativa hidratar.
- toda chamada a `hydrateRuntimeStoreContext`, `refreshRuntimeStoreSettings` ou `fetchRemoteStoreData` deve repassar o `tenantId` (ex.: `auth.activeTenantId`); sem ele a query `tenantId` some e usuarios `platform_admin` com mais de um tenant acessivel recebem 400 `validation_error` em `/v1/settings`.
- `web/app/stores/dashboard.ts` e apenas uma facade temporaria de compatibilidade.
- O runtime de compatibilidade foi fatiado em `web/app/stores/dashboard/runtime/shared.ts`, `state.ts`, `status.ts` e `actions/*`.
- Se precisar tocar esse runtime, editar o menor modulo responsavel e nao voltar a concentrar regra em `create-dashboard-runtime.ts`.
- Se precisar de novo dominio, criar um store proprio em `app/stores`.
- Ao adicionar workspace novo no dashboard, sincronizar sempre `workspaces.ts`, `permissions.ts` e `web/layers/*/nav.config.ts` (o menu legado `web/app/utils/sidebar-nav.ts` foi removido em 2026-06-13; o nav vem 100% dos layers); no caso do menu `Site`, ids irmaos nao podem compartilhar o mesmo `workspaceId` ou o item ativo fica incorreto.
- Gating de modulos por conta ativa (modo "view-as" do admin): o switcher e a ferramenta do admin para ver como o cliente. Menu (`useDashboardNav.ts` → `isItemAllowed`) e rotas (`module-enabled.global.ts` → `MODULE_PATH_GUARDS`) filtram por `useCoreAccountStore().enabledModules` da conta ativa SEMPRE que houver `activeAccount` carregada — inclusive para `platform_admin` (nao ha mais bypass de admin no front). Conta com 0 modulos → so itens `core`/Manage (moduleId vazio ou `core`, fora de `MODULE_PATH_GUARDS`). O gating e **fail-closed** via `useCoreAccountStore().accountsLoaded` (vira `true` no `finally` do `fetchAccounts`, sucesso OU erro): item/rota de modulo sem `activeAccount` MAS com `accountsLoaded === true` (sem membership, `/v2/me/accounts` vazio ou erro) e escondido/redirecionado — nao revela modulo so pelo papel. Enquanto `accountsLoaded === false` (hidrate inicial) NAO filtra, para evitar flash de menu vazio; `reset()` volta `accountsLoaded` para `false`. Item de nav novo com `moduleId` exige a tag espelhada em `MODULE_PATH_GUARDS`, senao o menu esconde mas a rota direta abre. Rota gated sem o modulo redireciona para `/perfil` (fallback nunca-gated e sem `workspaceId`, seguro contra loop com `index.vue`/`auth.homePath`). O gating de API `RequireModuleByPath` no back segue isentando `platform_admin` de proposito (o admin precisa da API para o Manage; o bloqueio de rota no front ja impede abrir a pagina gated).
- Gating do runtime de fila (`/v1/settings`) por modulo `queue`: conta sem o modulo `queue` NAO busca `/v1/settings` (config da fila) — evita o 403 `module_disabled` e o aviso "Modo degradado de configuracoes" indevido. O gate passa pelos callers: `auth.ts` (`syncRuntimeAccess`/`setActiveStore`) resolve `canFetchQueueSettings()` (garante `fetchAccounts` carregado e checa `enabledModules.includes('queue')`) e repassa para `hydrateRuntimeStoreContext`; os refresh em tempo real (`useContextRealtime.ts`/`useOperationsRealtime.ts`) passam `{ canFetchQueueSettings: enabledModules.includes('queue') }` para `refreshRuntimeStoreSettings`. Em `runtime-remote.ts`, `canFetchQueueSettings === false` pula o `apiRequest('/v1/settings')` e resolve `settingsLoadState: 'skipped'` (estado NEUTRO — nao degradado; usa o fallback de settings sem aviso). `applyRuntimeSettingsStatus` trata `'skipped'` limpando qualquer aviso anterior. O banner em `dashboard.vue` so renderiza com `runtimeSettingsNotice && hasQueueModule` (`enabledModules.includes('queue')`). Quem TEM `queue` mantem o fluxo de hoje (default `canFetchQueueSettings: true`).
- Login blindado contra falha de dado da fila (`runtime-remote.ts` → `fetchRemoteStoreData`): o roster de gestao (`/v1/consultants`) e o snapshot da operacao (`/v1/operations/snapshot`) sao **best-effort** e NUNCA derrubam o login/boot. Qualquer erro nessas duas chamadas degrada para vazio (`consultantsLoadState`/`operationsSnapshotLoadState = 'degraded'`, roster `[]` / snapshot `null` que `applyRemoteStoreData` tolera) e o login completa; `logRemoteDataDegraded` registra `console.warn('[runtime-remote]', …)` com status/mensagem para diagnostico. ANTES so 403 (consultores) e 400/403 (snapshot) degradavam: um 500 inesperado era re-lancado, o `catch` do `login()` em `auth.ts` limpava a sessao e travava o usuario na tela de login com a mensagem do backend (ex.: "Erro ao processar o consultor." quando uma VPS de schema stale tinha `core.users` sem `employee_code`). Settings continua com a logica propria (`skipped`/`degraded`), independente disso.
- **Login sem workspace (correcao 2026-08-12):** usuario ativo com credenciais validas autentica mesmo com `allowedWorkspaces=[]`. `auth.homeWorkspaceId` fica vazio e `auth.homePath` aponta para `/perfil`, rota universal sem `workspaceId`/gate de modulo. NUNCA restaurar o fallback `/operacao`: `auth.global` rejeita a workspace e redireciona novamente para `homePath`, criando loop na tela "Entrando...". `/perfil` e a pagina comum para identidade, senha e avatar de qualquer usuario autenticado.
- Switcher de contas (`CoreAccountSwitcher.vue`) e ferramenta SO de `platform_admin`: no `DashboardHeader.vue` ele so renderiza com `isAuthenticated && hasMultipleAccounts && role === 'platform_admin'` — cliente comum nao ve o botao. O dropdown tem 3 secoes com divisor: (1) **Admin da plataforma** — cabecalho reservado para itens so-admin futuros; hoje so um placeholder muted "Plataforma"; (2) **Organizacoes** — contas com `isAgency === true` (conta-agencia, ex.: Crow Visuals), ordenadas por nome; (3) **Clientes** — contas com `isAgency === false`, AGRUPADAS por `organizationName` (sub-cabecalho por org), ordenadas por nome dentro de cada grupo. Os 2 campos `isAgency`/`organizationName` vem de `/v2/me/accounts` na `AccountSummary` e sao normalizados com default (`false`/`''`) em `fetchAccounts` — tratar undefined sem quebrar.
- Gating agencyOnly (superficies internas so na agencia): itens globais da plataforma (`/manage/users`, `/manage/organizations`, `/manage/role-templates`, `/manage/clientes-web`) e as superficies internas `/themes`, `/manage/auditoria`, `/manage/integracoes` e `/roadmap` tem `agencyOnly: true` no `nav.config.ts`. `useDashboardNav.isItemAllowed` esconde esses itens quando `activeAccount?.isAgency` e falsy, e `module-enabled.global.ts` usa a politica pura de `account-route-access.ts` para redirecionar as URLs diretas a `/perfil`. Itens operacionais do Manage (com `moduleId`, ex.: Fila) seguem gated por modulo, nao por `agencyOnly`; por isso o grupo pai nao deve ser marcado como exclusivo da agencia.
- Controles compactos de `/calendario`: o botao de configuracao permanece visivel, mas so habilita quando `activeAccount.isAgency === true`; conta-cliente nao abre o drawer nem por `?config=`. O gerador de plano por IA fica oculto ate liberacao de produto, o chat usa seus gatilhos dedicados fora dessa barra e o toggle de barras de tarefas so aparece quando uma tarefa com inicio/fim intersecta o mes em foco.
- O laboratorio `/editor` pertence ao modulo `tools`: sua entrada de nav declara `moduleId: 'tools'` e a URL direta e espelhada por `EDITOR_MODULE_PATH_GUARD`. Conta somente com `calendar` nao pode revelar Tools por causa desse item legado.

### 2. SSR e Pinia

- Estado exposto por store precisa ser serializavel.
- Nao retornar objetos com funcoes aninhadas dentro de propriedades de estado.
- Ao extrair refs de stores, usar `storeToRefs()`.
- Evitar ler store Pinia como se computed/ref desembrulhado ainda tivesse `.value` fora do contexto correto.

### 3. Organizacao de codigo

- `app/pages`: paginas finas, sem regra pesada.
- `app/components` e `app/features`: interface e comportamento de tela.
- `app/composables`: composicao de comportamento compartilhado da UI.
- `app/stores`: estado e actions por dominio.
- `app/domain/utils`: funcoes puras de negocio reutilizaveis no frontend.
- `app/domain/data`: defaults, templates e seeds locais enquanto a API nao entra.
- `app/utils`: helpers de app e infraestrutura de frontend.

### 4. Legado

- Nao criar novos imports de `@core/*`.
- Nao recriar alias legado no `nuxt.config.ts`.
- Se surgir codigo utilitario compartilhavel, ele deve nascer em `web/app/domain/*` ou `web/app/utils/*`.

### 5. Integracao futura com API

Ao criar ou ajustar actions:

- pensar no formato futuro de request/response;
- manter a action do store como fachada unica para a UI;
- evitar espalhar persistencia diretamente dentro de componentes;
- evitar persistencia client-side para fonte de verdade.
- `operations`, `consultants` e `settings` ja usam a API Go + PostgreSQL como fonte principal.
- o picker de produtos do modal de `/operacao` deve consumir `GET /v1/catalog/products/search`; `settings.productCatalog` fica apenas como catalogo manual/administrativo e nao como fonte principal dessa busca.
- `reports` ja usa a API Go como fonte principal para a tela `/relatorios`.
- `analytics` ja usa a API Go como fonte principal para `/ranking`, `/dados` e `/inteligencia`.
- `multiloja` ja usa a API Go para CRUD de lojas e gestao de usuarios/acessos.
- `usuarios` agora e workspace propria e deve concentrar o CRUD administrativo de acesso.
- `perfil` e pagina de autoatendimento do usuario autenticado, separada da area administrativa.
- `perfil` tambem e o lugar obrigatorio para sair de senha temporaria no primeiro acesso.
- `/operacao` agora diferencia:
  - leitura da loja ativa por snapshot
  - leitura integrada de todas as lojas acessiveis por `overview`
- a troca entre `Loja ativa` e `Todas as lojas` deve continuar explicita na propria tela de operacao, nao escondida apenas no header.
- o seletor global do header deve refletir o escopo global salvo na sessao sem redirecionar o usuario para `/operacao`.
- a opcao `Todas as lojas` no header deve aparecer sempre que a sessao tiver mais de uma loja acessivel, mesmo fora das rotas que suportam leitura integrada.
- quando `Todas as lojas` estiver ativo e a rota nao suportar leitura integrada, o header deve permanecer em `Todas as lojas` e a pagina continua exibindo o recorte padrao da loja ativa.
- hoje o seletor global tambem deve filtrar corretamente `/operacao`, `/relatorios`, `/ranking`, `/dados`, `/inteligencia`, `/consultor` e `/campanhas`.
- `/consultor` em `Todas as lojas` deve consolidar o roster acessivel, comparativos por loja e filtros locais por loja, nome, status e meta.
- `/campanhas` em `Todas as lojas` deve consolidar o historico das lojas acessiveis e oferecer filtro local por loja sem perder o escopo global salvo no header.
- para tirar alguem da fila por tarefa ou reuniao, usar `operationsStore.assignTask(...)`; nao reaproveitar pausa generica sem distinguir o tipo.
- contas `consultant` devem nascer pela gestao de consultores, nao pela tela administrativa de usuarios.
- a tela `usuarios` nao deve editar, convidar nem inativar contas `consultant`; ali so cabe listar e resetar senha quando necessario.
- o runtime local deve ficar restrito a compatibilidade de tela, estado efemero e drafts de UI.
- `ranking`, `dados` e `inteligencia` nao devem voltar a recalcular historico bruto no browser quando houver endpoint server-side cobrindo o caso.

No auth:

- usar `POST /v1/auth/login` e `GET /v1/me/context` como fluxo principal de sessao;
- usar `GET /v1/auth/invitations/{token}` e `POST /v1/auth/invitations/accept` para primeiro acesso por convite;
- usar `POST /v1/auth/password-reset/request` e `POST /v1/auth/password-reset/confirm` para o fluxo publico de esqueci minha senha;
- deixar `auth` como bootstrap da hidratacao remota por loja para `consultants` e `settings`;
- deixar `GET /v1/auth/me` como endpoint auxiliar de identidade quando precisar;
- guardar o token no cookie de app;
- derivar workspaces e loja ativa a partir do principal autenticado;
- usar `NUXT_PUBLIC_API_BASE` para chamadas do browser;
- usar `NUXT_API_INTERNAL_BASE` para SSR/container.
- o link de onboarding entra por `web/app/pages/auth/convite/[token].vue` e deve continuar usando o mesmo layout `auth`.
- a recuperacao publica de senha entra por `web/app/pages/auth/esqueceu-senha.vue` e deve continuar no mesmo layout `auth`.
- o header autenticado deve continuar oferecendo acesso rapido a `/perfil`.
- para acesso `store_terminal`, a tela `/operacao` deve manter comandos operacionais liberados apenas na propria loja.
- quando `auth.user.mustChangePassword` vier verdadeiro, o frontend deve redirecionar para `/perfil` e bloquear o restante ate a troca de senha.
- usuario autenticado sem modulo/permissao tambem deve entrar em `/perfil`; ausencia de workspace e autorizacao vazia, nao falha de autenticacao.

## Regras de implementacao

- Preferir composicao por dominio, nao um store gigante.
- Manter classes CSS semanticas.
- Reaproveitar componentes existentes antes de criar variacoes paralelas.
- `AppSelectField.vue` e o seletor simples padrao do app e deve seguir a mesma linguagem visual do `product-pick`.
- `SettingsOptionManager.vue` e o ponto unico para catalogos ordenaveis simples da area de configuracoes, incluindo `pausas`.
- Para listagens administrativas reutilizaveis, preferir `AppEntityGrid.vue` em vez de tabelas HTML novas.
- Quando a listagem precisar de status booleano ou detalhes laterais/modal, compor com `AppToggleSwitch.vue` e `AppDetailDialog.vue` antes de criar widgets paralelos.
- Componente carregado sob demanda (chunk lazy) deve usar `defineLazyComponent` (`~/utils/lazy-component`), nunca `defineAsyncComponent` pelado: o wrapper re-tenta o import e mostra um errorComponent acionavel (`CoreAsyncError`) se o chunk falhar, em vez de tela em branco silenciosa (ver `.claude/skills/principios-engenharia/references/registro-de-falhas.md` #10).
- Quando alterar comportamento de tela importante, atualizar a documentacao correspondente.

## Typecheck (Fase 6.4 do PLANO_REFATORACAO)

Configurado a partir de 2026-05-18:

- `vue-tsc` para checagem de tipos em `.vue`/`.ts`
- Config própria em [tsconfig.json](tsconfig.json) (estende `./.nuxt/tsconfig.json`)
- Endurecimento **gradual**: `strict: false` agora; flags estruturais ligadas; flags pesadas (`noImplicitAny`, `strictNullChecks`) entram nas Fases 7 e 8.

Scripts:

```bash
npm --prefix web run typecheck         # checagem com config atual (pragmatica)
npm --prefix web run typecheck:strict  # checagem com --strict cheio (alvo aspiracional)
```

Baseline registrada em 2026-05-18:

- Config atual (pragmatica): **387 erros TS** — dominados por TS2339 (336 — property does not exist on type) que vao cair na Fase 7 com fatiamento + tipagem.
- Config strict cheio: **388 erros TS** — quase igual; significa que a divida real esta em propriedades nao tipadas, nao em null safety.

**Importante**: `typecheck` NAO esta no pre-commit hook nem no `npm run build` para nao bloquear desenvolvimento atual. Sera adicionado ao CI na Fase 9 (testes/CI).

## Lint, format e testes (Fase 6 do PLANO_REFATORACAO)

Configurados a partir de 2026-05-18:

- **ESLint** (`@nuxt/eslint` standalone, flat config em [eslint.config.mjs](eslint.config.mjs))
- **Prettier** (config em [.prettierrc.json](.prettierrc.json), ignores em [.prettierignore](.prettierignore))
- **eslint-plugin-unused-imports** habilitado

Scripts:

```bash
npm --prefix web run lint           # ver problemas (warning + error)
npm --prefix web run lint:fix       # auto-fixar o que dá
npm --prefix web run format         # aplicar Prettier em todos os arquivos
npm --prefix web run format:check   # so checar sem alterar
```

Regras com peso especial:

- `max-lines: 500` (WARN durante Fase 6; vira ERROR após a Fase 7.5 de fatiamento)
- `@typescript-eslint/no-explicit-any` (WARN)
- `no-console` (WARN — `console.warn`/`console.error` permitidos)
- `no-debugger` (ERROR)

Baseline registrada em 2026-05-18: **0 errors, 193 warnings**. Top categorias: 75 `any`, 40 `max-lines`, 26 `no-dynamic-delete`, 24 unused vars. A redução acontece de forma incremental nas Fases 7 e 8 do PLANO_REFATORACAO.

Onda 1 de testes (AC-15, 2026-07): cobertura de `domain/utils/permissions` (matriz role×workspace + fail-closed + bypass platform_admin), `reports`, `color`, `erp-display` e das stores `settings`/`multistore`/`cardapio` com `$fetch` global mockado (`web/test/setup.ts`). Testes determinísticos: sem rede, sem timers reais, datas sem timezone, moeda normalizando NBSP. `stores/erp.ts` ficou de fora de propósito (backlog de refatoração AC-07). Rodar com `docker compose run --rm web npm run test`.

## Validacao minima

Quando houver mudanca de codigo em `web/`:

- rodar `npm --prefix web run lint` para garantir que nao introduziu novo error;
- rodar `npm --prefix web run build` sempre que viavel;
- conferir se nao voltou nenhum import de `@core/*`;
- conferir se paginas/workspaces nao passaram a depender direto do bridge monolitico.

## Referencias internas

- `../AGENT.md`
- `../docs/NUXT_4_STORE_ARCHITECTURE.md`
- `PANEL_EMBED_CONTRACT.md`
- `app/components/AGENT.md`
- `app/components/operation/AGENT.md`
- `../docs/operacao/operations.md`

## Comandos uteis

```bash
npm run dev
npm run dev:build
npm run dev:logs
npm run dev:down
npm run build
npm run preview
```
