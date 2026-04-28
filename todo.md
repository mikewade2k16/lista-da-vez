# Todo - Painel de Atendimento

## 🔴 PRIORIDADES IMEDIATAS

### 1. Botão "Na vez" - Iniciar Atendimento
- [x] Tornar botão "Na vez" clicável para iniciar atendimento
- [x] Feedback visual claro de que o atendimento foi iniciado
- [x] Evitar confusão de usuário (consultor achou que tinha iniciado sem ter)
- [x] Componente de toast customizado criado e integrado
- [x] Feedback também no botão "fora da vez"
- [x] Animações do checkmark, X e barra de progresso
- [x] Status: **CONCLUÍDO** ⚠️ *Nota: A animação de reposicionamento dos toasts ao desaparecer não foi totalmente resolvida*

### 2. Atendimento Paralelo - Consultor com múltiplos atendimentos simultâneos
> **Conceito:** o mesmo consultor pode manter 2+ atendimentos em aberto, cada um com seu proprio cronometro e modal de encerramento. Na operacao real isso costuma representar atendimentos em sequencia que ficam abertos para fechamento posterior, nao concorrencia real. Cada novo atendimento em aberto NAO consome ninguem da fila (o consultor ja esta em `service`); apenas abre outro `serviceId`. O consultor so volta para a fila apos encerrar TODOS os atendimentos abertos. O limite por consultor hoje e configurado em settings **tenant-wide** (default 1 = comportamento atual).

#### 2.1 Banco de dados (migrations) ✅
- [x] Migration `0030_active_services_parallel.sql`: trocar PK de `operation_active_services` de `(store_id, consultant_id)` para `(store_id, service_id)`
- [x] Adicionar índice não-único `(store_id, consultant_id)` em `operation_active_services`
- [x] Adicionar colunas `parallel_group_id text`, `parallel_start_index integer`, `sibling_service_ids_json jsonb`, `start_offset_ms bigint` em `operation_active_services`
- [x] Adicionar as mesmas colunas em `operation_service_history` (preservar para análise)
- [x] Atualizar check de `start_mode` para incluir `'parallel'`
- [x] Migration `0032_tenant_per_consultant_concurrency.sql`: adicionar `max_concurrent_services_per_consultant int not null default 1` em `tenant_operation_settings`, com fallback legado para leitura antiga

#### 2.2 Backend - módulo `operations` ✅
- [x] `model.go`: adicionar campos paralelo em `ActiveService`, `ActiveServiceState`, `ServiceHistoryEntry`
- [x] `model.go`: adicionar `ServiceID` em `FinishCommandInput` e `MutationAck`
- [x] `model.go`: novo `StartParallelCommandInput { StoreID, PersonID }` (sem targetIndex)
- [x] `service.go`: novo método `StartParallel` — exige consultor já em service, não toca na fila, gera novo `serviceID`, calcula `parallelGroupId`/`parallelStartIndex`/`startOffsetMs`/`siblingServiceIds`
- [x] `service.go`: `Start` (fila normal) — bloquear se consultor já em service (mantém regra atual)
- [x] `service.go`: validar limite por consultor (`max_concurrent_services_per_consultant`) antes de aceitar paralelo
- [x] `service.go`: continuar validando limite por loja (`max_concurrent_services`)
- [x] `service.go`: `Finish` passa a localizar atendimento por `ServiceID` (não mais `PersonID`); só retorna consultor para a fila quando o último atendimento ativo dele encerrar
- [x] `service.go`: status transitions simplificadas — sem transição ao iniciar 2º paralelo, transição apenas ao encerrar o último
- [x] `store_postgres.go`: ajustar SELECT/INSERT de `operation_active_services` para os novos campos
- [x] `store_postgres.go`: ajustar SELECT/INSERT de `operation_service_history` para os novos campos
- [x] `store_postgres.go`: novo `GetMaxConcurrentServicesPerConsultant(storeID)`
- [x] `http.go`: novo endpoint `POST /v1/operations/services/parallel`
- [x] `errors.go`: novos erros (`ErrConsultantNotAvailable`, `ErrConcurrentServiceLimitPerConsultantReached`)
- [x] Documentação `back/internal/modules/operations/CONCURRENT_SERVICES.md` criada

