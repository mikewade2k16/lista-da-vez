# Nexo - Referencia Completa para Migracao Nuxt

Gerado em: 2026-03-19  
Atualizado em: 2026-03-30  
Fonte: leitura direta do codigo atual do MVP.  
Objetivo: servir como documento unico para migrar o sistema para Nuxt 3 / Vue 3 sem perder comportamento, regras e contratos do frontend atual.

---

## 0. Mudancas incorporadas nesta versao da referencia

Esta revisao ja considera os ajustes mais recentes aplicados no MVP:

- Modo apresentacao com `presentation.css`.
- Configuracoes separadas em tabs reais: `operacao`, `modal`, `produtos`, `consultores`, `motivos`, `origens`, `profissoes`.
- Correcao do id da tab de consultores (`consultores` no botao e no painel).
- Catalogo de profissoes com persistencia e cadastro automatico no encerramento.
- Modal de encerramento com picker de produtos com busca, multipla selecao, opcao `Nenhum` e cadastro inline de produto nao catalogado.
- Novo componente reutilizavel de picker de catalogo (`catalog-picker.js`) usado para motivo da visita e origem do cliente.
- Motivo da visita e origem do cliente agora usam o picker reutilizavel com busca, opcao `Nao informado` e detalhe opcional configuravel.
- Relatorios com filtros em painel recolhido, grupos de filtros por chip, chips ativos removiveis, acoes por icone e qualidade de preenchimento.
- KPIs de relatorio renomeados para `Media de atendimento` e `Media de espera`.
- Exportacao CSV/PDF atualizada para incluir preenchimento e os novos labels.
- Ao sair da pausa, o consultor volta direto para a fila; nao fica mais em estado intermediario disponivel.

---

## 1. Design system atual

### 1.1 Tokens globais

Arquivo: `web/app/assets/styles/tokens.css`

```css
:root {
  --bg-page: #060a12;
  --bg-shell: #060a12;
  --bg-panel: #0d121d;
  --bg-muted: #121926;
  --bg-brand: #0d121d;
  --bg-brand-strong: #060a12;
  --text-main: #e2e8f0;
  --text-muted: #94a3b8;
  --text-inverse: #f7f9fb;
  --line-soft: #1f2937;
  --line-strong: #2d3748;
  --accent-info: #38bdf8;
  --accent-warning: #fbbf24;
  --accent-success: #22c55e;
  --accent-focus: #818cf8;
  --shadow-shell: 0 28px 60px rgba(0, 0, 0, 0.64);
  --shadow-card: 0 4px 16px rgba(0, 0, 0, 0.4);
  --radius-shell: 28px;
  --radius-card: 14px;
  --radius-soft: 10px;
}
```

### 1.2 Base global

Arquivos: `web/app/assets/styles/base.css`, `web/app/assets/styles/components.css`

- Tema atual e escuro.
- `body` usa gradiente radial + linear.
- Fonte atual:
  `ui-sans-serif, system-ui, -apple-system, "SF Pro Display", "SF Pro Text", "Inter", "Segoe UI", Roboto, Arial, sans-serif`
- Scrollbar e compacta e discreta.
- Containers principais usam `#0d121d` e `#121926`.

### 1.3 Modo apresentacao

Arquivo: `web/app/assets/styles/presentation.css`

Ativacao:

No Nuxt atual, a ativacao e feita via `web/nuxt.config.ts` na chave `css`.

Efeitos principais:

- trava a pagina em 100vh;
- move scroll para areas internas;
- fixa header e barra de consultores dentro da composicao;
- compacta paddings e fontes;
- evita que paineis administrativos provoquem scroll de pagina;
- mantem a barra de tabs de configuracao alinhada no topo sem esticar botoes.

Nenhuma regra de negocio muda no modo apresentacao.

---

## 2. Estrutura geral da aplicacao

Estrutura de render atual:

```html
<div id="app">
  <main class="shell">
    <section class="app-surface">
      <AppHeader />
      <div class="workspace">
        <WorkspaceNav />
        <!-- workspace ativo -->
      </div>
    </section>
    <FinishModal />
  </main>
</div>
```

Workspaces existentes:

