# ERP CRM Store Attribution

## Objetivo

Documentar de forma simples como a CRM deve separar vendas por loja comercial
(Jardins, Riomar, Garcia e 13 de Julho/Treze) usando os dados crus do ERP.

## Problema real do ERP

O ERP nao entrega a loja comercial de forma consistente em um unico campo.

Principais efeitos observados:

- `store_cnpj` aparece quase sempre como `12583959000186`, que puxa tudo para Riomar se usado sozinho.
- `store_id_raw` e a melhor chave comercial quando vem preenchido.
- muitos pedidos chegam sem `store_id_raw`.
- alguns vendedores existem no cadastro interno, mas com `employee_code` diferente do `employee_id` vindo no ERP.

Exemplo real:

- ERP `employee_id=231` -> `DIANA NICORY GOMES`
- cadastro interno -> `employee_code=321`, loja `GAR`

## CNPJs comerciais confirmados

- `12583959000186` -> Riomar
- `56173889000163` -> Jardins
- `53578278000107` -> Garcia
- `43068099000176` -> 13 de Julho / Treze

Observacao:

- `24291381000173` apareceu em cliente/consulta, nao em pedidos. Ele nao resolve a
  atribuicao principal de venda por loja.

## Regra simples de calculo

Para cada pedido ativo no periodo:

1. Agrupar linhas por `order_id`.
2. Calcular o valor do pedido:
   - usar `max(total_amount_cents)` quando existir.
   - se nao existir, usar `sum(amount_cents)`.
3. Calcular produtos e unidades:
   - `product_sales_cents = sum(amount_cents)`
   - `units = sum(quantity)` usando `1` quando `quantity <= 0`
4. Resolver a loja comercial nesta ordem:
   - `store_id_raw` do pedido, quando preenchido
  - override especial por vendedor ERP quando a pessoa atua em multi-loja
  - loja do vendedor no cadastro interno: `orders.employee_id` -> `users.employee_code` -> `consultants` ou `user_store_roles` -> `stores`
  - loja dominante do historico ERP do vendedor, olhando pedidos antigos com `store_id_raw` preenchido
   - `store_cnpj` como ultimo fallback
5. Agregar os pedidos resolvidos por loja e por consultor.

## Formulas das metricas

- `salesCents = sum(order_total_cents)`
- `orders = count(pedidos)`
- `units = sum(units)`
- `ticketAverageCents = salesCents / orders`
- `valuePerProductCents = productSalesCents / units`
- `paScore = units / orders`

## Cruzamento simples com o cadastro interno

O cruzamento que tende a ser mais robusto e:

1. `erp_order_raw.employee_id`
2. `users.employee_code`
3. `consultants.store_id` ou `user_store_roles.store_id`
4. `stores.code` / `stores.name`
5. mapa canonico para CNPJ comercial

Em termos praticos:

- `RIO` -> `12583959000186`
- `JAR` -> `56173889000163`
- `GAR` -> `53578278000107`
- `TRE` -> `43068099000176`

## Casos que ainda precisam de confirmacao manual

Casos com maior impacto e ainda sem chave interna perfeita:

- `ANDRE FILIPE CUNHA ALMEIDA` (ERP `employee_id=16`)
  - tratado como `Gerencia / Multi-loja` quando o pedido nao traz loja comercial explicita
  - se o pedido vier com `store_id_raw`, a venda continua indo para a loja explicita do ERP
- `ERP 15` (sem nome resolvido no cadastro interno)
  - fallback atual: Jardins pelo historico ERP

Casos que ja ficaram coerentes por cadastro interno ou por historico ERP:

- `ROSELI DE ANDRADE PAIXAO` -> Garcia
- `DIANA NICORY GOMES` -> Garcia
- `DIANA NICORY GOMES` mudou de loja ao longo do tempo; cadastro atual ajuda, mas nao deve sobrepor a evidencia historica do ERP quando ela existir
- `DAYANNE BARBOSA DE SOUZA MATOS` -> Jardins
- `TAUVANI MISSIELLY OLIVEIRA` -> Jardins
- `EVERLAND ALVES DOS SANTOS` -> Jardins
- `RAYANE TAVARES SANTOS ARAUJO` -> Riomar
- `DANIELLA DE MORAIS OLIVEIRA` -> Riomar
- `RITA DAMARIS MELO DA SILVA` -> Treze

## Recomendacao para bater o numero com consistencia

Se o objetivo for estabilidade operacional, a forma mais simples e segura de
cruzar esses dados e manter uma tabela de override por vendedor ERP:

- chave: `tenant_id + erp_employee_id`
- valor: loja comercial (`RIO`, `JAR`, `GAR`, `TRE`)

Fluxo sugerido:

1. tentar `store_id_raw`
2. tentar override manual por `erp_employee_id`
3. tentar cadastro interno por `employee_code`
4. usar historico ERP do vendedor
5. cair em `store_cnpj` so como ultimo recurso

Isso reduz bastante a fragilidade do ERP cru e deixa a regra auditavel.