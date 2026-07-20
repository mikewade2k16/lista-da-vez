# OMNI-F1 — Front verbatim + costura

- **Prioridade:** P0
- **Plano canônico:** [`docs/omnichannel/PLANO_ATENDIMENTO.md`](../PLANO_ATENDIMENTO.md) (§9 F1, §11, §14)
- **Contratos verbatim — NÃO duplicados aqui:** [`SPECS_PORT_OMNICHANNEL.md`](../SPECS_PORT_OMNICHANNEL.md) F1.1–F1.6 · [`PLANO_PORT_OMNICHANNEL.md`](../PLANO_PORT_OMNICHANNEL.md) §5
- Ler a skill `principios-engenharia` antes de começar.

> ## LIBERADO PARA IMPLEMENTAÇÃO (2026-07-17, decisão do dono)
>
> A branch `refactor/multi-tenant-complete` fechou e o dono **liberou a implementação em
> 2026-07-17**. O congelamento que bloqueava esta fase **não existe mais**.

---

## Objetivo

`/omnichannel` abre logado e **é** o inbox do legado, pixel a pixel. Nenhum dado carrega
(o Go do módulo ainda não tem rotas) e um badge **"SEM BACKEND"** diz isso na cara do admin.
Ao fechar a F1, o front inteiro está no repo e a única coisa que falta é o backend.

## Depende de / Bloqueia

| | Fases |
|---|---|
| **Depende de** | Nenhuma. Roda em paralelo com F2 e F3 (canônico §9). **Nada bloqueia** — o congelamento saiu em 2026-07-17 (decisão do dono). |
| **Bloqueia** | F2 (schema + leitura), F5 (realtime reescreve `useOmnichannelInboxRealtime.ts`), F6, F7, F12 |

## Entregas

O **como** de cada item está no `SPECS_PORT_OMNICHANNEL.md` — aqui vai só o alvo e o delta.

| # | Entrega | Alvo no disco | Detalhe |
|---|---|---|---|
| 1 | Copiar os verbatim | `web/app/composables/omnichannel/` · `web/app/components/omnichannel/` · `web/app/pages/omnichannel/index.vue` | SPECS_PORT **F1.1**. Ver contagem real abaixo |
| 2 | Os **5** arquivos de costura | `web/app/composables/` · `web/app/stores/` · `web/app/types/` | SPECS_PORT **F1.2** — mas são **5, não 6** (ver Contratos) |
| 3 | Os 5 repontados | `web/app/composables/omnichannel/` | SPECS_PORT **F1.3**. Só a URL; comportamento intacto |
| 4 | Registro no painel | 5 pontos de front + 1 de Go | SPECS_PORT **F1.4**. Ver armadilha do gate |
| 5 | **Shell Go do módulo** | `back/internal/modules/omnichannel/module.go` | Zero rotas. Sem ele o gate de módulo fecha a página (ver Contratos) |
| 6 | Limpar o demo | `web/app/pages/omnichannel.vue` + `web/app/utils/demo-pages.ts:22` | SPECS_PORT **F1.5**. **No mesmo commit** do item 1 |
| 7 | Badge "SEM BACKEND" | página `/omnichannel`, visível **só para admin** | SPECS_PORT **F1.6** + canônico §14.1 |
| 8 | `docs/LEGADO.md` | itens 1, 2, 3 e 5 do canônico §14 | Costura = 5 adaptadores, alvo F14 |

---

## Contratos — o delta da fusão (tudo verificado no disco)

### C1 · D4 **DECIDIDA: o código morto fica de FORA** — e a contagem honesta dos arquivos

**D4 — decisão do dono, 2026-07-17: `OmnichannelAuditModule.vue` + `useOmnichannelAudit.ts`
NÃO são copiados.** Deixou de ser decisão pendente; deixou de ser blocker da F1.

> **Racional (registrado para ninguém re-decidir):** os dois **nunca renderizam — nem no legado**,
> porque as páginas que os chamariam redirecionam para fora. Não é remover funcionalidade
> (princípio 3): é **não importar código inalcançável**. Bônus: `useOmnichannelAudit.ts` era a
> **única dependência** de `~/components/docs/ProjectDocsModule.vue` — com o D4 fora, esse arraste
> some junto.