- `operacao`
- `consultor`
- `ranking`
- `dados`
- `inteligencia`
- `relatorios`
- `campanhas`
- `multiloja`
- `configuracoes`

Mapa por perfil:

- `admin`: todos
- `manager`: `operacao`, `consultor`, `ranking`, `dados`, `inteligencia`, `relatorios`, `campanhas`, `multiloja`
- `consultant`: `operacao`, `consultor`, `dados`

Permissoes:

```ts
canManageSettings(role)    => role === "admin"
canManageConsultants(role) => role === "admin"
canManageCampaigns(role)   => role === "admin"
canManageStores(role)      => role === "admin"
canAccessReports(role)     => role === "admin" || role === "manager"
```

Observacao:
- em runtime ainda sao usados `window.alert(...)` e `window.prompt(...)`;
- na migracao para Nuxt isso deve virar dialog/toast proprios.

---

## 3. Resumo dos workspaces

### 3.1 Operacao

- duas colunas:
  - `Lista da vez`
  - `Em atendimento`
- barra inferior de consultores com estados:
  - `available`
  - `queue`
  - `service`
  - `paused`

### 3.2 Consultor

- selector em pills;
- meta mensal;
- vendido no mes;
- comissao estimada;
- simulador de vendas adicionais;
- metricas individuais do consultor.

### 3.3 Ranking

- tabelas diaria e mensal;
- ordenacao por valor vendido, depois conversoes e taxa.

### 3.4 Dados

- leituras operacionais brutas;
- produtos, motivos, origens, profissoes e desfechos;
- inteligencia de tempo.

### 3.5 Inteligencia

- score operacional;
- cards com diagnosticos;
- acoes recomendadas;
- contexto rapido.

### 3.6 Relatorios

- filtros avancados em painel recolhivel;
- KPIs;
- qualidade de preenchimento;
- tabela de historico filtrado;
- exportacao CSV e PDF.

### 3.7 Campanhas

- CRUD de regras comerciais;
- segmentacao por origem, motivo, desfecho e cliente recorrente;
- bonus fixo e percentual.

### 3.8 Multi-loja

- consolidado por loja;
- score e metricas por snapshot;
- CRUD administrativo de lojas.

### 3.9 Configuracoes

- tabs internas para operacao, modal, produtos, consultores, motivos, origens e profissoes.

---

## 4. Workspace de operacao

### 4.1 Lista da vez

- `start-service` sem `personId`: atende o primeiro da fila.
- `start-service` com `personId`: atende fora da vez.
- quando o atendimento comeca:
  - gera `serviceId`;
  - calcula `queueWaitMs`;
  - registra `queuePositionAtStart`;
  - define `startMode`:
    - `queue`
    - `queue-jump`
  - salva `skippedPeople[]` se furou fila.

### 4.2 Em atendimento

Cada card mostra:

- consultor;
- `serviceId`;
- hora de inicio;
- modo (`Na vez` ou `Fora da vez`);
- timer vivo;
- botao para encerrar atendimento.

### 4.3 Barra de consultores

Acoes por estado:

- `available`
  - entrar na fila
  - pausar
- `queue`
  - pausar
- `paused`
  - retomar
- `service`
  - sem acoes diretas

### 4.4 Regra atual de pausa e retorno

Fluxo atual do store:

- `pauseEmployee(personId, reason)`:
  - exige motivo;
  - remove da fila se estiver nela;
  - adiciona em `pausedEmployees`;
  - troca status para `paused`.

- `resumeEmployee(personId)`:
  - remove de `pausedEmployees`;
  - reinsere o consultor no fim de `waitingList`;
  - troca status diretamente para `queue`.

Importante:
- o play ja manda para a fila;
- nao existe mais a etapa "retomar e depois entrar na fila".

---

## 5. FinishModal atual

Arquivos principais:

- `web/app/components/operation/OperationFinishModal.vue`
- `web/app/components/operation/OperationProductPicker.vue`
- `web/app/stores/dashboard.ts`
- `web/app/stores/dashboard/runtime/create-dashboard-runtime.ts`

### 5.1 Estrutura geral

O modal continua sendo um overlay fixo com:

- header com titulo configuravel;
- subtitulo com nome do consultor e `serviceId`;
- formulario `data-action="finish-service-form"`.

