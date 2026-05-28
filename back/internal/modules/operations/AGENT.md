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
- cronometro autoritativo dos alertas temporais ligados ao estado operacional aberto
- persistencia do historico operacional
- persistencia das sessoes de status dos consultores

Ele nao deve cuidar de:

- auth
- configuracoes do modal e catalogos
- busca remota de produtos para autocomplete
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
- a busca dinamica de produtos do modal deve consumir o modulo `catalog`; `operations` nao deve conhecer tabela ERP nem catalogo manual de settings
- a source `erp_current` do `catalog` esta tenant-scoped neste momento, porque os dados importados do ERP ainda vivem apenas na loja `184`; a Operacao continua enviando `storeId` para controle de acesso, nao para escolher tabela/coluna
- no modo novo de fechamento (`erp-reconciliation`), `operations` deve apenas persistir `purchaseCode` como referencia de conciliacao para compras; nao tentar buscar ERP em tempo real dentro do fechamento

## Alertas recentes do fluxo

- o nome legado `atendimento paralelo` continua aparecendo em partes do codigo, mas a regra operacional esperada e permitir mais de um atendimento em aberto para o mesmo consultor, mantendo encerramento individual posterior por `serviceId`
- cada atendimento em aberto precisa manter cronometro, historico e fechamento proprios; o consultor so volta para a fila ao encerrar o ultimo atendimento ativo dele
- `POST /v1/operations/finish` e identificado por `serviceId`; qualquer cache ou draft do frontend precisa ser invalidado se esse `serviceId` nao existir mais no snapshot atual
- incidente recente: o erro de encerramento reportado passou a aparecer quando o modal foi reaberto com draft restaurado; o frontend agora invalida rascunho stale por `storeId + serviceId + serviceStartedAt` antes de reaproveitar ou submeter esse payload
- atendimentos abertos `na sequencia` precisam herdar do primeiro atendimento do grupo `queueJoinedAt`, `queueWaitMs`, `queuePositionAtStart` e `skippedPeople`; perder esses metadados no backend volta a quebrar o insert em `operation_service_history` ou distorce o historico
- a duracao efetiva de um atendimento em aberto na sequencia nao vai ate o momento do fechamento manual quando ja existe um proximo atendimento do mesmo grupo; ela deve ser truncada no `startedAt` do proximo `serviceId` do grupo
- no modo integrado, mutacoes e fechamento precisam usar a `storeId` do proprio servico; depender apenas de `activeStoreId` reintroduz erro silencioso em `Todas as lojas`
- houve incidente de ambiente com a API cerca de 4,5s a frente do navegador/host; enquanto esse skew existir, os timestamps persistidos no backend continuam corretos para auditoria, mas a UI precisa compensar `savedAt -> Date.now()` para o cronometro nao parecer atrasado ao iniciar atendimento
- `POST /v1/operations/finish` com `action=cancel` reinsere o consultor na fila usando o `QueueJoinedAt` original como chave de ordenacao: percorre a fila atual e insere antes da primeira pessoa cujo `QueueJoinedAt` seja maior; isso preserva a ordem relativa corretamente mesmo quando a fila encolheu (ex.: era o 2o, o 1o foi para atendimento, o 10o ficou na fila — volta como 1o porque entrou antes do 10o)
- `POST /v1/operations/finish` com `action=stop` nao exige `stopReason`; o campo e opcional e gravado se vier preenchido; sem justificativa obrigatoria nos dois modais (cancel e stop)
- `operations` continua como fonte de verdade dos alertas temporais operacionais: ele le os thresholds configurados em `alerts`, roda a reavaliacao temporal no backend e emite sinais leves para `alerts` abrir, atualizar ou resolver a instancia correspondente
- `alerts` nao deve ter timer proprio para `long_open_service`; quando o tempo vence, e `operations` quem dispara o sinal `long_open_service.triggered`, e quando o atendimento termina, cancela ou para, e `operations` quem dispara `long_open_service.resolved`
- atendimento com `StoppedAt` preenchido continua visivel no snapshot como atendimento parado, mas sai do cronometro de `long_open_service`; parar ou cancelar atendimento deve resolver qualquer alerta operacional aberto daquele `serviceId`
- `readJSONLenient` agora carrega o body antes de decodificar e devolve preview do payload + `Content-Type` no campo `details.cause` quando o JSON falha; manter esse comportamento facilita debug de payload mismatch via toast no frontend

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

