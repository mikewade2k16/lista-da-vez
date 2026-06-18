# AGENT_RULES.md

Regras canonicas que todo agente/IA deve ler antes de iniciar qualquer tarefa neste projeto.

Fonte de verdade: este arquivo. UI vive em `/roadmap` > aba "Regras" e exporta este arquivo.
Princípios de engenharia aprofundados (pilares segurança/performance/UX, registro de falhas): [docs/ENGINEERING_PRINCIPLES.md](docs/ENGINEERING_PRINCIPLES.md). Quando a Frente B (modulo Go `roadmap`) for entregue, este `.md` passa a ser regenerado automaticamente a partir do banco.

Para entender o que esta pronto / em beta / pendente, consulte `/roadmap` > aba "Modulos".

---

## Legado, mocks e fonte da verdade (PRIORIDADE MAXIMA)

Objetivo final: **uma unica fonte de verdade no banco real**. Nada de tabela/codigo legado paralelo, front mock, dado so em localStorage ou qualquer coisa que nao persista no banco pode ficar escondido como se estivesse pronto.

### Nunca deixar legado/mock para tras — resolver o quanto antes
- Se uma feature so fica "pronta" removendo o legado, ela **NAO esta pronta** enquanto o legado existir. Tabelas legadas paralelas (ex.: `user_tenant_roles`/`user_store_roles`/`user_platform_roles` vs `core.account_users` + `core.user_role_assignments`), front mock, dado so em localStorage, BFF mock, qualquer coisa que nao seja banco real → devem ser **ELIMINADOS**, nao mantidos vivos.
- Manter DOIS sistemas em sync (ex.: gravar papel no legado E no core) e' **band-aid temporario** e PRECISA estar marcado para remocao, nunca tratado como solucao final.

### Sempre AVISAR, DOCUMENTAR e MOSTRAR no front (so admin) o que e' legado/mock
Ao tocar em qualquer modulo e encontrar legado / mock / localStorage / nao-persistido / qualquer coisa que nao seja banco real, e' **OBRIGATORIO**:
1. **Avisar o usuario na hora** — dizer explicitamente "isso aqui ainda e legado/mock, nao esta no banco real, nao esta pronto".
2. **Documentar** — no `AGENT.md` do modulo E num registro central [docs/LEGADO.md](docs/LEGADO.md) (criar se nao existir): o que e', por que e' legado, qual o alvo, status de remocao.
3. **Mostrar no front, SO para `platform_admin`** — um marcador visivel (badge/aviso tipo "MOCK", "LEGADO", "localStorage", "nao persiste") na propria tela, para ninguem desenvolver achando que esta pronto quando e' mock/localStorage/legado.

- **Por que:** ja se perdeu tempo desenvolvendo sobre coisas que pareciam prontas mas eram mock/localStorage/legado (BFF Nitro, session-simulation, etc.). Marcador visivel + documentacao + aviso evita repetir.
- **Aplica quando:** SEMPRE que encontrar ou criar dependencia legada/mock — mesmo que temporaria.

### Modelo-alvo de usuario (exemplo concreto do principio)
Um usuario = **UMA linha em `core.users`** (fonte da verdade), sem tabela legada paralela. Config/opcoes **especificas por modulo** NAO viram tabela legada nem coluna espalhada: vivem em jsonb por modulo — ex.: tabela `core.user_module_settings (user_id, module_id, config jsonb)` (ou coluna `module_settings jsonb` em core.users). Cada modulo (tela) renderiza a **projecao/opcoes dele** sobre o MESMO usuario:
- `/operacao/usuarios` = usuarios daquele cliente no modulo Fila, com as opcoes especificas da Fila.
- `/manage/users` = visao global de identidade (admin).
- outro modulo = outras opcoes especificas daquele modulo.

Papeis/permissoes migram 100% para `core.*` (`core.account_users` + `core.user_role_assignments` + `core.role_permissions`), eliminando `user_tenant_roles`/`user_store_roles`/`user_platform_roles`. O auth deve resolver papel a partir do `core.*`, nao do legado.

### Nada hardcoded — toda informacao vem do banco (round-trip completo banco <-> back <-> front)

Nao trabalhamos com dado hardcoded. **Toda** informacao exibida no front vem do banco, atraves do back; **toda** alteracao no front vai ao back e e' gravada no banco. O fluxo e' SEMPRE:
- **Leitura:** banco -> back (API) -> front.
- **Escrita:** front -> back (API) -> banco -> e o front re-le do banco (ou usa o objeto que a API retornou). Nunca confiar so no estado local "achando que salvou".

