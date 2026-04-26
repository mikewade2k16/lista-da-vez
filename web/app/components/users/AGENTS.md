# AGENTS

## Escopo

Estas instrucoes valem para `web/app/components/users`.

## Responsabilidade

Esta pasta concentra a workspace de usuarios e acessos.

## Regras atuais

- [UsersAccessManager.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/users/UsersAccessManager.vue) deve manter a grade fluida durante edicoes inline.
- mutacoes locais da grade devem atualizar a linha afetada primeiro e evitar recarregar a tabela inteira logo em seguida.
- o websocket de contexto continua obrigatorio para sincronizar outras instancias, mas a revalidacao local deve acontecer de forma silenciosa, sem overlay de loading na grade.
- contas `consultant` continuam sendo tratadas como sensiveis, mas `platform_admin` pode destravar manutencao inline e mudanca de perfil em ambiente administrativo.
- quando o perfil inline permitir `consultant`, o select tambem deve continuar respeitando a loja unica exigida para papeis store-scoped.

## Fonte de dados

- CRUD administrativo via `web/app/stores/users.ts`
- reconciliacao cross-tab e cross-maquina via `web/app/composables/useContextRealtime.ts`
