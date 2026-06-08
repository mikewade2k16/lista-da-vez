# Plano — Remoção do legado public.* (itens 2 e 3 do LEGADO.md)

> Objetivo: eliminar a camada `public.*` legada para o código usar SÓ as tabelas canônicas. Paralelo Claude (engenheiro/identidade) + Codex (programador/Fila). Fecha o "zero legado" depois do UMU (papéis).

## Estado atual (auditado 2026-06-06)
| public.* | tipo | canônico | refs de código |
|---|---|---|---|
| `users` | VIEW | `core.users` | ~6 |
| `stores` | VIEW | `queue.stores` | ~36 |
| `consultants` | VIEW | `queue.consultants` | ~7 |
| `tenants` | **TABELA REAL** | `core.accounts` (superset, 0 drift) | ~21 |

Total ~110 refs em 19 arquivos. `core.accounts` tem TODAS as colunas de `tenants` + mais.

## Mapeamento (em TODO o código)
- `users` → `core.users`
- `stores` → `queue.stores`
- `consultants` → `queue.consultants`
- `tenants` → `core.accounts`

## Divisão (arquivos inteiros por agente — zero conflito)

### Claude (identidade/CRM + os WRITES arriscados)
- `tenants/store_postgres.go` (writes tenant → `core.accounts`, com colunas obrigatórias) + `tenants/scope_queries.go`
- `auth/store_postgres.go`, `users/store_postgres.go`
- `crm/erp/repository_scope.go`, `repository_crm_scope.go`, `repository_crm_queue.go`, `repository_crm_links.go`
- `access/repository_postgres.go`
- `platform/database/bootstrap_owner.go`, `bootstrap_erp_store.go` (writes tenant/store → core/queue)

### Codex (Fila/queue/stores — majoritariamente READS mecânicos)
- `queue/consultants/store_postgres.go` (consultants→queue.consultants; writes de loja já foram p/ core no U4b)
- `stores/store_postgres.go` (writes stores→queue.stores) + `stores/scope_queries.go`
- `queue/operations/store_postgres.go`, `relations_resolver.go`
- `queue/settings/store_postgres.go`, `queue/reports/store_postgres.go`, `queue/alerts/store_postgres_signals.go`
- Prompt: `docs/codex/ITEM23_QUEUE_STORES.md`

## Regras críticas
- **WRITES de `tenants`/`stores`** precisam das colunas obrigatórias do destino. `core.accounts` exige além de slug/name/is_active: checar NOT NULL (organization_id, plan_code, billing_mode…) e usar defaults/valores corretos. `queue.stores` idem. **Cada agente valida o INSERT do seu módulo.**
- **NÃO dropar** as views/tabela ainda — isso é o passo final do Claude.
- `account_id` do Principal, params posicionais, gofmt, máx 450 linhas, sem `_` em erro.
- Arquivos compartilhados (`roadmap-data.ts`, `LEGADO.md`, este plano) → Claude consolida; Codex lista no resumo.

## Sequência
1. **Sweep paralelo** (Claude + Codex): migrar todas as refs dos seus módulos. `go build ./...` + testes verdes por agente.
2. **Claude revisa** o Codex + roda grep confirmando **zero** ref a `\b(from|join|into|update) (users|stores|consultants|tenants)\b` (nome cru) no código vivo.
3. **Claude (destrutivo, com backup):** migration `0136_drop_legacy_public.sql` — `drop view public.users/stores/consultants` + `drop table public.tenants`. Idempotente.
4. **Teste geral** (roteiro abaixo) — você roda no browser + eu rodo os smokes de API.

## ✅ STATUS (2026-06-06) — código + DROP concluídos
- **Sweep paralelo:** Claude (11 arquivos) + Codex (8 arquivos) migrados; `go build ./...` + `go test ./...` verdes.
- **Grep:** ZERO ref crua a `users|stores|consultants|tenants` no código vivo (só comentários + 1 teste de integração que cria a própria view).
- **Migration `0136_drop_legacy_public_objects.sql`:** repontou as **27 FKs** `tenant_id → public.tenants` para `core.accounts(id)`, dropou as 3 views + a tabela. Aplicada (`migration_up_ok`); 0 FKs em `public.tenants`, 47 em `core.accounts`. Backup: `C:\tmp\omni_pre_drop_public.dump`.
- **Falta:** smoke de login via HTTP (precisa credencial) + browser (seção C).

## Roteiro de TESTE ÚNICO (rodar tudo de uma vez no fim)
> Eu rodo os itens de API/SQL; você roda os de browser. Marcamos juntos.

### A. Banco (eu rodo) — ✅ CONCLUÍDO
- [x] `select to_regclass('public.users'),to_regclass('public.stores'),to_regclass('public.consultants'),to_regclass('public.tenants');` → tudo **NULL** (sumiram).
- [x] grep zero refs a nome cru no código vivo.
- [x] `go build ./...` + `go test ./...` verdes.

### B. API/login (eu rodo) — ⏳ aguardando credencial de teste
- [ ] login mike (platform_admin), filipe (owner), 1 consultor → todos 200.
- [ ] criar user no manage (role) → 201 + login.
- [ ] criar consultor → loja no core; criar/editar loja; criar/editar tenant (account).
- [ ] GET /v1/tenants, /v1/stores, /v1/consultants, /v1/erp/crm, /v1/operations → 200.

### C. Browser (VOCÊ roda) — fluxos ponta a ponta
- [ ] **Login** como admin → entra, menu carrega.
- [ ] **/manage/clientes-web**: criar um cliente novo (account); editar módulos (toggle); editar billing no modal; ver board card.
- [ ] **/manage/users**: criar usuário com cliente + papel; ele aparece e loga.
- [ ] **/manage/organizations**: criar uma organization; vincular.
- [ ] **/operacao/usuarios**: usuário aparece; criar/editar; o LegacyMarker reflete a verdade.
- [ ] **/operacao** (Fila): abrir uma loja, ver consultores, fila funciona.
- [ ] **Consultores**: criar consultor com acesso (user+loja) → loga + aparece na operação.
- [ ] **CRM/ERP**: abrir /erp, ver vendas por loja/consultor (atribuição funciona).
- [ ] **Multiloja/lojas**: criar/editar loja, trocar de loja no header.
- [ ] **Módulos**: desabilitar um módulo de um cliente → rota dele dá 403; reabilitar → volta.
- [ ] **Troca de account** (platform_admin no switcher) → menu recarrega com módulos certos.
- [ ] **Roadmap** (/roadmap) abre, aba Módulos/Regras.

### D. Critério de "FINALIZADO"
Nenhuma tabela/view `public.*` legada; código só em `core.*`/`queue.*`; todos os fluxos B+C verdes; `AUTH_ROLES_SOURCE=core`.

## Notas de Deploy (ordem exata)
1. **Migration nova `0136_drop_legacy_public_objects.sql`** (auto no boot da API). Idempotente. **Destrutiva** — dropa `public.users/stores/consultants` (views) + `public.tenants` (tabela), repontando 27 FKs para `core.accounts`. **Pré-requisito:** 0 drift entre `public.tenants` e `core.accounts` (garantido pela 0101). **Fazer `pg_dump -Fc` antes** (feito local: `C:\tmp\omni_pre_drop_public.dump`).
2. **Rebuild obrigatório da API** (mudou Go + migration embedada): `docker compose up -d --build api`. Restart não basta.
3. Sem env nova, sem dep nova, sem mudança de Dockerfile.
4. **Rollback:** restaurar do dump pré-drop (`pg_restore`). Não há "down migration" — o drop é definitivo; a recuperação é via backup.
