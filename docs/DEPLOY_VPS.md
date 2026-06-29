# Deploy na VPS

Doc **único** de deploy deste repositorio. Operacao do dia-a-dia, rollback, migrations,
arquitetura da VPS e procedimentos raros (primeiro go-live, ERP, release especial).

> Docs de apoio (deep-dive, so quando precisar): plano canonico do pipeline em
> [deploy/REGISTRY_STAGING_DEPLOY_PLAN.md](deploy/REGISTRY_STAGING_DEPLOY_PLAN.md) e setup de
> staging (Caddy/DNS) em [deploy/STAGING_SETUP.md](deploy/STAGING_SETUP.md). O antigo
> `DEPLOY_CHECKLIST.md` foi consolidado aqui em 2026-06-23.

---

## TL;DR — os 2 comandos (sem enrolacao)

**`npm run deploy:fast:prod`** -> builda na sua maquina e ja manda. Um comando, na hora.
Nao depende de git nem de CI. (Faz backup do banco; rollback disponivel.) **E' o do dia-a-dia.**

**`npm run deploy:prod`** -> nao builda nada. So puxa uma imagem que o CI ja construiu.
Por isso exige duas coisas, em ordem, com espera no meio:

```bash
# 1. sobe pro git
git push

# 2. ESPERA o CI (GitHub Actions) terminar de buildar e publicar — leva uns minutos.
#    Confere se ficou verde:
gh run list --workflow=build-images.yml --limit 1

# 3. SO DEPOIS de verde:
npm run deploy:prod
```

> NAO digite `git push` e `npm run deploy:prod` colados — o deploy:prod falha no pull se o
> CI ainda nao publicou a imagem daquele commit. Espere o CI ficar verde no meio.

Regra de ouro por tras dos dois: **a VPS nunca compila.** A imagem e' buildada uma vez (no CI
ou na sua maquina), publicada no GHCR (`ghcr.io/mikewade2k16/omni-{api,web}:<tag>`), e a VPS so
faz `docker pull` + `up -d --no-build`. (O build do Nuxt pede ~4GB de heap numa VPS de ~6GB —
compilar la com prod no ar = risco de OOM.) O `push`/`pull` so transfere as camadas que mudaram.

Diferenca entre os dois: ONDE a imagem e' buildada.

| | builda onde | precisa git push + CI? | backup | rollback |
|---|---|---|---|---|
| `deploy:fast:prod` | sua maquina | nao | sim | sim |
| `deploy:prod` | no CI | sim (push -> espera verde -> deploy) | sim | sim |

Atalhos uteis:

```bash
npm run deploy:fast:prod -- -Service api   # so a API (1a vez por tag use -Service both)
npm run deploy:staging -- -Tag sha-<40hex> # sobe um SHA em staging
npm run deploy:promote                     # promove a MESMA imagem do staging pra prod
```

---

## Rollback (voltar pra imagem anterior)

