# Auditoria técnica — Omni (lista-da-vez)

**Data:** 2026-11-23 · **Branch:** `feat/observabilidade` (working tree com alterações não commitadas) · **Tipo:** análise estática, somente leitura.

## 1. Resumo executivo

| Severidade | Qtde | Tema dominante                                                                                                                    |
| ---------- | ---- | --------------------------------------------------------------------------------------------------------------------------------- |
| Crítico    | 1    | XSS armazenado em comentários de tasks (token em cookie legível por JS)                                                           |
| Alto       | 6    | Rate limit contornável, `/uploads` público listável, sem shutdown/recover, timeouts, listagens sem paginação, dependências do web |
| Médio      | 17   | Migrator sem lock, CI fraco, erros engolidos, FKs sem índice, acoplamento, bundle, backup                                         |
| Baixo      | 8    | 403 vs 404, a11y, waterfalls, artefatos soltos na raiz, defaults inseguros não bloqueados                                         |
| **Total**  | 32   |                                                                                                                                   |

**Pontos fortes confirmados (não são achados, mas calibram o relatório):**

- Isolamento por tenant bem aplicado: 15 handlers amostrados em 9 módulos (finance, accounting, customerdata, omnichannel, calendar, tools, bio, stores, cardapio, meta_ads, tasks) filtram `account_id` no SQL ou fazem pré-checagem de escopo no service. Nenhum IDOR/BOLA confirmado.
- SQL 100% parametrizado: os `fmt.Sprintf` com SQL montam apenas lista de colunas/`where` com placeholders (`tools/store_qr_codes.go:200`, `core/admin_repository.go:59`, `storage/repository.go:200` com coluna vinda de constante interna). Nenhuma injeção encontrada.
- Nenhum segredo literal versionado (`git ls-files` só lista `*.example` e arquivos de código com "secret" no nome; regex de chaves AWS/GitHub/Slack/OpenAI sem acerto).
- Webhooks usam `hmac.Equal` (`omnichannel/channel/meta_whatsapp/provider.go:157`, `instagram/provider.go:122`, `site/http_tracking.go:88`); SSRF guard em `omnichannel/ssrf.go:35` e `knowledge/ingestion_fetch.go:21`; segredos em repouso via `secretbox` com fail-fast.
- Idempotência presente nos fluxos críticos de omnichannel (39 arquivos), meta_ads (`action_model.go:52`), accounting, customerdata, storage, tasks, calendar.
- Seeds de dev são pulados em produção (`cmd/migrate/main.go:39` → `SkipDataSeeds`, lista em `migrator.go:34`).

## 2. Escopo e limitações

**Coberto:** `back/` (Go: platform + 32 módulos), `web/` (Nuxt: config, utils, amostra de componentes/páginas), migrations SQL, `.github/workflows`, `docker-compose.prod.yml`, scripts de backup.

**Não coberto / limitações:**

- **Nada foi executado em runtime**: sem `go test`, `go vet`, `govulncheck` (host sem Go; ver memória do projeto), sem EXPLAIN em banco, sem teste de carga. Achados de performance são por leitura de código e precisam de medição.
- **Cobertura de testes não medida** — só contagem de arquivos.
- **IDOR/BOLA por amostragem** (~15 handlers de ~600 rotas); não é prova de ausência. Recomendado teste automatizado cross-tenant por rota.
- **Varredura N+1/paginação/FK por heurística** (script que detecta `for` + `Query/Exec` e `List*` sem `limit`): pode haver falsos positivos/negativos. FK sem índice: heurística conta só a 1ª coluna dos índices e não enxerga índices compostos criados fora do padrão.
- **`npm audit`** executado (rede disponível); `npm audit --omit=dev` inclui dependências transitivas do Nuxt que podem não ser alcançáveis em runtime (SPA).
- **Bundle:** `.output/` é de 2026-11-16 (7 dias de defasagem); medidas de chunk valem como indicação, não como estado atual.
- **Working tree em movimento:** `back/cmd/api/main.go`, `platform/app/app.go` (assinatura de `BuildHTTPHandler` mudou), `httpapi/middleware.go`, `database/pool.go` e `platform/observability/` estão sendo alterados em paralelo (instrumentação `feat/observabilidade`). Linhas citadas refletem o disco em 2026-11-23; podem deslocar poucas linhas.
- Não auditados: n8n/workflows (`automation/`), `qa-bot/`, `meta-ads-assistant/`, Caddy/infra da VPS (não versionado: premissas sobre `X-Forwarded-For` não verificáveis), LGPD/retenção em profundidade.

## 3. Achados

Formato: **ID — título** · `arquivo:linha` · Problema · Risco · Correção.

### 3.1 Crítico

#### C-01 — XSS armazenado em comentários de tasks → roubo de sessão (inclusive platform_admin)

- **Local:** `web/layers/tasks/components/TasksTaskModal.vue:769` (`v-html="comment.bodyHtml"`); `back/internal/modules/tasks/service.go:711-712` (`AddComment` só faz `TrimSpace`, sem sanitizar); `back/internal/modules/tasks/http.go:49` (rota); `web/app/stores/auth.ts:84` (token em `useCookie`, legível por JS).
- **Problema:** o front monta o HTML escapando o texto (`useTaskComments.ts:111-116`), mas a API aceita `bodyHtml` arbitrário e o persiste em `tasks.task_comments.body_html` (`repository_postgres_collab.go:13-18`). A renderização injeta o HTML cru no DOM de quem abre a task.
- **Risco:** qualquer membro com `tasks.comment` envia via API `<img src=x onerror=...>`; o script roda no navegador de colegas e de admins que abrirem a task, lê o cookie `ldv_access_token` (não é HttpOnly) e assume a conta. Contra um `platform_admin` isso é comprometimento cross-tenant.
- **Correção:** (1) sanitizar no servidor com allowlist (ex.: `bluemonday` com `<p><br><a><strong><em><ul><ol><li>` e `rel="noopener nofollow"`) em `AddComment` e na edição; (2) sanitizar também no cliente antes do `v-html` (DOMPurify) por defesa em profundidade; (3) sanear os registros já gravados (migration de limpeza ou re-sanitização na leitura); (4) cookie de sessão HttpOnly/Secure (ver M-08); (5) adicionar CSP no front (hoje o CSP restritivo só existe na API, `httpapi/security_headers.go:18`).

- **Status: CORRIGIDO em 2026-11-23 (itens 1 e 3).** `tasks.Service.AddComment` sanitiza `bodyHtml` com `htmlsanitize.RichText` (bluemonday) antes de gravar e `ListComments` re-sanitiza na leitura, cobrindo linhas antigas; testes em `tasks/comment_sanitize_test.go`. Varredura de `v-html`: único outro uso (chat omnichannel) escapa o texto antes de montar o HTML. Pendentes: DOMPurify no cliente (item 2), cookie HttpOnly e CSP no front (itens 4 e 5, ver M-08).

### 3.2 Alto

#### A-01 — Rate limit global contornável, usável como DoS por tenant, e sem limite específico de login