A conta, em uma linha: 78 no disco (50 composables + 23 componentes + 5 pages) − **4 redirects**
(SPECS_PORT F1.1 manda não copiar) − **2 do D4** = **72 copiados** − 5 repontados = **67 byte a byte**.

**Use 67/72 — número final, cravado.** O "73 verbatim + 5 repontados" do canônico §9 conta os
redirects que a própria F1.1 manda não copiar: divergência de aritmética, não de escopo.
Os componentes copiados ficam em `components/omnichannel/inbox/**` (20) + **2** na raiz
(`OmnichannelInboxLoading.vue`, `OmnichannelInboxModule.vue` — o `OmnichannelAuditModule.vue` era
o 3º e saiu pelo D4); **preservar a subpasta `inbox/`**, os imports do legado dependem dela.

### C2 · A costura são **5** arquivos, não 6 — `stores/ui.ts` JÁ EXISTE

**`web/app/stores/ui.ts` já existe no Omni** e é compatível: exporta `useUiStore` com
`confirm(options) → Promise<{confirmed, value}>` — exatamente o que o único consumidor do
módulo usa (`OmnichannelWhatsAppSessionModal.vue:14` importa `~/stores/ui`, `:25` faz
`useUiStore()`, chama `ui.confirm({...})`).

> **NÃO criar, NÃO sobrescrever, NÃO "adaptar".** É store viva do Omni com outros consumidores;
> sobrescrever = remover funcionalidade existente (princípio 3). Custo dessa costura: **zero**.

| Costura | Estado no disco | Ação |
|---|---|---|
| `composables/useApi.ts` | livre | criar |
| `composables/useAdminSession.ts` | livre | criar |
| `composables/usePageBootstrapLoading.ts` | livre | criar (1 uso: `useOmnichannelInbox.ts:26`) |
| `stores/session-simulation.ts` | livre | criar |
| `types/index.ts` | **o diretório `web/app/types/` não existe** | criar dir + barrel |
| ~~`stores/ui.ts`~~ | **JÁ EXISTE e serve** | **nada** |

### C3 · `ApiClientError` — o Omni não tem, a costura define

`web/app/utils/api-client.ts` exporta `createApiRequest` (`:231`) mas **nenhuma classe de
erro** — usa `$fetch`, que estoura `FetchError` do ofetch. O módulo depende do `instanceof`:
`useOmnichannelInboxHistory.ts:3` importa `ApiClientError` de `~/composables/useApi` e testa
`error instanceof ApiClientError && error.statusCode === 404` (`:270`, `:369`, `:573`). Se o
`apiFetch` deixar vazar o `FetchError` cru, o `instanceof` dá **false silencioso** e a
paginação de histórico quebra sem erro visível.

Superfície obrigatória (idêntica ao legado):

```ts
export class ApiClientError extends Error {
  statusCode: number   // default 500 quando ausente
  data: unknown        // default null
  // this.name = "ApiClientError"
}
```

`useApi().apiFetch` **tem que** capturar a falha do `createApiRequest` e re-lançar como
`ApiClientError`, mapeando `statusCode` do status HTTP e `data` do corpo do erro.

### C4 · Superfícies mínimas das costuras (confirmadas por uso real)

Implementar **só o que o módulo consome** — costura é adaptador, não reimplementação.

| Costura | Membros realmente usados | Call-sites |
|---|---|---|
| `useAdminSession()` | `user` · `coreUser` · `token` · `coreToken` · `legacyRole` · `tenantSlug` · `logout` · `syncSessionFromToken` | `useOmnichannelInbox.ts:49` · `useOmnichannelAdmin.ts:34` · `useOmnichannelWhatsAppSession.ts:29` · `InboxChatPanel.vue:114` (`useOmnichannelAudit.ts:9` **não conta** — saiu pelo D4, C1) |
| `useSessionSimulationStore()` | `canSimulate` · `clientId` · `effectiveClientId` · `clientOptions` · `hasModule` · `requestHeaders` · `setClientId` | `useOmnichannelInbox.ts` · `useInboxChatMediaActions.ts` · `OmnichannelInboxModule.vue` |
| `useApi()` | `apiFetch` + `ApiClientError` | 6 arquivos |
| `types/index.ts` | 36 exports | **48 arquivos** — é a costura de maior fan-in; errar um tipo quebra meio módulo |

