# AGENT

## Escopo

Estas instrucoes valem para `back/internal/modules/settings`.

## Responsabilidade do modulo

O modulo `settings` cuida do pacote configuravel da operacao por loja.

Hoje ele deve responder por:

- bundle de settings consumido pelo Nuxt
- modal config
- catalogos de motivos/origens/perdas/profissoes
- catalogo de produtos
- selecao de template operacional

Ele nao deve cuidar de:

- fila e atendimento
- auth
- campanhas
- relatorios server-side

## Contrato atual

- `GET /v1/settings?storeId=...`
- `PUT /v1/settings`
- `PATCH /v1/settings/operation`
- `PATCH /v1/settings/modal`
- `POST /v1/settings/options/{group}`
- `PATCH /v1/settings/options/{group}/{itemId}`
- `DELETE /v1/settings/options/{group}/{itemId}`
- `PUT /v1/settings/options/{group}`
- `POST /v1/settings/products`
- `PATCH /v1/settings/products/{itemId}`
- `DELETE /v1/settings/products/{itemId}`
- `PUT /v1/settings/products`

## Regras de escopo

- leitura: qualquer usuario com acesso a loja
- escrita: `owner` e `platform_admin`

## Regra de persistencia

- os catalogos e configuracoes desta fase vivem em tabelas normalizadas:
  - `store_operation_settings`
  - `store_setting_options`
  - `store_catalog_products`
- templates operacionais continuam versionados no codigo do backend
- o `GET /v1/settings` continua entregando um bundle para o Nuxt por conveniencia de leitura
- a API de escrita deve preferir endpoints por secao em vez de trafegar o bundle inteiro a cada alteracao
- em listas e catalogos, a escrita deve preferir mutacao por item em vez de substituir a colecao inteira
- em `PATCH /operation` e `PATCH /modal`, a UI deve enviar apenas os campos alterados; o backend aplica merge sobre o estado atual
- campos opcionais/default nao devem ser enviados sem necessidade; ausencia deve ser tratada como "manter valor atual" em patch parcial
- endpoints `PUT` de secoes/listas ficam reservados para bulk replace intencional, importacao ou aplicacao de template
