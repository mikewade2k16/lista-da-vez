# AGENTS — back/internal/modules/meta_ads

## Escopo e fonte de verdade

Estas instruções valem para o módulo Go `meta_ads`. Regras herdadas:
`AGENT_RULES.md`, `docs/ENGINEERING_PRINCIPLES.md` e
`docs/meta-ads/ASSISTENTE_360_STATUS_E_ROADMAP.md`.

Go/PostgreSQL são autoritativos para conexão, inventário, cache de relatórios,
escopo agência→cliente, policies, propostas e execução. A Graph é uma fonte externa;
o modelo/LLM nunca fornece IDs, métricas ou autorização por conta própria.

O produto possui um único chat em `/v1/assistant/chat/*`, implementado no módulo
Calendar com adapters injetados no composition root. Meta Ads fornece contexto
read-only, resources/cards e um executor de propostas. Não criar outro histórico,
prompt, OAuth ou UI de chat neste módulo.

## Isolamento e autorização

- `accountID` vem de `auth.Principal` hidratado por `RequireAuthWithAccount`; não reler
  `X-Account-Id` nem aceitar account no body.
- Toda tabela operacional possui owner `account_id`. Recurso fora de owner/scope
  retorna `404`.
- Leituras exigem `meta_ads.view`, sync/mapping/policy/action exigem
  `meta_ads.manage`, conexão/OAuth exigem `meta_ads.connect`.
- Conta-cliente pode usar a conexão central da agência canônica somente para ad
  accounts e identidades Page/Instagram explicitamente vinculadas àquele cliente.
- O store e o service repetem filtros de organização, conexão ativa, token válido,
  `is_current` e mapping. Não confiar em filtro do frontend ou do prompt.
- Permission set não resolvido é fail-closed; owner/platform continuam sujeitos aos
  guardrails financeiros e ao kill switch.

## Arquitetura atual

Arquivos principais:

- `meta_client.go`: Graph read client, paginação por cursor e insights.
- `meta_client_instagram.go`: Pages/Instagram Business e mídia recente.
- `meta_client_actions.go`: GET/preflight e POST de ações first-party.
- `oauth_*.go`: Facebook Login first-party, state hash/TTL/single-use e grants.
- `store_snapshots.go`: token + ad accounts e reporting snapshots por revision.
- `store_assistant_context.go` / `assistant_context.go`: projeção bounded para o chat.
- `store_instagram_mapping.go` / `service_instagram_mapping.go`: Page/IG→cliente.
- `store_actions.go`, `action_guard.go`, `service_actions.go`: proposal, policy,
  lifecycle, claim, auditoria e reconcile.
- `action_connection_lease.go`: token-at-revision sob advisory lock.
- `action_executor_graph.go` / `action_executor_instagram.go`: executor first-party
  de campanhas e da árvore de anúncio baseada em post real.
- `action_steps.go`: recibos at-most-once por campaign/ad set/creative/ad.
- `http*.go`: handlers finos; validação e autorização permanecem no service/store.

`service_assistant.go`, `store_assistant.go`, `runner_client.go` e o sidecar
`meta-ads-assistant` são legado/compatibilidade interna. As rotas públicas
`/v1/meta-ads/assistant/*` não são registradas e têm teste de ausência. O runner
permanece account-isolated, autenticado e read-only; não é executor de produto.

## Migrations e dados

- `0149`: `connections`, `ad_accounts`, `campaigns`, `insights_daily`.
- `0150`/`0151`: histórico/config do assistente legado; não são fonte do chat 360.
- `0282`: capabilities por surface e `entry_surface` imutável do Assistente 360.
- `0283`: resources das mensagens e módulos de contexto persistidos.
- `0284`: referências tenant-scoped às credenciais do cofre compartilhado.
- `0285`: OAuth states SHA-256, TTL 10 minutos e consumo atômico.
- `0286`: action policies, proposals e eventos append-only.
- `0287`: recibos idempotentes de `/ask` e execução atômica dos cards locais do
  Calendário; embora pertença ao módulo Calendar, integra o contrato compartilhado
  do Assistente 360.
- `0288`: vínculo `(ig_user_id,page_id) -> client_account_id`.
- `0289`: binding ao card, cancelamento e expiração da proposal.
- `0290`: snapshots/hashes de connection/mapping/policy/campaign e claim guardado.
- `0291`: connection revision e `is_current` fail-closed para caches.
- `0292`: credenciais Anthropic no Assistente 360 compartilhado.
- `0293`: guard BRL também para criação de campanha com budget.
- `0294`: action `promote_instagram_post` e recibos por etapa da árvore de anúncio.

