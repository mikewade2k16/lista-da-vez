# AGENT — platform/app

## Escopo

Pacote `back/internal/platform/app/`. Bootstrap do binário: cria pool, monta
config, instancia módulos (legados + Module Registry quando
`CORE_V2_ENABLED=true`), pluga middlewares globais (CORS, RequestID, RateLimit,
Logging) e retorna o handler HTTP final.

## Peças

- `app.go` — `BuildHandler(...)`: ponto de entrada. Cria stores (`tenants`,
  `users`, `stores`, `consultants`, `settings`, `auth`, etc.), services, HTTP
  handlers; quando `CORE_V2_ENABLED`, instancia `Registry` e roda
  `SyncCatalog` + `Build`.
- `context_http.go` — `/v1/me/context` legado.
- `*_adapter.go` — adapters finos entre módulos legados (ex:
  `operations_store_scope_adapter.go`).
- `GET /healthz` (em `app.go`) — desde AC-16 faz `pool.Ping` (timeout 2s) e devolve
  `200 + db:"ok"` ou `503 + db:"unreachable"` (erro só no `slog.Warn healthz_db_ping_failed`,
  nunca no corpo — endpoint público). Consumidores do 200: healthcheck do compose, smoke do
  deploy, UptimeRobot e `scripts/monitoring/check-vps.sh`.

## Estado do multi-tenant — atualizado 2026-06-04 (C20 — guard ATIVO)

### O que está ativo agora

- **`AccountModulesGuard` ATIVO** — instanciado uma vez como `modulesGuard`, passado via `Dependencies.ModulesGuard` (parou de ser descartado com `_ =`) E aplicado no `Chain` via `modulesGuard.RequireModuleByPath(moduleGatingRules())`. O gating é centralizado por prefixo de path (gate-list), não por wrapper em cada handler — ver `moduleGatingRules()` neste pacote e [httpapi/AGENT.md](../httpapi/AGENT.md).
- **Invalidação reativa** — `app.go` assina `account.modules.changed` → `modulesGuard.Invalidate(accountID)` (403 sem esperar o TTL de 60s).
- **`SetAccountChecker` chamado** — `authMiddleware.SetAccountChecker(auth.NewPostgresAccountMemberChecker(pool))` habilita `RequireAuthWithAccount` (membership em `core.account_users`).
- **`queue.New()` e `crm.New()` registrados no Registry** — sem isso `core.modules` não teria as entradas `queue`/`crm`, a seed 0124 viraria no-op e o guard fail-close quebraria queue/crm. Rotas continuam no wiring legado (`Build` retorna handle sem rotas).
- **`core.account_modules` seedado** — migrations 0124/0125 populam `queue`, `tasks`, `crm` para accounts ativas.

### Gating por path (espelha o front)

O prefixo `/v1/performance-feedback` tambem pertence ao modulo `queue` e segue o
mesmo gate por account das rotas de analytics e operacao.

Gateados: `queue` (`/v1/operations,/v1/alerts,/v1/reports,/v1/analytics,/v1/feedback,/v1/consultants,/v1/settings,/v1/stores`), `crm` (`/v1/erp,/v1/catalog`), `tasks` (`/v1/tasks,/v1/task-boards`) e os módulos satélites declarados em `moduleGatingRules()`, incluindo `social_publishing` (`/v1/social-publishing`). Rotas não listadas (auth, me, **admin**, users, notifications, access, tenants, webhooks, realtime, bi, roadmap) passam direto. O front injeta `X-Account-Id = auth.activeTenantId` em todo request via `createApiRequest` (plugin `account-id-bridge.client.ts`).

### O que ainda está pendente

- `CORE_V2_ENABLED=false`: Registry não roda; `modulesGuard` fica nil; o gating é pulado (modo legado sem multi-tenant) e endpoints `/v2/me/*` não existem.
- `RequireModule("site")` nas rotas admin do site segue deferido (site não está na seed 0124; rotas `/v1/admin/*` não são gateadas).

## Regras ao mexer aqui

- Não introduzir feature flag nova sem registrar em [config/AGENT.md](../config/AGENT.md).
- Rota nova de módulo que deve ser gateada por contratação: adicionar o prefixo em `moduleGatingRules()` (mapa prefixo → moduleId). O front precisa enviar `X-Account-Id` (já automático via `createApiRequest`).
- Rotas de gestão platform_admin (`/v1/admin/*`) NÃO entram no gate-list — são gestão, não uso por tenant.
- Não importar nada de `web/server/` no Go (BFF Nitro marcado para remoção em C7).