Qualquer deploy deixa a imagem salva no GHCR, entao rollback = so apontar a VPS pra uma tag
antiga (NAO rebuilda nada, e' instantaneo):

```bash
powershell.exe -ExecutionPolicy Bypass -File scripts/deploy/deploy-pull.ps1 -Environment prod -Tag <tag-anterior>
```

Como achar a `<tag-anterior>`:
- **No fim de cada deploy** o script imprime `imagem <tag> no ar` — anote a tag de antes.
- **Tag atual em prod:** `ssh -i ~/.ssh/gh_actions_omnichannel_vps deploy@85.31.62.33 "grep IMAGE_TAG /home/deploy/lista-atendimento/.env.production"`
- **Imagens ja baixadas na VPS:** `ssh ... "docker images ghcr.io/mikewade2k16/omni-api"`
- Tags do `deploy:fast` sao `local-<timestamp>`; tags do `deploy:prod` sao `sha-<commit>`.

---

## Migrations / mudancas no banco sobem sozinhas

A imagem da API roda `migrate up` no startup (`CMD ["sh","-c","migrate up && migrate
bootstrap-erp-store && api"]` em `back/Dockerfile`). Entao, ao subir uma imagem nova, toda
migration pendente em `back/internal/platform/database/migrations/` e' aplicada ANTES da API
ligar (cria tabela/coluna/indice automaticamente; o que ja rodou e' pulado via
`schema_migrations`). Pontos de atencao:

- A migration precisa estar NA IMAGEM: `deploy:fast:prod` builda dos arquivos locais (basta
  salvar o `.sql`); `deploy:prod` usa a imagem do CI (tem que **commitar/pushar** a migration).
- Migration que quebra = API nao sobe (o `&&` trava). O `-BackupDatabase` (padrao no `:prod`)
  e' o seguro pra restaurar. Teste local antes.
- Env var nova NAO viaja na imagem: adicione no `.env.production` **na VPS, manualmente**.
- O migrator roda o `.sql` INTEIRO, sem `goose Down` — escreva SQL plano e idempotente.

---

## Pre-requisitos (one-time, ja feitos no ambiente atual)

- `.env.production` (e `.env.staging`) ja existem na VPS, a partir dos `.example`. Os scripts
  NUNCA sobrescrevem o `.env` remoto — so atualizam a linha `IMAGE_TAG`.
- `docker login ghcr.io` **na VPS** (user `deploy`) com PAT classic `read:packages` — sem isso o
  pull de imagem privada da `unauthorized`. One-time:
  `ssh -t -i ~/.ssh/gh_actions_omnichannel_vps deploy@85.31.62.33 "docker login ghcr.io -u mikewade2k16"`
  e cola o token `ghp_...` no prompt `Password:` (nao e' a senha do site; e' o PAT).
- So pro `deploy:fast`: `docker login ghcr.io` **na sua maquina** com PAT `write:packages` (pra dar push).
- Docker Desktop aberto na sua maquina (pro build local do `deploy:fast`).

---

## Validacao pos-deploy

Os scripts ja rodam smoke test publico no fim. Checks manuais:

```bash
# publico + healthcheck
curl -I https://omni.crowvisuals.com.br
curl -I https://omni.crowvisuals.com.br/healthz

# headers de seguranca
curl -sI https://omni.crowvisuals.com.br \
  | grep -Ei 'strict-transport|x-content-type|x-frame|referrer|permissions|content-security'

# logs/containers (via SSH)
ssh -i ~/.ssh/gh_actions_omnichannel_vps deploy@85.31.62.33 \
  "cd /home/deploy/lista-atendimento && docker compose --env-file .env.production -f docker-compose.prod.yml ps && \
   docker compose --env-file .env.production -f docker-compose.prod.yml logs --tail=100 api"
```

Checks funcionais (browser, voce faz): login -> dashboard carrega -> uma operacao autenticada ->
WebSocket na Operacao -> DevTools sem erro de CORS/403. Com `platform_admin`, confirmar que
`GET /v1/settings?tenantId=<activeTenantId>` retorna 200 e o switcher de contas resolve (depende
do roteamento `/v2/*` no Caddy — ver abaixo).

Check externo opcional: `https://securityheaders.com/` apontando pra `https://omni.crowvisuals.com.br`.

---

## Arquitetura da VPS (contexto auditado)

VPS `85.31.62.33` (host `srv1507028`, Ubuntu 24.04 LTS, user de deploy `deploy`, ~6GB RAM).
Esta stack sobe isolada da outra que ja roda na VPS (`omnichannel-mvp`):

- `COMPOSE_PROJECT_NAME=omni` separa containers/rede/volumes; banco, volume de uploads e segredos
  (JWT, SMTP) sao proprios — **nao compartilhar** com a outra stack.
- caminho remoto prod: `/home/deploy/lista-atendimento`; staging: `/home/deploy/lista-atendimento-staging`.
- portas locais (so 127.0.0.1, pra debug): web `13003`, api `18080`; postgres nao e' publicado.
  Staging usa `13005`/`18082`. (A stack `omni-crow` usa `13004`/`18081` — conferir colisao com `docker ps`.)
- servicos desta stack: `postgres`, `api`, `web` (o compose tambem traz redis/waha/n8n da automacao).

### Proxy reverso (Caddy compartilhado)

Esta stack **nao** sobe proxy. O Caddy central da stack `omnichannel-mvp`
(`/opt/omnichannel/Caddyfile`) roteia o dominio publico **`omni.crowvisuals.com.br`** pros
aliases `lista-web`/`lista-api` (a stack conecta na rede externa `omnichannel-mvp_default` com
esses aliases). Bloco do host:

```caddy
omni.crowvisuals.com.br {
  header {
    Strict-Transport-Security "max-age=31536000; includeSubDomains"
    X-Content-Type-Options "nosniff"
    X-Frame-Options "SAMEORIGIN"
    Referrer-Policy "strict-origin-when-cross-origin"
    Permissions-Policy "geolocation=(), microphone=(), camera=()"
    Content-Security-Policy "default-src 'self'; img-src 'self' data: https:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self' wss://omni.crowvisuals.com.br;"
  }

  handle /v1/* { reverse_proxy lista-api:8080 }
  handle /v2/* { reverse_proxy lista-api:8080 }   # <- necessario p/ o switcher de contas
  handle /uploads/* { reverse_proxy lista-api:8080 }
  handle /healthz { reverse_proxy lista-api:8080 }
  handle { reverse_proxy lista-web:3003 }
}
```

Aplicar mudanca no Caddy:

```bash
cd /opt/omnichannel
docker compose -f docker-compose.yml -f docker-compose.prod.yml --profile channels --env-file .env.prod up -d caddy
```

**Armadilhas do Caddy (ja custaram tempo):**
- **`/v2/*` separado e' obrigatorio.** A accounts API (`/v2/me/accounts`, `/v2/me/context`) move o
  switcher de contas. Sem o bloco `/v2/*`, a conta ativa nao resolve pra agencia e somem do menu
  os itens `agencyOnly` (Clientes Web, Usuarios Admin, Organizations) — mesmo com dado certo no
  banco. `handle /v1/* /v2/* {` (dois paths no mesmo handle) e' INVALIDO; use dois blocos (ou um
  matcher nomeado `@api path /v1/* /v2/*`).
