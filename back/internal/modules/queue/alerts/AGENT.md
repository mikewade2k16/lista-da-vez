# AGENT

## Escopo

Estas instrucoes valem para `back/internal/modules/alerts`.

## Responsabilidade do modulo

O modulo `alerts` cuida da ativacao, persistencia, roteamento, entrega e auditoria
dos alertas operacionais da plataforma.

Ele existe para desacoplar a camada de alerta das regras centrais da fila,
permitindo que a Operacao continue como fonte de verdade do estado operacional e
que Alertas vire o ponto unico para:

- registrar regras operacionais de alerta
- materializar instancias de alerta abertas, reconhecidas e resolvidas
- deduplicar alertas recorrentes sobre o mesmo contexto
- decidir destino e canal de entrega
- publicar notificacoes em tempo real para a UI
- registrar acknowledge, resolucao e acoes administrativas
- preparar integracoes futuras com canais externos e eventos positivos

Ele nao deve cuidar de:

- auth como fonte de verdade
- cronometro autoritativo do atendimento
- deteccao primaria do tempo de atendimento ou ociosidade operacional
- snapshot da operacao
- CRUD estrutural de lojas
- horario de funcionamento como dado de cadastro
- logica de fila, inicio, pausa, retomada ou encerramento como regra primaria

## Regra principal de arquitetura

`operations` e a origem do sinal operacional.

`alerts` nao deve ficar recalculando por conta propria a regra de negocio que ja
pertence a Operacao quando a origem do evento depende do estado da fila, do
atendimento, do tempo de atendimento, do tempo ocioso ou do horario de
funcionamento da loja.

Fluxo esperado:

1. a Operacao conhece o estado corrente
2. a Operacao conhece os tempos configurados que disparam alerta
3. quando uma condicao e atingida, `operations` emite um sinal leve
4. `alerts` recebe esse sinal, resolve a regra de entrega, cria ou atualiza a instancia do alerta, audita e publica o evento para os destinos necessarios

Resumo pratico:

- `operations` detecta
- `alerts` orquestra
- `realtime` transporta
- `stores` fornece horario de funcionamento

Atualizacao do contrato operacional:

- `operations` tambem e a fonte autoritativa do timer dos alertas temporais; ele le o threshold salvo em `alerts`, acompanha o vencimento e reenfileira o sinal quando o limite e atingido
- `alerts` nao deve abrir loop, ticker ou cronometro proprio para `long_open_service`; seu papel continua sendo materializar, deduplicar, auditar, entregar e resolver a instancia a partir do sinal vindo de `operations`
- eventos de `finish`, `cancel` e `stop` tambem devem ser comunicados por `operations` para `alerts`, para que a resolucao do alerta continue coerente com o `serviceId` real da operacao

## Dependencias oficiais do modulo

O modulo `alerts` pode depender de adapters pequenos para os seguintes dominios:

- `operations`
  - para receber sinais de incidente operacional
  - para consultar referencias minimas do servico/loja quando necessario
  - para executar acoes administrativas controladas, como encerramento manual via service da Operacao
- `realtime`
  - para publicar invalidacoes e notificacoes leves para UI
- `stores`
  - para ler metadados da loja e seu horario de funcionamento cadastrado
- `access`
  - para validar permissoes de leitura, acknowledge, gestao de regras e acao administrativa
- `auth`
  - apenas na borda HTTP, nunca como dependencia direta da regra de negocio

Dependencias futuras permitidas:

- `notifications` ou adapter equivalente para WhatsApp, e-mail, push ou webhooks
- `users` para roteamento mais inteligente por perfil, grupo ou dono do incidente
- `reports` / `analytics` para eventos positivos e gamificacao

## Contrato conceitual inicial

O modulo deve nascer preparado para dois tipos de entrada:

### 1. Sinal operacional

Entrada vinda de `operations`, por exemplo:

- atendimento aberto acima do limite
- loja em horario de funcionamento sem uso da operacao acima do limite ocioso
- atendimento atravessando o fechamento da loja
- atendimento parado sem encerramento apos tolerancia configurada

O sinal deve ser leve e orientado a contexto, por exemplo:

- `tenantId`
- `storeId`
- `serviceId`, quando existir atendimento
- `consultantId`, quando existir consultor associado
- `signalType`
- `triggeredAt`
- `metadata` minima para auditoria e roteamento

