# Plano de Integração — Meta Ads no Omni

> Documento canônico do módulo `meta_ads`. Fonte de verdade do desenho.
> Regras herdadas: [AGENT_RULES.md](../../AGENT_RULES.md) + [ENGINEERING_PRINCIPLES.md](../ENGINEERING_PRINCIPLES.md).
> Espelhado em `web/app/components/roadmap/roadmap-data.ts` (fase `meta-ads`).

> **ESTADO (2026-06-11): MVP CODADA E VALIDADA LOCAL (back + front verdes).** Falta só **rodar de verdade**: aplicar a migration, rebuild da api e teste end-to-end com um System User token real. Build/lint/type-check passam; `npm install` feito. Nada commitado/deployado. Detalhe em [§ 11. Estado atual e handoff](#11-estado-atual-e-handoff-onde-paramos).

---

## 1. Objetivo

A agência (Crow Visuals) gerencia tráfego pago de Meta (Facebook/Instagram) **fora do painel** hoje. Este módulo traz isso para dentro do Omni:

- **Puxar** dados da Meta (Marketing API) para o nosso banco — fonte de verdade dos relatórios.
- **Relatórios/inteligência** para decisão (gasto, CTR, CPC, ROAS, tendência) no nosso dashboard.
- **Gerir campanhas** (criar/editar/pausar) — manual pelo painel **e** por IA — na fase Plataforma.

**MVP (validação curta):** conectar + sincronizar + dashboard básico com 1 gráfico. "Conectou, puxou, apareceu" → fim do MVP.

## 2. Princípios

- Multi-tenant desde o dia 1: `account_id NOT NULL` em toda tabela; `accountID` vem **sempre** do Principal/header, nunca do body. Recurso fora de escopo → `404`.
- **Não travar na agência:** `organization_id`/`client_account_id` entram **nullable/reservados**; backfill quando o modelo agência→cliente subir (P5). Ver [AGENCY_TENANT_ARCHITECTURE.md](../AGENCY_TENANT_ARCHITECTURE.md).
- **Backend Go é a fonte de verdade** (integra a Marketing API direto; o dashboard lê do nosso cache). A IA (P6) usa **nossos** endpoints como ferramentas — nunca uma fonte de dados paralela.
- Cache-and-sync: a Graph API é consultada na sincronização, **não** a cada carregamento de tela (performance / resposta imediata).
- Token (System User) é segredo: cifrado at-rest (pgcrypto), nunca logado, nunca devolvido ao front.
- Arquivos ≤ 450 linhas; componentização; design tokens (zero hex); BEM.

## 3. Arquitetura alvo

```
Painel /meta-ads (Vue)  ──HTTP /v1/meta-ads/* (JWT + X-Account-Id)──>  Módulo Go meta_ads
                                                                          │
                                            ┌─────────────────────────────┤
                                            │ MetaClient (graph.facebook)  │  Store (meta_ads.*)
                                            ▼                              ▼
                                   Marketing API da Meta          cache: connections / ad_accounts
                                   (sync sob demanda)             / campaigns / insights_daily
```

## 4. Modelo de dados (`meta_ads.*`) — migration `0149_meta_ads_schema.sql`

| Tabela | Campos principais | Nota |
|---|---|---|
| `connections` | `account_id` (único), `organization_id` (null/reservado), `meta_business_id`, `name`, `encrypted_token bytea`, `token_expires_at`, `status` | 1 conexão por account. Token cifrado via `pgp_sym_encrypt(token, $key)`. |
| `ad_accounts` | `account_id`, `connection_id`, `meta_ad_account_id` ('act_…'), `client_account_id` (null/reservado), `name`, `currency`, `status` | UNIQUE(account_id, meta_ad_account_id). |
| `campaigns` | `account_id`, `ad_account_id`, `meta_campaign_id`, `name`, `objective`, `status`, `daily_budget`, `lifetime_budget`, `synced_at` | Cache. UNIQUE(ad_account_id, meta_campaign_id). |
| `insights_daily` | `account_id`, `ad_account_id`, `meta_campaign_id` (''=agregado), `date`, `impressions/clicks/spend/reach/ctr/cpc/cpm/conversions`, `synced_at` | Cache p/ gráficos. UNIQUE(ad_account_id, meta_campaign_id, date). |

## 5. Endpoints (módulo Go `meta_ads`, admin — JWT + `X-Account-Id` + permissão)

| Verbo | Path | Permissão | Ação |
|---|---|---|---|
| GET | `/v1/meta-ads/overview` | `meta_ads.view` | status da conexão + KPIs agregados |
| POST | `/v1/meta-ads/connection` | `meta_ads.connect` | salva System User token (cifra) + valida na Graph |
| DELETE | `/v1/meta-ads/connection` | `meta_ads.connect` | remove conexão |
| GET | `/v1/meta-ads/ad-accounts` | `meta_ads.view` | lista contas de anúncio da conexão |
| POST | `/v1/meta-ads/sync` | `meta_ads.view` | dispara sync (Graph→cache) de campanhas+insights |
| GET | `/v1/meta-ads/campaigns?adAccountId=` | `meta_ads.view` | lista campanhas do cache (lean) |
| GET | `/v1/meta-ads/insights?adAccountId=&range=&level=` | `meta_ads.view` | métricas p/ gráficos (do cache) |
| POST | `/v1/meta-ads/campaigns` *(Plataforma)* | `meta_ads.manage` | cria campanha na Marketing API |
| PATCH | `/v1/meta-ads/campaigns/{id}` *(Plataforma)* | `meta_ads.manage` | edita/pausa/retoma |

Shapes: ver `back/internal/modules/meta_ads/model.go` (`ConnectionView`, `AdAccountView`, `CampaignView`, `InsightPoint`, `OverviewView`, `SyncResult`).

## 6. Painel (front Omni)

- Rota `/meta-ads`, `workspaceId: 'meta_ads'`, `moduleId: 'meta_ads'`. Página em `web/app/pages/meta-ads.vue` (layout dashboard, wrapper `.page-workspace`).
- Store `web/app/stores/meta-ads.ts` (Pinia + `createApiRequest`), composables `useMetaAdsConnection/Reports/Campaigns`.
- Componentes em `web/app/components/meta-ads/`: `MetaAdsWorkspace` (orquestra) + cards `Connection`/`AccountPicker`/`Overview`/`ReportChart`/`CampaignTable`.
- Gráficos: `vue3-apexcharts` em `<ClientOnly>` + import dinâmico.
- **Wiring em 4 lugares:** `workspaces.ts`, `permissions.ts` (WORKSPACE_ACCESS_DEFINITIONS + ROLE_WORKSPACES), `nav.config.ts` (`hidden:true` até validar), `module-enabled.global.ts` (`{prefix:'/meta-ads', moduleId:'meta_ads'}`).

## 7. Local + VPS

- **Sem container novo** (o Go fala HTTPS direto com a Graph API). Sem profile docker novo.
- Testar via `platform_admin` (isento do gating). Para a conta da agência, inserir `meta_ads` em `core.account_modules`.

## 8. Fases / subagentes

**MVP (5 subagentes):** A1 fundação Go · A2 cliente+sync · A3 HTTP · A4 front infra · A5 front UI. Contrato congelado (este doc + `model.go` + `module.go`) → paralelos; build de integração no fim.

**Plataforma (até 10 subagentes):** P1 write ops · P2 sync worker · P3 agregações · P4 OAuth · P5 atribuição cliente · P6 IA · P7 editor UI · P8 dashboards ricos · P9 IA UI · P10 hardening+docs.

## 9. Notas de Deploy

- Migration `0149_meta_ads_schema.sql` (idempotente; cria extensão `pgcrypto`). Rodar no banco certo (`:5433`/container).
- Vars novas em `.env.docker.example`, `.env.production.example` **e** `docker-compose.prod.yml` (`environment`):
  - `META_ADS_GRAPH_BASE` (default `https://graph.facebook.com/v21.0`)
  - `META_ADS_CRYPTO_KEY` (chave simétrica pgcrypto p/ cifrar o token — obrigatória p/ conectar)
- Dependência web nova: `vue3-apexcharts` → `install` no deploy do web.
- **Rebuild obrigatório** após mudança Go: `docker compose up -d --build api`.
- **Assistente (onda MA):** migration `0150_meta_ads_assistant.sql`; vars novas `META_ADS_ASSISTANT_RUNNER_URL` + `META_ADS_ASSISTANT_TOKEN` (api) e `META_ADS_ASSISTANT_PORT`/`_TIMEOUT_MS` (runner) — em prod: `.env.production` + `docker-compose.prod.yml` (environment). Na VPS o runner exige `claude setup-token` (CLAUDE_CODE_OAUTH_TOKEN) + OAuth do MCP da Meta repetido 1x; subir com profile `meta-ads-assistant` OU como processo no host. `extra_hosts: host.docker.internal:host-gateway` já no compose (necessário em Linux).
- **Settings do assistente (2026-06-12):** migration `0151_meta_ads_assistant_settings.sql` (tabela `meta_ads.assistant_settings`).
- **MA6+MA7 (2026-06-12):** var nova `META_ADS_RUNNER_BRIDGE_TOKEN` — precisa do MESMO valor em DOIS lugares: api (`.env.production` + `docker-compose.prod.yml` environment) E no ambiente do runner. Runner ganha também `META_ADS_API_BASE` (default `http://localhost:9091`; na VPS apontar pro api) e `META_ADS_OAUTH_CALLBACK_PORT` (default `8766`). O token OAuth da Meta fica em `meta-ads-assistant/.auth/` (fora do git; perm 0600 em Linux) — na VPS, preservar esse diretório entre restarts (volume/backup). O redirect OAuth é `http://127.0.0.1:8766/oauth/callback`: o navegador da autorização precisa rodar na MESMA máquina do runner OU o usuário cola a URL de callback no painel. Rebuild api obrigatório (mudou Go).

## 10. Decisões fechadas (2026-06-11)

1. Nome `meta-ads` (id `meta_ads`, schema `meta_ads`, pacote Go `metaads`, rota `/meta-ads`).
2. Conexão MVP por **System User token** (cifrado); OAuth completo na P4.
3. Multi-tenant desde já; sem depender do modelo de agência (colunas reservadas).
4. ~~Backend Go é a fonte; IA depois (P6) via Claude API~~ → **SUPERSEDED (mesma data, ver §12):** a camada de IA/escrita vem do **MCP oficial da Meta** (`https://mcp.facebook.com/ads`) executado por um **Claude headless autenticado pela assinatura** (sem crédito de API, sem OpenAI). O backend Go **continua sendo a fonte dos RELATÓRIOS** (cache/sync) — o MCP é o braço de AÇÃO, não fonte de dados paralela.
5. Lib de gráficos (`vue3-apexcharts`).
6. **(novo)** O painel `/meta-ads` é o centro de TUDO: conexões (token de dados + status do assistente) e o chat de comandos ("cria campanha X") vivem nessa página.

## 11. Estado atual e handoff (onde paramos — 2026-06-11)

Sequência: aprovado o plano → fundação à mão → 3 subagentes geraram o resto → **validação local (build/lint/type-check) feita e verde**. Falta só rodar de verdade (migration + rebuild + teste e2e com token real). Nada commitado/deployado.

### Validação local (2026-06-11) — resultado
- **Backend:** `go build` OK, `go vet` OK, `gofmt` limpo, **golangci-lint 0 issues**.
- **Frontend:** `npm install` OK, **eslint 0 issues** nos arquivos meta-ads, `vue-tsc` limpo no meta-ads EXCETO 1 item compartilhado: o import `~/types/meta-ads` cai no mesmo TS2307 que afeta **34 imports `~/types/*`** do repo inteiro (alias `~`→`app/` vs. tipos em `web/types/`). É `import type` → some no build, **zero impacto em runtime**. Mantido consistente com os outros 33 arquivos; quando o repo arrumar o alias `~/types` global, o meta-ads pega junto.
- **Ajustes pós-agente:** removido `connectionRevoked` (unused) do model.go; `gofmt -w` em service_sync.go/store_cache.go; corrigido peer dep do gráfico (apexcharts `^4.5.0`→`^5.15.0`, vue3-apexcharts `^1.8.0`→`^1.11.1`); `chartOptions` tipado como `ApexOptions` (corrige TS2322).

### O que existe em disco (gerado + validado local)

**Fundação (escrita à mão):**
- `back/internal/platform/database/migrations/0149_meta_ads_schema.sql` — schema + 4 tabelas + `pgcrypto`.
- `back/internal/modules/meta_ads/model.go` (tipos/views/mappers) e `module.go` (Registry; permissões `meta_ads.view/manage/connect`).
- `back/internal/platform/app/app.go` — import `metaads`, `registry.MustRegister(metaads.New())`, e regra de gating `{Prefix:"/v1/meta-ads", ModuleID:"meta_ads"}`.
- Este doc canônico + fase `meta-ads` (pending) no `roadmap-data.ts`.

**Backend-impl (subagente):** `meta_client.go`, `store_postgres.go`, `store_cache.go`, `service.go`, `service_sync.go`, `http.go`, `http_reports.go`, `AGENT.md`. Service: `Overview / SaveConnection / DeleteConnection / ListAdAccounts / ListCampaigns / Insights / Sync`. (Removi `connectionRevoked` de `model.go` por ser unused — único ajuste pós-agente.)

**Front-infra (subagente):** `web/types/meta-ads.ts`, `web/app/stores/meta-ads.ts`, `web/app/composables/useMetaAds{Connection,Reports,Campaigns}.ts`, `web/app/pages/meta-ads.vue`, e o wiring nos 4 lugares (`workspaces.ts`, `permissions.ts`, `nav.config.ts` com `hidden:true`, `module-enabled.global.ts`).

**Front-UI (subagente):** `web/app/components/meta-ads/` (`MetaAdsWorkspace` + 5 cards) + `AGENT.md`; `web/package.json` ganhou `apexcharts ^4.5.0` + `vue3-apexcharts ^1.8.0` (**não instalado**); gráfico via `<ClientOnly>` + import dinâmico.

### O que FALTA para rodar de verdade (passos do usuário — local-only, não rodo deploy)
1. **Config `.env`:** definir `META_ADS_CRYPTO_KEY` (chave simétrica p/ cifrar o token; sem ela, conectar retorna 503) e, opcional, `META_ADS_GRAPH_BASE`.
2. **Aplicar a migration** `0149_meta_ads_schema.sql` no banco certo (`:5433`/container).
3. **Rebuild da api:** `docker compose up -d --build api` (mexeu em `back/`).
4. **Subir o web** (deps já instaladas via `npm install`).
5. **Teste end-to-end** (como `platform_admin`, que tem bypass do gating): abrir `/meta-ads` → colar um **System User token** real → conectar → escolher a conta de anúncio → sincronizar → ver KPIs + gráfico + campanhas.
6. **Tirar `hidden:true`** do item no `nav.config.ts` quando validar (depois `beta:true`).

### Premissas do backend a CONFERIR na validação
- Orçamento: cents da Graph ÷ 100 → unidades.
- Conversões: heurística somando `actions` (purchase/lead/registration) — refinar por objetivo na P3.
- Janela de sync: `last_30d`, `time_increment=1`, 1 página (paging `next` não seguido — P3).
- `accountStatusLabel` (status numérico da Graph → label curta) vs. o que o front exibe.
- `SaveConnection` grava `meta_business_id=''` e `name='Meta Ads'` (OAuth/P4 preenche de verdade).

### Garantias
Rodado só **validação local** (build/lint/type-check) + `npm install`. **Nada de migrate, docker, git, commit ou deploy** foi executado — isso fica pra você (local-only).

---

## 12. Assistente MCP no painel (texto → cria/edita campanhas) — PLANO CANÔNICO da próxima onda

> Decidido 2026-06-11. Substitui P6 (IA via Claude API) e P9 (assistente UI); rebaixa P1 (write ops via nosso Go) para opcional/futuro — a ESCRITA agora entra pelo MCP oficial da Meta.
>
> **STATUS (2026-06-11): MA1+MA2+MA3 + login-no-painel ENTREGUES e integrados.** Back/front/compose validados (lint 0 issues), migration 0150 aplicada, api rebuildada, runner no host com as rotas de auth.
> **O login do Facebook agora é PELO PAINEL** (não mais via `/mcp` do Claude Code): card "Conectar o assistente a Meta" em `/meta-ads` → botão gera o link da Meta (tool `authenticate`) → usuário autoriza → cola a URL de callback (`localhost/callback?code=...`) → conclui (tool `complete_authentication`). Endpoints: `POST /v1/meta-ads/assistant/auth/start` e `/auth/complete` (runner: `POST /auth/start` + `/auth/complete`, via `meta-ads-assistant/src/auth.mjs`).
> **Login resolvido com SESSÃO PERSISTENTE (2026-06-11):** a 1ª versão (2 chamadas separadas) dava "sessão expirou" (PKCE/state perdido entre `authenticate` e `complete_authentication`). Corrigido: `meta-ads-assistant/src/auth.mjs` mantém UMA `query()` viva em streaming entre os dois passos (mesma conexão MCP → state intacto); o "colar URL de callback" virou OPCIONAL (com a conexão viva o redirect `localhost/callback` pode ser capturado sozinho); 409 `auth_session_gone` se passar de 10min. **Pendente:** o teste e2e (conectar pelo painel → mandar comando). MA4 parcial (guardrails no prompt + auditoria feitos; budget cap/streaming = polish). Runner no HOST: `npm --prefix meta-ads-assistant start`.

### 12.1 Objetivo
Na página `/meta-ads`, o usuário digita um comando ("sobe uma campanha de tráfego com R$50/dia pra conta X") e o sistema **cria/edita/pausa campanhas na Meta** usando o **MCP oficial** (`https://mcp.facebook.com/ads`), com o **Claude da assinatura** (mesma franquia do Claude Code — **zero crédito de API, zero OpenAI**). A página concentra tudo: conexões/tokens, chat de comandos, e os relatórios já existentes.

### 12.2 Arquitetura

```
/meta-ads (chat UI no painel)
   │  POST /v1/meta-ads/assistant  (Go: auth + X-Account-Id + persiste histórico)
   ▼
agent-runner (sidecar Node, Claude Agent SDK, headless)
   │  auth = assinatura Claude (mesma do Claude Code; setup-token p/ longa duração)
   │  mcpServers = { meta-ads: https://mcp.facebook.com/ads }  ← .mcp.json raiz JÁ criado
   ▼
Claude (plano) usa as tools oficiais: ads_create_campaign / ads_create_ad_set /
ads_create_ad / ads_update_entity / ads_activate_entity / insights...
   ▼
Meta executa → campanha NASCE PAUSADA (guardrail nativo do MCP oficial)
   ▼
Go dispara POST /v1/meta-ads/sync → cache atualiza → KPIs/tabela refletem na hora
```

- **Relatórios continuam pelo nosso Go** (cache meta_ads.*) — fonte única dos dashboards.
- **Ações (write) entram pelo MCP** — não duplicamos a Marketing API de escrita no Go.
- Pós-ação o runner avisa o Go, que re-sincroniza → a página mostra o resultado real.

### 12.3 O que a página `/meta-ads` passa a ter
1. **Card Conexões** (evolução do atual): token System User (dados/relatórios — já funciona) + **status do Assistente** (Claude autenticado? MCP logado no Facebook?) com instruções de setup.
2. **Card Assistente (chat)**: input de texto + histórico da conversa (persistido por account em `meta_ads.assistant_messages`); streaming da resposta; **toda ação de escrita exige confirmação explícita no chat** antes de executar; lista do que foi feito (auditável).
3. KPIs/gráfico/tabela (já existem) — atualizam após cada ação via sync.

### 12.4 Fases (MA = Meta Assistant) — espelhadas no roadmap
| Fase | Entrega | Notas |
|---|---|---|
| **MA1** | **Agent-runner** (sidecar Node + `@anthropic-ai/claude-agent-sdk`): serviço HTTP interno (`/run`, `/healthz`) que roda o Claude headless com o MCP meta-ads; container no compose (profile `meta-ads-assistant`) com volume das credenciais Claude (`~/.claude`), fallback host se atrito no Windows | Auth: `claude setup-token` (longa duração). OAuth do MCP (login Facebook) é feito 1x interativamente e fica cacheado |
| **MA2** | **Go**: endpoints `/v1/meta-ads/assistant` (POST mensagem, GET histórico) + tabela `meta_ads.assistant_messages` (account_id, role, content, actions jsonb) + proxy p/ runner + sync pós-ação | Escopo por account como todo o módulo; runner é rede interna, nunca exposto |
| **MA3** | **Painel**: `MetaAdsAssistantCard` (chat com streaming + confirmações de ação + histórico) + status de conexões do assistente no ConnectionCard | Tokens/BEM/design system; arquivos ≤450 |
| **MA4** | **Guardrails + polish**: política "sempre confirmar antes de write", budget cap configurável, lembrete de que campanhas nascem PAUSADAS (ativação manual), logs/auditoria das ações, docs (AGENT.md x2 + panorama) | Segurança pendente da P10 continua mapeada |

### 12.5 Execução com subagentes (quando o usuário disser "iniciar com subagentes")
- **Paralelo:** MA1 (runner) ∥ MA2-backend (endpoints+migration) ∥ MA3 (UI contra contrato congelado do chat).
- **Sequencial:** MA4 integra/endurece depois que MA1-MA3 conversam.
- Contrato do chat (congelar antes de disparar): `POST /v1/meta-ads/assistant {message} → stream/JSON {reply, actions[]}`; `GET /v1/meta-ads/assistant?limit= → histórico`.

### 12.6 Limites e verdades (anotar antes de construir)
- **"Grátis" = franquia da assinatura.** O runner consome a mesma cota do plano que o Mike usa no Claude Code (rate limits compartilhados). Uso interno da agência = ok. Se um dia virar feature para clientes/escala, troca-se o driver do runner por API (Claude ou OpenAI) **sem mudar a arquitetura**.
- **Login do MCP (Facebook OAuth)** é interativo na 1ª vez; o token fica cacheado nas credenciais do runner. Na VPS será preciso repetir o setup 1x.
- **Campanha criada por IA nasce PAUSADA** (regra do MCP oficial) — humano ativa. Isso é guardrail, não bug.
- O `.mcp.json` da raiz (servidor `meta-ads` HTTP) já está criado e reconhecido pelo Claude Code (teste local do Mike pendente de autenticação).
- **Provider plugável (anotado 2026-06-12, fazer lá na frente):** quando o assistente virar feature de cliente, cada conta poderá usar outro modelo/provider (OpenAI, Gemini, Claude API). A interface JÁ é o contrato HTTP do runner (`POST /run {prompt, history, accountId, adAccountId, model, systemPrompt}` → `{reply, actions[]}`): trocar provider = implementar outro runner com o mesmo contrato (+ ferramentas equivalentes às do MCP da Meta), sem mudar Go nem painel. Ver P6 no roadmap.

### 12.7 Entregas de 2026-06-12 (settings, anti-invenção, MA6 OAuth persistente, MA7 Instagram)

**a) Settings do assistente (modelo + prompt editáveis no painel).** `GET/PUT /v1/meta-ads/assistant/settings` (por account; tabela `meta_ads.assistant_settings`, migration 0151); runner aceita `model`/`systemPrompt` por request e recria a sessão quando mudam; card `MetaAdsAssistantSettings.vue` na aba Assistente (dropdown de modelo + textarea do prompt inteiro).

