# AGENTS

## Escopo

Estas instruções valem para `web/app/components/operation/`.

## Gravacao experimental do atendimento

- Handoff detalhado do bloco atual e do proximo passo com Whisper:
  `docs/operacao/GRAVACAO_TRANSCRICOES_HANDOFF_2026-07-24.md`.
- O gate global continua em `core.platform_settings` pela feature
  `attendanceAudioRecording`; somente o admin da plataforma pode altera-lo.
- Quando o backend confirma um novo atendimento, `OperationQueueColumns` inicia
  `useAttendanceAudioRecordingStore` e vincula a sessao local ao `serviceId`,
  `storeId` e consultor retornados pelo snapshot.
- O `MediaRecorder` emite blocos a cada 5 segundos. Cada bloco e persistido no
  IndexedDB `omni-attendance-audio-v1`, separado dos demais atendimentos. Ao
  recarregar a pagina, uma sessao que ainda estava `recording` vira
  `interrupted`, preservando os blocos ja gravados.
- Cada bloco tambem e enviado de forma idempotente ao backend; o PostgreSQL
  guarda metadados e o binario permanece no storage privado configurado por
  `ATTENDANCE_AUDIO_DIR`. Ao parar, o backend consolida os blocos recebidos.
- `OperationAudioRecordingStatus` atua somente como controller de ciclo de vida.
  O microfone vermelho pulsante pertence à linha do atendimento em
  `OperationActiveServiceCard`; a operação atualiza a lista de gravações ativas
  em polling curto para também sinalizar capturas iniciadas em outros terminais.
- A rota `/transcricoes` agrupa os registros autoritativos por loja e consultor,
  entrega o audio por endpoint autenticado e solicita a transcricao duravel no
  Whisper local. A pagina fica dentro do shell visual da Fila e faz polling curto
  para receber novas gravacoes e atualizacoes do Whisper sem recarregar.
- Na pagina de transcricoes, cada loja e um bloco recolhivel e os atendimentos
  aparecem em cards compactos num grid. O card mostra somente resumo/estado;
  audio amplo, relatorio e transcricao completa ficam em `OmniEntityDrawer`.
- Os selects de Loja e Consultor continuam no topo como filtros compactos. A
  organizacao recolhivel por loja complementa esses filtros e nunca autoriza
  remove-los sem pedido explicito.
- Loja, Consultor e Configurar compartilham a mesma linha de controles no
  desktop; Configurar usa `margin-left: auto` para permanecer no extremo direito.
  Todo controle com icone e texto nesta pagina preserva um `gap` visivel.
- Componentes desta pagina importam `AppPanelButton` explicitamente de
  `components/ui`; deixar o nome sem resolver faz o Vue renderizar um componente
  anonimo sem semantica, cursor ou estados reais de botao.
- O botao `Configurar` usa o mesmo `AppPanelButton` ghost do cabecalho
  Omnichannel e abre o drawer de configuracao no canto direito.
- A previa quase ao vivo chega apos a primeira janela de aproximadamente 25
  segundos. O frontend identifica `transcriptLive`, mas nao confunde essa previa
  com a transcricao definitiva feita sobre o audio completo ao encerrar.
- A configuracao de transcricoes usa `OmniEntityDrawer`, reutiliza
  `ConfigAiCredentials` para o cofre global da conta e separa Whisper de
  resumo/prompt em abas. Nenhuma chave e digitada ou devolvida pela tela Queue.
- Quando ainda nao existe configuracao persistida, o drawer prioriza e seleciona
  a primeira credencial OpenAI da conta; Gemini continua disponivel como opcao.
- O rodape de salvar permanece fixo no drawer e a UI distingue rascunho
  "Nao salvo" de configuracao ativa. No workspace agregado, o store replica a
  configuracao para as accounts operacionais visiveis; a API revalida cada uma.
- Existe uma sessao de gravacao independente por `serviceId`. O navegador
  compartilha a captura do microfone entre varios `MediaRecorder`, permitindo
  gravacoes simultaneas no mesmo terminal; cada sessao possui fila de chunks,
  persistencia e encerramento proprios.