### 2. Comando administrativo

Entrada humana via painel, por exemplo:

- acknowledge do alerta
- marcar como resolvido
- encerrar atendimento por acao administrativa
- dispensar falso positivo com justificativa

## Escopos de configuracao

### Regras do modulo `alerts`

Devem morar no proprio modulo, nao em `settings`, quando forem regras de alerta
operacional ou de entrega.

Exemplos:

- minutos para atendimento aberto virar alerta
- minutos de ociosidade da loja em horario de funcionamento
- janela de tolerancia apos fechamento da loja
- se o alerta vai para UI, grupo, gestor ou consultores
- severidade padrao
- cooldown entre reenvios

### Horario de funcionamento da loja

Nao pertence a `alerts`.

Horario de funcionamento e dado estrutural da loja e deve morar no dominio de
`stores` / workspace Multi-loja.

`alerts` apenas consome esse dado.

### Thresholds analiticos atuais

Os thresholds atuais de performance do Ranking e da antiga aba de alertas em
`settings` nao entram automaticamente neste modulo.

Eles continuam onde estao ate migracao explicita.

Regressao a evitar:

- nao misturar alerta operacional em tempo real com threshold analitico mensal
- nao transformar `alerts` em novo `settings`

## Fontes de alerta previstas

### Primeira onda: alertas operacionais negativos

- `long_open_service`
  - atendimento aberto acima do tempo configurado
- `idle_store_during_business_hours`
  - loja em horario de funcionamento sem uso relevante da operacao acima do limite configurado
- `service_outside_business_hours`
  - atendimento aberto fora do horario da loja
- `store_open_without_operation_usage`
  - loja aberta com plataforma ociosa ou sem engajamento minimo

### Segunda onda: acoes administrativas e higiene operacional

- atendimento esquecido sem encerramento
- atendimento aberto do dia anterior
- necessidade de intervencao do gestor
- fechamento administrativo com justificativa

### Terceira onda: eventos positivos e gamificacao

- venda muito acima do padrao
- consultor bateu meta cedo
- recuperacao de loja ociosa
- sequencia boa de atendimento
- campanhas internas e reconhecimentos

Regra de produto:

o modulo deve nascer com nomes, categorias e schema que permitam suportar
alertas negativos e eventos positivos sem refazer a modelagem base.

## Canais de entrega esperados

### Canal obrigatorio inicial

- pagina dedicada de Alertas na plataforma
- atualizacao em tempo real via WebSocket
- avisos contextuais no fluxo da Operacao quando o perfil nao usar a pagina como centro operacional

### Canais futuros previstos

- grupo de WhatsApp dos consultores
- grupo de WhatsApp da gestao
- notificacao individual por usuario
- e-mail administrativo
- webhook para automacoes externas

Regra importante:

o primeiro contrato do modulo deve separar claramente:

- `alert event` = o fato detectado
- `delivery target` = para quem vai
- `delivery channel` = por onde vai
- `delivery attempt` = cada tentativa de envio

Sem isso, a futura integracao com WhatsApp vira acoplamento irreversivel.

## Papel do realtime

Realtime aqui significa entrega em tempo quase imediato do alerta depois que a
Operacao emite o sinal, e nao calculo autonomo do tempo dentro do transporte.

`realtime` continua apenas como meio de transporte.

Fluxo esperado para UI:

1. `operations` detecta e emite sinal
2. `alerts` cria ou atualiza a instancia
3. `alerts` publica invalidacao/evento leve
4. frontend revalida leitura autoritativa do modulo `alerts` e, quando necessario, da propria Operacao

Regra de arquitetura:

- nao publicar snapshot inteiro pelo WebSocket
- nao colocar logica de deduplicacao no frontend
- nao deixar o frontend decidir sozinho se o alerta existe ou nao

## Papel da Operacao como fonte de verdade

Regras que pertencem a `operations` e nao devem migrar para `alerts`:

- quando um atendimento realmente comecou
- quando esta em aberto
- quanto tempo esta em aberto
- se a loja esta sendo usada ou nao
- se ha atendimento ativo, fila ativa, pausa ou retorno
- quando um atendimento foi encerrado

O modulo `alerts` pode manter seus proprios estados de negocio sobre o alerta,
mas nunca substituir o estado autoritativo da Operacao.

Exemplo correto:

- `operations` informa que o `serviceId` X entrou em condicao de atraso
- `alerts` abre o alerta Y e entrega
- admin encerra via `alerts`
- `alerts` delega o encerramento para `operations`
- `operations` grava o historico autoritativo
- `alerts` grava a auditoria administrativa da acao

## Escopo e permissoes esperados

Permissoes previstas para o modulo:

- `workspace.alertas.view`
- `workspace.alertas.edit`
- `alerts.rules.manage`
- `alerts.actions.manage`
- `alerts.deliveries.manage`, se a camada de canais externos ficar neste modulo

Visao inicial por perfil:

- `platform_admin`
  - visao completa, regras, auditoria e canais
- `owner`
  - visao tenant-wide, regras e acoes
- `manager`
  - visao das lojas acessiveis, acknowledge e acoes operacionais limitadas
- `store_terminal`
  - visao operacional das lojas acessiveis e acknowledge basico
- `consultant`
  - sem workspace administrativa dedicado na primeira onda; recebe avisos contextuais no fluxo operacional

Regra de autorizacao:

- leitura e acao sempre respeitam escopo de loja acessivel
- regras tenant-wide nao podem vazar entre tenants
- a pagina pode ser ampla, mas o conteudo precisa ser filtrado por perfil e loja

## Contratos e endpoints esperados

O desenho fino da API ainda sera implementado, mas o modulo deve nascer mirando
algo proximo de:

- `GET /v1/alerts`
- `GET /v1/alerts/overview`
- `GET /v1/alerts/{alertId}`
- `PATCH /v1/alerts/rules`
- `POST /v1/alerts/{alertId}/acknowledge`
- `POST /v1/alerts/{alertId}/resolve`
- `POST /v1/alerts/{alertId}/dismiss`
- `POST /v1/alerts/{alertId}/execute/admin-close`

Endpoints futuros:

- `POST /v1/alerts/deliveries/test`
- `GET /v1/alerts/deliveries`
- `PATCH /v1/alerts/channels`

## Modelagem esperada

O modulo deve ser pensado em pelo menos quatro blocos:

### 1. Regras

Configuracoes tenant-wide de quando e como alertar.

### 2. Instancias de alerta

Registro materializado do incidente ou evento.

Campos esperados:

- `id`
- `tenantId`
- `storeId`
- `serviceId`, quando houver
- `consultantId`, quando houver
- `type`
- `category`
- `severity`
- `status`
- `sourceModule`
- `openedAt`
- `lastTriggeredAt`
- `resolvedAt`
- `metadata`

### 3. Acoes / auditoria

Cada acknowledge, resolucao, dismiss ou acao administrativa deve ficar auditado.

### 4. Entregas

Cada envio para UI, WhatsApp ou outro canal precisa ter trilha separada para
nao misturar estado do alerta com sucesso ou falha do canal.

## Estado atual planejado

Este modulo ainda vai nascer.

Decisoes ja fechadas antes da implementacao:

- sera um modulo proprio e desacoplado
- o sinal inicial vem de `operations`
- horario de funcionamento fica em `stores` / Multi-loja
- thresholds analiticos existentes nao entram automaticamente no novo modulo
- o primeiro canal obrigatorio e a propria plataforma com atualizacao em tempo real
- a arquitetura precisa nascer pronta para canais externos e eventos positivos

## Erros de arquitetura a evitar

### 1. Alertas virarem uma extensao informal de `settings`

Se a regra e sobre incidente operacional, canal, entrega, cooldown ou estado de
alerta, ela pertence a `alerts`.

### 2. Alertas recalcularem a Operacao inteira

Se o modulo duplicar logica de tempo de atendimento, fila ou ociosidade, o risco
de divergencia e alto.

### 3. Realtime virar fonte de verdade

WebSocket deve transportar invalidacao/evento leve. A leitura autoritativa tem
que continuar no backend.

### 4. Acoplar canal externo na primeira versao do schema

Nao modelar `whatsapp_message_id` direto na tabela principal do alerta.
Canal e tentativa de entrega precisam ser estruturas separadas.

### 5. Misturar alerta negativo com celebracao positiva sem categoria

O schema precisa carregar `category` ou equivalente desde o inicio.

### 6. Encerramento administrativo sobrescrever auditoria operacional

`alerts` pode coordenar a acao, mas o historico oficial do atendimento continua
em `operations`.