#### 2.3 Backend - módulo `settings` ✅
- [x] `model.go`: adicionar `MaxConcurrentServicesPerConsultant int` em `AppSettings` e `*int` em `AppSettingsPatch`
- [x] `service.go`: validar `>= 1` e `<= max_concurrent_services` (não faz sentido paralelo por consultor maior que limite da loja)
- [x] `defaults.go`: default 1 em todos os 3 templates (retrocompatível)
- [x] `defaults.go`: propagação do novo campo em `DefaultBundle`
- [x] `store_postgres.go`: incluir coluna em SELECT (com coalesce), INSERT, UPDATE, RETURNING
- [x] Validação circular: `normalizeAppSettings` + `applyAppSettingsPatch` validam limite

#### 2.4 Frontend - operação ✅
- [x] `OperationActiveServiceCard.vue`: novo botão "+ Iniciar outro atendimento" no rodapé do card (ao lado de "Encerrar"), visível apenas quando consultor não atingiu limite paralelo
- [x] `OperationActiveServiceCard.vue`: badge no header indicando "2/3 paralelos" quando há paralelo
- [x] `OperationActiveServiceCard.vue`: chip mostrando offset do paralelo (ex.: "iniciado 1m23s após o 1º")
- [x] `OperationQueueColumns.vue`: agrupar visualmente cards do mesmo consultor (borda compartilhada ou container)
- [x] `OperationQueueColumns.vue`: passar limite paralelo para os cards
- [x] `stores/dashboard/runtime/actions/operation-actions.ts`: nova action `startParallelService(personId)`
- [x] `stores/dashboard/runtime/actions/operation-actions.ts`: trocar `finishModalPersonId` por `finishModalServiceId` em todo o fluxo
- [x] `stores/operations.ts`: novo método de store para `startParallelService` com chamada HTTP
- [x] `OperationFinishModal.vue`: chave do draft em `sessionStorage` por `serviceId` (preserva rascunhos individuais)
- [x] `OperationFinishModal.vue`: identificar atendimento por `serviceId` no `closeFinishModal`/`finishService`
- [x] Toast customizado: "Iniciando 2º atendimento paralelo de {nome}"
- [x] Atualizar `web/app/components/AGENTS.md` se houver referência a active services (não necessário, componentes existentes melhorados)

#### 2.5 Frontend - settings
- [x] `SettingsWorkspace.vue`: novo campo "Atendimentos paralelos por consultor" (input numérico 1-5)
- [x] Texto de ajuda explicando: "Quantos atendimentos cada consultor pode manter em aberto neste tenant"
- [x] Validação client-side: `>= 1` e `<= maxConcurrentServices`

#### 2.5.1 Ajustes pós-entrega em aberto
- [x] Ajustar copy e comportamento visual para comunicar `atendimentos em aberto` / `na sequencia`, sem vender o fluxo como paralelismo real
- [x] Revisar `OperationActiveServiceCard.vue` e `OperationQueueColumns.vue` para ordenar e rotular os cards pela sequencia de abertura do mesmo consultor
- [x] Corrigir a reabertura do `OperationFinishModal.vue` quando o draft restaurado de `sessionStorage` for reutilizado; o erro atual de encerramento parece ocorrer nesse cenario
- [x] Garantir invalidacao de draft stale por `storeId:serviceId` ao reabrir, fechar ou concluir atendimento
- [ ] Redesenhar o topo do workspace para ganhar altura vertical: remover menu superior de paginas, mover navegacao para sidebar lateral e tirar a informacao de campanha ativa do topo

#### 2.6 Métricas (preparação para futuro relatório)
- [x] Garantir que `parallelGroupId`, `parallelStartIndex`, `startOffsetMs`, `siblingServiceIds` cheguem no `ServiceHistoryEntry` no encerramento
- [ ] Expor agregado `parallelism` no payload do `Snapshot` (consultor X tem N paralelos ativos)
- [ ] (Futuro) Relatório "Qualidade × Paralelismo" — comparar conversão/ticket médio entre atendimentos solo vs. paralelos

