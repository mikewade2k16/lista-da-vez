# Plano — Gate SSO do n8n/WAHA pelo login do Omni (forward_auth)

> Status: **BACKEND IMPLEMENTADO local em 2026-06-18** (`go build`/`vet` verdes, gateway.go
> sem issues de lint). Falta: teste local por curl, deploy (api rebuild + Caddy + .env) e
> ligar os subdomínios. Desenho aprovado em 2026-06-18.
> Objetivo: liberar `n8n.crowvisuals.com.br` e `waha.crowvisuals.com.br` pela URL,
> protegidos pelo **mesmo login do painel Omni**, sem senha à parte (sem basic_auth).

## 1. Decisões fechadas (2026-06-18)
- **WAHA:** atrás do gate Omni (`forward_auth`), só `platform_admin`. Obrigatório porque a
  API REST da WAHA é aberta (`WAHA_NO_API_KEY=true`).
- **n8n:** **SEM** gate Omni — fica com o **login próprio do n8n** (community não tem SSO
  externo nem dá pra desligar o login dele; gate em cima = login duplo, que o dono dispensou).
  ⚠️ **Antes de expor n8n publicamente, CRIE o owner do n8n** (via túnel SSH), senão o
  primeiro visitante cria a conta dona (land-grab).
- **Quem entra (WAHA):** só `platform_admin` (mike/maykell/tony). Validado contra o auth do Omni.
- **Interino:** nada exposto publicamente até estar pronto — n8n/waha seguem só por túnel SSH.
- **Onde valida (WAHA):** Caddy faz `forward_auth` → nossa API Go decide (200/302/403).

## 2. Por que precisa de código novo (o ponto central)
Hoje o `POST /v1/auth/login` devolve **só um Bearer token em JSON** (`accessToken`,
formato `ldv1.<payload>.<sig>`, HMAC) — o SPA guarda no localStorage. **Não há cookie.**

Quando o navegador abre **direto** `n8n.crowvisuals.com.br`, ele **não manda o Bearer**
(esse header só existe nas chamadas `fetch` do SPA). Logo, o gate precisa de um **cookie**
que o browser envie automaticamente ao subdomínio. Como `omni.` e `n8n.`/`waha.`
compartilham o domínio pai `crowvisuals.com.br`, um cookie com `Domain=.crowvisuals.com.br`
é enviado para os três.

## 3. Arquitetura
```
Browser → https://n8n.crowvisuals.com.br
  Caddy → reverse_proxy automation-n8n:5678        (SEM gate; login próprio do n8n)

Browser → https://waha.crowvisuals.com.br
  Caddy → forward_auth lista-api:8080 { uri /v1/auth/gateway/verify }
       ├─ 200 → reverse_proxy automation-waha:3000  (libera)
       ├─ 302 → https://omni.crowvisuals.com.br/auth/login   (sem sessão)
       └─ 403 → "sem permissão"                       (logado, não é platform_admin)
```
O webhook WAHA→n8n continua **interno** (`http://n8n:5678/webhook/webhook`), nunca passa
pelo Caddy — então nada disso afeta o fluxo do bot.

## 4. Mudanças de backend (Go) — módulo `auth` (IMPLEMENTADO)

> Decisão de implementação: tudo ficou no módulo **`auth`** (não `automation`), porque o
> gate valida a sessão do Omni + papel `platform_admin` (concern de auth) e precisa ficar
> **fora** do gating por módulo de `/v1/automation/*` (que exigiria `X-Account-Id`, que o
> navegador não manda no forward_auth). O `auth.Service` já vive ali — sem injeção extra.

### 4.1 Cookie de sessão no login — `auth/gateway.go` + `auth/http.go`
- `SetGatewayCookie` chamado em `POST /v1/auth/login` e `POST /v1/auth/invitations/accept`;
  `ClearGatewayCookie` em `POST /v1/auth/logout`.
- Cookie dedicado `omni_gw` = o próprio token HMAC. `HttpOnly; Secure; SameSite=Lax;
  Domain=AUTH_GATEWAY_COOKIE_DOMAIN; Expires=<exp do token>`.
- **Env nova:** `AUTH_GATEWAY_COOKIE_DOMAIN` (config.go) — vazio em dev (host-only);
  `.crowvisuals.com.br` em prod. Passada nos dois composes.
- Non-breaking: o SPA ignora o cookie e segue usando o Bearer do JSON.

### 4.2 Endpoint `GET /v1/auth/gateway/verify` — `auth/gateway.go`
- Registrado em `auth.RegisterRoutes` (público no roteamento; valida dentro do handler).
- Lógica: lê cookie `omni_gw` (fallback `Authorization` Bearer p/ curl) → `AuthenticateToken`
  (honra expiração + revogação em `core.user_sessions`) → checa `RolePlatformAdmin`.
  - sem/inválido → **302** `WEB_APP_URL/auth/login` (rota real do painel; `login.vue`
    só aceita `?redirect=` interno começando com `/`, então bounce-back cross-subdomínio
    fica como melhoria futura). Se `WEB_APP_URL` vazio (dev), responde **401** (curl-friendly).
  - papel ≠ platform_admin → **403**.
  - OK → **200** + header `X-Gateway-User: <email>`.
