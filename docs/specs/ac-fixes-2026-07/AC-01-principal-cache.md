# AC-01 — Ligar o PrincipalCache (auth deixa de ir ao banco a cada request)

> Spec de implementação. Achado canônico **AC-01** (P0, esforço S, impacto alto) de
> `scratchpad/fatos.json` → `achados_canonicos.AC-01`.
> Branch: `refactor/multitenant-complete`. Backend Go em `back/`.

---

## 1. Contexto

**Achado:** o `PrincipalCache` está implementado e testado, mas **nunca foi ligado**:

- `back/internal/platform/httpapi/principal_cache.go:24` — struct genérica `PrincipalCache[T]` pronta,
  com `Get/Set/InvalidateSession/InvalidateUser/InvalidateAll/Cleanup` (145 linhas, thread-safe).
- `back/internal/platform/httpapi/principal_cache_test.go` — 8 testes unitários já passam.
- `back/internal/modules/auth/service.go:12-15` — interface `PrincipalCacheStore` (Get/Set) e campo
  `principalCache` no `Service`; `SetPrincipalCache` em `:59-61`. **Zero chamadas a `SetPrincipalCache`
  fora de testes** (verificado por grep).
- `back/internal/modules/auth/AGENT.md:97` — confirma: *"PrincipalCache NAO ligado"*.

**Custo hoje** — `AuthenticateToken` (`auth/service.go:132-194`) roda em TODA request autenticada e faz
**~3 round-trips ao banco por request**:
1. `sessions.IsRevoked` (`auth/sessions.go:51`) — SELECT em `core.user_sessions`;
2. `users.LoadUserForAuth` — user + papel + escopo via `core.*`;
3. `permissions.ResolveUserPermissions` — permissões efetivas v1.

Pior: em rotas gateadas por módulo, o bypass do `AccountModulesGuard`
(`back/internal/platform/app/app.go:374-377`) chama `authService.Authenticate` **de novo** para checar
`platform_admin` — ou seja, até **2× o custo acima por request**. O cache corta os dois caminhos.

**Correção factual do diagnóstico:** o fatos.json/briefing sugere invalidar pelos "eventos realtime já
existentes `user.session.revoked` / `role.permissions.changed`". **Esses eventos NÃO existem no código**
(grep em `back/` só encontra as menções em comentário/AGENT.md). O único evento real no bus é
`account.modules.changed` (`core/admin_service.go:139` → subscribe em `app.go:398`), e o bus
(`events.NewInMemoryBus`) só é criado dentro do bloco `if cfg.CoreV2Enabled` (`app.go:317`).
**Decisão desta spec:** invalidação **direta e síncrona** via interface (setter injection), não via bus —
segurança não pode depender de flag de feature nem de handler cujo erro é engolido/logado
(`events/bus.go:141-151`). Não criar eventos novos no bus (fica como evolução junto com AC-08/Redis).

---

## 2. Objetivo e não-objetivos

**Objetivo:** ligar o `PrincipalCache` no boot com TTL curto configurável, com invalidação síncrona
imediata nos pontos que mudam sessão/papel/permissão/ativação, métrica de hit rate em log e testes.

**Não-objetivos (explicitamente FORA):**
- NÃO mover cache para Redis nem tocar em rate-limit/module-guard (isso é AC-08).
- NÃO criar refresh token, mudar TTL do token JWT ou mexer em `AUTH_TOKEN_TTL` (AC-10).
- NÃO criar eventos novos no bus (`user.session.revoked` etc.) nem tabela de outbox.
- NÃO revogar sessões em troca de senha / reset de senha (comportamento atual não revoga; mudar isso é
  feature nova — regra "não remover/alterar funcionalidade para resolver problema").
- NÃO passar a chamar `sessions.Touch` (hoje `Touch` não é chamado em lugar nenhum; manter).
- NÃO refatorar `app.go` (já tem 559 linhas, preexistente); a nova lógica de wiring vai em arquivo novo.
- NÃO mexer no front (o comportamento HTTP externo não muda, exceto latência menor).
- NÃO tocar em migrations/banco — este AC é 100% código Go em memória.

---

