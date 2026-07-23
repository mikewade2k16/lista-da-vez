# Relatório de lacunas — API Pérola × ERP atual

Data da análise: 23/07/2026

## Objetivo

Determinar se as seis entidades confirmadas na API Pérola conseguem substituir
os arquivos do ERP atualmente importados pelo Omni e listar, de forma objetiva,
o que precisa ser solicitado ao fornecedor da API.

Fontes usadas:

- contrato observado da API Pérola, documentado em
  `back/internal/modules/bi/AGENT.md`;
- [documentação oficial da API](https://documenter.getpostman.com/view/18113175/2s9YXh6iRz);
- schema PostgreSQL real do ambiente local;
- payloads originais preservados em `raw_payload` pelo importador ERP;
- DTOs, parsers e agregações do módulo `back/internal/modules/crm/erp`.

Importante: o contrato da Pérola foi observado com uma amostra controlada de
um registro por entidade. Ele ainda não deve ser tratado como schema formal e
versionado do fornecedor.

Limitação desta entrega: a configuração Pérola do container local estava vazia
na validação final da Fase 3. Por isso, a nova rota tipada foi validada com
servidor HTTP controlado e com o contrato real já observado na Fase 1, mas não
foi executado um novo smoke contra o ambiente externo nesta data.

## Conclusão executiva

**A API Pérola, no contrato atual, ainda não substitui integralmente o ERP.**

Ela já cobre melhor que o ERP atual:

- notas e itens fiscais;
- impostos, descontos, frete, seguro e devoluções fiscais;
- custo, preço de entrada e preço médio;
- atributos técnicos detalhados dos produtos;
- metadados de imagens;
- movimentos de inventário.

Porém faltam dados essenciais para preservar as funções atuais do Omni:

1. identidade comercial estável do produto — SKU, nome, descrição e ligação
   entre `itemSaldoId` e Item/SKU;
2. identidade comercial do pedido — ligação confirmada entre pedido ERP e
   Nota Pérola;
3. forma de pagamento;
4. base/estado explícito de pedidos cancelados;
5. valores comerciais de exclusão, débito e devolução usados pelo ERP;
6. contatos e atributos de CRM do cliente;
7. cadastro mestre do funcionário, situação ativa e vínculo com loja;
8. saldo atual de estoque por produto/loja, se a substituição também eliminar
   o arquivo de Item;
9. filtros incrementais confiáveis por alteração/período para sincronização.

Sem esses itens, retirar o ERP causaria perda funcional em CRM, campanhas,
filtros de compras, cancelamentos, metas por vendedor, catálogo e conciliação.

## Dimensão real do ERP hoje

Contagens observadas no PostgreSQL local:

| Dataset ERP | Registros/linhas atuais | Observação |
| --- | ---: | --- |
| Produto atual | 360.686 | projeção corrente por loja/produto |
| Cliente bruto | 348.802 | pode conter versões por lote |
| Funcionário bruto | 21.476 | pode conter versões por lote |
| Pedido bruto | 775.822 | linhas de itens, não pedidos únicos |
| Pedido cancelado bruto | 44.952 | linhas de itens cancelados |

Cobertura de campos críticos observada:

| Campo | Linhas preenchidas |
| --- | ---: |
| Produto com SKU | 360.686 |
| Produto com nome | 360.686 |
| Produto com descrição | 32.431 |
| Produto com preço comercial | 360.686 |
| Cliente com e-mail | 199.068 |
| Cliente com telefone | 138.385 |
| Cliente com celular | 283.878 |
| Cliente com tags | 241.467 |
| Pedido com forma de pagamento | 775.822 |
| Pedido com SKU | 775.822 |
| Cancelado com forma de pagamento | 44.952 |
| Cancelado com SKU | 44.952 |

Esses números demonstram que os campos ausentes não são apenas colunas
legadas vazias: vários deles sustentam centenas de milhares de registros.

## Contrato confirmado na API Pérola

Foram confirmadas seis entidades, totalizando 154 campos observados:

| Entidade | Endpoint | Campos observados | Principal conteúdo |
| --- | --- | ---: | --- |
| Item | `/item/find` | 52 | classificação e atributos técnicos |
| Imagem Item | `/imagemItem/find` | 4 | `itemId`, arquivo e ordem |
| Item Saldo Preço Compra | `/itemSaldoPrecoCompra/find` | 11 | custo, entrada e preço médio |
| Nota | `/nota/find` | 52 | documento fiscal, cliente, empresa e impostos |
| Nota Item | `/notaItem/find` | 23 | quantidade, preço, custo, desconto e devolução |
| Inventário | `/inventario/find` | 12 | movimentos por `itemSaldoId` e empresa |

## Matriz de lacunas por domínio

### 1. Produto e catálogo

O ERP atual entrega:

- `sku`;
- `identifier`;
- `name`;
- `description`;
- `supplierreference`;
- `brandname`;
- `seasonname`;
- `category1`, `category2`, `category3`;
- `size`;
- `color`;
- `unit`;
- preço comercial;
- datas de criação e atualização.

A API Pérola entrega atributos mais ricos, como marca, coleção, material,
pedras, acabamento, movimento, vidro, visor, formato, cor e tamanho. Entretanto,
na amostra observada não foram encontrados:

- nome comercial do produto;
- descrição comercial;
- SKU explicitamente identificado;
- preço comercial/de venda do cadastro;
- ligação direta de Item com `itemSaldoId`.

`referencia` pode eventualmente corresponder a SKU ou referência do fornecedor,
mas isso **não foi confirmado** e não pode ser assumido.

Solicitar:

- `sku`/código comercial estável em Item;
- `nome` e `descricao`;
- `precoVenda` ou tabela de preço comercial aplicável;
- `itemSaldoId` ou endpoint de Item Saldo que devolva simultaneamente
  `itemId`, SKU, empresa/loja e saldo atual;
- confirmação formal se `referencia` corresponde a SKU, código interno ou
  referência do fornecedor;
- `updatedAt` filtrável para sincronização incremental.

### 2. Cliente e CRM

O ERP atual entrega:

- nome e apelido;
- CPF/identificador;
- e-mail;
- telefone e celular;
- gênero;
- nascimento;
- endereço completo, incluindo complemento e país;
- funcionário/vendedor relacionado;
- data de cadastro;
- tags;
- loja de origem.

A Nota da Pérola entrega nome/razão social, CPF/CNPJ, RG/IE, nascimento e
endereço fiscal. Não foram observados:

- e-mail;
- telefone;
- celular;
- apelido;
- gênero;
- complemento;
- país;
- tags;
- data de cadastro;
- loja de origem do cadastro;
- consentimentos ou preferências de contato.

Solicitar, caso a API vá substituir também o CRM do ERP:

- entidade mestre de Cliente/Pessoa paginada;
- e-mail, telefone e celular;
- apelido, gênero, complemento, país, tags e data de cadastro;
- vendedor e loja de origem;
- status ativo/inativo e data de atualização;
- indicadores de consentimento/LGPD para uso de contato.

PII deve continuar sendo minimizada no backend e nunca registrada em logs.

### 3. Funcionários e vendedores

O ERP atual entrega:

- identificador original;
- nome;
- endereço básico;
- status ativo;
- loja vinculada.

A Pérola expõe `colaboradorId`, `colaboradorNome` e `colaboradorCpfCnpj` dentro
de Nota e Nota Item, mas não foi observada uma entidade mestre de colaborador.

Isso não é suficiente para:

- listar todos os vendedores, inclusive quem não vendeu no período;
- saber quem está ativo;
- resolver transferência ou vínculo atual de loja;
- ligar de forma estável o colaborador a um usuário/consultor do Omni.

Solicitar:

- endpoint mestre de Colaborador/Funcionário;
- ID estável, código de funcionário e nome;
- status ativo/inativo;
- empresa/loja atual;
- data de admissão/desligamento, se disponível;
- campo que corresponda ao `employee_id/original_id` do ERP atual.

### 4. Pedidos, pagamentos e cancelamentos

O ERP atual entrega por linha:

- `order_id`;
- `identifier`;
- `customer_id`;
- data do pedido;
- total do pedido;
- devolução de produto;
- SKU;
- valor e quantidade da linha;
- funcionário;
- forma de pagamento;
- valor de exclusão;
- valor de débito;
- loja;
- base separada de pedidos cancelados.

A combinação Nota + Nota Item cobre grande parte dos valores fiscais, porém não
foi observada equivalência confirmada para:

- `order_id`;
- `identifier`;
- forma de pagamento;
- status comercial do pedido;
- motivo/data/usuário do cancelamento;
- dataset explícito de cancelados;
- valores comerciais `total_exclusion` e `total_debit`;
- SKU na linha;
- ligação oficial de Nota Item com Item/SKU.

Solicitar:

- chave que ligue Nota à venda/pedido original;
- confirmação se `numDocumento` corresponde a `order_id`, `identifier` ou
  outro código;
- forma(s) de pagamento, parcelas e valores;
- status do pedido/nota no ciclo comercial;
- `cancelado`, data, motivo e responsável pelo cancelamento;
- endpoint ou filtro confiável de cancelados;
- valores de devolução, exclusão e débito com semântica documentada;
- SKU/itemId em Nota Item;
- filtros por data de emissão/alteração e empresa.

### 5. Estoque e custo

A API é superior ao ERP atual em custo e movimentação, mas ainda falta a chave
central para uso seguro:

- Inventário e preço de compra usam `itemSaldoId`;
- Nota Item usa `itemSaldoId`;
- Item usa `itemId`;
- na amostra, nenhuma entidade ligou `itemSaldoId` a `itemId`/SKU.

Sem essa ponte, não é seguro calcular:

- giro por SKU;
- margem por produto;
- cobertura/ruptura;
- rentabilidade por marca, coleção ou material;
- conciliação de produto vendido com movimento de estoque.

Solicitar:

- endpoint Item Saldo com `itemSaldoId`, `itemId`, SKU, empresa/loja e
  quantidade atual;
- saldo disponível, reservado e físico, se existirem;
- data da última atualização;
- unidade e depósito/local de estoque;
- semântica completa de `tipoInventario` e sinal da quantidade.

## Chaves que precisam ser confirmadas

| Origem ERP/Omni | Possível chave Pérola | Situação |
| --- | --- | --- |
| Produto `sku`/`identifier` | Item `referencia` ou `itemId` | não confirmada |
| Linha de pedido `sku` | Nota Item `itemSaldoId` | ponte ausente |
| Pedido `order_id`/`identifier` | Nota `numDocumento` | não confirmada |
| Funcionário `original_id` | `colaboradorId` | não confirmada |
| Loja `store_id/store_cnpj` | `empresaId/empresaCnpj` | CNPJ é candidato forte |
| Cliente `identifier/cpf` | `pessoaCpfCnpj` | viável com normalização e LGPD |
| Atendimento `purchaseCode` | Nota/pedido | campo de destino ainda não confirmado |

## Solicitação recomendada ao fornecedor

### Prioridade P0 — bloqueia a substituição

1. Ponte `itemSaldoId → itemId/SKU`.
2. SKU, nome, descrição e preço comercial do produto.
3. Chave oficial `pedido/venda → Nota`.
4. Forma de pagamento.
5. Cancelamento comercial estruturado.
6. SKU/itemId em Nota Item.
7. Saldo atual por produto e empresa/loja.
8. Filtros incrementais por `updatedAt` e período.

### Prioridade P1 — preserva CRM e operação

1. Entidade mestre de Cliente com e-mail, telefone, celular e data de cadastro.
2. Tags, gênero, apelido, complemento e país.
3. Entidade mestre de Funcionário com status e loja.
4. Loja de origem do cliente e vínculo comercial com vendedor.
5. Valores e motivos de devolução/exclusão/débito documentados.

### Prioridade P2 — qualidade e governança

1. Schema/OpenAPI versionado.
2. Definição formal de tipos decimais, datas e timezone.
3. Campo de exclusão lógica e data de exclusão em todas as entidades.
4. Changelog ou cursor incremental.
5. Limites, rate limit e códigos de erro documentados.

## Critérios técnicos de aceite da API substituta

A disponibilização dos campos não basta; a integração precisa suportar:

- paginação determinística com total e ordenação estável;
- limite máximo documentado;
- filtro fechado de período;
- filtro por empresa/loja;
- filtro por `updatedAt` ou cursor incremental;
- IDs estáveis e relações explícitas;
- exclusões/cancelamentos detectáveis incrementalmente;
- valores monetários como decimal/string documentada, nunca float binário;
- datas em ISO 8601 com timezone definido;
- resposta parcial/projeção de campos para evitar payload excessivo;
- timeout e rate limit documentados;
- ambiente de homologação;
- schema versionado e aviso de breaking changes.

## O que não precisa ser solicitado à Pérola

Os campos abaixo pertencem à governança do importador Omni, e não ao negócio
do ERP:

- nome e checksum do arquivo;
- lote/data de extração;
- linha de origem;
- run/file ID;
- horário de importação;
- payload bruto de auditoria.

Ao trocar arquivos por API, o Omni deve substituir essa auditoria por cursor,
request ID, página, duração, quantidade e janela incremental.

## Relação com a Fase 3 do módulo BI

A Fase 3 implementa consultas seguras para as seis entidades, mas não declara
que a substituição do ERP está pronta. Ela garante:

- dataset escolhido por ID interno, sem URL ou body arbitrário;
- paginação explícita e limite máximo por entidade;
- ordenação e filtros em allowlist;
- período fechado de no máximo 31 dias para Nota;
- `itemSaldoId` obrigatório para Inventário;
- autenticação automática apenas no backend;
- resposta estruturada sem headers, token ou body bruto da origem.

A migração ERP → API só deve avançar depois que os itens P0 forem fornecidos e
as chaves forem validadas com dados reais dos dois lados.
