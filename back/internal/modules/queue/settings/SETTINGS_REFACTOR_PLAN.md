# SETTINGS REFACTOR PLAN

## Objetivo

Este documento descreve a refatoracao estrutural do sistema de configuracoes
para reduzir fragilidade, diminuir o acoplamento entre secoes e impedir que uma
falha em `settings` derrube o bootstrap do painel.

## Problema atual

Hoje o modulo `settings` tem quatro fragilidades principais:

1. `tenant_operation_settings` concentra campos demais de dominios diferentes.
2. `PATCH /operation` e `PATCH /modal` ainda dependem de carregar o bundle inteiro.
3. O boot do frontend depende de `/v1/settings` junto com outros dados criticos.
4. A leitura/escrita da tabela principal ainda depende de codigo manual demais.

Consequencias praticas:

- mudar um campo pequeno tem blast radius alto
- regressao em `settings` pode quebrar login ou bootstrap do painel
- aplicar template pode gerar estado parcial se uma chamada falhar no meio
- manutencao fica cara conforme o painel cresce

## Arquitetura alvo

### 1. `tenant_operation_core_settings`

Responsavel por configuracoes operacionais estaveis e tipadas.

Campos alvo:

- `tenant_id`
- `selected_operation_template_id`
- `max_concurrent_services`
- `max_concurrent_services_per_consultant`
- `timing_fast_close_minutes`
- `timing_long_service_minutes`
- `timing_low_sale_amount`
- `service_cancel_window_seconds`
- `test_mode_enabled`
- `auto_fill_finish_modal`
- `updated_at`
- `updated_by`

Observacao:

- `updated_by` pode ser `uuid null` na primeira fase se o contexto do usuario
  ainda nao estiver passando isso de forma padronizada.

### 2. `tenant_finish_modal_settings`

Responsavel apenas pela configuracao do modal de encerramento.

Campos alvo:

- `tenant_id`
- `finish_flow_mode`
- `schema_version`
- `config jsonb`
- `updated_at`
- `updated_by`

O `jsonb` deve guardar:

- labels
- placeholders
- show/hide
- required
- modos de selecao
- modos de detalhe
- futuras flags especificas do modal

Regra importante:

- `config jsonb` deve manter estrutura previsivel e versionada
- nao usar `jsonb` para tudo; usar apenas para a parte de UI/modal que muda com
  mais frequencia

### 3. `tenant_alert_settings`

Responsavel pelos thresholds e alertas operacionais.

Campos alvo:

- `tenant_id`
- `alert_min_conversion_rate`
- `alert_max_queue_jump_rate`
- `alert_min_pa_score`
- `alert_min_ticket_average`
- `updated_at`
- `updated_by`

Decisao:

- alertas ficam em tabela propria nesta refatoracao
- nao vao permanecer misturados no core para evitar novo crescimento da tabela
  central

### 4. `tenant_setting_options`

Permanece como tabela relacional normalizada.

Responsabilidade:

- motivos de visita
- origens
- pausas
- cancelamento
- parada
- fora da vez
- perdas
- profissoes

### 5. `tenant_catalog_products`

Permanece separado, mas continua sendo catalogo administrativo/manual.

Regra:

- autocomplete operacional de produtos deve continuar migrando para o modulo
  `catalog`
- esta tabela nao deve voltar a ser misturada no core de settings

## Contrato externo durante a migracao

### Mantido no inicio

- `GET /v1/settings`
- `PATCH /v1/settings/operation`
- `PATCH /v1/settings/modal`
- rotas de options
- rotas de products

### Evolucao planejada

- `GET /v1/settings` continua existindo como bundle agregado para o frontend
- `PATCH /operation` passa a persistir apenas no core
- `PATCH /modal` passa a persistir apenas no modal
- alertas podem ganhar rota propria depois da separacao fisica, mas a UX pode
  continuar igual na primeira entrega
- aplicar template completo deve virar uma mutacao backend unica e transacional

## Estrategia de migracao

### Fase 0 - Preparacao

- documentar arquitetura alvo
- congelar comportamento atual com testes e checklist manual
- mapear consumidores de `runtime.state.settings` e `runtime.state.modalConfig`

### Fase 1 - Blindagem do boot