### 5.2 Campos de desfecho e flags

Desfechos:

- `reserva`
- `compra`
- `nao-compra`

Flags:

- `is-window-service`
- `is-gift`
- `is-existing-customer`

Comportamento reativo:

- `is-gift` so aparece para `compra` ou `reserva`;
- o bloco de produtos fechados tambem so aparece para `compra` ou `reserva`.

### 5.3 Picker de produtos

O fluxo antigo de input simples nao existe mais.

Agora ha um widget de produto com:

- dropdown;
- campo de busca;
- opcao de escolher produto do catalogo;
- opcao de cadastrar produto nao catalogado inline;
- chips para produtos vistos;
- lista selecionada + total para produtos comprados/reservados.

#### Produto visto

Comportamento:

- multipla selecao visual;
- botao `Nenhum`;
- cadastro manual de produto nao catalogado;
- persistencia no submit como `productsSeen[]`.

#### Produto comprado / reservado

Comportamento:

- multipla selecao visual;
- cadastro manual de produto nao catalogado;
- soma automatica do total;
- persistencia no submit como `productsClosed[]`.

#### Observacao critica de migracao

Hoje o submit envia `productsSeen[]` e `productsClosed[]`, e o historico persistido mantem os arrays junto com os campos derivados:

- `productsSeen[]` = lista completa de itens vistos;
- `productsClosed[]` = lista completa de itens fechados;
- `productSeen` = nome do primeiro item visto;
- `productClosed` = nome do primeiro item fechado;
- `productDetails` = `productClosed || productSeen`;
- `saleAmount` = soma dos `productsClosed`.

Para a API/banco, a recomendacao e tratar os arrays como fonte de verdade e manter os campos escalares apenas por compatibilidade.

### 5.4 Dados do cliente

Campos:

- `customer-name`
- `customer-phone`
- `customer-email` se `showEmailField === true`
- `customer-profession` se `showProfessionField === true`

Profissao agora funciona assim:

- select com profissoes cadastradas;
- opcao `Cadastrar nova profissao`;
- ao escolher `__custom__`, abre campo `customer-profession-custom`;
- no `finishService`, se for uma profissao nova, ela e incorporada ao catalogo `professionOptions`.

### 5.5 Motivo da visita e origem do cliente

Mudanca estrutural importante:

- nao sao mais listas de checkboxes expandidas;
- agora usam o componente reutilizavel `catalog-picker.js`.

Regras atuais:

- selecao unica;
- busca por texto dentro do dropdown;
- opcao `Nao informado`;
- chip da escolha atual com botao de remover;
- detalhe opcional so aparece quando ha uma opcao real selecionada.

Campos gerados:

- `visit-reasons`
- `visit-reasons-none`
- `visit-reason-detail`
- `customer-sources`
- `customer-sources-none`
- `customer-source-detail`

Observacao:
- a camada de dados continua aceitando arrays (`visitReasons[]`, `customerSources[]`) por compatibilidade;
- `Motivo da visita` agora aceita multisselecao na UI;
- `Origem do cliente` pode operar como escolha unica ou multipla via configuracao.

### 5.6 Relacao analitica entre motivo e desfecho

Regra de negocio desejada:

- motivo da visita nao deve restringir `compra`, `reserva` ou `nao-compra`;
- todo motivo aceita qualquer desfecho;
- o valor dessa relacao e analitico, para relatorios e inteligencia operacional.

Estado atual do MVP:

- o modal nao restringe motivo por desfecho;
- se algum payload legado ainda trouxer `outcomes[]`, esse dado deve ser ignorado.

### 5.7 Motivo fora da vez e observacoes

Campos condicionais:

- `queue-jump-reason`
  - aparece apenas quando `service.startMode === "queue-jump"`
  - obrigatorio nesse caso
- `notes`
  - aparece se `showNotesField === true`

### 5.8 Validacoes exatas do submit

Ordem atual:

