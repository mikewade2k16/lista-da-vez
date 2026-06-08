# DEMANDA CODEX — UMU/U3: `/operacao/usuarios` lê de `core.*` + projeção da Fila em `core.user_module_settings`

> Você é o **programador** (Codex). O engenheiro especificou e vai **revisar** contra os "Critérios de aceite". Execute exatamente o escopo — não dropar legado (é U4), não remover o dual-write ainda.

## 0. Leia primeiro (contexto obrigatório)
1. `docs/USER_MODEL_UNIFICATION_PLAN.md` — você executa o estágio **U3**.
2. `docs/LEGADO.md` — item 1 (papéis legados). U2 já entregou: auth lê de core (flag `AUTH_ROLES_SOURCE`, default `core_with_fallback`) + migration 0133 backfillou legado→core + `back/internal/modules/auth/core_role_resolver.go` (mapeamento core-role.code ↔ papel coarse — REUSE este mapeamento, não duplique).
3. `AGENT_RULES.md` (seção Legado + Backend/Banco) e `docs/ENGINEERING_PRINCIPLES.md`. Cumpra todas.
4. Arquivos centrais:
   - `back/internal/modules/users/store_postgres.go` → hoje a LISTAGEM lê de `users` (view) + `user_tenant_roles`/`user_store_roles` (LEGADO) filtrando por tenant.
   - `back/internal/modules/users/service.go` / `http.go` → CRUD da tela `/operacao/usuarios`.
   - `back/internal/modules/auth/core_role_resolver.go` → mapeamento role-core ↔ coarse (U2).
   - Tabelas core: `core.account_users`, `core.user_role_assignments`, `core.roles`, `core.user_module_settings` (module_id='queue').

## 1. O problema (por quê)
A tela `/operacao/usuarios` é "os usuários daquele cliente no módulo Fila". Hoje ela **LÊ do legado** (`user_tenant_roles`/`user_store_roles`), então um usuário criado só no core (ex.: via manage, ou no futuro) **não aparece** lá. Queremos que a listagem venha de `core.*`, e que as opções específicas da Fila (lojas do consultor/gerente, vínculo de consultor) morem em `core.user_module_settings(module_id='queue')` — não em tabela legada.

## 2. Escopo desta demanda (U3) — o que fazer

### 2.1 LISTAGEM lê de core (read-side)
- Reescrever a query de listagem do users module para montar cada usuário a partir de:
  - **Membership/escopo:** `core.account_users` (WHERE `account_id` = a account ativa — vem do `X-Account-Id`/Principal, NUNCA do body; ver ENGINEERING_PRINCIPLES pilar 1).
  - **Identidade:** `core.users` (display_name, email, nick, `employee_code`, `job_title` — estes 2 ficam em core.users, já existem).
  - **Papel coarse:** de `core.user_role_assignments` + `core.roles.code`, usando o MESMO mapeamento do `core_role_resolver.go` (extraia/reuse — se precisar, mova o mapeamento para um ponto compartilhável; não copie/cole).
  - **storeIds (Fila):** de `core.user_module_settings(module_id='queue').config -> 'storeIdsByAccount' -> <accountId>`.
- O shape do `UserView` retornado deve continuar **idêntico** (mesma resposta JSON), só muda a FONTE. Sem quebrar o front.

### 2.2 Projeção da Fila em user_module_settings (write-side dos campos Fila)
- Ao criar/editar um usuário em `/operacao/usuarios`, gravar os campos **específicos da Fila** (storeIds; vínculo de consultor se houver) em `core.user_module_settings(module_id='queue')` no formato `{"storeIdsByAccount": {"<accountId>": ["<storeId>",...]}}` (mesmo formato que a 0133 usa).
- `employee_code`/`job_title` continuam em `core.users` (são identidade; já lá).
- Migration `0134_backfill_queue_user_settings.sql` SE faltar algum dado já não coberto pela 0133 (a 0133 já fez storeIds; só complete o que faltar — provavelmente nada além de garantir consistência). Idempotente.

