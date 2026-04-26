# AGENT

## Escopo

Estas instrucoes valem para `back/internal/modules/realtime`.

## Responsabilidade do modulo

O modulo `realtime` cuida do transporte em tempo real da plataforma.

Hoje ele deve responder por:

- conexoes WebSocket autenticadas
- assinatura por loja
- assinatura administrativa por tenant
- entrega de eventos leves para a UI revalidar estado
- isolamento do transporte realtime em relacao aos modulos de negocio

Ele nao deve cuidar de:

- auth como fonte de verdade
- regra da fila
- montagem de snapshot operacional
- persistencia do estado

## Contrato atual

- `GET /v1/realtime/operations?storeId=...&access_token=...`
- `GET /v1/realtime/context?tenantId=...&access_token=...`

O token tambem pode vir por header `Authorization: Bearer ...`, util para clientes que nao precisam passar token na query string.

Eventos atuais:

- `realtime.connected`
- `operation.updated`
- `context.updated`

Shape atual do evento:

- `type`
- `tenantId`, quando for evento de contexto
- `storeId`, quando for evento operacional
- `action`
- `resource`
- `resourceId`
- `personId`
- `savedAt`

## Regras de arquitetura

- o payload do evento deve ser leve e orientado a invalidacao, nao um snapshot inteiro
- a leitura autoritativa continua em `GET /v1/operations/snapshot`
- `context.updated` serve para invalidacao leve de:
  - lojas acessiveis
  - usuarios e acessos
  - settings operacionais (agora tenant-wide; vale para todas as lojas do tenant)
  - header/contexto autenticado
- para settings, que agora e tenant-wide, o contrato publicado e apenas:
  - `context.updated` com `resource = settings`, `action = updated` e `resourceId = {tenantId}`
- o frontend pode revalidar snapshot apos receber um evento
- o frontend pode revalidar `GET /v1/me/context` e leituras administrativas apos `context.updated`
- o frontend atual revalida `GET /v1/settings?storeId=...` tanto no canal de contexto quanto no canal operacional quando a loja afetada coincide com a loja ativa
- a implementacao atual usa hub em memoria por processo para manter a base simples
- cada assinatura usa buffer pequeno e descarta evento antigo quando o consumidor fica para tras; realtime e invalidacao, nao fila duravel
- quando houver escala horizontal, este modulo deve trocar o hub local por broker externo sem quebrar o contrato WebSocket
- middlewares HTTP que embrulham `http.ResponseWriter` precisam preservar `http.Hijacker` e `http.Flusher`, senao o upgrade do websocket quebra
- conexoes enviam `ping` periodico e esperam `pong`; mensagens recebidas do cliente sao lidas apenas para manter a conexao viva

## Regras de seguranca

- toda conexao precisa autenticar token valido
- toda conexao precisa validar acesso do usuario a `store_id`
- a conexao operacional exige permissao efetiva `workspace.operacao.view` quando o principal ja vem com matriz resolvida; sem matriz resolvida, cai no fallback por papel operacional
- a conexao de contexto resolve o tenant pelo principal; `platform_admin` pode informar `tenantId`, e usuarios tenant-scoped nao podem assinar outro tenant
- o modulo deve respeitar a mesma politica de `Origin` configurada para o HTTP

## Publicadores atuais

- `operations` publica `operation.updated` para comandos da fila/atendimento
- `settings` publica somente `context.updated` quando a configuracao do tenant muda (modal, operation settings, produtos e catalogos ordenaveis); como settings agora e tenant-wide, o canal de contexto ja entrega a invalidacao a todos os atendentes do tenant e o evento `operation.updated` por loja deixou de ser usado por settings
- `stores`, `users` e `auth` publicam `context.updated` para reidratar contexto administrativo e autenticado

## Evolucao esperada

1. eventos para outros dominios
2. broker externo para multiplas replicas
3. resume/replay idempotente
4. observabilidade e metricas de conexao