- **Local:** `back/internal/platform/httpapi/rate_limit.go:176` (identidade = hash de bearer **não validado**), `:181-189` (usa o **primeiro** hop de `X-Forwarded-For`, forjável), `:203` (aceita `?access_token=`); `back/internal/platform/app/app.go:882-894` (middleware roda antes de `RequireAuth`; `AccountResolver` lê `X-Account-Id` do header); `back/internal/modules/auth/http.go:138` (login sem limiter próprio); `rate_limit.go:60` (um goroutine de limpeza por store, sem parada).
- **Problema:** (a) o `Resolver` por `principal.UserID` nunca resolve porque o principal ainda não está no contexto; cai no hash do token, que o atacante troca a cada requisição (token aleatório → bucket novo, memória cresce até a limpeza de 5 min); (b) sem token, a identidade é o primeiro IP de `X-Forwarded-For`, que o cliente controla (os módulos `cardapio/rate_limit.go:75-80` e `omnichannel/rate_limit.go:98-104` já usam o **último** hop e comentam exatamente esse risco); (c) a cota por tenant é incrementada por qualquer requisição **não autenticada** que envie `X-Account-Id: <uuid da vítima>`, esgotando `limit*5` (1500/min em produção, `config.go:244-249`) e devolvendo 429 a usuários legítimos.
- **Risco:** brute-force de senha/código de reset sem freio efetivo; negação de serviço seletiva contra um tenant; crescimento de memória.
- **Correção:** rodar a cota por tenant/usuário **depois** da autenticação (ou validar assinatura do token antes de usá-lo como chave); usar o último hop (ou `trusted proxies` configurável) em todo lugar; limiter específico e persistente (por e-mail + IP) em `/v1/auth/login` e `/v1/auth/password-reset/*` com backoff; limite no tamanho do mapa de buckets.

- **Status: CORRIGIDO em 2026-11-23.** (1) `httpapi/attempt_limiter.go` + `auth/attempt_guards.go`: login por e-mail normalizado (5 falhas/15 min) e por IP (20/15 min), bloqueio de 1 min dobrando a cada falha extra até 15 min, 429 genérico com `Retry-After` idêntico para e-mail existente ou não; sucesso zera só o e-mail; mesmo tratamento em `password-reset/request` (conta todo pedido), `password-reset/confirm` e convites (IP, token inexistente). (2) `httpapi/client_ip.go`: `X-Forwarded-For`/`X-Real-IP` só valem se a conexão TCP vier de proxy confiável (`TRUSTED_PROXIES`, padrão loopback e redes privadas), lendo o XFF da direita para a esquerda. (3) Rate limit global chaveia por usuário/tenant do token com **assinatura validada** (`Service.RateLimitIdentity`, sem banco); token inválido cai para IP; `X-Account-Id` deixou de ser usado, então requisição anônima não consome cota de tenant; teto de 100 mil buckets. (4) `Service.Login` roda o bcrypt contra hash fictício quando o e-mail não existe (igualar tempo). Testes: `httpapi/{client_ip,attempt_limiter,rate_limit}_test.go`, `auth/login_attempts_test.go`. Pendentes: estado em memória (reinicia com a API e não é compartilhado entre instâncias); o bloqueio por e-mail permite que terceiro trave temporariamente a conta-alvo (até 15 min); tempo de `password-reset/request` ainda difere entre e-mail existente e inexistente.

#### A-02 — `/uploads/` público, sem autenticação e com listagem de diretório

- **Status 2026-11-23 (parcial):** listagem desligada (diretório/arquivo oculto -> 404, `platform/app/uploads_handler.go`, testes de arquivo servido e traversal). `feedback/` (anexos sensíveis) segue acessível por URL com sufixo aleatório de 48 bits: autenticar exige o front buscar a imagem com Bearer (blob URL) em `FeedbackDetailPanel.vue`, `UserFeedbackWorkspace.vue` e `stores/feedback.ts` + checagem de autoria/conta no back; pendente.

- **Local:** `back/internal/platform/app/app.go:304-305` (`http.FileServer(http.Dir(cfg.UploadsDir))` em `GET /uploads/`); nomes: `auth/avatar_storage.go:47` (`<userID>-<rand>`), `queue/feedback/image_storage.go:67` (`<nome>-<rand>`).
- **Problema:** `http.FileServer` lista diretórios sem `index.html` (`/uploads/avatars/`, `/uploads/feedback/`) e serve qualquer arquivo sem checar sessão/tenant.
- **Risco:** enumeração de IDs de usuário (prefixo do avatar) e download de anexos de feedback (prints de telas internas) de qualquer tenant por quem acessar a URL; viola a regra de isolamento por tenant.
- **Correção:** envolver o `FileServer` em um handler que nega listagem (404 em diretório), restringe a prefixos públicos explícitos (`site/products`, `avatars`) e serve `feedback/` apenas autenticado e escopado por account; preferir R2 com URL assinada para anexos privados.

#### A-03 — Sem graceful shutdown; goroutines de background sem contexto cancelável e sem `recover`

- **Local:** `back/cmd/api/main.go:63` (`ListenAndServe` sem `signal.NotifyContext` / `Shutdown`); `back/internal/platform/app/app.go:161` (`go transcriptionsWorker.Run(context.Background())`), `:170` e `:193` (tickers de alertas e limpeza de anexos em `for { ...; <-ticker.C }`), `:254` (scheduler ERP), `:867` (`transcriptionsAnalysisWorker`); `platform/app/principal_cache_wiring.go:37` (loop eterno); `finance/module.go:95-99` (cria `stopRetention`, mas nada o chama no shutdown da API); demais `go func()` em `calendar/ai_dispatch.go:125`, `omnichannel/sla_scheduler.go:21`, `storage/staged_delivery.go:97`, `cardapio/telemetry_retention.go:21`, `customerintelligence/retention_scheduler.go:67`. Único `recover()` do projeto: `httpapi/middleware.go:127` (só protege goroutines de request).
- **Problema:** `SIGTERM` (deploy/`docker stop`) mata o processo no meio de requisições e de jobs (OCR, ERP sync, envio); um `panic` em qualquer goroutine de background derruba o processo inteiro (o `Recover` HTTP não cobre). O contexto `Background()` impede cancelar workers e fechar o pool de forma ordenada (`defer pool.Close()` em `main.go:39` nunca roda porque o programa termina por `os.Exit`/sinal).
- **Risco:** requisições 5xx a cada deploy; sync ERP e jobs interrompidos no meio (há `RecoverOrphanedSyncRuns`, `app.go:241`, que mascara o problema); queda total por um panic num worker.
- **Correção:** `ctx, stop := signal.NotifyContext(...)`; passar `ctx` aos workers; `httpServer.Shutdown(ctx)` com timeout (~25 s) e depois `wg.Wait()` dos workers; helper `safego.Go(ctx, name, fn)` com `recover` + log + métrica para todos os `go func`.
- **Status 2026-11-23 (corrigido nos pontos centrais):** `main.go` usa `signal.NotifyContext` (SIGTERM/SIGINT), `server.Serve` faz `Shutdown` com `SHUTDOWN_TIMEOUT` (padrão 20 s) e depois cancela o contexto raiz dos workers, roda os `Close` dos módulos e espera o `background.Group`, para `obs.Stop` e fecha o pool por último (logs `shutdown_started`/`shutdown_completed`). `platform/background` (`Go`/`Loop` com recover, stack no log e reinício com backoff) cobre transcrição (final e análise), monitor de alertas operacionais, limpeza de anexos de feedback, scheduler ERP, manutenção do cache de principals e alertas de observabilidade (por contexto). `platform/jobs` recupera panic por job (vira retry/dead pela política) e por ciclo, não pega job novo após cancelar e deixa o job em andamento terminar. Pendentes: goroutines internas de módulos que ainda não usam o grupo (calendar `ai_dispatch`, `storage` `staged_delivery`/`analytics_client`, `cardapio` retenção/backfill, `site/image_cache`, `bi`, `httpapi/rate_limit` cleanup, `realtime/presence`) — param pelos `Close` dos módulos ou morrem com o processo; sem métrica de panic ainda (só log `background_panic`).

