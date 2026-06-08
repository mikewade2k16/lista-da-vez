# Arquitetura - Melhorias recomendadas em 2026-06-07

> Este documento e a segunda parte da analise: o que melhorar a partir da fotografia em `docs/ARQUITETURA_ESTADO_ATUAL_2026-06-07.md`.
> Prioridade aqui nao e "fazer tudo". E separar o que protege o produto agora, o que prepara escala, e o que deixa a base pronta para virar uma plataforma grande.

## Direcao geral

A base ja tem uma boa separacao por dominio e um caminho claro para multi-tenant. O principal trabalho agora e fechar as frestas entre contratos antigos e novos:

- account vs tenant;
- Core V2 vs rotas legadas;
- catalogo de modulos vs rotas realmente registradas;
- token assinado vs sessao revogavel;
- frontend mostrando modulo vs backend aceitando modulo;
- isolamento por aplicacao vs isolamento tambem no banco.

## P0 - Corrigir antes de crescer

### 1. Remover senha do `localStorage`

Problema:

- `web/app/stores/auth.ts` salva email e senha em `ldv_remembered_login`.
- Qualquer XSS, extensao maliciosa ou script de terceiro conseguiria ler senha em texto claro.

Acao:

- Manter no maximo email lembrado.
- Se quiser experiencia de "lembrar-me", usar sessao refresh/revocable ou deixar o browser/password manager cuidar.
- Migrar limpando entradas antigas de `ldv_remembered_login`.

Aceite:

- `localStorage` nunca guarda senha, token ou segredo.
- Teste garante que `saveRememberedLogin` nao persiste `password`.

### 2. Ligar sessoes reais no backend

Problema:

- `core.user_sessions` e `PostgresSessionRepository` existem.
- `auth.Service` tem `SetSessionRepository` e `SetPrincipalCache`.
- No bootstrap atual, nao vi esses setters sendo chamados.
- Logout tende a ser apenas limpeza client-side/idempotente, sem revogacao real do token ja emitido.

Acao:

- Em `app.go`, criar `auth.NewPostgresSessionRepository(pool)`.
- Chamar `authService.SetSessionRepository(...)`.
- Criar `PrincipalCache` com TTL curto e invalidacao.
- Garantir que login emite claim `sid`.
- Fazer logout revogar `sid`.
- Invalidar cache em logout, troca de role, desativacao de user e mudanca de membership.

Aceite:

- Login cria linha em `core.user_sessions`.
- Logout preenche `revoked_at`.
- Token antigo com `sid` revogado passa a retornar 401.
- Teste cobre revogacao e cache.

### 3. Alinhar `X-Account-Id`

Problema:

- O header e `X-Account-Id`.
- O backend Core V2 valida membership por account.
- O provider global do front usa `auth.activeTenantId`.
- O store novo `useCoreAccountStore` ja tem `activeAccountId`.

Acao:

- Fazer o provider global ler `useCoreAccountStore().activeAccountId`.
- Manter fallback temporario para `auth.activeTenantId` apenas se necessario e com comentario de remocao.
- Renomear variaveis de UI onde "tenant" agora significa "account".
- Padronizar cookie: `ldv_active_account_id` deve ser a fonte de account ativa; `ldv_active_store_id` continua loja ativa.

Aceite:

- Trocar account no switcher muda `X-Account-Id` imediatamente.
- Requests multi-tenant usam account ativa correta.
- Teste de front cobre switch de account e header.

### 4. Fechar drift de modulos registrados

Problema:

- Banco/catalogo contem `site` e `roadmap`.
- Front guarda rotas de `site`.
- Modulos Go `site`, `roadmap` e `operationgoals` existem.
- No bootstrap atual, nao vi esses modulos registrados em `app.go`.

Acao:

- Decidir por modulo:
  - registrar de verdade no Core V2;
  - ou esconder/remover do catalogo/front temporariamente.
- Para `site`, se vai existir no painel, registrar `site.New()` no registry e confirmar `RequireModuleByPath` no backend.
- Para `roadmap`, decidir se e core/sempre aberto ou modulo contratavel.
- Para `operationgoals`, registrar rotas ou integrar dentro de `queue`.

Aceite:

- Toda rota visivel no menu tem endpoint correspondente ativo.
- Toda rota backend sensivel tem guard coerente.
- `core.modules` reflete somente modulos vivos ou planejados explicitamente.