Regras concretas:
1. **Sem valor cravado no codigo nem default que minta.** Nenhum default no front (ex.: `'bairro'`) pode sobreviver a uma leitura do banco que diga outra coisa. Default e' so o estado inicial *antes* do dado real chegar — assim que o endpoint autoritativo responde, o valor do banco vence.
2. **So a fonte autoritativa do recurso renderiza o dado de verdade.** O front NUNCA mostra um campo real (que existe no banco) a partir de fonte parcial/secundaria (contexto, cache, fallback, localStorage) que nao tem aquele campo. Se a fonte autoritativa (o endpoint do recurso) ainda nao chegou, mostrar loading/vazio — nao um default que mascare o dado.
3. **Draft/estado de edicao re-hidrata do back.** Qualquer rascunho de edicao no front e' re-hidratado a partir da resposta do back assim que ela chega; so se preserva enquanto houver edicao pendente do usuario naquela linha/campo (flag `touched`/dirty). Draft semeado de fonte incompleta NAO pode "grudar".
4. **Cadeia completa para campo novo.** Campo novo numa tela exige a cadeia inteira confirmada: coluna/jsonb no banco -> SELECT no repo -> DTO no service/handler -> store/composable no front -> componente. Faltou um elo = o dado nao fecha o ciclo.

- **Por que:** Aconteceu (2026-06-17): o "Tipo de loja" (`store_type`) no multiloja era gravado certo no banco (`queue.stores`, e a API `/v1/stores` ja devolvia `storeType`), mas o front montava o draft a partir de `auth.storeContext` (contexto, SEM `storeType`) que chegava ANTES do `/v1/stores`; um `??` preservava esse draft semeado de `'bairro'` e ignorava o valor real (`'shopping'`) que vinha do banco logo depois. O usuario trocava para Shopping, recarregava e "voltava" para Bairro — parecia que nao salvava, mas o banco ja estava correto; o bug era o front nao re-hidratar do banco.
- **Aplica quando:** SEMPRE. Ao depurar "nao salvou / reverteu", checar PRIMEIRO se o dado esta no banco (psql) e se a API autoritativa o devolve, ANTES de mexer no back — o problema costuma ser o front exibindo de uma fonte que nao e' o banco. Detalhe dos pilares em [docs/ENGINEERING_PRINCIPLES.md](docs/ENGINEERING_PRINCIPLES.md).

### Mapa: calculo de comissao (recebimento por atingimento de meta) — onde vive

Exemplo canonico de "logica no back, fonte unica, embutida no payload" + "config e dado no banco". Para achar rapido:

- **Politica (config editavel na tela):** JSONB `queue.tenant_operation_core_settings.crm_goal_payout_policy` (por tenant). Migrations `0139`/`0162`/`0163`. Default no back: `back/internal/modules/queue/settings/defaults.go` (`defaultCRMGoalPayoutPolicyJSON`). Grupos: `consultant`, `managerShopping`, `managerBairro`, `support` + `consultantRules` (base, penalidade por metrica, gate da loja: floor/full/reducedRate/reducedRequiresOwnPercent).
- **Editor (front):** `web/app/components/settings/sections/SettingsCrmGoalsSection.vue` + `SettingsCrmConsultantRules.vue` → `useSettingsWorkspace.js` → `stores/settings.ts` `updateCrmCommercialPolicy` → `PATCH /v1/settings/crm-policy` (auth: `platform_admin`/`director`).
- **Calculo (FONTE UNICA, Go puro):** pacote `back/internal/modules/queue/commission/` — `calculate.go` (`Calculate`, `ResolveRule`, `MapRoleToGroup`), `model.go` (`Policy`/`Input`/`Result`), `policy_json.go` (parse do JSONB). Tipos espelhados (so normalize, NAO recalcula) no front: `web/app/domain/utils/crm-performance-policy.ts`.
- **Onde e aplicado (embute no payload existente):** `back/internal/modules/crm/erp/repository_crm_payout.go` (`loadCRMPayoutInputs` + `applyCRMPayouts`), chamado em `repository_crm.go` no build do `GET /v1/erp/crm`. Metas vem de `queue.operation_goal_targets` (meta de loja = `consultant_id` null; meta de consultor = `consultant_id` preenchido; ticket/PA do consultor herdam os da loja quando 0).
- **DTO consumido pelo front (`crm/erp/model.go`):** por consultor `payout {amount, ratePercent, base, group, ruleLabel, penaltyApplied}` + `goalProgress` (% do consultor); por loja `managerPayout`/`supportPayout` + `storeProgress`/`storeSold`/`storeGoal`/`storeType` (% da loja). O front SO exibe (nao recalcula); `mapRoleToPayoutGroup` decide consultor/gerente/caixa.
- **AGENT.md detalhados:** `back/internal/modules/queue/commission/AGENT.md`, `crm/erp/AGENT.md`, `queue/settings/AGENT.md`.

---

## Frontend

### Componentes reutilizaveis acima de tudo
Sempre que houver repeticao de markup ou logica visual, extrair em componente proprio em `web/app/components/` ou na layer adequada. Workspaces nao podem virar arquivos gigantes; quebrar em cards/secoes/listas.

- **Por que:** Evita duplicacao e drift visual entre paginas. Facilita aplicar mudanca em um lugar so.
- **Aplica quando:** Qualquer feature nova ou refactor que adicione UI.

### Classes semanticas BEM-like
Sempre usar nomes semanticos no estilo `.nome-componente__elemento--modificador`. Nao usar utility classes inline ou IDs para estilizacao.

- **Por que:** Permite leitura rapida do escopo de cada estilo e evita colisao global.
- **Aplica quando:** Estilizacao de qualquer componente novo.

