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
Tambem reconcilia o profile `automation` em prod (`redis`/`waha`/`n8n`/`whisper`) e reimporta os
workflows versionados do n8n quando eles mudarem.

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
npm run deploy:fast:prod -- -ForceAutomationWorkflowImport  # reimporta n8n mesmo sem hash novo
npm run deploy:staging -- -Tag sha-<40hex> # sobe um SHA em staging
npm run deploy:promote                     # promove a MESMA imagem do staging pra prod
```

### Automation/n8n no deploy rapido

Desde 2026-07-09, o atalho `deploy:fast:prod` chama `deploy-fast.ps1 -DeployAutomation`.
**Desde 2026-07-13 tambem passa `-ForceAutomationWorkflowImport`** (import de n8n SEMPRE, sem
depender do hash): como sobe muita mudanca de n8n, o marker sha256 pulando o import ja deixou a
VPS rodando uma versao ANTIGA do workflow por dias (o Calendar Chat da VPS ficou sem a logica de
anotacao do mes, enquanto o dev/arquivo ja tinham). Na pratica, alem de `api`/`web`, ele:

- envia para a VPS somente `automation/export/workflow-*.json` (NAO envia
  `credentials.decrypted.json`);
- roda `docker compose --profile automation pull/up -d --no-build redis waha n8n whisper`;
- compara o hash dos workflows com `.deploy/automation-workflows.sha256` (com
  `-ForceAutomationWorkflowImport`, sempre trata como "mudou");
- faz backup dos workflows atuais em `backups/n8n/`, importa os JSONs **conferindo
  "Successfully imported" por arquivo** (import silencioso NAO grava o marker), mantem ativos
  `calendaromni0001`, `calendarchat0001`, `calendartrans001` e `omnichatmvp00001`, preserva
  qualquer outro workflow que ja estava ativo antes do import, reinicia o n8n;
- **VERIFICA pos-restart** que cada workflow no banco do n8n (via `export:workflow`, que aplica o
  WAL) ficou identico aos `nodes` do arquivo versionado; se algum divergir ou algum import falhar,
  **aborta (exit 1) e NAO grava o marker** — o proximo deploy re-tenta em vez de "marcar como feito".
  So grava o hash novo quando import E verificacao passam 100%.

O n8n vive no `docker-compose.prod.yml` (`profiles: ["automation"]`); os comandos do deploy usam
`-f docker-compose.prod.yml`, entao enxergam o servico. Rodar `docker compose ... n8n` SEM esse `-f`
(so o `docker-compose.yml`) NAO ve o n8n (retorna vazio) — foi o que mascarou o diagnostico.

**Auto-export dos workflows antes do envio (OBS-08, desde 2026-07-13):** como o n8n guarda os
workflows no PROPRIO banco (dev), `automation/export/*.json` pode estar ATRAS do que voce editou no
n8n. Por isso, sob `-DeployAutomation`, o `deploy-fast.ps1` roda `n8n-export.ps1 -Sync` **ANTES** de
buildar/enviar: se o n8n dev estiver a frente, ele **auto-exporta** os workflows divergentes e SEGUE
(nunca trava), para o deploy levar SEMPRE a versao atual sem passo manual. Detalhes:

- So roda se o container n8n dev (`omni-n8n-1`) estiver up. Se estiver down, imprime
  `AVISO: n8n dev fora; ... deploy seguira com os arquivos versionados atuais` e segue (nunca zera).
- O `-Sync` pode deixar `automation/export/*.json` **modificados na working tree** (NAO commitados). O
  deploy os USA mesmo sem commit (deployar antes de commitar); o **dono commita depois**.
- Pular o gatilho: `npm run deploy:fast:prod -- -SkipWorkflowExport` (para deployar uma versao
  versionada especifica em vez do n8n dev atual).
- Verificacao anti-credencial embutida: se um workflow trouxer campo de credencial alem de `id`/`name`,
  o export ABORTA aquele arquivo (segredo nunca vai pro repo/deploy).

**Checklist pre-deploy (n8n):** se editou algum workflow no n8n dev, rode `npm run n8n:export` (ou
confie no gatilho `-Sync` acima) para garantir que os `.json` versionados batem com o n8n antes de
subir. O guard do pre-commit (`n8n:export:check`) AVISA — nao bloqueia — se o repo ficar atras.

Pre-requisito one-time: o `.env.production` da VPS precisa ter o bloco `AUTOMATION_*`, o n8n ja
precisa ter as credenciais/community nodes necessarios no volume, e a WAHA precisa estar pareada
quando o workflow de WhatsApp estiver em uso. Credenciais continuam manuais por seguranca.

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

## Role de runtime `omni_app` (AC-04) — PRE-REQUISITO de deploy

> **Armadilha que ja derrubou prod (incidente 2026-07-03).** A imagem AC-04 nao conecta
> mais como superuser: a API abre o pool com a role least-privilege **`omni_app`**
> (`DATABASE_APP_URL`). Essa role tem que **existir no banco** antes da imagem subir. Os
> scripts de deploy (`deploy:fast`/`deploy:prod`) so fazem `pull + up` — **NAO criam a role**.
> Subir a imagem sem a role = outage total:
>
> - `api` em crash-loop com `FATAL: password authentication failed for user "omni_app" (28P01)`
>   e o log `app_role_grants_skipped` (o boot viu a role faltando e so pulou, nao criou);
> - `web` presa em `Created` (`depends_on: api healthy` nunca satisfaz);
> - Caddy sem upstream vivo → **502** no painel.
>
> O `Validate()` NAO protege disso: ele so checa que `DATABASE_APP_URL` nao esta vazia — a URL
> existe (com role inexistente / senha vazia), passa no check, e o container morre na conexao.

**Setup one-time por ambiente** (prod ja feito em 2026-07-03; refazer em staging/ambiente novo):

1. Env no `.env.production` da VPS (NAO reutilizar `POSTGRES_PASSWORD`; senha alfanumerica evita urlencode):
   ```bash
   cd /home/deploy/lista-atendimento
   grep -q '^APP_DB_ROLE_PASSWORD=' .env.production || \
     printf 'APP_DB_ROLE=omni_app\nAPP_DB_ROLE_PASSWORD=%s\n' "$(openssl rand -hex 24)" >> .env.production
   ```
2. Criar a role (idempotente, re-rodavel). Superuser real da VPS e **`listaatendimento`** (nao `omni`):
   ```bash
   PW=$(grep '^APP_DB_ROLE_PASSWORD=' .env.production | cut -d= -f2)
   docker exec listaatendimento-postgres-1 psql -U listaatendimento -d listaatendimento -v ON_ERROR_STOP=1 \
     -c "DO \$\$ BEGIN IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname='omni_app') THEN CREATE ROLE omni_app LOGIN; END IF; END \$\$;" \
     -c "ALTER ROLE omni_app WITH LOGIN PASSWORD '$PW' NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS NOREPLICATION;" \
     -c "GRANT CONNECT ON DATABASE listaatendimento TO omni_app;"
   ```
   (Equivale ao `scripts/db/create-app-role.sql`, que ainda NAO esta copiado na VPS.) Depois:
   `docker compose --env-file .env.production -f docker-compose.prod.yml up -d --no-build --force-recreate api web`.
   O `migrate up` do boot sincroniza os GRANTs (`app_role_grants_ok` no log) e a api sobe como `omni_app`.

**Rollback de emergencia** (voltar a conectar como superuser, sem trocar imagem): setar
`APP_DB_ROLE=listaatendimento` e `APP_DB_ROLE_PASSWORD=$POSTGRES_PASSWORD` no `.env.production`
+ `up -d api web`.

**Prevencao (ac-04b) — CODIGO IMPLEMENTADO 2026-07-13, pendente de validacao+commit:** o `migrate up`
passa a **auto-provisionar a role** a partir de `DATABASE_APP_URL` (o `migrate` ja roda como superuser
e ja tem nome+senha na URL) — `EnsureAppRole` faz `CREATE ROLE ... LOGIN` (se ausente) +
`ALTER ROLE ... PASSWORD` least-privilege + `GRANT CONNECT`, idempotente, todo boot, ANTES dos grants.
Em production sem senha/URL, o `migrate` falha alto e cedo (`app_role_ensure_failed`, exit 1) em vez do
crash-loop `28P01`. Com isso o `deploy:fast` volta a ser suficiente sozinho e este passo manual vira
FALLBACK (imagens antigas + initdb). A partir da imagem que contem o ac-04b, criar a role deixa de ser
pre-requisito manual. Rastreado no roadmap (`ac-04b-migrate-auto-provision-role`) e no
`MULTITENANT_COMPLETION_PLAN.md` (AC-04). RESTA: rebuild da api + validar em volume limpo + commit.

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
(`/opt/omnichannel/Caddyfile`, container **`omnichannel-mvp-caddy-1`**) roteia o dominio publico
**`omni.crowvisuals.com.br`** pros aliases `lista-web`/`lista-api` (a stack conecta na rede externa
`omnichannel-mvp_default` com esses aliases).

> **Um `npm run deploy:*` NAO apaga estas mudancas.** O Caddyfile pertence a outra stack
> (`omnichannel-mvp`) e e' editado direto no disco da VPS; os scripts de deploy so mexem na stack
> `lista-atendimento` (api/web) e nunca tocam nele nem no `.env.production` (so atualizam `IMAGE_TAG`).
> O `TOOLS_PUBLIC_BASE_URL` tambem persiste. So somem se alguem restaurar um `Caddyfile.bak-*` antigo.

> **O arquivo REAL na VPS diverge do exemplo idealizado que estava aqui.** Bloco real auditado em
> 2026-07-13 (usa `import secure_headers` em vez de `header {...}` inline, cobre `omni.` **e**
> `www.omni.`, e ha um segundo host `lista.{$DOMAIN}` servindo o MESMO painel). Sempre confira o
> arquivo com `ssh ... "awk '/omni.crowvisuals/,/^}/' /opt/omnichannel/Caddyfile"` antes de editar.

```caddy
omni.crowvisuals.com.br, www.omni.crowvisuals.com.br {
    import secure_headers
    encode zstd gzip

    handle /v1/* { reverse_proxy lista-api:8080 }
    handle /v2/* { reverse_proxy lista-api:8080 }   # <- necessario p/ o switcher de contas
    handle /uploads/* { reverse_proxy lista-api:8080 }
    handle /s/* { reverse_proxy lista-api:8080 }    # <- redirects do encurtador (modulo tools)
    handle /q/* { reverse_proxy lista-api:8080 }    # <- redirects rastreados dos QR Codes
    handle /healthz { reverse_proxy lista-api:8080 }
    handle { reverse_proxy lista-web:3003 }         # <- TUDO que nao casar cai no Nuxt
}
```

Ha um segundo host equivalente, `lista.{$DOMAIN}`, com os mesmos `handle` apontando pra
`lista-api`/`lista-web` — **ao adicionar path novo (ex.: `/s/*`), replique nos DOIS blocos**, senao
o acesso pelo outro dominio nao roteia.

> **Incidente do encurtador (`/s/{slug}` dava 404) — RESOLVIDO 2026-07-13.** O modulo `tools` ja
> estava 100% pronto na API (a rota `GET /s/{slug}` respondia — `curl` interno em `/s/x` devolvia o
> JSON `{"error":{"code":"not_found",...}}` do handler, nao o `404 page not found` generico do mux),
> mas o Caddy **nunca teve** os `handle /s/*` e `/q/*` no arquivo real. Sem eles, `/s/lma` casava o
> `handle { reverse_proxy lista-web:3003 }` final e ia pro **Nuxt** → tela 404 do frontend (e, com
> sessao, redirect pro `/auth/login`). Sintoma de reconhecimento: 404 e' **HTML do Nuxt**, nao JSON
> da API. Fix = adicionar os dois `handle` nos blocos `omni.` e `lista.` + recriar o container
> (ver armadilha do inode abaixo). Validado de fora: `/s/lma` → `302` com `Location` pro destino.

#### Links curtos na raiz `crowvisuals.com.br` (sem o `omni.`) — ATIVO desde 2026-07-13

O `shortUrl` novo sai limpo em **`https://crowvisuals.com.br/s/{slug}`**. Como foi feito (e como o
estado atual esta montado):

1. `TOOLS_PUBLIC_BASE_URL=https://crowvisuals.com.br` no `.env.production` da stack lista + recriar a
   api (`docker compose --env-file .env.production -f docker-compose.prod.yml up -d --no-build
   --force-recreate api`). Isso muda **so o TEXTO** exibido em links NOVOS; quem resolve o redirect e'
   o host que o Caddy rotear. (Links ja criados guardam o texto antigo, mas resolvem igual pelos dois
   dominios — ver abaixo.)
2. A raiz `crowvisuals.com.br` ja e' servida por este Caddy central (o site da agencia roda no
   container `crow-web`). O bloco raiz, que era `reverse_proxy crow-web:80` direto, virou blocos
   `handle`: `/s/*` e `/q/*` vao pra `lista-api:8080` e **TODO o resto** (`handle { ... }`) segue pro
   `crow-web` — o site da agencia fica intacto. Bloco real:

```caddy
crowvisuals.com.br, www.crowvisuals.com.br {
    import secure_headers
    encode zstd gzip

    handle /s/* { reverse_proxy lista-api:8080 }   # (na VPS escrito em 3 linhas — ver armadilha inline)
    handle /q/* { reverse_proxy lista-api:8080 }

    handle {                                        # catch-all: site da agencia, inalterado
        reverse_proxy crow-web:80 {
            header_up Host {host}
            header_up X-Real-IP {remote_host}
            header_up X-Forwarded-For {remote_host}
            header_up X-Forwarded-Proto {scheme}
            header_up X-Forwarded-Host {host}
        }
    }
}
```

**Os dois dominios resolvem** (mantido de proposito): `crowvisuals.com.br/s/{slug}` (novo, limpo) E
`omni.crowvisuals.com.br/s/{slug}` (links antigos ja compartilhados nao quebram). O que muda com o
`TOOLS_PUBLIC_BASE_URL` e' so qual texto o painel EXIBE pra links novos.

> Se um dia a raiz sair desta VPS, reverta o `TOOLS_PUBLIC_BASE_URL` (volta pro `omni.`) — o
> `omni.crowvisuals.com.br/s/{slug}` continua funcionando sozinho.

**Aplicar mudanca no Caddy (procedimento validado 2026-07-13):**

```bash
# 1. backup preservando permissoes
cp -a /opt/omnichannel/Caddyfile /opt/omnichannel/Caddyfile.bak-$(date -u +%Y%m%d-%H%M%S)

# 2. editar. NAO use sed -i (troca o inode). Gere o novo em /tmp e sobrescreva o inode do host:
#    (edite /tmp/Caddyfile.new com o conteudo desejado, ex.: via python que insere os handle)
cat /tmp/Caddyfile.new > /opt/omnichannel/Caddyfile

# 3. validar a sintaxe ANTES de aplicar (dentro do container, contra o arquivo novo):
docker cp /tmp/Caddyfile.new omnichannel-mvp-caddy-1:/tmp/Caddyfile.new
docker exec omnichannel-mvp-caddy-1 caddy validate --config /tmp/Caddyfile.new --adapter caddyfile
#   -> "Valid configuration" (warnings de header_up/fmt sao pre-existentes, pode ignorar)

# 4. recarregar. Se o `caddy reload` nao pegar (ver armadilha do inode), RESTART o container:
docker restart omnichannel-mvp-caddy-1
```

> **NAO use** `docker compose ... --profile channels ... up -d caddy` (o que estava aqui antes):
> na VPS o servico `caddy` tem `depends_on: atendimento-online-api` (profile `atendimento`), e sem o
> env-file/profile certos o compose falha com `depends on undefined service "atendimento-online-api"`.
> `docker restart <container>` nao passa pelo compose, nao resolve dependencias, e e' o caminho seguro
> pra so remontar o bind e reler o arquivo.

**Armadilhas do Caddy (ja custaram tempo):**
- **`/v2/*` separado e' obrigatorio.** A accounts API (`/v2/me/accounts`, `/v2/me/context`) move o
  switcher de contas. Sem o bloco `/v2/*`, a conta ativa nao resolve pra agencia e somem do menu
  os itens `agencyOnly` (Clientes Web, Usuarios Admin, Organizations) — mesmo com dado certo no
  banco. `handle /v1/* /v2/* {` (dois paths no mesmo handle) e' INVALIDO; use dois blocos (ou um
  matcher nomeado `@api path /v1/* /v2/*`).
- **`handle /x/* { reverse_proxy ... }` INLINE (uma linha so) quebra a adaptacao** neste build do
  Caddy: `Error: adapting config ... Unexpected next token after '{' on same line`. Escreva SEMPRE em
  3 linhas (`handle /s/* {` / `    reverse_proxy lista-api:8080` / `}`), como os blocos que ja validam.
  E' o que derrubou o Caddy num restart durante o fix da raiz — por isso o passo 3 (`caddy validate`)
  e' obrigatorio ANTES de `docker restart`, e o restart so pode rodar SE a validacao imprimiu
  `Valid configuration` (nao encadeie `edita > restart` sem o gate no meio).
- **Bind-mount por inode — PIOR do que so o `sed -i`.** O Docker monta `/opt/omnichannel/Caddyfile`
  resolvendo o **inode** no momento em que o container subiu. Se o arquivo do host tiver trocado de
  inode DEPOIS (um `cp -a`, `mv`, editor que reescreve, ou um `sed -i` antigo), o container fica preso
  ao inode ORFAO antigo e **nem `cat > Caddyfile` nem `caddy reload` chegam nele** — o `reload` recarrega
  o arquivo velho que o container ainda enxerga. Foi exatamente o que aconteceu no fix do encurtador:
  editei/reloadei e `/s` continuava indo pro Nuxt. Diagnostico: compare o inode host vs container —
  `ls -li /opt/omnichannel/Caddyfile` e `docker exec omnichannel-mvp-caddy-1 ls -li /etc/caddy/Caddyfile`;
  se diferirem, o `reload` e' inutil. **Cura confiavel: `docker restart omnichannel-mvp-caddy-1`** (remonta
  o bind no inode atual do host; ~3s de downtime do proxy). Confirme com o `grep -c "handle /s/"` dentro
  do container batendo com o host.

### Variaveis principais do `.env.production`

```bash
COMPOSE_PROJECT_NAME=omni
POSTGRES_DB=omni
POSTGRES_USER=omni
POSTGRES_PASSWORD=<senha-forte>
APP_DB_ROLE=omni_app                 # AC-04: role least-privilege de runtime da api
APP_DB_ROLE_PASSWORD=<senha-alfanumerica>   # obrigatorio em prod; role tem que existir no banco (ver secao AC-04)
PROXY_NETWORK_NAME=omnichannel-mvp_default
PROXY_API_ALIAS=lista-api
PROXY_WEB_ALIAS=lista-web
WEB_APP_URL=https://omni.crowvisuals.com.br
NUXT_PUBLIC_API_BASE=https://omni.crowvisuals.com.br
NUXT_PUBLIC_API_WS_BASE=wss://omni.crowvisuals.com.br
NUXT_API_INTERNAL_BASE=http://api:8080
CORS_ALLOWED_ORIGINS=https://omni.crowvisuals.com.br
AUTH_TOKEN_SECRET=<segredo-longo-e-aleatorio>
CALENDAR_AI_WEBHOOK_URL=http://n8n:5678/webhook/calendar-omni
CALENDAR_AI_SERVICE_TOKEN=<segredo-forte>
CALENDAR_AI_CALLBACK_BASE=http://api:8080
CALENDAR_CHAT_WEBHOOK_URL=http://n8n:5678/webhook/calendar-chat
CALENDAR_TRANSCRIBE_WEBHOOK_URL=http://n8n:5678/webhook/calendar-transcribe
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
  `skip_smoke_tests`, `deploy_automation`, `force_automation_workflow_import`. Secret necessario:
  `DEPLOY_VPS_SSH_KEY`.

```bash
gh workflow run deploy-vps.yml --repo mikewade2k16/lista-da-vez \
  -f environment=prod -f image_tag=sha-<40hex> -f backup_database=true -f deploy_automation=true
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

## Backup

Backup diario AGENDADO do Postgres: cron do host (06:40 UTC) roda
`/home/deploy/lista-atendimento/scripts/backup-db.sh` (fonte: `scripts/backup/backup-db.sh`
no repo) — retencao 7 diarios + 4 semanais em `backups/daily|weekly/`, off-site opcional via
rclone, status em `backups/last_backup_status`. O workflow `backup-check.yml` confere/roda
fallback todo dia e alerta por e-mail se falhar. Instalacao, teste de restore mensal e
restore real: [BACKUP_RESTORE.md](BACKUP_RESTORE.md).

O backup on-demand dos deploys (`-BackupDatabase` / input `backup_database`) continua
existindo e segue OBRIGATORIO em release que toca schema.

Ainda manual (pendente): volume `api_uploads` e arquivo `.env.production`.

---

## Monitoração

Monitoração mínima de produção (AC-16), em 3 camadas complementares:

1. **UptimeRobot** — de fora, vigia a URL pública e avisa se o painel cair.
2. **`check-vps.sh`** no cron do host — de dentro, alerta disco/RAM/load/containers doentes e a
   **saude do n8n** (container do profile automation no ar + workflows criticos `active`, OBS-07).
3. **`/healthz` com ping de banco** — a api se auto-reporta: `200` (banco ok) ou `503`
   (`db:"unreachable"`), então o healthcheck do compose e o smoke do deploy enxergam banco morto.

Estágio 2 (fora de escopo agora): Prometheus/Grafana/cAdvisor/Netdata — só quando houver VPS
dedicada/maior; pré-requisito é o AC-11 (limites de memória no compose) já aplicado.

### 1. Uptime externo (UptimeRobot — one-time, sem código)

- Criar conta free em uptimerobot.com → **Add Monitor** → tipo `HTTP(s)`.
- URL: `https://omni.crowvisuals.com.br/healthz`, intervalo 5 min.
- Alert contact: e-mail do operador (opcional: webhook pro mesmo tópico ntfy).
- Como o `/healthz` agora devolve `503` sem banco, o monitor simples de status já cobre api
  E banco — não precisa de keyword monitor.

### 2. Instalar o script de alertas (one-time)

```bash
# da sua maquina (repo local)
scp -i ~/.ssh/gh_actions_omnichannel_vps scripts/monitoring/check-vps.sh \
  deploy@85.31.62.33:/home/deploy/monitoring/check-vps.sh
ssh -i ~/.ssh/gh_actions_omnichannel_vps deploy@85.31.62.33 \
  "chmod +x /home/deploy/monitoring/check-vps.sh"
```

### 3. Configurar o canal de alerta (na VPS, NUNCA no repo)

Criar `/home/deploy/.omni-monitoring.env` com `chmod 600`. Exemplo (usar ntfy e/ou Telegram —
o que estiver preenchido; sem env file o script só ecoa no stdout):

```bash
# /home/deploy/.omni-monitoring.env  (chmod 600)
ALERT_NTFY_URL=https://ntfy.sh/omni-<algo-aleatorio-secreto>
# ALERT_TELEGRAM_BOT_TOKEN=123456:ABC...
# ALERT_TELEGRAM_CHAT_ID=123456789
# DISK_USAGE_MAX=85  MEM_AVAILABLE_MIN_PCT=10  LOAD_PER_CORE_MAX=2
# --- OBS-07 (sonda do n8n): so preencher se path/porta divergirem do default ---
# N8N_COMPOSE_DIR=/home/deploy/lista-atendimento   (onde vive docker-compose.prod.yml + .env.production)
# N8N_ENV_FILE=.env.production
# N8N_CRITICAL_IDS="calendaromni0001 calendarchat0001 calendartrans001 omnichatmvp00001"  (= deploy-pull.ps1:246)
```

A **sonda do n8n** (check #7, OBS-07) roda como o proprio `deploy` — que ja esta no grupo docker,
entao **nao precisa de token/credencial do n8n** para ler o estado dos workflows. Ela le via
`docker compose --profile automation exec n8n n8n export:workflow --all` (respeita o WAL do SQLite).
Se `N8N_COMPOSE_DIR` nao existir no host, o check e NO-OP. A lista `N8N_CRITICAL_IDS` e a MESMA de
`scripts/deploy/deploy-pull.ps1:246` (contrato: mudou uma, muda a outra). Antes de instalar na VPS,
conferir a porta e o nome do servico: `grep AUTOMATION_N8N_PORT .env.production` e confirmar que o
servico compose e `n8n`.

### 4. Crontab (na VPS, user deploy)

```bash
( crontab -l 2>/dev/null; echo '*/5 * * * * /home/deploy/monitoring/check-vps.sh >> /home/deploy/monitoring/check-vps.log 2>&1' ) | crontab -
```

O script é quiet: o `.log` só cresce quando há alerta/erro.

### 5. Log rotation dos containers

O `docker-compose.prod.yml` agora tem `logging:` json-file com teto (~250MB total no host:
api 20m×5 + demais 10m×3). **A mudança só aplica ao RECRIAR o container** — no próximo deploy
usar o input `force_recreate` do `deploy-vps.yml` ou, na VPS:

```bash
docker compose --env-file .env.production -f docker-compose.prod.yml up -d --force-recreate
```

### 6. Testar o alerta

Na VPS, forçar o alerta de disco e depois rearmar:

```bash
DISK_USAGE_MAX=1 /home/deploy/monitoring/check-vps.sh   # deve disparar no canal configurado
rm /home/deploy/.omni-monitoring-state/disk.last        # rearma o cooldown
```

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