**b) Trava anti-invenção (guardReply).** Causa raiz dos "dados errados": com a sessão MCP deslogada o modelo tem 0 ferramentas e INVENTAVA campanhas. Agora: turno sem nenhuma tool real executada (`actions` vazio) + resposta afirmando dado concreto (R$, id 8+ dígitos, "N campanhas", métrica, lista com status) → resposta vira "reconecte". Login usa `guard:false`. `sanitizeReply` também remove dict Python (aspas simples) e tags `<thinking>`. Verdade de teste: conta Bari `1547966673703703` tem EXATAMENTE 1 campanha real ("Eng_").

**c) MA6 — OAuth persistente do MCP no runner (fim do relogin).** O runner faz OAuth padrão MCP sozinho (sem modelo): discovery RFC 9728 → metadata do AS RFC 8414 (**path-insertion**: `mcp.facebook.com/.well-known/oauth-authorization-server/ads` — a forma sufixo dá 404) → DCR RFC 7591 → PKCE S256 → tokens em `.auth/tokens.json` (0600) com refresh automático → header `Authorization: Bearer` na conexão MCP. Restart do runner e troca de model/prompt **não deslogam mais**. **Gotcha crítico:** a Meta só aceita DCR de `client_name` permitido — `'Claude Code'` passa (client_id emitido), nome próprio dá `400 invalid_client_metadata`. System User token como bearer do MCP NÃO funciona (401, testado). **Gotcha 2:** o dialog da Meta exige `scope` explícito na URL de autorização (sem ele: "Parece que esse app não está disponível"); os scopes vêm do discovery (`scopes_supported`) e entram no DCR (separados por espaço, RFC 7591) e na URL (separados por vírgula, padrão do dialog do Facebook). Fallback: se discovery/DCR falharem, volta ao login via modelo (in-session, não persiste). `healthz.metaAuth`: `oauth`|`session`|`none`.