### 2.3 Reverse-direction: create/update grava core (fim do gap)
- `/operacao/usuarios` create/update passa a gravar **também** `core.account_users` (membership) + `core.user_role_assignments` (papel core equivalente, via o mapeamento) — assim o usuário criado na operação aparece no `/manage/users`.
- **MANTENHA o dual-write legado** (`user_tenant_roles`/`user_store_roles`) por enquanto — é band-aid de transição, removido só no U4. Documente que é dual-write temporário.

### 2.4 Front: atualizar o LegacyMarker
- `web/app/pages/usuarios.vue` já tem `<LegacyMarker>` (ver `web/app/components/admin/LegacyMarker.vue`). Atualize o texto: a LEITURA agora é core; o que resta é o **dual-write** legado (band-aid até U4). Ex.: label "Leitura já em core.*; writes ainda dual-gravam no legado (band-aid até U4)".

### 2.5 Testes Go (obrigatório)
- Usuário **core-only** (core.account_users + role_assignment, SEM `user_tenant_roles`) **aparece** na listagem de `/operacao/usuarios` da account dele.
- Usuário existente (com legado) continua aparecendo com o MESMO shape (sem regressão).
- Create em `/operacao/usuarios` resulta em `core.account_users` criado (reverse-direction).
- Estilo dos testes: igual aos atuais do projeto (mock/seed in-memory; veja `consultants/http_test.go`, `account_guard_test.go`).

## 3. O que NÃO fazer
- **NÃO** dropar/alterar `user_tenant_roles`/`user_store_roles`/`user_platform_roles` (U4).
- **NÃO** remover o dual-write legado (U4).
- **NÃO** mexer em `password_hash` nem rodar seed que sobrescreva dados de usuário.
- **NÃO** mudar o shape do `UserView` (o front depende dele).

## 4. Regras a cumprir
- Banco: container `omni-postgres-1` porta `5432` (confirme `docker ps`). `account_id` sempre do Principal/header, nunca do body. Migration idempotente. Parâmetros posicionais (`$1`), sem SQL concatenado.
- Go: `gofmt`, máx 450 linhas/arquivo, camadas handler→service→repo, sem `_` em erro, scan nullable `*string`.
- Documentar: `back/internal/modules/users/AGENT.md` (nova fonte = core), `docs/LEGADO.md` (item 1: leitura migrada; resta dual-write), e marcar `umu-operacao-core` done em `web/app/components/roadmap/roadmap-data.ts`.
- Validar local: `go -C back build ./...`, `go -C back test ./...`, e teste manual (descreva curl/SQL): um usuário core-only aparece em `GET /v1/users?tenantId=<account>`.
- NÃO commitar/push/deploy.

## 5. Critérios de aceite (o engenheiro vai verificar)
1. `go -C back build ./...` e `go -C back test ./...` verdes (inclui os testes novos).
2. `GET /v1/users` (operação) de uma account lista usuários montados de `core.*` (não de `user_tenant_roles`/`user_store_roles`).
3. Um usuário **core-only** (sem `user_tenant_roles`) **aparece** na listagem da account dele.
4. Usuário criado em `/operacao/usuarios` passa a ter `core.account_users` (aparece no /manage/users).
5. Sem regressão: usuários legados existentes aparecem com o MESMO shape/role/storeIds.
6. Legado intacto (tabelas não dropadas; dual-write mantido).
7. `users/AGENT.md`, `LEGADO.md`, `roadmap-data.ts` e o LegacyMarker atualizados. Zero senha alterada.

## 6. Entrega
Resuma: arquivos alterados, a nova query/fonte, como reusou o mapeamento do `core_role_resolver`, migration 0134 (se houve), e a saída de `go test` + do teste manual (core-only aparece na operação). O engenheiro revisa contra a seção 5.
