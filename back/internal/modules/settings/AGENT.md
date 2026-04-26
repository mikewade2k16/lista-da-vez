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

## Contrato atual

- `GET /v1/settings`
- `PUT /v1/settings`
- `PATCH /v1/settings/operation`
- `PATCH /v1/settings/modal`
- `POST /v1/settings/options/{group}`
- `PATCH /v1/settings/options/{group}/{itemId}`
- `DELETE /v1/settings/options/{group}/{itemId}`
- `PUT /v1/settings/options/{group}`
- `POST /v1/settings/products`
- `PATCH /v1/settings/products/{itemId}`
- `DELETE /v1/settings/products/{itemId}`
- `PUT /v1/settings/products`

Os endpoints continuam aceitando `storeId` no payload e na query string para
nao quebrar clientes legados, mas o backend ignora esse valor e resolve o
tenant pelo principal autenticado. Nunca usar `storeId` para escolher escopo
de gravacao em settings.

## Regras de escopo

- leitura: qualquer usuario com acesso ao tenant
- escrita: `owner` e `platform_admin`
- escopo de gravacao: tenant resolvido pelo principal (`principal.TenantID`)

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
- antes de gravar uma opcao recebida via `POST /options/{group}`, o service materializa os defaults do grupo se a tabela ainda estiver vazia para aquele tenant; isso garante que um cadastro novo nao "apaga" os defaults vistos no front
- mudanca de settings publica somente `context.updated`:
  - `resource = settings`, `action = updated`, `resourceId = {tenantId}`
  - todos os clientes do tenant revalidam o bundle apos receber esse evento
  - o canal `operation.updated` deixou de ser usado por settings; o canal de contexto ja chega a todos os atendentes do tenant

## Override por loja

Por enquanto nao existe overlay de loja. Quando um caso real exigir uma
configuracao especifica por loja (ex: template operacional diferente em uma
unica unidade), a abordagem combinada e:

- criar uma tabela `store_<recurso>_override` apenas para aquele recurso
- expor um seletor interno daquela secao na UI ("Personalizar para loja X")
  com aviso visual claro de que sera um override por loja
- nao reaproveitar o seletor de loja generico do header para isso
