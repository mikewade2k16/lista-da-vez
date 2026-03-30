# Nexo MVP -> Nuxt Migration Blueprint

Atualizado em: 2026-03-23  
Objetivo: este documento descreve **todo o comportamento funcional e tecnico** implementado no MVP atual (HTML + JS) para migracao 1:1 para Nuxt, sem perda de regra de negocio, fluxo de dados, UX, permissoes e metricas.

## 1. Escopo funcional atual

### 1.1 Operacao de fila

- Entrada de consultor na fila pela barra de Consultores.
- Atendimento normal: "Atender primeiro da fila".
- Atendimento fora da vez: botao em cada card da fila (exceto primeiro).
- Encerramento de atendimento abre modal obrigatorio de fechamento.
- Ao encerrar atendimento, consultor retorna automaticamente ao fim da fila.
- Pausa e retomada de consultor (pausa exige motivo; retomada em 1 clique).
- Limite simultaneo de atendimentos configuravel (`settings.maxConcurrentServices`, default 10).

### 1.2 Coleta de dados no encerramento

- Desfecho obrigatorio: `reserva`, `compra`, `nao-compra`.
- Produto visto (`productSeen`) e produto fechado (`productClosed` em compra/reserva).
- Valor da venda/reserva derivado automaticamente da soma de `productsClosed` (nao existe input manual de valor).
- Campos de cliente:
- Nome e telefone obrigatorios (configuravel).
- Email opcional (configuravel).
- Profissao opcional (configuravel).
- Motivo da visita: selecao unica na UI atual + detalhe opcional por item (dominio continua aceitando array por compatibilidade).
- Origem do cliente: selecao unica na UI atual + detalhe opcional por item (dominio continua aceitando array por compatibilidade).
- Motivo de fora da vez: aparece somente se atendimento iniciou como `queue-jump`.
- Flags: atendimento de vitrine, presente, cliente recorrente.
- `serviceId` unico por atendimento para auditoria.

### 1.3 Paineis administrativos e analiticos

- `Consultor`: meta mensal, progresso, simulador.
- `Ranking`: ranking mensal e diario.
- `Dados`: leitura bruta operacional (produto, motivo, origem, horario, tempos).
- `Inteligencia`: diagnostico automatico com score e recomendacoes.
- `Relatorios`: filtros avancados + exportacao CSV/PDF.
- `Campanhas`: regras comerciais com aplicacao automatica no fechamento.
- `Multi-loja`: operacao por loja + comparativo consolidado.
- `Configuracoes`: controle de regras do modal, opcoes e modo teste.

### 1.4 Multi-loja

- Cada loja possui estado operacional proprio (fila, atendimento, historico, roster etc).
- Troca de loja por dropdown no header (`data-action="set-active-store"`).
- Troca de loja preserva dados da loja anterior e carrega snapshot da loja selecionada.
- Painel consolidado compara lojas por fila, atendimento, conversao, vendas, ticket, score.
- CRUD basico de lojas (admin): adicionar, editar, arquivar.

### 1.5 Perfis e acesso

- Perfil de teste por dropdown no header (`admin`, `manager`, `consultant`).
- Sem login real nesta fase (autenticacao adiada).
- Workspaces permitidos por perfil:
- `admin`: todos.
- `manager`: todos exceto `configuracoes`.
- `consultant`: `operacao`, `consultor`, `dados`.

### 1.6 Modo teste

- `settings.testModeEnabled`.
- `settings.autoFillFinishModal`.
- Preenche modal de forma automatica para acelerar testes.

## 2. Arquitetura atual (MVP)

- UI agora renderizada por paginas, layouts e componentes Vue em `web/app/`.
- Estado centralizado em `core/domain/app-store.ts`.
- Persistencia em `localStorage` via `web/app/utils/queue-storage.ts`.
- Exportacao de relatorios em `web/app/utils/report-export.ts`.
- Regras analiticas em `core/utils/admin-metrics.ts`.
- Regras de campanhas em `core/utils/campaigns.ts`.
- Filtros de relatorio em `core/utils/reports.ts`.

## 3. Contrato de estado (store)

## 3.1 Estado global

