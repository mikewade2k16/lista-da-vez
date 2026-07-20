# OMNI-F14 — Refactor: pagar a dívida do port · **P1**

Plano canônico: `docs/omnichannel/PLANO_ATENDIMENTO.md` (§9.2 F14, §14, §15).
Anexo técnico do front: `docs/omnichannel/PLANO_PORT_OMNICHANNEL.md` · `SPECS_PORT_OMNICHANNEL.md`.
Contagem honesta da costura e dos verbatim: `OMNI-F1.md` (C1, C2). Ler `principios-engenharia` antes.

> ## LIBERADO PARA IMPLEMENTAÇÃO (2026-07-17, decisão do dono)
> A branch `refactor/multi-tenant-complete` fechou (**D-D**, canônico §2) e o congelamento que
> vigorava aqui **não existe mais**. A F14 segue **dependendo de F0–F13 verdes** — isso é *blocker*
> técnico, não congelamento.

> ## Régua de medição — o alvo é o módulo PORTADO, não o `web-reference/` cru
> Todo número de C1/C2/C3 mede os **67 verbatim + 5 repontados = 72 copiados** (canônico §9 F1; a
> conta está em `OMNI-F1.md` C1) — **não** o `web-reference/app/` inteiro. A **D-F** (canônico §2,
> 2026-07-17) tirou do port `OmnichannelAuditModule.vue` + `useOmnichannelAudit.ts`: **eles não
> existirão** em `web/` no dia da F14. Quem medir contra o `web-reference/` cru conta 2 arquivos
> fantasmas e erra todos os totais abaixo.
>
> Os números desta spec já estão **medidos no disco com o audit descontado** — mas a régua da C1
> vale para as três seções: **medir na hora, não herdar**. Entre esta spec e o `grep`/ESLint do dia,
> **o disco vence**.

---

## Objetivo

A dívida que a F1 assumiu **conscientemente** (verbatim, arquivos gigantes, adaptadores, fora de
layer) é paga aqui — e o `docs/LEGADO.md` esvazia dos itens do port. Nada de comportamento novo:
ao fim da F14 o inbox faz **exatamente** o que fazia no fim da F13, com o mesmo pixel, e o
`git diff` inteiro é estrutura.

**Esta fase não toca nenhum outro módulo.** O `omnichannel` é independente por construção
(canônico §4): não há convergência, integração nem interface cross-módulo neste escopo.

## Depende de / Bloqueia

| | |
|---|---|
| **Depende de** | **F0–F13 verdes.** Refatorar antes de a feature fechar é refatorar duas vezes |
| **Bloqueia** | Nada. É a última fase do plano |
| **Não bloqueia** | A migração dos segredos do calendário para o `platform/secretbox` (§14.5) tem alvo **"depois da F3"** — **não é da F14**, e o item **fica** no LEGADO.md |

---

## Entregas

| # | Entrega | Alvo |
|---|---|---|
| 1 | Remoção dos adaptadores de costura | `web/app/composables/useApi.ts` · `useAdminSession.ts` · `usePageBootstrapLoading.ts` · `web/app/stores/session-simulation.ts` — ver **C2** |
| 2 | Módulo vira layer | `web/app/**/omnichannel/**` → `web/layers/omnichannel/` + `web/nuxt.config.ts:21` — ver **C3** |
| 3 | Split dos arquivos acima do teto | inventário **medido na hora**, não a lista de 5 do roadmap — ver **C1** |
| 4 | Convergência de design system | 4 tokens indefinidos + 3 hex + `UModal` → `OmniEntityDrawer` — ver **C4** |
| 5 | `docs/LEGADO.md` sem os itens do port | itens 2, 3 e 4 do canônico §14 — ver **C5** |
| 6 | `AGENT.md` front + back fechados | `web/layers/omnichannel/AGENT.md` · `back/internal/modules/omnichannel/AGENT.md` |
| 7 | Roadmap + canônico sincronizados | `phases-part7.ts` · `PLANO_ATENDIMENTO.md` (feedback_three_docs_sync) |
| 8 | **Decisão** DELIVERED/READ registrada | **não é tarefa de código** — ver **C6** |

### Ordem de execução (decisão desta spec)

