# Pagina `operacao`

## Objetivo

Esta pagina e o cockpit operacional da fila. Ela concentra a entrada de consultores na lista da vez, o inicio de atendimentos, a pausa/retomada de pessoas e o fechamento completo do atendimento.

## Estrutura de arquivos

- `web/app/pages/operacao/index.vue`: entrada da rota `/operacao`.
- `web/app/features/operation/components/OperationWorkspace.vue`: composicao principal da pagina.
- `web/app/features/operation/components/OperationScopeBar.vue`: seletor explicito de loja e de modo da operacao.
- `web/app/features/operation/components/OperationQueueColumns.vue`: coluna da fila e coluna de atendimentos em andamento.
- `web/app/features/operation/components/OperationActiveServiceCard.vue`: card reutilizavel de atendimento ativo.
- `web/app/features/operation/components/OperationConsultantStrip.vue`: barra inferior com todos os consultores.
- `web/app/features/operation/components/OperationFinishModal.vue`: modal de encerramento do atendimento.
- `web/app/features/operation/components/OperationProductPicker.vue`: seletor reutilizado para produtos vistos e produtos fechados.
- `web/app/stores/operations.ts`: store de dominio usado pela pagina.
- `web/app/utils/runtime-remote.ts`: hidratacao remota da loja ativa, incluindo snapshot operacional.
- `web/app/stores/dashboard.ts`: facade Pinia temporaria de compatibilidade.
- `web/app/stores/dashboard/runtime/create-dashboard-runtime.ts`: runtime de compatibilidade usado como camada visual/efemera.

## Fonte de verdade atual

Hoje a operacao funciona assim:

- snapshot e comandos operacionais saem da API Go;
- o Postgres e a fonte de verdade da fila, atendimentos ativos, pausas, historico e sessoes;
- o runtime do frontend continua existindo para sustentar compatibilidade de tela, modal e composicao local.
- comandos `POST` devolvem apenas `ack` minimo; depois disso a UI revalida o snapshot operacional da loja ativa.
- a pagina tambem abre um WebSocket por loja em `GET /v1/realtime/operations` para receber invalidacoes leves e revalidar o snapshot sem refresh.
- para `owner` e `platform_admin`, a pagina tambem pode trabalhar em modo integrado multi-loja por `GET /v1/operations/overview`, mantendo o snapshot da loja ativa como fallback e detalhe operacional.

Endpoints atualmente usados pela pagina:

- `GET /v1/operations/snapshot`
- `GET /v1/operations/overview`
- `POST /v1/operations/queue`
- `POST /v1/operations/pause`
- `POST /v1/operations/resume`
- `POST /v1/operations/assign-task`
- `POST /v1/operations/start`
- `POST /v1/operations/finish`
- `GET /v1/realtime/operations`

## Blocos visuais da pagina

### 1. Barra de escopo

Arquivo: `OperationScopeBar.vue`

Responsabilidade:
- deixar explicito qual loja esta sendo vista;
- permitir trocar a loja ativa sem depender so do header;
- permitir que `owner` e `platform_admin` alternem entre `Loja ativa` e `Todas as lojas`;
- permitir filtro interno por loja quando a visao integrada estiver aberta.

Elementos:
- select `Loja`
- select `Modo`
- select `Filtro por loja`

Comportamento:
- consultor, gerente e `store_terminal` seguem operando apenas na loja ativa;
- `owner` e `platform_admin` podem abrir a visao integrada multi-loja;
- todos os filtros simples dessa barra devem reaproveitar [AppSelectField.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ui/AppSelectField.vue);
- no modo integrado, a tela continua mostrando o nome da loja em cada card para auditoria visual.

### 2. Lista da vez

Arquivo: `OperationQueueColumns.vue`

Responsabilidade:
- exibir a fila atual em ordem;
- destacar o primeiro da fila;
- permitir iniciar o primeiro atendimento;
- permitir atendimento fora da vez para qualquer pessoa que nao esteja na primeira posicao.

Elementos:
- botao `Atender primeiro da fila`
- cards da fila com posicao, avatar, nome, cargo e status
- botao de raio para atendimento fora da vez

