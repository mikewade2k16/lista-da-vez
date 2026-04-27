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
> **Conceito:** o mesmo consultor pode tocar 2+ clientes ao mesmo tempo, cada atendimento com seu próprio cronômetro e modal de encerramento. Cada atendimento paralelo NÃO consome ninguém da fila (o consultor já está em service); só abre outro cronômetro. O consultor só volta para a fila após encerrar TODOS os atendimentos paralelos. O limite de paralelos é configurado **por loja** (default 1 = comportamento atual).

#### 2.1 Banco de dados (migrations)
- [ ] Migration `0030_active_services_parallel.sql`: trocar PK de `operation_active_services` de `(store_id, consultant_id)` para `(store_id, service_id)`
- [ ] Adicionar índice não-único `(store_id, consultant_id)` em `operation_active_services`
- [ ] Adicionar colunas `parallel_group_id text`, `parallel_start_index integer`, `sibling_service_ids_json jsonb`, `start_offset_ms bigint` em `operation_active_services`
- [ ] Adicionar as mesmas colunas em `operation_service_history` (preservar para análise)
- [ ] Atualizar check de `start_mode` para incluir `'parallel'`
- [ ] Migration `0031_per_consultant_concurrency.sql`: adicionar `max_concurrent_services_per_consultant int not null default 1` em `store_operation_settings`

#### 2.2 Backend - módulo `operations`
- [ ] `model.go`: adicionar campos paralelo em `ActiveService`, `ActiveServiceState`, `ServiceHistoryEntry`
- [ ] `model.go`: adicionar `ServiceID` em `FinishCommandInput` e `MutationAck`
- [ ] `model.go`: novo `StartParallelCommandInput { StoreID, PersonID }` (sem targetIndex)
- [ ] `service.go`: novo método `StartParallel` — exige consultor já em service, não toca na fila, gera novo `serviceID`, calcula `parallelGroupId`/`parallelStartIndex`/`startOffsetMs`/`siblingServiceIds`
- [ ] `service.go`: `Start` (fila normal) — bloquear se consultor já em service (mantém regra atual)
- [ ] `service.go`: validar limite por consultor (`max_concurrent_services_per_consultant`) antes de aceitar paralelo
- [ ] `service.go`: continuar validando limite por loja (`max_concurrent_services`)
- [ ] `service.go`: `Finish` passa a localizar atendimento por `ServiceID` (não mais `PersonID`); só retorna consultor para a fila quando o último atendimento ativo dele encerrar
- [ ] `service.go`: `applyStatusTransitions` deve ignorar transição quando o consultor já está em `service` e ganha mais um paralelo
- [ ] `store_postgres.go`: ajustar SELECT/INSERT de `operation_active_services` para os novos campos
- [ ] `store_postgres.go`: ajustar SELECT/INSERT de `operation_service_history` para os novos campos
- [ ] `store_postgres.go`: novo `GetMaxConcurrentServicesPerConsultant(storeID)`
- [ ] `http.go`: novo endpoint `POST /api/operations/services/parallel`
- [ ] `permissions.go`: paralelo herda mesma permissão de `operations:edit`
- [ ] Atualizar `back/internal/modules/operations/AGENTS.md` (ou criar) com a nova regra

#### 2.3 Backend - módulo `settings`
- [ ] `model.go`: adicionar `MaxConcurrentServicesPerConsultant int` em `OperationSettings` e `*int` no patch
- [ ] `service.go`: validar `>= 1` e `<= max_concurrent_services` (não faz sentido paralelo por consultor maior que limite da loja)
- [ ] `defaults.go`: default 1 em todos os templates (retrocompatível)
- [ ] `store_postgres.go`: incluir coluna em todos os SELECT/INSERT/UPDATE
- [ ] Atualizar `back/internal/modules/settings/AGENTS.md` se existir

#### 2.4 Frontend - operação
- [ ] `OperationActiveServiceCard.vue`: novo botão "+ Iniciar outro atendimento" no rodapé do card (ao lado de "Encerrar"), visível apenas quando consultor não atingiu limite paralelo
- [ ] `OperationActiveServiceCard.vue`: badge no header indicando "2/3 paralelos" quando há paralelo
- [ ] `OperationActiveServiceCard.vue`: chip mostrando offset do paralelo (ex.: "iniciado 1m23s após o 1º")
- [ ] `OperationQueueColumns.vue`: agrupar visualmente cards do mesmo consultor (borda compartilhada ou container)
- [ ] `OperationQueueColumns.vue`: passar limite paralelo para os cards
- [ ] `stores/dashboard/runtime/actions/operation-actions.ts`: nova action `startParallelService(personId)`
- [ ] `stores/dashboard/runtime/actions/operation-actions.ts`: trocar `finishModalPersonId` por `finishModalServiceId` em todo o fluxo
- [ ] `stores/operations.ts`: novo método de store para `startParallelService` com chamada HTTP
- [ ] `OperationFinishModal.vue`: chave do draft em `sessionStorage` por `serviceId` (preserva rascunhos individuais)
- [ ] `OperationFinishModal.vue`: identificar atendimento por `serviceId` no `closeFinishModal`/`finishService`
- [ ] Toast customizado: "Iniciando 2º atendimento paralelo de {nome}"
- [ ] Atualizar `web/app/components/AGENTS.md` se houver referência a active services

#### 2.5 Frontend - settings
- [ ] `SettingsWorkspace.vue`: novo campo "Atendimentos paralelos por consultor" (input numérico 1-5)
- [ ] Texto de ajuda explicando: "Quantos atendimentos cada consultor pode tocar simultaneamente nesta loja"
- [ ] Validação client-side: `>= 1` e `<= maxConcurrentServices`

#### 2.6 Métricas (preparação para futuro relatório)
- [ ] Garantir que `parallelGroupId`, `parallelStartIndex`, `startOffsetMs`, `siblingServiceIds` cheguem no `ServiceHistoryEntry` no encerramento
- [ ] Expor agregado `parallelism` no payload do `Snapshot` (consultor X tem N paralelos ativos)
- [ ] (Futuro) Relatório "Qualidade × Paralelismo" — comparar conversão/ticket médio entre atendimentos solo vs. paralelos

#### 2.7 Testes
- [ ] Teste backend: consultor com limite 1 não consegue iniciar paralelo
- [ ] Teste backend: consultor com limite 2 inicia paralelo, gera novo `serviceId`, mantém status `service`
- [ ] Teste backend: encerrar 1 dos 2 paralelos NÃO devolve consultor para fila
- [ ] Teste backend: encerrar o último paralelo devolve consultor para fila
- [ ] Teste backend: limite de loja continua bloqueando quando atingido
- [ ] Teste backend: `parallelGroupId` é o mesmo para os 2+ paralelos sobrepostos
- [ ] Teste manual UI: golden path (iniciar paralelo, ver 2 cards, encerrar cada um individualmente)

- [ ] Status: **PENDENTE**

### esse aqui depende da integração com o ERP, mas é importante ter a visão do que queremos para o futuro
### 3. Atendimentos finalizados em Compras - Auto-preenchimento via Código da Venda
- [ ] Atualização instantânea do C10 toda vez que houver uma nova venda
- [ ] Campo para preenchimento do Código da Venda no modal de encerramento
- [ ] Auto-preencher Produtos Comprados quando código informado
- [ ] Auto-preencher Dados do Cliente quando código informado
- [ ] Status: **PENDENTE**
---

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