As migrations 0282–0294 formam a cadeia local do Assistente 360/Meta e devem ser
validadas na ordem; ausência de um número intermediário não pode ser ignorada.
Não editar migration aplicada. Mudança posterior usa novo número e backfill seguro.
Caches legados não comprovados permanecem não atuais até refresh/sync.

Checkpoint 2026-08-24: staging estava em `0158`; para não pular dependências, a
sequência pendente `0159–0294` foi aplicada transacionalmente após backup. O banco
terminou com todos os 13 registros `0282–0294` e voltou ao estado desligado, sem
deploy da API/web. A integração PostgreSQL
`TestAgencyTwoClientScopePostgresIntegration` é o gate local de escopo: agência,
dois clientes e organização externa devem provar mappings, `all`/cliente, contexto
Instagram e bloqueio cross-org. Ela também protege o cast explícito do owner UUID
em `ListAssistantAdAccounts`; remover o cast volta a quebrar o primeiro contexto
Meta conectado com `operator does not exist: uuid = text`.

Checkpoint P0 local 2026-08-27: o criativo de `promote_instagram_post` usa os
campos aceitos pelo codegen atual do Business SDK oficial: `object_id` para a Page,
`instagram_user_id` e `source_instagram_media_id`. `page_id` top-level foi removido.
Os testes do executor/HTTP cobrem targeting somente Instagram, árvore pausada,
timeout pós-request como `unknown`, `429` sem repetir etapa, replay terminal sem
nova execução e reconciliação pelos recibos parciais. Isso endurece o contrato
local, mas não substitui o aceite desses campos por um asset Graph real.

Tokens ficam cifrados com pgcrypto, nunca são devolvidos ou logados. Toda reconexão
gera nova `connections.revision`; token/expiry e snapshot de ad accounts são gravados
na mesma transação.

## Facebook Login e token manual

`POST /v1/meta-ads/oauth/start` cria state aleatório, persiste somente o hash e devolve
a authorization URL. `GET /v1/public/meta-ads/oauth/callback` resolve account/user
exclusivamente pelo state consumido; nunca por query/body fornecido pelo navegador.

O callback troca code→short-lived→long-lived token server-side e exige `granted` em:

- `ads_management`;
- `ads_read`;
- `business_management`;
- `pages_show_list`;
- `pages_read_engagement`;
- `instagram_basic`.

O fallback manual exige os mesmos grants antes de consultar/salvar ad accounts.
Missing/declined/provider error não persiste conexão e não reflete token ou corpo livre
da Meta. Login, 2FA, consentimento, criação do Meta App e app review são etapas humanas.
Refresh/revogação automática ainda é roadmap.

## Snapshot e relatórios

O sync:

1. captura owner, ad account e connection revision;
2. busca todas as páginas de campanhas e os dois níveis de insights `last_90d`,
   `time_increment=1`, antes de abrir a transação;
3. se qualquer página/consulta falhar, não altera o banco;
4. sob lock/revision check, marca ausentes como não atuais e substitui somente o recorte
   coberto;
5. token rotacionado durante a busca retorna `connection_changed` e descarta o snapshot.

List/Get/overview/action/context filtram conexão ativa, token não expirado e recursos
`is_current`. Snapshot vazio é um zero real, não preserva item antigo.

Conversões continuam sendo soma heurística de actions reconhecidas. Não inventar ROAS,
CPA ou receita até existir fonte/atribuição autoritativa.

## Contexto do Assistente 360

`AssistantContextBundleForScope` devolve somente dados read-only e não confiáveis para
o prompt:

- conexão;
- até 12 ad accounts autorizadas;
- até 100 campanhas atuais;
- performance por ad account: 30d, 7d, 7d anteriores e série diária;
- spend, impressions, clicks, reachDailySum, CTR, CPC e conversions;
- até 12 contas/90 dias/360 pontos, com freshness `fresh|stale|empty`;
- identidades e até 12 posts reais do Instagram, com autoria Page/IG/cliente.

Flags `adAccountsTruncated`, `campaignsTruncated`, `performanceTruncated` e
`dailyTruncated` dizem ao modelo que o recorte não é a lista completa. Nunca incluir
token, segredo, encrypted fields ou payload Graph bruto.

No client scope, Instagram exige interseção exata entre identidade atual da Graph e
mapping 0288. Sem mapping, `scope_unavailable`; não fazer fallback para `accounts[0]`.

## Propostas e escrita segura

