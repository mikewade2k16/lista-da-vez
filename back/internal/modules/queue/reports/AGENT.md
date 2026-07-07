# AGENT

## Escopo

Estas instrucoes valem para `back/internal/modules/reports`.

## Responsabilidade do modulo

O modulo `reports` cuida das leituras analiticas e gerenciais derivadas do historico operacional.

Hoje ele deve responder por:

- agregacoes server-side para `/relatorios`
- leitura paginada de resultados
- visao de ultimos atendimentos
- overview multiloja para gestao administrativa
- fechamento correto de produtos fechados usando `productsClosed[]` como fonte de verdade

Hoje `/relatorios` no Nuxt ja consome:

- `GET /v1/reports/overview`
- `GET /v1/reports/results`
- `GET /v1/reports/recent-services`
- `GET /v1/reports/multistore-overview`
- `GET /v1/reports/pauses`

Ele nao deve cuidar de:

- mutacoes operacionais da fila
- configuracoes da loja
- autenticacao como fonte de verdade

## Contrato atual

- `GET /v1/reports/overview`
- `GET /v1/reports/results`
- `GET /v1/reports/recent-services`
- `GET /v1/reports/multistore-overview`
- `GET /v1/reports/pauses`

## Regras de arquitetura

- relatorio nao deve depender de campo legado escalar quando existir colecao estruturada mais confiavel
- para produtos fechados, usar primeiro `productsClosed[]`; `productClosed` fica como fallback de compatibilidade
- endpoints de leitura volumosa devem ser separados por caso de uso
- `overview` entrega agregados
- `results` entrega linhas paginadas
- `recent-services` entrega leitura administrativa dos ultimos atendimentos
- `multistore-overview` entrega comparativo por loja com metricas historicas e contadores vivos da operacao
- `pauses` agrega `queue.operation_status_sessions` (`status='paused'`, `kind` pause; `assignment`/tarefa fica de fora) por consultor/motivo/hora: `summary`, `byConsultant` (com `byReason`), `byReason`, `byHour` (hora UTC) e `rows` (timestamps crus para o front formatar em hora local). Reusa o mesmo escopo/permissao (`canViewReports`) e a resolucao de loja/tenant dos demais relatorios; filtra a janela por `ended_at`
- backend deve filtrar por `store_id` e acesso do usuario antes de qualquer agregacao
- quando `storeId` for omitido, leituras agregadas devem atravessar apenas as lojas acessiveis da sessao dentro do tenant resolvido

## Teto de leitura do historico (`?limit=` + `historyWindow`)

- `listHistoryQuery` aplica `LIMIT` no SQL. O teto default e' `2000`
  (`defaultHistoryFetchLimit`); o cliente pode pedir outro via `?limit=`, clampado
  em `5000` (`maxHistoryFetchLimit`). `?limit=` invalido (nao-inteiro) -> `400`
  `validation_error`, mesmo padrao de `page`/`pageSize`.
- A ordenacao `finished_at desc, created_at desc` garante que o truncamento
  preserva os atendimentos MAIS RECENTES (indice `0007 (store_id, finished_at desc)`).
- Os filtros dinamicos do SQL vivem em `appendHistoryFilters`
  (`store_postgres_filters.go`), compartilhado entre `listHistoryQuery` e
  `CountHistory` para nunca divergirem (placeholders numerados por `len(args)+1`).
- Os 4 responses (`overview`, `results`, `recent-services`, `multistore-overview`)
  carregam `historyWindow`: `{ limit, fetched, total, truncated }`.
  - `fetched` = linhas que o SQL trouxe (apos LIMIT, ANTES dos filtros em memoria).
  - `total` = `count(*)` com os MESMOS filtros SQL (sem os filtros em memoria como
    `search`/`sourceIds`/`campaignIds`, que rodam depois no Go).
  - `truncated=true` quando `total > fetched`.
  - O `CountHistory` SO roda quando a janela bateu no teto (`fetched >= limit`);
    abaixo do teto, `total == fetched` e nenhum count extra e' executado.
- O front (`reports.ts`) le por campos nomeados e ignora `historyWindow`/`filters.limit`;
  o comportamento so muda quando o historico filtrado por SQL passa do teto — e o
  corte fica explicito no metadado.
- Fora de escopo (follow-up): mover os filtros em memoria (`search`, `sourceIds`,
  `visitReasonIds`, `completionLevels`, `campaignIds`) para SQL; cursor/keyset real
  (`finished_at < $cursor`) se o teto de 2000 apertar.

## Regras de payload

- nao devolver bundles gigantes quando o caso de uso for um card, tabela ou lista especifica
- preferir:
  - agregados pequenos para dashboards
  - linhas paginadas para tabelas
  - filtros claros e previsiveis em query string
- formatacao de moeda, percentuais e labels visuais deve continuar no frontend quando nao for necessaria no contrato

## Evolucao esperada

1. filtros por campanha com semantica final alinhada ao futuro modulo `campaigns`
2. exportacao server-side
3. cache e materializacao seletiva quando o volume crescer

## Direcao de plugabilidade

Este modulo faz parte do core reutilizavel do painel.

Dependencias reais dele:

- contexto de acesso
- escopo de lojas acessiveis
- historico operacional

Direcao arquitetural:

- alinhar o service ao mesmo contrato `AccessContext + StoreScopeProvider` do modulo `operations`
- nao depender do modulo completo de auth como unica forma de uso