## 3. Mudanças (passo a passo)

### Passo 1 — Config: TTL configurável `AUTH_PRINCIPAL_CACHE_TTL`

**Arquivo:** `back/internal/platform/config/config.go`

1. No struct `Config` (bloco de auth, após `AuthPasswordResetTTL` na linha ~67), adicionar:
   ```go
   AuthPrincipalCacheTTL time.Duration
   ```
2. Em `Load()` (junto do `AuthTokenTTL` na linha ~166), adicionar:
   ```go
   AuthPrincipalCacheTTL: getEnvDuration("AUTH_PRINCIPAL_CACHE_TTL", 30*time.Second),
   ```

**Decisão de TTL = 30s (default), e por quê:**
- A revogação de sessão (logout) e as mudanças de papel/permissão/ativação têm **invalidação síncrona
  no mesmo processo** (passos 3-6) — o TTL NÃO é a defesa primária.
- O TTL é o **teto de exposição** apenas para caminhos sem invalidação direta: (a) a corrida
  Set-após-Logout descrita no Passo 3; (b) mudanças feitas direto no banco (SQL manual); (c) futuro
  multi-instância antes do AC-08. 30s mantém esse teto curto.
- Ganho já é quase total em 30s: um usuário ativo a 1 req/s passa de ~3 queries/request para 1 rajada
  de ~3 queries a cada 30s (hit rate > 96%). Dobrar para 60s dobraria a exposição por ganho marginal.
- **`0s` desliga o cache** (comportamento atual preservado, rollback operacional sem rebuild).

### Passo 2 — httpapi: contadores de hit/miss no cache

**Arquivo:** `back/internal/platform/httpapi/principal_cache.go` (145 → ~185 linhas; teto 450 ok)

1. Importar `sync/atomic`. No struct `PrincipalCache[T]` (linha 24), adicionar campos:
   ```go
   hits   atomic.Int64
   misses atomic.Int64
   ```
2. Em `Get` (linha 42): incrementar `c.misses` em todo retorno `false` (sessionID vazio, não encontrado,
   expirado) e `c.hits` no retorno `true`.
3. Adicionar dois métodos novos ao final do arquivo:
   ```go
   // Stats retorna contadores cumulativos desde o boot (nao resetam).
   func (c *PrincipalCache[T]) Stats() (hits, misses int64) {
       return c.hits.Load(), c.misses.Load()
   }

   // Len retorna o numero de entradas atualmente no cache (inclui expiradas
   // ainda nao varridas pelo Cleanup).
   func (c *PrincipalCache[T]) Len() int {
       c.mu.RLock()
       defer c.mu.RUnlock()
       return len(c.bySession)
   }
   ```

**Arquivo:** `back/internal/platform/httpapi/principal_cache_test.go` — adicionar 1 teste:
```go
func TestPrincipalCache_Stats(t *testing.T) {
    cache := httpapi.NewPrincipalCache[testPrincipal](2 * time.Minute)
    cache.Set("s1", "u1", testPrincipal{UserID: "u1"})
    cache.Get("s1")          // hit
    cache.Get("nao-existe")  // miss
    hits, misses := cache.Stats()
    if hits != 1 || misses != 1 {
        t.Fatalf("esperado 1 hit / 1 miss, veio %d/%d", hits, misses)
    }
    if cache.Len() != 1 {
        t.Fatalf("esperado Len=1, veio %d", cache.Len())
    }
}
```

### Passo 3 — auth: interface com invalidação + Logout derruba a entrada

**Arquivo:** `back/internal/modules/auth/service.go`

1. Estender a interface (linhas 12-15) — `*httpapi.PrincipalCache[auth.Principal]` já satisfaz tudo:
   ```go
   type PrincipalCacheStore interface {
       Get(sessionID string) (Principal, bool)
       Set(sessionID, userID string, p Principal)
       InvalidateSession(sessionID string)
       InvalidateUser(userID string)
   }
   ```