- Wiring: `app.go` passa `auth.GatewayConfig{CookieDomain: cfg.AuthGatewayCookieDomain,
  LoginURL: cfg.WebAppURL}` para `auth.RegisterRoutes`.

### 4.3 AGENT.md
- `back/internal/modules/auth/AGENT.md` atualizado (endpoint + cookie + `gateway.go`). ✅

## 5. Caddy (VPS — `/opt/omnichannel/Caddyfile`)
Anexar 2 blocos (modelo igual ao bloco `db.{$DOMAIN}` que já usa auth):
```caddy
# n8n: SEM gate (login próprio do n8n). ⚠️ criar o owner do n8n ANTES de expor.
n8n.crowvisuals.com.br {
    import secure_headers
    encode zstd gzip
    reverse_proxy automation-n8n:5678 {
        header_up Host {host}
        header_up X-Forwarded-Proto {scheme}
    }
}
# waha: COM gate Omni (API aberta exige). forward_auth -> nossa API decide.
waha.crowvisuals.com.br {
    import secure_headers
    encode zstd gzip
    forward_auth lista-api:8080 {
        uri /v1/auth/gateway/verify
        copy_headers X-Gateway-User
    }
    reverse_proxy automation-waha:3000 {
        header_up Host {host}
        header_up X-Forwarded-Proto {scheme}
    }
}
```
- `lista-api`, `automation-n8n`, `automation-waha` já são aliases na rede do proxy. ✅
- Editar **preservando inode** (`cat novo > Caddyfile`, não `sed -i`) — gotcha conhecido
  do bind-mount; depois `caddy validate` → `caddy reload`. Backup antes.

## 6. `.env.production` (VPS) — corrigir bloco AUTOMATION (hoje está em dev)
```
AUTOMATION_N8N_HOST=n8n.crowvisuals.com.br        # era localhost
AUTOMATION_N8N_WEBHOOK_URL=https://n8n.crowvisuals.com.br   # era http://localhost:15680
AUTOMATION_N8N_PROTOCOL=https                      # era http
AUTOMATION_N8N_SECURE_COOKIE=true                  # era false
AUTOMATION_WAHA_HOST=waha.crowvisuals.com.br       # era localhost
AUTH_GATEWAY_COOKIE_DOMAIN=.crowvisuals.com.br     # NOVA
```
Recriar n8n: `up -d n8n` (não mexer no `ENCRYPTION_KEY` → workflow/credenciais intactos).

## 7. DNS — ✅ FEITO (2026-06-18)
- `n8n.crowvisuals.com.br` → `85.31.62.33` ✅
- `waha.crowvisuals.com.br` → `85.31.62.33` ✅ (corrigido pelo dono; era `192.185.211.209`).

## 8. Plano de teste (local primeiro) — ✅ VALIDADO 2026-06-18
Local não tem Caddy, então o gate foi validado por curl contra a API local:
- `POST /v1/auth/login` → `Set-Cookie: omni_gw=...; HttpOnly; Secure; SameSite=Lax` ✅
- `verify` com cookie de platform_admin → **200** + `X-Gateway-User` ✅
- `verify` sem cookie / cookie inválido → **302** `http://localhost:3003/auth/login` ✅
- `go build ./...` + `go vet` verdes; `gateway.go` sem issues no golangci-lint.
- Pendente só de teste no navegador o caso 403 (usuário comum) — coberto pela lógica.

## 9. Deploy (ordem) — comandos devolvidos ao usuário ([[feedback_local_only]])
1. Backup `.env.production` e `Caddyfile` na VPS.
2. Subir código (api mudou → **rebuild**: `up -d --build api` [[feedback_backend_rebuild]]).
3. Ajustar `.env.production` (bloco §6) + `up -d n8n`.
4. **Criar o owner do n8n** pelo túnel SSH (`http://127.0.0.1:15680`) ANTES de expor —
   senão land-grab (n8n fica sem gate, §1).
5. Anexar blocos no Caddyfile (§5, preservando inode): n8n = reverse_proxy puro,
   waha = forward_auth → `caddy validate` → `caddy reload`.
6. DNS (§7) já feito.
7. Verificar:
   - `https://n8n.crowvisuals.com.br` → cai no **login do próprio n8n** (sem gate Omni).
   - `https://waha.crowvisuals.com.br` logado no Omni como admin → entra; deslogado → vai
     pro `/auth/login`; usuário comum → 403.

## 10. Segurança / riscos
- Cookie `HttpOnly+Secure+SameSite=Lax` + GET idempotente no verify → risco de CSRF baixo.
- `forward_auth` roda em toda request (inclui upgrade WebSocket do editor n8n) — o cookie
  vai junto, então o push do n8n funciona.
- O Caddyfile é **compartilhado** com o app vizinho (`omnichannel-mvp`): `caddy validate`
  antes do reload; se falhar, reload não derruba a config viva; rollback = restaurar backup.
- Não toca banco da Pérola nem `N8N_ENCRYPTION_KEY`.

## 11. Rollback
- Caddy: restaurar `Caddyfile.bak` + `caddy reload`.
- `.env.production`: restaurar `.env.production.bak` + `up -d n8n`.
- Código: o cookie é aditivo e o endpoint é novo; reverter = deploy do SHA anterior.
