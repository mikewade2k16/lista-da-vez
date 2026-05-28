# AGENT - Feedback Components

## Escopo

Componentes administrativos de feedback em `web/app/components/feedback/`.

## Padrao Atual

- `FeedbackWorkspace.vue` e o host fino da tela: header, filtros, lista e painel de detalhes.
- `FeedbackFilters.vue`, `FeedbackList.vue` e `FeedbackDetailPanel.vue` concentram a UI administrativa.
- `FeedbackFormModal.vue` segue preservado para envio de feedback pelo usuario.
- `useFeedbackWorkspace.js` concentra selecao, polling, respostas, status e sincronizacao com rota.
- `feedback-display.js` concentra opcoes e formatadores puros de tipo, status e data.
- `feedback-workspace.css` guarda os estilos compartilhados entre o host e os subcomponentes.

## Regras Locais

- Nao mover regra de store para componentes visuais.
- Consumir o contexto do workspace via `useFeedbackWorkspaceContext()` nos subcomponentes administrativos.
- Manter componentes filhos abaixo de 500 linhas e validar filtros/lista/detalhe em `/feedback`.
- Preservar `FeedbackFormModal.vue` e `UserFeedbackWorkspace.vue` ao mexer na tela administrativa.