2. Em `Logout` (linhas 115-121), invalidar **depois** do revoke no banco (ordem importa: DB primeiro,
   para que um miss concorrente já leia `revoked_at` preenchido):
   ```go
   func (service *Service) Logout(ctx context.Context, principal Principal) error {
       if service.sessions == nil || strings.TrimSpace(principal.SessionID) == "" {
           return nil
       }
       if err := service.sessions.Revoke(ctx, principal.SessionID); err != nil {
           return err
       }
       if service.principalCache != nil {
           service.principalCache.InvalidateSession(principal.SessionID)
       }
       return nil
   }
   ```
3. Atualizar o comentário da linha 138 ("TTL 2min") para "TTL configurável via
   AUTH_PRINCIPAL_CACHE_TTL (default 30s)".

**Corrida conhecida (documentar no AGENT.md, não "resolver"):** request A tem miss, lê
`IsRevoked=false`, e ANTES do `Set` (linha 190) o Logout revoga+invalida; o `Set` de A repovoa a entrada
já revogada. Janela de milissegundos; exposição máxima = TTL (30s). Aceito por decisão desta spec
(tombstone fica para a versão Redis/AC-08).

### Passo 4 — access: invalidar em mudança de matriz de papel / overrides

**Arquivo:** `back/internal/modules/access/service.go`

1. Adicionar interface local + campo + setter (padrão idêntico ao `ContextPublisher` das linhas 17-30):
   ```go
   // PrincipalCacheInvalidator derruba Principals cacheados quando permissao muda.
   // Definida aqui para nao importar httpapi (mesma razao do auth.PrincipalCacheStore).
   type PrincipalCacheInvalidator interface {
       InvalidateUser(userID string)
       InvalidateAll()
   }
   ```
   Campo `principalCache PrincipalCacheInvalidator` no struct `Service` (linha 11) e
   `func (service *Service) SetPrincipalCacheInvalidator(cache PrincipalCacheInvalidator)`.
2. `UpdateRolePermissions` (linha 74): após `ReplaceRolePermissions` bem-sucedido (linha 83) e antes do
   `publishRoleMatrixUpdate`, chamar:
   ```go
   if service.principalCache != nil {
       service.principalCache.InvalidateAll()
   }
   ```
   **Decisão:** `InvalidateAll` (não por usuário) porque a matriz v1 é por papel-coarse e o cache não
   indexa por papel; é operação administrativa rara e o custo de repovoar é 1 rajada de queries.
3. `UpdateUserOverrides` (linha 121): após `ReplaceUserOverrides` bem-sucedido (linha 139):
   ```go
   if service.principalCache != nil {
       service.principalCache.InvalidateUser(subject.UserID)
   }
   ```

### Passo 5 — users: invalidar em update (papel/escopo/ativação) e arquivamento

**Arquivo:** `back/internal/modules/users/service.go`

1. Mesma interface local (só `InvalidateUser` é necessário aqui):
   ```go
   type PrincipalCacheInvalidator interface {
       InvalidateUser(userID string)
   }
   ```
   Campo `principalCache PrincipalCacheInvalidator` no struct `Service` + setter
   `SetPrincipalCacheInvalidator`.
2. `Update` (linha 157): após `repository.Update` bem-sucedido (linha 246), antes do
   `publishContextEvents`, chamar `service.invalidatePrincipal(updated.ID)`.
3. `Archive` (linha 261): após `repository.Update` (linha 285), idem.
4. Helper nil-safe no fim do arquivo:
   ```go
   func (service *Service) invalidatePrincipal(userID string) {
       if service.principalCache != nil {
           service.principalCache.InvalidateUser(userID)
       }
   }
   ```
   Cobre: desativação (`Active=false`), troca de papel, troca de tenant/lojas — tudo passa por `Update`.

### Passo 6 — core (admin v2): Dependencies + RBAC/AdminUser invalidam

**Arquivo:** `back/internal/platform/modules/module.go` — no struct `Dependencies` (linha 111), após
`PasswordHasher`, adicionar campo **concreto** (nil quando cache desligado — evita typed-nil em
interface):
```go
// PrincipalCache permite aos modulos derrubar Principals cacheados quando
// papel/ativacao de usuario muda (AC-01). nil = cache desligado.
PrincipalCache *httpapi.PrincipalCache[auth.Principal]
```
(`modules` já importa `httpapi` e `auth` — sem import novo.)

