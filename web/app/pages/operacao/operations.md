# Pagina `operacao`

## Objetivo

Esta pagina e o cockpit operacional da fila. Ela concentra a entrada de consultores na lista da vez, o inicio de atendimentos, a pausa/retomada de pessoas e o fechamento completo do atendimento.

## Estrutura de arquivos

- `web/app/pages/operacao/index.vue`: entrada da rota `/operacao`.
- `web/app/features/operation/components/OperationWorkspace.vue`: composicao principal da pagina.
- `web/app/features/operation/components/OperationQueueColumns.vue`: coluna da fila e coluna de atendimentos em andamento.
- `web/app/features/operation/components/OperationConsultantStrip.vue`: barra inferior com todos os consultores.
- `web/app/features/operation/components/OperationFinishModal.vue`: modal de encerramento do atendimento.
- `web/app/features/operation/components/OperationProductPicker.vue`: seletor reutilizado para produtos vistos e produtos fechados.
- `web/app/stores/dashboard.ts`: ponte Pinia para as acoes do dominio.
- `core/domain/app-store.ts`: regra autoritativa atual da pagina enquanto o backend Go nao entra.

## Blocos visuais da pagina

### 1. Lista da vez

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

### 2. Em atendimento

Arquivo: `OperationQueueColumns.vue`

Responsabilidade:
- listar atendimentos ativos;
- mostrar hora de inicio, ID do atendimento, tipo de entrada e cronometro em tempo real;
- abrir o modal de encerramento.

Elementos:
- cards de atendimento ativo
- cronometro atualizado a cada segundo
- botao `Encerrar atendimento`

Comportamento:
- se nao houver atendimento ativo, mostra estado vazio;
- o rotulo do atendimento informa se ele entrou `Na vez` ou `Fora da vez`;
- quando houve queue jump, o card informa quantas pessoas foram puladas.

### 3. Barra de consultores

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

Botoes:
- `Entrar na fila`
- `Pausar`
- `Retomar`

Comportamento:
- consultor disponivel pode entrar na fila ou ser pausado;
- consultor na fila pode ser pausado;
- consultor pausado mostra o motivo da pausa e pode ser retomado;
- consultor em atendimento nao recebe acao direta por essa barra.

### 4. Modal de encerramento

Arquivo: `OperationFinishModal.vue`

Responsabilidade:
- capturar o resultado do atendimento;
- registrar produto visto e produto fechado;
- registrar dados do cliente;
- registrar motivo da visita, origem, observacoes e motivo de furar fila quando aplicavel;
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
  - acao: `dashboard.startService()`
  - efeito: remove o primeiro da fila e cria um atendimento ativo com `startMode = queue`
- botao de raio em um card fora da primeira posicao
  - acao: `dashboard.startService(personId)`
  - efeito: inicia atendimento fora da vez com `startMode = queue-jump`
- `Encerrar atendimento`
  - acao: `dashboard.openFinishModal(personId)`
  - efeito: abre o modal preenchido com um draft inicial

### `OperationConsultantStrip.vue`

- `Entrar na fila`
  - acao: `dashboard.addToQueue(personId)`
  - efeito: inclui o consultor no fim da fila com `queueJoinedAt`
- `Pausar`
  - acao: abre prompt da store `ui`, depois chama `dashboard.pauseEmployee(personId, reason)`
  - efeito: remove o consultor da fila, se estiver nela, e registra a pausa
- `Retomar`
  - acao: `dashboard.resumeEmployee(personId)`
  - efeito: remove o estado de pausa e devolve o consultor para a fila

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
  - adiciona itens vistos pelo cliente
- produto comprado/reservado
  - so aparece em `compra` ou `reserva`
  - define a base do valor vendido
- toggle `Nao informado` em motivo e origem
  - limpa a selecao atual e marca o campo como nao informado
- `Cancelar`
  - acao: `dashboard.closeFinishModal()`
- `Salvar e encerrar`
  - acao: `dashboard.finishService(personId, closureData)`
  - efeito: remove do atendimento ativo, salva historico e devolve consultor para a fila

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

## Regras de negocio atuais no dominio

As regras abaixo vivem hoje em `core/domain/app-store.ts`.

- `addToQueue(personId)`
  - ignora a acao se o consultor nao existir, ja estiver na fila, estiver em atendimento ou pausado
- `pauseEmployee(personId, reason)`
  - exige motivo nao vazio
  - ignora a acao se ja estiver pausado
  - nao pausa quem estiver em atendimento
  - remove da fila antes de registrar a pausa
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

Ao abrir o modal, a pagina usa `buildRandomFinishModalDraft(...)` do dominio para preencher um rascunho inicial. Isso existe para acelerar o MVP de teste local com `localStorage`.

Impacto:
- ajuda a simular preenchimento rapido durante demonstracoes;
- precisa ser removido ou trocado por defaults reais quando entrarmos em backend Go.

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

## Seletores estaveis para automacao

Estes `data-testid` foram adicionados para suportar testes automatizados e um robo generico no futuro.

- `operation-board`
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
- `operation-customer-profession`
- `operation-visit-reason`
- `operation-visit-reason-not-informed`
- `operation-visit-reason-detail`
- `operation-customer-source`
- `operation-customer-source-not-informed`
- `operation-customer-source-detail`
- `operation-queue-jump-reason`
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

- o estado ainda depende de `localStorage`, entao cada teste precisa controlar ou limpar a persistencia;
- o modal hoje recebe um draft aleatorio do dominio, o que exige fixtures previsiveis para testes mais deterministas;
- ainda nao existem seeds/fixtures oficiais por cenario operacional;
- quando o backend Go entrar, a fonte de verdade da fila deve sair do front e os testes precisam passar a validar API + websocket.

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
