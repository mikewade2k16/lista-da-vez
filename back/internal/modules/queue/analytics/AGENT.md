# AGENT

## Escopo

Estas instrucoes valem para `back/internal/modules/analytics`.

## Responsabilidade do modulo

O modulo `analytics` entrega leituras gerenciais prontas para o frontend, evitando que `ranking`, `dados` e `inteligencia` recalcularem historico bruto no browser.

Hoje ele deve responder por:

- ranking mensal e diario por consultor
- alertas de desempenho por consultor
- agregados de produtos, motivos, origens, profissoes e horarios
- inteligencia operacional com diagnosticos e acoes recomendadas

Ele nao deve cuidar de:

- comandos operacionais da fila
- CRUD de configuracao
- autenticacao

## Contrato atual

- `GET /v1/analytics/ranking?storeId=...`
- `GET /v1/analytics/data?storeId=...`
- `GET /v1/analytics/intelligence?storeId=...`
- os mesmos endpoints tambem aceitam `tenantId` para consolidar todas as lojas acessiveis da sessao dentro do tenant ativo

## Regras

- analytics deve consumir a fonte persistida do backend, nunca runtime local do frontend
- o frontend deve receber payloads prontos para renderizar, nao historico bruto para recalcular tudo
- respostas devem ser pequenas e orientadas ao caso de uso da tela
- toda leitura deve respeitar escopo autenticado de loja/tenant
- quando `tenantId` subir sem `storeId`, a agregacao deve atravessar apenas as lojas acessiveis da sessao, sem depender da workspace administrativa `multiloja`
- se uma tela precisar de outro agregado, preferir abrir um endpoint especifico antes de devolver bundles genericos

## Direcao de plugabilidade

Este modulo faz parte do core reutilizavel do painel.

Dependencias reais dele:

- contexto de acesso
- validacao de loja acessivel
- snapshot operacional
- roster
- settings por loja

Direcao arquitetural:

- alinhar o service ao mesmo contrato `AccessContext + StoreScopeProvider` do modulo `operations`
- manter a leitura analitica desacoplada do modulo concreto de auth do projeto host