## Responsabilidade dos componentes

Este diretório cuida da renderização visual da operação, incluindo:

- Workspace principal de operação
- Cards e estado dos consultores
- Queue visível
- Alertas operacionais em diversos formatos
- Modais e diálogos operacionais

## Anel de meta no avatar da fila (OperationConsultantAvatarRing)

`OperationConsultantAvatarRing.vue` (novo) envolve o avatar de CADA consultor com um anel de progresso da meta mensal, em DOIS lugares: a coluna "Lista da vez" (cards WAITING em `OperationQueueColumns.vue`) e a coluna "Em atendimento" (`OperationActiveServiceCard.vue`, avatar do `primaryService`). O MESMO componente e reusado nos dois (tamanho/cor/grossura controlados num lugar so); `goalStats` chega tanto na `waitingList` quanto nos `activeServices` do snapshot/overview.

**Props:**

- `initials: string` — iniciais exibidas no avatar.
- `color?: string` — cor de fundo do avatar (`--avatar-accent`). Reusa a classe global `.queue-card__avatar`, preservando o visual atual.
- `goalStats?: GoalStats | null` — vem de `person.goalStats` no snapshot do back. Pode ser `null`/`undefined` (ate o rebuild da api) sem quebrar.

**Contrato `GoalStats`** (alinhado com o snapshot do back, definido localmente no componente):

```ts
{ monthlyGoal: number; soldValue: number; remainingToGoal: number; progress: number; hasGoal: boolean } | null
```

**Comportamento:**

- Anel SVG (`stroke-dasharray`, mesma tecnica do gauge de `ConsultantPlayerCard.vue`), arco = `clamp(progress, 0, 100)`.
- Cor do anel = VERDE solido (`rgb(var(--success))`) quando ha meta; sem gradiente. (Era gradiente vermelho->amarelo->verde, mas o stop `--primary` e azul no design system, entao virou verde puro a pedido do usuario.)
- Sem dado (`goalStats` null OU `hasGoal === false`): anel neutro (tier `none`, `--muted`), sem inventar %.
- Popover no hover E no foco (acessibilidade); fecha no `mouseleave`, `blur` e `Esc`. Usa `Teleport` para o `body` com posicao calculada via `getBoundingClientRect` (reposiciona em scroll/resize) para nao ser cortado pelo overflow do board. Conteudo = lista de stats (Meta / Atingido (com %) / Falta), ou linha "Sem meta cadastrada".

**Propagacao de `goalStats`:** `OperationWorkspace.vue` repassa em `mapIntegratedWaitingItem`, `mapIntegratedActiveItem` e `mapScopedActiveItem` (modo integrado/all) e via spread `...item` em `buildOperableStoreState` + `servicesGroupedByConsultant` (modo loja unica). O anel aparece na fila E no "Em atendimento" (avatar do `primaryService`); a faixa de consultores do rodape NAO usa o anel.

## Coluna lateral da operacao (OperationSidePanel)

`OperationSidePanel.vue` e a 3a coluna do board (`.queue-grid`), renderizada dentro de `OperationQueueColumns.vue` depois das colunas "Lista da vez" e "Em atendimento". Dois blocos: **Comunicados** (topo) e **Omni Chat** (rodape).

- Comunicados vem de `queue.communications` por
  `GET /v1/operations/communications?publishedOnly=true&storeId=...`.
- A lista respeita conta, lojas-alvo, publicacao, vigencia e ordem, aceita varios
  cards e atualiza a cada 30 segundos. Nao existe fallback hardcoded.
- A gestao fica na secao Comunicados do hub `/campanhas`; `/comunicados`
  redireciona para essa secao por compatibilidade. Ela cria, edita, publica e
  exclui logicamente os registros, e o editor usa `OmniEntityDrawer`.
- Leitura usa `workspace.operacao.view`; acoes administrativas usam
  `queue.communications.manage`.
