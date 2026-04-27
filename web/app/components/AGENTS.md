# AGENTS

## Escopo

Estas instrucoes valem para `web/app/components`.

## Objetivo

Esta pasta concentra componentes reutilizaveis de pagina, workspace e UI base do frontend.

Antes de criar componente novo:

1. verificar esta lista
2. verificar se a necessidade cabe em extensao pequena de componente existente
3. evitar duplicar variacoes visuais ou selects paralelos

## Regras de reutilizacao

- para selects simples de filtro e escolha unica, preferir [AppSelectField.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ui/AppSelectField.vue)
  Ele segue a mesma linguagem visual do `.product-pick` do fechamento e deve substituir selects nativos soltos.
- para grades administrativas reutilizaveis sem `<table>`, preferir [AppEntityGrid.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ui/AppEntityGrid.vue)
- para toggles booleanos compactos em linhas administrativas, preferir [AppToggleSwitch.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ui/AppToggleSwitch.vue)
- para modal de leitura detalhada, preferir [AppDetailDialog.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ui/AppDetailDialog.vue)
- para dialogos e prompts globais, usar [AppDialogHost.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ui/AppDialogHost.vue) via `uiStore`
- para toasts globais, usar [AppToastStack.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ui/AppToastStack.vue) via `uiStore`
- para selecao pesquisavel, multi-select e detalhes por item, verificar primeiro o componente de feature [OperationProductPicker.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/features/operation/components/OperationProductPicker.vue) antes de inventar outro picker
- workspaces devem continuar finos: recebem estado/stores prontos e compoem a tela

## Catalogo atual

### `campaigns`

- [CampaignWorkspace.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/campaigns/CampaignWorkspace.vue)
  Workspace da tela de campanhas. Centraliza CRUD, regras, metas e configuracao comercial.
  Em `Todas as lojas`, deve consolidar o historico das lojas acessiveis sem mudar o escopo salvo no header e permitir filtro local por loja.

### `consultant`

- [ConsultantWorkspace.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/consultant/ConsultantWorkspace.vue)
  Workspace principal do painel do consultor.
  Em `Todas as lojas`, deve alternar para a visao integrada sem perder o escopo global do header.
- [ConsultantIntegratedWorkspace.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/consultant/ConsultantIntegratedWorkspace.vue)
  Visao consolidada de consultores entre lojas, com filtros por loja, nome, status e meta, alem de comparativo por loja e ranking consolidado.
- [ConsultantSelector.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/consultant/ConsultantSelector.vue)
  Seletor visual de consultor dentro do painel administrativo/individual.
- [ConsultantMetrics.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/consultant/ConsultantMetrics.vue)
  Cards e indicadores resumidos do desempenho do consultor.
- [ConsultantSimulator.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/consultant/ConsultantSimulator.vue)
  Simulador de impacto de vendas extras e metas no painel do consultor.

### `dashboard`

- [DashboardHeader.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/dashboard/DashboardHeader.vue)
  Header autenticado com loja ativa, conta atual e acoes de sessao.
  O select global de loja deve apenas trocar o escopo global salvo na sessao; nao deve empurrar o usuario para `/operacao`.
  A opcao `Todas as lojas` deve aparecer sempre que a sessao tiver mais de uma loja acessivel; em rotas sem suporte integrado, o header apenas informa isso e preserva a loja ativa como fonte de dados da pagina.
- [DashboardWorkspaceNav.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/dashboard/DashboardWorkspaceNav.vue)
  Navegacao principal entre workspaces do app.

### `data`

- [DataWorkspace.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/data/DataWorkspace.vue)
  Workspace da tela `/dados`.
- [InsightHourlyTable.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/data/InsightHourlyTable.vue)
  Tabela de leitura horaria/temporal.
- [InsightTagList.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/data/InsightTagList.vue)
  Lista compacta de tags/resumos de leitura.

### `intelligence`

- [IntelligenceWorkspace.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/intelligence/IntelligenceWorkspace.vue)
  Workspace da tela `/inteligencia`.
- [IntelligenceDiagnosisCard.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/intelligence/IntelligenceDiagnosisCard.vue)
  Card de diagnostico com severidade, contexto e recomendacoes.

### `multistore`

- [MultiStoreWorkspace.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/multistore/MultiStoreWorkspace.vue)
  Workspace administrativo de lojas e comparativo multiloja.
- [MultiStoreUserAccessCard.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/multistore/MultiStoreUserAccessCard.vue)
  Card de gerenciamento de acessos, papeis e onboarding de usuarios.

### `ranking`

- [RankingWorkspace.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ranking/RankingWorkspace.vue)
  Workspace da tela `/ranking`.
- [RankingTable.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ranking/RankingTable.vue)
  Tabela/ranking consolidado de consultores.

### `reports`

- [ReportsWorkspace.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/reports/ReportsWorkspace.vue)
  Workspace da tela `/relatorios`.
- [ReportsFilterToolbar.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/reports/ReportsFilterToolbar.vue)
  Barra de filtros, chips e acoes de exportacao.
- [ReportsResultsTable.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/reports/ReportsResultsTable.vue)
  Tabela principal de resultados/fechamentos.
- [ReportsQualityTable.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/reports/ReportsQualityTable.vue)
  Tabela de qualidade operacional.
- [ReportsRecentServicesTable.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/reports/ReportsRecentServicesTable.vue)
  Tabela de ultimos atendimentos para auditoria.