### 7. Colocar horario de funcionamento dentro de `alerts`

Horario pertence a loja. `alerts` so consome.

### 8. Abrir pagina para todos sem filtragem real por escopo

O modulo precisa respeitar tenant, loja e papel desde a primeira versao.

## Dependencias de futuro ja esperadas

Itens que combinam naturalmente com este modulo e devem ser considerados no desenho:

- 4.2 monitoramento de servicos e APIs
- 4.1 monitoramento da VPS
- 5.1 central de notificacoes
- 3.2 auditoria de usuarios online
- 3.3 acoes remotas em sessoes
- gamificacao comercial e operacional

Regra de roadmap:

o modulo deve nascer com nomenclatura ampla o suficiente para receber no futuro:

- alertas operacionais
- incidentes de infraestrutura
- notificacoes administrativas
- celebracoes e reconhecimentos

sem quebrar o contrato base.

## Contrato minimo para plugar em outro projeto

O service de `alerts` nao deve depender do projeto host inteiro.

Contratos minimos esperados:

- `AccessContext`
- `Repository`
- `SignalSource` ou callback/adapters de origem
- `DeliveryPublisher`
- `OperationsActionGateway`
- `StoreScheduleProvider`

Se o host nao quiser canais externos, o modulo precisa continuar funcionando com
delivery noop para tudo que nao for UI.

## Como validar mudancas neste modulo

Antes de considerar qualquer entrega pronta, validar pelo menos:

1. deduplicacao do mesmo alerta para o mesmo atendimento/contexto
2. acknowledge e resolve sem perder a trilha de auditoria
3. filtro por tenant e loja acessivel
4. fechamento administrativo chamando a Operacao sem duplicar historico
5. evento realtime leve e leitura autoritativa coerente
6. regra de horario da loja aplicada a partir do modulo `stores`
7. ausencia de regressao no fluxo operacional quando o modulo de alertas falhar

## Regra critica de resiliencia

Falha no modulo `alerts` nao pode derrubar:

- login
- bootstrap do painel
- leitura do snapshot operacional
- inicio ou encerramento de atendimento

Se uma entrega de alerta falhar:

- a Operacao continua funcionando
- a auditoria precisa registrar a falha do canal
- a UI nao pode quebrar o fluxo principal da loja

## Evolucao esperada

1. workspace dedicada de Alertas no painel
2. regras operacionais e auditoria administrativa
3. horario de funcionamento por loja consumido de Multi-loja
4. alertas em tempo real na plataforma
5. integracao com canais externos como WhatsApp
6. eventos positivos e gamificacao
7. unificacao futura com monitoramento e central de notificacoes sem reescrever a base

## Registro de implementacao (4.3 notificacao ao consultor)

### Campos novos em alert_instances (migration 0044)

- `interaction_kind varchar(30) default 'none'`: tipo de interacao esperada do usuario
  - `none`: passivo, apenas workspace admin
  - `reminder`: banner informativo sem resposta
  - `response_required`: banner com botoes de resposta (padrão para `long_open_service`)
- `interaction_response varchar(30)` nullable: resposta dada pelo usuario (`still_happening` | `forgotten`)
- `responded_at timestamptz` nullable: quando respondeu
- `external_notified_at timestamptz` nullable: placeholder para futura entrega externa (WhatsApp)

### Endpoint novo

`POST /v1/alerts/{id}/respond`
- Body: `{ "response": "still_happening" | "forgotten" }`
- Response: `{ "alert": AlertView, "openFinishModal": bool, "serviceId": string }`
- Permissao: todos os papeis operacionais incluindo `consultant`
- Quando `forgotten` → `openFinishModal=true`, frontend abre FinishModal

### Comportamento de interacao

- `long_open_service` nasce com `interaction_kind = 'response_required'`
- Frontend exibe banner persistente acima do workspace de operacao
- Consultor ve apenas alertas dos proprios atendimentos; manager/owner veem todos da loja
- Toast automatico (auto-dismiss 6s) quando alerta novo chega via WebSocket
- Destaque visual no card do consultor (borda amarela + badge)

### Estrutura de notificacao externa (placeholder)

- Flag `notifyExternal` nas regras ja existia; agora e lida por `RespondToAlert`
- Quando `notifyExternal=true`: chama `MarkExternalNotified` que persiste `external_notified_at` sem chamada real
- Integracao real com WhatsApp vai aqui na proxima onda