### Seguir o design system — usar as variaveis de cor, nunca hex hardcoded
O projeto TEM design system (tokens em `web/app/assets/styles/omni-tokens.css` + aliases em `tokens.css`). Toda cor/borda/sombra/raio de componente novo usa as variaveis existentes — **nunca** hex/rgb cravado e **nunca** inventar nome de variavel.

| Situacao | Errado | Certo |
|---|---|---|
| Cor cheia | `#16a34a`, `var(--color-primary, #16a34a)` | `rgb(var(--primary))` |
| Cor com transparencia | `rgba(99,102,241,.16)` | `rgb(var(--primary) / 0.16)` |
| Texto / texto fraco | `#111827` / `#6b7280` | `var(--text-main)` / `var(--text-muted)` |
| Borda | `1px solid #e5e7eb` | `1px solid var(--line-soft)` |
| Fundo de card/input | `#fff` | `rgb(var(--surface) / 0.9)` / `rgb(var(--surface-2) / 0.76)` |
| Sombra / raio | `0 8px 24px rgba(...)` / `0.75rem` | `var(--shadow-card)` / `var(--radius-card)` |
| Botao primario | `background: var(--color-primary)` | `linear-gradient(135deg, rgb(var(--primary)), rgb(var(--primary-600)))` |

Tokens base (triplet RGB): `--bg --surface --surface-2 --border --text --muted --primary --primary-600 --success --danger --ring`. Aliases prontos: `--text-main --text-muted --line-soft --line-strong --shadow-card --shadow-shell --radius-card --radius-soft --accent-warning`. Os tokens ja viram dark mode / temas (`.dark`, `.theme-apple-blue`) sozinhos.

- **Por que:** Aconteceu (AutomationWorkspace.vue): o CSS usava `var(--color-primary, #16a34a)` etc. — esses nomes `--color-*` NAO existem no design system, entao caia sempre no fallback hex e o componente ignorava o tema (ficava verde/claro fora do dark mode do resto do painel). Hex hardcoded = componente que nao acompanha tema nem rebranding.
- **Aplica quando:** Qualquer `<style>` de componente novo ou refactor. Conferir o nome do token em `omni-tokens.css`/`tokens.css` antes de usar; se a cor que voce precisa nao existe como token, perguntar/adicionar token — nao cravar hex.

### Layout compacto — horizontal, minimiza scroll vertical
Em editores/workspaces, preferir layout HORIZONTAL e denso ao inves de uma-propriedade-por-linha. Agrupar campos relacionados de um mesmo item na MESMA linha via grid responsivo (`grid-template-columns: repeat(auto-fit, minmax(...))`). Listas de itens repetidos (lojas, slides, links) ficam em GRID de cards lado a lado, nao empilhados um por linha. Reduzir o gap vertical entre blocos. Colapsar para coluna em telas estreitas (o `auto-fit`/`minmax` ja faz isso). **Barras de acao em UMA linha**: botoes cujo significado o icone ja transmite ficam **so com icone** (sem label), com o nome no **hover** (`title`/tooltip); reservar label de texto so para a acao primaria. Padding/altura das barras enxutos — nada de barra alta com muito espaco vazio.

- **Por que:** Editor de bio tinha cada campo numa linha e cada loja/slide empilhado, gerando scroll vertical longo so para ver poucas opcoes. Layout horizontal/denso cabe muito mais em tela sem rolar e da visao do conjunto.
- **Aplica quando:** Qualquer editor/workspace com varios campos ou lista de itens repetidos.

### Blocos de edicao colapsaveis (accordion)
Blocos de edicao com varios campos ou listas (Menu do topo, Links, Slides, Lojas, carrossel, etc.) devem ser COLAPSAVEIS: o usuario abre so o bloco que vai mexer e os demais ficam recolhidos (mostrando so o titulo + um resumo, ex.: contagem de itens). Cada bloco guarda seu estado aberto/fechado.

- **Por que:** Com todos os blocos abertos, o editor vira um scroll vertical sem fim. Colapsar da a visao do conjunto e foca no que esta sendo editado — junto da regra de layout compacto.
- **Aplica quando:** Qualquer editor/workspace com multiplos blocos/secoes de campos repetidos.

### Pagina nova precisa rolar como as outras (overflow do layout)
O layout `dashboard` envolve a pagina em `.module-workspace-full` que e `overflow: hidden`. O componente-raiz da pagina precisa ser o container de rolagem: `flex: 1; min-height: 0; overflow-y: auto` (ou usar o wrapper `.page-workspace`). Sem isso o conteudo que passa da altura fica **cortado, sem scroll**.

- **Por que:** Aconteceu (AutomationWorkspace.vue): a raiz so tinha `padding`, sem `flex/overflow`, entao o editor de persona longo ficava cortado e a pagina nao rolava como todas as outras.
- **Aplica quando:** Criar componente-raiz de pagina/workspace nova no layout dashboard.

### Sem emojis em UI nem em codigo
Nao usar emojis em labels, mensagens de UI, codigo, comentarios ou commits, salvo se o usuario pedir explicitamente.

- **Por que:** Mantem consistencia visual e profissional do produto.
- **Aplica quando:** Sempre.

### Esconder pagina nao pronta via hidden no menu
Quando um modulo/pagina nao esta pronto, usar `hidden: true` em `web/app/utils/sidebar-nav.ts` E em `web/layers/queue/nav.config.ts`. Para itens em beta, usar `beta: true` (renderiza badge).

