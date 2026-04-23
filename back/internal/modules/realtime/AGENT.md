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

Eventos atuais:

- `realtime.connected`
- `operation.updated`
- `context.updated`

## Regras de arquitetura

- o payload do evento deve ser leve e orientado a invalidacao, nao um snapshot inteiro
- a leitura autoritativa continua em `GET /v1/operations/snapshot`
- `context.updated` serve para invalidacao leve de:
  - lojas acessiveis
  - usuarios e acessos
  - header/contexto autenticado
- o frontend pode revalidar snapshot apos receber um evento
- a implementacao atual usa hub em memoria por processo para manter a base simples
- quando houver escala horizontal, este modulo deve trocar o hub local por broker externo sem quebrar o contrato WebSocket
- middlewares HTTP que embrulham `http.ResponseWriter` precisam preservar `http.Hijacker` e `http.Flusher`, senao o upgrade do websocket quebra

## Regras de seguranca

- toda conexao precisa autenticar token valido
- toda conexao precisa validar acesso do usuario a `store_id`
- o modulo deve respeitar a mesma politica de `Origin` configurada para o HTTP

## Evolucao esperada

1. eventos para outros dominios
2. broker externo para multiplas replicas
3. resume/replay idempotente
4. observabilidade e metricas de conexao