O `useAdminSession` do legado devolve ~20 membros. O módulo usa **8**. Portar os 8.

### C5 · Vocabulário de `status` do front — 3 valores, e a projeção agora cobre os 3

`web-reference/app/types/index.ts:91`:

```ts
export type ConversationStatus = "OPEN" | "PENDING" | "CLOSED";
```

**`PENDING` não é código morto.** É filtro e é ação, renderizados:

| Onde | O quê |
|---|---|
| `useOmnichannelInbox.ts:134` | filtro `{ label: "Pendentes", value: "PENDING" }` |
| `useOmnichannelInbox.ts` (`statusActionItems`) | **ação** `{ label: "Pendente", value: "PENDING" }` — o usuário *grava* PENDING |
| `InboxConversationsSidebar.vue:316,328,437,453` · `InboxDetailsSidebar.vue:92,104` · `useInboxChatPresentation.ts:48,60` | badge/rótulo |

A F1 registrou o gap (a projeção do canônico §7.3 mapeava `state → status` só para **OPEN** e
**CLOSED**, e nenhum `state` produzia `PENDING`) e delegou a decisão. **Já foi decidido.**

> **DECIDIDO — D-E, opção A (2026-07-17, decisão do dono):** `pending` vira o **7º `state`** da
> máquina (`new`, `ai_active`, `routing`, `queued`, `human_active`, `pending`, `closed`), com o
> **12º evento `human.pending`** (`PATCH /conversations/{id}/status` → `PENDING`). A projeção do
> canônico **§7.2/§7.3** ganha a linha `pending → PENDING`.
>
> **Consequência para a F1: nenhuma — e é esse o ponto.** O front continua **verbatim**, que é
> exatamente o que a opção A preserva: **a ação "Pendente" da UI fica**, o filtro "Pendentes"
> passa a devolver conversa de verdade e o botão grava um `state` que a máquina reconhece.
> Não é mais lacuna de ninguém. O **risco 4 do canônico (§12)** deixa de se materializar.
>
> Quem implementa a máquina é a **F8** (matriz **7 × 12 = 84 pares**); quem nasce com o `CHECK`
> de **7 valores** em `conversations.state` é a **F2** — a F8 **não faz `ALTER`**.
>
> **O candidato `queued → PENDING` está descartado com evidência** (não re-propor): `queued` é
> produzido pelo **motor**, então mapeá-lo trocaria "filtro sempre vazio" por "filtro sempre
> cheio". `PENDING` é **rótulo manual do operador** ("parei nesta, estou esperando algo"), sem
> produtor automático e sem limpeza automática — é **ortogonal** ao roteamento.

### C6 · Multi-provider (D-A) × front verbatim — o delta é **negativo**

O front **não conhece `provider`**. `WhatsAppInstanceRecord` (`types/index.ts:39-56`) tem
`instanceName`, `displayName`, `isDefault`, `userScopePolicy`, `assignedUserIds` e um
`hasEvolutionApiKey?: boolean` — que **nenhum arquivo do módulo lê** (grep: zero call-sites).

Regra da F1:

- **NÃO** adicionar `provider`, `state`, `department_id`, `queue_id`, `extracted_fields` nem
  `capabilities` ao `types/index.ts`. O barrel é **reexport verbatim dos 36 tipos do legado**.
- **NÃO** "preparar" a UI de instâncias para multi-provider: tela de números por provider é
  **F10**; degradação por `Capabilities()` é **F10/F11**.
- `hasEvolutionApiKey` entra no barrel como está (opcional e morto). A F2 pode não emitir.

Multi-provider **não muda uma linha do front na F1**. Quem estiver editando um dos 67 verbatim
por causa da D-A está fazendo a fase errada.

### C7 · Ícones — dois vocabulários diferentes (o canônico conflaciona)