- alterar `web/app/utils/runtime-remote.ts`
- substituir `Promise.all` por carregamento degradavel
- impedir que falha em `/v1/settings` derrube sessao ou painel
- usar defaults locais quando settings falhar

Resultado esperado:

- settings pode falhar sem quebrar login

### Fase 1 executada em 2026-04-29

Implementacao entregue nesta fase:

- `web/app/utils/runtime-remote.ts`
  - troca do bootstrap de `settings` para carregamento degradavel
  - separacao explicita entre dependencia opcional (`settings`) e dependencias
    duras da tela (`consultants`, `operationsSnapshot`)
  - fallback seguro para defaults de runtime usando `createEmptyState()`
- `web/app/stores/auth.ts`
  - `settings` deixou de ser tratado como falha de sessao
  - a store agora mantem `runtimeSettingsStatus`,
    `runtimeSettingsNotice` e `runtimeSettingsLastError`
- `web/app/layouts/dashboard.vue`
  - banner persistente de modo degradado quando `settings` falhar
- `web/app/components/settings/SettingsWorkspace.vue`
  - aviso contextual adicional dentro da tela de configuracoes
- `back/internal/modules/settings/http.go`
  - gatilho de falha controlada apenas fora de `production` para smoke local
  - modos iniciais: `500` e `slow-500`
- refreshes realtime e hidratacoes auxiliares passaram a reaproveitar o mesmo
  estado de degradacao para nao ficar cada fluxo com um comportamento diferente
- guia manual criado em `back/internal/modules/settings/PHASE1_SMOKE_GUIDE.md`

Erros estruturais encontrados nesta fase:

1. `Promise.all` no bootstrap acoplava demais as dependencias.

- Antes: um erro isolado de `/v1/settings` cancelava o lote inteiro.
- Efeito visivel: login parecia quebrado, sessao era limpa e o painel nao subia.

2. O caminho de autenticacao e o caminho de configuracao estavam misturados.

- `fetchContext` -> `syncRuntimeAccess` -> `hydrateRuntimeStoreContext`
  propagava erro de settings como se fosse erro de sessao.
- Efeito visivel: trocar um campo de config ou ter 500 em settings derrubava o
  bootstrap inteiro.

3. O runtime nascia de `mockQueueState`.

- Sem um fallback neutro, o primeiro login com erro em settings poderia manter
  labels, catalogos e configuracoes demo no tenant real.
- Isso era especialmente perigoso para `productCatalog` e `modalConfig`.

4. O realtime escondia o problema.

- Os refreshes de settings estavam usando `.catch(() => null)`.
- Efeito visivel: o operador nao via o motivo da degradacao e o time perdia
  rastreabilidade na depuracao.

Pendencias ainda abertas para encerrar a Fase 1:

- validar manualmente os cenarios:
  - timeout em `/v1/settings`
  - `500 internal_error` no backend
  - erro de schema/coluna faltando
  - API de settings indisponivel com login ainda funcional
- decidir se o aviso degradado deve ganhar telemetria server-side alem do
  `console.warn` estruturado do frontend

### Fase 2 - Separacao de camadas no backend

- separar repositorios e DTOs por dominio
- parar de tratar bundle inteiro como unidade de escrita
- manter bundle apenas como agregacao de leitura

Resultado esperado:

- o backend passa a ter fronteiras claras entre core, modal e alertas

### Fase 2 executada em 2026-04-29

Implementacao entregue nesta fase:

- `back/internal/modules/settings/sections.go`
  - consolidacao dos DTOs internos:
    - `OperationCoreSettings`
    - `AlertSettings`
    - `OperationSectionRecord`
    - `ModalSectionRecord`
  - helpers de split/compose entre bundle publico e secoes internas
  - defaults e normalizacao por secao
- `back/internal/modules/settings/service.go`
  - `SaveOperationSection` agora carrega e persiste apenas `OperationSectionRecord`
  - `SaveModalSection` agora carrega e persiste apenas `ModalSectionRecord`
  - `SaveOptionSection` e `SaveProductSection` passaram a carregar apenas o
    grupo/catalogo necessario
  - `SaveOptionItem` e `SaveProductItem` passaram a usar mutacao granular por
    item no caminho normal
  - a materializacao de defaults de `options` e `productCatalog` ficou
    explicitada em helpers por secao, sem depender de bundle completo