1. sem `outcome` -> alerta.
2. `requireVisitReason && visitReasons.length === 0 && !visitReasonsNotInformed` -> alerta.
3. `requireProduct && productsSeen.length === 0 && !productsSeenNone` -> alerta.
4. `(outcome === "compra" || outcome === "reserva") && requireProduct && productsClosed.length === 0` -> alerta.
5. `requireCustomerNamePhone && (!customerName || !customerPhone)` -> alerta.
6. `requireCustomerSource && customerSources.length === 0 && !customerSourcesNotInformed` -> alerta.
7. `activeService.startMode === "queue-jump" && !queueJumpReason` -> alerta.

Importante:
- nao existe mais input manual de `saleAmount`;
- `saleAmount` e derivado da soma de `productsClosed`.

### 5.9 Payload logico do fechamento

Shape atual usado em `store.finishService(...)`:

```ts
{
  outcome: "reserva" | "compra" | "nao-compra"
  isWindowService: boolean
  isGift: boolean
  productSeen: string
  productClosed: string
  productsSeen: Array<{ id: string, name: string, price: number, code?: string, isCustom?: boolean }>
  productsClosed: Array<{ id: string, name: string, price: number, code?: string, isCustom?: boolean }>
  productDetails: string
  customerName: string
  customerPhone: string
  customerEmail: string
  customerProfession: string
  isExistingCustomer: boolean
  visitReasons: string[]        // multisselecao suportada na UI
  visitReasonDetails: Record<string, string>
  customerSources: string[]     // escolha unica ou multipla conforme configuracao
  customerSourceDetails: Record<string, string>
  saleAmount: number            // soma de productsClosed
  queueJumpReason: string
  notes: string
}
```

Observacao:
- `productsSeen[]` e `productsClosed[]` sao a fonte de verdade recomendada para API/banco;
- `productSeen`, `productClosed` e `productDetails` permanecem como campos derivados/legados para compatibilidade.

### 5.10 Auto-fill de teste

Quando `settings.testModeEnabled` e `settings.autoFillFinishModal` estao ativos:

- o draft agora gera 1 motivo da visita;
- gera 1 origem;
- preenche produtos, cliente, observacoes e detalhes automaticamente.

---

## 6. Relatorios operacionais

Arquivos principais:

- `web/app/components/reports/ReportsWorkspace.vue`
- `web/app/domain/utils/reports.ts`
- `web/app/utils/report-export.ts`
- `web/app/stores/dashboard.ts`

### 6.1 Contrato atual de filtros

Shape persistido:

```ts
{
  dateFrom: string
  dateTo: string
  consultantIds: string[]
  outcomes: string[]
  sourceIds: string[]
  visitReasonIds: string[]
  startModes: string[]
  existingCustomerModes: string[]
  completionLevels: string[]
  minSaleAmount: string
  maxSaleAmount: string
  search: string
}
```

Compatibilidade:

- `normalizeReportFilters()` ainda aceita formatos legados singulares:
  - `consultantId`
  - `outcome`
  - `sourceId`
  - `visitReasonId`
  - `startMode`
  - `existingCustomer`
  - `completionLevel`

### 6.2 UI atual dos filtros

O relatorio nao usa mais um grid fixo de filtros sempre aberto.

Agora o fluxo e:

1. card `Filtros`;
2. icone `filter_alt` para abrir/fechar;
3. grupos de filtro em chips:
   - `Consultor`
   - `Desfecho`
   - `Origem`
   - `Motivo`
   - `Tipo`
   - `Cliente`
   - `Preenchimento`
   - `Periodo e busca`
4. chips ativos no topo;
5. remocao individual por `close`;
6. limpar tudo por icone `filter_alt_off`;
7. exportacoes por icone:
   - `table_view`
   - `picture_as_pdf`

Estado de UI:

```ts
reportUiState = {
  filtersExpanded: boolean
  expandedGroup: string | null
}
```

Importante:
- `reportUiState` e efemero;
- nao vai para `localStorage`;
- apenas os filtros em si sao persistidos.

### 6.3 KPIs atuais

Cards exibidos:

- `Atendimentos`
- `Conversao`
- `Valor vendido`
- `Ticket medio`
- `Media de atendimento`
- `Media de espera`
- `Fora da vez`
- `Bonus campanhas`

### 6.4 Qualidade de preenchimento

Nova camada de analise em `buildReportData()`:

Campos-base avaliados:

- `customerName`
- `customerPhone`
- `productClosed || productSeen || productDetails`
- `visitReasons`
- `customerSources`