#### A-04 — Timeouts do servidor incompatíveis com operações síncronas longas

- **Local:** `back/internal/platform/server/server.go:13-14` (`ReadTimeout` 15 s, `WriteTimeout` 30 s); `calendar/chat.go:39` (`chatAskTimeout = 120s`), `finance/document_pdf.go:22` (90 s), `finance/document_ocr.go:31` (35 s), `cardapio/media_storage.go:14` (vídeo até 60 MB).
- **Problema:** `WriteTimeout` fecha a escrita da resposta 30 s após o fim da leitura do header; o handler do chat espera o n8n até 120 s e só então escreve. Apenas `bio/sse.go:90`, `tasks/http_videos.go:68`, `calendar/http_media.go:170` e `storage/http.go:184` estendem o deadline com `ResponseController`; o chat, o PDF e o OCR não. `ReadTimeout` de 15 s também aborta upload de 60 MB em conexão móvel lenta.
- **Risco:** respostas perdidas (cliente vê erro/timeout mesmo com o trabalho concluído no servidor; consumo de tokens do LLM sem retorno); uploads grandes falhando de forma intermitente. **Verificar em runtime** o comportamento atrás do Caddy.
- **Correção:** `SetWriteDeadline`/`SetReadDeadline` por rota via `http.NewResponseController`, ou mover chat/PDF/OCR para jobs assíncronos com polling (já existe `platform/jobs`).

#### A-05 — Listagens sem paginação/limite em tabelas que crescem com o tenant

- **Local:** `back/internal/modules/omnichannel/store_postgres_admin.go:67` (`ListContacts`) e `:94` (`ListVisibleContacts`): `select` de **todos** os contatos da conta com `lateral join` da última conversa, sem `limit`; `calendar/store_postgres.go:143` (`ListEvents`: `From/To` opcionais → sem filtro retorna todos os eventos); `calendar/chat_store.go:177` (`ListConversations`); `cardapio/store_catalog.go:148` (`ListProductsFull`) e `:199` (`ListMenuProducts`); `omnichannel/store_media_analysis.go:81`; `queue/feedback/store_postgres.go:486` (`ListMessages`); `meta_ads/store_cache.go:138` (`ListInsights`). (~105 funções `List*` sem `limit`; a maioria é catálogo pequeno de configuração, os itens acima crescem sem teto.)
- **Problema:** resposta e memória proporcionais ao volume do tenant; `lateral join` por contato custa O(contatos) subconsultas a cada abertura do CRM.
- **Risco:** latência e uso de memória crescentes, pool de 10 conexões ocupado por consultas longas (ver M-09), efeito vizinho ruidoso.
- **Correção:** paginação por cursor (`created_at,id`) com `limit` máximo obrigatório; exigir intervalo em `ListEvents`; índice `(account_id, updated_at desc, created_at desc)` em `messaging.contacts` (confirmar com `EXPLAIN`).

#### A-06 — Dependências do front com vulnerabilidades altas/críticas

- **Local:** `web/package.json` / `web/package-lock.json`; CI trata como aviso: `.github/workflows/build-images.yml:92-99` (`continue-on-error: true`).
- **Evidência:** `npm audit --omit=dev --json` (2026-11-23): **6 críticas, 16 altas, 53 moderadas, 4 baixas (79)**. Críticas: `@nuxt/devtools`, `@simple-git/argv-parser`, `seroval`, `shell-quote`, `simple-git`, `tar`. Altas relevantes: `nuxt`, `vue`/`@vue/server-renderer` (≤3.5.41), `vite`, `@tiptap/core`, `prosemirror-view`, `devalue`, `postcss`, `nanoid`, `node-forge`, `js-yaml`, `svgo`. Todas com `fixAvailable` (atualização de `nuxt`/`@nuxt/ui` resolve a maior parte).
- **Risco:** várias são de toolchain/dev e não alcançáveis em runtime (SPA com `ssr:false`), mas `@tiptap/core`/`prosemirror-view`/`vue` estão no bundle do cliente; o aviso permanente no CI vira ruído e esconde regressões.
- **Correção:** atualizar Nuxt/Vue/Tiptap numa janela dedicada; tornar o audit bloqueante para `high` em dependências de runtime; Dependabot/Renovate. (`govulncheck` do backend **não foi executado** — ver §2.)

### 3.3 Médio

#### M-01 — Código de redefinição de senha gravado em claro no log quando SMTP não está configurado

- **Local:** `back/internal/modules/auth/password_reset_delivery.go:46-49` (fallback para `LogPasswordResetDelivery` quando `!config.Enabled()`), `:62-69` (loga `email` e `code`); `back/internal/platform/config/config.go:217` (`Validate` em produção não exige SMTP).
- **Risco:** em produção sem `SMTP_HOST` (default vazio, `docker-compose.prod.yml:144`) quem lê os logs redefine qualquer conta (inclusive admin) e o e-mail vira dado pessoal em log.
- **Correção:** em produção, `Validate()` deve falhar sem SMTP; o delivery de log só em `development`; nunca logar o código.

#### M-02 — Migrator sem lock, migrations não-concorrentes e prefixos duplicados

- **Local:** `back/internal/platform/database/migrator.go:151-176` (cada migration em `tx` sem `pg_advisory_lock`); 154 arquivos com `create index` e **0** com `concurrently` (`back/internal/platform/database/migrations/`); prefixos duplicados: `0019`, `0031`, `0039`, `0110`, `0197`, `0328` (`0328_tools_script_row_ids.sql` e `0328_tools_short_links_active.sql`).
- **Problema:** dois `migrate` simultâneos (deploy + restart) podem aplicar a mesma migration; índices criados em transação bloqueiam escrita da tabela durante o build; prefixo duplicado funciona (versão = nome completo, `migrator.go:70-74`, ordenação lexicográfica por `sort.Strings`), mas a ordem entre duplicados é implícita e confunde reviews/rollback.
- **Risco:** deadlock/falha de deploy; bloqueio de tabelas grandes (ex.: `messaging.messages`, `erp_*_raw`) em migrations futuras.
- **Correção:** `select pg_advisory_lock(<const>)` no início do `migrate`; suportar migrations marcadas "no-tx" para `create index concurrently`; CI que rejeita prefixo duplicado novo (as 6 existentes entram em allowlist).

#### M-03 — CI/CD sem gate em PR, sem lint/typecheck/vet e com ferramenta não fixada

- **Local:** `.github/workflows/build-images.yml:11-16` (só `workflow_dispatch` e `push` em `main`/`refactor/multitenant-complete`; **não há `pull_request`**), `:109-111` (`go run golang.org/x/vuln/cmd/govulncheck@latest`), `:92-99` (audit web não bloqueante); `deploy-vps.yml` é só `workflow_dispatch`. Não há `go vet`, `golangci-lint`, `eslint`, `vue-tsc` nem `go test -race` no CI (lint roda só em hooks Husky locais).
- **Risco:** código chega a `main` sem os checks; `@latest` é vetor de supply chain e quebra de build não determinística; hooks locais podem ser ignorados.
- **Correção:** trigger `pull_request`; jobs `go vet` + `golangci-lint` + `eslint` + `vue-tsc --noEmit` + `go test -race ./...`; fixar versão do `govulncheck`; branch protection exigindo o job.