- **Por que:** Evita que usuario navegue para pagina quebrada. Beta deixa explicito que a feature pode mudar.
- **Aplica quando:** Adicionar/remover modulo do menu lateral.

### Criar pagina nova — checar rota-pai, gating de path e workspace ANTES (falha silenciosa)
Ao criar `web/app/pages/<...>.vue`, rodar esta checagem ANTES. Os tres primeiros itens falham **sem erro de build nem de type-check** — so aparecem testando a rota no browser (a tela "abre outra pagina" ou redireciona):

1. **Rota-pai engole a filha.** Se ja existe o ARQUIVO `pages/<x>.vue`, qualquer `pages/<x>/<y>.vue` vira rota-FILHA dele e so renderiza se `<x>.vue` tiver `<NuxtPage/>`. Sem isso, o pai mostra o proprio conteudo e a filha "some". Antes de criar `pages/<x>/<y>.vue`, conferir que NAO existe `pages/<x>.vue` (arquivo) — se existir, usar outro prefixo.
2. **Gating de path em `module-enabled.global.ts`.** A path herda o gating do prefixo: cair num `MODULE_PATH_GUARDS` (`/configuracoes`, `/operacao`, `/consultor`, `/ranking`, `/dados`, `/relatorios`, `/multiloja`, `/alertas`, `/feedback` → `queue`; `/crm`, `/erp` → `crm`; `/site/*` → `site`; `/cardapio` → `cardapio`; `/meta-ads` → `meta_ads`) exige aquele modulo contratado pela conta ativa. Pagina GLOBAL/admin (nao-de-modulo) NUNCA pode ficar sob prefixo de modulo. `/manage/*` (fora de `AGENCY_ONLY_PATHS`) e "sempre acessivel" — lar correto de tela global/admin de plataforma.
3. **Gate de workspace em `auth.global.ts`.** `definePageMeta({ workspaceId })` redireciona se o id nao estiver em `auth.allowedWorkspaces`. Por isso o wiring de 3 arquivos e obrigatorio: `web/app/utils/workspaces.ts` + `web/app/domain/utils/permissions.ts` (com o id no `ROLE_WORKSPACES` do papel-alvo) + `web/layers/queue/nav.config.ts` — ver ENGINEERING_PRINCIPLES (registro 2026-05-29).
4. **Estatico vence dinamico.** Se a pasta tem `[param].vue`, a rota estatica nova vence no path exato (ok) — mas confirme que e a SUA pagina que renderiza.

- **Por que:** Aconteceu (2026-06-16): criei `pages/configuracoes/menu.vue` (config GLOBAL do menu) e a rota abria a pagina da FILA — `configuracoes.vue` (arquivo) virava rota-pai e engolia a filha; alem disso `/configuracoes` e gated por `queue` (uma config de plataforma nao pode depender do modulo da Fila). Resolvido movendo para `pages/manage/menu-layout.vue` (`/manage/menu-layout`, sempre acessivel). Nenhum dos tres checks da erro de build/type-check — so o browser revela.
- **Aplica quando:** Criar QUALQUER pagina nova (`pages/**/*.vue`). Rodar os 4 checks; depois abrir a rota no browser pelo papel-alvo e confirmar que renderiza a pagina certa (nao a do pai nem um redirect).

### Cabecalho de pagina admin SEMPRE via `AdminPageHeader` (respeita o toggle global)
O eyebrow/titulo/descricao do topo de QUALQUER pagina admin tem que vir do componente compartilhado `AdminPageHeader` — que consome `useAdminPageHeaderVisibility` (layer core) e respeita o toggle GLOBAL de cabecalho (themes > "PAGE HEADERS": eyebrow/title/description / "Desativar tudo"). NUNCA renderizar eyebrow/titulo/descricao a mao (markup proprio) numa pagina, senao o "desativar" nao funciona naquela pagina.

- **Por que:** Aconteceu (2026-06-14): `/site/produtos` mostrava o cabecalho mesmo com o toggle desligado em themes, porque o `AdminPageHeader` resolvido por auto-import (`layers/tasks/...`) nao chamava `useAdminPageHeaderVisibility` (so o do core chamava). Resultado: drift — umas paginas respeitavam o toggle, outras nao. Os dois `AdminPageHeader` (core e tasks) agora chamam o composable; o ideal e consolidar num so.
- **Aplica quando:** Criar pagina/workspace admin nova OU revisar uma existente. Verificar: a pagina usa `AdminPageHeader` (nao markup proprio de header) e, ao desligar em themes > PAGE HEADERS, o cabecalho some. Auditar as paginas existentes contra esse padrao.

### Dropdown/popover/menu SEMPRE fecha no clique-fora e no Esc
Todo dropdown, popover, menu suspenso ou seletor aberto por clique DEVE fechar quando: (1) o usuario clica em QUALQUER lugar fora dele, (2) o usuario seleciona uma opcao/aperta um botao de dentro dele, (3) o usuario aperta `Esc`. Implementar com listener `pointerdown` no `document` que fecha se o alvo nao esta dentro do root do componente (`rootRef.contains(target)`), + listener de `keydown` para `Escape`, ambos removidos no `onBeforeUnmount`. Componentes prontos (UPopover/USelectMenu do Nuxt UI) ja fazem isso — a regra vale para dropdown FEITO A MAO (markup proprio + `open` ref).