**Arquivo:** `back/internal/modules/core/rbac_service.go`
1. Interface local + campo + setter no `RBACService` (linha 11):
   ```go
   type PrincipalCacheInvalidator interface {
       InvalidateUser(userID string)
       InvalidateAll()
   }
   ```
   `func (s *RBACService) SetPrincipalCacheInvalidator(cache PrincipalCacheInvalidator)`.
   (Interface definida UMA vez no pacote core — `admin_users_service.go` reusa.)
2. Invalidar `InvalidateUser(userID)` após sucesso em:
   - `AssignRoleToUser` (linha 131, após `s.rbac.AssignRoleToUser` ok);
   - `RemoveRoleFromUser` (linha 148, após `s.rbac.RemoveRoleFromUser` ok);
   - `SetUserRoles` (linha 162, após `ReplaceUserRoleAssignments` ok, linha 178).
   Motivo: `core.user_role_assignments` alimenta o papel-coarse/escopo do Principal via
   `auth/core_role_resolver.go`.
3. `DeleteRole` (linha 109): após delete ok, `InvalidateAll()` (pode afetar N usuários; sem índice
   papel→sessões).
4. **NÃO** invalidar em `UpdateRolePermissions` v2 (linha 56): `core.role_permissions` alimenta a RBAC
   por-account resolvida por request (`/v2/me/context`), que NÃO passa pelo Principal cacheado.
   Deixar comentário de 1 linha dizendo isso no método.

**Arquivo:** `back/internal/modules/core/admin_users_service.go`
1. Campo `principalCache PrincipalCacheInvalidator` no `AdminUserService` (linha 15) + setter
   `SetPrincipalCacheInvalidator`.
2. `UpdateUser` (linha 146): trocar o `return s.repo.UpdateUser(...)` (linha 204) por capturar o
   retorno, e se `err == nil && s.principalCache != nil` → `s.principalCache.InvalidateUser(userID)`
   antes de devolver. Cobre desativação/`IsPlatformAdmin` via painel admin v2.
3. `DeleteUser` (linha 210): após o soft-delete ok, `InvalidateUser(userID)`.

**Arquivo:** `back/internal/modules/core/module.go` — em `Build` (linha 185), após criar
`rbacSvc` (linha 188) e `adminUserSvc` (linha 203):
```go
if deps.PrincipalCache != nil {
    rbacSvc.SetPrincipalCacheInvalidator(deps.PrincipalCache)
    adminUserSvc.SetPrincipalCacheInvalidator(deps.PrincipalCache)
}
```

### Passo 7 — app: wiring + goroutine de manutenção/telemetria

**Arquivo NOVO:** `back/internal/platform/app/principal_cache_wiring.go` (~70 linhas)

```go
package app

// wirePrincipalCache liga o PrincipalCache (AC-01) quando AUTH_PRINCIPAL_CACHE_TTL > 0.
// Retorna nil quando desligado (0s) — callers fazem nil-check antes de injetar.
// Invalidacao e direta/sincrona (nao via bus): logout, access e users derrubam a
// entrada na hora; o TTL e so o teto de exposicao dos caminhos sem invalidacao.
func wirePrincipalCache(
    cfg config.Config,
    logger *slog.Logger,
    authService *auth.Service,
    accessService *access.Service,
    usersService *users.Service,
) *httpapi.PrincipalCache[auth.Principal] {
    if cfg.AuthPrincipalCacheTTL <= 0 {
        logger.Info("principal_cache_disabled", "ttl", cfg.AuthPrincipalCacheTTL.String())
        return nil
    }

    cache := httpapi.NewPrincipalCache[auth.Principal](cfg.AuthPrincipalCacheTTL)
    authService.SetPrincipalCache(cache)
    accessService.SetPrincipalCacheInvalidator(cache)
    usersService.SetPrincipalCacheInvalidator(cache)
    logger.Info("principal_cache_enabled", "ttl", cfg.AuthPrincipalCacheTTL.String())

    // Manutencao: Cleanup a cada 60s; hit rate logado a cada 5 min quando houve trafego.
    go func() {
        const statsEvery = 5
        ticker := time.NewTicker(time.Minute)
        defer ticker.Stop()
        tick := 0
        for range ticker.C {
            cache.Cleanup()
            tick++
            if tick%statsEvery != 0 {
                continue
            }
            hits, misses := cache.Stats()
            if total := hits + misses; total > 0 {
                logger.Info("principal_cache_stats",
                    "hits", hits,
                    "misses", misses,
                    "hit_rate_pct", float64(hits)*100/float64(total),
                    "entries", cache.Len(),
                )
            }
        }
    }()

    return cache
}
```
(Imports: `log/slog`, `time`, `config`, `httpapi`, `auth`, `access`, `users` — todos já usados pelo
pacote `app`, exceto conferir `access`/`users` que `app.go` já importa nas linhas 12 e 40.)

