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

- [OperationCampaignBrief.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/features/operation/components/OperationCampaignBrief.vue)
  Alerta enxuto de campanha ativa na operacao, com CTA para a pagina de campanhas.

- [OperationFinishModal.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/features/operation/components/OperationFinishModal.vue)
  Modal de encerramento do atendimento, com formulario completo e validacoes operacionais.

- [OperationProductPicker.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/features/operation/components/OperationProductPicker.vue)
  Picker pesquisavel reutilizado para produtos, motivos, origens e outros catalogos da operacao.

- [OperationOverviewBoard.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/features/operation/components/OperationOverviewBoard.vue)
  Blocos resumidos de leitura operacional e consolidado do modo integrado.

## Diretrizes rapidas

- se a alteracao for em `Em atendimento`, avaliar primeiro [OperationActiveServiceCard.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/features/operation/components/OperationActiveServiceCard.vue)
- se a alteracao for em `Lista da vez`, avaliar primeiro [OperationQueueColumns.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/features/operation/components/OperationQueueColumns.vue)
- se a alteracao for em filtros de operacao, usar [AppSelectField.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ui/AppSelectField.vue)
- se a alteracao for em catalogos selecionaveis do modal, usar [OperationProductPicker.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/features/operation/components/OperationProductPicker.vue)