```ts
type AppState = {
  isReady: boolean
  configSchemaVersion: number
  brandName: string
  pageTitle: string

  profiles: Profile[]
  activeProfileId: string

  stores: StoreDescriptor[]
  activeStoreId: string
  storeSnapshots: Record<string, StoreScopedState>

  activeWorkspace: WorkspaceId

  operationTemplates: OperationTemplate[]
  selectedOperationTemplateId: string

  reportFilters: ReportFilters
  campaigns: CampaignRule[]

  visitReasonOptions: OptionItem[]
  customerSourceOptions: OptionItem[]
  productCatalog: ProductItem[]

  modalConfig: ModalConfig
  settings: AppSettings

  finishModalPersonId: string | null
  finishModalDraft: FinishDraft | null

  // Espelho da loja ativa (carregado de storeSnapshots[activeStoreId])
  selectedConsultantId: string | null
  consultantSimulationAdditionalSales: number
  waitingList: ConsultantQueueItem[]
  activeServices: ActiveServiceItem[]
  roster: ConsultantProfile[]
  consultantActivitySessions: ConsultantSession[]
  consultantCurrentStatus: Record<string, ConsultantStatus>
  pausedEmployees: PausedEmployee[]
  serviceHistory: ServiceHistoryEntry[]
}
```

## 3.2 Estado por loja (`StoreScopedState`)

```ts
type StoreScopedState = {
  selectedConsultantId: string | null
  consultantSimulationAdditionalSales: number
  waitingList: ConsultantQueueItem[]
  activeServices: ActiveServiceItem[]
  roster: ConsultantProfile[]
  consultantActivitySessions: ConsultantSession[]
  consultantCurrentStatus: Record<string, ConsultantStatus>
  pausedEmployees: PausedEmployee[]
  serviceHistory: ServiceHistoryEntry[]
}
```

## 3.3 Entidades principais

- `StoreDescriptor`: `id`, `name`, `code`, `city`.
- `ConsultantProfile`: `id`, `name`, `role`, `initials`, `color`, `monthlyGoal`, `commissionRate`.
- `ActiveServiceItem`: dados de atendimento ativo, incluindo `serviceId`, `serviceStartedAt`, `queueWaitMs`, `startMode`, `skippedPeople`.
- `ServiceHistoryEntry`: auditoria final do atendimento, incluindo:
- dados de tempo (`durationMs`, `queueWaitMs`),
- desfecho e valor,
- cliente e contexto,
- `storeId` / `storeName`,
- campanhas aplicadas (`campaignMatches`, `campaignBonusTotal`).

## 4. Fluxo de dados (runtime)

## 4.1 Bootstrap

1. `createAppStore()` cria estado vazio.
2. `loadQueueState()` tenta carregar `localStorage`.
3. `store.hydrate(initialState)` normaliza schema e ativa loja/perfil/workspace.
4. `store.subscribe(renderApp)` renderiza UI a cada mutacao.
5. `store.subscribe(saveQueueState)` persiste cada mutacao.
6. `setInterval(1000)` atualiza telas com timers ao vivo (`operacao`, `dados`, `inteligencia`, `multiloja`).

## 4.2 Multi-loja (troca de contexto)

No `setActiveStore(storeId)`:

1. Snapshot da loja atual e salvo em `storeSnapshots[currentStoreId]`.
2. Snapshot da nova loja e carregado para o espelho de estado ativo.
3. `finishModal` e fechado para evitar fechar atendimento na loja errada.
4. `updateState` sincroniza novamente `storeSnapshots[activeStoreId]`.

Resultado: cada loja mantem fila/historico independentes.

## 4.3 Encerramento de atendimento

No `finishService(personId, closureData)`:

1. Busca atendimento ativo.
2. Monta `historyEntry` com tempos, cliente, desfecho, origem, motivo, etc.
3. Injeta metadados de loja (`storeId`, `storeName`).
4. Aplica campanhas (`applyCampaignsToHistoryEntry`).
5. Remove consultor de `activeServices`.
6. Reinsere consultor no fim de `waitingList`.
7. Acrescenta registro em `serviceHistory`.
8. Atualiza status para `queue`.

## 5. Fluxo de UX/UI por workspace

## 5.1 Operacao

- Coluna `Lista da vez`:
- cards da fila com posicao.
- primeiro card marcado `Na vez`.
- demais cards com botao `Atender fora da vez`.
- Coluna `Em atendimento`:
- cards com timer ao vivo.
- botao `Encerrar atendimento`.
- Barra inferior:
- entrada na fila,
- pausa,
- retomar pausa.

## 5.2 Consultor

- Selecao do consultor.
- Meta, progresso, comissao estimada.
- Simulador de venda adicional para projeção.

## 5.3 Ranking

- Tabela mensal e tabela diaria.
- Ordenacao por valor vendido (desempate por conversoes/taxa).

## 5.4 Dados

- Chips/tags de agregacao:
- produtos vendidos/procurados,
- motivos,
- origem,
- profissoes,
- desfecho.
- Tabela de horarios com mais venda.
- Bloco de tempos operacionais.

