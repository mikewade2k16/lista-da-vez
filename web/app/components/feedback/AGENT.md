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

## Polling / nao-lidos / preview

- `FeedbackNotificationsDropdown.vue` fica montado no header de TODO o painel.
- **Sino e lista NAO baixam mensagens.** O contador de nao-lidos (`getUnreadCount` / badge `has-unread`) e o preview (`getFeedbackPreview`) vem direto do list: `GET /v1/feedback(/me)` devolve `unread_count` + `last_message_body` + `last_message_at` por feedback (ver AGENT.md do backend). O sino so renderiza feedbacks com `unread_count > 0` e `status != 'closed'`. So o chamado ABERTO baixa a thread real (`loadSelectedMessages` -> `fetchMessages`).
- Ao marcar como lido, `feedbackStore.applyLocalReadState` zera o `unread_count` local na hora (sem esperar o proximo list). O `upsertIntoCollection` (store) preserva `unread_count`/`last_message_*` quando a resposta nao os traz (mutacoes), entao um PATCH/POST nao apaga o badge que veio do list.
- Intervalos: sino 60s; lista do workspace 30s; mensagens do chamado aberto 15s. Todos os loads tem guard de `document.visibilityState` (nao requisitam com a aba oculta).
- Ao mudar o ritmo de polling em `useFeedbackWorkspace.js`, espelhar em `UserFeedbackWorkspace.vue` (versao admin e a do usuario do mesmo fluxo). O mesmo vale para `getUnreadCount`/`getFeedbackPreview`.
- **Evolucao futura mapeada:** trocar o polling por WebSocket (modulo `realtime`), publicando feedback novo / resposta nova. Roadmap: fase-7, task `feedback-realtime`.