- **Configurar o Omni Chat (so platform_admin):** o botao `tune` abre
  `OperationOmniChatConfigDrawer.vue` no mesmo padrao lateral do Calendario. O
  drawer controla status, prompt soberano, credencial global da conta,
  provider/modelo, temperatura e memoria. A persistencia usa
  `GET/PUT /v1/omni-chat/config`; uma unica configuracao vale para todas as lojas
  e usuarios da account ativa. `ConfigAiCredentials` e o mesmo cofre global
  reutilizado pelo Omnichannel e pelas transcricoes; nenhuma chave crua entra no
  componente. O antigo editor inline nao e mais renderizado.
- Em visao de agencia, a account ativa do topo pode ser diferente da dona das
  lojas. `OperationOverviewStore.accountId` e `StoreView.tenantId` resolvem a
  account operacional e ela e enviada explicitamente no `X-Account-Id` do
  config/chat. Assim o admin da Crow configura a conta Perola, e os usuarios da
  Perola leem exatamente a mesma linha. A lista/gestao de chaves continua no
  cofre global da account ativa da agencia; o backend permite a credencial
  compartilhada somente dentro da mesma organizacao.
- **Largura das colunas:** as 3 larguras estao em `web/app/assets/styles/layout.css` no `.queue-grid` (`--queue-grid-left-column`, `--queue-grid-right-column`, `--queue-grid-side-column`) — ajuste num lugar so (fr ou px). No mobile o grid colapsa para 1 coluna.

## Skeleton de carregamento (Fase 9 — apply-operacao)

`OperationSkeleton.vue` (novo) e o estado de loading visual da pagina `pages/operacao/index.vue`.

**Objetivo:** "responde na hora" — ao abrir `/operacao` o skeleton aparece imediatamente (sem tela vazia) ENQUANTO o realtime/snapshot conecta, e e descartado quando os dados chegam.

**Props:**

- `scopeMode: string` (`single` | `all`) — apenas para a mensagem de leitor de tela (texto descritivo do que esta sincronizando).

**Onde entra/sai (estado loading):**

- A pagina renderiza `<OperationSkeleton>` no ramo `v-else-if="!isRemoteRosterReady"`, substituindo o antigo bloco textual `loading-state` "Carregando operacao...".
- Some assim que `isRemoteRosterReady` vira `true` (snapshot confiavel via `_operationSnapshotFetchedAt` no modo single, ou `overview`/`!overviewPending` no modo all). O ramo de erro (`pageErrorMessage`) e o conteudo real (`OperationWorkspace`) ficam intactos.

**Regra:** e ADITIVO. Nao toca realtime (`useOperationsRealtime`), faixa de consultores (`OperationConsultantStrip`) nem o roster — apenas pinta o placeholder do layout (scopebar + 2 colunas de fila + faixa de avatares) usando `CoreSkeleton` (variantes `block`/`card`/`avatar`/`text`) e design tokens.

## Fechamento de atendimento (Fase 7)

`OperationFinishModal.vue` agora funciona como orquestrador fino do wizard de encerramento.

Estrutura atual:

- `OperationWorkspace.vue` — lazy-load do `OperationFinishModal.vue` via `defineAsyncComponent()` + `Suspense`, renderizando o chunk apenas quando `finishModalServiceId` estiver ativo.
- `OperationFinishModal.vue` — instancia stores, chama `useFinishModalController()` e renderiza o fluxo.
- `finish/useFinishModalController.js` — concentra estado reativo, watchers, busca remota de produtos, persistencia de rascunho e submissao.
- `finish/FinishStepOutcome.vue` — passo visual do desfecho inicial (`reserva`, `compra`, `nao-compra`) e codigo de compra.
- `finish/FinishStepProduct.vue` — produtos vistos/fechados, justificativas do passo 1 e CTA de avancar.
- `finish/FinishStepClient.vue` — dados do cliente, profissao, motivo da visita e origem.
- `finish/FinishStepNotes.vue` — fila furada, motivo de perda, observacoes, resumo final e CTA de concluir.