## 5.5 Inteligencia

- Score operacional.
- Diagnosticos por severidade.
- Hipotese e acao recomendada por diagnostico.

## 5.6 Relatorios

- Filtros:
- data inicial/final,
- consultor,
- desfecho,
- origem,
- motivo,
- tipo de atendimento (`queue`/`queue-jump`),
- cliente recorrente,
- valor minimo/maximo,
- busca livre.
- KPIs filtrados:
- atendimentos,
- conversao,
- valor vendido,
- ticket medio,
- tempo medio,
- espera media,
- taxa fora da vez,
- bonus de campanhas.
- Exportacoes:
- CSV completo,
- PDF via print layout.

## 5.7 Campanhas

- CRUD de regras comerciais (admin).
- Campos de regra:
- nome, descricao, periodo,
- desfecho alvo,
- cliente recorrente (all/yes/no),
- venda minima,
- duracao maxima,
- bonus fixo/percentual,
- somente fora da vez,
- origem alvo (multi),
- motivo alvo (multi),
- ativa/inativa.
- Exibe aplicacoes historicas e bonus acumulado.

## 5.8 Multi-loja

- Visao consolidada entre lojas:
- fila, em atendimento, pausados,
- atendimentos,
- conversao,
- vendas e ticket,
- espera media,
- taxa fora da vez,
- score operacional.
- Botao de troca rapida para "abrir loja".
- CRUD de lojas (admin).

## 5.9 Configuracoes

- Template de operacao.
- Regras de tempo e limite.
- Flags de modo teste.
- Textos e obrigatoriedades do modal.
- CRUD de opcoes de motivo/origem.
- CRUD de consultores.
- CRUD de catalogo mock de produtos.

## 6. Validacoes e regras de negocio

## 6.1 Fila e atendimento

- Nao entra na fila se ja estiver em fila/atendimento/pausa.
- Nao inicia atendimento se fila vazia.
- Respeita `maxConcurrentServices`.
- `queue-jump` quando inicia atendimento de posicao > 1.
- Ao pausar: remove da fila e muda status para `paused`.
- Ao retomar: reinsere o consultor no fim da fila e muda status para `queue`.

## 6.2 Modal de encerramento

- Exige desfecho.
- Exige produto visto quando `modalConfig.requireProduct`.
- Em `compra`/`reserva` exige produto fechado; o valor e calculado automaticamente a partir dos produtos fechados.
- Exige nome + telefone quando configurado.
- Exige origem quando configurado.
- Exige motivo de fora da vez quando `startMode === queue-jump`.

## 6.3 Campanhas

Uma campanha so aplica quando:

- `isActive === true`
- periodo valido (`startsAt` / `endsAt`)
- desfecho compativel
- venda minima atingida
- duracao maxima respeitada
- origem/motivo (se definidos) contem intersecao
- regra de fora da vez respeitada
- filtro de cliente recorrente respeitado

Bonus calculado:

- `bonusFixed + (saleAmount * bonusRate)`
- acumulado em `campaignBonusTotal` no historico.

## 7. Metricas e formulas

## 7.1 Tempo

- `durationMs = finishedAt - serviceStartedAt`
- `queueWaitMs = serviceStartedAt - queueJoinedAt`
- agregacao por status:
- `available`, `queue`, `service`, `paused`
- media de espera:
- media de `queueWaitMs` no historico filtrado.

## 7.2 Conversao e venda

- Conversao = `(compras + reservas) / atendimentos`.
- Ticket medio = `soma saleAmount(convertidos) / qtd convertidos`.
- Fora da vez % = `atendimentos queue-jump / total`.

## 7.3 Score operacional (inteligencia)

- Base 100.
- Penalidades por diagnosticos `critical` e `attention`.
- Resultado final em `healthScore`.

## 8. Eventos de UI (contrato de actions)

Lista central no `main.js`:

- Click actions:
- `set-workspace`
- `set-active-store`
- `select-consultant`
- `add-to-queue`
- `pause-employee`
- `resume-employee`
- `start-service`
- `open-finish-modal`
- `close-finish-modal`
- `remove-option`
- `remove-product`
- `apply-operation-template`
- `archive-consultant`
- `reset-report-filters`
- `export-report-csv`
- `export-report-pdf`
- `remove-campaign`
- `archive-store`

- Change actions:
- `set-active-profile`
- `set-active-store`
- `set-report-filter`
- `set-simulation-value`
- `set-setting`
- `set-modal-config`
- `update-product`

- Submit actions:
- `add-option`
- `update-option`
- `add-product`
- `add-consultant`
- `update-consultant`
- `add-store`
- `update-store`
- `add-campaign`
- `update-campaign`
- `finish-service-form`