`C2 (adaptadores) → C3 (layer) → C1 (split) → C4 (DS) → C5/C6 (docs)`.

Racional: a remoção dos adaptadores **encolhe o grafo de imports** antes de mexer em paths (2
imports somem, as decisões de `ApiClientError`/`legacyRole` ficam resolvidas); o layer é **um
commit mecânico** de caminho, revisável, com o grafo já menor; o split entra por último **nos
arquivos já no destino final**, senão cada arquivo novo tem os imports reescritos duas vezes e o
diff do layer vira irrevisável. Se o dono preferir split primeiro, é defensável — muda **aqui**,
não no meio da execução.

---

## C1 — Split: o inventário é medido, não herdado

**A lista de 5 arquivos do roadmap está incompleta.** Medido no disco (`web-reference/app`, a
fonte do port), **14 arquivos do módulo passam de 450 linhas** e o 4º maior — `InboxChatMessageRow.vue`,
1.059 linhas — **não está na lista**. Ver *Divergências* 1 e 2.

**A régua real do linter** (`web/eslint.config.mjs:42`):
`'max-lines': ['warn', { max: 500, skipBlankLines: true, skipComments: true }]` — teto **500**
(não 450), contando **sem** linhas em branco e **sem** comentários, e como **warn** (não bloqueia).

| Arquivo (`web-reference/app/…`) | Bruto | Efetivo (aprox.) | Na lista do roadmap? |
|---|---|---|---|
| `composables/omnichannel/useOmnichannelInbox.ts` | 1467 | ~1328 | sim |
| `components/omnichannel/inbox/InboxConversationsSidebar.vue` | 1128 | ~956 | sim |
| `components/omnichannel/inbox/InboxChatPanel.vue` | 1110 | ~1019 | sim |
| **`components/omnichannel/inbox/InboxChatMessageRow.vue`** | **1059** | **~926** | **NÃO** |
| `composables/omnichannel/useOmnichannelInboxHistory.ts` | 774 | ~661 | sim |
| `composables/omnichannel/useOmnichannelAdmin.ts` | 764 | ~711 | sim |
| **`composables/omnichannel/useInboxChatMessageRendering.ts`** | **736** | **~614** | **NÃO** |
| **`composables/omnichannel/useInboxChatMediaActions.ts`** | **696** | **~585** | **NÃO** |
| **`components/omnichannel/OmnichannelInboxModule.vue`** | **591** | **~535** | **NÃO** |
| `useOmnichannelInboxRealtime.ts` · `useOmnichannelInboxShared.ts` · `useOmnichannelInboxOutboundPipeline.ts` · `useOmnichannelWhatsAppSession.ts` · `useInboxChatMessageIdentity.ts` | 552–454 | ~462–379 | não |

**São 9 acima do teto do linter e 14 acima da diretriz de ~450 da casa.** A coluna "efetivo" é
**aproximação** (grep de linhas não-brancas e não-comentário); **a autoridade é a saída do
ESLint na hora** — rodar antes de planejar o corte. `useOmnichannelInboxRealtime.ts` é
**reescrito na F5**: seu tamanho no dia da F14 é outro — medir, não copiar daqui.

**Protocolo — INCREMENTAL, um arquivo por vez** (lição do `useTasksPageContext`, 3.063 linhas,
cujo split "de uma vez" foi adiado por ser arriscado):

1. Um split = **um commit**. Extrair por **responsabilidade**, nunca por contagem de linhas.
2. Preferir **menos arquivos coesos** a muitos rasos (princípio de código limpo). O módulo já é
   `useX` + sub-composables injetados: seguir esse grão, não inventar outro.
3. **Smoke no browser depois de cada split** — não é typecheck: abrir `/omnichannel`, listar
   conversas, abrir uma, mandar mensagem, ver chegar ao vivo. Quebrou = reverte **aquele**
   commit, não a fase.
4. **Zero mudança de comportamento por split.** Se o corte "melhorou" algo, virou outra tarefa.

## C2 — Adaptadores: quantos morrem de verdade

O canônico §14.2/§9.2 fala em **6**. A F1 verificou no disco que são **5** (`web/app/stores/ui.ts`
**já existe** e serve). Verificado de novo aqui: `useUiStore` com `confirm(options) → Promise<{confirmed, value}>`,
store viva do Omni com outros consumidores. **Dos 5, 4 morrem e 1 muda de casa.** Ver *Divergências* 3.