#### M-04 — Erro de persistência ignorado em fluxo financeiro (originais de comprovantes)

- **Local:** `back/internal/modules/finance/document_originals.go:81` (`_, _ = s.store.pool.Exec(ctx, "insert into finance.document_originals ...")`); `finance/retention.go:162` (update `purge_attempts` ignorado); demais `_ =` com `Exec/Query`: `knowledge/ingestion_worker.go:139`, `queue/alerts/store_postgres_signals.go:172`.
- **Problema:** o arquivo já foi gravado no storage (`originals.Put`, linha 74) e a linha de controle pode não ser registrada; nenhum log/retorno.
- **Risco:** original órfão no R2 sem registro (nunca expira/purga, descumpre retenção) ou documento sem original; impossível diagnosticar.
- **Correção:** tratar o erro (log estruturado + métrica; remover o objeto órfão em caso de falha); revisar os 234 `_ =` fora de testes com foco em `Exec/Query`.

#### M-05 — Decodificação de JSON sem teto de tamanho em alguns handlers; `ReadJSON` trunca em vez de devolver 413

- **Local:** `back/internal/modules/core/rbac_http.go:126` e `:187` (`json.NewDecoder(r.Body).Decode` sem `MaxBytesReader`); `back/internal/platform/httpapi/json.go:30` (`io.LimitReader(r.Body, 1<<20)`: excedente vira erro de decode "unexpected EOF" → 400 genérico, não 413).
- **Risco:** corpo ilimitado em rotas administrativas autenticadas (`ReadTimeout` de 15 s limita, mas não o volume); mensagens de erro enganosas ao cliente. Os demais 39 módulos usam `ReadJSON` ou `MaxBytesReader` corretamente (ex.: `finance/fiscal_http.go:15`, `accounting/http.go:17`, `customerdata/http.go:61`).
- **Correção:** usar `httpapi.ReadJSON` nas duas rotas; trocar `LimitReader` por `http.MaxBytesReader` e mapear `*http.MaxBytesError` para 413.

#### M-06 — Pedido público do cardápio sem idempotência

- **Local:** `back/internal/modules/cardapio/http_public.go:28` (`POST /v1/public/restaurants/{slug}/orders`), `service_orders.go:23` (`PlaceOrder`), `store_orders.go:95-` (`CreateOrder` incrementa `last_order_number` e insere sempre).
- **Problema:** não há `Idempotency-Key` nem unicidade por `session_id`+payload; duplo clique/retry de rede cria pedidos e números duplicados.
- **Risco:** pedidos duplicados para o restaurante; o rate limit próprio (`cardapio/rate_limit.go`) não impede duplicidade legítima.
- **Correção:** chave de idempotência gerada no cliente (por carrinho) com `unique (restaurant_id, idempotency_key)` e retorno do pedido existente.

#### M-07 — FKs sem índice de apoio (amostra) em tabelas grandes com `ON DELETE CASCADE`

- **Local:** `back/internal/platform/database/migrations/0034_erp_ftp_foundation.sql:29,52-53,88-89,125-126,149-150,185-186,246-247` (`run_id`/`file_id` → `erp_sync_runs`/`erp_sync_files` com `on delete cascade`; nenhum índice em `run_id`/`file_id` encontrado em 0034/0058/0110/0179); `0239_messaging_channel_client_bindings.sql:14,201` (`client_account_id`, sem índice dedicado). Heurística global: de 1011 FKs declaradas, ~463 não têm índice cujo 1º campo seja a coluna (muitas são colunas de auditoria `created_by_user_id`, de baixo impacto).
- **Risco:** apagar um `erp_sync_run` ou conta faz seq scan nas tabelas `erp_*_raw` (milhões de linhas) e segura locks; `JOIN`s por `run_id` também ficam lentos.
- **Correção:** `create index concurrently` em `erp_*_raw(run_id)`, `(file_id)`; revisar com `pg_stat_user_tables`/`pg_constraint` em prod quais FKs de alta cardinalidade carecem de índice.

#### M-08 — Token de sessão em cookie legível por JS, sem `Secure`/`HttpOnly`, e aceito em query string

- **Local:** `web/app/stores/auth.ts:84-88` (`useCookie` com `sameSite:'lax'` apenas); `back/internal/platform/httpapi/rate_limit.go:203` e `back/internal/modules/realtime/tickets.go:129-150` (legado `?access_token=`/`?token=` em WebSocket, só com warning).
- **Risco:** amplifica C-01 e qualquer XSS futuro; token em URL vai para logs de proxy/histórico.
- **Correção:** emitir cookie `HttpOnly; Secure; SameSite=Lax` pelo backend (já existe o cookie do gateway SSO, `config.go` `AuthGatewayCookieDomain`) ou ao menos `secure:true`; remover o fallback por query após migrar os clientes de WS para tickets.

#### M-09 — Pool de conexões pequeno e compartilhado com muitos workers, sem métricas

- **Local:** `back/internal/platform/config/config.go:179` (`DATABASE_MAX_CONNS` default 10; `docker-compose.prod.yml:135` idem; `MinConns` 0); `platform/database/pool.go:49-59`.
- **Problema:** 10 conexões atendem requisições, ~10 loops de background (ERP, alertas, transcrição, retenção, SLA, jobs) e `pg_advisory_xact_lock` em transações (`storage/repository.go`, `customerdata`). Não há `AcquireTimeout` nem exposição de `pool.Stat()`; `healthz` só faz `Ping` (`app.go:309-322`).
- **Risco:** esgotamento do pool sob carga → timeouts em cascata sem sinal claro de causa.
- **Correção:** medir `pool.Stat()` (a instrumentação em `platform/observability` é o lugar), dimensionar (20-30 com Postgres de 1 GB é plausível) e separar um pool pequeno para jobs.

#### M-10 — Escritas em laço dentro de transação (round-trips N)

- **Local:** `back/internal/modules/queue/operations/store_postgres.go:535,562,617,634,656,736` (snapshot por item); `meta_ads/store_snapshots.go:131,189,217` (um `Exec` por conta/campanha/insight); `cardapio/store_catalog.go:366,378`; `planning/store_postgres.go:114,238-270`; `customerintelligence/store_headless.go:153,401,599-649`.
- **Problema:** não achei N+1 de **leitura** (nenhum `Query` dentro de `rows.Next()`); os casos acima são N inserts/updates sequenciais.
- **Risco:** latência linear no tamanho do lote (insights Meta podem ser milhares); transação aberta por mais tempo.
- **Correção:** `pgx.Batch`/`CopyFrom`/`insert ... select unnest($1::...)`.

#### M-11 — Acoplamento e composição concentrada