## Comportamento sem feature-flag

`CORE_V2_ENABLED=false` (default): Registry e guard não existem. Apenas wiring legado.

`CORE_V2_ENABLED=true`: Registry roda, SyncCatalog popula catálogos, guard ativo, `RequireAuthWithAccount` disponível.

## PrincipalCache wiring (AC-01)

- `principal_cache_wiring.go` — `wirePrincipalCache(cfg, logger, authService, accessService, usersService)` cria o `httpapi.PrincipalCache[auth.Principal]` quando `AUTH_PRINCIPAL_CACHE_TTL > 0` (default `30s`; `0s` retorna `nil` e loga `principal_cache_disabled`). Liga os setters de `auth`/`access`/`users` e dispara a goroutine de manutencao (`Cleanup()` a cada 60s + log `principal_cache_stats` a cada 5 min quando houve trafego).
- `app.go` (2 pontos): chama `wirePrincipalCache` logo apos criar `usersService`; injeta o cache retornado em `modules.Dependencies{PrincipalCache: principalCache}` (nil-safe — `core` faz nil-check no Build). Invalidacao e direta/sincrona, NAO via bus. Fora do bloco `CoreV2Enabled` o cache ja funciona para auth/access/users; os setters do `core` so ligam quando o Registry roda.

## Referências

- [docs/ROADMAP.md → "Estado real em 2026-05-28"](../../../../docs/ROADMAP.md)
- [docs/MULTITENANT_COMPLETION_PLAN.md](../../../../docs/MULTITENANT_COMPLETION_PLAN.md)
- [docs/CONTRACT_FREEZE.md](../../../../docs/CONTRACT_FREEZE.md)
- [platform/httpapi/AGENT.md](../httpapi/AGENT.md)
- [platform/modules/AGENT.md](../modules/AGENT.md)
- [modules/core/AGENT.md](../../modules/core/AGENT.md)

## Wiring das fontes de Customer Intelligence (2026-07-23)

- `app.go` registra os adapters owner-owned de Calendar, ERP, Site e BI no
  `customerintelligence.Service`.
- ERP e Site passam primeiro por Customer Data e so consultam o owner para
  `source_link` exato. Calendar produz Business Context sem escopo pessoal. BI
  faz apenas health/validacao local e nao dispara chamada externa.
- Os adapters vivem neste pacote para preservar a direcao de dependencias:
  modulos nao importam Customer Intelligence nem um ao outro.
- Erro permanente de contrato/configuracao e classificado por reason code e nao
  entra em retry; indisponibilidade de qualquer fonte permanece isolada do
  runtime de chat.

Finance recebe a instância já inicializada de `secretbox.Box` no registro do módulo (2026-11-02), para o cofre write-only de credenciais fiscais pessoais. Não foi criada outra chave/env nem alterado o pipeline de autenticação.

## Chat Finance (2026-11-02)

Bootstrap injeta resolvedor do cofre Automation no Finance e runtime/contexto Finance no Calendar via `WithFinanceAssistant`. Dependência por funções, sem importar Calendar dentro do Finance. Erros de planilha/perfil fora do escopo mapeados para404.

## Accounting — CT-03

Registro `accounting.New(...)`, prefixo `/v1/accounting` no gating. `accounting.go`
compõe Policy de Auth/Core, fonte pessoal owner Finance e storage privado. Nenhum
conector fiscal/fake é registrado em runtime. Módulo permanece opt-in por conta;
não semear habilitação ou permissões do usuário. A configuração Plus continua no
owner Finance e não faz parte desta entrega de domínio.

## Wiring Plus — CT-05

`finance_plus.go` conecta portas Calendar → Finance → Accounting. Accounting
instanciado uma vez. Autorização reconsulta associação ativa, módulo e permissões
efetivas; grants de emitente/recurso continuam no owner. Sem SQL de negócio aqui.
Porta Knowledge ausente até CT-06: indisponibilidade explícita. Teste HTTP real
com fixture sintética em `PLUS_APP_TEST_DATABASE_URL` (`plus_test_app*`).

## Wiring conhecimento — CT-06