| Costura | Destino na F14 | O que a remoção custa (medido) |
|---|---|---|
| `composables/useApi.ts` | **morre** | **4** sítios criam (`useOmnichannelAdmin.ts:35` · `useOmnichannelInbox.ts:51` · `useOmnichannelWhatsAppSession.ts:30` · `InboxChatPanel.vue:115`); **68 chamadas de `apiFetch` em 16 arquivos** recebem por injeção — ver abaixo |
| `composables/useAdminSession.ts` | **morre** | **4** sítios; `legacyRole` em **5 pontos de 2 arquivos** |
| `stores/session-simulation.ts` | **morre** | `useOmnichannelInbox.ts:50,53,57` · `useInboxChatMediaActions.ts:32,414` · `OmnichannelInboxModule.vue` (10+) |
| `composables/usePageBootstrapLoading.ts` | **MOVE** (não morre) | 65 linhas, 1 consumidor (`useOmnichannelInbox.ts:114`). **A casa não tem equivalente** em `web/app/composables/` — conferir antes de deletar; sem equivalente, apagar é **remover funcionalidade** (princípio 3) |
| `types/index.ts` (36 exports) | **MOVE** (não morre) | **47** imports `~/types`. É o **contrato de tipos do módulo**, não adaptador de mapeamento — vai para `web/layers/omnichannel/types/` |
| ~~`stores/ui.ts`~~ | **não existe** | **NÃO criar, NÃO sobrescrever, NÃO apagar.** É store da casa (F1 C2) |

**`useApi` morre; a indireção não.** `useApi().apiFetch` prefixa `/v1/omnichannel` e é **injetado**
em **16** arquivos. Trocar por `createApiRequest` em cada um dos **68** sítios espalharia o prefixo
pelo módulo inteiro — pior que a costura. O destino é o **padrão confirmado da casa** (76 arquivos usam
`createApiRequest`; molde: `web/layers/finance/composables/useFinancesConfigManager.ts:15,43,59`
com `FINANCE_CONFIG_API_BASE`):

```ts
// web/layers/omnichannel/composables/useOmnichannelApi.ts — client do MÓDULO, não adaptador
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
export const OMNICHANNEL_API_BASE = '/v1/omnichannel'
// apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)
```

O que morre é o **adaptador**: o arquivo que finge ser o `~/composables/useApi` do legado e
reimplementa classe de erro. A forma da função (`fn(path, opts)`) é compatível com
`createApiRequest`, então os **68** sítios **não mudam**. *Decisão desta spec* — se o dono quiser o
prefixo inline nos 68, muda aqui.

**`ApiClientError` — decisão aberta (pequena, mas real).** `web/app/utils/api-client.ts` exporta
`getApiErrorMessage` (`:37`) e `createApiRequest` (`:231`), e **nenhuma classe de erro**. O módulo
depende de `instanceof ApiClientError && statusCode === 404` em 3 pontos
(`useOmnichannelInboxHistory.ts:270,369,573`) — e o `instanceof` que dá **false silencioso**
quebra a paginação do histórico **sem erro visível** (F1 C3). Opções:

| # | Opção | Consequência |
|---|---|---|
| **A** *(recomendada)* | Adicionar `isApiNotFound(error)` a `web/app/utils/api-client.ts` e trocar os 3 `instanceof` por ele | **Aditivo** (não remove nada da casa), sem classe nova, e resolve para todo mundo. Padrão já existente para copiar: `web/app/stores/feedback.ts:285` (`statusCode === 404 \|\| status === 404 \|\| response?.status === 404`) |
| **B** | Manter `ApiClientError` como tipo do módulo, dentro do layer | Sobra um pedaço da costura vivo — a dívida não zera |