### `settings`

Importante: as configuracoes desta tela sao tenant-wide. Nao existe seletor
interno de loja nesta area, e o seletor de loja do header nao escopa nada
aqui. Toda alteracao salva vale para todas as lojas do tenant. Se for
necessario um override por loja para um item especifico no futuro, isso deve
ser implementado com seletor proprio dentro daquela secao, com aviso visual
explicito de override por loja.

- [SettingsWorkspace.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/settings/SettingsWorkspace.vue)
  Workspace principal de configuracoes. Mostra um banner permanente
  reforcando o escopo tenant-wide. Os payloads enviados pela
  `useSettingsStore` nao incluem `storeId` ate que um override por loja seja
  modelado.
- [SettingsTabs.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/settings/SettingsTabs.vue)
  Navegacao interna por abas de configuracao.
- [SettingsConsultantManager.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/settings/SettingsConsultantManager.vue)
  CRUD de consultores/configuracao do roster.
- [SettingsOperationTemplateManager.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/settings/SettingsOperationTemplateManager.vue)
  Aplicacao e gerenciamento de templates operacionais.
- [SettingsOptionManager.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/settings/SettingsOptionManager.vue)
  CRUD de catalogos simples como motivos, origens, pausas, fora da vez, perdas e profissao.
  Tambem concentra a ordenacao manual exibida nos selects operacionais.
- [SettingsProductManager.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/settings/SettingsProductManager.vue)
  CRUD do catalogo de produtos.

### `feedback`

- [FeedbackFormModal.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/feedback/FeedbackFormModal.vue)
  Modal flutuante para usuarios enviarem sugestoes, duvidas ou reportarem problemas.
  Acessivel via botao flutuante no layout do dashboard. Qualquer usuario autenticado pode enviar feedback.
- [FeedbackWorkspace.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/feedback/FeedbackWorkspace.vue)
  Workspace administrativo para visualizar, filtrar e responder feedbacks.
  Permitir filtros por tipo (sugestao, duvida, problema) e status (aberto, em analise, resolvido, fechado).
  Acessivel apenas para owner, manager e platform_admin.

### `ui`

- [AppDialogHost.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ui/AppDialogHost.vue)
  Host global dos dialogos e prompts do app.
- [AppDetailDialog.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ui/AppDetailDialog.vue)
  Modal reutilizavel para leitura detalhada de entidades administrativas.
- [AppEntityGrid.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ui/AppEntityGrid.vue)
  Grade CSS-grid reutilizavel para listagens administrativas com busca, filtros e colunas configuraveis.
- [AppToastStack.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ui/AppToastStack.vue)
  Host global das notificacoes/toasts. Renderiza automaticamente via Teleport. Suporta tres tipos: `success`, `error` e `info`.
  
  **Animacoes:**
  - Entrada: slide suave da direita com fade in (0.4s)
  - Ícones se desenhando: checkmark (sucesso), X (erro) ou fade in (info) com efeito de escala e rotacao (0.4s + 0.5s)
  - Barra de progresso: anima diminuindo de 100% a 0% (duração: 4s para sucesso/info, 5.5s para erro)
  - Reposicionamento: quando um toast sai, os demais sobem com animacao fluida (0.4s cubic-bezier)
  - Saida: slide e fade para direita (0.3s)
  
  **Uso:**
  ```javascript
  const ui = useUiStore();
  ui.success("Operacao concluida!", "Sucesso");  // Desaparece em 4s
  ui.error("Algo deu errado", "Erro");            // Desaparece em 5.5s  
  ui.info("Informacao importante", "Info");       // Desaparece em 4s
  ```
  
  O componente e automaticamente adicionado no layout e gerencia a pilha de notificacoes. Toasts são descartáveis pelo usuario (botão fechar).
- [AppSelectField.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ui/AppSelectField.vue)
  Select reutilizavel para filtros e escolhas simples de uma opcao, com dropdown custom no padrao visual do `product-pick`.
- [AppToggleSwitch.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ui/AppToggleSwitch.vue)
  Switch compacto para status booleanos em cards e grades administrativas.

### `users`

- [UsersAccessManager.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/users/UsersAccessManager.vue)
  Grade administrativa reutilizavel para usuarios, com cadastro via `+`, filtros, detalhes em modal editavel e acoes por icone.
  O inline da grade deve preferir patch local da linha e deixar o websocket reconciliar em segundo plano, sem reabrir o estado de loading da tabela a cada alteracao.
  Para `platform_admin`, a grade pode liberar manutencao inline de contas `consultant` quando isso for necessario no ambiente de desenvolvimento.
- [UsersRoleMatrixManager.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/users/UsersRoleMatrixManager.vue)
  Editor da matriz padrao por perfil, com visibilidade e capacidade de edicao por workspace.
- [UsersWorkspace.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/users/UsersWorkspace.vue)
  Workspace dedicado da area administrativa de usuarios, com abas para usuarios e matriz por perfil.

## Diretrizes rapidas

- se a tela for um painel inteiro, procurar primeiro um `*Workspace.vue`
- se for tabela ou card especializado, procurar primeiro na pasta de dominio correspondente
- se for acao global de notificacao ou confirmacao, usar `uiStore` com os hosts de `ui`
- se for filtro simples, nao criar novo `<select>` solto; encapsular em [AppSelectField.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ui/AppSelectField.vue) ou evoluir esse componente
