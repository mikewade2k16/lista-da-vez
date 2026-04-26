# AGENT

## Escopo

Estas instrucoes valem para `back/internal/modules/tenants`.

## Responsabilidade do modulo

O modulo `tenants` cuida da leitura do escopo de cliente/grupo acessivel ao usuario autenticado.

Hoje ele deve responder por:

- listar tenants acessiveis
- ajudar a montar o contexto autenticado do usuario
- criar cliente/grupo quando autorizado
- atualizar dados basicos do cliente/grupo quando autorizado
- arquivar e restaurar cliente/grupo
- servir de base para futuras regras cross-store do cliente

Ele nao deve cuidar de:

- login e token
- CRUD de lojas
- regra operacional da fila
- websocket

## Contrato atual

- `GET /v1/tenants`
- `POST /v1/tenants`
- `PATCH /v1/tenants/{id}`
- `POST /v1/tenants/{id}/archive`
- `POST /v1/tenants/{id}/restore`

`GET /v1/tenants` aceita `includeInactive=true` para leitura administrativa.

## Regras de escopo

- `platform_admin` pode listar todos os tenants ativos
- `owner`, `director` e `marketing` listam os tenants em que possuem membership
- `manager` e `consultant` enxergam o tenant derivado das lojas a que pertencem
- criacao de tenant continua restrita a `platform_admin`
- atualizacao, arquivamento e restauracao exigem permissao de edicao de clientes ou papel autorizado; a validacao final acontece no service e no repositorio acessivel

## Regras de implementacao

- este modulo pode depender de `auth.Principal` para resolver escopo
- o repositorio PostgreSQL deve manter o filtro de tenant no banco, nao no handler
- respostas publicas devem usar `TenantView`, nunca a entidade interna completa

## Evolucao esperada

Quando crescer, este modulo deve absorver:

1. configuracoes do cliente/grupo
2. billing/planos por tenant
3. auditoria administrativa cross-store
