# DEMANDA CODEX — UMU/U4a (lote B): `stores` e `tenants` leem/gravam `core.*` em vez do legado

> Você é o **programador** (Codex). Trabalho **EM PARALELO com o Claude** (engenheiro), que está migrando `crm/erp` + `queue/settings` ao mesmo tempo. Para não conflitar, **fique SÓ nos módulos `stores` e `tenants`** e **NÃO edite os arquivos compartilhados** listados na seção 3. O engenheiro revisa contra os "Critérios de aceite".

## 0. Leia primeiro (contexto obrigatório)
1. `docs/USER_MODEL_UNIFICATION_PLAN.md` — estágio **U4a**. `docs/LEGADO.md` item 1.
2. **Padrão a seguir (JÁ FEITO no U3):** `back/internal/modules/users/core_projection.go` e `core_assignments.go` — mostram exatamente como ler membership/papel/storeIds de `core.*` e como gravar dual-write. **Copie esse padrão.** Reuse o mapeamento de `back/internal/modules/auth/core_role_resolver.go` (não duplique).
3. `AGENT_RULES.md` (Legado + Backend/Banco) e `docs/ENGINEERING_PRINCIPLES.md`.
4. Fontes core: `core.account_users` (membership), `core.user_role_assignments` + `core.roles` (papel), `core.user_module_settings(module_id='queue').config->'storeIdsByAccount'->'<accountId>'` (lojas do usuário). Tudo já backfillado pela 0133.

## 1. O problema
`stores` e `tenants` ainda fazem JOIN em `user_tenant_roles`/`user_store_roles` (LEGADO) para listar membros/escopo. Enquanto lerem do legado, não dá pra dropar as tabelas (U4c). Migrar a LEITURA pra `core.*` e a ESCRITA pra dual-write (core + legado), igual o U3 fez no users.

## 2. Escopo (SÓ stores + tenants)

### 2.1 `back/internal/modules/tenants/store_postgres.go`
- Linhas ~155/171/204/219 fazem `join user_tenant_roles utr ...` / `join user_store_roles usr ...` para contar/listar membros por tenant. Reescrever para vir de:
  - membros do tenant = `core.account_users WHERE account_id = <tenant_id>` (lembre: `core.accounts.id == public.tenants.id`).
  - papel = `core.user_role_assignments` + `core.roles.code` (mapeado p/ coarse via o resolver).
  - lojas do usuário = `core.user_module_settings(queue).config->'storeIdsByAccount'`.
- Manter o shape de retorno idêntico.

### 2.2 `back/internal/modules/stores/store_postgres.go` + `service.go`
- **Leitura** (joins ~311/346/407 em user_tenant_roles/user_store_roles): migrar pra `core.*` igual acima.
- **Escrita** (delete/insert em `user_store_roles`, ~214): passar a gravar **também** em core — `core.user_module_settings(queue).config.storeIdsByAccount[accountId]` (set/merge das lojas do usuário) + garantir `core.account_users`. **MANTER o write legado em paralelo (dual-write)** — não remover (é U4b/U4c).

### 2.3 Testes Go
- `stores` e `tenants`: usuário **core-only** (sem `user_tenant_roles`/`user_store_roles`) é contado/listado corretamente.
- Sem regressão: usuário legado continua igual.
- Estilo: igual aos testes atuais (mock/seed in-memory; veja `consultants/http_test.go`).

## 3. Limites do paralelismo (CRÍTICO — não conflitar com o Claude)
- **NÃO toque** em: `crm/erp/*`, `queue/settings/*`, `users/*`, `auth/*`, `core/admin_users_*` (são do Claude ou de outras etapas).
- **NÃO edite os arquivos compartilhados** (o Claude consolida pra evitar conflito): `web/app/components/roadmap/roadmap-data.ts`, `docs/LEGADO.md`, `docs/USER_MODEL_UNIFICATION_PLAN.md`. Em vez disso, **liste no seu resumo final** o que mudou pra eu atualizar esses 3.
- Pode/deve atualizar os AGENT.md DOS SEUS módulos: `back/internal/modules/stores/AGENT.md` e (se existir) `tenants/AGENT.md`.
- Se precisar de migration, use o número **0135** (o Claude usa 0136+). Provavelmente NÃO precisa (dados já estão em core pela 0133) — só reescrita de query.

## 4. O que NÃO fazer
- **NÃO** dropar `user_tenant_roles`/`user_store_roles`/`user_platform_roles` (U4c).
- **NÃO** remover dual-write legado (U4b/U4c).
- **NÃO** mexer em `password_hash` nem sobrescrever dados de usuário.
- **NÃO** mudar shape de resposta JSON dos endpoints.

## 5. Regras
- Banco: `omni-postgres-1` porta `5432`. `account_id` do Principal/header, nunca do body. Params posicionais `$1`. Migration idempotente.
- Go: `gofmt`, máx 450 linhas/arquivo, camadas, sem `_` em erro, scan nullable `*string`.
- Validar local: `go -C back build ./...`, `go -C back test ./internal/modules/stores/... ./internal/modules/tenants/...`, e descrever um teste manual (core-only listado em stores/tenants).
- NÃO commitar/push/deploy.

## 6. Critérios de aceite (o engenheiro verifica)
1. `go -C back build ./...` verde + testes de `stores`/`tenants` verdes.
2. `stores` e `tenants` não fazem mais JOIN em `user_tenant_roles`/`user_store_roles` na LEITURA (grep limpo nas queries de listagem).
3. Usuário core-only é listado/contado corretamente em stores e tenants.
4. Escrita de stores grava core (storeIdsByAccount) E mantém o legado (dual-write).
5. Sem regressão nos usuários legados.
6. Tabelas legadas intactas (nada dropado); só os AGENT.md dos SEUS módulos tocados; arquivos compartilhados intactos.
7. Resumo final lista o que mudar em roadmap-data.ts/LEGADO.md pro engenheiro consolidar.

## 7. Entrega
Resuma: arquivos alterados (só stores/tenants), as queries novas, e a saída de `go build`/`go test` + teste manual. Liste as mudanças pendentes em roadmap-data.ts/LEGADO.md. O engenheiro revisa contra a seção 6.