**`legacyRole` — a parte que é segurança, não faxina.** `legacyRole.value === "ADMIN"` gateia
**dois** computeds no módulo portado: `canManageTenant` (`useOmnichannelAdmin.ts:109`) e
`canManageChannel` (`useOmnichannelWhatsAppSession.ts:51`). **O terceiro gate era o audit
(`useOmnichannelAudit.ts:20`) e saiu com a D-F** — não procurar por ele. Sai o papel mapeado,
entram as permissões `omnichannel.*` do canônico §5.2 — **com `isPlatformAdmin || has(...)`**:
`has()` devolve **false** para o `platform_admin` e o gating sumiria justamente para quem
administra. Mapa: `canManageChannel` → `omnichannel.instances.manage`; `canManageTenant` →
`omnichannel.settings.manage`.

> **`omnichannel.audit.view` não some do módulo** — continua entre as 9 keys do canônico §5.2,
> declaradas pelo shell Go desde a F1. O que não existe é **tela** para ela gatear no front: a
> trilha de auditoria não foi portada. Não inventar tela aqui; se virar UI, é fase própria.

> **O 5º ponto de `legacyRole` não é `=== "ADMIN"`.** `useOmnichannelAdmin.ts:115`
> (`canViewOpsDashboard`) testa `legacyRole.value === "ADMIN" || legacyRole.value === "SUPERVISOR"`
> — some junto com a costura e **precisa de destino**. Mapa **a confirmar com o dono**: nenhuma das
> 9 keys do §5.2 é obviamente "ver o painel de operações". **Não afrouxar** enquanto não decide:
> quem não via, continua sem ver.

**`session-simulation` — troca de fonte de conta.** O legado usa `clientId` **numérico**
(`Number(sessionSimulation.effectiveClientId || …)`, `useOmnichannelInbox.ts:57`) e
`hasModule("atendimento")` (`OmnichannelInboxModule.vue:23`); no Omni a conta é **UUID** e o módulo
é `omnichannel` em `core.account_modules`. Fonte correta: **`useCoreAccountStore().activeAccountId`**
— nunca `auth.activeTenantId` direto (`project_account_source_divergence`; molde:
`web/layers/tasks/composables/useRealtimeSocket.ts:4`).

## C3 — Layer: o que quebra é `~`, e só em parte dele

`web/layers/finance/AGENT.md:48-53` documenta a regra: dentro de layer **`~` = `web/app`**, e o
layer referencia **o que é dele** por caminho relativo. Logo:

| Import do módulo | Quantos | Depois do move |
|---|---|---|
| `~/composables/omnichannel/*` | **84** | relativo (`./x`, `../composables/x`) — **quebra** se ficar `~` |
| `~/types` | **47** | relativo — **quebra** se ficar `~` |
| `~/components/omnichannel/inbox/*` | **3** | relativo — **quebra** se ficar `~` |
| `~/composables/useApi` · `usePageBootstrapLoading` | 2 | **somem** na C2 (ou viram relativos) |
| `~/stores/ui` (`OmnichannelWhatsAppSessionModal.vue:14`) | 1 | **fica `~`** — aponta para a casa e continua resolvendo |

São **134 linhas de import a reescrever** de **137**. **`sed` cego em `~/` quebra o único que
aponta para a casa (`~/stores/ui`)** — reescrever só os 3 prefixos do módulo.

> **`~/components/docs/*` saiu desta conta — e é a prova da D-F.** O único import dele em todo o
> `web-reference/app/` era `OmnichannelAuditModule.vue:4` (`ProjectDocsModule.vue`), e a **D-F**
> tirou esse arquivo do port: `~/components/docs/` **deixa de ser arrastado junto** (canônico §2,
> racional da D-F). Sem o audit, o módulo tem **um só** import que legitimamente fica `~`.
>
> A conta, contra o `web-reference/` cru: 141 `~/` imports − **4** carregados pelos 2 arquivos do
> audit (2× `~/composables/omnichannel/*`, 1× `~/types`, 1× `~/components/docs/*`) = **137**.

Checklist do move:

1. `web/nuxt.config.ts:21` — `extends: ['./layers/core', './layers/queue', './layers/tasks', './layers/finance']`
   ganha `'./layers/omnichannel'`. **Mudar `nuxt.config.ts` exige restart do dev** (config não é
   hot-reload) — ver Notas de Deploy.
2. `web/layers/omnichannel/nuxt.config.ts` = `export default defineNuxtConfig({})` (molde:
   `web/layers/finance/nuxt.config.ts`).
