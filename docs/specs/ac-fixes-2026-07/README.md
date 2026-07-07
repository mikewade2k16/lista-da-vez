# ac-fixes-2026-07 — Índice das specs de correção

Conjunto de specs de implementação derivadas do diagnóstico de 2026-07-02
(`scratchpad/fatos.json → achados_canonicos.*`), na branch
`refactor/multitenant-complete`. Cada `.md` desta pasta é **autossuficiente**: o
implementador recebe só o arquivo do seu AC e nada mais.

Este README é o índice das 11 specs, o registro do plano de ondas e o estado real
desta rodada de implementação.

---

## 1. As 11 specs

| Arquivo | AC | Título | Prioridade | Status |
|---|---|---|---|---|
| [`AC-01-principal-cache.md`](AC-01-principal-cache.md) | AC-01 | Ligar o PrincipalCache (auth deixa de ir ao banco a cada request) | P0 · S | Implementado nesta rodada |
| [`AC-02-stacks-orfas-vps.md`](AC-02-stacks-orfas-vps.md) | AC-02 | Runbook: higiene das stacks órfãs da VPS (crash-loop + omni-crow/omni-staging) | P0 · S | Runbook manual — pendente de execução |
| [`AC-03-tmp-dump-segredos.md`](AC-03-tmp-dump-segredos.md) | AC-03 | tmp/ com dump real de banco (245MB) e tokens/secret em disco | P0 · S | Runbook manual — pendente de execução |
| [`AC-04-db-role-least-privilege.md`](AC-04-db-role-least-privilege.md) | AC-04 | Role de runtime least-privilege no Postgres (`omni_app`) | P1 · M | Implementado nesta rodada |
| [`AC-05-backup-agendado-offsite.md`](AC-05-backup-agendado-offsite.md) | AC-05 | Backup agendado do Postgres + retenção + off-site + teste de restore | P1 · S | Implementado nesta rodada |
| [`AC-06-paginacao-listagens.md`](AC-06-paginacao-listagens.md) | AC-06 | Paginação/teto em listagens sem limite (reports, analytics, BI) | P1 · M | Implementado nesta rodada |
| [`AC-07-arquivos-gigantes-front.md`](AC-07-arquivos-gigantes-front.md) | AC-07 | Arquivos gigantes no front (>450 linhas) + components.css monolítico | P1 · L | Implementado nesta rodada |
| [`AC-11-compose-limites-healthchecks.md`](AC-11-compose-limites-healthchecks.md) | AC-11 (+16) | Compose: limites de memória/CPU + healthchecks + restart/depends_on | P1 · S | Implementado nesta rodada |
| [`AC-12-finance-api-real.md`](AC-12-finance-api-real.md) | AC-12 | Finance na API Go real: eliminar o último mock do BFF Nitro | P1 · M | Implementado nesta rodada |
| [`AC-15-testes-front.md`](AC-15-testes-front.md) | AC-15 | Testes front Onda 1: stores e domain/utils puros | P1 · M | Implementado nesta rodada |
| [`AC-16-monitoracao-producao.md`](AC-16-monitoracao-producao.md) | AC-16 | Monitoração mínima de produção (uptime + alertas + healthz com DB + log rotation) | P1 · M | Coberto pela parte "compose" do AC-11 nesta rodada |

> Nota AC-11/AC-16: a spec do AC-11 absorve a base mínima de monitoração do AC-16
> (healthchecks como fundação). Por isso o par é tratado como **AC-11+16** no plano
> de ondas abaixo. O restante do AC-16 (uptime externo, alertas de host, healthz
> com ping no DB, log rotation) permanece na sua spec para execução futura.

> **Status 2026-07-03:** a rodada 1 foi para PRODUÇÃO (deploy `local-20260703-101734`),
> com evidência medida na VPS (PrincipalCache ~89% hit, api como `omni_app`, mem_limit,
> /healthz com DB, backup agendado). AC-03 executado (tmp/ higienizado). Incidente do
> deploy AC-04 (~1h de 502 por runbook pulado) registrado no ENGINEERING_PRINCIPLES
> (falha nº 13) — prevenção virou a spec AC-04b da rodada 2 abaixo.

---

## 1b. Rodada 2 — pendências (specs escritas em 2026-07-03)

17 specs novas cobrindo TUDO que restou (fonte: tabela "Ações prioritárias" de
`docs/relatorios/2026-07/index.html` + fases `ac-fixes-2026-07`, `observabilidade-n8n` e
`deploy-registry-staging` do roadmap). Fora desta rodada por decisão do dono: SEC-1 (RLS)
e VPS-2 (migração de VPS).

**Como despachar:** 1 agente **Opus** por spec (`model: "opus"`), na ordem recomendada
abaixo, aguardando a dependência quando indicada. Cada spec é autossuficiente; o agente
NÃO decide nada fora dela e NUNCA roda git.