Classificacao:

- `excellent` -> `Completo + observacao`
- `complete` -> `Completo`
- `incomplete` -> `Incompleto`

Regra:

- completo = todos os campos-base preenchidos;
- completo + observacao = completo + `notes`;
- incompleto = faltou qualquer item-base.

### 6.5 Niveis por consultor

Resolucao atual:

- `Destaque`
  - `completeRate >= 85`
  - `excellentRate >= 35`
- `Consistente`
  - `completeRate >= 70`
- `Precisa melhorar`
  - resto

Tabela por consultor:

- Consultor
- Atendimentos
- Completo
- Completo + obs
- Incompleto
- Observacoes
- Nivel

### 6.6 Tabela principal do relatorio

Colunas atuais:

- Loja
- Data/Hora
- Consultor
- Desfecho
- Valor
- Duracao
- Espera fila
- Preenchimento
- Modo
- Cliente
- Origem
- Campanhas

Observacao:
- a tela limita a exibicao aos primeiros 200 registros;
- o total real continua mostrado no header do card.

### 6.7 Exportacoes

CSV agora inclui:

- preenchimento;
- observacoes;
- motivos;
- origens;
- bonus de campanha.

PDF agora usa os labels atuais:

- `Media de atendimento`
- `Media de espera`

### 6.8 Busca livre

A busca textual filtra por:

- loja;
- `serviceId`;
- nome do consultor;
- nome do cliente;
- telefone;
- email;
- profissao;
- produto visto;
- produto fechado;
- `productDetails`;
- observacoes.

---

## 7. Configuracoes

Arquivo principal: `web/app/components/settings/SettingsWorkspace.vue`

### 7.1 Tabs atuais

As tabs internas do painel sao:

```ts
[
  "operacao",
  "modal",
  "produtos",
  "consultores",
  "motivos",
  "origens",
  "profissoes"
]
```

Observacao:
- a tab de consultores precisava usar `consultores` tanto no botao quanto no `data-tab-panel`;
- essa referencia ja considera a correcao aplicada.

### 7.2 Aba Operacao

Conteudo:

- templates de operacao;
- limites e timings:
  - `maxConcurrentServices`
  - `timingFastCloseMinutes`
  - `timingLongServiceMinutes`
  - `timingLowSaleAmount`
  - `testModeEnabled`
  - `autoFillFinishModal`

### 7.3 Aba Modal

Textos configuraveis:

- `title`
- `customerSectionLabel`
- `notesLabel`
- `notesPlaceholder`
- `queueJumpReasonLabel`
- `queueJumpReasonPlaceholder`
- `productSeenLabel`
- `productSeenPlaceholder`
- `productClosedLabel`
- `productClosedPlaceholder`

Toggles:

- `showEmailField`
- `showProfessionField`
- `showNotesField`
- `showVisitReasonDetails`
- `showCustomerSourceDetails`
- `requireProduct`
- `requireVisitReason`
- `requireCustomerSource`
- `requireCustomerNamePhone`

### 7.4 Aba Produtos

CRUD do catalogo:

- nome;
- categoria;
- preco base.

Usos atuais:

- picker de produtos do modal;
- auto-fill de teste.

### 7.5 Aba Consultores

CRUD administrativo:

- nome;
- cargo;
- cor;
- meta mensal;
- comissao.

### 7.6 Abas Motivos e Origens

Cada grupo usa `OptionManager`:

- editar label;
- salvar;
- excluir;
- adicionar nova opcao.

Grupos:

- `visit-reason`
- `customer-source`

### 7.7 Aba Profissoes

Novo grupo dedicado:

- lista de profissoes cadastradas;
- CRUD manual;
- usado no modal de encerramento;
- profissao nova digitada no modal tambem entra nesse catalogo automaticamente.

---

## 8. Outros paineis administrativos

### 8.1 Campanhas

Permanece com:

- CRUD completo;
- filtros por origem e motivo;
- bonus fixo + percentual;
- `queueJumpOnly`;
- `existingCustomerFilter`.

### 8.2 Multi-loja

Mantem:

- consolidado por snapshot;
- CRUD de lojas;
- troca de contexto salvando espelho atual e carregando o da nova loja;
- fechamento forcado do modal na troca de loja.