Comportamento:
- se nao houver ninguem na fila, mostra estado vazio;
- se o limite de atendimentos ativos for atingido, o botao principal e os botoes fora da vez ficam desabilitados;
- iniciar atendimento fora da vez registra quantas pessoas foram puladas.
- no modo `Todas as lojas`, esta mesma coluna passa a listar todos os consultores em fila das lojas acessiveis;
- nesse modo, cada card mostra um badge com a loja e troca a acao de queue-jump pelo botao `Tirar`.

### 3. Em atendimento

Arquivos: `OperationQueueColumns.vue` e `OperationActiveServiceCard.vue`

Responsabilidade:
- listar atendimentos ativos;
- mostrar hora de inicio, tipo de entrada e cronometro em tempo real;
- abrir o modal de encerramento.

Elementos:
- cards de atendimento ativo
- cronometro atualizado a cada segundo
- botao `Encerrar atendimento`

Comportamento:
- se nao houver atendimento ativo, mostra estado vazio;
- o rotulo do atendimento informa se ele entrou `Na vez` ou `Fora da vez`;
- quando houve queue jump, o card informa quantas pessoas foram puladas.
- o card nao exibe mais o ID tecnico nem o titulo redundante da propria coluna, para manter a leitura compacta;
- no modo `Todas as lojas`, esta mesma coluna passa a listar todos os atendimentos ativos das lojas acessiveis;
- nesse modo, cada card mostra um badge com a loja para auditoria visual imediata.

### 4. Visao integrada multi-loja

Arquivos: `OperationWorkspace.vue`, `OperationQueueColumns.vue` e `OperationConsultantStrip.vue`

Responsabilidade:
- reaproveitar o mesmo layout da operacao da loja ativa;
- consolidar fila, atendimentos e roster de todas as lojas acessiveis em uma unica tela;
- mostrar de forma simples qual consultor pertence a qual loja;
- permitir auditoria rapida sem mudar a estrutura mental da operacao normal.

Comportamento:
- a pagina continua com as mesmas duas colunas principais e a mesma faixa inferior de consultores;
- `Lista da vez` mostra todos que estao aguardando, de todas as lojas acessiveis;
- `Em atendimento` mostra todos os atendimentos ativos, de todas as lojas acessiveis;
- a barra inferior continua exibindo o roster, agora com badge de loja quando necessario;
- `owner` e `platform_admin` podem filtrar internamente uma unica loja sem sair do modo integrado;
- cada card mostra a loja de origem de forma visual;
- a acao `Tirar para tarefa` registra `assignment`, nao uma pausa comum.

### 5. Barra de consultores

Arquivo: `OperationConsultantStrip.vue`

Responsabilidade:
- mostrar todos os consultores do roster;
- permitir entrar na fila;
- permitir pausar;
- permitir retomar pausa.

Estados possiveis por consultor:
- `available`: disponivel
- `queue`: na fila
- `service`: em atendimento
- `paused`: pausado
- `assignment`: em tarefa/reuniao, persistido como pausa tipada

Botoes:
- `Entrar na fila`
- `Direcionar para tarefa`
- `Pausar`
- `Retomar`

Comportamento:
- consultor disponivel pode entrar na fila ou ser pausado;
- consultor disponivel ou na fila pode ser deslocado para tarefa/reuniao;
- consultor na fila pode ser pausado;
- consultor pausado mostra o motivo da pausa e pode ser retomado;
- consultor em atendimento nao recebe acao direta por essa barra.

### 6. Modal de encerramento

Arquivo: `OperationFinishModal.vue`

Responsabilidade:
- capturar o resultado do atendimento;
- registrar produto visto e produto fechado;
- registrar dados do cliente;
- registrar motivo da visita, origem, observacoes e motivo de furar fila quando aplicavel;
- obedecer a configuracao de selecao e descricao definida em /configuracoes para motivos e origens;
- validar o formulario antes de persistir no store.

Secoes do modal:
- resultado do atendimento
- flags operacionais
- produtos vistos
- produtos comprados ou reservados
- dados do cliente
- profissao
- motivo da visita
- origem do cliente
- motivo do atendimento fora da vez
- observacoes
- resumo do valor vendido derivado dos produtos fechados

## Inventario detalhado de botoes e acoes

### `OperationQueueColumns.vue`

- `Atender primeiro da fila`
  - acao: `operationsStore.startService()`
  - efeito: envia comando HTTP, remove o primeiro da fila e cria um atendimento ativo com `startMode = queue`
