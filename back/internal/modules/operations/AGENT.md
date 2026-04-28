# AGENT

## Escopo

Estas instrucoes valem para `back/internal/modules/operations`.

## Responsabilidade do modulo

O modulo `operations` cuida da fila operacional por loja.

Hoje ele deve responder por:

- snapshot da operacao por loja
- overview integrado da operacao para usuarios com escopo multi-loja
- entrada na fila
- pausa e retomada
- retirada da fila para tarefa ou reuniao
- inicio de atendimento
- encerramento de atendimento
- persistencia do historico operacional
- persistencia das sessoes de status dos consultores

Ele nao deve cuidar de:

- auth
- configuracoes do modal e catalogos
- campanhas como fonte de verdade
- relatorios server-side
- websocket

## Contrato minimo para plugar em outro projeto

O service de `operations` nao deve depender do projeto host inteiro.

Hoje o contrato minimo de entrada do modulo e:

- `AccessContext`
- `Repository`
- `StoreScopeProvider`
- `EventPublisher`

### `AccessContext`

Representa o minimo que o modulo precisa da autenticacao:

- `user_id`
- `tenant_id`
- `role`
- `store_ids[]`

O adapter HTTP atual converte `auth.Principal` para esse contrato, mas outro projeto host pode fornecer o mesmo shape a partir do seu proprio auth.

### `StoreScopeProvider`

Representa o minimo que o modulo precisa do cadastro de lojas:

- listar lojas acessiveis do usuario
- devolver `id`, `tenantId`, `code`, `name`, `city`

O modulo nao deve depender do CRUD inteiro de lojas para funcionar.

### `Repository`

Representa a persistencia operacional:

- roster
- estado corrente da fila
- atendimentos ativos
- pausas/tarefas
- status corrente
- append de historico
- append de sessoes

### `EventPublisher`

Representa a invalidacao/realtime.

Pode ser:

- websocket
- broker
- noop

Se o host nao quiser realtime, o modulo continua funcionando com publisher noop.

## Contrato atual

- `GET /v1/operations/snapshot?storeId=...`
- `GET /v1/operations/overview`
- `POST /v1/operations/queue`
- `POST /v1/operations/pause`
- `POST /v1/operations/resume`
- `POST /v1/operations/assign-task`
- `POST /v1/operations/start`
- `POST /v1/operations/finish`

Regra de resposta:

- `GET /v1/operations/snapshot` devolve o snapshot operacional completo da loja
- `GET /v1/operations/overview` devolve a visao operacional integrada das lojas acessiveis da sessao autenticada
- comandos `POST` devolvem apenas `ack` minimo (`ok`, `storeId`, `savedAt`, `action`, `personId`)
- o frontend deve revalidar o snapshot por `GET /v1/operations/snapshot` apos mutacao bem-sucedida
- no modo integrado, o frontend deve revalidar `GET /v1/operations/overview` apos mutacao bem-sucedida
- `POST /v1/operations/finish` deve receber apenas os campos aplicaveis ao desfecho atual; por exemplo, `lossReasons*` so sobem em `nao-compra`
- campos opcionais/default sem valor de negocio nao devem subir como string vazia, array vazio ou objeto vazio

## Regras de escopo

- leitura: `consultant`, `store_terminal`, `manager`, `marketing`, `director`, `owner` e `platform_admin`
- comando: `consultant`, `manager`, `owner` e `platform_admin`
- leitura integrada multi-loja: qualquer sessao com mais de uma loja acessivel
- sempre validar `store_id` contra o principal autenticado

## Regra de persistencia

- o estado corrente vive em tabelas correntes por loja:
  - `operation_queue_entries`
  - `operation_active_services`
  - `operation_paused_consultants`
  - `operation_current_status`
- `operation_paused_consultants.kind` diferencia pausa comum de deslocamento operacional:
  - `pause`
  - `assignment`
- auditoria vive em tabelas append-only:
  - `operation_status_sessions`
  - `operation_service_history`
- o snapshot enviado ao Nuxt deve manter compatibilidade com o shape atual do runtime, para reduzir retrabalho no frontend
- comandos nao devem devolver o snapshot inteiro da loja; isso aumenta payload, confunde debug e mistura leitura com mutacao
- o modulo ja esta integrado ao Nuxt via `web/app/stores/operations.ts` e `web/app/utils/runtime-remote.ts`

## Alertas recentes do fluxo

- o nome legado `atendimento paralelo` continua aparecendo em partes do codigo, mas a regra operacional esperada e permitir mais de um atendimento em aberto para o mesmo consultor, mantendo encerramento individual posterior por `serviceId`
- cada atendimento em aberto precisa manter cronometro, historico e fechamento proprios; o consultor so volta para a fila ao encerrar o ultimo atendimento ativo dele
- `POST /v1/operations/finish` e identificado por `serviceId`; qualquer cache ou draft do frontend precisa ser invalidado se esse `serviceId` nao existir mais no snapshot atual
- incidente recente: o erro de encerramento reportado passou a aparecer quando o modal foi reaberto com draft restaurado; o frontend agora invalida rascunho stale por `storeId + serviceId + serviceStartedAt` antes de reaproveitar ou submeter esse payload
- atendimentos abertos `na sequencia` precisam herdar do primeiro atendimento do grupo `queueJoinedAt`, `queueWaitMs`, `queuePositionAtStart` e `skippedPeople`; perder esses metadados no backend volta a quebrar o insert em `operation_service_history` ou distorce o historico
- a duracao efetiva de um atendimento em aberto na sequencia nao vai ate o momento do fechamento manual quando ja existe um proximo atendimento do mesmo grupo; ela deve ser truncada no `startedAt` do proximo `serviceId` do grupo
- no modo integrado, mutacoes e fechamento precisam usar a `storeId` do proprio servico; depender apenas de `activeStoreId` reintroduz erro silencioso em `Todas as lojas`
- houve incidente de ambiente com a API cerca de 4,5s a frente do navegador/host; enquanto esse skew existir, os timestamps persistidos no backend continuam corretos para auditoria, mas a UI precisa compensar `savedAt -> Date.now()` para o cronometro nao parecer atrasado ao iniciar atendimento

## Estado atual

Hoje este modulo ja sustenta:

- fila por loja em PostgreSQL
- atendimentos ativos
- pausas e retomadas
- designacao de tarefa/reuniao com retirada controlada da fila
- historico de atendimento
- sessoes de status
- hidratacao do frontend no login/troca de loja
- visao integrada da operacao para sessoes com mais de uma loja acessivel
- cards operacionais com identificacao visual da loja de origem

## Regra de acoplamento

- qualquer dependencia com `auth`, `stores` ou outro modulo host deve entrar por adapter pequeno na borda
- a regra de negocio do service deve falar a linguagem do proprio modulo
- este modulo ja usa `AccessContext` no service; nao voltar a passar `auth.Principal` direto para a regra de negocio

Proximo passo natural:

- filtros administrativos mais ricos sobre historico operacional e ultimos atendimentos
- notificacao operacional estruturada para tarefa/reuniao
- refinamentos de auditoria cross-store
