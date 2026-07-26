# AGENT

## Escopo

Estas instrucoes valem para `back/internal/modules/bi`.

## Objetivo

O modulo BI concentra integracoes server-side com APIs externas de BI usadas
pela aplicacao. A primeira integracao e o proxy controlado da Perola BI.

## Direcao aprovada para o redesenho (2026-07-23)

O fluxo normal e sempre automatico:

1. o frontend pede somente o dado necessario;
2. o backend resolve as credenciais da Perola BI pelo ambiente;
3. o backend reutiliza o JWT valido mantido apenas em memoria;
4. se nao houver token, ele estiver expirado ou a origem responder 401, o
   backend autentica novamente e repete a leitura no maximo uma vez;
5. chamadas concorrentes nao podem provocar uma tempestade de logins
   (usar secao critica/singleflight ao implementar o redesenho).

O token, CompanyKey, login, senha e CNPJ nunca vao para o payload normal do
frontend. A conexao manual continua existindo somente como diagnostico:

- acao secundaria, recolhida e fora do fluxo principal;
- restrita a usuario administrativo autorizado;
- nunca persiste segredo no browser, banco ou logs;
- nunca e requisito para o usuario carregar o BI.

O redesenho sera incremental. A primeira etapa e este inventario de contrato;
ela nao autoriza importar ou varrer todos os registros da API.

## Politica obrigatoria de volume

- Toda rota `*/find` e paginada e deve receber `pageNumber`, `limit`,
  `orderBy` e `conditions` explicitamente.
- E proibido implementar "buscar tudo", loop automatico ate a ultima pagina
  ou fan-out das seis entidades ao abrir a tela.
- A UI deve buscar sob demanda, com filtro, paginacao e projecao adequados ao
  caso de uso. O backend impoe limite maximo mesmo que o frontend envie valor
  maior.
- A descoberta de contrato usa `limit: 1`. Ela nao e um mecanismo de sync.
- Datasets pesados devem exigir filtro seletivo antes da chamada. Inventario
  nao pode aceitar leitura aberta no fluxo normal.
- Uma falha/timeout de uma entidade nao dispara retry recursivo nem aumenta o
  limite. Retry de autenticacao e no maximo um e somente para token invalido.
- Respostas e logs registram endpoint, status, duracao, pagina e quantidade,
  nunca token, credenciais, body bruto ou valores de PII.

### Evidencia de performance da descoberta

Teste real em 2026-07-23, com uma unica autenticacao e `limit: 1`:

| Entidade | Endpoint | Resultado | Duracao observada |
| --- | --- | --- | --- |
| Item | `/item/find` | 200, 1 registro | ~0,6 s |
| Imagem Item | `/imagemItem/find` | 200, 1 registro | ~0,6 s |
| Item Saldo Preco Compra | `/itemSaldoPrecoCompra/find` | 200, 1 registro | ~0,9 s |
| Nota | `/nota/find` | 200, 1 registro | ~6,2 s |
| Nota Item | `/notaItem/find` | 200, 1 registro | ~2,6 s |
| Inventario aberto | `/inventario/find` | timeout, nenhum registro consumido | >35 s |
| Inventario por `itemSaldoId` | `/inventario/find` | 200, 1 registro | ~0,4 s |

Conclusao operacional: `limit: 1` limita a resposta, mas nao garante consulta
barata na origem. Inventario exige filtro seletivo; Nota tambem deve ser
tratada como fonte cara.

## Contrato externo confirmado

Documentacao oficial:
`https://documenter.getpostman.com/view/18113175/2s9YXh6iRz`.

- Base documentada: `https://api.client.c10.srv.br/api/v1`.
- Base real da Perola: `https://api.perola.c10.srv.br/api/v1`.
- Login: `POST /sessoes`, headers `dsCompanyKey` e `dsCnpjEmpresa`, body
  `{login, pass}`. A resposta observada possui `token` e `user`.
- Leituras: Bearer Token + `dsCompanyKey` + `dsCnpjEmpresa`.
- Envelope observado nas seis entidades:
  `{paginacao: {...}, registros: [...]}`.

O login real respondeu 200 com e sem `dsCnpjEmpresa`; o redesenho deve enviar
o CNPJ mesmo assim para permanecer aderente ao contrato publicado.

## Inventario das seis entidades

Levantamento feito com exatamente um registro por entidade. Os tipos abaixo
sao tipos observados na amostra, nao um schema formal da fornecedora:

