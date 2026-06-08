# DEMANDA CODEX — UMU/U4b (lote B): remover dual-write legado em `users`, `stores`, `consultants`

> Você é o **programador** (Codex). Trabalho **EM PARALELO com o Claude**, que está fazendo `core/admin_users_repository` + `bootstrap_owner` (lado auth). Fique **SÓ** em `users`, `stores`, `consultants`. **NÃO edite os arquivos compartilhados** (seção 3). O engenheiro revisa contra os critérios de aceite.
>
> **ATENÇÃO MÁXIMA:** este é o passo ANTES do DROP (U4c). Se um writer parar de gravar legado mas o write CORE estiver incompleto, **perde-se papel/loja do usuário**. Cada writer SÓ pode largar o legado depois que o core grava TUDO (membership + papel + storeIds). Valide isso com teste.

## 0. Contexto obrigatório
1. `docs/USER_MODEL_UNIFICATION_PLAN.md` (estágio U4b) + `docs/LEGADO.md` item 1.
2. **Padrões a reusar (NÃO duplicar):**
   - `back/internal/modules/users/core_assignments.go` → `upsertCoreAssignmentsTx` (grava `core.account_users` + `core.user_role_assignments` + `core.user_module_settings` storeIds) e `ensureCoreQueueRoleTx`. É o molde do que "core completo" significa.
   - `back/internal/modules/stores/core_scope.go` → `deleteCoreStoreScopeTx` (mexe em storeIdsByAccount).
   - `back/internal/modules/auth/core_role_resolver.go` → mapeamento papel↔core.
3. `AGENT_RULES.md` (Legado + Banco) e `docs/ENGINEERING_PRINCIPLES.md`.

## 1. Escopo (SÓ users + stores + consultants)

### 1.1 `users/store_postgres.go` — remover legado (core JÁ completo)
- A função de Create/Update já chama `upsertCoreAssignmentsTx` (core completo). Os blocos legados (`delete from user_platform_roles`/`user_tenant_roles`/`user_store_roles` + os `insert into ...` correspondentes, ~linhas 266–292) podem ser **removidos**.
- Confirme que `upsertCoreAssignmentsTx` cobre os 3 casos (platform_admin, tenant-scoped, store-scoped) e que LIMPA assignments antigos no update (idempotente). Se faltar algo, complete no core ANTES de remover o legado.

### 1.2 `stores/store_postgres.go` — remover legado (core JÁ completo)
- O `Delete` já chama `deleteCoreStoreScopeTx` (~linha 220). O `delete from user_store_roles` (~linha 214) pode ser **removido**.

### 1.3 `consultants/store_postgres.go` — ADICIONAR core, depois remover legado
- Hoje grava só `user_store_roles` (~418/426/511) ao vincular consultor↔user↔loja.
- **Adicionar** o write core (espelhando `core_assignments.go`/`core_scope.go`): quando o consultor tem `user_id`, garantir `core.account_users(account_id=tenant_id, user_id)` E inserir/mergear a loja em `core.user_module_settings(module_id='queue').config.storeIdsByAccount[tenant_id]`. Quando o consultor é desvinculado/arquivado, **remover** a loja do storeIdsByAccount daquele user (igual o `deleteCoreStoreScopeTx` faz por loja).
- Tratar `user_id` nulo (consultor sem usuário vinculado) → não escreve core (não tem user).
- Só DEPOIS de o core estar completo, **remover** os writes `user_store_roles`.
- Extraia a lógica core num arquivo próprio (`consultants/core_scope.go`) se passar de 450 linhas no store.

### 1.4 Testes Go (obrigatório — prova que não perdeu dado)
- `users`: criar/atualizar usuário → core tem membership+role(+storeIds) corretos e **nenhuma** linha em `user_*_roles`.
- `consultants`: vincular consultor com user → `core.user_module_settings` do user ganha a loja; desvincular → loja sai; **nenhum** write em `user_store_roles`.
- Sem regressão nos testes existentes.
- Estilo: igual aos testes atuais (mock/seed in-memory).

## 2. O que NÃO fazer
- **NÃO** dropar as tabelas legadas (U4c — Claude faz).
- **NÃO** tocar `core/admin_users_repository.go` nem `bootstrap_owner.go` (são do Claude).
- **NÃO** mexer em `password_hash` nem sobrescrever dados de usuário.
- **NÃO** mudar shape de resposta dos endpoints.

## 3. Limites do paralelismo (não conflitar com o Claude)
- **SÓ** mexa em: `modules/users/*`, `modules/stores/*`, `modules/queue/consultants/*` (+ seus AGENT.md).
- **NÃO edite** (Claude consolida): `web/app/components/roadmap/roadmap-data.ts`, `docs/LEGADO.md`, `docs/USER_MODEL_UNIFICATION_PLAN.md`. Liste no resumo o que mudou pra eu atualizar.
- Sem migration nesta etapa (é só código). Se achar que precisa, PARE e avise — provavelmente não precisa.

## 4. Regras
- Go: `gofmt`, máx 450 linhas/arquivo, camadas, sem `_` em erro, scan nullable `*string`. Banco: params `$1`, `account_id` do Principal.
- Validar local: `go -C back build ./...`, `go -C back test ./internal/modules/users/... ./internal/modules/stores/... ./internal/modules/queue/consultants/...`. NÃO commitar/push/deploy.

## 5. Critérios de aceite (o engenheiro verifica)
1. `go -C back build ./...` + testes (users, stores, consultants) verdes, incluindo os novos.
2. **Zero** `insert into`/`delete from user_*_roles` em `users`, `stores`, `consultants` (grep limpo).
3. `users` cria/atualiza com core COMPLETO (membership+role+storeIds) — teste prova.
4. `consultants` vincular/desvincular reflete em `core.user_module_settings` storeIds — teste prova; e garante `core.account_users`.
5. Sem regressão (usuários/consultores existentes).
6. Tabelas legadas **intactas** (nada dropado); só seus módulos + AGENT.md tocados; compartilhados intactos.
7. Resumo lista o que mudar em roadmap-data.ts/LEGADO.md.

## 6. Entrega
Resuma: arquivos alterados, como o core ficou completo em consultants, saída de `go test` + um teste manual (criar consultor com user → loja aparece no core; nenhuma linha legada). Engenheiro revisa contra a seção 5.