- **Por que:** Aconteceu (CoreAccountSwitcher): o dropdown so fechava ao selecionar item ou clicar no trigger; clicar fora deixava ele aberto/preso. Dropdown que nao fecha no clique-fora atrapalha a navegacao e parece travado.
- **Aplica quando:** Criar OU revisar QUALQUER pagina/componente com dropdown/popover/menu feito a mao. Ao entrar numa pagina que tem dropdown, VERIFICAR esse comportamento (fecha fora/opcao/Esc) antes de considerar pronto.

### Config/dado faltando = aviso ACIONAVEL inline; editar de QUALQUER tela via API (sem caçar a tela de config)

Em producao, dados que o calculo usa VAO faltar (ex.: meta de ticket/PA da loja, meta por consultor, `store_type`) ate o pessoal se acostumar a preencher. A tela NUNCA pode mascarar isso nem obrigar o usuario a procurar a pagina especifica de edicao. Padrao obrigatorio:

1. **Transparencia primeiro — o numero nunca mente sobre a origem.** Onde o dado faltante muda o resultado, mostrar um aviso claro e honesto ali mesmo: "Sem meta de ticket/PA cadastrada — penalidade de qualidade desligada"; "Sem meta individual — meta da loja R$ X dividida igualmente entre N consultores"; "Loja sem `store_type` — usando padrao bairro". O usuario entende de onde veio o valor.
2. **Aviso CLICAVEL = editor inline na hora.** Se o usuario tem permissao de editar aquilo, o aviso abre um editor inline (popover/drawer/modal) NA PROPRIA tela, que grava via a MESMA API canonica do recurso. Proibido mandar o usuario "ir em Configuracoes > X" para corrigir um dado que o aviso ja apontou.
3. **Mesma acao em qualquer tela — componente/composable compartilhado.** O editor inline e UM componente + composable reutilizavel (ex.: `useGoalQuickEdit` + `QuickEditPopover`), usado identico em /operacao, /consultor, /ranking, multiloja, etc. O dado aparece em N telas → a acao de editar existe nas N telas, sem reimplementar por tela e sem divergir.
4. **Gate por permissao espelhando o back + re-hidrata.** So quem pode editar ve o aviso clicavel (os demais veem so o informativo). Apos salvar, re-hidratar do back (regra [[Nada hardcoded — toda informacao vem do banco]]). A FONTE continua unica (a API do recurso); muda so o PONTO DE ENTRADA da edicao, nunca a fonte.

- **Por que:** Aconteceu (2026-06-17, Perola Jardins): a loja estava sem meta de ticket/PA (penalidade de qualidade silenciosamente desligada) e sem meta por consultor (meta da loja dividida por igual entre os consultores, com uma conta renomeada inflando o divisor) — tudo isso mudava o calculo da comissao SEM nenhum aviso na tela, e a unica forma de corrigir era achar a tela de config certa. Aviso acionavel inline torna o gap visivel e corrigivel de onde o usuario ja esta: dinamico e facil, em vez de engessado.
- **Aplica quando:** QUALQUER tela que consome dado/config que pode faltar e cuja ausencia altera o resultado exibido. Preferir SEMPRE aviso acionavel inline a (a) mascarar o gap com um default silencioso ou (b) obrigar navegacao ate a tela de config.

---

## Backend

### Padrao de modulo Go
Cada modulo em `back/internal/modules/<nome>/` tem: `model.go` (tipos), `store_postgres.go` (persistencia), `service.go` (regras), `http.go` (handlers), `AGENT.md` (documentacao). Modulos plugaveis se registram via Module Registry quando `CORE_V2_ENABLED`.

- **Por que:** Consistencia entre modulos facilita onboarding e troca de agente.
- **Aplica quando:** Criar novo modulo backend.

### IDs como string, nunca uuid externo
Usar `string` para IDs no Go; nao importar pacote `uuid` externo. Casts e geracao ficam centralizados em `internal/platform/ids/`.

- **Por que:** Reduz dependencia externa e facilita refatoracao do esquema de IDs.
- **Aplica quando:** Qualquer struct nova com ID.

### Scan de campos NULL com `*string`
Para colunas nullable, declarar `*string` (ou `sql.NullString` se preferir) no Scan; nunca `string` puro.

- **Por que:** Evita panic em scan de NULL.
- **Aplica quando:** Implementar `store_postgres.go`.

### Permissoes vivem no banco (RBAC dinamico)
Nao hardcoded permission names em codigo Go. Permissoes vivem em `core.permissions` + `core.role_permissions`; service consulta via Module Registry.

- **Por que:** Permite agencia customizar role sem deploy.
- **Aplica quando:** Implementar checagem de permissao em handler ou service.

---

## Seguranca multi-tenant e otimizacao (PRIORIDADE MAXIMA)

