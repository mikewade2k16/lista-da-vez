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
- ao agregar por tenant, uma loja sem dado (ex.: sem linha em `store_operation_settings`) NAO pode derrubar o tenant inteiro: `LoadSettings` trata `pgx.ErrNoRows` como defaults, em vez de propagar erro (senao vira 500 em `/data`). Backfill historico em migration `0164`.
- o `default` de `writeServiceError` (500) deve logar o erro real (`request_id` + path); 500 cego ja escondeu causa-raiz antes
- se uma tela precisar de outro agregado, preferir abrir um endpoint especifico antes de devolver bundles genericos

## Janela de carga do historico (default 90 dias)

- O historico bruto e' carregado via `Repository.LoadSnapshotWithHistorySince`
  (nunca mais o `LoadSnapshot` sem janela): so agregamos, entao carregar todo o
  historico da loja a cada request era o gargalo (N lojas x historico completo em
  memoria no escopo tenant).
- A janela default e' `90` dias (`defaultHistoryWindowDays`), calculada por
  `historySinceMillis(dateFrom, now)` em `helpers.go`:
  - `ranking` passa o `dateFrom` da query; se o `dateFrom` explicito for MAIS
    ANTIGO que a janela de 90 dias, a janela recua para respeita-lo (range antigo
    do ranking nao pode perder dados).
  - `data` e `intelligence` passam `dateFrom=""` -> sempre 90 dias. Decisao de
    produto: essas 2 telas passam a agregar sobre os ultimos 90 dias (cobre mes
    corrente + 2 anteriores). O front ja trabalha com mentalidade de mes corrente
    e essas 2 rotas nem parseiam `dateFrom/dateTo` hoje.
- Follow-up (fora deste corte): parsear `dateFrom/dateTo` tambem em
  `/v1/analytics/data|intelligence` (o front ja envia; hoje sao ignorados).

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