| Arquivo | Campo | Vocabulário | Valor para o omnichannel |
|---|---|---|---|
| `web/layers/queue/nav.config.ts:11` | `icon` | **chave do `NAV_ICON_MAP`** (`useDashboardNav.ts:44`; `messages: MessagesSquare` está na `:61`) | `messages` — **já está lá, correto** |
| `web/app/utils/workspaces.ts` | `icon` | **ligature do Material Icons Round** — `DashboardWorkspaceNav.vue:85` renderiza `<span class="material-icons-round">{{ workspace.icon }}</span>` | uma ligature real (`forum`, `chat`, `message`) — **`messages` NÃO é ligature válida** |

O canônico §11 e a SPECS_PORT F1.4 item 2 dizem que o `icon` do `workspaces.ts` é chave do
`NAV_ICON_MAP`. **Não é** — vizinhos no disco: `task_alt`, `calendar_month`, `pending_actions`.
Usar `messages` ali renderiza o texto quebrado. Escolher a ligature e confirmar no browser.

---

## Armadilhas / o que NÃO fazer

1. **O gate de módulo fecha a página inteira — inclusive para o `platform_admin`.**
   `module-enabled.global.ts:104` faz `if (!enabledModules.has(guard.moduleId)) navigateTo('/perfil')`,
   e o comentário `:97-103` é explícito: **view-as gateia o `platform_admin` também**; só
   `account.platformView` (switcher "Plataforma (dev)") passa direto.
   Adicionar `{ prefix: '/omnichannel', moduleId: 'omnichannel' }` **sem** o módulo existir no
   Go ⇒ `SyncCatalog` nunca cria a linha em `core.modules` ⇒ nenhuma conta tem `omnichannel` em
   `enabledModules` ⇒ **`/omnichannel` cai em `/perfil` e o Verificável da F1 falha.**
   **É por isso que a entrega 5 (shell Go) é obrigatória na F1** — e é o que a SPECS_PORT F1.4
   item 6 já mandava. O shell implementa `modules.Module`
   (`back/internal/platform/modules/module.go:30`): `ID()`, `Metadata()`, `Permissions()`,
   `RoleTemplates()`, `Build(deps) (Handle, error)`, com `RegisterRoutes` **no-op** (zero rotas
   = requests 404 = o Verificável). Molde: `back/internal/modules/tools/module.go`.
   `Permissions()` = as 9 keys do canônico §5.2; `RoleTemplates()` = `attendant`/`supervisor`/`manager`.
   `EnableAllModulesOnAgencyAccounts` (`catalog_postgres.go:147`) auto-habilita na conta-agência
   no boot — é o que faz a página abrir sem ninguém clicar nada.
2. **Conflito de rota:** `web/app/pages/omnichannel.vue` (demo atual) e
   `web/app/pages/omnichannel/index.vue` (o port) resolvem para **o mesmo path**. Apagar o
   placeholder **no mesmo commit** em que a página nova entra — senão o Nuxt resolve o errado
   e o debug é caro.
3. **`definePageMeta`:** o legado usa `layout: 'admin'` (não existe aqui). Usar
   `{ layout: 'dashboard', workspaceId: 'omnichannel' }`. O demo atual usa `workspaceId: ''`
   (nunca-gated) — **não herdar isso**.
4. **Não rodar Prettier/ESLint `--fix` nos 67.** Verbatim é byte a byte. `max-lines` vai
   acusar (o módulo chega a 1.467 linhas) — **é esperado**, está registrado no canônico §14.3,
   alvo F14. O teto de ~450 linhas vale para **código novo** (o shell Go, as 5 costuras), não
   para o port.
5. **Não copiar em layer.** Dentro de layer, `~` resolve para `web/app` e os imports dos 67
   quebram (`web/layers/finance/AGENT.md:48-53`). Vai em `web/app/` mesmo. Layer é F14.
6. **`useApi` NÃO seta `X-Account-Id` na mão.** O provider global já injeta
   (`web/app/plugins/account-id-bridge.client.ts` → `setApiAccountIdProvider`, `api-client.ts:213`).
   Setar de novo = duas fontes de conta = o bug de `project_account_source_divergence`.
7. **Auto-imports que parecem bug e não são:** `OmnichannelInboxLoading.vue` usa `USkeleton`
   **sem bloco `<script>`**; `useOmnichannelInboxRealtime.ts:42` usa `ref()` **sem importar de
   `vue`**. Não "consertar" — é auto-import do Nuxt.