- `string` inclui datas e valores monetarios retornados como texto;
- `number` e numero JSON;
- `null` significa apenas que o registro amostrado veio sem valor;
- nenhum valor real, token, credencial ou PII foi documentado.

### 1. Item — `POST /item/find`

Campos observados:

```text
acabamentos:string
classe:string; classeId:number
colecao:null; colecaoId:null
cor:string; corId:number
coresPulseiras:null
created:string
departamento:string; departamentoId:number
estilo:string; estiloId:number
formato:string; formatoId:number
fornecedor:string
funcoes:null
fundamental:string; fundamentalId:number
id:number; itemId:number
marca:string; marcaId:number
materiaisPulseiras:null
material:string; materialId:number
modified:string
moeda:null; moedaId:null
mostradores:null
movimento:null; movimentoId:null
obs:string
pedras:string
pesoBruto:null; pesoLiquido:null
preenchimento:null; preenchimentoId:null
referencia:string
resistenciaAgua:null
subtipo:string; subTipoId:number
tamanho:string
tipo:string; tipoId:number
tipoFabricacao:null; tipoFabricacaoId:null
un:string
vidro:null; vidroId:null
visor:null; visorId:null
```

### 2. Imagem Item — `POST /imagemItem/find`

Campos observados:

```text
filename:string
id:number
itemId:number
ordem:null
```

Relacao evidente: `itemId` referencia a entidade Item. O filename e dado
externo; nao assumir URL publica nem montar caminho sem contrato explicito.

### 3. Item Saldo Preco Compra — `POST /itemSaldoPrecoCompra/find`

Campos observados:

```text
data:string
empresaId:number
id:number
itemSaldoId:number
notaItemId:null
precoCusto:string; precoCustoMoeda:string
precoEntrada:string; precoEntradaMoeda:string
precoMedio:string; precoMedioMoeda:string
```

Valores monetarios chegaram como `string`; conversao futura deve ser
decimal controlado, nunca `float` binario.

### 4. Nota — `POST /nota/find`

Campos observados:

```text
baseIcms:string; baseIcmsSt:string
colaboradorCpfCnpj:string; colaboradorId:number; colaboradorNome:string
created:string; modified:string
dataEmissao:string; dataSaidaEntrada:string
empresaCnpj:string; empresaFantasia:string; empresaId:number
empresaRazaoSocial:string; empresaSigla:string
excluido:boolean
id:number
informacoesComplementares:null
numDocumento:string
obs:string
origemDestino:string; origemDestinoId:number; origemDestinoSigla:string
pessoaBairro:string; pessoaCep:string; pessoaCidade:string
pessoaCpfCnpj:string; pessoaDatNascimento:string
pessoaLogradouro:string; pessoaNomeRazaoSocial:string
pessoaNumero:string; pessoaRgIe:string; pessoaUf:string
serie:string
tipoFrete:null
tipoNota:string; tipoNotaSigla:string
valorAcrescimo:string; valorCofins:string
valorConhecimentoFrete:string; valorCotacaoMoeda:null
valorDesconto:string; valorFrete:string
valorIcms:string; valorIcmsSt:string; valorIpi:string
valorOutrasDespesas:string; valorOutrosCustos:string
valorPis:string; valorSeguro:string
valorTotal:string; valorTotalItens:string; valorVendaAutorizacao:string
```

Esta entidade contem PII e dados fiscais. CPF/CNPJ, RG/IE, nascimento,
endereco e nome de pessoa nao entram em log, telemetria, cache compartilhado
ou resposta de tela que nao precise deles.

### 5. Nota Item — `POST /notaItem/find`

Campos observados:

```text
colaboradorCpfCnpj:string; colaboradorId:number; colaboradorNome:string
created:null; modified:null
estoqueOperacao:string
excluido:boolean
id:number
itemSaldoId:number
notaId:number
precoCusto:string; precoCustoMoeda:string
precoEntrada:string; precoEntradaMoeda:string
precoMedio:string; precoMedioMoeda:string
precoTotal:string; precoUnitario:string
quantidade:string; quantidadeDevolvida:number
valorAcrescimo:string; valorDesconto:string; valorDevolucao:string
```

Relacoes observaveis: `notaId` liga a Nota; `itemSaldoId` liga os dados de
saldo/preco e permite filtrar Inventario.

### 6. Inventario — `POST /inventario/find`

Campos observados na consulta filtrada por `itemSaldoId`:

```text
created:string
data:string
empresaCnpj:string; empresaFantasia:string; empresaId:number
empresaRazaoSocial:string; empresaSigla:string
id:number
itemSaldoId:number
quantidade:string
tipoInventario:string; tipoInventarioSigla:string
```

Regra especifica: nao executar essa entidade sem filtro seletivo. A consulta
aberta com `limit: 1` excedeu 35 segundos; com `equalsTo.itemSaldoId`, retornou
um registro em menos de um segundo.

## Matriz de inteligencia: Perola BI, ERP e Fila

Levantamento interno confirmado em 2026-07-23:

- ERP (`crm/erp`) persiste `item`, `customer`, `employee`, `order` e
  `ordercanceled` no PostgreSQL. A base local observada possui produtos,
  clientes, funcionarios, pedidos e cancelados reais; a inteligencia nao deve
  buscar esses dados novamente na Perola quando o ERP ja e a fonte adequada.
- Fila (`queue`) e a fonte da jornada operacional: espera, duracao,
  atendimento, desfecho, produto visto/fechado/nao encontrado, motivo da
  visita, origem, campanha, furo de fila, pausas e qualidade de preenchimento.
- Perola BI e a fonte fiscal e de custo: notas, itens da nota, impostos,
  descontos, devolucoes, custo/entrada/medio, atributos detalhados do produto,
  imagens e movimentos de inventario.

### Dados existentes no ERP que nao foram observados na Perola

- telefone, celular e e-mail do cliente;
- apelido, genero, pais, complemento, tags e data de cadastro;
- SKU, nome e descricao comercial do produto;
- forma de pagamento;
- dataset explicito e separado de pedidos cancelados;
- valores de exclusao e debito;
- vinculo de funcionario com loja/perfil;
- agregacoes prontas de pedidos, faturamento, ticket, PA, meta, comissao e
  cancelamento.

### Inteligencias viaveis por fonte

- ERP + Fila, com ingredientes ja internos: desempenho comercial completo,
  uso da fila, conversao, ticket, PA, meta/comissao, cancelamentos, qualidade
  da jornada, segmentacao RFM, recompra e contato.
- Perola, apos consulta segura por periodo: margem bruta estimada por nota e
  consultor, carga fiscal, descontos/devolucoes e cobertura do mix por
  atributos/imagens.
- Cruzamentos, apos confirmar chaves: conciliacao comercial x fiscal,
  rentabilidade por SKU/atributo, giro/cobertura de estoque, jornada 360 do
  cliente e rentabilidade de campanhas.

### Chaves pendentes antes de cruzar

- confirmar ERP `order_id`/`identifier` x Perola `numDocumento` ou outra chave
  oficial da Nota;
- obter/confirmar a ponte Perola `itemSaldoId` x Item/SKU. `Nota Item` nao
  trouxe `itemId`/SKU e `Item` nao trouxe `itemSaldoId`; atribuir margem ou
  inventario a um produto sem essa ponte seria incorreto;
- priorizar `purchaseCode`/documento para ligar Fila e venda. CPF/telefone sao
  PII e so podem ser usados com normalizacao, escopo e minimizacao.

A aba Inteligencia do frontend documenta essas possibilidades e o nivel de
prontidao. Um card nessa aba nao significa KPI ao vivo: numeros reais so podem
aparecer depois de agregacao backend, filtro de periodo e consulta intencional.

## Gaps confirmados no modulo atual

- A allowlist da Fase 3 cobre os seis endpoints confirmados, inclusive
  `/imagemItem/find` e `/itemSaldoPrecoCompra/find`.
- O overview atual foi desenhado antes deste inventario e nao deve ser
  expandido para seis chamadas automaticas na montagem da tela.
- A experiencia manual ja foi recolhida como diagnostico secundario no front,
  mas as rotas ainda dependem de roles hardcoded e serao separadas por
  permissao na Fase 5.
- A Fase 5 ainda precisa impor autorizacao por permissao/account antes da
  liberacao definitiva.

## Plano incremental do redesenho

### Fase 1 — contrato e amostra minima (concluida)

- Confirmar os seis endpoints oficiais.
- Autenticar com as credenciais reais sem expor segredos.
- Ler um registro por entidade e documentar somente campos/tipos.
- Provar o risco do Inventario e definir filtro obrigatorio.

### Fase 2 — cliente externo e autenticacao automatica (concluida em 2026-07-23)

- `PerolaClient` concentra transporte, login e ciclo do JWT, separado da
  agregacao historica mantida no `Service`.