> Detalhe + inventario + gaps: [docs/ENGINEERING_PRINCIPLES.md §10](docs/ENGINEERING_PRINCIPLES.md). Regra de ouro: **um usuario NUNCA ve dado de outra account/loja**; nenhum payload/parametro forjado contorna isso.

### Todo escopo (tenant/store/account) e validado contra o Principal — nunca confiado do client
`tenantId`/`storeId`/`accountId` vindo de query/body e SEMPRE filtro DENTRO do permitido, validado por `resolveTenantScope`/`resolveStoreScope`/membership (`core.account_users`). Pedido fora do escopo → `ErrForbidden`. Handler novo que aceita um desses IDs SEM validar = bug de seguranca.

- **Por que:** Sem a validacao no service, o client passa o id de outro tenant e vaza dado.
- **Aplica quando:** Qualquer handler/service que receba um id de escopo. Defesa em profundidade: a query do repo TAMBEM filtra por `tenant_id`.

### Erro uniforme: recurso fora do escopo retorna 404, nao 403
Nao revelar a existencia de recurso de outro tenant. Fora do escopo → `404 not_found` (nao `403`).

- **Por que:** `403` vs `404` diferentes vazam que o recurso existe (enumeration).

### Pedir so o necessario (otimizacao + UX de resposta imediata)
- Front NAO dispara request que a role nao consome (gatear fetch por permissao ANTES, espelhando o back). Sem 403 de ruido no bootstrap.
- Sem N+1: agregar por `WHERE id = ANY($1)`, nunca 1 query por item em loop.
- Lista grande → paginacao (cursor para as que crescem); projecao lean (so os campos que a tela usa), nao o objeto inteiro.
- Carregar primeiro o above-the-fold; detalhe (memberships, stores) so ao abrir o modal.

- **Por que:** Payload menor + menos round-trips = UI que responde na hora.
- **Aplica quando:** Qualquer endpoint de listagem ou fetch no boot/montagem de tela.

---

## Banco

### Migration idempotente (IF NOT EXISTS)
Toda migration usa `IF NOT EXISTS` / `CREATE OR REPLACE`. Numerar sequencialmente em `back/internal/platform/database/migrations/####_nome.sql`. Nunca renumerar migration ja aplicada.

- **Por que:** Migrations falhas no meio precisam poder ser reaplicadas sem dropar dados.
- **Aplica quando:** Criar migration nova.

### Schema-per-modulo + account_id em todas as tabelas tenant-scoped
Schemas: `core`, `queue`, `tasks`, `alerts`, `settings`, `roadmap`. Toda tabela tenant-scoped tem `account_id` NOT NULL com FK para `core.accounts`. Public schema pode ter VIEWS sobre tabelas dos schemas.

- **Por que:** Multi-tenancy com isolamento logico e queries por schema mais previsiveis.
- **Aplica quando:** Criar tabela nova.

### Mover tabela para schema: criar view publica
Quando mover tabela de `public.*` para `schema.*`, criar `CREATE OR REPLACE VIEW public.<tabela> AS SELECT * FROM schema.<tabela>` para manter compat com codigo legado.

- **Por que:** Evita quebrar queries antigas que ainda apontam para public.*.
- **Aplica quando:** Refactor de schema.
- **ANTES de transformar tabela-COM-DADOS em view sobre outra tabela:** comparar os DADOS (nao so as colunas) das duas. Se houver divergencia (ex.: `password_hash` diferente para o mesmo id), a view vai servir o lado errado e pode trancar usuarios. Reconciliar os dados primeiro (a fonte VIVA vence) e so entao trocar.

### NUNCA mudar senha / rodar seed que sobreescreve dados de usuario sem permissao explicita
NUNCA alterar `password_hash`, nem rodar migration/seed/bootstrap/view-swap que sobreescreva senhas, perfis ou outros dados de usuario existentes, sem permissao MUITO explicita do usuario para AQUELE comando especifico. Mesmo "autorizado a rodar a sequencia", isso NAO inclui sobrescrever senha.

- **Por que:** seed/bootstrap/troca de fonte que reescreve senha TRANCA o usuario para fora. Aconteceu em 2026-06-05: o view-swap `public.users`->`core.users` passou a servir o `password_hash` stale do `core.users` (congelado desde o seed 0101) em vez do hash vivo de `public.users`; o admin ficou sem login ("Email ou senha invalidos"). Restaurado do `users_backup` (`update core.users set password_hash = b.password_hash from users_backup b where b.id=c.id`).
- **Aplica quando:** QUALQUER comando que escreva em dados EXISTENTES (UPDATE/DELETE em users, seeds de senha `0018`/`0033`, bootstrap_owner, view-swap sobre tabela com dados). Operacao destrutiva de dados exige: (1) backup antes, (2) checagem de divergencia, (3) confirmacao explicita do que muda — e senha so com pedido direto.

---

## Qualidade de codigo — lint Go, SQL e TypeScript/Vue

O projeto roda `golangci-lint` (Go), linter de migrations SQL e ESLint (web) no pre-commit via Husky/lint-staged. Todo codigo gerado por agente JA deve respeitar as regras abaixo.

### Go — padroes obrigatorios

