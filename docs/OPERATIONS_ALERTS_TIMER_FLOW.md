# Operations x Alerts Timer Flow

## Objetivo

Registrar o contrato atual entre `operations` e `alerts` para os alertas operacionais baseados em tempo, principalmente `long_open_service`.

Este documento consolida a regra alinhada em produto e implementacao:

- `operations` e a fonte de verdade do estado operacional
- `operations` e a fonte de verdade do timer que decide quando um alerta temporal vence
- `alerts` nao calcula o tempo por conta propria
- `alerts` recebe sinais de `operations` e materializa a instancia do alerta

Resumo curto:

- `operations` detecta
- `operations` dispara o sinal
- `alerts` gera, atualiza ou resolve o alerta
- `realtime` so transporta invalidacao para a UI

## Regra arquitetural

Para alertas temporais operacionais, o modulo `alerts` nao deve ser o dono do cronometro.

O estado necessario para saber se um alerta venceu ja pertence a `operations`:

- atendimento ativo
- horario de inicio do atendimento
- estado de parada do atendimento
- finalizacao do atendimento
- cancelamento do atendimento
- loja e consultor envolvidos

O threshold configuravel pertence ao workspace/modulo de `alerts`, mas a avaliacao temporal pertence a `operations`.

Fluxo correto:

1. o usuario configura o threshold no modulo `alerts`
2. `operations` le esse threshold pelo contrato `LoadOperationalRules`
3. `operations` acompanha os atendimentos ativos e o vencimento do tempo
4. quando o limite vence, `operations` emite `long_open_service.triggered`
5. `alerts` recebe o sinal e abre ou atualiza a instancia deduplicada
6. quando o atendimento e finalizado, cancelado ou parado, `operations` emite `long_open_service.resolved`
7. `alerts` resolve a instancia aberta daquele `serviceId`

## Implementacao atual

Hoje a reavaliacao temporal roda dentro do backend, no modulo `operations`, por uma varredura periodica de atendimentos ativos.

Pontos principais:

- `operations.Service.ProcessTimedAlerts` percorre lojas com atendimento ativo
- o service carrega o threshold atual via `AlertCoordinator.LoadOperationalRules`
- o service so considera atendimentos realmente monitoraveis para `long_open_service`
- atendimento com `StoppedAt > 0` sai do monitoramento temporal
- sinais sao enviados para `alerts` via `AlertCoordinator.ReceiveOperationalSignals`

Arquivos chave:

- `back/internal/modules/operations/service.go`
- `back/internal/modules/operations/alerts.go`
- `back/internal/modules/alerts/service.go`
- `back/internal/modules/alerts/store_postgres.go`
- `back/internal/platform/app/app.go`

## Contrato do frontend

No frontend, os timers da operacao e os alertas tambem precisam respeitar a mesma fronteira de responsabilidade.

Regra:

- o frontend nao decide vencimento de `long_open_service`
- o frontend nao abre alerta por passar tempo localmente
- o frontend apenas exibe o cronometro visual da operacao
- o frontend apenas reage aos dados autoritativos e aos eventos leves de invalidacao

Fluxo atual da UI:

1. os cards e boards da operacao exibem cronometros locais para leitura visual do atendimento em aberto
2. `operations` continua sendo a fonte autoritativa do tempo no backend
3. quando `operations` materializa ou resolve um alerta via `alerts`, o backend publica `context.updated`
4. `web/app/composables/useContextRealtime.ts` recebe a invalidacao com `resource=alerts`
5. `web/app/stores/alerts.ts` faz refetch de lista e regras via `refreshRealtimeState()`
6. a workspace de alertas re-renderiza o estado materializado pela API

Consequencias praticas:

- `setInterval` dos componentes de operacao nao deve ser reutilizado para disparar alerta
- nenhum componente Vue deve comparar `Date.now()` com threshold de alerta para abrir instancia local
- a workspace `alertas` continua sendo leitura e acao sobre instancias ja materializadas, nao um motor autonomo de temporizacao
- se a estrategia interna do backend mudar no futuro, a UI continua igual: refetch e render dos dados autoritativos

## Resolucao por tipo de evento

### Finish

Quando o atendimento e finalizado de verdade, `operations` grava `ServiceHistoryEntry` e emite `long_open_service.resolved` com base no `serviceId` encerrado.

### Cancel

Quando o atendimento e cancelado:

- o consultor pode voltar para a fila
- o atendimento sai de `ActiveServices`
- `operations` emite `long_open_service.resolved` para o mesmo `serviceId`
- `alerts` resolve qualquer alerta aberto daquele atendimento

### Stop

Quando o atendimento e parado:

- o atendimento continua aparecendo no snapshot como parado
- `StoppedAt` e `StopReason` ficam no estado operacional
- esse atendimento deixa de contar para o timer de `long_open_service`
- `operations` emite `long_open_service.resolved` para o mesmo `serviceId`
- `alerts` resolve qualquer alerta aberto daquele atendimento

Isso evita um bug importante: atendimento parado nao pode continuar sendo reavaliado como se ainda estivesse correndo normalmente no cronometro de alerta.

## Responsabilidades por modulo

### Operations

Responsavel por:

- ler o threshold operacional configurado em `alerts`
- manter o timer autoritativo dos atendimentos
- decidir quando a condicao temporal venceu
- emitir sinais `triggered` e `resolved`
- garantir que `cancel`, `stop` e `finish` reflitam no ciclo de vida do alerta

Nao deve delegar para `alerts`:

- controle de cronometro
- calculo de tempo decorrido do atendimento
- decisao primaria de vencimento da condicao operacional

### Alerts

Responsavel por:

- persistir regras de alerta
- receber sinais de `operations`
- deduplicar por contexto e `serviceId`
- abrir, atualizar, reconhecer e resolver alertas
- auditar acoes
- publicar invalidacao para a UI e preparar entregas futuras

Nao deve assumir:

- timer proprio para alertas operacionais baseados em atendimento
- reavaliacao autonoma do tempo do atendimento
- ownership do estado da fila/atendimento

## Regra de evolucao futura

Se no futuro a implementacao do timer deixar de ser uma varredura periodica e passar para um scheduler interno mais sofisticado por atendimento, o contrato nao muda:

- `operations` continua detectando
- `operations` continua emitindo o sinal
- `alerts` continua materializando o alerta

Ou seja, a fronteira entre os modulos deve permanecer a mesma mesmo se o mecanismo interno de temporizacao evoluir.

## Checklist de manutencao

Sempre que mexer nessa integracao, validar:

1. `long_open_service.triggered` continua saindo apenas de `operations`
2. `finish`, `cancel` e `stop` resolvem o alerta do `serviceId` correto
3. atendimento parado nao continua sendo considerado pelo timer temporal
4. `alerts` nao ganhou cronometro proprio por atalho de implementacao
5. a UI continua apenas reagindo a invalidacao e refetch dos dados autoritativos
6. os cronometros visuais da operacao continuam sendo apenas display, sem materializar alerta por conta propria