Regras de manutencao:

- preservar `data-testid`, classes e textos visiveis do fluxo de fechamento;
- subcomponentes em `finish/` nao acessam store diretamente;
- qualquer regra nova de negocio do wizard deve entrar primeiro no controller;
- qualquer alteracao comportamental no modal precisa ser validada contra os cards da operacao, porque o fluxo operacional e espelhado visualmente no board.

## Arquitetura de alertas (novo em Fase 6)

### AlertDisplayHost.vue (novo)

Componente roteador que orquestra todos os tipos de display de alerta.

**Props:**

- `storeId: string` — identifica a loja para filtrar alertas

**Comportamento:**

- Consulta `alertsStore.activeAlertsForStore(storeId)`
- Agrupa alertas por `displayKind`
- Renderiza cada grupo com o componente correto:
  - `OperationAlertBanner` para `banner`
  - `AlertDisplayCornerPopup` para `corner_popup`
  - `AlertDisplayCenterModal` para `center_modal`
  - `AlertDisplayFullscreen` para `fullscreen`
  - Toast system (não este componente) para `toast`
  - Card badges (não este componente) para `card_badge`

**Uso:**
Substitui a referência direta a `OperationAlertBanner` no `pages/operacao/index.vue`.

### OperationAlertBanner.vue (refatorado em Fase 6)

Componente de banner persistente no topo da operação.

**Props (novo):**

- `alerts: Array<Record<string, any>>` — array de alertas a exibir

**Comportamento:**

- Renderiza cada alerta como um banner empilhado
- Usa `alert.colorTheme` para determinar a cor (6 variantes)
- Renderiza `alert.titleTemplate` com substituição de variáveis
- Para cada alerta, renderiza buttons para cada item em `alert.responseOptions`
- Ao clicar um botão, chama `respondToAlert(alertId, optionValue)`

### AlertDisplayCornerPopup.vue (novo em Fase 6)

Popups flutuantes no canto inferior direito, não-bloqueantes.

**Props:**

- `alerts: Array<Record<string, any>>` — array de alertas para exibir

**Comportamento:**

- Cada alerta é um card empilhado no canto inferior direito
- Anima entrada via slideIn (300ms)
- Mostra apenas alertas não dismissidos
- Ao clicar, chama `alertsStore.respondToAlert()`

### AlertDisplayCenterModal.vue (novo em Fase 6)

Modal centralizado, blocking, para alertas importantes.

**Props:**

- `alerts: Array<Record<string, any>>` — mostra apenas o primeiro alerta

**Comportamento:**

- Renderiza overlay com backdrop
- Modal centralizado com barra colorida no topo
- Exibe `titleTemplate` e `bodyTemplate`
- Renderiza `responseOptions` como botões primários

### AlertDisplayFullscreen.vue (novo em Fase 6)

Display mais agressivo: tela inteira com gradiente de fundo.

**Props:**

- `alerts: Array<Record<string, any>>` — mostra apenas o primeiro

**Comportamento:**

- Ocupa tela inteira (`position: fixed; inset: 0`)
- Fundo gradiente intenso
- Título XL com emoji de alerta (⚠️)
- Renderiza `responseOptions` como botões GRANDES
- SEMPRE `isMandatory` (não fecha sem responder)

## Integração com operacao/index.vue

**Antes:**

```vue
<OperationAlertBanner v-if="bannerStoreId" :store-id="bannerStoreId" />
```

**Depois:**

```vue
<AlertDisplayHost v-if="bannerStoreId" :store-id="bannerStoreId" />
```

## Filtro de loja ("Todas as lojas") — mora no nav, nao na pagina

Para economizar espaco vertical, o seletor de loja do modo "Todas as lojas"
**nao** fica mais no corpo da operacao: foi movido para o nav
(`components/dashboard/DashboardWorkspaceNav.vue`, area `workspace-nav-context`),
**antes** das pills de resumo. O nav shell renderiza em todo `/operacao/*`
(incluindo as rotas filhas `/operacao/clientes` e `/operacao/usuarios`), entao o
seletor e gateado pelo workspace ATIVO exato (`activeWorkspace === 'operacao'`),
nao pelo prefixo do path — so existe na propria pagina de operacao. Tambem so
aparece quando `auth.canUseAllStores` (quem enxerga 2+ lojas).

