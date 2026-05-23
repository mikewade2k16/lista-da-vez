# AGENTS

## Escopo

Estas instrucoes valem para `web/app/components/users`.

## Responsabilidade

Esta pasta concentra a workspace de usuarios e acessos.

## Padrao atual

- [UsersAccessManager.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/users/UsersAccessManager.vue) e host fino da tela.
- `UsersAccessCreateModal.vue`, `UsersAccessTable.vue`, `UsersAccessDetailDrawer.vue`, `UsersAccessDetailSummary.vue`, `UsersAccessDetailForm.vue` e `UsersAccessPermissionPanel.vue` concentram a UI.
- `useUsersAccessManager.js` concentra stores, filtros, permissoes, acoes de usuario e fluxo de detalhe.
- `useUserAccessDrafts.ts` concentra drafts de criacao, linha e detalhe.
- `user-access.ts` concentra helpers puros de normalizacao, labels, tons e overrides.
- `users-access-manager.css` guarda os estilos compartilhados entre host e subcomponentes.

## Regras atuais

- A grade deve continuar fluida durante edicoes inline.
- Mutacoes locais da grade devem atualizar a linha afetada primeiro e evitar recarregar a tabela inteira logo em seguida.
- O websocket de contexto continua obrigatorio para sincronizar outras instancias, mas a revalidacao local deve acontecer de forma silenciosa, sem overlay de loading na grade.
- Contas `consultant` continuam sendo tratadas como sensiveis, mas `platform_admin` pode destravar manutencao inline e mudanca de perfil em ambiente administrativo.
- Quando o perfil inline permitir `consultant`, o select tambem deve continuar respeitando a loja unica exigida para papeis store-scoped.

## Fonte de dados

- CRUD administrativo via `web/app/stores/users.ts`
- reconciliacao cross-tab e cross-maquina via `web/app/composables/useContextRealtime.ts`
