# Módulo `tools` — Encurtador de Link + Gerador de QR Code

Duas ferramentas por conta, reconstruídas como back Go real (o projeto antigo era mock em
`globalThis` no BFF Nitro eliminado). Plano/decisões: `docs/tools/PLANO_MODULO_TOOLS.md`.

## Escopo
- **Encurtador**: `tools.short_links` → redirect público `GET /s/{slug}` (302 + `hits++`).
- **QR Code**: `tools.qr_codes` → redirect **rastreado** `GET /q/{slug}` (302 + `scan_count++`,
  `last_scanned_at`, só se `is_active`). O QR codifica o `qrUrl` (`/q/{slug}`), não a URL final —
  por isso o toggle Ativo e o contador de scans funcionam de verdade. A imagem PNG é gerada **no
  cliente** (lib `qrcode`) a partir de `qrUrl` + cores + `size`; o backend não renderiza/armazena PNG.

## Schema (migration `0197_tools_module.sql`)
- `tools.short_links(id, account_id→core.accounts, slug UNIQUE, target_url, hits, created_at, updated_at)`
- `tools.qr_codes(id, account_id→core.accounts, slug UNIQUE, target_url, fill_color, back_color,
  size, is_active, scan_count, last_scanned_at, created_at, updated_at)`

`slug` é **único global por tabela** (o redirect resolve sem X-Account-Id). Colisão no create/patch →
o store adiciona sufixo `-2/-3`… (loop atômico sobre a violação de unique `23505`). `/s` e `/q` são
tabelas distintas: o mesmo slug pode existir nas duas.

## Rotas
Painel (RequireAuth + gating de módulo `tools` no Chain via `/v1/tools`):
- `GET/POST /v1/tools/short-links`, `PATCH /v1/tools/short-links/{id}`, `DELETE /v1/tools/short-links/{id}`
- `GET/POST /v1/tools/qr-codes`, `PATCH /v1/tools/qr-codes/{id}`, `DELETE /v1/tools/qr-codes/{id}`

O `PATCH` do encurtador é parcial (`slug`/`targetUrl`; `nil` = não mexe) e alimenta a **edição
inline** da tabela no painel. Trocar o `slug` mantém o unique global (mesmo loop de sufixo `-2/-3` do
create) — atenção: muda o `shortUrl`, então links já divulgados com o slug antigo param de resolver.

Público (sem auth, fora do gating — prefixos `/s` e `/q` não estão em `moduleGatingRules`):
- `GET /s/{slug}` → 302 destino; `GET /q/{slug}` → 302 destino (se ativo).

O `Resolve` (redirect) só devolve o destino se a conta dona está `is_active` **e** o módulo `tools`
está habilitado para ela (`core.account_modules`) — igual ao `PublicLookup` do bio. Qualquer falha =
404 uniforme (`pgx.ErrNoRows`), sem vazar existência.

## Isolamento multi-tenant (defesa em profundidade)
`RequireAuthWithAccount` rejeitaria `platform_admin` (sem linha em `core.account_users`), então as
rotas usam `RequireAuth` + validação **no handler** (`scopeContext` em `http.go`):
- `platform_admin`: confia no `X-Account-Id` (troca de conta); `""` = todas as contas (list) /
  remove/edita por id em qualquer conta (mutations). No create pode mirar qualquer `accountId` do body.
- Usuário comum: exige `X-Account-Id`, valida `IsMember` contra `core.account_users`; escopo = essa
  conta; `accountId` do body é **ignorado** (nunca escreve em conta alheia). Delete/patch filtram
  `id = $1 and account_id = $2` → linha de outra conta vira 404.

Além disso toda query do store filtra por `account_id` (quando não-admin). `accountId` inexistente no
create cross-conta do admin → violação de FK `23503` → 400 `invalid_account`.

## Permissões (declaradas em `module.go`, sync no boot)
`tools.shortlinks.view/manage`, `tools.qr.view/manage`. Templates: `tools.manager`, `tools.viewer`.
A visibilidade do menu "Tools" continua na permissão de página `workspace.tools.view` (módulo core).

## Env
- `TOOLS_PUBLIC_BASE_URL` (opcional): base absoluta de `shortUrl`/`qrUrl`. Fallback:
  `PUBLIC_API_BASE_URL` (já usada por bio/cardapio); vazio = URL relativa (o front prefixa com apiBase).
- Em produção o reverse proxy precisa rotear `/s/*` e `/q/*` para a api (para o link funcionar ao
  ser clicado/escaneado).

## Arquivos
`module.go` (registry/permissions/Build) · `model.go` (DTOs + inputs) · `service.go` (normalização,
slug, publicBase, orquestração) · `store_short_links.go` (+ helpers `isUUID/tsString/
isUniqueViolation/isForeignKeyViolation`) · `store_qr_codes.go` · `http.go` (painel + `scopeContext`)
· `http_public.go` (`/s`,`/q`) · `errors.go` (sentinelas).

## Rebuild
Mudou Go → `docker compose up -d --build api` (restart não basta). Migration roda no boot.
