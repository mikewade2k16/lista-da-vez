# AGENT

## Escopo

Estas instrucoes valem para `back/internal/modules/settings`.

## Responsabilidade do modulo

O modulo `settings` cuida do pacote configuravel da operacao por tenant.

A configuracao deixou de ser por loja: agora existe uma unica fonte da verdade
por tenant que vale para todas as lojas dele. Isso evita que um admin com a
loja errada selecionada no header acabe gravando opcoes em uma so loja.

Hoje ele deve responder por:

- bundle de settings consumido pelo Nuxt
- modal config
- selecao do modo de fechamento do modal (`legacy` vs `erp-reconciliation`)
- catalogos de motivos de visita, origens, pausas, fora da vez, perdas e profissoes
- catalogo de produtos
- selecao de template operacional
- ordenacao explicita dos catalogos por `sort_order`
- publicacao de invalidacao realtime quando a configuracao do tenant muda

Ele nao deve cuidar de:

- fila e atendimento
- auth
- campanhas
- relatorios server-side
- busca operacional remota de produtos

## Contrato atual

- `GET /v1/settings`
- `PUT /v1/settings`
- `PATCH /v1/settings/operation`
- `PATCH /v1/settings/modal`
- `POST /v1/settings/templates/{templateId}/apply`
- `POST /v1/settings/options/{group}`
- `PATCH /v1/settings/options/{group}/{itemId}`
- `DELETE /v1/settings/options/{group}/{itemId}`
- `PUT /v1/settings/options/{group}`
- `POST /v1/settings/products`
- `PATCH /v1/settings/products/{itemId}`
- `DELETE /v1/settings/products/{itemId}`
- `PUT /v1/settings/products`

Os endpoints continuam aceitando `storeId` no payload e na query string para
nao quebrar clientes legados, mas o backend ignora esse valor. Clientes devem
enviar `tenantId` do contexto ativo quando o principal for global. Nunca usar
`storeId` para escolher escopo de gravacao em settings.

## Regras de escopo

- leitura: qualquer usuario com acesso ao tenant
- escrita: `owner` e `platform_admin`
- escopo de leitura/gravacao: `tenantId` explicito validado contra o acesso do principal, ou `principal.TenantID` quando o usuario ja for tenant-scoped
- para principals globais como `platform_admin`, a UI deve chamar `/v1/settings?tenantId={activeTenantId}` e enviar esse mesmo query param nas escritas
- se um principal global omitir `tenantId`, o fallback so pode resolver automaticamente quando existir exatamente um tenant acessivel; zero ou multiplos tenants devem falhar por escopo ambiguo
- regressao a evitar: `platform_admin` normalmente autentica sem `tenantId` no token; o boot do painel precisa carregar `/v1/me/context`, usar `activeTenantId` e entao chamar `/v1/settings?tenantId={activeTenantId}`
- a hidratacao automatica do runtime no login (em `web/app/utils/runtime-remote.ts`: `hydrateRuntimeStoreContext`, `refreshRuntimeStoreSettings`, `fetchRemoteStoreData`) tambem precisa receber `auth.activeTenantId`; do contrario o `tenantId` derivado do `runtime.state.stores` pode estar vazio e o backend cai em `ErrTenantRequired` (HTTP 400 `validation_error` "Verifique os dados de configuracao")

## Regra de persistencia

- os catalogos e configuracoes desta fase vivem em tabelas normalizadas por tenant:
  - `tenant_operation_settings`
  - `tenant_setting_options`
  - `tenant_catalog_products`
- as tabelas legadas `store_operation_settings`, `store_setting_options` e
  `store_catalog_products` permanecem no banco como fonte de backfill ate que
  a estrategia de uniao final seja definida no deploy
- templates operacionais continuam versionados no codigo do backend
- o `GET /v1/settings` continua entregando um bundle para o Nuxt por conveniencia de leitura
- a API de escrita deve preferir endpoints por secao em vez de trafegar o bundle inteiro a cada alteracao
- em listas e catalogos, a escrita deve preferir mutacao por item em vez de substituir a colecao inteira
- o `catalogo de produtos` daqui continua sendo administrativo/manual; o autocomplete operacional de produtos deve migrar para o modulo `catalog`
- o modo `erp-reconciliation` do modal serve para captura operacional leve: nesse fluxo o `settings` so controla exibicao, obrigatoriedade e copy do `codigo da compra`; ele nao resolve compra nem ERP em tempo real
- os grupos atuais de `tenant_setting_options.kind` sao:
  - `visit_reason`
  - `customer_source`
  - `pause_reason`
  - `queue_jump_reason`
  - `loss_reason`
  - `profession`
