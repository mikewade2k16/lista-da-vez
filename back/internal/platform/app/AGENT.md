# AGENT — platform/app

## Escopo

Pacote `back/internal/platform/app/`. Bootstrap do binário: cria pool, monta
config, instancia módulos (legados + Module Registry quando
`CORE_V2_ENABLED=true`), pluga middlewares globais (CORS, RequestID, RateLimit,
Logging) e retorna o handler HTTP final.

## Peças

- `app.go` — `BuildHandler(...)`: ponto de entrada. Cria stores (`tenants`,
  `users`, `stores`, `consultants`, `settings`, `auth`, etc.), services, HTTP
  handlers; quando `CORE_V2_ENABLED`, instancia `Registry` e roda
  `SyncCatalog` + `Build`.
- `context_http.go` — `/v1/me/context` legado.
- `*_adapter.go` — adapters finos entre módulos legados (ex:
  `operations_store_scope_adapter.go`).

## Estado real do multi-tenant — 2026-05-28

> **Aviso crítico**: o esqueleto multi-tenant existe em código mas **NÃO está
> efetivamente plugado no runtime**. Ver
> [docs/MULTITENANT_COMPLETION_PLAN.md](../../../../docs/MULTITENANT_COMPLETION_PLAN.md)
> para o plano da finalização.

Pontos onde o runtime ignora o que o ROADMAP marca como "done":

### `AccountModulesGuard` está descartado

[app.go:313](app.go#L313) faz literalmente:

```go
// Guard fica disponivel para modulos satelites a partir da Fase 6.
// Modulos do registry hoje (so o core) nao usam o guard porque o core
// e o ponto de descoberta dos modulos habilitados.
_ = httpapi.NewAccountModulesGuard(pool)
```

Consequência: nenhuma rota está gateada por `core.account_modules`. Habilitar
ou desabilitar um módulo no banco **não tem efeito** em runtime. A
[Fase 2 do ROADMAP](../../../../docs/ROADMAP.md) está marcada como `done`
no `roadmap-data.ts` mas funcionalmente é parcial.

### `core.account_modules` está vazio

Migrations `0100`-`0103` criaram o schema e seedaram `core.accounts` /
`core.users` a partir de `public.tenants` / `public.users`, mas
**`core.account_modules` nunca foi seedado**. Mesmo se o guard fosse
plugado, qualquer rota retornaria `403 module_disabled` porque nenhum
módulo está habilitado para nenhum account.

### `Principal.AccountID` ainda vem do legado

O middleware atual deriva `Principal.AccountID` do JWT v1 (`tenantId`), não
do header `X-Account-Id`. Endpoints `/v2/me/context` aceitam `accountId` na
query como atalho documentado em
[modules/core/AGENT.md](../../modules/core/AGENT.md). A regra inegociável de
`CONTRACT_FREEZE.md` (account_id só via Principal) está ativa só nas rotas
v2/me/*.

## Quando essas pendências saem daqui

Plano canônico:
[docs/MULTITENANT_COMPLETION_PLAN.md](../../../../docs/MULTITENANT_COMPLETION_PLAN.md)
seções **C1**, **C2**, **C3**. Branch alvo: `refactor/multi-tenant-complete`
(a criar depois do merge do snapshot atual para `main`).

Resumo do que muda aqui dentro:

1. Remover o `_ =` da linha 313, criando o guard como variável real e
   passando ao chain dos módulos satélites.
2. Trocar a derivação de `Principal.AccountID` para vir de `X-Account-Id` em
   toda rota multi-tenant, com validação de membership em `core.account_users`.
3. Quando módulos `queue`/`crm` virarem subpackages com `Module` interface
   (C4/C5 do plano), trocar o wiring manual dos módulos legados (`operations`,
   `alerts`, `erp`, etc.) por `registry.MustRegister(queue.New())` /
   `crm.New()`.

## Regras gerais ao mexer aqui

- Não introduzir feature flag nova sem registrar em [config/AGENT.md](../config/AGENT.md).
- Wiring manual de módulos legados continua válido **enquanto** não houver
  `Module` interface implementada para eles. Não criar adapters intermediários
  sem necessidade — vai ser jogado fora na C4/C5.
- Não importar nada de `web/server/` no Go. O BFF Nitro está marcado para
  remoção integral.

## Comportamento sem feature-flag

`CORE_V2_ENABLED=false` (default): nem o Registry nem `SyncCatalog` rodam.
Apenas wiring legado de módulos. Endpoints `/v2/me/*` não existem.

`CORE_V2_ENABLED=true`: Registry roda, `SyncCatalog` popula `core.modules` /
`core.permissions` / `core.role_templates`. Endpoints `/v2/me/*` ficam
disponíveis. Guard continua descartado (ver acima).

## Referências

- [docs/ROADMAP.md → "Estado real em 2026-05-28"](../../../../docs/ROADMAP.md)
- [docs/MULTITENANT_COMPLETION_PLAN.md](../../../../docs/MULTITENANT_COMPLETION_PLAN.md)
- [docs/CONTRACT_FREEZE.md](../../../../docs/CONTRACT_FREEZE.md)
- [platform/httpapi/AGENT.md](../httpapi/AGENT.md)
- [platform/modules/AGENT.md](../modules/AGENT.md)
- [modules/core/AGENT.md](../../modules/core/AGENT.md)