- O Caddyfile e' **bind-mount por inode**: `sed -i` troca o inode e o container continua lendo o
  arquivo antigo. Edite preservando o inode (`cat novo > Caddyfile`) + `caddy reload`, ou reinicie
  o container.

### Variaveis principais do `.env.production`

```bash
COMPOSE_PROJECT_NAME=omni
POSTGRES_DB=omni
POSTGRES_USER=omni
POSTGRES_PASSWORD=<senha-forte>
PROXY_NETWORK_NAME=omnichannel-mvp_default
PROXY_API_ALIAS=lista-api
PROXY_WEB_ALIAS=lista-web
WEB_APP_URL=https://omni.crowvisuals.com.br
NUXT_PUBLIC_API_BASE=https://omni.crowvisuals.com.br
NUXT_PUBLIC_API_WS_BASE=wss://omni.crowvisuals.com.br
NUXT_API_INTERNAL_BASE=http://api:8080
CORS_ALLOWED_ORIGINS=https://omni.crowvisuals.com.br
AUTH_TOKEN_SECRET=<segredo-longo-e-aleatorio>
IMAGE_TAG=<sha-ou-local-timestamp>   # gravado pelos scripts a cada deploy
```

---

## Deploy via GitHub Actions (alternativa aos scripts locais)