## Registro de implementacao (Fase 5-6: Regras dinâmicas e componentes de display)

### Migrações (0046, 0047, 0048)

**0046_alert_rule_definitions.sql**
- Cria tabela `alert_rule_definitions` com 21 campos para regras tenant-wide customizáveis
- Campos: `id`, `tenant_id`, `name`, `description`, `is_active`, `trigger_type` (5 tipos), `threshold_minutes`, `severity`, `display_kind` (6 tipos), `color_theme` (6 cores), `title_template`, `body_template`, `interaction_kind` (none/dismiss/confirm_choice/select_option), `response_options` (jsonb), `is_mandatory`, `notify_dashboard`, `notify_operation_context`, `notify_external`, `external_channel`, `created_by`, `updated_by`, `created_at`, `updated_at`
- Backfill: cria regra padrão `long_open_service` para cada tenant existente a partir de `tenant_operational_alert_rules`
- Índices: (tenant_id, is_active, trigger_type) e (trigger_type)

**0047_alert_instances_display_snapshot.sql**
- Adiciona 5 colunas a `alert_instances`: `rule_definition_id`, `display_kind`, `color_theme`, `response_options` (jsonb), `is_mandatory`
- Expande CHECK de `interaction_kind` para suportar 6 valores novos
- Backfill: alertas existentes `long_open_service` ganham display clássico (banner, amber, confirm_choice, 2 opções)
- Índice: (display_kind, store_id)

**0048_alert_instances_consultant_name.sql**
- Adiciona `consultant_name` (snapshot) a `alert_instances`
- Backfill: via join com `consultants` table
- Índice: (store_id, consultant_id, consultant_name)

### Model (model.go)

**Novas constantes:**
- 5 trigger types: long_open_service, long_queue_wait, long_pause, idle_store, outside_business_hours
- 6 display kinds: card_badge, banner, toast, corner_popup, center_modal, fullscreen
- 6 color themes: amber, red, blue, green, purple, slate
- 3 interaction kinds: dismiss, confirm_choice, select_option (none já existia)
- 3 external channels: whatsapp, email, none

**Novos structs:**
- `ResponseOption`: {value, label}
- `RuleDefinition`: 21 campos mapeando alert_rule_definitions
- `RuleDefinitionView`: JSON tags correspondentes com método View()
- `CreateRuleInput`, `UpdateRuleInput`, `ListRulesInput`: DTOs de CRUD
- `OperationalSignalInput` estendido com: `ConsultantName`, `ElapsedMinutes`, `TriggerType`

**Novos métodos em Repository interface:**
- `ListRules(ctx, input)`: filtros por tenantID, triggerType, onlyActive
- `GetRule(ctx, ruleID)`: detalhe de uma regra
- `CreateRule(ctx, input, actor)`: persiste nova regra
- `UpdateRule(ctx, ruleID, input, actor)`: atualização parcial
- `DeleteRule(ctx, ruleID)`: soft-delete lógico
- `LoadActiveRulesForTrigger(ctx, tenantID, triggerType)`: carrega regras ativas de um trigger (usado pelo scheduler)

**Utilitários:**
- `RenderTemplate(template, vars)`: substitui {consultant}, {elapsed}, {store}, {threshold} no título/corpo
- `FormatElapsed(duration)`: formata duração em "1h17min"
- `ElapsedMinutesSince(startTime)`: calcula minutos decorridos

### Store (store_postgres.go)

- `scanAlert` atualizado para ler 6 novas colunas + JSON unmarshaling de `response_options`
- Todos SELECT de `alert_instances` (List, GetByID, trigger path, dedup check) incluem os 6 novos campos
- **ListRules**: query com filtros, retorna array de regras
- **GetRule**: por ID, com tratamento de ErrNotFound
- **CreateRule**: serializa `response_options` em JSON, retorna regra criada
- **UpdateRule**: builder dinâmico que só persiste campos que mudam
- **DeleteRule**: delete simples (físico ou soft, dependendo de auditoria)
- **LoadActiveRulesForTrigger**: carrega regras ativas de um tipo de trigger para a Operação
- `scanRuleDefinition`: parser de rows para RuleDefinition

### Service (service.go)