8. **Não implementar realtime aqui.** `useOmnichannelInboxRealtime.ts` na F1 é **só repontar a
   URL**; a reescrita sobre `useRealtimeSocket` é **F5**.
9. **Não portar o bug do filtro de instância** (`whatsapp-instances.ts:681-683` do legado: o
   ternário devolve o mesmo nos dois ramos, então todo usuário vê todas as instâncias). É
   backend (F2), mas se aparecer na costura, portar **corrigido** — é isolamento (princípio 2).

## Segurança

A F1 não expõe rota Go nova nem lê dado — mas as regras já valem para o que ela registra:

| Regra | Como se aplica na F1 |
|---|---|
| `account_id` **sempre** do Principal, nunca do body | A costura não inventa header de conta: `X-Account-Id` vem do provider global. `tenantSlug` é **exibição**, nunca escopo |
| Repositório filtra por conta também; fora de escopo → **404, nunca 403** | Sem repositório nem rota na F1. **A F2 nasce com essas regras** — não relaxar lá |
| Módulo desabilitado | Gate de rota no front + `moduleGatingRules` no Go (F2). O shell da F1 registra o módulo, então o gate **resolve** em vez de falhar aberto |
| Permissões | 9 keys `omnichannel.*` (canônico §5.2) declaradas no shell. `InvalidPermissionKeys` (`rbac_repository.go:385`) já valida chave contra módulo habilitado via JOIN — de graça |
| **`platform_admin` no front** | `has()` devolve **false** para ele. Todo gating de menu/seção precisa de `isPlatformAdmin || has(...)` — senão o módulo some justamente para quem administra (canônico §5.2) |

## Verificável

Um humano prova a F1 assim, no browser, logado:

1. `/omnichannel` **abre** (não redireciona para `/perfil`) na conta-agência, e o layout é o
   inbox do legado: sidebar de conversas, painel de chat, sidebar de detalhes.
2. **Badge "SEM BACKEND" visível** para admin na própria página.
3. **Console: requests 404** (`/v1/omnichannel/conversations` etc.) — é o esperado, não há rotas.
4. O item **Omnichannel aparece no menu** (não mais `hidden: true`), com selo `beta`.
5. Trocar para uma **conta-cliente sem o módulo** no switcher ⇒ `/omnichannel` redireciona para
   `/perfil` e o item some do menu. Habilitar via `PUT /v1/admin/accounts/{id}/modules`
   (`core/admin_http.go:31`) ⇒ volta a abrir. **Prova o gate nos dois sentidos.**
6. Banco: `select id from core.modules where id = 'omnichannel'` devolve linha; e
   `select count(*) from core.permissions where key like 'omnichannel.%'` devolve **9** —
   prova que o `SyncCatalog` rodou.
7. `npm run dev` sobe **sem erro de resolução de import** (é o que prova que as 5 costuras e o
   barrel de 36 tipos estão completos). ESLint acusando `max-lines` nos arquivos do port é
   **esperado**.
8. `docs/LEGADO.md` lista: front sem backend, as **5** costuras, os arquivos >450 linhas e o
   módulo fora de layer.

## Notas de Deploy

| # | Item | Detalhe |
|---|---|---|
| 1 | **Migration** | **Nenhuma.** O schema `messaging.*` (a partir de **0200** — conferir o disco: há dois `0197`, a última é `0199`) é da **F2** |
| 2 | **Rebuild da API** | **Obrigatório** — a F1 toca `back/` (o shell do módulo): `docker compose up -d --build api`. Sem migration nova, `--no-cache` não é necessário |
| 3 | Build do web | Rebuild de dev normal ao mudar o front |
| 4 | Env var | Nenhuma. `OMNI_SECRETS_KEY` é da **F3** |
| 5 | Container novo | Nenhum |
| 6 | Ordem | shell Go → `up -d --build api` (SyncCatalog registra módulo + 9 permissões no boot) → build web → conferir `/omnichannel` |

**AGENT.md:** `back/internal/modules/omnichannel/AGENT.md` **nasce nesta fase**, junto com o
shell (canônico §11: não existe módulo omnichannel no disco hoje, nada a sincronizar antes).
Documentar: o módulo tem zero rotas na F1 por decisão, e o porquê do shell (o gate).