#### 2.7 Testes
- [x] Teste backend: consultor com limite 1 não consegue iniciar paralelo
- [x] Teste backend: consultor com limite 2 inicia paralelo, gera novo `serviceId`, mantém status `service`
- [x] Teste backend: encerrar 1 dos 2 paralelos NÃO devolve consultor para fila
- [x] Teste backend: encerrar o último paralelo devolve consultor para fila
- [x] Teste backend: limite de loja continua bloqueando quando atingido
- [x] Teste backend: `parallelGroupId` é o mesmo para os 2+ paralelos sobrepostos
- [ ] Teste manual UI: golden path (iniciar paralelo, ver 2 cards, encerrar cada um individualmente)
- [ ] Teste manual UI: reabrir modal com draft restaurado e confirmar encerramento sem `internal_error`

- [ ] Status: **PENDENTE**

### esse aqui depende da integração com o ERP, mas é importante ter a visão do que queremos para o futuro
### 3. Atendimentos finalizados em Compras - Auto-preenchimento via Código da Venda
- [ ] Atualização instantânea do C10 toda vez que houver uma nova venda
- [ ] Campo para preenchimento do Código da Venda no modal de encerramento
- [ ] Auto-preencher Produtos Comprados quando código informado
- [ ] Auto-preencher Dados do Cliente quando código informado
- [ ] Status: **PENDENTE**
---

### 4. ERP - Pagina administrativa atual e dados importados
- [x] Criar modulo ERP no backend com rotas autenticadas
- [x] Criar tabelas raw/projecao para produtos, clientes, funcionarios, compras e cancelados
- [x] Conectar aba Produtos ao banco `erp_item_current`
- [x] Importar/bootstrap da loja 184 para produtos, clientes, funcionarios, compras e cancelados
- [x] Exibir dados completos nas abas, sem limitar a codigo/arquivo de origem
- [x] Exibir cards de status por tipo de dado importado
- [x] Renomear Pedidos para Compras no front
- [x] Ajustar busca geral nas grades ERP
- [x] Ajustar busca especifica por aba: Compra, CPF, ID funcionario e Compra cancelada
- [x] Corrigir erro 500 da rota de Compras causado por parametros SQL inconsistentes
- [x] Validar rotas ERP de compras, clientes, funcionarios e cancelados com filtros retornando 200
- [x] Validar build do front com `npm --prefix web run build`
- [x] Validar backend com `go test ./...`
- [ ] Proximo passo: levar os dados do ERP para a Operacao e preparar automacoes
- [ ] Proximo passo: usar codigo da compra no encerramento para preencher compra, cliente, funcionario e produtos
- [ ] Status: **PENDENTE PARA INTEGRACAO COM OPERACAO**

### 5. ERP - Filtros analiticos futuros apos automacoes e operacao
- [ ] Depois das automacoes e depois de levar dados do ERP para a Operacao, criar uma pagina de cruzamento/analise
- [ ] Adicionar filtros por periodo, ex.: compras do dia X ate o dia Y
- [ ] Cruzar compras, clientes, funcionarios, cancelamentos e produtos para visoes operacionais
- [ ] Definir filtros adicionais por cliente/CPF, funcionario, forma de pagamento, SKU, loja/subloja e faixa de valor
- [ ] Status: **PENDENTE**

## 📋 ROADMAP DETALHADO

### Fase 1: Integração ERP

