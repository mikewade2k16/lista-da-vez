# AGENT

## Escopo

Estas instrucoes valem para `back/internal/modules/stores`.

## Responsabilidade do modulo

O modulo `stores` cuida do catalogo de lojas acessiveis no tenant e das operacoes administrativas basicas sobre loja.

Hoje ele deve responder por:

- listar lojas acessiveis
- criar loja
- atualizar loja
- arquivar loja
- restaurar loja
- remover loja apenas quando a regra de negocio permitir
- entregar um formato de loja compativel com o Nuxt atual

Ele nao deve cuidar de:

- login e token
- CRUD de tenant
- regra da fila
- snapshot operacional

## Contrato atual

- `GET /v1/stores`
- `POST /v1/stores`
- `PATCH /v1/stores/{id}`
- `POST /v1/stores/{id}/archive`
- `POST /v1/stores/{id}/restore`
- `DELETE /v1/stores/{id}`

## Regras de escopo

- `platform_admin` pode listar e administrar lojas de qualquer tenant
- `owner` pode listar e administrar lojas do proprio tenant
- `marketing` e `director` podem listar lojas do proprio tenant, mas nao administrar
- `manager` e `consultant` listam apenas as lojas a que pertencem
- leituras de escopo de usuario devem vir de `core.account_users`, `core.user_role_assignments`/`core.roles` e `core.user_module_settings(module_id='queue').config.storeIdsByAccount`
- `user_store_roles` e legado temporario: manter somente dual-write/limpeza enquanto U4c nao remove as tabelas

## Regras operacionais obrigatorias

- persistencia de lojas usa `queue.stores`; limpeza de vinculos operacionais usa `queue.consultants`; nao voltar aos aliases publicos `stores`/`consultants`
- o contexto operacional normal deve continuar lendo apenas lojas ativas
- a visao administrativa pode pedir `includeInactive=true` quando precisar trabalhar com lojas arquivadas
- ao excluir loja, remover tambem o id da loja em `core.user_module_settings` para todos os usuarios com escopo naquele account
- manter a limpeza em `user_store_roles` enquanto existir o dual-write legado

## Regras de compatibilidade com o front

O DTO atual de loja deve continuar amigavel ao runtime local do Nuxt, incluindo:

- `id`
- `tenantId`
- `code`
- `name`
- `city`
- `isActive`
- `defaultTemplateId`
- `monthlyGoal`
- `weeklyGoal`
- `avgTicketGoal`
- `conversionGoal`
- `paGoal`

## Evolucao esperada

Quando crescer, este modulo deve absorver:

1. configuracoes locais por loja
2. auditoria de ciclo administrativo da loja
3. leitura gerencial consolidada cross-store