| Arquivo | ID | Título | Prio | Depende de |
|---|---|---|---|---|
| [`AC-04b-migrate-auto-provision-role.md`](AC-04b-migrate-auto-provision-role.md) | AC-04b | Auto-provisão da role `omni_app` no migrate (deploy self-healing) | P1 · S | — |
| [`OBS-01-check-vps-webhook-n8n.md`](OBS-01-check-vps-webhook-n8n.md) | OBS-01 | check-vps.sh → webhook n8n + severidade + check de backup | P1 · S | — (URL só entra após OBS-02) |
| [`OBS-02-n8n-fanout-alertas.md`](OBS-02-n8n-fanout-alertas.md) | OBS-02 | Workflow n8n de fan-out (e-mail/Telegram/ntfy) | P1 · S | contrato com OBS-01 |
| [`OBS-03-uptime-externo.md`](OBS-03-uptime-externo.md) | OBS-03 | Uptime externo (UptimeRobot) no /healthz + painel | P1 · S | — (runbook) |
| [`AC-05b-offsite-restore-drill.md`](AC-05b-offsite-restore-drill.md) | AC-05b | Off-site (rclone) + drill de restore mensal | P1 · S | credenciais do bucket (dono) |
| [`D3-staging-env-vps.md`](D3-staging-env-vps.md) | D3 | `.env.staging` real na VPS (staging sob demanda) | P2 · S | AC-04b facilita |
| [`AC-17-load-test-explain.md`](AC-17-load-test-explain.md) | AC-17 | Load test k6 (docker) + EXPLAIN dos endpoints quentes | P2 · M | credenciais de teste (dono) |
| [`AC-07b-onda-2a-tasks.md`](AC-07b-onda-2a-tasks.md) | AC-07b·2a | Refactor layer tasks (3 maiores arquivos do repo) | P1 · L | — |
| [`AC-07b-onda-2b-multistore-operacao.md`](AC-07b-onda-2b-multistore-operacao.md) | AC-07b·2b | Refactor multistore + modal de encerrar | P1 · L | após 2a (ou paralelo, áreas distintas) |
| [`AC-07b-onda-2c-stores-centrais.md`](AC-07b-onda-2c-stores-centrais.md) | AC-07b·2c | Refactor dos 6 stores centrais | P1 · L | após 2b |
| [`AC-07b-onda-2d-erp-crm.md`](AC-07b-onda-2d-erp-crm.md) | AC-07b·2d | Refactor ERP/CRM | P1 · L | não paralelo com 2c |
| [`AC-07b-onda-2e-composables-domain.md`](AC-07b-onda-2e-composables-domain.md) | AC-07b·2e | Refactor permissions/metrics/reports + OmniEditor | P1 · L | rodar SOZINHA |
| [`AC-07b-onda-2f-cauda.md`](AC-07b-onda-2f-cauda.md) | AC-07b·2f | Cauda por área (8 lotes, spec-molde) | P1 · L | lotes 1/2/3 após 2a/2b/2c |
| [`AC-15b-testes-onda-2.md`](AC-15b-testes-onda-2.md) | AC-15b | Testes front onda 2 (13 alvos + happy-dom) | P2 · L | após 2c |
| [`OBS-06-alertas-negocio.md`](OBS-06-alertas-negocio.md) | OBS-06 | Alertas de negócio fase 1 (long_queue_wait/long_pause) | P2 · M | — |
| [`OBS-05-painel-status-interno.md`](OBS-05-painel-status-interno.md) | OBS-05 | Painel de status real (substitui o mock /monitoramento) | P2 · M | melhor após OBS-06 |
| [`OBS-04-anomalia-n8n.md`](OBS-04-anomalia-n8n.md) | OBS-04 | Anomalia via n8n agendado (regras v1) | P2 · M | **OBS-02 + OBS-05** |

**Ordem recomendada de despacho:** AC-04b → OBS-01+OBS-02+OBS-03 (paralelo ok) → AC-05b →
D3 → AC-17 → AC-07b 2a→2b→2c→2d→2e→2f (sequencial por onda; 2f em lotes) → AC-15b →
OBS-06 → OBS-05 → OBS-04 (por último). Itens com "(dono)" precisam de credencial/decisão
sua no meio — o agente para e pergunta.

---

## 2. Plano de ondas usado

A implementação seguiu ondas para respeitar dependências de arquivo e o gate humano
dos runbooks:

- **Onda 1 — segurança/perf de base:** AC-01, AC-04, AC-05, AC-06.
  - Cadeia de edição sequencial no mesmo arquivo de boot
    (`back/internal/platform/app/app.go`): **AC-01 → AC-12 → AC-11+16**. Os três
    tocam o wiring/boot da API, então foram encadeados para evitar conflito de merge
    no mesmo arquivo.