**Novos métodos:**
- `ListRules(ctx, principal, input)`: filtra por tenant da sessão e aplica permissão `canManageAlertRules`
- `GetRule(ctx, principal, ruleID)`: lê uma regra, valida acesso ao tenant
- `CreateRule(ctx, principal, input)`: valida entrada (interaction_kind vs response_options, isMandatory), publica evento, retorna regra
- `UpdateRule(ctx, principal, ruleID, input)`: atualiza parcialmente, valida tipo de tenant
- `DeleteRule(ctx, principal, ruleID)`: remove regra, operação irreversível
- `ApplyRuleNow(ctx, principal, ruleID)`: retroatividade — carrega regra, chama `operationsScanner.ScanForRule`, processa sinais retornados
- `SetOperationsScanner(scanner)`: injeção de dependência para `OperationsScanner` interface

**Validações de negócio:**
- Quando `interaction_kind` é `confirm_choice` ou `select_option`: `response_options` deve ter ≥ 2 itens
- Quando `is_mandatory=true`: `interaction_kind` não pode ser `none`
- `threshold_minutes > 0`
- `display_kind`, `color_theme`, `trigger_type` ∈ lista permitida

**Permissão:**
- Todas as operações CRUD usam `canManageAlertRules` do `principal` acessível

### HTTP (http.go)

**Novos endpoints:**
- `GET /v1/alerts/rules` (query: tenantId, triggerType, onlyActive) → `rulesListResponse`
- `POST /v1/alerts/rules` (status 201) → `createRuleRequest` → `ruleResponse`
- `GET /v1/alerts/rules/{id}` → detalhe da regra
- `PATCH /v1/alerts/rules/{id}` → `updateRuleRequest` → `ruleResponse`
- `DELETE /v1/alerts/rules/{id}` (status 204)
- `POST /v1/alerts/rules/{id}/apply-now` → `applyRuleResponse` com `appliedCount`

**Structs de request/response:**
- `createRuleRequest`, `updateRuleRequest`: DTOs que mapeiam `CreateRuleInput` / `UpdateRuleInput`
- `ruleResponse`, `rulesListResponse`: marshalizam `RuleDefinitionView` para JSON
- `applyRuleResponse`: `{ appliedCount: int }`

### Operations integration

**Novos signal types em alerts.go:**
- `SignalLongQueueWaitTriggered` / `SignalLongQueueWaitResolved`
- `SignalLongPauseTriggered` / `SignalLongPauseResolved`
- `SignalIdleStoreTriggered` / `SignalIdleStoreResolved`
- `SignalOutsideBusinessHoursTriggered` / `SignalOutsideBusinessHoursResolved`

**Novos builders em operations/service.go (stubs para MVP):**
- `buildLongQueueWaitSignals()`: detecção de fila > threshold
- `buildLongPauseSignals()`: detecção de pausa > threshold
- `buildIdleStoreSignals()`: loja sem atividade em horário comercial
- `buildOutsideBusinessHoursSignals()`: atendimento fora do horário da loja

**OperationsScanner interface:**
- `ScanForRule(ctx, rule) interface{}`: retroatividade — retorna array de `OperationalSignalInput`
- Implementação em operations/service.go que carrega snapshot atual e roda builder específico do trigger

**Injeção em app.go:**
```go
alertsService.SetOperationsScanner(operationsService)
```

### Comportamento de materialização

Quando um sinal operacional (de qualquer trigger) chega ao alerts:
1. Carrega a regra ativa (`LoadActiveRulesForTrigger`) para o trigger type
2. Se encontrada, renderiza templates: `titleTemplate` e `bodyTemplate` substituindo `{consultant}`, `{elapsed}`, `{store}`, `{threshold}`
3. Cria `alert_instance` com snapshot da regra: `display_kind`, `color_theme`, `response_options`, `is_mandatory`
4. Se a regra for editada depois, alertas já criados mantêm seus valores de snapshot
5. Novos alertas usam os valores atualizados da regra

### Endpoint de retroatividade

`POST /v1/alerts/rules/{id}/apply-now`:
1. Valida permissão `canManageAlertRules`
2. Carrega a regra
3. Chama `operationsScanner.ScanForRule(ctx, rule)`
4. Processa sinais retornados e cria alertas
5. Retorna `{ appliedCount: N }`

Usado para feedback imediato após salvar/editar uma regra; não substitui o scheduler de 15s que continua como fonte primária.