### 8.3 Consultor, Ranking, Dados e Inteligencia

Esses paineis nao tiveram mudanca estrutural recente de regra de negocio, mas seguem ativos e devem ser migrados com o comportamento atual do MVP.

---

## 9. Estado global e persistencia

Arquivos principais:

- `web/app/stores/app-runtime.ts`
- `web/app/stores/dashboard/runtime/create-dashboard-runtime.ts`
- `web/app/domain/data/mock-queue.ts`

### 9.1 Campos importantes do estado

Estado atual inclui, entre outros:

```ts
{
  configSchemaVersion: 4
  brandName: string
  pageTitle: string
  profiles: Profile[]
  activeProfileId: string
  stores: Store[]
  activeStoreId: string
  storeSnapshots: Record<string, StoreSnapshot>
  activeWorkspace: WorkspaceId
  selectedConsultantId: string | null
  consultantSimulationAdditionalSales: number
  operationTemplates: OperationTemplate[]
  selectedOperationTemplateId: string
  reportFilters: ReportFilters
  campaigns: Campaign[]
  waitingList: QueueEntry[]
  activeServices: ActiveService[]
  roster: Consultant[]
  finishModalDraft: object | null
  finishModalPersonId: string | null
  visitReasonOptions: Option[]
  customerSourceOptions: Option[]
  professionOptions: Option[]
  productCatalog: Product[]
  modalConfig: object
  consultantActivitySessions: ActivitySession[]
  consultantCurrentStatus: Record<string, { status: string, startedAt: number }>
  pausedEmployees: { personId: string, reason: string, startedAt: number }[]
  settings: object
  serviceHistory: HistoryEntry[]
}
```

### 9.2 Persistencia atual

Estado atual:

- `auth`, `tenants`, `stores`, `consultants` e `settings` ja usam backend Go + PostgreSQL.
- `GET /v1/settings` continua entregando um bundle completo para bootstrap da UI.
- as escritas de `settings` passaram a ser setoriais:
  - `PATCH /v1/settings/operation`
  - `PATCH /v1/settings/modal`
  - `PUT /v1/settings/options/{group}`
  - `PUT /v1/settings/products`
- a operacao ainda nao esta no backend; fila, atendimento ativo e historico continuam apenas em memoria via `app-runtime`.
- `localStorage` deixou de ser usado no codigo ativo do frontend.

### 9.3 Normalizacao de schema

`queue-service.js`:

- normaliza estado legado;
- garante `configSchemaVersion = 4`;
- reidrata historicos antigos;
- preserva compatibilidade com formatos anteriores de relatorio;
- reaproveita `mockQueueState` como base segura.

---

## 10. Fluxos criticos do store

### 10.1 `finishService(personId, closureData)`

Fluxo atual:

1. localiza o atendimento ativo;
2. calcula `durationMs`;
3. monta `historyEntry`;
4. injeta loja ativa (`storeId`, `storeName`);
5. soma bonus de campanha;
6. remove de `activeServices`;
7. reinsere consultor no fim da fila;
8. adiciona ao historico;
9. fecha modal;
10. muda status do consultor para `queue`;
11. se a profissao for nova, adiciona em `professionOptions`.

### 10.2 `setActiveStore(storeId)`

Fluxo:

1. salva snapshot da loja atual;
2. carrega snapshot da loja nova;
3. fecha modal;
4. sincroniza espelho e snapshots.

### 10.3 `resumeEmployee(personId)`

Fluxo atual:

1. tira de `pausedEmployees`;
2. recoloca na fila;
3. muda status para `queue`.

---

## 11. Contrato atual de acoes e eventos

### 11.1 Clicks principais

