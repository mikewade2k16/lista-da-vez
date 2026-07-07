# ADR 0002 — Remover BFF Nitro mock; frontend conversa direto com Go

- **Status:** Aceito — concluído (2026-07-02)
- **Data:** 2026-05-29
- **Decisores:** Mike Wade
- **Referência cruzada:** [MULTITENANT_COMPLETION_PLAN.md](../MULTITENANT_COMPLETION_PLAN.md) seção C7, C9, C14, C15, C17.

## Contexto

Durante o desenvolvimento inicial (pré-multitenant), o frontend Nuxt usava um BFF (Backend-For-Frontend) interno via Nitro server routes em `web/server/`. Esse BFF tinha:

- Repositórios in-memory hardcoded (clientes 101-106, produtos fake, leads de teste).
- Validações próprias paralelas às do Go.
- Adapter `useBffFetch` no frontend que decidia entre BFF mock ou API real por feature flag.
- Cerca de 1500 linhas de TypeScript replicando contratos do Go.

A ideia original era isolar o frontend do backend e permitir desenvolvimento UI sem depender da API Go. Na prática gerou:

1. **Drift constante de contratos.** Quando a tela de clientes admin foi reescrita, o BFF tinha IDs numéricos `coreTenantId: 101` enquanto o Go retornava UUIDs reais `aaaaaaaa-...`. Trocar um exigia reescrever o outro.
2. **Dados fake misturados com dados reais.** A página `/clientes` da fila puxava do Go (real), enquanto `/manage/clientes-web` puxava do BFF (mock). Usuários viam duas listas diferentes de "clientes".
3. **Validações divergentes.** Email aceito no Nitro era rejeitado no Go (e vice-versa) por causa de regex levemente diferentes.
4. **Custo de manutenção dobrado.** Toda mudança de endpoint precisava: Go + TS BFF + composable frontend = 3 lugares.

## Decisão

Remover totalmente o BFF Nitro. Frontend Nuxt conversa direto com a API Go via `$fetch` + `createApiRequest` (utilitário em `web/app/utils/api-client.ts`).

### Como ficou a "isolação"

A isolação real da API não vem de proxy intermediário — vem de:

- **CORS configurado no Go** (`CORS_ALLOWED_ORIGINS` no docker-compose).
- **JWT validado no Go** (middleware `auth.RequireAuth`).
- **Rate limiting no Go** (`httpapi.RateLimit` no boot do app).
- **Caddy à frente em produção** termina TLS + rate-limit por IP.

### Cenários onde BFF voltaria a fazer sentido

- Agregar respostas de múltiplos backends (microservices distintos).
- Esconder estrutura interna de uma API legada que não pode mudar.
- SSR com data-fetching server-side que precisa de credenciais não expostas ao browser.

Nenhum desses é o caso de Omni hoje (backend único, frontend SPA com cookie de auth).

### Para webhooks externos

Webhooks de leads/products (C17) vão direto para Go em `POST /v1/webhooks/{leads,products}/{sourceSlug}` com validação HMAC SHA-256. Caddy à frente faz TLS terminator + rate-limit. Nenhum BFF intermediário.

## Consequências

### Positivas

- ~1500 linhas de TypeScript removidas (`web/server/` inteiro + `useBffFetch`).
- Contratos definidos em um único lugar (Go DTOs → frontend types via normalize).
- Erros 4xx/5xx vêm direto do Go com `error_code` padronizado.
- Deploy mais simples: um único binário Go vs. Nitro + Go.

### Negativas

- Desenvolvedor frontend precisa subir o backend local (`docker compose up -d api`) para testar features. Mitigação: o Docker Compose dev sobe API + Postgres + frontend juntos.
- Mock para testes de UI exige `vi.mock` no Vitest do composable (em vez de chamar BFF que retornava dados conhecidos). Mitigação: padrão estabelecido em `useContextRealtime.test.ts`.

### Neutras

- Cookies de sessão precisam estar acessíveis no domínio do Go. Em desenvolvimento ambos rodam em `localhost`, problema resolvido. Em produção, Caddy serve API e front no mesmo domínio via `/v1/*` reverse-proxy.

## Histórico de remoção (cronológico)

1. **C7** (2026-05-29) — Removido diretório `web/server/` inteiro + `useBffFetch`.
2. **C8** (2026-05-29) — Removidos mocks de session/admin (`session-simulation.ts`, `useAdminSession.ts`, `useTenantRealtime.ts`).
3. **C9** (2026-05-29) — `useClientsManager` reescrito contra `/v1/admin/accounts` real.
4. **C14** (2026-05-29) — `useAdminUsersManager` novo contra `/v1/admin/users`.
5. **C15** (2026-05-29) — `useAdminOrganizationsManager` novo contra `/v1/admin/organizations`.
6. **C17** (2026-05-29) — `useLeadsManager` + `useProductsManager` reescritos contra `/v1/admin/{leads,products}` real (módulo Go novo `site`).
7. **AC-12** (2026-07-02) — Removido o resquício final: mock finance (`web/server/` inteiro, incluindo `financeMockStore.ts`). Módulo Go novo `finance` (migration `0187`, rotas `/v1/finance/*`); composables da layer finance migrados para `createApiRequest`; `LegacyMarker` retirado de `/finance`. `web/server/` deixou de existir.

## Referências

- [docs/MULTITENANT_COMPLETION_PLAN.md](../MULTITENANT_COMPLETION_PLAN.md)
- [docs/CONTRACT_FREEZE.md](../CONTRACT_FREEZE.md)
- [back/internal/modules/site/AGENT.md](../../back/internal/modules/site/AGENT.md) — exemplo de módulo novo construído pós-BFF