Mesmo fluxo de pull, disparado pelo CI em vez da sua maquina. Util pra rastreabilidade/rollback
por SHA sem usar Windows.

- `build-images.yml` — builda api/web no CI (com gate de testes) e publica no GHCR. Dispara no
  push de `main`/`refactor/multitenant-complete` ou `gh workflow run build-images.yml`.
- `deploy-vps.yml` — deploy por pull (`workflow_dispatch`). Inputs: `environment`, `image_tag`
  (vazio => `sha-<HEAD do git_ref>`), `git_ref`, `backup_database`, `force_recreate`,
  `skip_smoke_tests`. Secret necessario: `DEPLOY_VPS_SSH_KEY`.

```bash
gh workflow run deploy-vps.yml --repo mikewade2k16/lista-da-vez \
  -f environment=prod -f image_tag=sha-<40hex> -f backup_database=true
gh run list --repo mikewade2k16/lista-da-vez --workflow deploy-vps.yml --limit 1
```

---

## Primeiro go-live (bootstrap, so na 1a vez)

Em producao o backend pula as seeds demo (sem usuarios/senhas de exemplo). O primeiro acesso
precisa de bootstrap explicito do owner:

```bash
cd /home/deploy/lista-atendimento
docker compose --env-file .env.production -f docker-compose.prod.yml run --rm \
  -e BOOTSTRAP_TENANT_SLUG=whenthelightsdie \
  -e BOOTSTRAP_TENANT_NAME='When The Lights Die' \
  -e BOOTSTRAP_STORE_CODE=MATRIZ -e BOOTSTRAP_STORE_NAME='Loja Matriz' -e BOOTSTRAP_STORE_CITY='Aracaju' \
  -e BOOTSTRAP_OWNER_NAME='Owner Inicial' \
  -e BOOTSTRAP_OWNER_EMAIL='seu-email@dominio' \
  -e BOOTSTRAP_OWNER_PASSWORD='troque-essa-senha-agora' \
  api sh -lc 'migrate bootstrap-owner'
```

Cria/atualiza (idempotente) o tenant inicial, a primeira loja e o owner. A loja ERP `184` e'
criada/reativada pelo `migrate bootstrap-erp-store` que ja roda no startup (config por
`ERP_BOOTSTRAP_STORE_CODE`/`ERP_BOOTSTRAP_TENANT_SLUG`); se o `bootstrap-owner` rodar depois da
API subir, `restart api` pra disparar o bootstrap ERP.

---

## Backup minimo

- dump do PostgreSQL desta stack (os scripts de deploy fazem com `-BackupDatabase`; ficam em
  `/home/deploy/lista-atendimento/backups/`)
- volume `api_uploads`
- arquivo `.env.production`

---

## Procedimentos especiais (raros)

### Carga ERP por dump de banco

Quando os consolidados ja foram importados no Postgres local, NAO subir a pasta
`erp-source-local` pra VPS — gere e transfira so um dump das tabelas `erp_*` (`erp_sync_runs`,
`erp_sync_files`, `erp_item_raw`, `erp_customer_raw`, `erp_employee_raw`, `erp_order_raw`,
`erp_order_canceled_raw`, `erp_item_current`, `erp_export_outbox`).

