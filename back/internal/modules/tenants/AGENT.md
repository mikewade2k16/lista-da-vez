# AGENT

## Escopo

Estas instrucoes valem para `back/internal/modules/tenants`.

## Responsabilidade do modulo

O modulo `tenants` cuida da leitura do escopo de cliente/grupo acessivel ao usuario autenticado.

Hoje ele deve responder por:

- listar tenants acessiveis
- ajudar a montar o contexto autenticado do usuario
- servir de base para futuras regras cross-store do cliente

Ele nao deve cuidar de:

- login e token
- CRUD de lojas
- regra operacional da fila
- websocket

## Contrato atual

- `GET /v1/tenants`

O endpoint e somente leitura nesta fase.

## Regras de escopo

- `platform_admin` pode listar todos os tenants ativos
- `owner`, `director` e `marketing` listam os tenants em que possuem membership
- `manager` e `consultant` enxergam o tenant derivado das lojas a que pertencem

## Regras de implementacao

- este modulo pode depender de `auth.Principal` para resolver escopo
- o repositorio PostgreSQL deve manter o filtro de tenant no banco, nao no handler
- respostas publicas devem usar `TenantView`, nunca a entidade interna completa

## Evolucao esperada

Quando crescer, este modulo deve absorver:

1. CRUD de tenants
2. configuracoes do cliente/grupo
3. billing/planos por tenant
4. auditoria administrativa cross-store