accounting_knowledge.go conecta Storage privado e extrator PDF Finance por portas
a Accounting/knowledge; Plus recebe a busca escopada. Regras/SQL continuam nos
owners. Instância de Accounting única, sem dependência comercial de Omnichannel.
Teste HTTP Plus usa PostgreSQL descartável e verifica evidência real/revogação.

## Composição IA Plus — CT-07

Finance usa llm.Client nativo; credencial/modelo Plus resolvidos pela mesma porta
Automation existente. accountingEvidenceValidator injeta validação em lote do
owner Knowledge; Calendar delega ValidateOutput para Finance. Sem lógica fiscal
ou SQL de outro módulo em platform/app. Teste real HTTP usa somente cliente fake
e PostgreSQL descartável, inclusive replay e histórico após revogação.

## Artefatos privados — CT-08

accountingStorage também implementa ArtifactReader pela API pública
Storage.DownloadForSource(account,object,"accounting",""). Accounting valida grant/
referência/hash antes de expor e revalida após leitura. Wiring na instância única
de Accounting. Nenhum connector NFS-e/fake foi registrado em runtime: pré-requisitos
e estado parcial em docs/accounting/handoffs/CT-08.md. Não altera sessão Finance.

## Transcrição local Finance — 2026-11-06

Composição injeta WithFinanceTranscriptionAuthorization pelo authorizer canônico
Finance/Core existente (associação, módulo, finance.sheets.view). Calendar controla
a política de transcrição local e revalida antes/depois; platform não contém regra
de emissão ou chave/modelo de conversa. Whisper local não depende de cfg.Enabled.

## Autorização Finance por conta — 2026-11-06

`ResolveAccountPrincipal` revalida visibilidade da conta, usuário ativo, papéis e
RBAC efetiva no owner Auth. O adapter Finance usa a política canônica
`CanFinancePermission`: admin transversal e owner com `core.account.manage`
seguem o mesmo contrato das rotas Finance; custom precisa da permissão efetiva.
Módulo Finance habilitado continua obrigatório neste adapter. Accounting
mantém membership e grants de negócio explícitos. Teste PostgreSQL descartável
em `finance_scope_test.go` cobre permissões, revogação e módulo desabilitado.
O cliente selecionado no chat é enviado em header, validado no middleware e
revalidado no serviço; não muda a conta global da página.

## Busca pública no chat — pacotes 1/2, 2026-11-06

0317 cria `knowledge.public_sources`: acervo global exclusivamente público,
curado e informativo; não possui tenant porque não recebe uploads ou dados privados.
Não copia nem abre `messaging.knowledge_*`/bindings Accounting. Sem endpoint de
publicação pública, sem scraping no chat. Versões publicadas imutáveis (trigger),
retirada por status; nova busca seleciona versão publicada mais recente. Referências
históricas publicadas permanecem válidas; retiradas são ocultadas em lote no GET.
0318 adiciona cinco sínteses curtas com origem/revisão/limites em manifesto, sem
alterar configurações, contas, credenciais ou seeds anteriores. Vigência fiscal
não confirmada; são evidências informativas, nunca regra de cálculo do emitente.

SearchPublic → Store, FTS simple com normalização de acentos, até12termos e3trechos.
Aliases de vocabulário no roteador não contêm valores fiscais. Finance usa portas
PublicKnowledge/ValidatePublicEvidence, autorizadas pelo Finance canônico antes/
depois. Não exige Accounting contratado, emitente, modelo, cofre, Jeff ou API paga
para ler material público. Corpus privado empresarial conserva todos os gates.

finance.access_messages.public_evidence persiste snapshot estruturado separado do
texto; List revalida até180IDs e oculta fontes retiradas/indisponíveis. Conta/usuário
continuam filtrados no SQL. Perguntas fiscais explícitas no chat chamam busca antes
de qualquer modelo. Falta de fonte difere de falta de serviço. Nenhuma resposta é
marcada como confirmação jurídica ou execução de nota/declaração.

Frontend reutiliza AccountingEvidence e cartões/mensagens do Crow; mostra título,
trecho, fonte HTTPS e versão. Roteiro manual em docs/finance/ROTEIRO_TESTES.md.
Testes PostgreSQL sintéticos, corpus público inicial e regressões de conversa
passaram. Comparação visual, microfone real e aceite do usuário não executados.