**Arquivo:** `back/internal/platform/app/app.go` — 2 edições pontuais:
1. Após a criação de `usersService` (linha 219), inserir:
   ```go
   // AC-01: cache de Principals com TTL curto + invalidacao sincrona.
   principalCache := wirePrincipalCache(cfg, logger, authService, accessService, usersService)
   ```
2. No literal `modules.Dependencies{...}` (linhas 383-390), adicionar o campo:
   ```go
   PrincipalCache: principalCache,
   ```
   (`principalCache` nil quando desligado — Passo 6 já faz nil-check no core.)

### Passo 8 — Teste de integração do fluxo no auth

**Arquivo NOVO:** `back/internal/modules/auth/service_principal_cache_test.go` (~150 linhas,
`package auth` — white-box, para montar `User` direto; importa `httpapi` só no teste, sem ciclo:
`httpapi` não importa `auth`).

Fakes mínimos com embedding de interface (só os métodos usados; os demais explodem se chamados):
```go
type fakeAuthUserRepo struct {
    UserRepository
    user      User
    loadCalls int
}
func (f *fakeAuthUserRepo) LoadUserForAuth(ctx context.Context, userID string) (User, error) {
    f.loadCalls++
    return f.user, nil
}

type fakeSessionRepo struct {
    SessionRepository
    revoked      map[string]bool
    revokedCalls int
}
func (f *fakeSessionRepo) IsRevoked(ctx context.Context, sessionID string) (bool, error) {
    f.revokedCalls++
    return f.revoked[sessionID], nil
}
func (f *fakeSessionRepo) Revoke(ctx context.Context, sessionID string) error {
    f.revoked[sessionID] = true
    return nil
}
```
Setup comum: `tokens := NewHMACTokenManager("test-secret", time.Hour)`;
`service := NewService(repo, nil, tokens, nil, nil, nil, nil)`;
`service.SetSessionRepository(sessions)`;
`service.SetPrincipalCache(httpapi.NewPrincipalCache[Principal](time.Minute))`;
token emitido com `tokens.Issue("sess-1", user)`.

Casos obrigatórios (nomes exatos):
1. `TestAuthenticateToken_SecondCallHitsCache` — duas chamadas a `AuthenticateToken`; assert
   `loadCalls == 1` e `revokedCalls == 1` (segunda foi hit) e principal igual nas duas.
2. `TestLogout_InvalidatesCachedSession` — autentica (povoa cache), `Logout`, autentica de novo →
   deve retornar `ErrUnauthorized` **imediatamente** (sem esperar TTL), e `revokedCalls` deve ter
   incrementado (miss forçado foi ao banco e viu revogado).
3. `TestAuthenticateToken_LegacyTokenSkipsCache` — token emitido com `Issue("", user)` (sem sid);
   duas chamadas → `loadCalls == 2` (cache nunca usado; comportamento legado preservado).
4. `TestAuthenticateToken_CacheDisabledKeepsCurrentBehavior` — service SEM `SetPrincipalCache`;
   duas chamadas → `loadCalls == 2`.

Ajustar os campos de `User` ao struct real de `model.go` (usar papel `RoleDirector` ou outro do
catálogo com escopo de tenant, `Active: true`).

### Passo 9 — Documentação (AGENT.md dos módulos tocados + env examples)