```bash
# 1. dump local (data-only, custom)
mkdir -p tmp
docker compose --env-file .env.docker exec -T postgres pg_dump -U omni -d omni \
  -Fc --data-only --no-owner --no-privileges \
  -t public.erp_sync_runs -t public.erp_sync_files -t public.erp_item_raw \
  -t public.erp_customer_raw -t public.erp_employee_raw -t public.erp_order_raw \
  -t public.erp_order_canceled_raw -t public.erp_item_current -t public.erp_export_outbox \
  -f /tmp/erp_data.dump
docker cp omni-postgres-1:/tmp/erp_data.dump ./tmp/erp_data.dump

# 2. enviar pra VPS
scp -i ~/.ssh/gh_actions_omnichannel_vps ./tmp/erp_data.dump \
  deploy@85.31.62.33:/home/deploy/lista-atendimento/tmp/erp_data.dump

# 3. NA VPS: backup completo antes, depois truncate das erp_* + pg_restore
cd /home/deploy/lista-atendimento
container=$(docker compose --env-file .env.production -f docker-compose.prod.yml ps -q postgres)
docker cp tmp/erp_data.dump "$container:/tmp/erp_data.dump"
docker compose --env-file .env.production -f docker-compose.prod.yml exec -T postgres \
  psql -U omni -d omni -v ON_ERROR_STOP=1 -c \
  'truncate table erp_export_outbox, erp_item_current, erp_order_canceled_raw, erp_order_raw,
   erp_employee_raw, erp_customer_raw, erp_item_raw, erp_sync_files, erp_sync_runs;'
docker compose --env-file .env.production -f docker-compose.prod.yml exec -T postgres \
  pg_restore -U omni -d omni --data-only --no-owner --no-privileges \
  --single-transaction --exit-on-error /tmp/erp_data.dump

# 4. apagar os dumps temporarios (local, host remoto e container) depois de validar contadores
```

Se a loja `184` remota tiver outro UUID e nao tiver referencias, alinhar antes do restore
(`update stores set id='bbbbbbbb-...-bbbbbbbb0184'::uuid where tenant_id='aaaaaaaa-...-aaaaaaaaaaaa'::uuid and code='184'`).
Estado ERP em prod: `ERP_SOURCE_KIND=ftp`, `ERP_ALLOW_MANUAL_SYNC=true`,
`ERP_SYNC_AUTOMATIC_ENABLED=true` (intervalo 24h, hora 4 UTC).

### Release especial — multitenant-complete (ja executado em prod 2026-06-23)

Mantido como referencia. Release com 37 migrations (0100-0136), DROP irreversivel de tabelas
publicas e a armadilha 0124. Se for refazer num ambiente novo, a ordem obrigatoria e':

1. confirmar historico de migrations na VPS (`select version from schema_migrations order by version desc limit 5`)
2. **backup completo** (`-Fc`) — a 0136 dropa `public.{tenants,users,stores,consultants}`, sem backup nao ha rollback
3. checar divergencia de `password_hash` entre `public.users` e `core.users` (a 0136 vira view; hash stale tranca o owner) — reconciliar com `update core.users c set password_hash=p.password_hash from public.users p where p.id=c.id and p.password_hash<>c.password_hash`
4. subir a API (migrations rodam no startup) e acompanhar o log ate `server listening`
5. **reseed `core.account_modules`** (armadilha 0124: a seed roda antes do `SyncCatalog` que popula `core.modules`, insere 0 linhas; sem o reseed o guard de modulos bloqueia tudo):
   ```sql
   insert into core.account_modules (account_id, module_id, enabled)
   select a.id, m.id, true from core.accounts a cross join core.modules m
   where a.is_active = true on conflict (account_id, module_id) do nothing;
   ```
6. subir o frontend, smoke HTTP + browser

Rollback do banco (so enquanto o backup `.dump` existir): `stop api web` -> `dropdb`+`createdb`
-> `pg_restore --no-owner <backup>`.

---

## Fallback de emergencia (legado, sem npm)

Se o GHCR estiver indisponivel, os scripts `.ps1` legado continuam no repo (sem atalho npm),
mas buildam o Nuxt **na VPS** (lento, risco de OOM) — use so em ultimo caso:

- `scripts/deploy/deploy-vps-fast.ps1` / `deploy-vps-incremental.ps1` — tar + `up -d --build` na VPS.
- `scripts/deploy/deploy-ship.ps1` — build local + `docker save | ssh 'docker load'` (sem registry).

O caminho normal e' sempre GHCR (`deploy:fast:prod` / `deploy:prod`).