#### 1.1 Importação automática de CSV do ERP via FTP
- [ ] Configurar acesso ao FTP onde o ERP deposita os arquivos CSV
- [ ] Definir a pasta específica do FTP que será monitorada
- [ ] Criar rotina automática para buscar arquivos CSV (por volta de 1h da manhã)
- [ ] Criar controle para saber quais arquivos/registros já foram importados
- [ ] Criar leitura dos CSVs gerados pelo ERP
- [ ] Salvar os dados originais do ERP em uma base/tabela de referência
- [ ] Criar transformação dos dados do ERP para o formato usado no sistema
- [ ] Mapear o CSV de item do ERP para produto no banco interno
- [ ] Transformar categoria 1, categoria 2 e categoria 3 em um único campo de categorias
- [ ] Definir formato de categorias (JSON, array ou texto estruturado)
- [ ] Criar rotina incremental para importar apenas dados novos ou alterados
- [ ] Criar logs de importação
- [ ] Registrar falhas de importação
- [ ] Criar alerta caso FTP esteja inacessível
- [ ] Criar alerta caso CSV esperado não esteja disponível
- [ ] Criar alerta caso transformação dos dados falhe
- [ ] Criar opção de reprocessar manualmente uma importação, se necessário

#### 1.2 Banco intermediário e transformação dos dados do ERP
- [ ] Definir quais tabelas vão receber os dados brutos vindos do ERP
- [ ] Definir quais tabelas internas vão receber os dados já transformados
- [ ] Criar mapeamento ERP → sistema para produtos
- [ ] Criar mapeamento ERP → sistema para clientes
- [ ] Criar mapeamento ERP → sistema para ordens/pedidos de compra
- [ ] Criar regra para identificar registros novos
- [ ] Criar regra para atualizar registros existentes
- [ ] Criar regra para evitar duplicidade
- [ ] Criar validação de campos obrigatórios antes de salvar no banco interno
- [ ] Documentar o que cada coluna do CSV representa dentro do sistema

### Fase 2: Alterações do Modal de Atendimento

#### 2.1 Alteração do modal de encerramento de atendimento
- [ ] Alterar o modal atual de encerramento para usar dados vindos do ERP
- [ ] Criar campo para buscar produto por código
- [ ] Criar select/autocomplete de produto usando o código digitado
- [ ] Exibir o código e o nome do produto no resultado da busca
- [ ] Permitir adicionar múltiplos produtos de interesse
- [ ] Permitir registrar produto comprado
- [ ] Permitir registrar produto reservado
- [ ] Permitir registrar atendimento em que cliente se interessou mas não comprou
- [ ] Permitir registrar atendimento em que cliente comprou outro produto
- [ ] Permitir registrar atendimento em que cliente reservou um produto
- [ ] Salvar no atendimento a relação entre produtos de interesse, comprado e reservado
- [ ] Ajustar validações do modal conforme o tipo de encerramento (compra, reserva ou não compra)

#### 2.2 Preenchimento de cliente no encerramento
- [ ] Criar campo de CPF no modal de encerramento
- [ ] Buscar cliente pelo CPF usando os dados importados do ERP
- [ ] Preencher automaticamente os dados do cliente quando CPF for encontrado
- [ ] Definir o comportamento quando o CPF não for encontrado
- [ ] Evitar que o consultor redigite dados que já existem no ERP

#### 2.3 Uso de código da ordem do ERP em compras
- [ ] Avaliar fluxo de encerramento de compra usando código da ordem do ERP
- [ ] Criar campo para informar o código da ordem
- [ ] Buscar ordem no banco importado do ERP
- [ ] Preencher automaticamente produto vendido, cliente e demais dados da compra
- [ ] Definir se esse fluxo será usado apenas para compras
- [ ] Definir o que acontece quando o código da ordem não for encontrado

### Fase 3: Gerenciamento de Sessões e Permissões

#### 3.1 Permissões em tempo real usando WebSocket existente
- [ ] Ajustar o WebSocket existente para emitir eventos quando permissões forem alteradas
- [ ] Aplicar atualização em tempo real quando mudar permissão por tipo de usuário
- [ ] Aplicar atualização em tempo real quando mudar permissão de um usuário específico
- [ ] Atualizar permissões do usuário conectado sem precisar recarregar a página
- [ ] Fazer o frontend reagir às permissões atualizadas (esconder ou bloquear ações perdidas)
- [ ] Validar se múltiplas sessões do mesmo usuário recebem a atualização corretamente
- [ ] Criar fallback caso a atualização por WebSocket falhe (solicitar reload controlado)
- [ ] Testar alteração de permissões com usuários logados em páginas diferentes do painel

