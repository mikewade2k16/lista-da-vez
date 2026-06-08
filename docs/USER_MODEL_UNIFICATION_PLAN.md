# Plano — Unificação do modelo de usuário (remover legado de papéis)

> Fonte de verdade desta workstream. Objetivo: **zero legado** — um usuário = uma linha em `core.users`, papéis 100% em `core.*`, config por módulo em `core.user_module_settings`, e os `user_*_roles` legados REMOVIDOS. Regra: [AGENT_RULES.md → Legado](../AGENT_RULES.md), registro em [LEGADO.md](LEGADO.md).

## Contexto

A identidade já foi unificada (`public.users` → VIEW sobre `core.users`, 2026-06-05). Mas o **modelo de papel** continua DUPLO:
- Legado: `user_tenant_roles`/`user_store_roles`/`user_platform_roles` — usado pelo **auth login** (`LoadUserForAuth`) e por `/operacao/usuarios`.
- Core: `core.account_users` + `core.user_role_assignments` + `core.role_permissions` — usado por `/manage/users` + guard de módulos.

Hoje o manage-create grava nos **dois** (band-aid). Login e operação ainda dependem do legado. Isto NÃO está pronto.

## Decisões (2026-06-05)

- **Ritmo:** planejar primeiro; começar pelo seguro. O passo crítico (auth ler de core) entra depois, planejado e testado isoladamente — mexe no login de todos.
- **Config por módulo:** tabela dedicada **`core.user_module_settings (user_id, module_id, config jsonb)`** (PK composta). Cada módulo escreve só a sua linha; não incha `core.users`.

## Estágios (ordem = risco crescente)

### U1 — Front legacy-marker (SEGURO, primeiro) — `core.user_module_settings`
- Componente reutilizável (badge/aviso "LEGADO"/"MOCK"/"não persiste") visível **só para `platform_admin`**, plugado nas telas que dependem de legado (começar por `/operacao/usuarios`). Cumpre a regra "mostrar no front".
- Migration `core.user_module_settings (user_id uuid fk core.users, module_id text, config jsonb, pk(user_id,module_id))`.

### U2 — Auth resolve papel de `core.*` (KEYSTONE, planejar+testar isolado)
- `LoadUserForAuth` passa a montar `role`/permissões a partir de `core.account_users` + `core.user_role_assignments` (+ fallback legado durante transição, atrás de flag).
- Testes Go cobrindo: user core-only loga; papel/permissões corretos; platform_admin via `core.users.is_platform_admin`.
- Critério: um user criado só no core (sem `user_tenant_roles`) loga e tem o acesso certo.

### U3 — `/operacao/usuarios` lê de `core.*` + projeção da Fila
- Users module lista de `core.account_users` (scoped à account) + `core.user_module_settings` (opções da Fila: employee_code, store, link consultor).
- Mover os campos Fila-específicos (hoje em colunas/legado) para `core.user_module_settings(module_id='queue')`.

### U4 — Migrar os DEMAIS leitores/escritores + DROPAR legado
Auditoria (2026-06-05) achou que `auth` + `users` não eram os únicos: `crm/erp` (scope), `queue/settings`, `stores`, `tenants` ainda LEEM as tabelas legadas, e `consultants`/`stores`/`bootstrap` ESCREVEM. Dropar antes de migrar esses quebraria tudo. Então:

#### U4a — Migrar os leitores de escopo pra `core.*` (PARALELO)
Mesmo padrão do U3 (ler core, dual-write na escrita). 4 módulos independentes, divididos em 2 lotes paralelos:
- **Lote A (Claude):** `crm/erp` (scope/autorização) + `queue/settings`.
- **Lote B (Codex):** `stores` + `tenants`. Prompt: `docs/codex/UMU_U4a_STORES_TENANTS_CORE.md`.
- Arquivos compartilhados (`roadmap-data.ts`, `LEGADO.md`, este plano) consolidados pelo Claude para evitar conflito.

#### U4b — Remover dual-write legado
- Tirar os writes legados de `users`, `core/admin` (band-aid), `queue/consultants`, `stores`, `bootstrap_owner`. A partir daqui, escrita só em core.

#### U4c — DROP (Claude faz direto; destrutivo)
- Backup + grep de zero-usos de `user_tenant_roles`/`user_store_roles`/`user_platform_roles` no código.
- Dropar as 3 tabelas legadas (com backup, idempotente, reversível). Flag `AUTH_ROLES_SOURCE=core`.

## Notas
- Cada estágio: backup antes de tocar dados; validar local; atualizar [LEGADO.md](LEGADO.md) (status) + AGENT.md + roadmap-data.
- Nada de sobrescrever senha/dados sem permissão (regra).

## Notas de Deploy
- **[PENDENTE no deploy de prod]** O U4c **dropou** as tabelas `user_*_roles` (migration 0135) e o auth roda em `AUTH_ROLES_SOURCE=core`. No deploy de prod:
  1. Garantir que a migration 0133 (backfill legado→core) **já rodou** ANTES da 0135 (drop) — a ordem das migrations garante isso num banco que veio do legado.
  2. Declarar `AUTH_ROLES_SOURCE=core` em `docker-compose.prod.yml` (`environment`) + `.env.production` na VPS. **NUNCA** `legacy`/`core_with_fallback` em prod pós-drop (as tabelas não existem mais → login quebra). O default do compose já é `core`.
  3. Migration 0135 é idempotente (DROP IF EXISTS) — segura para rodar no boot.
  - **Apagar esta nota depois de aplicado no deploy de prod.**