- **Local:** `back/internal/platform/app/app.go:67-910` (uma função `BuildHTTPHandler` de ~840 linhas; arquivo de 1042 linhas); `back/internal/modules/realtime` importa 9 módulos (`access auth calendar omnichannel queue stores tasks tenants tools`); `crm` importa `access erp queue stores`; `accounting`→`knowledge`, `omnichannel`→`knowledge`, `calendar`→`tasks`/`tenants`, `tasks`→`notifications`. Todos importam `auth` (aceitável como núcleo).
- **Levantamento de imports cruzados (produção, sem testes):** `access→auth`; `accounting→auth,knowledge`; `calendar→auth,tasks,tenants`; `cardapio→auth,core`; `crm→access,auth,erp,queue,stores`; `customerdata→auth,core`; `customerintelligence→auth,core`; `operationgoals→access,auth,stores`; `planning→access,auth,stores`; `queue→access,auth,stores,users`; `realtime→(9)`; `socialpublishing→auth,core`; `stores→access,auth`; `tasks→auth,notifications`; `tenants→access,auth`; `users→access,auth`; demais só `auth`. Os adapters em `platform/app/*_adapter.go` mostram o padrão correto (composition root injeta interface); o `realtime` é a exceção que vira "hub conhecedor de tudo".
- **Risco:** ciclos de import latentes, difícil testar módulo isolado, qualquer módulo novo mexe em `app.go` (conflito de merge e erro de wiring só em runtime).
- **Correção:** quebrar `BuildHTTPHandler` por módulo (`module.Register(deps)`), `realtime` consumir um barramento de eventos (`platform/events`) em vez de importar módulos; god-files a dividir: `bi/service.go` (1862), `calendar/chat.go` (1651), `customerintelligence/store.go` (1310).

#### M-12 — Gating de módulos é fail-open e depende de flag

- **Local:** `back/internal/platform/httpapi/account_guard.go:109-117` (rota fora da gate-list segue sem checar módulo); `back/internal/platform/app/app.go:900-904` e `config.go:207` (`CORE_V2_ENABLED` default `false`; `docker-compose.prod.yml:66` força `true` em prod).
- **Risco:** rota nova esquecida em `moduleGatingRules()` (`app.go:920`) fica acessível a contas sem o módulo contratado (a autorização por permissão continua valendo; é bypass de plano/licença, não de tenant).
- **Correção:** teste que enumera as rotas do `mux` e falha se alguma `/v1/<modulo>` não casar com regra ou allowlist explícita.

#### M-13 — Cliente HTTP do front sem timeout, retry nem cancelamento

- **Local:** `web/app/utils/api-client.ts` (dedupe de GET em voo nas linhas 77 e 305-311; sem `timeout`/`AbortController`/retry em todo o arquivo).
- **Risco:** requisição pendurada mantém spinner eterno e (com dedupe) todas as chamadas iguais esperam a mesma promessa travada.
- **Correção:** `timeout` padrão (ex.: 30 s, maior para rotas de IA) e retry com backoff apenas em GET idempotente; cancelar ao trocar de rota.

#### M-14 — Bundle: chunk de editor/emoji grande e ApexCharts em mais de um chunk (suspeita — verificar com build novo)

- **Local:** `web/.output/public/_nuxt/` (build de 2026-11-16): `CBWrHLfw.js` 1,16 MB (270 KB gzip; ProseMirror/Tiptap + emoji), `BS5dg3Wl.js` 605 KB e `CHmRg-9z.js` 518 KB (ambos contêm ApexCharts, ~167 KB e ~140 KB gzip), outros 4 chunks >400 KB; 359 arquivos. `web/app/components/omni/OmniEditor.vue:11-12` importa `@tiptap/extension-emoji` (com `gitHubEmojis`) estaticamente; `@emoji-mart/data` em `package.json`. `jspdf`/`html2canvas`/`quill`/`sortablejs` já são `import()` dinâmicos (`useScriptPdf.ts:7-8`, `ScriptQuillCell.vue:48-49`, `ScriptSortable.vue:12`); `qrcode` é estático (`app/utils/qr-image.ts:1`, pequeno).
- **Risco:** primeira carga das telas com editor (tasks, calendário, feedback) paga >1 MB; ApexCharts duplicado indica falta de chunk compartilhado (`manualChunks`).
- **Correção:** carregar o editor com `defineAsyncComponent`/`Lazy*` e o pacote de emojis sob demanda; `build.rollupOptions.output.manualChunks` para `apexcharts`; rodar `nuxi analyze` e fixar um orçamento de tamanho no CI.

#### M-15 — Backup: off-site opcional, sem criptografia nem teste de restauração automatizado

- **Local:** `scripts/backup/backup-db.sh:20,69-79` (rclone off-site só se `BACKUP_RCLONE_REMOTE` definido; `.sql.gz` sem criptografia; retenção local/remota por idade); `.github/workflows/backup-check.yml:11-12,38-60` (verifica o status do último backup e roda fallback; não restaura). Ponto único de falha: Postgres e API na mesma VPS (`docker-compose.prod.yml`, `mem_limit: 1g` no banco, linha 20).
- **Risco:** perda da VPS = perda de dados desde o último backup off-site (se configurado); backup contém PII/segredos cifrados em texto aberto; backup nunca testado pode não restaurar.
- **Correção:** tornar o off-site obrigatório em produção; `age`/`gpg` antes do upload; job mensal de restore em Postgres descartável com checagem de contagem de linhas; considerar `pgBackRest`/WAL archiving para RPO < 24 h.

#### M-16 — Clientes HTTP de saída sem timeout próprio (dependem só do contexto)

- **Local:** `back/internal/modules/calendar/chat.go:445` (`&http.Client{}`) e `:460` (`http.DefaultClient`); `omnichannel/channel/meta_whatsapp/provider.go:245,343` e `channel/instagram/provider.go:259,313` (`client = http.DefaultClient` quando `p.http == nil`; o construtor padrão usa `Timeout: 30s`, `provider.go:44`, mas o fallback não); `omnichannel/ai_models.go:86` (`http.DefaultClient.Do` — mitigado por `WithTimeout(10s)` na linha 65).
- **Risco:** se algum chamador passar `context.Background()` (35 usos fora de testes), a chamada fica pendurada indefinidamente. Os demais clientes (`storage/analytics_client.go:29`, `site/perola_client.go:74`, `socialpublishing/provider_instagram.go:40`, `bi/perola_client.go`) têm `Timeout`.
- **Correção:** construtor único `httpx.NewClient(timeout)` + lint que proíbe `http.DefaultClient`; retries com backoff e jitter nas integrações Meta/Evolution/R2 (hoje não há retry genérico fora de `platform/jobs`).

#### M-17 — Repositórios fail-open quando `accountID` vem vazio (defesa em profundidade)

- **Local:** `back/internal/modules/tools/store_qr_codes.go:141-148,218-222`, `tools/store_short_links.go:185,266`; `calendar/store_postgres.go:179-183,243-250,255-258`; `bio/store_postgres.go:173,187,199,207` (`Patch/Publish/Unpublish` atualizam **só por id**, a segurança vem do `GetByID` com escopo em `bio/service.go:238,267,298`); `stores/store_postgres.go:153` e `operationgoals/store_postgres.go:182` (`update ... where id = $1` com pré-checagem no service).
- **Problema:** hoje seguro (platform_admin é o único a passar vazio, `tools/http.go:60-79`; os services pré-checam), porém um novo caller que esqueça o escopo ou passe `""` por engano opera cross-tenant sem erro, contrariando a regra "tenant nunca opcional".
- **Correção:** separar métodos `...AsAdmin` explícitos e tornar `accountID` obrigatório nos demais; incluir `and account_id = $n` nos UPDATE/DELETE por id; teste de contrato cross-tenant por repositório.

### 3.4 Baixo

#### B-01 — `403 account_not_member` em vez de 404

- **Status 2026-11-23: corrigido.** `RequireAuthWithAccount` agora responde `404 not_found` para conta alheia; testes em `auth/middleware_account_test.go` e `finance/http_access_test.go`.

- `back/internal/modules/auth/middleware.go:108`. A regra da casa é 404 fora do escopo; 403 confirma que o UUID de account existe. Risco baixo (UUID não adivinhável). Trocar por 404 ou resposta neutra.

