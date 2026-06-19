# Runbook — Mostarda em produção (omni.crowvisuals.com.br)

> Pôr o cardápio Fase 2 + os dados/imagens do Mostarda na VPS, e ligar o front
> TAVOLA (já no ar em https://mostarda.crowvisuals.com.br/). Criado 2026-06-19.
> Regra: o dono roda os comandos de prod; este doc é a ordem exata.

## ⚠️ Ponto crítico: IDs são diferentes em prod
O seed/import LOCAL usa o id da conta Crow **local** (`80caf5d5-...`). Na VPS a
conta Crow tem **outro id**. Então os SQLs precisam ser re-gerados com o id de
prod (`$CROW_PROD`) ANTES de rodar. O caminho da imagem também leva o id da conta
(`/uploads/cardapio/$CROW_PROD/mostarda/<slug>.jpg`).

## Passo 0 — Deploy do código (com backup) — DONO
```powershell
npm run deploy:ship:prod
```
Builda api+web local (o working tree atual = todo o cardápio Fase 2) → `docker save`→`load` na VPS por SSH → **backup do Postgres** (gzip em `backups/`) → `up -d --no-build` → migrations (0153/0166/0167) rodam no startup → smoke em `/healthz`. Precisa do Docker Desktop rodando + a chave SSH em `~/.ssh/gh_actions_omnichannel_vps`.

## Passo 1 — Descobrir o id da conta Crow em prod
Na VPS (`/home/deploy/lista-atendimento`):
```bash
docker compose --env-file .env.production -f docker-compose.prod.yml exec -T postgres \
  psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -tA -c \
  "select id, name from core.accounts where name ilike '%crow%';"
```
Anote o id → é o `$CROW_PROD`. (Me mande esse id que eu re-gero os SQLs com ele.)

## Passo 2 — Habilitar o módulo cardápio na conta Crow (prod pula seed demo)
```bash
docker compose ... exec -T postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c \
  "insert into core.account_modules (account_id, module_id, enabled)
   values ('$CROW_PROD','cardapio',true)
   on conflict (account_id, module_id) do update set enabled = true;"
```

## Passo 3 — PUBLIC_API_BASE_URL (pras imagens absolutizarem certo)
No `.env.production` da VPS, garantir:
```
PUBLIC_API_BASE_URL=https://omni.crowvisuals.com.br
```
(O TAVOLA chama `omni.crowvisuals.com.br`; as imagens precisam absolutizar pra um host HTTPS alcançável. Depois `restart api`.)

## Passo 4 — Seed do Mostarda + 203 produtos (re-targetado p/ $CROW_PROD)
Eu re-gero `seed_mostarda_fase2.sql` e `import_mostarda.sql` com `$CROW_PROD` (e o
mesmo restaurant id `b1b1b1b1-...`). Rodar na VPS:
```bash
docker compose ... exec -T postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" < seed_mostarda_prod.sql
docker compose ... exec -T postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" < import_mostarda_prod.sql
```

## Passo 5 — Copiar as 203 imagens pro volume da VPS
As imagens (`C:/tmp/mostarda_jpg/*.jpg`) vão pro volume `api_uploads`, na pasta da
conta de prod. Via `tar` por SSH (igual fizemos local):
```bash
DIR=/app/data/uploads/cardapio/$CROW_PROD/mostarda
tar -cf - -C /c/tmp/mostarda_jpg . | ssh -i ~/.ssh/gh_actions_omnichannel_vps deploy@85.31.62.33 \
  "cd /home/deploy/lista-atendimento && docker compose --env-file .env.production -f docker-compose.prod.yml exec -T api sh -c 'mkdir -p $DIR && tar -xf - -C $DIR'"
```

## Passo 6 — Registrar o domínio do front
No painel (omni.crowvisuals.com.br) → estabelecimento Mostarda → aba **Domínios** →
adicionar host `mostarda.crowvisuals.com.br` (primário). Isso faz o resolve por host
do TAVOLA funcionar.

## Passo 7 — Testar
- Rápido (sem domínio): `https://mostarda.crowvisuals.com.br/?slug=<slug-do-mostarda>` (o `?slug=` sobrepõe o resolve).
- Por host: `https://mostarda.crowvisuals.com.br/` (depois do Passo 6).
- Conferir no público: `https://omni.crowvisuals.com.br/v1/public/restaurants/<slug>` deve devolver o cardápio + imagens absolutas em `https://omni.crowvisuals.com.br/uploads/...`.

## Notas
- CORS do `/v1/public/*` é `*` (no código) — o front em outro domínio chama sem problema.
- `<img>` cross-origin não precisa de CORS; as imagens carregam mesmo de domínio diferente.
- Se o slug do Mostarda em prod for diferente de `mk`, ajustar o `?slug=` e o seed.