## 9. Persistencia local (localStorage)

Chave: `nexo-queue-state`.

Campos persistidos:

- metadados de app e perfil
- stores, activeStoreId, storeSnapshots
- workspace
- filtros de relatorio
- campanhas
- estado operacional da loja ativa (espelho)
- configuracoes, catalogo, opcoes
- historico e sessoes

Compatibilidade:

- normalizacao para entradas antigas
- `configSchemaVersion` atual: `4`

## 10. Mapa de permissao

- `admin`:
- acesso total
- gerencia configuracoes, consultores, campanhas e lojas
- `manager`:
- acesso a operacao, leitura analitica, relatorios, campanhas, multiloja
- sem CRUD administrativo sensivel
- `consultant`:
- operacao + visao consultor + dados basicos

## 11. Blueprint de migracao para Nuxt

## 11.1 Estrutura sugerida (Nuxt 3)

```txt
app/
  pages/
    index.vue                    # shell + workspace tabs
  components/
    header/AppHeader.vue
    workspace/WorkspaceNav.vue
    operation/QueueColumn.vue
    operation/ConsultantStrip.vue
    operation/FinishModal.vue
    admin/ConsultorPanel.vue
    admin/RankingPanel.vue
    admin/DadosPanel.vue
    admin/InteligenciaPanel.vue
    admin/RelatoriosPanel.vue
    admin/CampanhasPanel.vue
    admin/MultiLojaPanel.vue
    admin/ConfiguracoesPanel.vue
  stores/
    app.ts                       # estado central (Pinia)
  composables/
    usePermissions.ts
    useReports.ts
    useCampaigns.ts
    useMetrics.ts
  services/
    queueStorage.ts              # local fallback
    reportExport.ts
    api/                         # futura integracao backend
```

## 11.2 Estado no Pinia (`stores/app.ts`)

- Migrar o shape atual de `AppState` sem reduzir campos.
- Manter `storeSnapshots` + espelho de loja ativa para nao quebrar regra.
- Manter nomes de campos para facilitar diffs e testes.

## 11.3 Fluxo de eventos em Nuxt

- Cada `data-action` atual vira handler explicito em Vue:
- `@click`, `@change`, `@submit.prevent`.
- Mutacoes devem continuar centralizadas no store.
- Regra: componente nao conhece regra de negocio, apenas dispara intents.

## 11.4 Persistencia na migracao

Fase 1 Nuxt (sem backend):

- replicar `loadQueueState`/`saveQueueState` no client.
- manter schema version e normalizacao.

Fase 2 Nuxt (com backend):

- manter contrato dos payloads do store.
- trocar somente a origem (API) em `services/api`.
- manter fallback local para modo offline/teste.

## 11.5 Exportacao de relatorio

- CSV: manter serializacao e escaping.
- PDF: manter estrategia print-first enquanto nao houver lib dedicada.

## 11.6 SSR e hidratacao

- Store deve inicializar no client para `localStorage`.
- Em SSR, retornar estado base e hidratar no `onMounted`.
- Evitar acessar `window` fora do client.

## 12. Checklist de aceite para migracao

- Fluxo fila -> atendimento -> modal -> encerramento -> retorno fila.
- Regras de validacao do modal idempotentes.
- Campanhas aplicando com mesmos criterios.
- Relatorios com os mesmos filtros e mesmos totais.
- Export CSV/PDF contendo os mesmos campos.
- Multi-loja preservando snapshots ao trocar de loja.
- Permissoes por perfil restringindo workspaces corretamente.
- Timers ao vivo atualizando nos workspaces esperados.
- Persistencia local recuperando estado apos reload.

## 13. Itens ainda fora do escopo (nao implementados)

- Backend de persistencia (estado ainda localStorage).
- Login/autenticacao real.
- API real de produtos (catalogo ainda mock).

## 14. Referencias de codigo

- Estado e regras: `core/domain/app-store.ts`
- Store Nuxt/Pinia: `web/app/stores/dashboard.ts`
- Persistencia: `web/app/utils/queue-storage.ts`
- Regras de metricas: `core/utils/admin-metrics.ts`
- Regras de campanhas: `core/utils/campaigns.ts`
- Filtros de relatorio: `core/utils/reports.ts`
- Exportacao: `web/app/utils/report-export.ts`
- Tela consolidada multi-loja: `web/app/components/multistore/MultiStoreWorkspace.vue`

---

Se a migracao para Nuxt seguir este documento sem cortar contratos de dados, o comportamento funcional deve ser reproduzido com alta fidelidade.