#### B-02 — Download de imagens sem guarda SSRF (suspeita — verificar)

- `back/internal/modules/site/image_cache.go:162` baixa qualquer `http(s)://` presente nos itens sincronizados (`image_cache.go:118-126`), sem a validação de IP privado usada em `omnichannel/ssrf.go`. As URLs vêm da API Perola cuja `base_url` é restrita a duas constantes (`site/service.go:262-264`, `model.go:188-189`), portanto hoje só é explorável se a Perola for comprometida. Aplicar o mesmo dialer com bloqueio de IP privado/metadata.

#### B-03 — Acessibilidade: botões só com ícone sem nome acessível

- Varredura em `web/app` e `web/layers`: **~37** `<UButton ... icon=... />` sem `label`/`aria-label`/`title` (de 310 `UButton` auto-fechados). Exemplos: `web/app/components/manager/clients/ClientsStoresPopover.vue:210` (botão "+"), `omnichannel/inbox/InboxMessageActionMenu.vue:48,52`, `omnichannel/inbox/InboxChatFooterStatus.vue:87,133,137,156`, `omnichannel/inbox/InboxChatComposerAttachmentMenu.vue:83`, `omnichannel/inbox/InboxAudioMessagePlayer.vue:227`. `<img>` sem `alt`: **0** reais (50 `<img>`; o único hit é comentário em `bio/BioMediaField.vue:8`). 1271 ocorrências de `button`/`UButton` no total (heurística por regex; falsos positivos possíveis com `v-bind`).
- Correção: regra ESLint `vuejs-accessibility/accessible-emoji` + `aria-label` obrigatório em `UButton` sem `label`.

#### B-04 — Waterfalls de `await` em `onMounted`

- `web/app/pages/operacao/index.vue:80-88` (`ensureSession` → `loadOperationView` → `loadOperableStoreSnapshot` em série), `web/app/components/cardapio/CardapioListWorkspace.vue:357-363` (`ensureSession` → `ensureLoaded` → `refresh`), `web/layers/tasks/pages/editor.vue:31-44`. Só 4 casos com ≥3 `await` sequenciais (heurística). `loadOperationView` e `loadOperableStoreSnapshot` parecem independentes: `Promise.all` após `ensureSession` economiza uma ida e volta.

#### B-05 — Artefatos soltos na raiz do repositório

- 15 arquivos `*.png`/`*.html` versionados na raiz (`mostarda.png` ≈ 1 MB, `themes-smoke.png`, `pix-mvp.html`, `pix-fixo.html` …) e `.DS_Store` rastreado (já staged como `D`). Mover para `docs/` ou remover; `.gitignore` para `*.png` na raiz.

#### B-06 — Defaults inseguros que `Validate()` não barra em produção

- `back/internal/platform/config/config.go:205` (`AUTH_CONSULTANT_DEFAULT_PASSWORD` default `Omni@123`; compose de prod exige a variável, `docker-compose.prod.yml:155`, mas `Validate` não rejeita o default nem senha fraca); `:13` (`productionMinBcrypt = 10`, custo baixo para 2026; OWASP recomenda ≥12 com bcrypt). Adicionar à `Validate`: senha default proibida e custo ≥ 12 (medir latência de login).

#### B-07 — `healthz` único para liveness e readiness, e expõe a lista de módulos

- `back/internal/platform/app/app.go:309-335`. Um 503 por indisponibilidade breve do banco faz o orquestrador reciclar a API. Separar `/livez` (processo) e `/readyz` (DB + migrations aplicadas); retirar a lista de módulos da resposta pública.

#### B-08 — Cache de principal de 30 s atrasa revogação

- `back/internal/platform/config/config.go:193` (`AUTH_PRINCIPAL_CACHE_TTL` 30 s) + `auth/service.go:152-156`: logout/desativação/mudança de permissão só valem após o TTL, mitigado por invalidação explícita (`principal_cache_wiring.go:33-34`). Aceitável; documentar e invalidar também no logout e na troca de senha.

## 4. Áreas auditadas sem achado relevante

- **Autorização de módulos amostrados:** finance (`store_sheet_mutations.go:125-145,224-236`: `account_id` no `where` e `lockSheetForWrite`), customerdata (`repository_matching_merge.go:101-105` + `resourceScope` no service), accounting (escopo + `Idempotency-Key` em `http.go:133,161,233,250,259`), omnichannel (todas as ~50 consultas por id amostradas filtram `account_id`), meta_ads (guardas de ação/lease e `scope_integration_test.go`), tasks, cardapio (pedido vincula `restaurant_id`/`account_id` na transação, `store_orders.go:95-150`).
- **Headers/CORS:** `httpapi/security_headers.go` (nosniff, DENY, CSP `default-src 'none'`, HSTS em produção); CORS por allowlist, `*` só em `/v1/public/*` sem credenciais (`middleware.go:137-151`).
- **Uploads:** todos os `io.ReadAll` de corpo usam `LimitReader`/`MaxBytesReader` (exceto `bio/http.go:245`: `io.ReadAll(file)` após `ParseMultipartForm` com teto — verificar o limite do formulário) .
- **Idempotência em envios:** omnichannel (envio WhatsApp), meta_ads (ações de gasto), socialpublishing, storage multipart.
- **Concorrência de OCR/PDF:** `finance/document_service.go:20-24` limita a 2 slots simultâneos.

## 5. Observabilidade, confiabilidade e testes

- **Logs:** no HEAD, `slog.NewTextHandler` sem `request_id` nas linhas de módulos (só no middleware `Logging`, `httpapi/middleware.go:84-97`); o working tree já troca por `observability.NewLogger` (JSON + correlação, `main.go:23`) — **concluir e commitar**. `Logging` não registra IP/usuário/account (útil para A-01).
- **Dados sensíveis em logs:** além de M-01, não encontrei `slog` com e-mail/telefone/CPF/token em módulos (grep de `email|phone|cpf|token|password|authorization` perto de chamadas `slog`/`logger`); `realtime/tickets.go:130` loga só o nome do parâmetro. `observability/sanitize.go` novo cobre atributos — validar com teste.
- **Health:** ver B-07. Sem métricas RED nem de pool no HEAD (a instrumentação em curso cobre; falta `Stop` ordenado, ver A-03 — `obs.Start`/`Stop` em `main.go:41`).
- **Backup:** M-15. **Deploy:** manual via `workflow_dispatch` com backup opcional ("obrigatório em release de schema" apenas como descrição do input, `deploy-vps.yml:37`); sem rollback automatizado de migration.
- **Testes (contagem):** **358** arquivos `*_test.go` e **157** `*.test.ts` (web). Módulos Go sem nenhum teste: `notifications` (11 arquivos, 1252 linhas), `operationgoals` (5, 1080). Com teste proporcionalmente fraco: `tenants` (1 teste / 6 arquivos), `roadmap` (1/6), `contentoperations` (1/5), `access` (2/6), `stores` (2/7), `users` (2/7), `tools` (5/15), `auth` (5/19 — módulo mais crítico para segurança; conferir se login, reset e lockout têm teste), `core` (5/50), `automation` (2/26), `bi` (4/10). Fortes: `omnichannel` (69/183), `queue` (29/106), `finance` (31/72), `meta_ads` (21/41), `customerintelligence` (20/41). Cobertura percentual **não medida**; CI roda `go test ./...` sem `-race` nem `-cover` (`build-images.yml:103`). Sem teste end-to-end do front (apenas Vitest).
- **Código morto/duplicação (indícios):** `docs/` com ~10 arquivos `CALENDARIO_SPECS*.md` e relatórios HTML versionados (596 KB `docs/relatorios/2026-07/index.html`); três arquivos de instrução (`AGENT.md`, `AGENTS.md`, `AGENT_RULES.md`) na raiz; `erp-source-local/` com 0 B em disco e `meta-ads-assistant/` (26 arquivos rastreados, fora do pipeline de build); `app_role_grants.go` com `fmt.Sprintf` de DDL (seguro, identificadores citados). Não rodei análise de símbolos não usados (`deadcode`/`staticcheck U1000`) — recomendado.