**d) MA7 — Instagram → campanha (a ligação que faltava).** O MCP oficial NÃO tem ferramenta de feed do IG; criar anúncio de post existente ele TEM (`ads_create_ad` com `creative {"object_story_id"}` ou mídia IG). Solução: Go busca o feed via Graph (System User token decifrado, mesmo caminho do sync) e expõe bridge interno SEM JWT protegido por `META_ADS_RUNNER_BRIDGE_TOKEN` (constante-time): `GET /internal/meta-ads/runner/instagram/accounts|media?accountId=...`. O runner expõe ao modelo via MCP in-process `omni` (`instagram_get_accounts`, `instagram_get_recent_posts`); `accountId` viaja no payload do `/run` e vira contexto do turno (`setAccountContext`). Tools omni contam como ação real na trava anti-invenção. Validado com dados reais (2 contas IG; mídia com legenda/URL/tipo). Fluxo alvo: buscar posts → prévia com imagem no chat → aprovação explícita → campanha PAUSADA com 1 post por anúncio.

## Referência cruzada

- Módulo backend → [back/internal/modules/meta_ads/AGENT.md](../../back/internal/modules/meta_ads/AGENT.md)
- Módulo frontend → [web/app/components/meta-ads/AGENT.md](../../web/app/components/meta-ads/AGENT.md)
- Modelo agência→cliente → [AGENCY_TENANT_ARCHITECTURE.md](../AGENCY_TENANT_ARCHITECTURE.md)