- botao de raio em um card fora da primeira posicao
  - acao: `operationsStore.startService(personId)`
  - efeito: envia comando HTTP e inicia atendimento fora da vez com `startMode = queue-jump`
- `Encerrar atendimento`
  - acao: `operationsStore.openFinishModal(personId)`
  - efeito: abre o modal preenchido com um draft inicial local de compatibilidade

### `OperationConsultantStrip.vue`

- `Entrar na fila`
  - acao: `operationsStore.addToQueue(personId)`
  - efeito: envia comando HTTP, recebe `ack` minimo e revalida o snapshot; o consultor entra no fim da fila com `queueJoinedAt`
- `Direcionar para tarefa`
  - acao: abre prompt da store `ui`, depois chama `operationsStore.assignTask(personId, reason[, storeId])`
  - efeito: envia comando HTTP, recebe `ack` minimo e revalida o snapshot/overview; tira o consultor da fila quando aplicavel e registra o estado como `assignment`
- `Pausar`
  - acao: abre prompt da store `ui`, depois chama `operationsStore.pauseEmployee(personId, reason)`
  - efeito: envia comando HTTP, recebe `ack` minimo e revalida o snapshot; remove o consultor da fila, se estiver nela, e registra a pausa
- `Retomar`
  - acao: `operationsStore.resumeEmployee(personId)`
  - efeito: envia comando HTTP, recebe `ack` minimo e revalida o snapshot; remove o estado de pausa e devolve o consultor para a fila

### `OperationFinishModal.vue`

- radios `Reserva`, `Compra`, `Nao compra`
  - definem `form.outcome`
- checkbox `Atendimento de vitrine`
  - define `form.isWindowService`
- checkbox `Foi para presente`
  - so aparece em `compra` ou `reserva`
- checkbox `Ja era cliente`
  - define `form.isExistingCustomer`
- produto visto
  - adiciona um ou varios itens vistos pelo cliente
- produto comprado/reservado
  - so aparece em `compra` ou `reserva`
  - aceita um ou varios itens e define a base do valor vendido
- toggle `Nao informado` em motivo e origem
  - limpa a selecao atual e marca o campo como nao informado
- `Cancelar`
  - acao: `operationsStore.closeFinishModal()`
- `Salvar e encerrar`
  - acao: `operationsStore.finishService(personId, closureData)`
  - efeito: envia comando HTTP, recebe `ack` minimo e revalida o snapshot; remove do atendimento ativo, salva historico e devolve consultor para a fila

### `OperationProductPicker.vue`

- botao principal `Selecionar produto`
  - abre o dropdown pesquisavel do catalogo
- busca do dropdown
  - filtra por nome, categoria, codigo e, no modo de fechamento, tambem pelo valor
- clique em item do catalogo
  - insere produto do catalogo na lista local do form
- `Nenhum`
  - so existe no modo de produto visto
  - marca explicitamente que nenhum produto foi informado
- `Produto nao cadastrado`
  - abre formulario inline dentro do proprio dropdown
- `Confirmar`
  - inclui produto customizado no formulario
- botao `close` em tag/item
  - remove produto selecionado

## Validacoes atuais do fechamento

As validacoes abaixo estao no `submitForm()` de `OperationFinishModal.vue`.

- o resultado do atendimento e obrigatorio;
- se `requireVisitReason` estiver ativo, motivo da visita e obrigatorio, exceto quando `Nao informado` estiver marcado;
- se `requireProduct` estiver ativo, produto visto e obrigatorio, exceto quando `Nenhum` estiver marcado;
- se o resultado for `compra` ou `reserva` e `requireProduct` estiver ativo, pelo menos um produto fechado e obrigatorio;
- se `requireCustomerNamePhone` estiver ativo, nome e telefone sao obrigatorios;
- se `requireCustomerSource` estiver ativo, origem e obrigatoria, exceto quando `Nao informado` estiver marcado;
- se o atendimento foi `queue-jump`, o motivo de furar fila e obrigatorio;
- o valor da venda nao e digitado manualmente: ele e calculado pela soma dos produtos fechados.

## Contrato recomendado para API/DB do fechamento

Ao integrar o backend Go, trate estes campos como fonte de verdade do fechamento:

- `productsSeen[]`
- `productsClosed[]`
- `productsSeenNone`
- `visitReasons[]`
- `visitReasonDetails`
- `visitReasonsNotInformed`
- `customerSources[]`
- `customerSourceDetails`
- `customerSourcesNotInformed`
- `lossReasons[]`
- `lossReasonDetails`

Importante:

- `visitReasons[]` nao devem restringir o desfecho do atendimento;
- a combinacao entre motivo, desfecho e produtos fechados serve para relatorio e inteligencia, nao para travar operacao nem banco.
- `lossReasons[]` e `lossReasonDetails` devem ser persistidos apenas quando `finishOutcome = nao-compra`.
- no request de fechamento, o frontend deve omitir campos nao aplicaveis ao desfecho atual; por exemplo, nao enviar `lossReasons*` em `compra`/`reserva`, nem mandar strings/listas vazias sem necessidade.

Campos derivados que podem continuar existindo por compatibilidade e leitura rapida:

- `productSeen`
- `productClosed`
- `productDetails`
- `lossReasonId`
- `lossReason`
- `saleAmount`

## Regras de negocio atuais no dominio

As regras operacionais principais vivem hoje no backend Go em `back/internal/modules/operations/*`.

No frontend, o runtime de compatibilidade em `web/app/stores/dashboard/runtime/create-dashboard-runtime.ts` continua cuidando de:

- `openFinishModal(personId)`
- `closeFinishModal()`
- draft inicial do modal
- detalhes de composicao local da UI

- `addToQueue(personId)`
  - ignora a acao se o consultor nao existir, ja estiver na fila, estiver em atendimento ou pausado
- `pauseEmployee(personId, reason)`
  - exige motivo nao vazio
  - ignora a acao se ja estiver pausado
  - nao pausa quem estiver em atendimento
  - remove da fila antes de registrar a pausa
- `assignTask(personId, reason)`
  - exige motivo nao vazio
  - nao desloca quem estiver em atendimento
  - remove da fila quando aplicavel
  - registra `kind = assignment` para diferenciar tarefa/reuniao de pausa comum
- `resumeEmployee(personId)`
  - remove a pausa
  - recoloca o consultor no fim da fila se ele nao estiver em atendimento nem ja estiver aguardando
- `startService(personId?)`
  - ignora se a fila estiver vazia
  - ignora se o limite de atendimentos simultaneos foi atingido
  - sem `personId`, atende o primeiro da fila
  - com `personId`, inicia atendimento fora da vez se o consultor ainda estiver na fila
- `openFinishModal(personId)`
  - abre o modal apenas para quem estiver em atendimento
- `finishService(personId, closureData)`
  - exige `outcome` valido
  - remove o atendimento ativo
  - salva historico do atendimento
  - aplica campanhas ao historico
  - atualiza opcoes de profissao quando necessario
  - devolve o consultor para o fim da fila ao concluir

## Draft inicial do modal

Ao abrir o modal, a pagina ainda usa `buildRandomFinishModalDraft(...)` do dominio para preencher um rascunho inicial local. A fila e o historico ja vieram para a API Go, mas esse draft continua como camada de UX/compatibilidade.

Impacto:
- ajuda a simular preenchimento rapido durante demonstracoes;
- pode ser trocado por defaults reais mais a frente, sem alterar o contrato operacional da API.

## Dependencias de estado usadas pela pagina

Campos mais relevantes lidos de `state`:
- `waitingList`
- `activeServices`
- `roster`
- `pausedEmployees`
- `settings.maxConcurrentServices`
- `modalConfig`
- `finishModalPersonId`
- `finishModalDraft`
- `productCatalog`
- `visitReasonOptions`
- `customerSourceOptions`
- `professionOptions`

No modo integrado, a pagina tambem trabalha com:

- `overview.stores`
- `overview.waitingList`
- `overview.activeServices`
- `overview.pausedEmployees`
- `overview.availableConsultants`

## Seletores estaveis para automacao

Estes `data-testid` foram adicionados para suportar testes automatizados e um robo generico no futuro.