## 6. Top 10 de melhorias (impacto × esforço)

| #   | Melhoria                                                                                                                                    | Impacto | Esforço | Achados          |
| --- | ------------------------------------------------------------------------------------------------------------------------------------------- | ------- | ------- | ---------------- |
| 1   | Sanitizar `bodyHtml` de comentários (servidor + `v-html`) e migrar o token para cookie HttpOnly/Secure                                      | Crítico | Baixo   | C-01, M-08       |
| 2   | Proteger `/uploads/` (sem listagem, `feedback/` autenticado e escopado)                                                                     | Alto    | Baixo   | A-02             |
| 3   | Graceful shutdown + `signal.NotifyContext` + helper `safego` com `recover` para todos os workers                                            | Alto    | Médio   | A-03, M-09       |
| 4   | Reescrever o rate limit: último hop/proxies confiáveis, cota por tenant pós-auth, limiter de login/reset persistente                        | Alto    | Médio   | A-01, M-01       |
| 5   | Paginação por cursor nas listagens de contatos/eventos/produtos + índices de suporte                                                        | Alto    | Médio   | A-05, M-07, M-10 |
| 6   | Ajustar timeouts por rota (`ResponseController`) ou mover chat/PDF/OCR para jobs assíncronos                                                | Alto    | Médio   | A-04, M-16       |
| 7   | CI com `pull_request`, `go vet`/`golangci-lint`/`eslint`/`vue-tsc`/`-race`, `govulncheck` fixado e audit bloqueante                         | Alto    | Baixo   | M-03, A-06       |
| 8   | Atualizar Nuxt/Vue/Tiptap e zerar críticas do `npm audit`                                                                                   | Alto    | Médio   | A-06             |
| 9   | Migrator com `pg_advisory_lock`, suporte a `concurrently` e checagem de prefixo duplicado; índices nas FKs `erp_*_raw`                      | Médio   | Baixo   | M-02, M-07       |
| 10  | Backup off-site obrigatório e criptografado + restore mensal automatizado; `Validate()` de produção exigir SMTP e barrar defaults inseguros | Médio   | Médio   | M-15, M-01, B-06 |

**Sequência sugerida:** 1, 2, 7 e 9 cabem em uma semana e removem os maiores riscos de menor custo; 3, 4 e 6 formam uma segunda leva de robustez operacional; 5 e 8 exigem medição (`EXPLAIN`, `nuxi analyze`) antes de fechar o escopo.

## 7. Top 10 revisado com números reais (primeira execução, 2026-11-23)

Execução `20261123-022153` (`npm run obs:run`, ambiente local, banco cópia de produção,
**sem credencial de teste**). Ela cobre só:

- rotas públicas;
- testes negativos de autorização;
- banco (checagens estruturais e queries capturadas);
- saúde e dependências.

Endpoints autenticados e páginas logadas ficam para a próxima execução, quando as
credenciais existirem em `observability/.env.local`.

| medida                                         | valor                                                                                                                                                                                           |
| ---------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Endpoints no inventário                        | 919 (491 `auth_account`, 341 `auth`, 25 `role`, 26 públicos, 36 com auth dentro do handler)                                                                                                     |
| Rejeição sem token                             | 883 endpoints testados, **0 responderam 2xx sem token** (695 rejeitados com 400 `missing_account_id` pelo guard de módulo antes da auth, demais com 401, `gateway/verify` com 302 para o login) |
| Banco                                          | 396 relações; **428 FKs sem índice**, 12 grupos de índices duplicados, 40 índices nunca usados; `pg_stat_statements` não instalado                                                              |
| Consultas em segundo plano                     | ~15 consultas/s com o sistema ocioso: cada fila de outbox/job faz 1 claim/s (intervalo padrão de `platform/jobs`)                                                                               |
| Consultas lentas (> 200 ms) e suspeitas de N+1 | nenhuma no tráfego medido (sem tráfego autenticado ainda)                                                                                                                                       |
| Dependências Go                                | **29 vulnerabilidades alcançáveis** (`govulncheck`)                                                                                                                                             |
| Dependências web                               | `npm audit --omit=dev`: **6 críticas, 16 altas**                                                                                                                                                |
| Páginas públicas (`nuxt dev`)                  | LCP 5,4 s em `/auth/login`; `/offline` com TTFB de 10 s. Medir de novo no build de produção antes de concluir                                                                                   |
| Saúde                                          | API, web, Postgres e 9 serviços auxiliares saudáveis                                                                                                                                            |

| #   | melhoria                                                                                                                                         | impacto | esforço | base                        |
| --- | ------------------------------------------------------------------------------------------------------------------------------------------------ | ------- | ------- | --------------------------- |
| 1   | Sanitizar `bodyHtml` de comentários no servidor e no `v-html`; token em cookie HttpOnly                                                          | Crítico | Baixo   | C-01 (confirmado no código) |
| 2   | Atualizar a toolchain/dependências Go até zerar as 29 vulnerabilidades alcançáveis; `govulncheck` bloqueante no CI                               | Alto    | Baixo   | medido                      |
| 3   | Zerar as 6 críticas e 16 altas do `npm audit` do web (Nuxt/Vue/Tiptap)                                                                           | Alto    | Médio   | medido, A-06                |
| 4   | Proteger `/uploads/` (sem listagem; anexos de feedback autenticados e escopados)                                                                 | Alto    | Baixo   | A-02                        |
| 5   | Rate limit por identidade autenticada, último hop confiável e limiter de login                                                                   | Alto    | Médio   | A-01                        |
| 6   | Graceful shutdown e `recover` nos workers em segundo plano                                                                                       | Alto    | Médio   | A-03                        |
| 7   | Índices nas FKs de hot path: começar pelas colunas `account_id`/`*_id` usadas em `WHERE`/`JOIN` (das 428 sem índice), criados com `concurrently` | Médio   | Baixo   | medido, M-07                |
| 8   | Timeouts por rota ou jobs assíncronos para chat de IA, PDF e OCR                                                                                 | Alto    | Médio   | A-04                        |
| 9   | Paginação por cursor nas listagens sem limite                                                                                                    | Alto    | Médio   | A-05                        |
| 10  | CI em `pull_request` com lint, `vue-tsc`, `-race` e `obs:inventory --check`                                                                      | Médio   | Baixo   | M-03                        |

O que sai do Top 10 anterior:

- **Backup:** continua no item M-15.
- **Locks do migrator:** continuam no item M-02.

O que mudou de lugar:

- **Dependências vulneráveis subiram:** os números medidos mostram superfície concreta.
- **Rejeição sem token:** a execução confirmou que nenhuma rota com wrapper de auth
  responde sem token. Isso não cobre as 36 rotas com auth dentro do handler
  (WebSocket, `/internal`, `/v1/runtime`), que ficam para revisão manual.