| Situacao | Errado | Certo |
|---|---|---|
| Permissao de diretorio (`os.MkdirAll`) | `0o755` | `0o750` |
| Permissao de arquivo (`os.WriteFile`) | `0o644` | `0o600` |
| Cadeia if/else-if com 3+ ramos booleanos | `if a {} else if b {} else if c {}` | `switch { case a: ... case b: ... default: ... }` |
| Switch em variavel simples | `if x == "a" {} else if x == "b" {}` | `switch x { case "a": ... case "b": ... }` |
| Conversao redundante de tipo | `http.HandlerFunc(h)` quando `h` ja e' `http.HandlerFunc` | `h` direto |
| Request em teste sem context | `httptest.NewRequest(method, path, nil)` | `httptest.NewRequestWithContext(context.Background(), method, path, nil)` |
| Codigo morto / tipo nao usado | declarar struct/func sem usar | apagar ou nao criar |
| Falso-positivo gosec verificado | — | `//nolint:gosec` na linha especifica |

Linters ativos: `gosec` (G301/G306 perms, G602 slice), `gocritic` (ifElseChain), `staticcheck` (QF1003), `unconvert`, `noctx`, `unused`. Configuracao: `back/.golangci.yml`.

- **Por que:** `golangci-lint` audita TODOS os arquivos do pacote quando qualquer arquivo dele e' staged — erros pre-existentes aparecem. Escrever certo na primeira vez evita fixup commits.

### SQL — schema sempre qualificado

Todo DDL dentro de `back/internal/platform/database/migrations/*.sql` deve qualificar o schema explicitamente:

| Situacao | Errado | Certo |
|---|---|---|
| Tabela em `public` | `drop table if exists user_store_roles` | `drop table if exists public.user_store_roles` |
| Tabela em schema proprio | `create table consultants (...)` | `create table queue.consultants (...)` |
| Referencia de FK | `references users(id)` | `references public.users(id)` |

O linter `scripts/dev/lint-migrations-staged.sh` bloqueia o commit se encontrar DDL sem schema qualificado.

- **Por que:** O PostgreSQL resolve nomes sem schema pelo `search_path` (que pode mudar); schema explicito garante execucao igual em qualquer ambiente.

### TypeScript/Vue — padroes obrigatorios

| Situacao | Errado | Certo |
|---|---|---|
| Regex com caracter de controle (null byte etc.) | `/ /g` puro | `// eslint-disable-next-line no-control-regex` na linha anterior |
| `console.log` em codigo de producao | `console.log(x)` | `console.warn(x)` ou `console.error(x)` |
| Arquivo com mais de 500 linhas | arquivo unico gigante | dividir em composables/componentes menores |
| Variavel declarada mas nao usada | `const x = ...` sem uso | apagar ou prefixar com `_` |
| `any` explicito | `param: any` | tipo especifico ou `unknown` |

Linters ativos: `no-control-regex`, `no-console` (permite warn/error), `max-lines` (500 linhas), `unused-imports/no-unused-vars`, `@typescript-eslint/no-explicit-any`. Configuracao: `web/eslint.config.*`.

---

## Linguagens

### Go 1.26
Backend usa Go 1.26. Aproveitar generics, `max`/`min` builtins, `slices`/`maps` stdlib.

- **Por que:** Versao alinhada com infra de CI e Docker.
- **Aplica quando:** Backend.

### Vue 3 + Nuxt 4 + Pinia
Frontend usa Vue 3 (Composition API + `<script setup>`), Nuxt 4 (com layers em `web/layers/*`), Pinia para state. Tipos TS sempre que possivel.

- **Por que:** Stack escolhida pelo time; layers permitem isolar dominios.
- **Aplica quando:** Frontend.

### TypeScript strict
Codigo TS deve passar em `vue-tsc --noEmit`. Evitar `any`. Preferir tipos explicitos em props e composables.

- **Por que:** Pega bug em build time, nao em prod.
- **Aplica quando:** Qualquer codigo TS/Vue.

---

## Deploy

### Deploy via registry (GHCR) — a VPS NUNCA builda
As imagens (Go api + Nuxt web) sao publicadas no GHCR (`ghcr.io/mikewade2k16/omni-{api,web}:<tag>`) e a VPS so faz `docker compose pull` + `up -d --no-build` — nunca `--build`. DOIS caminhos para a MESMA esteira de imagens: (1) **rapido, sem git** — `deploy-fast.ps1` builda na maquina LOCAL e da push (dia a dia; npm `deploy:fast`); (2) **completo/rastreavel** — `build-images.yml` builda no GitHub Actions com gate de testes + tag por SHA (release formal). Staging roda na mesma VPS no projeto `omni-staging` (volumes/subdominio proprios), sob demanda. Scripts: `scripts/deploy/{deploy-fast,deploy-pull,staging-up,staging-down,promote}.ps1`.

- **Por que:** o build do Nuxt pede 4GB de heap; numa VPS de ~6GB com prod no ar isso causa sobrecarga/OOM. O build fica fora da VPS (maquina local OU CI); a VPS so puxa a imagem pronta. `docker push/pull` e' incremental (so camadas que mudaram) — resolve o "deploy rapido sem git" sem o problema de tamanho/comando do git. Plano: [docs/deploy/REGISTRY_STAGING_DEPLOY_PLAN.md](docs/deploy/REGISTRY_STAGING_DEPLOY_PLAN.md).
- **Aplica quando:** qualquer deploy ou mudanca em script/workflow de deploy. NUNCA reintroduzir `up -d --build` na VPS como caminho padrao (nem buildar o Nuxt na VPS).

