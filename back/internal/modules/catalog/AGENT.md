# AGENT

## Escopo

Estas instrucoes valem para `back/internal/modules/catalog`.

## Responsabilidade do modulo

O modulo `catalog` sera a fronteira unica para busca operacional de produtos.

Ele existe para desacoplar a Operacao da origem fisica do catalogo. A UI e os
modulos consumidores nao devem saber se o produto veio de:

- `erp_item_current`
- uma tabela interna futura como `products`
- outra projecao dedicada para atendimento

Hoje o primeiro provider previsto e `erp_current`, lendo `erp_item_current`.
No futuro, a troca para `internal_products` deve acontecer sem reescrever o
fluxo do modal de encerramento, do picker ou do frontend consumidor.

Importante no estado atual:

- a importacao ERP validada hoje existe apenas na loja `184`
- por isso, a source `erp_current` funciona como catalogo compartilhado por tenant
- a `storeId` enviada pelo frontend continua obrigatoria, mas serve para validar
  o contexto/acesso da sessao
- a consulta fisica em `erp_item_current` nao fica presa a `store_id` nesta
  source; ela deduplica por `sku` dentro do tenant

## Objetivo de arquitetura

Este modulo deve entregar um contrato estavel de busca, com shape normalizado:

- `id`
- `code`
- `name`
- `price`

Para a Operacao atual, isso e suficiente para:

- identificar o item selecionado
- exibir nome e codigo no picker
- somar valor final em compra, reservado ou interesse

Regra atual para `erp_current`:

- `id` = `sku`
- `code` = `sku`
- `name` = `name`
- `price` = `price_cents / 100`

Convencao atual de `price`:

- o ERP persiste `price_cents` em centavos
- o contrato do `catalog` devolve `price` no mesmo formato numerico ja usado hoje pela Operacao
- exemplo: `348800` no ERP vira `3488` no `catalog`, que a UI renderiza como `R$ 3.488,00`

O campo `identifier` existe no ERP, mas nao sobe para a Operacao neste momento.
Se no futuro a fonte trocar para uma tabela propria como `products`, o adapter da
fonte continua responsavel por preencher esse mesmo contrato canonico.

Regra principal:

- nomes de tabela e coluna nunca sobem para o frontend
- o frontend escolhe uma `sourceKey` conhecida, nao uma tabela arbitraria
- o backend resolve o provider e mapeia os campos para o shape canonico

## Fronteiras

Este modulo deve cuidar de:

- busca de produtos por prefixo de codigo
- padronizacao de resposta para a Operacao
- registry de providers/fontes de catalogo
- adaptacao entre schema fisico e contrato canonico

Este modulo nao deve cuidar de:

- configuracoes de modal
- CRUD administrativo de catalogo manual
- fila operacional
- fechamento de atendimento
- ingestao ERP

## Contrato esperado

Primeiro caso de uso previsto:

- busca operacional para o modal de encerramento
- consulta com prefixo de codigo, ex.: primeiros 3 caracteres do SKU
- retorno enxuto para autocomplete/picker

Contrato HTTP atual:

- `GET /v1/catalog/products/search`

Parametros:

- `storeId`
- `term`
- `limit`
- `sourceKey`

Regras:

- o frontend informa a `storeId` do contexto operacional
- o backend resolve a loja acessivel a partir da sessao autenticada
- a source pode decidir se a leitura fisica e `store-scoped` ou
  `tenant-shared`, sem mudar o contrato HTTP
- o frontend nunca informa tabela, coluna ou `tenantId` fisico para esta busca
- `term` deve ter pelo menos 3 caracteres
- `sourceKey` vazio cai em `erp_current`

## Shape preferido

- `model.go`
- `errors.go`
- `service.go`
- `http.go`
- `repository_*.go`

## Regras de evolucao

- a primeira implementacao deve usar whitelist de providers conhecidos
- nao aceitar nome de tabela/coluna livre vindo do client
- toda nova fonte deve entrar por adapter pequeno e testavel
- a Operacao deve depender do contrato deste modulo, nao do schema ERP
- no source `erp_current`, o codigo canonico da Operacao deve ser `sku`