Actions declaradas: `create_campaign`, `duplicate_campaign`, `update_campaign`,
`pause_campaign`, `resume_campaign`, `promote_instagram_post`. Toda action deve
continuar fail-closed quando executor, policy, source, mapping ou kill switch não
permitirem a execução.

Lifecycle:

1. intenção fechada é validada e persistida unbound;
2. somente após a mensagem/card existir, o backend faz source binding;
3. proposal expira em 30 minutos; rejeição/exclusão cancela durablemente;
4. confirmação visual ou textual exata revalida principal, scope, capability write,
   mapping, policy, campaign e snapshots;
5. claim atômico persiste connection/revision e attempt único antes da Graph;
6. resposta ambígua vira `unknown`, nunca retry cego; reconcile apenas observa.

Confirmação textual aceita somente o comando exibido no card:
`CONFIRMAR META <prefixo>` ou `CONFIRMAR GASTO META <prefixo>`. “Sim” ou
“confirmar” genérico não executa.

Executor concreto atual:

- `create_campaign`, sempre `PAUSED`;
- `duplicate_campaign`, deep copy `PAUSED`;
- `pause_campaign`;
- `update_campaign` de nome;
- `update_campaign` de budget somente em BRL, sob cap e confirmação reforçada;
- `resume_campaign` somente para budget CBO vivo em BRL dentro do cap;
- `promote_instagram_post`: revalida mapping + Page/IG + post vivo e cria campaign,
  ad set, creative e ad; campaign/ad set/ad são `PAUSED` e cada POST tem recibo.

Indisponíveis/fail-closed:

- criação/edição manual genérica de ad sets, ads e creatives fora do fluxo de post;
- audiences/interesses/exclusões, placements adicionais, bids e schedules livres;
- qualquer moeda de budget diferente de BRL;
- retry automático de uma criação externa com resposta incerta.

`META_ADS_WRITES_ENABLED=false` é o default em todos os ambientes. Não ligar antes de
migrations, Meta App/grants e E2E staging cobrirem timeout, 429, duplo clique, rotação,
expiry, delete, mapping/policy drift, unknown e reconcile.

O lease de ação mantém um advisory session lock na mesma conexão pgx usada para ler o
token da revision e durante GET/POST Graph. Reconnect/delete usam o mesmo namespace de
advisory xact lock e aguardam. Nunca segurar transação PostgreSQL durante chamada Graph.

## Endpoints atuais

Leitura/conexão:

- `GET /v1/meta-ads/overview`
- `POST|DELETE /v1/meta-ads/connection`
- `POST /v1/meta-ads/oauth/start`
- `GET /v1/public/meta-ads/oauth/callback`
- `GET /v1/meta-ads/ad-accounts`
- `GET /v1/meta-ads/campaigns`
- `GET /v1/meta-ads/insights`
- `POST /v1/meta-ads/sync`

Escopo/policy/actions:

- `PATCH /v1/meta-ads/ad-accounts/{id}/client`
- `GET|PATCH /v1/meta-ads/instagram-identities[/...]`
- `GET|PUT /v1/meta-ads/ad-accounts/{id}/action-policy`
- `GET|POST /v1/meta-ads/action-proposals`
- `GET /v1/meta-ads/action-proposals/{id}`
- `POST /v1/meta-ads/action-proposals/{id}/confirm`
- `POST /v1/meta-ads/action-proposals/{id}/cancel`
- `POST /v1/meta-ads/action-proposals/{id}/reconcile`

Mutações idempotentes exigem `Idempotency-Key`. Body usa decoder estrito e tamanho
limitado. Mapear drift/stale/concurrency sem vazar existência de recurso cross-tenant.

## Variáveis

- `META_ADS_GRAPH_BASE`
- `META_ADS_CRYPTO_KEY`
- `META_ADS_APP_ID`
- `META_ADS_APP_SECRET`
- `META_ADS_OAUTH_REDIRECT_URL`
- `META_ADS_WRITES_ENABLED` (default obrigatório `false`)

Variáveis `META_ADS_ASSISTANT_*` pertencem ao runner legado e não habilitam o chat 360
nem writes do executor first-party.

## Validação mínima

- `gofmt` e `go test ./internal/modules/meta_ads ./internal/platform/database`;
- migration suite completa em PostgreSQL limpo;
- testes Store reais para snapshot/revision, claim/stale e lease rotate/delete/expiry;
- `golangci-lint` focado;
- testes frontend do store, conexão, policy, cards e Assistente 360;
- `docker compose config` dev/prod;
- staging E2E antes de alterar o kill switch.

Não executar migrate, deploy, OAuth real, commit ou chamada Graph de write sem a
autorização correspondente.