3. Estrutura espelhando o finance: `components/omnichannel/**` (preservar a subpasta `inbox/`) ·
   `composables/` · `pages/omnichannel/index.vue` · `types/`.
4. **A página é MOVE, não cópia.** `web/app/pages/omnichannel/index.vue` e a do layer resolvem o
   **mesmo path** e o app vence — o layer nunca carregaria e o debug é caro (mesma armadilha da
   F1, item 2).
5. **Nada a fazer no warmup:** `nuxt.config.ts:150-153` já globa `../layers/*/**/*.vue` e
   `../layers/*/**/*.ts`. Sair de `web/app` e entrar em `web/layers` mantém o módulo aquecido.
6. `routeRules` (`nuxt.config.ts:58` + o `/omnichannel/**` da F1) **não mudam** — o path é o mesmo.

> **Armadilha das stores de layer** (`project_web_autoimport_layer_stores`): store de layer **não é
> auto-importada** — exige import explícito, senão **500 no SSR**. Onde isso morde aqui:
> `OmnichannelInboxModule.vue:16` chama **`useSessionSimulationStore()` sem importar**, contando
> com o auto-import de `web/app/stores/`. A C2 **apaga essa store**, então o call-site some — e
> **o módulo não tem `defineStore` próprio** (grep: zero). Resultado: depois da F14 o módulo
> consome só store da casa por import explícito (`~/stores/ui`), que funciona do layer. **A regra
> continua valendo para qualquer store futura do módulo.**

## C4 — Design system: o problema não é o que parece

Medido no disco, **três fatos que encolhem a tarefa e um que a cria**:

- **`U*` não é componente do legado.** O módulo usa `@nuxt/ui` — o **mesmo** da casa
  (`web/package.json:26`, `^4.7.1`; a casa tem 224 `<UButton>`, 190 `<UIcon>`). Trocar `UButton` por
  outra coisa é inventar trabalho.
- **O módulo já usa os tokens da casa:** `var(--muted)` ×37, `var(--primary)` ×30, `var(--border)`
  ×30, `var(--surface)` ×15, `var(--radius-sm)` ×14 — nomes e formato (tripla RGB) idênticos aos de
  `web/app/assets/styles/omni-tokens.css`. **Zero** classe utilitária de cor fixa (`bg-red-500` etc.).
- **Hex hardcoded são 3, em 2 arquivos** (não uma varredura): `InboxChatMessageRow.vue:991`
  (`#ef4444` → `rgb(var(--danger))`), `:992` (`#53bdeb`, azul do "lido" do WhatsApp — **cor de
  marca**, candidata a token próprio, não a `--primary`) e `OmnichannelWhatsAppSessionModal.vue:354`
  (`background: #fff` → `rgb(var(--surface))`; **hoje não troca de tema**). Os `&#039;` de
  `useInboxChatUtilities.ts:66,73` são **entidade HTML, não cor** — não "corrigir".

**O que a tarefa realmente é: 4 tokens que não existem em lugar nenhum.**

| Token usado | Onde (exemplos) | Situação |
|---|---|---|
| `--warning` (9×) | `InboxChatFooterStatus.vue:196,227,318` · `InboxChatMessageRow.vue:750,752` · `InboxConversationsSidebar.vue:946,1014` | **indefinido** no Omni **e no `web-reference`** |
| `--error` | `InboxChatFooterStatus.vue:314,375` · `InboxConversationsSidebar.vue:913` | **indefinido**; a casa tem **`--danger`** |
| `--surface-3` | `InboxChatMessageRow.vue:789` | **indefinido**; a casa tem `--surface-2` |
| `--primary-foreground` | `InboxChatFooterStatus.vue:354` | **indefinido** |

`rgb(var(--warning))` com `--warning` indefinido é **declaração inválida**: a cor simplesmente não
aplica. **Já está quebrado na origem** — não é regressão do port. Consertar **muda o visual** em
relação ao legado, e por isso é F14 (a F1 é verbatim) e **exige olho no browser**, não só diff.

- `--error` → `--danger`; `--surface-3` → `--surface-2`; `--primary-foreground` → token real da casa.
- `--warning` **não existe na casa**: criar em `omni-tokens.css` é **aditivo** e seguro, mas tem de
  entrar nos **4 blocos de tema** — `:root` (`:1`), `.dark` (`:51`), `.theme-apple-blue` (`:69`),
  `.theme-liquidglass` (`:91`). Definir em um só = a cor some ao trocar de tema (é a lição de
  `project_page_header_visibility_per_theme_bug`).