### 5. Corrigir dedupe de GET no frontend

Problema:

- `api-client.ts` deduplica GET por `baseURL`, `path` e headers.
- Se duas chamadas usam o mesmo path mas `options.query` diferente, podem compartilhar promise errada.

Acao:

- Incluir metodo, path normalizado, query, params e body relevante na chave.
- Ou limitar dedupe apenas a paths string ja completos com query.
- Adicionar teste unitario.

Aceite:

- GET `/v1/tasks?board=a` e `/v1/tasks?board=b` nunca compartilham resposta.

### 6. Alinhar defaults de producao

Problema:

- Dev usa `CORE_V2_ENABLED=true` e `AUTH_ROLES_SOURCE=core`.
- Prod compose usa `CORE_V2_ENABLED=false` e `AUTH_ROLES_SOURCE=core_with_fallback`.
- Migrations recentes removeram tabelas legadas de roles.

Acao:

- Revisar `docker-compose.prod.yml` e `.env.production.example`.
- Padronizar `CORE_V2_ENABLED=true` se essa ja e a arquitetura alvo.
- Padronizar `AUTH_ROLES_SOURCE=core`.
- Rodar smoke de login, roles, modules e account switch em ambiente limpo.

Aceite:

- Ambiente novo sobe sem depender de tabela legada.
- Prod e dev compartilham o mesmo caminho de auth/RBAC.

## P1 - Seguranca multi-tenant de plataforma

### 7. Introduzir RLS por etapas

Problema:

- O banco hoje nao tem Row-Level Security.
- Qualquer handler/repository novo que esquece filtro por account pode vazar dados.

Acao sugerida:

1. Mapear tabelas tenant/account-scoped: `queue.*`, `tasks.*`, `site.*`, `notifications.*`, `roadmap.*` e tabelas `public` residuais.
2. Criar middleware/transaction helper que executa `SET LOCAL app.account_id = '<uuid>'`.
3. Comecar por tabelas menos arriscadas, como `site.*` ou `notifications.*`.
4. Depois avancar para hot paths de `queue` e `tasks`.
5. Definir bypass controlado para `platform_admin`.

Aceite:

- Query sem `app.account_id` nao retorna dados cross-account.
- Teste de integracao prova que um account nao le dado de outro mesmo se o filtro do repository for removido em fixture controlada.

### 8. Padronizar erro cross-tenant

Problema:

- `403` em recurso de outro tenant pode revelar que o recurso existe.

Acao:

- Diferenciar "sem permissao para acao" de "fora do escopo".
- Fora do escopo deve virar `404 not_found` uniforme.
- Manter `403` para permissao/RBAC insuficiente dentro da account correta.

Aceite:

- ID inexistente e ID de outra account respondem igual.

### 9. Tirar token da query string do WebSocket

Problema:

- Query string tende a aparecer em logs, ferramentas de proxy, APM e historico de request.

Acao:

- Preferir autenticacao por cookie `httpOnly` com BFF/Nitro, ou protocolo de subprotocol/header quando possivel.
- Enquanto WebSocket nativo do browser nao permite header custom facil, usar ticket efemero:
  - client chama `/v1/realtime/ticket`;
  - backend emite ticket curto, one-time, por account/store;
  - socket usa `?ticket=...`, nao bearer principal.

Aceite:

- URL do WebSocket nao carrega access token reutilizavel.
- Ticket expira rapido e e vinculado a user/account/store.

### 10. Security headers e CSRF strategy

Acao:

- Adicionar middleware de security headers: HSTS em prod, `X-Content-Type-Options`, `X-Frame-Options`/`frame-ancestors`, `Referrer-Policy`, CSP.
- Se migrar token para cookie `httpOnly`, adicionar CSRF token/double-submit ou SameSite estrito conforme fluxo.

Aceite:

- Headers aparecem nas respostas.
- Login/API continuam funcionando no browser.

### 11. Auditoria de acoes sensiveis

Acao:

- Criar log de auditoria padrao para:
  - login/logout;
  - alteracao de roles/permissoes;
  - habilitar/desabilitar modulos;
  - trocar billing/webhook;
  - criar/desativar user;
  - rotacionar secrets/webhook keys.
- Padronizar campos: `account_id`, `actor_user_id`, `action`, `resource_type`, `resource_id`, `before`, `after`, `ip`, `user_agent`, `created_at`.

