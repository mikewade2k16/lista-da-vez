# AGENTS — web/app/components/meta-ads

## Escopo

Estas instrucoes valem para `web/app/components/meta-ads`. Regras herdadas:
[@AGENT_RULES.md](/c:/Users/Mike/Documents/Projects/fila-atendimento/AGENT_RULES.md) +
[@docs/ENGINEERING_PRINCIPLES.md](/c:/Users/Mike/Documents/Projects/fila-atendimento/docs/ENGINEERING_PRINCIPLES.md).
Doc canonico do modulo:
[docs/meta-ads/PLANO_INTEGRACAO_META_ADS.md](/c:/Users/Mike/Documents/Projects/fila-atendimento/docs/meta-ads/PLANO_INTEGRACAO_META_ADS.md).

## Responsabilidade

UI do workspace `meta_ads` (Meta Ads no painel Omni). MVP: conectar com System User
token, sincronizar e exibir KPIs + 1 grafico + tabela de campanhas. Fase MA3: chat do
assistente MCP (texto → cria/edita/pausa campanhas via runner headless). Esta pasta so
renderiza UI e consome a store; nao faz fetch direto, nao define tipos, nao mexe em
wiring/permissao.

## Arvore de componentes

- **`MetaAdsWorkspace.vue`** — orquestrador e **container de rolagem** da pagina
  (`flex: 1; min-height: 0; overflow-y: auto`, exigido pelo layout `dashboard` que e
  `overflow: hidden`). Chama `store.init()` no `onMounted` e, num `watch` de
  `store.connected` (`immediate: true`), `store.loadAssistant()` quando a conexao confirma
  (o `connected` so vira true depois do init async). Renderiza o cabecalho com
  seletor de periodo (`store.setRange`) e botao "Sincronizar" (`store.sync()`, desabilitado
  enquanto `store.syncing`), o bloco `.meta-ads__error` (de `store.error`), o
  `MetaAdsConnectionCard` (sempre) e, quando `store.connected`, os demais cards
  (AccountPicker → Overview → **AssistantCard** → ReportChart → CampaignTable).
- **`MetaAdsConnectionCard.vue`** — status da conexao; quando desconectado, textarea para
  colar o System User token + botao "Conectar" (`store.saveConnection(token)`, spinner via
  `store.connecting`). O campo e **limpo apos a tentativa** — o token nunca e ecoado de volta;
  ha nota admin de que e guardado cifrado. Quando conectado, mostra nome/business id/expiracao
  - "Desconectar" (`store.deleteConnection()`) + linha "Assistente" (de `store.assistantHealth`:
    `ok` → "Assistente pronto" em success; senao o `detail` como hint em muted).
- **`MetaAdsAssistantCard.vue`** — chat do assistente MCP (fase MA3). Header "Assistente" +
  badge de saude derivado de `store.assistantHealth`: `ok` → Online; `!ok && claudeAuth` →
  Desconfigurado; `!ok && !claudeAuth` → Offline (com hint "Runner do assistente nao esta
  rodando — veja meta-ads-assistant/README" + o `detail` do health); `null` → Verificando.
  Lista rolavel de mensagens (`store.assistantMessages`): bolha do usuario a direita
  (`.ma-assistant__msg--user`), do assistente a esquerda (`--assistant`); cada mensagem do
  assistente renderiza `actions` como chips (tool em mono + summary; `status` ok = token
  success, error = token danger). Auto-scroll pro fim a cada mensagem nova. Estado vazio
  explica o que da pra pedir e os guardrails (confirmacao no chat, campanhas nascem
  PAUSADAS). Erros em `.ma-assistant__error` (de `store.assistantError`). Input: textarea
  auto-grow (max 140px) + botao "Enviar"; **Enter envia, Shift+Enter quebra linha**;
  desabilitado durante `store.assistantSending`; em falha o texto volta pro campo.
  **Latencia 30-120s**: enquanto envia aparece uma bolha "pensando... acoes na Meta podem
  levar 1-2 minutos" (spinner local; o POST usa `skipLoadingIndicator` na store, entao a
  barra de loading global NAO acende).
- **`MetaAdsAccountPicker.vue`** — controle segmentado das `store.adAccounts`; clique chama
  `store.selectAdAccount(id)`. Mostra a moeda de cada conta.
- **`MetaAdsOverviewCard.vue`** — tiles de KPI de `store.kpis` (investimento, impressoes,
  cliques, CTR, CPC, conversoes). Investimento/CPC formatados em moeda
  (`store.selectedAdAccount.currency`, fallback `BRL`); CTR em `%`. Skeleton quando
  `store.pending` sem dados; estado vazio sem KPIs.
- **`MetaAdsReportChart.vue`** — grafico de linha/area de `store.insights` (x=data, series
  investimento + CTR). Estado vazio quando nao ha insights. Ver "Grafico" abaixo.
- **`MetaAdsCampaignTable.vue`** — tabela read-only de `store.campaigns` (nome, objetivo,
  badge de status, orcamento diario/total). Estado vazio orientativo. As acoes
  (editar/pausar) sao da fase **Plataforma** — a estrutura de colunas ja deixa espaco para
  a coluna de acoes (comentario no template), mas nenhum botao e construido agora.

## Dependencia de store (contrato congelado)