- **Onda 2 — front/qualidade:** AC-07 e AC-15.
- **Runbooks humanos (fora das ondas de código):** AC-02 e AC-03. Nenhum agente
  acessa a VPS nem toca em `tmp/`; a única mudança versionada é a documentação do
  runbook. A execução é 100% do usuário.

---

## 3. Regras da rodada

- **Implementação só com ok explícito do usuário.** Plano/specs não autorizam codar;
  cada onda de implementação exige aprovação.
- **Nenhum agente roda git.** Commit/push/deploy são sempre do usuário, manualmente —
  mesmo quando autorizado, o agente devolve os comandos.
- **Back valida com `docker compose up -d --build api`.** Toda mudança em `back/`
  exige rebuild da imagem da API (restart não basta). O orquestrador roda o build;
  os agentes não sobem containers.
- **npm/generate/build do web só com aprovação explícita** ("aprovei"). Implementar e
  parar para revisão no dev.
- **Sem credenciais inventadas.** Smokes autenticados (login no painel, Operação, CRM)
  ficam para o usuário; o agente nunca inventa login/senha.

---

## 4. Estado desta rodada

### Implementados

**AC-01 — PrincipalCache ligado (P0).**
Liguei o `PrincipalCache` no boot com TTL configurável `AUTH_PRINCIPAL_CACHE_TTL`
(default 30s; `0s` desliga e preserva o comportamento legado) via arquivo novo de
wiring, cortando ~3 round-trips ao banco por request autenticada a partir da 2ª
chamada da mesma sessão. Invalidação direta/síncrona (não via bus): logout derruba a
sessão; access/users/core RBAC/AdminUser derrubam o usuário; matriz v1 e DeleteRole
derrubam tudo. Contadores hit/miss + `Stats()`/`Len()`, goroutine de `Cleanup` (60s),
log de hit rate (5 min), 5 testes novos, 8 AGENT.md e 4 `.env*.example` atualizados.
Token legado sem `sid` continua ignorando o cache; `sessions.Touch` segue sem ser
chamado.
Pendências:
- Rebuild da API pendente (regra: o orquestrador roda `docker compose up -d --build api`).
- Fora do escopo AC-01: `go vet` no boot acusa arquivo novo não-commitado de outro
  agente — `back/internal/modules/queue/operations/store_postgres_history.go` — que
  **não compila** (`LoadSnapshot`/`loadServiceHistory` duplicados + chamada com
  aridade errada). Como `app` importa `queue`, o binário da API só linka depois que
  esse agente terminar. Não toquei nele.
- Fora do escopo AC-01: o agente do AC-04 adicionou validação `DATABASE_APP_URL` em
  config; `TestValidateAcceptsProductionWithSecureValues` falha até ele finalizar. A
  env var deste AC é independente e não causa isso.
- Corrida Set-após-Logout (janela de ms, teto = TTL 30s) aceita por decisão da spec;
  tombstone fica para a versão Redis (AC-08). Cache é local ao processo — escalar
  horizontalmente exige AC-08 antes.
- Smoke manual (login/navegar/logout/desativar usuário) não executado — precisa de
  credenciais; validar após o rebuild.

**AC-04 — role least-privilege `omni_app` (P1).**
Dois pools (`OpenAppPool` com `DATABASE_APP_URL` sem DDL para a API; `OpenPool` com
`DATABASE_URL` privilegiada para o migrate), script SQL idempotente de criação da
role, GRANTs auto-sincronizados a cada `migrate up` via `SyncAppRoleGrants` (USAGE em
schemas, DML em tabelas/views, sequences, `ALTER DEFAULT PRIVILEGES` global), guard de
produção no `Validate()` exigindo `DATABASE_APP_URL`, e teste de integração (positivo
DML + negativo DDL `42501` + idempotência). Composes dev/prod, 4 `.env` examples e docs
(2 AGENT.md, RLS_PLAN, runbook no plano canônico) atualizados. Coexiste sem conflito
com o `AUTH_PRINCIPAL_CACHE_TTL` do AC-01.
Pendências:
- 2 issues golangci `gocritic exitAfterDefer` em `cmd/api/main.go:30` e
  `cmd/migrate/main.go:26` são **pré-existentes** (o `os.Exit` após `defer cancel` já
  existia); fora do escopo. O helper `lint-go-staged.sh` usa `--new-from-rev=HEAD` e
  não deve bloqueá-los.
- Volume dev **existente** não roda o `initdb.d` automaticamente: rodar uma vez
  `docker compose exec -T postgres sh /docker-entrypoint-initdb.d/10-app-role.sh`.
  Volume novo cria a role sozinho.