#### 3.2 Página de auditoria de usuários online
- [ ] Criar página de auditoria de usuários online
- [ ] Listar usuários conectados no sistema
- [ ] Mostrar quando um usuário estiver online em múltiplas sessões
- [ ] Mostrar sessões separadas para usuários tipo terminal de loja
- [ ] Exibir exemplo de terminal de loja logado em vários computadores
- [ ] Mostrar quantidade de sessões abertas por usuário
- [ ] Mostrar informações de cada sessão individual
- [ ] Identificar sessão por computador/dispositivo
- [ ] Mostrar última atividade da sessão
- [ ] Mostrar status de conexão da sessão
- [ ] Mostrar se o WebSocket daquela sessão parece ativo ou com falha

#### 3.3 Ações remotas em sessões
- [ ] Criar ação para recarregar uma sessão específica remotamente
- [ ] Criar ação para recarregar todas as sessões de um usuário
- [ ] Criar ação para recarregar sessões de um terminal de loja
- [ ] Permitir que o administrador resolva problemas simples sem pedir reload
- [ ] Registrar log de quem executou o reload remoto
- [ ] Registrar data e hora da ação
- [ ] Registrar qual sessão foi recarregada
- [ ] Exibir confirmação de que o comando foi enviado para a sessão

### Fase 4: Monitoramento e Alertas

#### 4.1 Monitoramento da VPS pelo painel
- [ ] Criar área no painel para visualizar status da VPS
- [ ] Exibir consumo de memória
- [ ] Exibir uso de CPU, se possível
- [ ] Exibir espaço em disco
- [ ] Exibir risco de falta de espaço
- [ ] Exibir sinais de travamento ou sobrecarga
- [ ] Definir como o painel vai receber essas métricas da VPS
- [ ] Criar script/agente de infraestrutura para coletar métricas, se necessário
- [ ] Criar histórico básico de métricas críticas

#### 4.2 Monitoramento de serviços e APIs
- [ ] Monitorar status da API principal
- [ ] Monitorar status de integrações importantes
- [ ] Monitorar falhas em serviços do sistema
- [ ] Criar alerta quando uma API cair
- [ ] Criar alerta quando algum serviço crítico parar
- [ ] Exibir esses alertas dentro do painel
- [ ] Permitir que administrador saiba do problema antes dos usuários reportarem
- [ ] Criar histórico de incidentes
- [ ] Registrar horário em que o problema começou
- [ ] Registrar horário em que o problema foi resolvido

#### 4.3 Alertas para atendimentos abertos por tempo excessivo
- [ ] Criar regra de tempo máximo esperado para um atendimento
- [ ] Detectar atendimentos abertos por tempo excessivo
- [ ] Enviar notificação para a sessão/usuário responsável
- [ ] Perguntar se o atendimento ainda está acontecendo ou se foi esquecido
- [ ] Criar alerta para administradores sobre atendimentos muito longos
- [ ] Criar regra para atendimentos que passam do horário de funcionamento da loja
- [ ] Criar regra para atendimentos que ficam abertos durante a noite
- [ ] Criar status ou marcação para atendimento encerrado após alerta
- [ ] Registrar log dos alertas enviados

#### 4.4 Encerramento manual ou automático de atendimento aberto
- [ ] Permitir que administrador encerre atendimento aberto indevidamente
- [ ] Notificar o usuário quando o administrador encerrar um atendimento
- [ ] Registrar motivo do encerramento administrativo
- [ ] Criar possibilidade de obrigar o usuário a preencher/confirmar dados pendentes depois
- [ ] Avaliar bloqueio de novo atendimento até resolver atendimento anterior
- [ ] Exibir notificação ao logar caso exista atendimento aberto do dia anterior
- [ ] Para acesso de loja compartilhado, exibir notificação em todas as sessões daquela loja
- [ ] Garantir que o consultor encerre o atendimento enquanto ainda lembra das informações