| action | dataset adicional | observacao |
|---|---|---|
| `set-workspace` | `data-workspace-id` | troca workspace |
| `set-active-store` | `data-store-id` | troca loja no header ou em multi-loja |
| `select-consultant` | `data-person-id` | painel de consultor |
| `add-to-queue` | `data-person-id` | envia consultor para fila |
| `pause-employee` | `data-person-id` | pausa com prompt |
| `resume-employee` | `data-person-id` | volta direto para fila |
| `start-service` | `data-person-id` opcional | null = primeiro da fila |
| `open-finish-modal` | `data-person-id` | abre encerramento |
| `close-finish-modal` | - | fecha modal |
| `set-settings-tab` | `data-tab` | tabs internas de configuracao |
| `toggle-report-filters` | - | abre/fecha painel de filtros |
| `toggle-report-filter-group` | `data-filter-group` | abre grupo de filtro |
| `toggle-report-filter-value` | `data-filter-id`, `data-filter-value` | alterna item multi-select dos filtros |
| `clear-report-filter` | `data-filter-id`, `data-filter-value` opcional | remove chip/filtro |
| `reset-report-filters` | - | limpa tudo |
| `export-report-csv` | - | baixa CSV |
| `export-report-pdf` | - | abre impressao |
| `apply-operation-template` | `data-template-id` | aplica preset |
| `archive-consultant` | `data-consultant-id` | arquiva consultor |
| `remove-option` | `data-option-group`, `data-option-id` | motivos, origens ou profissoes |
| `remove-product` | `data-product-id` | usado no catalogo e no modal |
| `remove-campaign` | `data-campaign-id` | remove campanha |
| `archive-store` | `data-store-id` | arquiva loja |

### 11.2 Acoes do picker de produto

Usadas no modal:

- `product-pick-toggle`
- `product-pick-select`
- `product-pick-custom-toggle`
- `product-pick-custom-cancel`
- `product-pick-custom-add`
- `product-pick-none-toggle`
- `remove-product`

### 11.3 Acoes do picker reutilizavel de catalogo

Usadas para motivo e origem:

- `option-pick-toggle`
- `option-pick-select`
- `option-pick-clear`

### 11.4 Changes

| action | dataset adicional | origem |
|---|---|---|
| `set-active-profile` | - | header |
| `set-active-store` | - | header select |
| `set-report-filter` | `data-filter-id` | bloco `Periodo e busca` |
| `set-simulation-value` | - | painel consultor |
| `set-setting` | `data-setting-id` | configuracoes gerais |
| `set-modal-config` | `data-config-key` | configuracoes do modal |
| `update-product` | `data-product-id`, `data-product-field` | catalogo de produtos |

### 11.5 Input e keydown

Eventos adicionais registrados no app:

- `input`:
  - filtra dropdowns que tenham `data-picker-search-input`
- `keydown`:
  - previne submit acidental com `Enter` no search dos pickers

### 11.6 Forms

| action | dataset adicional | payload |
|---|---|---|
| `add-option` | `data-option-group` | `label` |
| `update-option` | `data-option-group`, `data-option-id` | `label` |
| `add-product` | - | `name`, `category`, `basePrice` |
| `add-consultant` | - | `name`, `role`, `color`, `monthlyGoal`, `commissionRate` |
| `update-consultant` | `data-consultant-id` | mesmo shape |
| `add-store` | - | `name`, `code`, `city`, `clone-active-roster` |
| `update-store` | `data-store-id` | `name`, `code`, `city` |
| `add-campaign` | - | payload completo de campanha |
| `update-campaign` | `data-campaign-id` | payload completo |
| `finish-service-form` | `data-person-id` | fechamento completo |

---

## 12. Bootstrap e timers

Ordem atual:

```txt
1. renderApp()
2. addEventListener("click", handleClick)
3. addEventListener("change", handleChange)
4. addEventListener("input", handleInput)
5. addEventListener("keydown", handleKeydown)
6. addEventListener("submit", handleSubmit)
7. store.subscribe(renderApp)
8. store.subscribe(saveQueueState)
9. loadQueueState()
10. store.hydrate(initialState)
11. setInterval(1000)
```

Guardas do intervalo:

- so atualiza se houver timers ativos;
- nao atualiza se o modal estiver aberto;
- nao atualiza se houver `input`, `select` ou `textarea` focado;
- em `operacao` atualiza so o texto dos timers sem rerender completo;
- em `dados`, `inteligencia` e `multiloja` pode rerenderizar.

---

## 13. Exportacao e relatorio impresso

Arquivo: `web/app/utils/report-export.ts`

CSV:

- delimitador `;`;
- nome `relatorio-nexo-{timestamp}.csv`;
- inclui preenchimento e observacoes.

PDF:

