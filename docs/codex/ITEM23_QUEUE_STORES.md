# DEMANDA CODEX — Itens 2/3 (lote Fila): migrar refs `stores`/`consultants`/`users` → canônico

> Você é o **programador** (Codex). Trabalho **EM PARALELO com o Claude** (que faz tenants/auth/crm/bootstrap). Fique **SÓ** nos arquivos da seção 1. **NÃO edite** os compartilhados (seção 3). O engenheiro revisa e faz o DROP final.

## 0. Contexto
- `docs/LEGACY_PUBLIC_REMOVAL_PLAN.md` (este trabalho). `docs/LEGADO.md` itens 2 e 3.
- Esquema: `public.stores`=VIEW sobre `queue.stores`; `public.consultants`=VIEW sobre `queue.consultants`; `public.users`=VIEW sobre `core.users`. São aliases — trocar o NOME nas queries é seguro (mesmos dados/colunas).
- `AGENT_RULES.md` + `docs/ENGINEERING_PRINCIPLES.md`.

## 1. Escopo — trocar nomes legados → canônico NESTES arquivos
Mapeamento: `stores`→`queue.stores`, `consultants`→`queue.consultants`, `users`→`core.users`.
- `back/internal/modules/queue/consultants/store_postgres.go` (consultants→queue.consultants; `users`→core.users; NÃO mexer no core_scope.go que já está certo)
- `back/internal/modules/stores/store_postgres.go` (inclui **WRITES**: `insert/update/delete stores`→`queue.stores`; `update consultants`→`queue.consultants`)
- `back/internal/modules/stores/scope_queries.go`
- `back/internal/modules/queue/operations/store_postgres.go`
- `back/internal/modules/queue/operations/relations_resolver.go`
- `back/internal/modules/queue/settings/store_postgres.go`
- `back/internal/modules/queue/reports/store_postgres.go`
- `back/internal/modules/queue/alerts/store_postgres_signals.go`

Em cada query SQL desses arquivos, substituir `from stores`/`join stores`/`into stores`/`update stores`/`delete from stores` por a forma `queue.stores` (idem consultants→queue.consultants, users→core.users). **NÃO** trocar `tenants` (é do Claude).

## 2. Cuidados
- **WRITES** (`stores/store_postgres.go`): `queue.stores` tem as mesmas colunas da view, então o insert/update/delete migram direto trocando o nome. Confirme com `go test ./internal/modules/stores/...`.
- NÃO mudar lógica nem shape de retorno — só o NOME da tabela na query.
- NÃO dropar nada (o Claude dropa as views no fim).
- NÃO tocar `tenants` (Claude), nem os core_scope.go/core_assignments.go (já canônicos).

## 3. Limites do paralelismo
- SÓ os arquivos da seção 1 (+ AGENT.md dos seus módulos).
- NÃO edite: `roadmap-data.ts`, `docs/LEGADO.md`, `docs/LEGACY_PUBLIC_REMOVAL_PLAN.md`, nem arquivos de `tenants/`, `auth/`, `crm/`, `users/`, `access/`, `platform/database/` (são do Claude). Liste mudanças pendentes no resumo.
- Sem migration (o Claude faz o drop).

## 4. Regras
- Go: gofmt, máx 450 linhas, sem `_` em erro, params `$1`. Validar: `go -C back build ./...` + `go -C back test ./internal/modules/queue/... ./internal/modules/stores/...`. NÃO commitar/push/deploy.

## 5. Critérios de aceite (o engenheiro verifica)
1. `go -C back build ./...` + testes (queue/*, stores) verdes.
2. Nos SEUS arquivos: **zero** `\b(from|join|into|update|delete from)\s+(stores|consultants|users)\b` com nome cru (grep limpo) — tudo `queue.stores`/`queue.consultants`/`core.users`.
3. WRITES de stores (create/update/delete loja) continuam funcionando (teste).
4. Sem regressão; shape de resposta idêntico.
5. `tenants` intacto (é do Claude); nada dropado.
6. Resumo lista o que mudar em roadmap-data.ts/LEGADO.md.

## 6. Entrega
Resuma arquivos alterados + saída de build/test. O engenheiro revisa, faz a parte dele (tenants/auth/crm/bootstrap), e então dropa as views.
