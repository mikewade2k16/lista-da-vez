# AGENT - Support Components

## Escopo

Componentes administrativos de suporte em `web/app/components/feedback/`.
O produto e a navegacao usam o nome **Suporte**. Arquivos, stores, permissoes,
workspace ID e endpoints continuam usando `feedback` por compatibilidade com os
chamados existentes.

## Padrao Atual

- `FeedbackWorkspace.vue` e o host fino da tela: header, filtros, lista e painel de detalhes.
- `FeedbackFilters.vue`, `FeedbackList.vue` e `FeedbackDetailPanel.vue` concentram a UI administrativa.
- `FeedbackFormModal.vue` segue preservado para abertura de chamado pelo usuario.
- `useFeedbackChat.js` e o NUCLEO compartilhado de chat (parametrizavel): autosize do textarea, ciclo da imagem anexada, scroll, leitura/marcacao e polling das mensagens do chamado aberto. Recebe `selectedFeedback`/`selectedMessages` (computed), `isReadFromOwnerPerspective` (perspectiva de nao-lido), `loadFeedbackUpdates` (recarga da lista) e `messagesLoadErrorMessage`. Tambem registra o tracking de visibilidade e a limpeza (`stopPolling`/`clearReplyImage`) no `onBeforeUnmount`.
- `useFeedbackWorkspace.js` (admin) consome o nucleo e adiciona o que e so do admin: filtros, busca, sincronizacao com rota, status (`editingStatus`/`saveStatus`), `canEditFeedback` e o `sendReply` com persistencia de status. Continua expondo o mesmo objeto `reactive(...)` via provide/inject.
- `UserFeedbackWorkspace.js` (usuario) consome o mesmo nucleo como wrapper fino, em modo so leitura/responder: sem filtros nem status, perspectiva de nao-lido invertida (`authorUserId !== ownUserId`) e `sendReply` simples.
- `feedback-display.js` concentra opcoes e formatadores puros de tipo, status e data.
- `feedback-workspace.css` guarda os estilos compartilhados entre o host e os subcomponentes.

## Regras Locais

- Nao mover regra de store para componentes visuais.
- Consumir o contexto do workspace via `useFeedbackWorkspaceContext()` nos subcomponentes administrativos.
- Manter componentes filhos abaixo de 500 linhas e validar filtros/lista/detalhe em
  `/suporte`; `/feedback` e `/operacao/feedback` continuam como aliases.
- Preservar `FeedbackFormModal.vue` e `UserFeedbackWorkspace.vue` ao mexer na tela administrativa.

## Polling / nao-lidos / preview

- `FeedbackNotificationsDropdown.vue` fica montado no header de TODO o painel.
- **Sino e lista NAO baixam mensagens.** O contador de nao-lidos (`getUnreadCount` / badge `has-unread`) e o preview (`getFeedbackPreview`) vem direto do list: `GET /v1/feedback(/me)` devolve `unread_count` + `last_message_body` + `last_message_at` por feedback (ver AGENT.md do backend). O sino so renderiza feedbacks com `unread_count > 0` e `status != 'closed'`. So o chamado ABERTO baixa a thread real (`loadSelectedMessages` -> `fetchMessages`).
- Ao marcar como lido, `feedbackStore.applyLocalReadState` zera o `unread_count` local na hora (sem esperar o proximo list). O `upsertIntoCollection` (store) preserva `unread_count`/`last_message_*` quando a resposta nao os traz (mutacoes), entao um PATCH/POST nao apaga o badge que veio do list.
- Intervalos: sino 60s; lista do workspace 30s; mensagens do chamado aberto 15s. Todos os loads tem guard de `document.visibilityState` (nao requisitam com a aba oculta).
- O ritmo de polling, `getUnreadCount`, `getFeedbackPreview` e a leitura/scroll agora moram em `useFeedbackChat.js` (fonte unica para admin e usuario) — mudar la vale para os dois. A unica peca de nao-lido que diverge e a perspectiva (`isReadFromOwnerPerspective`), injetada por cada lado.
- **Evolucao futura mapeada:** trocar o polling por WebSocket (modulo `realtime`), publicando feedback novo / resposta nova. Roadmap: fase-7, task `feedback-realtime`.
