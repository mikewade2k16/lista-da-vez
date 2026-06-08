# DEMANDA CODEX — UMU/U2: auth resolve papel/escopo a partir de `core.*` (com backfill + fallback)

> Você é o **programador** (Codex). O engenheiro especificou esta demanda e vai **revisar** o resultado contra os "Critérios de aceite" no fim. Execute exatamente o escopo abaixo — nem mais (não dropar legado ainda), nem menos.

## 0. Leia primeiro (contexto obrigatório)
1. `docs/USER_MODEL_UNIFICATION_PLAN.md` — o plano (você executa o estágio **U2**).
2. `docs/LEGADO.md` — item 1 (papéis legados) é o que estamos atacando.
3. `AGENT_RULES.md` — seção "Legado, mocks e fonte da verdade" + regras de Backend/Banco. **Cumpra todas.**
4. `docs/ENGINEERING_PRINCIPLES.md` — máx 450 linhas/arquivo, gofmt, camadas (handler→service→repo), nunca `_` em erro, etc.
5. Arquivos centrais desta demanda:
   - `back/internal/modules/auth/store_postgres.go` → função `LoadUserForAuth` (HOJE resolve `role`/`tenant_id`/`store_ids` SÓ das tabelas legadas `user_tenant_roles`/`user_store_roles`/`user_platform_roles`).
   - `back/internal/modules/auth/service.go` → `Login`/`Authenticate` usam o `User` que vem do store.
   - `back/internal/modules/auth/model.go` → struct `User` (campos role/tenantId/storeIds).
   - Schema core: `core.account_users`, `core.user_role_assignments`, `core.roles`, `core.role_permissions`, `core.users.is_platform_admin` (ver `migrations/0100_core_schema.sql`).

## 1. O problema (por quê)
A identidade já é única (`public.users` é VIEW sobre `core.users`). Mas o **papel** ainda é resolvido do **legado**. Um usuário que existe só em `core.*` (membership em `core.account_users`, sem `user_tenant_roles`) **não consegue logar** (`LoadUserForAuth` retorna papel vazio e o `Login` quebra → 500). Hoje há um **band-aid**: `admin_users_repository.CreateUser` grava o papel nos DOIS sistemas. Queremos o auth lendo de `core.*` para poder, no U4, **dropar o legado**.

## 2. Escopo desta demanda (U2) — o que fazer

### 2.1 Backfill (pré-requisito, idempotente, additivo)
- Migration nova `back/internal/platform/database/migrations/0133_backfill_legacy_roles_to_core.sql`:
  - Para cada `user_tenant_roles`/`user_store_roles` → garantir `core.account_users(account_id=tenant_id, user_id)` (já parcialmente feito; completar) **e** `core.user_role_assignments` apontando para o `core.roles` correspondente do módulo. Mapeie o papel legado → role template core:
    - `owner`/`director`/`marketing` (tenant) → role core equivalente do módulo `queue` (use os `core.roles` clonados dos templates `queue.supervisor`/`queue.consultant`; defina o mapeamento e DOCUMENTE em `docs/LEGADO.md`).
    - `manager`/`consultant`/`store_terminal` (store) → idem, role core do `queue`.
  - `user_platform_roles` → `core.users.is_platform_admin = true` (já feito na 0101; reconciliar).
  - `IF NOT EXISTS`/`ON CONFLICT DO NOTHING`. Idempotente. NÃO apague nada legado aqui.

### 2.2 Resolver de papel/escopo a partir de core
- Em `auth/store_postgres.go` (ou um novo `auth/core_role_resolver.go` se ficar > 450 linhas), implemente a resolução do `role`/`tenant_id`/`store_ids` a partir de `core.*`:
  - `is_platform_admin = true` → role `platform_admin`.
  - senão, derive o papel coarse a partir de `core.user_role_assignments`/`core.account_users` do usuário. **Você define o mapeamento core-role → role coarse** (o inverso do 2.1) e DOCUMENTA. Deve preservar o comportamento atual para os usuários existentes.
  - `tenant_id` = a account ativa do usuário (de `core.account_users`); `store_ids` = lojas acessíveis (de `core.*`/`queue.stores`).
- **Fallback legado (transição):** se a resolução core vier vazia, caia no caminho legado atual. Atrás de um flag de config (ex.: `AUTH_ROLES_SOURCE=core|legacy|core_with_fallback`, default `core_with_fallback`). **NÃO remova o caminho legado** — isso é U4.

### 2.3 Testes Go (obrigatório)
- `auth/...test.go` cobrindo:
  - Usuário **core-only** (em `core.account_users` + `core.user_role_assignments`, SEM `user_tenant_roles`) resolve papel correto e **loga** (hoje quebra).
  - Usuário **platform_admin** (is_platform_admin) → role `platform_admin`.
  - Usuário existente (com papel legado) continua com o MESMO papel/escopo (sem regressão) via o caminho core ou fallback.
  - Use mocks/seed in-memory no padrão dos testes atuais do projeto (são unitários com mock; veja `account_guard_test.go`/`tasks` para o estilo). Onde precisar de DB, deixe claro e mínimo.

## 3. O que NÃO fazer (fora do escopo)
- **NÃO** dropar nem alterar `user_tenant_roles`/`user_store_roles`/`user_platform_roles` (isso é U4).
- **NÃO** remover o band-aid de sync do `admin_users_repository.CreateUser` (U4).
- **NÃO** tocar em `password_hash` nem rodar seed que sobrescreva senha/dados de usuário (regra dura — AGENT_RULES).
- **NÃO** mexer no `/operacao/usuarios` (isso é U3).

## 4. Regras a cumprir (resumo do AGENT_RULES/ENGINEERING_PRINCIPLES)
- Banco: porta `5432` é o container `omni-postgres-1` (confirme via `docker ps`; o doc antigo diz 5433, a realidade é 5432). Migration idempotente. `account_id` em tabela tenant-scoped.
- Go: `gofmt`, máx 450 linhas/arquivo, camadas handler→service→repo, sem `_` em erro, IDs string, scan nullable com `*string`.
- Documentar: atualizar `back/internal/modules/auth/AGENT.md` (nova fonte de papel + flag), `docs/LEGADO.md` (item 1 → status + o mapeamento), e marcar `umu-auth-core` em `web/app/components/roadmap/roadmap-data.ts` quando concluir.
- **Validar local** antes de entregar: `go -C back build ./...`, `go -C back test ./...`, e um teste manual de login de um usuário core-only (descreva o curl/SQL usado).
- NÃO commitar/push/deploy — deixe as mudanças no working tree.

## 5. Critérios de aceite (o engenheiro vai verificar isto)
1. `go -C back build ./...` e `go -C back test ./...` verdes (inclui os testes novos do 2.3).
2. Um usuário criado **só no core** (membership + role_assignment, SEM `user_tenant_roles`) **loga com sucesso** (HTTP 200 em `/v1/auth/login`) e recebe role/tenant/store corretos.
3. Usuários existentes **não regridem** (mesmo papel/escopo de antes).
4. Caminho legado ainda existe (atrás do flag) — `user_*_roles` NÃO foram dropados nem o band-aid removido.
5. Migration 0133 idempotente (rodar 2x não duplica nem erra).
6. `auth/AGENT.md`, `docs/LEGADO.md` (com o mapeamento core↔legado) e `roadmap-data.ts` atualizados.
7. Nenhum `password_hash` alterado; nenhuma tabela legada dropada.

## 6. Entrega
Resuma: arquivos alterados, o mapeamento core↔legado que você definiu, a flag criada, e a saída de `go test` + do teste manual de login core-only. O engenheiro revisa contra a seção 5.
