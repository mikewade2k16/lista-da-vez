# AGENTS

## Escopo

Estas instruções valem para `web/app/components/operation/`.

## Responsabilidade dos componentes

Este diretório cuida da renderização visual da operação, incluindo:

- Workspace principal de operação
- Cards e estado dos consultores
- Queue visível
- Alertas operacionais em diversos formatos
- Modais e diálogos operacionais

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