- `--color-neutral-500` · `--color-neutral-900` · `--ui-border` · `--ui-text-muted` ·
  `--ui-text-highlighted`: **não estão** em `web/app/assets/styles/` — a hipótese é que venham do
  `@nuxt/ui` v4 em runtime. **Conferir no inspetor antes de mexer**; token que resolve não é bug.

**Troca de componente que sobra:** `UModal` (3 usos) → `OmniEntityDrawer`
(`web/app/components/ui/OmniEntityDrawer.vue`), onde couber. `OmniDataTable` vive em
`web/layers/tasks/components/omni/table/OmniDataTable.vue` e é **auto-importado entre layers** —
reuso sem import explícito e sem acoplamento de código (precedente: `finance/AGENT.md`,
"OmniMoneyInput é reaproveitado do layer tasks (não duplicar)"). **Confirmar no browser após a
troca:** bundle stale de container mostra CSS velho — conferir a versão **servida**, não o disco
(`project_hardcoded_colors_theming`).

## C5 — Docs (os 3 sincronizados)

| Doc | O que fazer |
|---|---|
| `docs/LEGADO.md` | **Remover** os itens do canônico §14: **2** (adaptadores), **3** (>450 linhas), **4** (fora de layer). O item **1** ("SEM BACKEND") já saiu na F2/F4. **O item 5 (segredos do calendário) FICA** — alvo "depois da F3", não é da F14. Hoje ele é a seção 8 do arquivo |
| `web/layers/omnichannel/AGENT.md` | Molde: `web/layers/finance/AGENT.md` + `web/layers/tasks/AGENT.md`. Se a F1 criou um AGENT.md de front em `web/app/…/omnichannel/`, ele **move**; se não existe, **nasce aqui**. Documentar: estrutura, a regra do `~`, o client `useOmnichannelApi`, tokens, e que o módulo **não tem store própria** |
| `back/internal/modules/omnichannel/AGENT.md` | Nasceu na F1 (shell) — aqui **fecha**: rotas finais, permissões, `messaging.*`, e o que ficou fora (§15) |
| `PLANO_ATENDIMENTO.md` + `phases-part7.ts` | F14 → `done`; §14 esvaziado dos itens 2–4; as divergências abaixo resolvidas no texto do canônico |

## C6 — DELIVERED/READ: decisão, não tarefa

**Não existe no legado** (canônico §15): `MessageStatus` é `"PENDING" | "SENT" | "FAILED"`
(`web-reference/app/types/index.ts:89`) — sem tracking de ACK. **Mas o front já está pronto para
renderizar**, verificado: `InboxChatMessageRow.vue:278-292` tem `case 'DELIVERED'` e `case 'READ'`
em `getStatusIcon(status: string)` (parâmetro alargado para `string`, então os ramos **compilam e
nunca executam**), e `:992` tem o azul do "lido". **Front pronto, backend nunca emite.**