#### 4.5 Horário de funcionamento por loja
- [ ] Criar cadastro de horário de funcionamento por loja
- [ ] Permitir horários diferentes para lojas de shopping
- [ ] Permitir horários diferentes para lojas de rua
- [ ] Permitir configuração individual por cliente/loja
- [ ] Usar o horário da loja para detectar atendimento aberto fora do expediente
- [ ] Usar o horário da loja para gerar alertas automáticos
- [ ] Avaliar fechamento automático de atendimentos após expediente
- [ ] Notificar responsáveis quando houver atendimento aberto fora do horário

### Fase 5: Comunicação e Suporte

#### 5.1 Central de notificações
- [ ] Criar área de notificações dentro do painel
- [ ] Permitir envio de notificações para usuários específicos
- [ ] Permitir envio de notificações para usuários por loja
- [ ] Permitir envio de notificações por tipo de usuário
- [ ] Permitir envio de notificações gerais
- [ ] Criar notificações automáticas do sistema
- [ ] Criar status de notificação lida/não lida
- [ ] Exibir notificações em tempo real usando o WebSocket existente
- [ ] Criar níveis de importância para notificações
- [ ] Criar destaque visual para notificações importantes

#### 5.2 Sugestões, dúvidas e melhorias dos usuários
- [ ] Criar área para usuários enviarem sugestões de melhoria
- [ ] Criar área para usuários enviarem dúvidas
- [ ] Criar área para usuários reportarem problemas
- [ ] Criar formulário simples dentro do painel
- [ ] Salvar as mensagens enviadas pelos usuários
- [ ] Criar tela administrativa para visualizar essas mensagens
- [ ] Permitir classificar mensagens por tipo (dúvida, melhoria, problema ou outro)
- [ ] Permitir marcar status (novo, em análise, resolvido ou descartado)
- [ ] Notificar administradores quando uma nova mensagem for enviada

#### 5.3 Chat interno ou suporte futuro
- [ ] Avaliar criação de chat interno no painel
- [ ] Avaliar chat entre consultor e suporte
- [ ] Avaliar integração com e-mail
- [ ] Definir se o chat será uma etapa futura ou parte da primeira versão
- [ ] Usar o WebSocket existente para mensagens em tempo real, caso aprovado
- [ ] Criar histórico de conversas, se o chat for implementado

### Fase 6: Relatórios e Inteligência (Segunda Etapa)

#### 6.1 Relatórios e inteligência
- [ ] Redesenhar área de relatórios
- [ ] Melhorar layout dos relatórios atuais
- [ ] Definir quais informações são realmente críticas para o usuário
- [ ] Evitar excesso de métricas na primeira tela
- [ ] Evitar tabelas grandes logo de início
- [ ] Criar visão inicial simples e objetiva
- [ ] Mostrar indicadores principais de atendimento, venda, reserva e não compra
- [ ] Cruzar dados dos atendimentos com dados do ERP
- [ ] Cruzar produtos de interesse com produtos vendidos
- [ ] Cruzar produtos apresentados com produtos comprados
- [ ] Cruzar desempenho por loja
- [ ] Cruzar desempenho por consultor
- [ ] Cruzar tempo de atendimento com resultado
- [ ] Criar indicadores que ajudem a entender por que uma loja não está vendendo
- [ ] Criar navegação por aprofundamento (resumo primeiro, detalhes depois)
- [ ] Criar botões ou caminhos de "ver mais" para investigar uma métrica
- [ ] Usar linguagem simples para usuários leigos
- [ ] Manter profundidade sem deixar a primeira tela confusa
- [ ] Criar relatórios bonitos, simples e úteis para tomada de decisão

---

## 📊 Status Geral

- **Não iniciado:** 0
- **Em progresso:** 0
- **Concluído:** 0

---

## 📝 Notas

- Revisar e priorizar conforme novas informações forem surgindo
- Atualizar status conforme tarefas forem sendo trabalhadas
- Documentar decisões importantes de design/arquitetura
