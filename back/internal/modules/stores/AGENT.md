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

## Regras operacionais obrigatorias

- o contexto operacional normal deve continuar lendo apenas lojas ativas
- a visao administrativa pode pedir `includeInactive=true` quando precisar trabalhar com lojas arquivadas
- exclusao de loja nunca deve depender apenas do `on delete cascade`
- antes de excluir, o modulo deve bloquear a operacao se ainda existirem:
  - consultores vinculados
  - usuarios vinculados por `user_store_roles`
  - fila, atendimento, pausa ou status operacional
  - historico operacional
- o bloqueio deve voltar para a UI com detalhes estruturados, para o admin entender o motivo

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
