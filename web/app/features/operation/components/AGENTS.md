# AGENTS

## Escopo

Estas instrucoes valem para `web/app/features/operation/components`.

## Objetivo

Esta pasta concentra a interface operacional da fila. Os componentes daqui devem continuar pequenos, orientados a uma responsabilidade clara e reaproveitar os componentes base do app sempre que possivel.

## Regras da pasta

- manter a pagina de operacao no mesmo modelo mental entre `Loja ativa` e `Todas as lojas`
- evitar criar componentes visuais paralelos para select, dropdown e picker
- para filtros simples, usar [AppSelectField.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ui/AppSelectField.vue)
- para selecao pesquisavel, multi-select e detalhes, reutilizar [OperationProductPicker.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/features/operation/components/OperationProductPicker.vue)
- cards operacionais devem ser compactos e sem informacao redundante com a coluna onde estao
- quando um card puder virar unidade reutilizavel, extraia antes de repetir markup inline
- quando o mesmo consultor tiver mais de um atendimento aberto, a UX deve comunicar `em aberto` ou `na sequencia`, sem sugerir concorrencia real; cada card continua sendo uma unidade fechada por `serviceId`
- [OperationFinishModal.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/features/operation/components/OperationFinishModal.vue) usa draft em `sessionStorage` por `storeId:serviceId`; ao reabrir, validar que o atendimento ainda existe em `activeServices` antes de reaproveitar o rascunho e descartar cache stale
- `OperationWorkspace.vue`, `OperationQueueColumns.vue`, `OperationFinishModal.vue` e `OperationOverviewBoard.vue` precisam ler o mesmo contexto operacional quando a tela estiver em `Todas as lojas`; se o modal ou o overview usarem um slice diferente do exibido nos cards, o fluxo de fechamento volta a falhar ou parecer desatualizado
- o cronometro dos cards e do overview nao pode depender de um tick de `1000ms` puro; usar refresh curto e sincronizar `now` quando `activeServices` muda evita o atraso visual do primeiro segundo apos iniciar atendimento
- para cronometro ativo em tela, nao truncar sempre a duracao para baixo no primeiro segundo; o display ao vivo deve arredondar a fracao positiva para cima, enquanto tempos congelados ou historicos continuam exibidos sem esse arredondamento
- quando houver skew entre relogio da API e relogio do navegador, os timers da operacao devem usar `serverClockOffsetMs` derivado do `savedAt` das mutacoes bem-sucedidas; comparar `Date.now()` puro com `serviceStartedAt` do backend volta a deixar o cronometro preso em `00:00`
- em cards `na sequencia`, o tempo exibido do atendimento anterior deve congelar no `serviceStartedAt` do proximo atendimento do mesmo grupo; nao usar sempre `Date.now() - serviceStartedAt`

## Catalogo atual

- [OperationWorkspace.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/features/operation/components/OperationWorkspace.vue)
  Composicao principal da pagina de operacao, incluindo barra de escopo, alerta de campanha, colunas e faixa de consultores.

- [OperationScopeBar.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/features/operation/components/OperationScopeBar.vue)
  Barra de contexto da operacao com loja, modo e filtro integrado. Usa `AppSelectField`.

- [OperationQueueColumns.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/features/operation/components/OperationQueueColumns.vue)
  Renderiza as colunas `Lista da vez` e `Em atendimento`, integra comandos da fila e abre o modal de fechamento.

- [OperationActiveServiceCard.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/features/operation/components/OperationActiveServiceCard.vue)
  Card dedicado de atendimento ativo. Mantem o bloco compacto, sem ID visivel e sem titulo redundante da coluna.

- [OperationConsultantStrip.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/features/operation/components/OperationConsultantStrip.vue)
  Faixa inferior com roster, entrada na fila, pausa, retomada e direcionamento para tarefa.
  A pausa deve consumir `pauseReasonOptions` do bundle de settings e nao pedir texto livre.

- [OperationCampaignBrief.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/features/operation/components/OperationCampaignBrief.vue)
  Alerta enxuto de campanha ativa na operacao, com CTA para a pagina de campanhas.

- [OperationFinishModal.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/features/operation/components/OperationFinishModal.vue)
  Modal de encerramento do atendimento, com formulario completo e validacoes operacionais.
  O rascunho local deve ser sempre reconciliado com o `serviceId` ativo antes de submeter o fechamento.

- [OperationProductPicker.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/features/operation/components/OperationProductPicker.vue)
  Picker pesquisavel reutilizado para produtos, motivos, origens e outros catalogos da operacao.

- [OperationPauseReasonDialog.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/features/operation/components/OperationPauseReasonDialog.vue)
  Dialogo de pausa que reaproveita `OperationProductPicker` em modo de selecao unica para motivos de pausa.

- [OperationOverviewBoard.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/features/operation/components/OperationOverviewBoard.vue)
  Blocos resumidos de leitura operacional e consolidado do modo integrado.

## Diretrizes rapidas

- se a alteracao for em `Em atendimento`, avaliar primeiro [OperationActiveServiceCard.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/features/operation/components/OperationActiveServiceCard.vue)
- se a alteracao for em `Lista da vez`, avaliar primeiro [OperationQueueColumns.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/features/operation/components/OperationQueueColumns.vue)
- se a alteracao for em filtros de operacao, usar [AppSelectField.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ui/AppSelectField.vue)
- se a alteracao for em catalogos selecionaveis do modal, usar [OperationProductPicker.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/features/operation/components/OperationProductPicker.vue)