Custo real, se o dono quiser: **backend** (o provider emitir ACK → persistir → `message.updated`)
+ **1 linha de tipo** no front. **A F14 não implementa** — é feature nova, fora do escopo do port.
A entrega é o **registro da decisão** (implementar → vira fase própria; não implementar → some do
LEGADO/roadmap e os ramos mortos ficam **documentados como intencionais**, não "removidos por
faxina" — princípio 3).

---

## Armadilhas / o que NÃO fazer

- **Não splitar tudo de uma vez.** É a lição do `useTasksPageContext` (3.063 linhas). Um arquivo,
  um commit, um smoke no browser. Typecheck verde **não é** smoke.
- **Não apagar `web/app/stores/ui.ts`.** Não é costura, é store da casa com outros consumidores.
- **Não apagar `usePageBootstrapLoading` sem equivalente na casa** — vira remoção de
  funcionalidade (princípio 3).
- **Não `sed` cego de `~/`** no move: `~/stores/ui` aponta para a casa e **continua certo**. É o
  **único** — `~/components/docs/*` saiu do módulo junto com o audit (D-F).
- **Não procurar o audit no `web/`.** `OmnichannelAuditModule.vue` e `useOmnichannelAudit.ts` **não
  foram portados** (D-F). Eles existem no `web-reference/`, não no destino: medir a costura contra o
  `web-reference/` cru infla toda contagem desta spec em 2 arquivos.
- **Não copiar a página para o layer** deixando a de `web/app/pages/` — o app vence e o layer nunca
  carrega.
- **Não usar `auth.activeTenantId` direto** ao matar a `session-simulation`; a fonte é
  `useCoreAccountStore().activeAccountId`. E **não setar `X-Account-Id` na mão**: o provider global
  já injeta (`account-id-bridge.client.ts` → `setApiAccountIdProvider`, `api-client.ts:213`). Duas
  fontes de conta = `project_account_source_divergence`.
- **Não trocar `legacyRole` por `has(...)` puro** — `platform_admin` tem `has()` = false e o módulo
  some para quem administra. Sempre `isPlatformAdmin || has(...)`.
- **Não "aproveitar" o refactor** para mudar comportamento, encurtar retry, mexer em contrato de
  rota ou reordenar evento de realtime. Refactor com feature junto é bug sem dono.
- **Não confiar em "ESLint limpo" como prova** — `max-lines` é **warn** e não falha nada. Ver
  Verificável 1.
- **Não tocar em outro módulo.** Sem convergência, sem interface cross-módulo, sem depreciar tabela
  alheia. Independência é decisão do dono (canônico §4).
- **Não migrar os segredos do calendário aqui** (§14.5, alvo "depois da F3").

## Segurança

F14 não cria rota nem lê dado novo — o risco é **regressão de isolamento** ao trocar a costura:

| Item | Regra |
|---|---|
| Escopo | `account_id` **sempre** do Principal, **nunca** do body. O front **não** passa a mandar conta no corpo ao perder a `session-simulation` — o header vem do provider global |
| Fonte da conta | `useCoreAccountStore().activeAccountId` (v2). Errar a fonte = REST/WS na conta errada |
| Gate de feature | `legacyRole` → permissões `omnichannel.*` **sem afrouxar**: quem não tinha, continua sem. `isPlatformAdmin || has(...)` |
| Fora de escopo | **404, nunca 403** — o back não muda na F14; o front não pode passar a distinguir "existe mas não vejo" |
| Bug do escopo de instância | Corrigido na F7 (o ternário inoperante do legado). **Continua corrigido** — não reintroduzir ao mexer em `useOmnichannelAdmin` |
| Repositório | Segue filtrando por conta (defesa em profundidade). Nada do back é relaxado por esta fase |

## Verificável

Um humano prova, no browser e no terminal:

1. **Teto de linhas — com a régua certa.** `cd web && npx eslint layers/omnichannel | grep max-lines`
   → **sem saída**. (`--max-warnings=0` também serve, mas falha por *qualquer* warn: o que a fase
   promete é **zero `max-lines`**, o resto é ruído de outra fase.)
2. **Costura morta.** `ls web/app/composables/useApi.ts web/app/composables/useAdminSession.ts
   web/app/stores/session-simulation.ts` → **não existem**;
   `grep -rn "useAdminSession\|useSessionSimulationStore\|ApiClientError" web/app web/layers` →
   **zero** ocorrências. E **`web/app/stores/ui.ts` continua existindo** (prova que a store da casa
   não foi levada junto).
3. **Layer.** `ls web/app/components/omnichannel web/app/composables/omnichannel` → **não existem**;
   `web/nuxt.config.ts` tem `'./layers/omnichannel'` no `extends`; `npm run dev` sobe **sem erro de
   resolução de import e sem 500 de SSR** (é o que prova que nenhum `~` do módulo ficou para trás e
   que nenhuma store de layer virou auto-import).
4. **Tokens resolvem.** Inspetor no rodapé do chat (`InboxChatFooterStatus`) e no balão de falha:
   a cor de aviso/erro **aparece** e o *computed style* é uma cor real, não vazio. Trocar entre os
   **4 temas** (`:root`, `.dark`, `.theme-apple-blue`, `.theme-liquidglass`): **nada** fica
   branco-no-branco. O modal de sessão do WhatsApp **acompanha o tema** (era `#fff` fixo).
5. **Smoke completo da F13 — o critério que manda.** Tudo que funcionava continua: listar, abrir,
   paginar histórico (scroll para cima **até o fim**, que é o caminho do `instanceof` removido),
   enviar texto/foto/áudio, reagir, encaminhar, apagar p/ mim e p/ todos, status, atribuir,
   participantes de grupo, sticker/GIF, QR e conexão de número, mensagem ao vivo em duas abas.
   **Nada some, nada muda de aparência** (fora os 4 tokens da C4).
6. **Gate.** Usuário sem `omnichannel.instances.manage` **não vê** a ação de canal; com, vê.
   `platform_admin` **vê tudo** (a prova de que o `isPlatformAdmin ||` está lá). Conta sem o módulo
   → `/omnichannel` cai em `/perfil`.
7. **Docs.** `docs/LEGADO.md` **sem** os itens 2, 3 e 4 do §14 e **com** o item dos segredos do
   calendário. `web/layers/omnichannel/AGENT.md` e `back/internal/modules/omnichannel/AGENT.md`
   descrevem o estado final. Roadmap com F14 `done`.
8. **ACK.** A decisão do dono está **escrita** (no canônico §15 e no roadmap), qualquer que seja.

## Notas de Deploy

**Sem migration. Sem env var nova. Sem container novo. Sem dependência nova**
(não rodar `npm install` no host Windows: quebra o `npm ci` do container — `project_web_npm_lockfile_cross_platform`).

| # | Passo | Detalhe |
|---|---|---|
| 1 | Dev do web | `npm run dev` (compose `up --build --watch`). **Obrigatório reiniciar**, não confiar em HMR: mudou `web/nuxt.config.ts` (`extends`) e o grafo de arquivos inteiro mudou de lugar. `up -d web` sozinho **congela o código** (`project_dev_slow_docker_windows`) |
| 2 | API | **Nada.** A F14 só toca `.md` dentro de `back/`. Se alguém encostar em `.go`, aí sim `docker compose up -d --build api`. **`--no-cache` não se aplica** — não há migration nova |
| 3 | Build de produção | Só depois do "aprovei" do dono (`feedback_no_npm_until_approved`) |

---

## Divergências com o canônico (registradas, não decididas por conta própria)

| # | Ponto | O canônico / roadmap diz | Esta spec faz | Por quê |
|---|---|---|---|---|
| 1 | Lista do split | 5 arquivos (1467, 1128, 1110, 774, 764) | Inventário **medido na hora**; a lista fixa vira exemplo | **14** arquivos passam de 450 no disco e **`InboxChatMessageRow.vue` (1.059)** — o 4º maior — **não está na lista**. Executar a lista literal deixa 4+ arquivos gigantes e o Verificável falha |
| 2 | "Nenhum arquivo do módulo >450 linhas" (§9.2 F14) | teto 450, por arquivo | Teto **500**, sem brancos nem comentários, **medido pelo ESLint** | `web/eslint.config.mjs:42` é a régua que existe. 450 é a diretriz da casa (prosa), 500 é o que o linter cobra. **São 9 acima de 500, 14 acima de 450** — dizer qual das duas o Verificável exige é do dono |
| 3 | "os 6 adaptadores de costura" (§14.2, §9.2 F14) | 6 | **4 morrem** (`useApi`, `useAdminSession`, `session-simulation`, e o `ApiClientError` junto do 1º), **2 mudam de casa** (`types/index.ts`, `usePageBootstrapLoading`), **1 nunca existiu** (`stores/ui.ts`) | A F1 já corrigiu 6 → 5 no disco (C2). Destes, `types/index.ts` é o **contrato de tipos** do módulo e `usePageBootstrapLoading` **não tem equivalente na casa** — apagá-los é remover funcionalidade (princípio 3), não pagar dívida |
| 4 | "ESLint sem max-lines" (verificável do roadmap) | como se fosse gate | Comando explícito de prova (Verificável 1) | `max-lines` é **`warn`**: nunca falhou nada e nunca falhará. "ESLint limpo" hoje é verdade **mesmo com** os 14 arquivos gigantes — o critério, como escrito, se auto-aprova |