- `back/internal/modules/settings/store_postgres_sections.go`
  - queries menores de leitura e upsert para `operation` e `modal` sobre a
    tabela legacy atual
  - `GetOptionGroup` e `GetProductCatalog` oficializados como leituras
    dedicadas do repositorio
- `back/internal/modules/settings/store_postgres.go`
  - `GetByTenant` virou agregador de leitura, montando o bundle a partir das
    secoes menores
  - o bundle continua existindo apenas como contrato externo de leitura
- `back/internal/modules/settings/store_postgres_test.go`
  - testes de alinhamento para as novas queries de `operation` e `modal`
- `back/internal/modules/settings/service_test.go`
  - testes cobrindo clamp de limite por consultor, defaults de modal por
    template, materializacao de defaults em options e mutacao granular de
    products

Erros estruturais encontrados nesta fase:

1. `PATCH /operation` e `PATCH /modal` ainda faziam merge via bundle completo.

- Fluxo antigo: `loadWritableBundle` -> `GetByTenant` -> `UpsertConfig`
- Efeito: mudar um campo simples ainda regravava operation + modal juntos,
  ampliando o blast radius na tabela larga

2. O service ignorava mutacoes granulares que o repositorio ja oferecia.

- `UpsertOption`, `DeleteOption`, `UpsertProduct` e `DeleteProduct` existiam,
  mas `POST/PATCH/DELETE` de item ainda reconstruia a colecao inteira
- Efeito: custo de manutencao alto e risco de sobrescrita desnecessaria

3. Os defaults de listas e catalogo estavam escondidos dentro do bundle.

- Sem helpers por secao, qualquer tentativa de granularizar `POST /options` ou
  `POST /products` corria risco de apagar os defaults esperados no front

4. O bundle estava acumulando dois papeis diferentes.

- contrato externo de leitura para o Nuxt
- unidade interna obrigatoria de merge e persistencia

Decisao aplicada:

- manter o bundle apenas como contrato de leitura agregada
- mover a logica interna de merge/default/validacao para DTOs e helpers de
  secao

Pendencias que continuam para a Fase 3:

- a separacao fisica de storage ainda nao aconteceu
- `tenant_operation_settings` segue como tabela legacy unica nesta fase
- `updated_by`, `schema_version` e tabelas novas entram apenas na migracao de
  banco da proxima fase

### Fase 3 - Criacao das novas tabelas

- criar `tenant_operation_core_settings`
- criar `tenant_finish_modal_settings`
- criar `tenant_alert_settings`
- popular via backfill a partir de `tenant_operation_settings`

Resultado esperado:

- novas tabelas prontas para dual-read e dual-write

### Fase 3 executada em 2026-04-29

Migrations criadas:

- `0040_tenant_operation_core_settings.sql`
  - tabela com 10 campos operacionais estaveis + updated_by + updated_at
  - backfill via SELECT de tenant_operation_settings com coalesce em colunas nullable
  - rollback: drop table if exists tenant_operation_core_settings
- `0041_tenant_alert_settings.sql`
  - tabela com 4 thresholds de alerta + updated_by + updated_at
  - backfill direto de tenant_operation_settings
  - rollback: drop table if exists tenant_alert_settings
- `0042_tenant_finish_modal_settings.sql`
  - tabela com finish_flow_mode (coluna tipada), schema_version e config jsonb
  - config jsonb schema_version 1: todos os campos do modal exceto finish_flow_mode
  - backfill via jsonb_build_object com coalesce em colunas nullable (purchase_code_*,
    cancel_reason_*, stop_reason_*, show_purchase_code_field, show_cancel_reason_field,
    show_stop_reason_field, cancel_reason_input_mode, stop_reason_input_mode,
    require_purchase_code_field, require_cancel_reason_field, require_stop_reason_field)
  - rollback: drop table if exists tenant_finish_modal_settings

Regras validas ate a Fase 4:

- as tres tabelas novas sao criadas mas ainda nao sao lidas nem escritas pelo service
- tenant_operation_settings segue como unica fonte de leitura e escrita
- as tabelas novas existem apenas para preparar o ambiente de dual-read
- nenhuma alteracao de Go code foi necessaria nesta fase
- go test ./... verde

### Fase 4 - Migracao do modal

- ler modal primeiro da tabela nova
- se nao existir, fazer fallback temporario para legado
- gravar em ambos durante janela de transicao
- validar round-trip de leitura e escrita