- em `PATCH /operation` e `PATCH /modal`, a UI deve enviar apenas os campos alterados; o backend aplica merge sobre o estado atual
- campos opcionais/default nao devem ser enviados sem necessidade; ausencia deve ser tratada como "manter valor atual" em patch parcial
- endpoints `PUT` de secoes/listas ficam reservados para bulk replace intencional, importacao ou aplicacao de template
- `PUT /v1/settings/options/{group}` deve preservar a ordem recebida e gravar isso em `sort_order`
- regressao recorrente a evitar: `tenant_operation_settings` tem muitos campos. Ao adicionar config nova, nao escrever placeholders `VALUES (...)` manualmente. A query de upsert deste modulo deve continuar sendo gerada a partir de `tenantOperationSettingsPersistedColumns` + `buildTenantOperationSettingsUpsertArgs`, e o teste `store_postgres_test.go` deve seguir verde para garantir alinhamento entre colunas, placeholders e argumentos
- regressao recorrente a evitar desde a Fase 2:
  - `PATCH /operation` nao deve voltar a depender de `GetByTenant`
  - `PATCH /modal` nao deve voltar a depender de `GetByTenant`
  - `POST/PATCH/DELETE` de option nao devem reconstruir a colecao inteira quando o grupo ja existe
  - `POST/PATCH/DELETE` de product nao devem reconstruir o catalogo inteiro quando a colecao ja existe
- antes de gravar uma opcao recebida via `POST /options/{group}`, o service materializa os defaults do grupo se a tabela ainda estiver vazia para aquele tenant; isso garante que um cadastro novo nao "apaga" os defaults vistos no front
- o mesmo cuidado vale para `productCatalog`: enquanto a tabela manual estiver vazia, o service materializa o catalogo default antes da primeira mutacao unitaria
- mudanca de settings publica somente `context.updated`:
  - `resource = settings`, `action = updated`, `resourceId = {tenantId}`
  - todos os clientes do tenant revalidam o bundle apos receber esse evento
  - o canal `operation.updated` deixou de ser usado por settings; o canal de contexto ja chega a todos os atendentes do tenant

## Arquitetura alvo da refatoracao

Esta secao registra a direcao oficial para a proxima grande refatoracao de
settings. O objetivo e reduzir acoplamento, blast radius e dependencia da
tabela larga atual.

- `tenant_operation_core_settings`
  - configuracoes operacionais estaveis e tipadas
  - exemplos: limites de concorrencia, tempos, ticket minimo, janela de cancelamento, template selecionado
- `tenant_finish_modal_settings`
  - configuracao do modal de encerramento
  - deve usar `finish_flow_mode` + `schema_version` + `config jsonb`
  - `jsonb` aqui e aceitavel porque o modal e a parte mais volatil da modelagem
- `tenant_alert_settings`
  - thresholds e alertas operacionais
  - exemplos: conversao minima, taxa maxima de fora da vez, PA minima, ticket medio minimo
- `tenant_setting_options`
  - continua relacional e separado
- `tenant_catalog_products`
  - continua separado enquanto existir o catalogo administrativo/manual

Durante a migracao:

- `tenant_operation_settings` continua existindo como camada legacy temporaria
- a ordem oficial e:
  - blindar boot/login
  - separar backend por secao
  - criar novas tabelas
  - migrar modal
  - migrar alertas
  - migrar core
  - reduzir replace amplo e aplicar template de forma transacional
  - cortar legado
- leituras novas devem entrar primeiro com fallback para legado
- escritas devem usar dual-write apenas pelo tempo necessario
- nenhuma fase pode remover o caminho antigo antes de validar a nova em leitura e escrita

Regra critica de UX/plataforma:

- falha em `/v1/settings` nao pode derrubar login, bootstrap nem sessao ja valida
- o frontend deve subir em modo degradado com defaults seguros quando settings falhar
- fase 1 dessa blindagem foi entregue em `2026-04-29` no frontend:
  - bootstrap degradavel em `web/app/utils/runtime-remote.ts`
  - estado explicito de degradacao em `web/app/stores/auth.ts`
  - banner persistente no layout autenticado e aviso na tela de configuracoes