`useMetaAdsStore` de [`~/stores/meta-ads`](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/stores/meta-ads.ts)
(setup-style Pinia, criada por outro agente). Os componentes consomem EXATAMENTE estes nomes:

- Estado: `connection`, `adAccounts`, `selectedAdAccountId`, `overview`, `campaigns`,
  `insights`, `range`, `pending`, `connecting`, `syncing`, `error`, e do assistente:
  `assistantMessages`, `assistantSending`, `assistantError`, `assistantHealth`.
- Computeds: `connected`, `kpis`, `selectedAdAccount`.
- Actions: `init`, `loadAdAccounts`, `selectAdAccount`, `loadOverview`, `loadCampaigns`,
  `loadInsights`, `saveConnection`, `deleteConnection`, `sync`, `setRange`, e do assistente:
  `loadAssistant` (historico + health), `loadAssistantHealth`, `sendAssistant(text) →
Promise<boolean>` (true = enviado; false = falha, o card restaura o rascunho).

Tipos vivem em
[`~/types/meta-ads`](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/types/meta-ads.ts)
(`MetaAdsConnection`, `MetaAdsAdAccount`, `MetaAdsCampaign`, `MetaAdsInsightPoint`,
`MetaAdsOverviewKpis`, `MetaAdsOverview`, `MetaAdsAssistantAction`,
`MetaAdsAssistantMessage`, `MetaAdsAssistantHealth`) — importar de la, nunca redefinir aqui.

## Assistente MCP — contrato CONGELADO com o backend (fase MA)

- `GET /v1/meta-ads/assistant/messages?limit=50` → array puro de mensagens
  (oldest→newest): `{ id, role: 'user'|'assistant', content, actions: [{ tool, summary,
status: 'ok'|'error' }], createdAt }`.
- `POST /v1/meta-ads/assistant/messages` body `{ message, adAccountId }` →
  `{ messages: [...], syncTriggered }` (a msg do usuario persistida + a resposta). A store
  ecoa a msg do usuario localmente na hora (`local-*`) e troca pelas persistidas quando o
  POST volta; se `syncTriggered`, recarrega overview+campanhas+insights. Erros: 503
  `assistant_not_configured` (runner ausente), 502 `assistant_error` — mapeados para
  mensagens amigaveis em `assistantError`.
- `GET /v1/meta-ads/assistant/health` → `{ ok, claudeAuth, detail }` (200 sempre).
- **Latencia**: a resposta do POST leva **30-120s** (Claude headless + MCP oficial da Meta).
  O POST passa `skipLoadingIndicator: true` no api-client (sem barra global); o feedback e
  local (`assistantSending` + bolha "pensando...").
- **Guardrails (MA4, lembrar na UI)**: toda acao de ESCRITA exige confirmacao explicita no
  chat antes de executar; campanha criada por IA **nasce PAUSADA** (regra do MCP oficial —
  ativacao e manual). O empty state do card ja comunica os dois.

O `range` default da store e `last_30d`; o seletor do workspace oferece
`last_7d`/`last_14d`/`last_30d`/`last_90d` (espelhar com o backend de insights ao evoluir).

## Grafico — abordagem escolhida (SSR-safe)

`vue3-apexcharts` (+ `apexcharts`) adicionados em `web/package.json` (`install` roda no
deploy do web, feito pelo integrador — esta pasta nao roda install). **Abordagem escolhida:**
`<ClientOnly>` + `defineAsyncComponent(() => import('vue3-apexcharts'))` dentro de
`MetaAdsReportChart.vue`. **NAO** ha plugin Nuxt (`apexcharts.client.ts`) — o import dinamico
client-only ja evita o break de SSR do ApexCharts e mantem a lib fora do bundle inicial.
As cores do grafico sao lidas dos tokens do design system em runtime no cliente
(`getComputedStyle` de `--primary`/`--success`/`--muted`/`--border`), com fallback hex apenas
como ultimo recurso quando `window` nao existe — zero cor de marca cravada na UI.

## Regras de estilo (seguidas)

- Tokens do design system somente: `rgb(var(--primary))`, `rgb(var(--primary) / 0.15)`,
  `rgb(var(--primary-600))`, `var(--text-main)`, `var(--text-muted)`, `var(--line-soft)`,
  `rgb(var(--surface))`, `rgb(var(--surface-2))`, `rgb(var(--success))`, `rgb(var(--danger))`,
  `rgb(var(--ring))`, `rgb(var(--border))`, `var(--radius-card)`, `var(--radius-soft)`,
  `var(--shadow-card)`. Zero hex de marca (o unico `rgb(255 255 255)` e o thumb/texto sobre
  gradiente primario, mesmo padrao dos cards de `automation`).
- Classes BEM-like (`.meta-ads__x`, `.ma-connection__y--active`, etc.), sem utility/inline.
- `<script setup lang="ts">` em todos; props/tipos explicitos; sem `any`; <= 500 linhas/arquivo.
- Sem emojis.

## Nao pertence a esta pasta (outro agente)

Store, pagina (`web/app/pages/meta-ads.vue`), tipos, composables e os 4 lugares de wiring
(`workspaces.ts`, `permissions.ts`, `nav.config.ts`, `module-enabled.global.ts`).