- `EnsureToken` mantem cache somente em memoria com expiracao e coalesce de
  chamadas concorrentes: uma unica autenticacao alimenta todos os callers que
  estavam aguardando.
- `RefreshAfterUnauthorized` compara o token rejeitado com o cache atual,
  evitando novo login quando outra chamada concorrente ja renovou o JWT.
- `Find` repete a leitura no maximo uma vez depois de 401. Token manual nao
  ganha retry implicito.
- Login e leituras enviam `dsCompanyKey` e `dsCnpjEmpresa`; o CNPJ tambem faz
  parte do payload interno do diagnostico manual, sem persistencia no browser.
- Testes com `httptest.Server` cobrem headers/login, cache, concorrencia,
  401+retry unico, teto de retry, timeout e cancelamento.

### Fase 3 — consultas seguras das seis entidades (concluida em 2026-07-23)

- `perolaDatasetRegistry` define ID interno, endpoint, limites, ordenacoes,
  filtros, operadores e alternativas de filtro obrigatorio por entidade.
- A consulta de produto recebe apenas dataset, pagina, limite, ordenacao e
  filtros tipados. Endpoint e body da origem sao montados pelo backend.
- `/imagemItem/find` e `/itemSaldoPrecoCompra/find` passaram a integrar a
  allowlist.
- Inventario exige `itemSaldoId` exato. Nota exige ID/documento exato ou
  periodo fechado de `dataEmissao` com no maximo 31 dias.
- Item, imagem, preco de compra e item de nota tambem exigem um filtro seletivo
  documentado no catalogo; nenhuma colecao grande nasce de busca aberta.
- A resposta publica contem paginacao, quantidade, `hasMore`, registros
  escalares e duracao. Headers, token, body bruto e estruturas aninhadas da
  origem nao sao devolvidos.
- Testes cobrem catalogo, limites, filtros obrigatorios, periodo de Nota,
  rejeicao de campo/ordenacao desconhecidos, payload enviado a origem e
  sanitizacao da resposta.

### Fase 4 — experiencia do painel (concluida em 2026-07-23)

- `/bi` continua nascendo em `Entidades`, sem carregar colecoes externas.
- `Entidades` usa o catalogo local para mostrar uma grade paginada e filtravel
  com todos os campos observados das seis entidades; essa grade nao consulta
  nenhuma API.
- `Lacunas ERP × API` materializa as 18 solicitacoes auditadas, com prioridade
  P0/P1/P2, dominio, evidencia do ERP, lacuna e texto para a fornecedora.
- `Consultas` nasce totalmente passiva. Ate o catalogo seguro exige clique
  explicito; depois disso, cada entidade exige filtro intencional e cada
  consulta busca uma unica pagina.
- O catalogo publico informa tambem alternativas de filtro obrigatorio e
  periodo maximo em formato estruturado, permitindo validacao previa da UI sem
  expor endpoint externo.
- A paginacao Anterior/Proxima repete no backend o ultimo conjunto submetido
  de filtros, limite e ordenacao; edicao ainda nao submetida nao altera a
  consulta em curso.
- Falha de configuracao ou da origem vira aviso acionavel. Autenticacao
  automatica continua sendo o caminho principal e o diagnostico manual
  permanece recolhido.
- Inventario continua exigindo `itemSaldoId`; nenhuma aba dispara fan-out das
  seis entidades ou busca aberta.
- O switch `Bloquear chamadas` nasce ativo no cabecalho e e autoritativo no
  store: recusa login, catalogo, overview e query antes do transporte e aborta
  requisicoes do BI que ja estejam em andamento. Desbloquear nao dispara
  nenhuma leitura automaticamente.

### Fase 5 — autorizacao, observabilidade e validacao

- Colocar o BI sob gate do modulo por account e permissao de leitura/
  diagnostico, sem depender apenas de roles hardcoded.
- Medir endpoint, status, duracao, pagina e quantidade sem PII ou secrets.
- Aplicar rate limit, timeout por dataset e limite server-side.
- Validar cada entidade com `limit: 1`, depois validar paginacao pequena e
  filtros reais sem executar carga total.
- Fazer smoke no navegador e confirmar que abrir `/bi` nao chama a API
  externa ate existir uma consulta intencional.

## Regras

- Nao criar proxy aberto para URLs arbitrarias.
- Manter hosts, bases e endpoints externos em allowlist no backend.
- Nao persistir tokens, CompanyKey, login ou senha neste modulo.
- O fluxo tipado retorna somente dados estruturados e metadados minimos. A
  resposta bruta permanece restrita ao diagnostico legado e nao deve ser usada
  por telas de produto.
