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

Ele nao deve cuidar de:

- mutacoes operacionais da fila
- configuracoes da loja
- autenticacao como fonte de verdade

## Contrato atual

- `GET /v1/reports/overview`
- `GET /v1/reports/results`
- `GET /v1/reports/recent-services`
- `GET /v1/reports/multistore-overview`

## Regras de arquitetura

- relatorio nao deve depender de campo legado escalar quando existir colecao estruturada mais confiavel
- para produtos fechados, usar primeiro `productsClosed[]`; `productClosed` fica como fallback de compatibilidade
- endpoints de leitura volumosa devem ser separados por caso de uso
- `overview` entrega agregados
- `results` entrega linhas paginadas
- `recent-services` entrega leitura administrativa dos ultimos atendimentos
- `multistore-overview` entrega comparativo por loja com metricas historicas e contadores vivos da operacao
- backend deve filtrar por `store_id` e acesso do usuario antes de qualquer agregacao
- quando `storeId` for omitido, leituras agregadas devem atravessar apenas as lojas acessiveis da sessao dentro do tenant resolvido

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