1. `back/internal/modules/auth/AGENT.md` — reescrever a linha 97: PrincipalCache **LIGADO** (data),
   TTL default 30s via `AUTH_PRINCIPAL_CACHE_TTL` (`0s` desliga), invalidação síncrona (logout →
   sessão; access/users/core → usuário; matriz v1/DeleteRole → all), corrida Set-após-Logout com teto
   = TTL, cache local ao processo (multi-instância = AC-08). Atualizar também a seção "Arquivos
   atuais" (`service.go`).
2. `back/internal/platform/httpapi/AGENT.md` — nota: cache ganhou `Stats()/Len()`; quem liga é
   `app/principal_cache_wiring.go`.
3. `back/internal/platform/modules/AGENT.md` — documentar o novo campo `Dependencies.PrincipalCache`.
4. `back/internal/modules/access/AGENT.md`, `back/internal/modules/users/AGENT.md`,
   `back/internal/modules/core/AGENT.md` — 2-3 linhas cada: onde invalidam e por quê.
5. `back/internal/platform/app/AGENT.md` e `back/internal/platform/config/AGENT.md` — wiring novo e
   env var nova.
6. `.env.production.example`, `.env.staging.example`, `.env.docker.example` — adicionar linha
   comentada:
   ```
   # Cache de Principal autenticado (AC-01). 0s desliga. Default 30s.
   # AUTH_PRINCIPAL_CACHE_TTL=30s
   ```

## Regras de execução (OBRIGATÓRIAS para o implementador)

- **NENHUM comando git** (sessão multi-agente — só o usuário roda git).
- **NÃO rodar npm/build/generate do web.** Validação do back: `docker compose up -d --build api`
  **PODE e DEVE** rodar (back/ mudou). `go test` do back também pode.
- Máx **450 linhas** por arquivo novo/refatorado (novos aqui: ~70 e ~150 linhas — ok). `app.go` já
  excede por herança: só as 2 inserções pontuais, sem refatorar.
- **Não remover funcionalidade existente**: token legado sem `sid` continua funcionando; `0s` restaura
  o comportamento atual; `sessions.Touch` continua não sendo chamado.
- Zero mock/legado novo; nenhuma migration (não tocar em banco).
- Go: **sem lib uuid externa**; scan nullable com `*string` (não aplicável aqui — sem SQL novo).
- **NUNCA sobrescrever password_hash/dados de usuário** — este AC não toca em dados.
- Portas fixas: api 9091 / web 3003 / postgres 5432.
- Atualizar os AGENT.md listados no Passo 9 ao final.
- Antes de encerrar, rodar `gofmt` nos arquivos tocados e garantir que `go vet ./...` (em `back/`) passa.

---

## 4. Critérios de aceite

1. `AUTH_PRINCIPAL_CACHE_TTL` existe no `Config` com default `30s`; `0s` desliga o cache e o boot
   loga `principal_cache_disabled`.
2. Com cache ligado, a **2ª** request autenticada da mesma sessão não executa `IsRevoked` nem
   `LoadUserForAuth` (provado por `TestAuthenticateToken_SecondCallHitsCache`).
3. **Logout invalida imediatamente**: request após logout leva 401 sem esperar TTL
   (`TestLogout_InvalidatesCachedSession`).
4. Token legado sem `sid` nunca usa o cache (`TestAuthenticateToken_LegacyTokenSkipsCache`).
5. `access.UpdateRolePermissions` → `InvalidateAll`; `access.UpdateUserOverrides`,
   `users.Update/Archive`, `core RBACService.AssignRoleToUser/RemoveRoleFromUser/SetUserRoles`,
   `core AdminUserService.UpdateUser/DeleteUser` → `InvalidateUser`; `core RBACService.DeleteRole` →
   `InvalidateAll`. Todos nil-safe (cache desligado não quebra nada).
6. Log periódico `principal_cache_stats` com `hits`, `misses`, `hit_rate_pct`, `entries` (a cada 5 min
   quando houve tráfego) + `Cleanup()` a cada 60s.