### VPS Hostinger com Caddy + Docker Compose
Deploy em VPS `85.31.62.33`, user `deploy`. Caddy reverse proxy em `/opt/omnichannel/Caddyfile`. Cada projeto roda em `/home/deploy/<projeto>` com `docker-compose.prod.yml`. Aliases por projeto na network proxy.

- **Por que:** Isolamento por projeto + um Caddy gerencia todos os dominios.
- **Aplica quando:** Deploy ou troubleshooting de prod.

### Feature flag em `.env.production` E `docker-compose.prod.yml`
Variaveis novas precisam de duas adicoes: `.env.production` (na VPS) E `docker-compose.prod.yml` na secao `environment`. Sem a segunda, o container nao recebe a variavel.

- **Por que:** Compose nao propaga automaticamente `.env` file inteiro; precisa de declaracao explicita.
- **Aplica quando:** Adicionar variavel de ambiente nova.

### Apos mudar upstream, restart Caddy (nao reload)
Caddy reload mantem cache do upstream antigo em alguns casos. Para garantir, fazer `docker restart omnichannel-mvp-caddy-1`.

- **Por que:** Sintoma classico: site continua mostrando versao antiga apos deploy.
- **Aplica quando:** Trocar upstream Caddy ou criar novo dominio.
 
---

## Padroes Gerais

### Oferecer paralelismo com agentes ao planejar
Sempre que um plano tiver 2+ partes independentes (front + back, testes + impl, U3 + U4), **oferecer ao usuario a opcao de rodar em paralelo com multiplos agentes** (ex.: Codex × Codex, Codex × Claude). Nao assumir sequencial por padrao.

- **Compensa:** trabalho mecanico/repetitivo, tarefas sem dependencia entre si, preservar contexto do agente principal em sessao longa.
- **Nao compensa:** keystones de design onde spec ≈ solucao (o prompt custa quase tanto quanto fazer direto); tarefa onde B depende do resultado de A.
- **Como oferecer:** ao final do plano, listar quais partes sao independentes, sugerir divisao de agentes e o tradeoff custo×velocidade.

- **Por que:** usuario pode querer velocidade (paralelo) ou economia (sequencial); sem a oferta ele nao tem a escolha.
- **Aplica quando:** qualquer plano com 3+ passos ou que envolva front + back + docs simultaneamente.

### Documentar antes de implementar
Antes de codar feature nao trivial: criar fase pending no `roadmap-data.ts` (status:'pending', tasks done:false), apresentar plano ao usuario, so depois codar.

- **Por que:** Evita retrabalho e mantem roadmap como fonte de verdade para o agente.
- **Aplica quando:** Tarefa com 3+ passos ou impacto em multiplas camadas.

### Atualizar AGENT.md ao alterar modulo
Toda mudanca em modulo backend (ou layer/area significativa do front) reflete no `AGENT.md` correspondente: novos endpoints, novas tabelas, novos contratos.

- **Por que:** AGENT.md e a fonte que outros agentes leem para entender o modulo.
- **Aplica quando:** PR que mexe em modulo.

### Sem Co-Authored-By Claude em commits
Commits nao devem ter `Co-Authored-By: Claude`. Atribuicao fica so com o desenvolvedor humano.

- **Por que:** Preferencia explicita do mantenedor.
- **Aplica quando:** Toda criacao de commit.

### NUNCA remover funcionalidade existente para resolver um problema — features coexistem
Ao corrigir um bug ou adicionar um comportamento, **NAO desligar/remover uma funcionalidade ja construida** como atalho. Se sao DUAS funcionalidades diferentes (ex.: abrir dropdown por **hover** E **fechar no clique-fora**), elas **TEM QUE COEXISTIR** — somam, nao se substituem. So e permitido remover/trocar quando: (a) uma nao pode existir sem quebrar a outra (mutuamente exclusivas tecnicamente), ou (b) e uma mudanca DELIBERADA de regra de negocio (ex.: trocar a formula de um calculo que antes era de um jeito e o time decidiu mudar). Nesses dois casos: **AVISAR e PERGUNTAR antes de remover** — nunca remover por conta propria.

- **Por que:** Aconteceu (DashboardHeader, 2026-06-15): ao adicionar "fechar dropdown no clique-fora", troquei a abertura por **hover** (CSS) por abertura so-no-clique — removi uma feature que o usuario usava para resolver outra. Eram duas coisas independentes que deviam coexistir (hover abre + clique-fora fecha).
- **Aplica quando:** QUALQUER correcao/refactor que toque comportamento existente. Antes de apagar/desligar/substituir algo que ja funcionava, confirmar que e mesmo inevitavel; se for, perguntar ao usuario primeiro.

### Validar local antes de qualquer coisa
Sempre rodar e testar local antes de propor commit ou deploy. UI changes precisam de browser test, nao so type-check.

- **Por que:** Type-check + test suite validam corretude de codigo, nao de feature.
- **Aplica quando:** Sempre.