- Usar timeout e `http.NewRequestWithContext` para chamadas externas.
- Teto de paginacao do fallback legado: `PEROLA_BI_MAX_PAGES` tem default `5`
  (`defaultPerolaMaxPages=5`). Esse teto so vale quando um dataset NAO fixa
  `MaxPages` proprio. As definicoes usadas pelo overview fixam `MaxPages: 1`;
  imagem e preco de compra ficam fora do overview e Inventario so entra quando
  solicitado explicitamente. A flag `source.truncated` continua sendo emitida
  quando ha mais paginas do que o teto.

## Rotas

- `POST /v1/bi/perola/login` — diagnostico: recebe `companyKey`,
  `cnpjEmpresa`, `login`, `pass` e devolve `PerolaProxyResponse` com o JWT
  capturado em `token`. Campos vazios usam a configuracao do backend.
- `GET /v1/bi/perola/datasets` — catalogo publico para a UI montar consultas
  permitidas sem conhecer endpoints externos. Informa limites, ordenacoes,
  filtros, operadores, alternativas obrigatorias e regra de periodo.
- `GET /v1/bi/perola/sales/recent` — consulta fixa de
  `/vendas/colaboradores` nos ultimos 31 dias, remove campos sensiveis,
  ordena pela data disponivel e devolve no maximo as 20 vendas mais recentes.
- `POST /v1/bi/perola/datasets/{dataset}/query` — consulta tipada de produto
  para `item`, `imagem-item`, `item-saldo-preco-compra`, `nota`, `nota-item`
  ou `inventario`. Autenticacao, endpoint e payload externo ficam no backend.
- `POST /v1/bi/perola/find` — proxy legado de diagnostico para os seis
  endpoints da allowlist. Aceita token/body manual e nao deve ser consumido
  por novas telas de produto.
- `GET /v1/bi/perola/overview` — agrega as fontes da Perola BI.
  - Query `cnpjEmpresa` sobrepoe o CNPJ default.
  - Header `X-Perola-Token` sobrepoe o token cacheado (usado pelo front
    quando o usuario gera o token via formulario de conexao).

## Carregamento no front (web/app/components/bi/BiWorkspace.vue)

- O painel NAO dispara nenhuma chamada automatica ao montar.
- O switch `Interromper API` inicia ativo. Enquanto estiver ativo, todas as
  actions HTTP do store BI retornam bloqueio sem chamar nem a sessao nem uma
  rota `/v1/bi/*`; ativar o switch durante uma leitura aborta o request.
- Abrir `Consultas`, alternar entre as seis tabelas e ver todas as colunas
  mapeadas nao chama nenhuma rota do BI. O catalogo interno exige clique em
  `Carregar catalogo de consultas`; registros externos so sao lidos depois do
  clique em `Consultar`.
- Abrir `Vendas` faz uma unica leitura de `/v1/bi/perola/sales/recent` quando
  o bloqueio global estiver desligado. A aba mostra no maximo 20 registros e
  permite atualizacao manual; nao recebe credencial nem periodo do browser.
- O carregamento so ocorre quando o usuario clica em "Atualizar BI" ou
  "Carregar com token". Login manual nao encadeia overview automaticamente.
- O store `web/app/stores/bi.ts` tambem nao faz fallback recursivo para
  carregar o inventario em background — chamadas a `/v1/bi/perola/overview`
  acontecem 1x por clique do usuario, evitando volume excessivo na API
  externa.

## Contrato com Customer Intelligence (2026-07-23)

- `customer_intelligence_availability.go` valida, sem rede, uma proposta de
  consulta contra o registry fechado de datasets, filtros, ordenacoes e limites
  do owner BI.
- `bi.perola` e somente `on_demand`. O adapter nao chama login, `find`,
  `overview` nem endpoint externo.
- O contrato atual do BI nao possui chave deterministica que relacione uma
  linha ao `subject_id + relationship_id` de Customer Data. Por isso a resposta
  canonica e `unavailable` com
  `deterministic_subject_link_unavailable`, mesmo para configuracao valida.
- Nunca substituir esse estado por query aberta, filtro por nome ou inferencia
  fuzzy. A ativacao futura exige vinculo deterministico e uma fachada tipada
  owner-owned; indisponibilidade nao pode bloquear o chat.