## Registro de implementacao (Fase 5-6: Novos signal types e builders)

### Novos signal types em alerts.go

Adicionados 8 constantes para 4 novos tipos de trigger:

- `SignalLongQueueWaitTriggered`: fila > threshold
- `SignalLongQueueWaitResolved`: fila normalizada
- `SignalLongPauseTriggered`: pausa > threshold
- `SignalLongPauseResolved`: pausa retomada
- `SignalIdleStoreTriggered`: loja sem atividade em horário comercial
- `SignalIdleStoreResolved`: atividade retomada
- `SignalOutsideBusinessHoursTriggered`: atendimento fora do horário
- `SignalOutsideBusinessHoursResolved`: atendimento encerrado ou em horário normal

### Extensão de OperationalAlertSignal

Novos campos adicionados ao struct `OperationalAlertSignal`:

- `ConsultantName string`: nome do consultor (denormalizando para o alerta)
- `ElapsedMinutes int`: minutos decorridos (para template `{elapsed}`)
- `TriggerType string`: qual trigger gerou o sinal (para identificar a regra a carregar)

### Builders em operations/service.go (stubs para MVP)

```go
func (s *Service) buildLongQueueWaitSignals(ctx, storeId, snapshot, rules, now) []OperationalAlertSignal {
    // Para cada item em snapshot.WaitingList:
    // if now - QueueJoinedAt > rule.threshold → emit signal
    // Stub retorna []
}

func (s *Service) buildLongPauseSignals(ctx, storeId, snapshot, rules, now) []OperationalAlertSignal {
    // Para cada item em snapshot.PausedEmployees:
    // if now - StartedAt > rule.threshold → emit signal
    // Stub retorna []
}

func (s *Service) buildIdleStoreSignals(ctx, storeId, snapshot, rules, now) []OperationalAlertSignal {
    // if (WaitingList.empty && ActiveServices.empty) && !finalizadoRecentemente → emit
    // ConsultantID vazio (alerta geral da loja)
    // Stub retorna []
}

func (s *Service) buildOutsideBusinessHoursSignals(ctx, storeId, snapshot, rules, now) []OperationalAlertSignal {
    // Para cada ActiveService:
    // if started_at fora do horário comercial da loja → emit
    // Requer integração com store hours de `stores` module
    // Stub retorna []
}
```

### OperationsScanner interface (para retroatividade)

Implementação em operations/service.go:

```go
func (s *Service) ScanForRule(ctx context.Context, rule alerts.RuleDefinition) (interface{}, error) {
    // Carrega snapshot atual de todas as lojas do tenant
    // Roda o builder específico para rule.TriggerType
    // Retorna signals que casam com a regra
    // Retorna interface{} para evitar import cycle
}
```

Injeção em app.go:
```go
alertsService.SetOperationsScanner(operationsService)
```

### Comportamento esperado

1. **Materializacao:** Quando um sinal chega ao `alerts`, o módulo carrega a regra ativa do tipo de trigger correspondente e cria a instância com snapshot da regra (cores, templates, interação).

2. **Retroatividade:** Quando o usuário salva uma nova regra via API `POST /v1/alerts/rules`, o endpoint chamado `POST /v1/alerts/rules/{id}/apply-now` faz:
   - Carrega a regra
   - Chama `operationsScanner.ScanForRule(ctx, rule)`
   - Processa sinais retornados
   - Cria alertas para atendimentos/contextos já em andamento que se enquadram na regra

3. **Scheduler (15s):** O `ProcessTimedAlerts` em `app.go` continua rodando, carregando as regras ativas de todos os triggers e materializando alertas para todos os contextos vivos que as acionam.

### Regra crítica: Evitar import cycle

Para que `operations` não dependa de `alerts` (e vice-versa), a interface `OperationsScanner` tem método que retorna `interface{}`, e `alerts` faz type assertion após receber:

```go
// Em alerts/service.go
signals := s.operationsScanner.ScanForRule(ctx, *rule)
typedSignals, ok := signals.([]alerts.OperationalSignalInput)
if !ok {
    return 0, errors.New("scanner retornou tipo inesperado")
}
// processa typedSignals...
```