- **Polling em segundo plano:** a medição mostrou ~15 consultas/s ociosas, aceitável.
  Só entra se o banco virar gargalo; nesse caso, o caminho é LISTEN/NOTIFY ou backoff
  quando a fila está vazia.

## 8. Execução autenticada (2026-11-23)

Execuções `20261123-081550` e `20261123-082204` (`npm run obs:run -- --load`, perfil
ADMIN, banco cópia de produção, escrita desligada). A `081550` é a referência de
latência. Na `082204` uma compilação Go rodou em paralelo e subiu o p95 de oito GETs
de ~12 ms para 60–120 ms. Uma execução isolada do módulo finance (`082818`) confirmou
a volta a 11–16 ms.

| medida                                          | valor                                                                                                                                                                                                                                                                                            |
| ----------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Resultados                                      | 2397: 1050 pass, 497 warn, 1 fail, 849 skip                                                                                                                                                                                                                                                      |
| Cobertura de endpoints                          | 125/919 (13,6%), ou 125 de 393 GETs (32%)                                                                                                                                                                                                                                                        |
| Endpoints não testados                          | 526 de escrita (banco é cópia de produção), 206 GETs com integração externa (`OBS_SKIP_EXTERNAL=0` libera), 52 sem valor para parâmetro de caminho, 36 com auth dentro do handler, 18 em `neverCall`                                                                                             |
| Cobertura de páginas e tabelas                  | páginas 72/80 (90%; as 8 restantes têm parâmetro dinâmico); tabelas 384/396 (97%)                                                                                                                                                                                                                |
| Autorização                                     | 857 testes sem token rejeitados, 0 falhas. Teste entre perfis não executado: credencial de consultor vazia                                                                                                                                                                                       |
| Latência da API                                 | todo GET 2xx com p95 abaixo de 40 ms local. Mais pesados: o próprio painel (payload, corrigido abaixo), `/v1/analytics/data` (30 ms, 10 queries/req), `/v1/performance-feedback/context` (17 queries/req), `/v1/settings` (15 queries/req), `/v1/task-boards/{boardId}` (10 queries/req)         |
| Consultas lentas (tráfego real das últimas 6 h) | `GET /v1/operations/snapshot`: média 1,2 s, máximo 2,5 s (31 ocorrências), CTE `known_orders` em `back/internal/modules/crm/erp/repository_crm_scope.go:172`. `GET /v1/erp/crm`: média 408 ms, máximo 979 ms (mesma CTE). `GET /v1/erp/status` 392 ms; `GET /v1/content-operations/brief` 402 ms |
| N+1                                             | 1 suspeita: `GET /v1/omnichannel/tenant/whatsapp/instances/access` repete 10 vezes por requisição a checagem de permissão (`core.user_role_assignments`)                                                                                                                                         |
| Páginas (`nuxt dev`)                            | `/tasks`: LCP 6,0–6,6 s, INP 416–848 ms, carga 7,3 s. `/calendario/config`: LCP 3,3 s. Demais abaixo de 2,7 s. Entre execuções o `nuxt dev` varia até 2x (`/feedback-desempenho` caiu de 9,7 s para 0,8 s depois da primeira compilação). Confirmar no build de produção                         |
| Carga leve                                      | 5 endpoints, 10 conexões, 18 req/s por 10 s: 0 erros, p99 até 82 ms                                                                                                                                                                                                                              |
| Erros 5xx                                       | `/tasks` recebeu 503 ao abrir um vídeo: o storage R2 não está disponível no ambiente local (limitação do ambiente). Os demais 500 vieram do fim da janela de carga: o cliente fecha a conexão e o handler responde 500 ao contexto cancelado                                                     |

Correções feitas a partir desta execução:

- **Painel:** o `/overview` não entregava `modules` e mandava as notas em formato
  cru, então a página mostrava "Nenhuma nota ainda". Agora ele segue o `SPEC.md` e
  usa só execuções completas. O payload caiu de 411 KB para 10 KB; a lista de
  execuções caiu de 284 KB para 7 KB. O detalhe de uma execução (`runs/{id}`, 785 KB)
  continua completo.
- **Telemetria:** um 5xx com o contexto cancelado agora é gravado como 499
  `client_closed_request` e não conta como erro do servidor.
- **Runner:**
  - um GET que responde 400 é repetido uma vez com os filtros que o front envia
    (tenant e loja ativos, período), lidos de `/v1/me/context`. Isso levou 15
    endpoints de 400 para 200 com dado real;
  - a comparação ignora métricas quando o status muda de faixa (ex.: 400 para 200).

### Top 10 revisado (execução autenticada)

| #   | melhoria                                                                                                                                            | impacto | esforço | base         |
| --- | --------------------------------------------------------------------------------------------------------------------------------------------------- | ------- | ------- | ------------ |
| 1   | Sanitizar `bodyHtml` de comentários no servidor e no `v-html`; token em cookie HttpOnly                                                             | Crítico | Baixo   | C-01         |
| 2   | Zerar as 29 vulnerabilidades Go alcançáveis; `govulncheck` bloqueante no CI                                                                         | Alto    | Baixo   | medido. 2026-11-23: 29 de 29 corrigidas (re-medido: 22 alcançáveis na data, 0 após toolchain go1.26.11 + x/net 0.60.0, x/crypto 0.57.0, x/text 0.42.0, aws s3 1.97.3); sem pendências; `govulncheck` bloqueante em `ci-race.yml` |
| 3   | Zerar as 6 críticas e 16 altas do `npm audit` do web (feito em 2026-11-23: 6 críticas e 16 altas para 0 críticas e 0 altas; restam 10 moderadas e 1 baixa que exigem major de vue-tsc/quill ou dependem do Nitro/Nuxt; nuxt 4.6.4, @nuxt/ui 4.13.1, Tiptap 3.35)                                                                                                | Alto    | Médio   | medido, A-06 |
| 4   | Reescrever a CTE `known_orders` (`operations/snapshot` 1,2–2,5 s, `erp/crm` até 1 s): índice em `order_id`/`store_id_raw` ou agregado materializado | Alto    | Médio   | medido Feito: predicados crus no filtro da CTE usam o índice parcial existente (sem migration); 1,7–3,0 s para 0,19–0,42 s (EXPLAIN ANALYZE, tenant/loja reais, 777 mil linhas), resultado idêntico |
| 5   | Proteger `/uploads/` (sem listagem; anexos autenticados e escopados)                                                                                | Alto    | Baixo   | A-02         |
| 6   | Rate limit por identidade autenticada e limiter de login                                                                                            | Alto    | Médio   | A-01         |
| 7   | `/tasks`: medir no build de produção; se o INP > 200 ms e o LCP > 2,5 s se confirmarem, virtualizar o quadro e adiar o carregamento dos vídeos      | Médio   | Médio   | medido (dev) |
| 8   | Graceful shutdown e `recover` nos workers em segundo plano                                                                                          | Alto    | Médio   | A-03         |
| 9   | Índices nas FKs de hot path (das 428 sem índice), com `concurrently`                                                                                | Médio   | Baixo   | medido, M-07 |
| 10  | Timeouts por rota ou jobs assíncronos para chat de IA, PDF e OCR                                                                                    | Alto    | Médio   | A-04         |

Saem do Top 10:

- **Paginação por cursor (A-05):** nenhuma listagem medida passou de 200 KB nem de
  40 ms.
- **CI em `pull_request` (M-03):** continua recomendado.
- **N+1 de permissão no WhatsApp:** fica como item médio (10 queries de ~1 ms).