- fase 7 dessa refatoracao foi entregue em `2026-04-30` com granularidade de escrita e template transacional:
  - `POST /v1/settings/templates/{templateId}/apply` aplica template em uma unica transacao
  - create/update/delete de options no frontend usam `POST`, `PATCH` e `DELETE`; `PUT` ficou para reorder/importacao
  - catalogo manual segue granular para create/update/delete
  - `ApplyOperationTemplate` preserva alertas/test-mode/autofill e troca core/modal/motivos/origens do template
  - proximo: Fase 8 (leitura segura por nome e reducao de `scan` manual)
- fase 6 dessa refatoracao foi entregue em `2026-04-30` com dual-read/write do core operacional:
  - `getCoreSettingsFromNew`: le 10 campos de `tenant_operation_core_settings`
  - `GetOperationSection` sobrepos CoreSettings + SelectedOperationTemplateID apos sobreposicao de alertas
  - `UpsertOperationSection` faz dual-write completo: core (nao fatal) + alertas (nao fatal) + legacy (autoritativa)
  - `upsertCoreSettingsToNew`: INSERT ON CONFLICT UPDATE para todos os campos do core
  - go test ./... verde; UX inalterada
  - proximo entregue: Fase 7 (granularidade de opcoes/catalogo e template transacional)
- fase 5 dessa refatoracao foi entregue em `2026-04-30` com dual-read/write de alertas:
  - `GetOperationSection` virou agregador: carrega legacy e sobrepos AlertSettings da nova tabela
  - `getAlertSettingsFromNew`: le os 4 thresholds de `tenant_alert_settings`
  - `UpsertOperationSection` faz dual-write: escreve alertas na nova (nao fatal) e tudo na legacy (autoritativa)
  - `upsertAlertSettingsToNew`: INSERT ON CONFLICT UPDATE para os 4 campos de alerta
  - go test ./... verde; UX inalterada
  - proximo: Fase 6 — dual-read/write do core operacional em tenant_operation_core_settings
- fase 4 dessa refatoracao foi entregue em `2026-04-30` com dual-read/write do modal:
  - `0043_fix_modal_config_jsonb_keys.sql`: converte backfill snake_case -> camelCase no jsonb, adiciona finishFlowMode
  - `GetModalSection` virou agregador: leitura preferencial de `tenant_finish_modal_settings`, fallback para legacy
  - `UpsertModalSection` faz dual-write: escreve na tabela nova (nao fatal) e na legacy (autoritativa)
  - `getModalSectionFromNew`: usa json.Unmarshal no config jsonb; coluna finish_flow_mode prevalece
  - 3 testes novos de round-trip JSON: marshal/unmarshal preserva campos, campos ausentes dao zero values, normalizacao corrige zeros
- fase 3 dessa refatoracao foi entregue em `2026-04-29` com as migrations de banco:
  - `0040_tenant_operation_core_settings.sql`: campos operacionais estaveis (10 colunas tipadas + updated_by + updated_at), backfill de tenant_operation_settings
  - `0041_tenant_alert_settings.sql`: thresholds de alerta (4 colunas + updated_by + updated_at), backfill de tenant_operation_settings
  - `0042_tenant_finish_modal_settings.sql`: modal com finish_flow_mode, schema_version e config jsonb (schema v1 com todos os campos do modal), backfill via jsonb_build_object
  - as tres tabelas novas existem e estao preenchidas mas ainda nao sao lidas nem escritas
  - proxima fase: dual-read com fallback para legacy, comecando pelo modal
- fase 2 dessa refatoracao foi entregue em `2026-04-29` no backend:
  - `GetByTenant` virou agregador de leitura
  - `GetOperationSection`, `GetModalSection`, `GetOptionGroup` e `GetProductCatalog` viraram portas internas oficiais
  - `UpsertOperationSection` e `UpsertModalSection` passaram a gravar apenas a propria secao na tabela legacy atual
  - `SaveOptionItem` e `SaveProductItem` passaram a preferir mutacao granular, materializando defaults apenas quando a colecao ainda esta vazia
- para smoke local de Fase 1, `GET /v1/settings` aceita falha simulada apenas
  fora de `production`:
  - query `__debugSettingsFailure=500|slow-500`
  - cookie `ldv_debug_settings_failure=500|slow-500`
  - guia oficial: `PHASE1_SMOKE_GUIDE.md`

## Override por loja

Por enquanto nao existe overlay de loja. Quando um caso real exigir uma
configuracao especifica por loja (ex: template operacional diferente em uma
unica unidade), a abordagem combinada e:

- criar uma tabela `store_<recurso>_override` apenas para aquele recurso
- expor um seletor interno daquela secao na UI ("Personalizar para loja X")
  com aviso visual claro de que sera um override por loja
- nao reaproveitar o seletor de loja generico do header para isso