Aceite:

- Toda mudanca administrativa critica e rastreavel.

## P1 - Performance e realtime

### 12. Preparar realtime para escala horizontal

Problema:

- Hub/event bus atual e em memoria.
- Com duas replicas da API, um evento publicado na replica A nao chega nos sockets conectados na replica B.

Acao:

- Introduzir broker para pub/sub:
  - Redis Pub/Sub ou Streams no curto prazo;
  - NATS/Kafka se o produto exigir garantias mais fortes.
- Manter contrato WebSocket atual.
- Separar "evento de invalidacao" de "evento de dominio persistido".

Aceite:

- Duas instancias da API recebem e entregam o mesmo evento de operacao/context/tasks.

### 13. Outbox para eventos importantes

Problema:

- Eventos em memoria podem se perder se a API cair depois de gravar no banco e antes de publicar.

Acao:

- Para eventos criticos, usar transactional outbox:
  - grava dado e evento na mesma transacao;
  - worker publica;
  - marca entregue/retry.

Aceite:

- Mudancas importantes nao somem em restart.

### 14. Graceful shutdown

Problema:

- `main.go` usa `ListenAndServe`.
- Loops background usam `context.Background()`.

Acao:

- Capturar `SIGTERM/SIGINT`.
- Usar `server.Shutdown(ctx)`.
- Passar contexto de aplicacao para schedulers.
- Fechar WebSockets com close frame quando shutdown iniciar.

Aceite:

- Container para sem cortar requests em andamento.
- Jobs encerram limpos.

### 15. Medir hot paths antes de otimizar demais

Acao:

- Adicionar logs/metricas por endpoint:
  - latencia p50/p95/p99;
  - status code;
  - account_id;
  - query count quando possivel;
  - tamanho de payload.
- Rodar `EXPLAIN ANALYZE` para:
  - `/v1/operations/snapshot`;
  - `/v1/reports/*`;
  - `/v2/me/accounts`;
  - `/v2/me/context`;
  - listagens de tasks/users/leads.

Aceite:

- Backlog de indices e query tuning baseado em dados reais.

### 16. Paginacao cursor e payload lean

Acao:

- Trocar offset por cursor em listas que crescem: users, tasks, leads, historico de atendimentos, notificacoes.
- Criar responses de resumo para lista e endpoint separado para detalhe.
- Evitar carregar memberships/relacoes pesadas em listagem inicial.

Aceite:

- Listar pagina N nao fica progressivamente mais caro por `OFFSET`.
- Payload de listagem contem somente campos usados na tabela/card.

### 17. Compressao HTTP

Acao:

- Middleware gzip ou brotli conforme reverse proxy.
- Respeitar `Accept-Encoding`.
- Pular compressao em respostas pequenas e streams/WebSocket.

Aceite:

- JSON grande sai com `Content-Encoding: gzip` ou equivalente no proxy.

## P1 - Banco e dados

### 18. Atualizar ERD e dicionario de dados

Problema:

- `back/database/ERD.md` reconhece que esta parcialmente legado.
- A base atual ja tem `core`, `queue`, `tasks`, `site`, `notifications` e `roadmap`.

Acao:

- Atualizar ERD com schemas atuais.
- Gerar dicionario de dados por tabela/coluna.
- Marcar owner por modulo.
- Explicar quais objetos `public` sao compatibilidade e quais sao fonte oficial.

Aceite:

- Nova pessoa entende account/user/module/queue/tasks/site sem ler migrations antigas.

### 19. Congelar convencao de migrations

Acao:

- Definir formato unico: `NNNN_descricao.sql`.
- Proibir duplicidade de prefixo, exceto quando formalmente documentado.
- Criar check simples no CI que falha se houver prefixo duplicado ou migration fora de ordem.
- Nao editar migration aplicada; criar nova migration de correcao.

Aceite:

- Ambiente limpo aplica migrations na mesma ordem que dev/prod.

### 20. Revisar `public` residual

Acao:

- Separar `public` em:
  - ainda necessario;
  - view de compatibilidade temporaria;
  - legado removivel;
  - tabela de sistema (`schema_migrations`).
- Remover dependencias no codigo aos poucos.

Aceite:

- Cada objeto `public.*` tem dono e plano: manter, migrar ou remover.

### 21. Backups, restore e PITR

Acao:

- Definir politica:
  - backup diario completo;
  - WAL/PITR se aplicavel;
  - restore testado mensalmente;
  - retencao por ambiente.
- Documentar runbook de restore.

Aceite:

- Restore testado em banco novo com tempo medido.

## P2 - Produto enterprise

### 22. Modelo de tenants/accounts mais explicito

Acao:

- Decidir vocabulario final:
  - `account` para cliente contratante/workspace;
  - `store` para loja;
  - `organization` para grupo de accounts;
  - evitar `tenant` no front novo, exceto em camada legada.
- Criar guia de nomes para API, DB e UI.

Aceite:

- Novos endpoints nao misturam `tenantId` e `accountId`.

### 23. Modulos como contrato de produto

Acao:

- Cada modulo deve ter:
  - catalogo `core.modules`;
  - permissoes;
  - role templates;
  - rotas backend;
  - guard backend;
  - guard frontend;
  - nav;
  - docs de enable/disable.
- Criar teste que compara nav/front guard/back guard/catalogo.

Aceite:

- Nao existe menu para modulo sem rota, nem rota sensivel sem modulo.

### 24. Observabilidade completa

Acao:

- Padronizar logs com `request_id`, `account_id`, `user_id`, `module`, `op`.
- Adicionar metricas Prometheus/OpenTelemetry.
- Dashboards:
  - latencia por endpoint;
  - erros por modulo;
  - WebSocket connections;
  - eventos publicados;
  - pool Postgres;
  - filas/jobs.
- Alertas de SLO.

Aceite:

- Incidente consegue ser investigado sem acessar container manualmente.

### 25. CI/CD e qualidade

Acao:

- Checks:
  - `go test ./...`;
  - `npm --prefix web run test`;
  - `npm --prefix web run typecheck`;
  - lint;
  - migration check;
  - smoke de API com banco limpo;
  - teste cross-tenant.
- Build de imagem reprodutivel.
- Version matrix documentada.

Aceite:

- PR que quebra contrato multi-tenant falha antes do merge.

## Roadmap sugerido

### Primeiros 7 dias

1. Remover senha do `localStorage`.
2. Ligar `core.user_sessions` no boot e revogacao no logout.
3. Corrigir provider de `X-Account-Id`.
4. Corrigir dedupe de GET.
5. Alinhar `docker-compose.prod.yml` com Core V2 e roles em `core`.
6. Decidir o destino imediato de `site`, `roadmap` e `operationgoals`.

### 30 dias

1. Registrar ou esconder todos os modulos pendentes.
2. Atualizar ERD/dicionario de dados.
3. Criar testes cross-account para queue, tasks, site e notifications.
4. Adicionar security headers.
5. Implementar graceful shutdown.
6. Medir hot paths e corrigir os primeiros N+1/indices.

### 60 dias

1. Introduzir broker de realtime.
2. Criar outbox para eventos criticos.
3. Implementar cursor pagination nas listas que crescem.
4. Comecar RLS por um schema de menor risco.
5. Criar dashboards operacionais.

### 90 dias

1. Expandir RLS para dominios principais.
2. Consolidar `public` residual.
3. Formalizar contrato de modulos.
4. Criar runbook de backup/restore/PITR.
5. Rodar teste de carga com massa realista e metas de SLO.

## Ordem recomendada para implementacao tecnica

1. Seguranca imediata no front: remover senha de storage.
2. Auth real no back: sessao, revogacao e cache de principal.
3. Semantica de account: `X-Account-Id` correto.
4. Coerencia de modulos: registrar/esconder site/roadmap/operationgoals.
5. Bugs de client: dedupe GET e guards.
6. Config de deploy: Core V2 e roles core em prod.
7. Observabilidade minima.
8. Realtime horizontal.
9. RLS por etapas.

## Criterio de pronto para porte maior

O projeto fica pronto para um salto de escala quando estas afirmacoes forem verdadeiras:

- Um usuario nao consegue ler dados de outra account mesmo com payload forjado.
- Logout invalida token/sessao no servidor.
- Cada modulo visivel no front existe no backend e no catalogo.
- O realtime funciona com mais de uma replica.
- Existe backup restauravel e testado.
- Existem metricas para saber onde esta lento.
- Migrations sobem em banco limpo sem drift.
- Ambientes dev/prod usam o mesmo caminho arquitetural, variando apenas segredos e sizing.