- ainda usa `window.open(...)` + `window.print()`;
- sem biblioteca externa de PDF;
- isso deve ser mantido na migracao inicial ou substituido conscientemente.

---

## 14. Estrutura recomendada para Nuxt 3

Sugestao atualizada:

```txt
app/
  pages/
    index.vue
  components/
    layout/
      AppHeader.vue
      WorkspaceNav.vue
    operation/
      QueueColumn.vue
      QueueCard.vue
      ServiceCard.vue
      ConsultantStrip.vue
    modal/
      FinishModal.vue
      ProductPick.vue
      CatalogPicker.vue
    reports/
      ReportsPanel.vue
      ReportFilterToolbar.vue
      ReportQualityTable.vue
      ReportResultsTable.vue
    settings/
      SettingsPanel.vue
      SettingsTabs.vue
      OptionManager.vue
      ProductManager.vue
      ConsultantCrudManager.vue
    panels/
      ConsultorPanel.vue
      RankingPanel.vue
      DadosPanel.vue
      InteligenciaPanel.vue
      CampanhasPanel.vue
      MultiLojaPanel.vue
  stores/
    app.ts
  composables/
    usePermissions.ts
    useReportFilters.ts
    useReportData.ts
    useCampaigns.ts
    useOperationTimers.ts
    useFinishModal.ts
    useCatalogPicker.ts
  services/
    queueStorage.ts
    reportExport.ts
  utils/
    reports.ts
    campaigns.ts
    time.ts
    object.ts
  assets/
    css/
      tokens.css
      base.css
      components.css
      presentation.css
```

---

## 15. Pontos criticos para nao perder na migracao

1. O modal ja esta reativo em Vue/Nuxt e precisa manter as mesmas regras quando a operacao migrar para a API.
2. O picker reutilizavel de catalogo ja existe conceitualmente; vale manter essa abstracao em Vue.
3. Motivo ja opera com multisselecao; origem pode ser single ou multi por configuracao.
4. `productsSeen[]` e `productsClosed[]` ja persistem no historico e devem seguir como fonte de verdade na API; os campos string sao derivados por compatibilidade.
5. Ao trocar de loja, o modal deve fechar obrigatoriamente.
6. Ao sair da pausa, o consultor deve voltar direto para a fila.
7. `reportUiState` nao e persistido; apenas `reportFilters`.
8. A referencia atual considera `configSchemaVersion: 4`.
9. `window.alert` e `window.prompt` devem ser trocados por componentes proprios.
10. Exportacao PDF ainda depende de `window.print()`.

---

## 16. Status do backlog real apos estas mudancas

### Ja entregue no MVP atual

- Fila completa com atendimento na vez e fora da vez.
- Retorno automatico do consultor para fila ao encerrar atendimento.
- Pausa e retomada com retorno direto para fila.
- Modal de encerramento com produto visto, produto fechado, cliente, observacoes, motivo e origem.
- Catalogo de produtos com busca e cadastro inline no modal.
- Catalogo de profissoes com persistencia e cadastro automatico.
- Configuracoes separadas por tabs.
- CRUD de consultores.
- Relatorios com filtros avancados, qualidade de preenchimento e exportacao.
- Campanhas.
- Multi-loja.

### Ainda pendente

- Persistencia backend real da operacao.
- Integracao real de produtos via API.

---

## 17. Arquivos-chave para a migracao

Se for reconstruir por partes, estes sao os arquivos mais relevantes do estado atual:

- `web/app/stores/dashboard.ts`
- `web/app/stores/dashboard/runtime/create-dashboard-runtime.ts`
- `web/app/stores/app-runtime.ts`
- `web/app/components/operation/OperationFinishModal.vue`
- `web/app/components/operation/OperationProductPicker.vue`
- `web/app/components/reports/ReportsWorkspace.vue`
- `web/app/domain/utils/reports.ts`
- `web/app/utils/report-export.ts`
- `web/app/components/settings/SettingsWorkspace.vue`
- `web/app/components/operation/OperationConsultantStrip.vue`
- `web/app/assets/styles/tokens.css`
- `web/app/assets/styles/base.css`
- `web/app/assets/styles/components.css`
- `web/app/assets/styles/presentation.css`
