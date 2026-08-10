# Planning module

## Responsabilidade

Mantem no PostgreSQL a configuracao de planejamento por loja e a escala de cada semana.

## Invariantes

- Toda leitura e gravacao repete `tenant_id` e `store_id`.
- Leitura exige `workspace.planejamento.view` e escrita exige `workspace.planejamento.edit`; permissoes de Multi-loja nao substituem essas chaves.
- A loja sempre e resolvida por `stores.Service.FindAccessible` antes do repositorio.
- Configuracao e um agregado JSON versionado por loja; contratos de jornada ficam normalizados e sao gravados na mesma transacao.
- Escala, rateio da semana ativa, metas semanais dos consultores e consolidacao mensal
  (`week=0`) sao gravados na mesma transacao. A meta mensal individual usa a participacao
  acumulada das semanas geradas e sempre fecha o total mensal da loja; ao gerar novas semanas,
  a consolidacao e recalculada.
- A primeira escala gerada no mes tambem preenche semanas individuais ainda ausentes ou zeradas,
  usando a participacao calculada nessa escala e a meta de cada semana da loja. Semanas que ja
  possuem rateio positivo sao preservadas e passam a ser refinadas quando sua propria escala for gerada.
- IDs de funcionarios presentes na escala devem pertencer a loja e ao tenant.
- `week_start` e sempre uma segunda-feira; turnos ficam dentro dos sete dias da semana.
- As semanas comerciais usam as ancoras dos dias 1, 8, 15, 22 e, quando o mes
  possui ao menos 29 dias, 29. O total e calculado automaticamente: quatro
  periodos em meses de 28 dias e cinco nos demais. As ancoras sao normalizadas
  para a segunda-feira anterior. Assim, a semana 1 pode comecar no
  mes anterior sem deixar de pertencer ao mes selecionado.
- Estados persistidos sao `saved` e `published`; nao existe rascunho somente local.
- Escala publicada e somente leitura ate reabertura explicita; toda escrita usa versao esperada para evitar sobrescrita concorrente.
- O agregado de configuracao guarda cobertura minima por tipo de loja, feriados,
  ausencias por data e regras individuais de domingos/feriados; o motor Go aplica
  essas regras tanto na geracao quanto na validacao.
- Toda alteracao de configuracao/escala publica `context.updated` com recurso
  `planning`; clientes reidratam o snapshot autoritativo.
- O GET inclui ate 30 revisoes da semana, com versao, status, autor e horario.

## API

- `GET /v1/operations/planning?storeId=&weekStart=`
- `PUT /v1/operations/planning/configuration`
- `PUT /v1/operations/planning/schedule`
- `POST /v1/operations/planning/schedule/generate`
- `POST /v1/operations/planning/schedule/reopen`

Geração automática, validação e rateio das metas individuais são executados no Go.
O frontend somente envia a configuração/semana e renderiza a escala, os alertas e
as alocações persistidas devolvidas pela API.

Salvar ou gerar escala também publica `operationgoal:generated`, para a grade de Metas
reidratar as linhas mensais e semanais criadas pelo planejamento.

As rotas ficam sob `/v1/operations` para herdar o gate do modulo `queue`.