7. `go test` verde nos pacotes `httpapi`, `auth`, `access`, `users`, `core`, `app`, `config`.
8. `docker compose up -d --build api` sobe, `/healthz` responde, login+navegação funcionam.
9. AGENT.md dos módulos tocados e `.env*.example` atualizados.

## 5. Validação

```powershell
# 1. Testes unitários (na raiz do repo)
cd back
go vet ./...
go test ./internal/platform/httpapi/... ./internal/modules/auth/... ./internal/modules/access/... ./internal/modules/users/... ./internal/modules/core/... ./internal/platform/app/... ./internal/platform/config/...
cd ..

# 2. Rebuild da api (obrigatório — back/ mudou)
docker compose up -d --build api

# 3. Boot log confirma o cache ligado
docker compose logs api | Select-String "principal_cache"
# esperado: principal_cache_enabled ttl=30s

# 4. Smoke manual (credenciais: PEDIR AO USUÁRIO — nunca inventar):
#    a) login no painel (http://localhost:3003) e navegar 2-3 telas;
#    b) docker compose logs api | Select-String "principal_cache_stats"  (após ~5 min de uso);
#    c) logout → qualquer chamada seguinte com o token velho = 401 IMEDIATO (testar re-navegando);
#    d) desativar um usuário de teste no painel de usuários → sessão dele cai na hora (401).

# 5. Rollback operacional sem código: AUTH_PRINCIPAL_CACHE_TTL=0s no .env + docker compose up -d api
```

## 6. Notas de Deploy

- **Migrations:** nenhuma.
- **Env var NOVA (opcional):** `AUTH_PRINCIPAL_CACHE_TTL` — default `30s` embutido; só setar para
  ajustar/desligar (`0s`). Adicionada comentada nos `.env*.example`.
- **Rebuild:** sim — `docker compose up -d --build api` (dev) e nova imagem `omni-api` no próximo
  deploy (build local→GHCR→pull VPS, fluxo padrão).
- **Ordem:** só o deploy da api; sem dependência de banco/web. Rollback = imagem anterior ou
  `AUTH_PRINCIPAL_CACHE_TTL=0s`.
- **Atenção multi-instância:** cache e invalidação são locais ao processo. Hoje a VPS roda 1 instância
  de api — ok. Escalar horizontalmente exige AC-08 (Redis) antes.

## 7. Arquivos tocados

| Arquivo | Ação |
| --- | --- |
| `back/internal/platform/config/config.go` | editar (campo + parse env) |
| `back/internal/platform/httpapi/principal_cache.go` | editar (Stats/Len + contadores) |
| `back/internal/platform/httpapi/principal_cache_test.go` | editar (teste Stats) |
| `back/internal/modules/auth/service.go` | editar (interface + Logout) |
| `back/internal/modules/auth/service_principal_cache_test.go` | **criar** (4 testes) |
| `back/internal/modules/access/service.go` | editar (invalidator + 2 call sites) |
| `back/internal/modules/users/service.go` | editar (invalidator + 2 call sites) |
| `back/internal/platform/modules/module.go` | editar (Dependencies.PrincipalCache) |
| `back/internal/modules/core/rbac_service.go` | editar (invalidator + 4 call sites) |
| `back/internal/modules/core/admin_users_service.go` | editar (invalidator + 2 call sites) |
| `back/internal/modules/core/module.go` | editar (setters no Build) |
| `back/internal/platform/app/principal_cache_wiring.go` | **criar** (wiring + goroutine) |
| `back/internal/platform/app/app.go` | editar (2 inserções pontuais) |
| `back/internal/modules/auth/AGENT.md` | editar (linha 97 + arquivos) |
| `back/internal/modules/access/AGENT.md` | editar |
| `back/internal/modules/users/AGENT.md` | editar |
| `back/internal/modules/core/AGENT.md` | editar |
| `back/internal/platform/httpapi/AGENT.md` | editar |
| `back/internal/platform/modules/AGENT.md` | editar |
| `back/internal/platform/app/AGENT.md` | editar |
| `back/internal/platform/config/AGENT.md` | editar |
| `.env.production.example` | editar (linha comentada) |
| `.env.staging.example` | editar (linha comentada) |
| `.env.docker.example` | editar (linha comentada) |