- Smoke autenticado (login + Operação + uma lista CRM com a role nova) fica para o
  usuário.
- Deploy em prod exige criar a role **antes** da imagem nova subir (a imagem com
  `Validate()` não sobe sem `DATABASE_APP_URL`); runbook em
  `docs/MULTITENANT_COMPLETION_PLAN.md` seção "AC-04 (2026-07-02)".
- Atende o pré-requisito crítico do `RLS_PLAN` (superuser ignora RLS); a fundação RLS
  segue inerte até ser retomada em fase própria.

**AC-05 — backup agendado + off-site + restore (P1).**
Os 9 critérios da seção 5 atendidos: `backup-db.sh` com `set -euo pipefail`, `flock`,
`gzip -t` + tamanho mínimo (`MIN_BYTES=10240`), retenção 7/4, off-site condicional a
`BACKUP_RCLONE_REMOTE`, status file `ok|fail` e zero credencial hardcoded; `bash -n`
sai 0; `backup-check.yml` às 09:00 UTC + `workflow_dispatch` usando o secret
`DEPLOY_VPS_SSH_KEY`, falha o job se o status não for `ok` fresco (<26h) mesmo após
fallback; `BACKUP_RESTORE.md` com 7 seções, crontab pronta e teste de restore mensal;
seção Backup do `DEPLOY_VPS.md` reescrita; `scripts/backup/AGENT.md` criado. Nada
removido, nenhum Go/TS tocado, nenhuma migration, nenhum arquivo >450 linhas (máx 213),
nenhum comando git.
Pendências:
- Backup do volume `api_uploads` e do `.env.production` continua manual (documentado
  como pendência; candidato a AC futuro — este AC é só o banco).
- Validação do YAML do workflow pendente localmente (sem pacote `yaml` / sem
  python-yaml e sem `npm install`); validar com `actionlint` ou editor. A estrutura
  espelha o `deploy-vps.yml` em produção.
- Dry-run local abortou no `flock` (não existe no Git Bash do Windows); é limitação do
  ambiente, não do script (`flock` é padrão no Ubuntu 24.04). Comportamento fail-safe
  confirmado: exit 1 + linha `fail` no status file. Validação real (script +
  `workflow_dispatch`) fica para a VPS.
- Nenhuma alteração em `back/` — **não** precisa `docker compose up -d --build api`.

**AC-06 — paginação/teto em listagens (P1).** Implementado nesta rodada (reports /
analytics / BI com teto de query em vez de carregar tudo em memória). Rebuild da API
pendente (mudou `back/`).

**AC-07 — arquivos gigantes no front (P1).** Implementado nesta rodada (Recorte 1:
arquivos >1.000 linhas fatiados + `components.css` dividido por domínio via `@import`).
Recortes 2+ ficam como backlog na própria spec. Validação `npm`/vitest só com "aprovei".

**AC-11+16 — compose: limites + healthchecks (P1).** Implementado nesta rodada
(limites de memória/CPU, healthchecks em web/redis/waha/n8n, `restart`/`depends_on`
com condition). Cobre a base mínima de monitoração do AC-16.

**AC-12 — Finance na API Go real (P1).** Implementado nesta rodada (elimina os 10
arquivos do BFF Nitro; migration 0187; rotas `/v1/finance/*`). Rebuild da API pendente
(mudou `back/`).

**AC-15 — testes front Onda 1 (P1).** Implementado nesta rodada (stores Pinia e
`domain/utils` puros, com `$fetch` global mockado via `web/test/setup.ts`). Onda 2
(componentes/DOM) fica para depois. `npm run test` é gate do CI; asserts
determinísticos.

### Runbooks manuais pendentes de execução

- **AC-02 — stacks órfãs da VPS.** Runbook documentado (seção "Higiene da VPS" em
  `docs/DEPLOY_VPS.md`). Execução por SSH é 100% do usuário. **Nunca parar/remover o
  Caddy** (`omnichannel-mvp-caddy-1`) — roteia o painel inteiro.
- **AC-03 — limpeza de `tmp/`.** Runbook + nota de prevenção no AGENT.md raiz. As
  seções de limpeza (dump real de banco de ~245MB, tokens/secret em texto plano) são
  executadas **só pelo dono do projeto**, manualmente. `tmp/` é gitignored — risco é
  de higiene de disco local, não de vazamento no git.

### Pendência transversal de validação

Rebuild da API (`docker compose up -d --build api`) está pendente e é bloqueado até o
arquivo não-compilável de outro agente
(`back/internal/modules/queue/operations/store_postgres_history.go`) ser corrigido,
já que `app` importa `queue`. Os ACs que mexeram em `back/` (AC-01, AC-04, AC-06,
AC-11+16, AC-12) só terminam a validação após esse rebuild.