Resultado esperado:

- mudancas no modal deixam de depender da tabela larga como caminho primario

### Fase 4 executada em 2026-04-30

Migrations criadas:

- `0043_fix_modal_config_jsonb_keys.sql`
  - converte o config jsonb de snake_case (backfill 0042) para camelCase (json.Marshal do Go)
  - adiciona a chave finishFlowMode ao jsonb a partir da coluna tipada
  - idempotente: WHERE NOT (config ? 'productSeenLabel') protege contra dupla execucao
  - rollback: nao ha rollback simples (forward-only); restaurar backup pre-0042 se necessario

Mudancas no backend:

- `store_postgres_sections.go`
  - `GetModalSection` virou agregador: tenta `getModalSectionFromNew` primeiro, cai em
    `getModalSectionFromLegacy` se nao encontrar ou se houver erro
  - `getModalSectionFromNew`: le de `tenant_finish_modal_settings` com JOIN para
    `selected_operation_template_id` (core > legacy > default); usa json.Unmarshal no config
    jsonb; finish_flow_mode da coluna tipada sempre prevalece
  - `UpsertModalSection`: faz dual-write; escreve na tabela nova primeiro (erro nao e fatal),
    depois na legacy (fonte autoritativa em Fase 4)
  - `upsertModalSectionToNew`: json.Marshal(ModalConfig) -> jsonb; INSERT/ON CONFLICT UPDATE
- `store_postgres_test.go`
  - `TestModalConfigJSONRoundTrip`: garante que marshal -> unmarshal preserva todos os campos
  - `TestModalConfigJSONMissingFieldsAreZeroValues`: jsonb parcial produce zero values seguros
  - `TestModalConfigJSONRoundTripAfterNormalization`: apos normalizacao, nenhum campo fica zero

Regras validas ate a Fase 5:

- legacy e a fonte autoritativa para leitura e escrita do modal
- a tabela nova recebe escrita em toda chamada de PATCH /modal
- leitura da tabela nova e preferencial; se falhar, legacy e a reserva
- next: Fase 5 — mover alertas para tenant_alert_settings com o mesmo padrao dual-read/write

### Fase 5 - Migracao de alertas

- mover thresholds para `tenant_alert_settings`
- trocar leitura/escrita para a tabela nova
- manter fallback temporario durante rollout

Resultado esperado:

- thresholds nao concorrem mais com modal e core no mesmo row

### Fase 5 executada em 2026-04-30

Mudancas no backend:

- `store_postgres_sections.go`
  - `GetOperationSection` virou agregador: carrega `getOperationSectionFromLegacy` e
    sobrepos `AlertSettings` com `getAlertSettingsFromNew` se encontrado (silencioso em erro)
  - `getAlertSettingsFromNew`: le `alert_min_conversion_rate`, `alert_max_queue_jump_rate`,
    `alert_min_pa_score` e `alert_min_ticket_average` de `tenant_alert_settings` por tenantId
  - `getOperationSectionFromLegacy`: corpo antigo do `GetOperationSection`, mantido como fallback
  - `UpsertOperationSection`: dual-write; escreve alertas na tabela nova (erro nao fatal),
    depois grava tudo na legacy (fonte autoritativa em Fase 5)
  - `upsertAlertSettingsToNew`: INSERT ON CONFLICT UPDATE para os 4 campos de alerta;
    nao toca em `updated_by` por enquanto (sem contexto de usuario disponivel)

Regras validas ate a Fase 6:

- legacy e a fonte autoritativa para leitura e escrita de toda a operacao
- `tenant_alert_settings` recebe escrita em toda chamada de PATCH /operation
- leitura da tabela nova e preferencial para alertas; se falhar, legacy e a reserva
- UX nao mudou: alertas continuam na mesma aba de configuracoes
- go test ./... verde

Next: Fase 6 — dual-read/write do core operacional em tenant_operation_core_settings.

### Fase 6 - Migracao do core

- mover limites, tempos e configuracoes operacionais estaveis
- reimplementar `PATCH /operation` sem recarregar bundle inteiro

Resultado esperado:

- alterar regra operacional simples nao depende de tabela larga nem de bundle
  completo

### Fase 6 executada em 2026-04-30

Mudancas no backend:

- `store_postgres_sections.go`
  - `GetOperationSection` atualizado: apos sobreposicao de alertas (Fase 5), agora
    sobrepos `CoreSettings` e `SelectedOperationTemplateID` com `getCoreSettingsFromNew`
  - `getCoreSettingsFromNew`: le 10 campos de `tenant_operation_core_settings`
    (tenant_id, selected_operation_template_id, 8 campos de OperationCoreSettings)
    via Scan manual por posicao; fallback silencioso em ErrNoRows ou erro
  - `upsertCoreSettingsToNew`: INSERT ON CONFLICT UPDATE para os 10 campos;
    nao toca em `updated_by` (sem contexto de usuario disponivel ainda)
  - `UpsertOperationSection`: dual-write completo agora —
    1. `upsertAlertSettingsToNew` (Fase 5, nao fatal)
    2. `upsertCoreSettingsToNew` (Fase 6, nao fatal)
    3. legacy (fonte autoritativa)

Regras validas ate a Fase 9:

- legacy e a fonte autoritativa para leitura e escrita de toda a operacao
- as tres tabelas novas (core, modal, alertas) recebem escrita em todo PATCH /operation ou /modal
- leitura preferencial das tabelas novas com fallback para legacy em caso de erro
- go test ./... verde

Next: Fase 7 (granularidade de escrita de options/catalog) foi concluida; seguir para Fase 8 (scan por nome nos DTOs migrados).

### Fase 7 - Granularidade de escrita

- frontend deixou de usar replace amplo para create/update/delete de options
- backend ganhou endpoint transacional para aplicar template inteiro

Resultado esperado:

- `POST /v1/settings/templates/{templateId}/apply` aplica core, modal, motivos de visita e origens em uma unica transacao
- create/update/delete de options usa `POST`, `PATCH` e `DELETE`; `PUT` fica reservado para reorder/importacao
- catalogo manual segue o mesmo principio granular para create/update/delete
- aplicar template nao deixa estado parcialmente salvo se uma etapa falhar

### Fase 8 - Leitura segura por nome e reducao de codigo manual

- substituir mapeamentos manuais extensos por DTOs/leitura por nome nas secoes migradas
- reduzir `scan` manual nas leituras de core, alertas e modal
- manter tratamento consistente para `NULL`, defaults e compatibilidade de versao

Resultado esperado:

- adicionar campo novo em modal/alerta/core nao exige editar varios blocos manuais independentes

### Fase 9 - Corte do legado

- remover dual-write
- remover fallback legado
- remover codigo morto

Resultado esperado:

- `tenant_operation_settings` deixa de ser dependencia critica

## Regras de implementacao

### Regras de banco

- toda nova tabela deve ter `updated_at`
- idealmente adicionar `updated_by`
- toda migration deve ter plano de rollback documentado
- campos com comportamento opcional devem nascer com default seguro

### Regras de backend

- preferir DTOs pequenos por secao
- reduzir `Scan` manual por posicao
- usar leitura por nome quando possivel
- nao carregar bundle inteiro para mudar um unico campo

### Regras de frontend

- login nao pode depender do sucesso de settings
- bootstrap deve tolerar falha de `/v1/settings`
- a UI deve exibir aviso quando entrar em modo degradado
- alteracao local so deve persistir a secao afetada

## Riscos conhecidos

### 1. Divergencia entre legado e novo durante dual-write

Mitigacao:

- validar leitura preferencial nova apenas depois de backfill e smoke
- instrumentar log de divergencia

### 2. Rollout parcial com frontend antigo e backend novo

Mitigacao:

- manter contratos externos no inicio
- migrar primeiro implementacao interna

### 3. Campo opcional novo quebrar parsing

Mitigacao:

- usar defaults fortes
- reduzir leitura por posicao
- testar round-trip por secao

### 4. Aplicacao de template ficar inconsistente

Mitigacao:

- endpoint transacional unico no backend

## Criterios de aceite finais

O trabalho so sera considerado concluido quando:

- falha em `/v1/settings` nao derrubar login nem bootstrap
- modal estiver persistido fora da tabela larga
- alertas estiverem persistidos fora da tabela larga
- core operacional estiver persistido fora da tabela larga
- aplicar template for transacional
- options nao dependerem de replace amplo para CRUD simples
- a manutencao de campo novo em settings nao exigir sincronizar varios blocos
  manuais independentes
