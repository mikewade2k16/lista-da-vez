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
- Salvar SO os modulos (overrides) de um usuario nao pode ser bloqueado pela validacao de dados basicos: em `saveDetails` (useUsersAccessManager.js) a validacao de nome/loja so roda quando os campos basicos mudaram (`basicChanged`). Mexer apenas nos modulos vai direto para o PUT de overrides, mesmo em usuario store-scoped sem loja vinculada.
- Honestidade do save: nunca dizer que os modulos foram salvos quando nao foram. Se a matriz de acesso nao carregou / a API de access falhou, mostrar o motivo REAL (`detailAccessError`) e deixar claro que os modulos NAO foram salvos — sem mascarar com "indisponivel".
- Revogacao ao vivo: o PUT de overrides faz o backend publicar um evento de contexto `access`; `useContextRealtime` re-busca `/v1/me/context` (atualiza `permissionKeys` -> menu) e roda `accessControl.refreshRealtimeState()`. O usuario afetado perde/ganha o modulo na hora, sem deslogar (backend nao usa cache de principal; re-resolve do banco a cada request).
- Aba "Perfis e padroes" (UsersRoleMatrixManager): reexposta em `/operacao/usuarios` (modo queue), mas SO para quem passa em `canManageRoleDefaults` (platform_admin / access.role_defaults.manage). Usuario comum da fila nao ve.
- Carregamento action-first (2026-06-26): `useUsersAccessManager` e SINCRONO (o `provideUsersAccessContext` precisa do ctx na hora, no setup). O `usersStore.ensureLoaded()` (usuarios + roles) roda em BACKGROUND via `.then`/`.catch` — NUNCA como `await` de topo no `<script setup>` do `UsersAccessManager.vue`. Aquele `const ctx = await useUsersAccessManager(...)` tornava o componente async e, sem `<Suspense>` no caminho, suspendia a troca de rota para `/usuarios`: a tela ficava parada na rota anterior ate `/v1/users` + `/v1/auth/roles` responderem, sem skeleton. Agora a rota troca na hora, o `AppEntityGrid` pinta o skeleton via `usersStore.pending` e os drafts re-sincronizam quando os dados chegam (reset pos-load). Ver docs/PERFORMANCE_AUDIT_PLAN.md §17.

## Fonte de dados

- CRUD administrativo via `web/app/stores/users.ts`
- reconciliacao cross-tab e cross-maquina via `web/app/composables/useContextRealtime.ts`