- **Estado compartilhado:** `integratedStoreId` agora e um `ref` no
  `stores/operations.ts` (com `setIntegratedStoreId`). Foi pra la porque e
  escrito no nav (layout) e lido na pagina (`pages/operacao/index.vue`) — ramos
  diferentes da arvore, sem relacao pai/filho, entao prop/emit nao serve.
  - O nav escreve via `setIntegratedStoreId`.
  - A pagina le via `storeToRefs(operationsStore)` e reage (carrega o snapshot
    operavel da loja filtrada) com os watchers ja existentes.
  - `OperationWorkspace` continua recebendo `integratedStoreId`/`stores`/`scopeMode`
    por prop (usados em `displayState`, `resolveStoreMeta`, `operableStoreId`).
- **`OperationScopeBar` virou campanha-only.** Perdeu o filtro, o `emit`
  `integrated-store-change` e os props `stores`/`integratedStoreId`/`scopeMode`;
  hoje so destaca campanha comercial ativa (`shouldRender = showCampaign`). Segue
  renderizado na pagina, recebendo apenas `:state`. (O nome "ScopeBar" ficou por
  heranca; o escopo real agora e o nav.)
- **Pagina:** banner de alerta (`AlertDisplayHost`) e a barra de campanha sao
  filhos diretos do `workspace-host`. A margem inferior (`0.5rem`) e aplicada so
  no elemento que de fato renderiza (`:deep(.operation-alert-banner-stack)` e a
  `section` da campanha, ambos com `v-if`), entao nao sobra espaco morto quando
  nao ha alerta nem campanha.

## Cores suportadas

- `amber`, `red`, `blue`, `green`, `purple`, `slate`
- Cada componente implementa mapeamento tema → cor CSS

## Variáveis de template

- `{consultant}` → `alert.consultantName` ou "Consultor"
- `{elapsed}` → minutos desde `lastTriggeredAt`
- `{threshold}` → valor do threshold da regra

## Toast system (não renderizado aqui)

Alertas com `displayKind === 'toast'` são controlados por `useContextRealtime.ts`:

- Filtram por `displayKind === 'toast'`
- Aparecem como notificações leves
- Auto-dismiss em 6 segundos

## Permissões

- Alertas respeitam autorização do backend
- Frontend confia na filtragem feita por `alertsStore.activeAlertsForStore(storeId)`
- `OperationConsultantStrip` lista os consultores a partir de `state.roster`. Esse roster vem de `/v1/consultants` (endpoint de GESTAO, restrito a `consultor.view`/`settings.view`); papeis operadores sem essa permissao (ex.: `consultant`) NAO buscam esse endpoint, entao o roster cai para a projecao ENXUTA que o snapshot da operacao ja entrega (`snapshot.roster`, montada em `applyOperationSnapshotToState`/`applyRemoteStoreData` no `runtime-remote.ts`). Assim TODO papel que pode operar a fila enxerga a faixa, sem vazar meta/comissao/e-mail (que o snapshot nao inclui). A faixa segue gateada por `canOperate` (`canMutateOperations`), entao papel read-only (director/marketing) nao ve botoes de mutacao. Bug historico: consultor de loja via a fila mas a faixa vinha vazia porque o roster so existia via `/v1/consultants`.

## Teste esperado

1. Criar regra com `displayKind = banner` → aparece no topo
2. Criar regra com `displayKind = corner_popup` → flutua no canto
3. Criar regra com `displayKind = center_modal` → modal blocking
4. Criar regra com `displayKind = fullscreen` → tela inteira
5. Responder a alerta → desaparece imediatamente
6. Aplicar regra via "Salvar e aplicar agora" → alertas em andamento são notificados