- `operation-board`
- `operation-campaign-brief`
- `operation-waiting-column`
- `operation-start-first`
- `operation-waiting-{personId}`
- `operation-start-specific-{personId}`
- `operation-service-column`
- `operation-service-{personId}`
- `operation-finish-{personId}`
- `operation-consultant-strip`
- `operation-consultant-{personId}`
- `operation-add-to-queue-{personId}`
- `operation-assign-task-{personId}`
- `operation-pause-{personId}`
- `operation-resume-{personId}`
- `operation-finish-modal`
- `operation-finish-close`
- `operation-outcome-reserva`
- `operation-outcome-compra`
- `operation-outcome-nao-compra`
- `operation-products-seen-*`
- `operation-products-closed-*`
- `operation-products-seen-trigger`
- `operation-products-seen-search`
- `operation-products-seen-option-{productId}`
- `operation-products-seen-custom-option`
- `operation-products-closed-trigger`
- `operation-products-closed-search`
- `operation-products-closed-option-{productId}`
- `operation-products-closed-custom-option`
- `operation-customer-name`
- `operation-customer-phone`
- `operation-customer-email`
- `operation-customer-profession-*`
- `operation-customer-profession-trigger`
- `operation-customer-profession-search`
- `operation-visit-reason-*`
- `operation-visit-reason-trigger`
- `operation-visit-reason-search`
- `operation-visit-reason-detail`
- `operation-customer-source-*`
- `operation-customer-source-trigger`
- `operation-customer-source-search`
- `operation-customer-source-detail`
- `operation-queue-jump-reason-*`
- `operation-queue-jump-reason-trigger`
- `operation-queue-jump-reason-search`
- `operation-notes`
- `operation-finish-cancel`
- `operation-finish-submit`

## Checklist funcional sugerido

### Fila

- adicionar um consultor disponivel na fila
- impedir duplicidade ao clicar novamente
- iniciar o primeiro da fila
- iniciar fora da vez e verificar `queue-jump`
- bloquear inicio quando `maxConcurrentServices` for atingido

### Pausa

- pausar consultor disponivel com motivo
- pausar consultor que esta na fila
- retomar consultor pausado e verificar retorno para o fim da fila
- impedir pausa de consultor que esta em atendimento

### Encerramento

- abrir modal de um atendimento ativo
- validar obrigatoriedade de resultado
- validar produto visto quando exigido
- validar produto fechado em `compra`/`reserva`
- validar nome e telefone quando exigidos
- validar origem quando exigida
- validar motivo de furar fila em `queue-jump`
- encerrar atendimento e confirmar retorno do consultor para o fim da fila
- confirmar gravacao em `serviceHistory`

### Produtos

- adicionar produto do catalogo
- adicionar produto manual
- remover produto selecionado
- marcar `Nenhum` em produto visto
- calcular valor total a partir dos produtos fechados

## Lacunas conhecidas para a futura automacao

- o realtime atual usa invalidacao + re-sync do snapshot; ainda nao existe replay de eventos nem broker externo para multiplas replicas;
- o modal ainda recebe um draft aleatorio do dominio, o que exige fixtures previsiveis para testes mais deterministas;
- ainda nao existem seeds/fixtures oficiais por cenario operacional completo;
- os testes end-to-end da operacao agora devem considerar `frontend + API + Postgres`, e depois evoluir para `frontend + API + websocket`.

## Comandos Git Bash do qa-bot

Suba o app em um terminal:

```bash
npm run dev:3001
```

Em outro terminal `Git Bash`, rode o smoke visivel:

```bash
./qa-bot/.venv/Scripts/python.exe qa-bot/main.py qa-bot/scenarios/operation_smoke.yaml --base-url http://localhost:3001 --headed --slow-mo 350 --pause-before-close
```

Variantes uteis:

```bash
# rapido
./qa-bot/.venv/Scripts/python.exe qa-bot/main.py qa-bot/scenarios/operation_smoke.yaml --base-url http://localhost:3001 --headed --slow-mo 120 --pause-before-close

# normal
./qa-bot/.venv/Scripts/python.exe qa-bot/main.py qa-bot/scenarios/operation_smoke.yaml --base-url http://localhost:3001 --headed --slow-mo 350 --pause-before-close

# inspecao detalhada
./qa-bot/.venv/Scripts/python.exe qa-bot/main.py qa-bot/scenarios/operation_smoke.yaml --base-url http://localhost:3001 --headed --slow-mo 700 --hold-open-ms 15000
```
