# Plano de Deploy — Registry (GHCR) + Staging sob demanda

> **Status:** `pending` (plano aprovado em 2026-06-15; implementação NÃO iniciada).
> **Doc canônico** desta frente. Espelhado em `roadmap-data.ts` (fase `deploy-registry-staging`, grupo `infra-deploy`).
> Decisões fechadas com o usuário (2026-06-15): **build de imagens no GitHub Actions + push pro GHCR**; **staging na MESMA VPS, sob demanda**.

---

## 1. Contexto e objetivo

Hoje o deploy do Omni (`scripts/deploy/deploy-vps-fast.ps1` + `.github/workflows/deploy-vps.yml`) é **full-sync + build na VPS**:

1. `tar` do workspace inteiro → SSH → `rm -rf` do remoto (preserva só `.env.production` e `backups`) → extrai.
2. `docker compose -f docker-compose.prod.yml up -d --build` → **a VPS compila Go E builda o Nuxt**.

Dois problemas:

- **Sobrecarga da VPS.** O build do Nuxt pede 4GB de heap (`web/Dockerfile:13`, `NODE_OPTIONS=--max-old-space-size=4096`). A VPS auditada tem ~6GB de RAM com produção já no ar (postgres+api+web, + perfil automation). Compilar na VPS com a produção rodando é o risco de OOM/lentidão que queremos eliminar.
- **Medo de quebrar prod.** Não existe ambiente intermediário: todo deploy é direto em produção.

Objetivo desta frente: **deploy rápido, sem build na VPS, com um staging isolado para testar antes**, seguindo o padrão de SaaS em escala (build-once, promote-the-same-artifact).

## 2. Por que NÃO portar o incremental do crow-php 1:1

O `crow-php/scripts/deploy/deploy-vps-incremental.ps1` é um **rsync feito à mão**: lê manifest remoto (`find -printf '%P\t%s\t%T@'`), faz diff por tamanho/mtime e envia só os arquivos alterados num `.tar.gz`.

Ele voa **porque crow-php é PHP (interpretado)**: o `Dockerfile` é `COPY . /var/www/html/`; o rebuild só re-executa `COPY`+`chown` (camada cara `docker-php-ext-install` fica em cache). Lá o gargalo era **transferir mídia/assets** — e o incremental matou exatamente isso.

No Omni o gargalo é **compilar**, não transferir. Sincronizar só os arquivos alterados economizaria transferência, mas a VPS continuaria compilando Go + Nuxt — resolveria o problema errado e manteria a sobrecarga.

A ideia central do crow-php ("só o que mudou") continua valendo — mas no Omni ela vive **no nível de camada Docker**: `docker pull` baixa só as camadas que mudaram. Movendo o build pra fora da VPS, ganhamos o incremental de graça e sem compilar no host de produção.

## 3. Arquitetura-alvo (build-once, promote)

```
                     ┌─────────────────────────────┐
   git push / tag →  │  GitHub Actions (CI)        │
                     │  - testa (go test, vitest)  │
                     │  - build buildx Go + Nuxt   │
                     │  - push GHCR :<sha> :<branch>│
                     └──────────────┬──────────────┘
                                    │ docker pull (só camadas novas)
              ┌─────────────────────┴───────────────────────┐
              ▼                                               ▼
   ┌──────────────────────┐                       ┌──────────────────────┐
   │ STAGING (sob demanda) │   promove a MESMA     │ PRODUÇÃO              │
   │ omni-staging          │   imagem (mesmo SHA)  │ omni                  │
   │ staging.lista.<dom>   │  ───────────────────► │ lista.whenthelights…  │
   │ DB/volumes próprios   │                       │ DB/volumes próprios   │
   └──────────────────────┘                       └──────────────────────┘
              │  pull + up                                    │  pull + up (--no-build)
              └──────────── mesma VPS, mesmo Caddy ───────────┘
```

Regras de ouro:

- **A VPS nunca compila.** Só `docker compose pull && up -d`. CPU/RAM de build ficam no GitHub Actions.
- **O artefato é imutável e rastreável por SHA.** `ghcr.io/mikewade2k16/omni-api:<sha>` e `omni-web:<sha>`.
- **Promoção = apontar prod pro MESMO SHA que rodou em staging.** Não recompila, não "torce": sobe o byte-a-byte testado.
- **Rollback = apontar de volta pro SHA anterior** (`pull` da camada já em cache + `up`), instantâneo.

### Tags no GHCR

| Tag | Quando | Uso |
|---|---|---|
| `:<git-sha>` | todo build | imutável, fonte de verdade da promoção |
| `:<branch>` (ex.: `main`, `refactor-…`) | todo build | conveniência ("último da branch") |
| `:staging` | ao subir em staging | ponteiro do que está em staging |
| `:prod` | ao promover | ponteiro do que está em produção |

`IMAGE_TAG` no `.env.staging`/`.env.production` recebe **o SHA** (não `latest`) — é o que garante "o que testei é o que subiu".

## 4. Componentes a criar / alterar

### 4.1 CI — novo workflow `.github/workflows/build-images.yml`
- Dispara em `push` (branches relevantes) + `workflow_dispatch`.
- Reusa os gates de teste já existentes no `deploy-vps.yml` (go test, vitest, audits, govulncheck) **como pré-requisito** do build.
- `docker/login-action` → GHCR (`GITHUB_TOKEN` com `packages: write`).
- `docker/build-push-action` x2 (api `./back`, web `./web` target prod), com `cache-from/to: type=gha` (camadas em cache entre runs → builds rápidos).
- Tags: `:<sha>` + `:<branch>`. Label `org.opencontainers.image.source` apontando pro repo (liga o pacote ao repositório no GHCR).

### 4.2 `docker-compose.prod.yml` — imagens parametrizadas por env
- `api.image` e `web.image` passam a ser `${API_IMAGE}:${IMAGE_TAG:-latest}` / `${WEB_IMAGE}:${IMAGE_TAG:-latest}`.
- Manter as seções `build:` (úteis pra dev / build local de emergência), mas o fluxo de prod usa `pull` + `up --no-build`.
- `postgres`, `redis`, `waha`, `n8n` continuam com imagens públicas (já são pinadas) — sem mudança.

### 4.3 Staging — `.env.staging` + isolamento por `COMPOSE_PROJECT_NAME`
- `COMPOSE_PROJECT_NAME=omni-staging` → containers, **volumes** (`omni-staging_postgres_data`, etc.) e rede ganham namespace próprio automaticamente. **Zero risco de tocar dados/volume de prod.**
- Path remoto: `/home/deploy/lista-atendimento-staging` (separado).
- Aliases de proxy próprios: `PROXY_API_ALIAS=lista-staging-api`, `PROXY_WEB_ALIAS=lista-staging-web`.
- URLs: `WEB_APP_URL`/`NUXT_PUBLIC_API_BASE`/`CORS_ALLOWED_ORIGINS` = `https://preview.whenthelightsdie.com`.
- `AUTH_TOKEN_SECRET`, `POSTGRES_PASSWORD` etc. **próprios** (nunca os de prod).
- Perfil `automation` **desligado** em staging por padrão (economia de RAM); liga sob demanda se precisar testar o bot.

### 4.4 Caddy central (`/opt/omnichannel/Caddyfile`) — bloco do staging
Mesmo desenho do bloco de prod, trocando host e upstreams:
```caddy
preview.whenthelightsdie.com {
  header { ... mesma matriz de segurança ... }
  handle /v1/*     { reverse_proxy lista-staging-api:8080 }
  handle /uploads/*{ reverse_proxy lista-staging-api:8080 }
  handle /healthz  { reverse_proxy lista-staging-api:8080 }
  handle           { reverse_proxy lista-staging-web:3003 }
}
```
(Opcional: basic auth no edge pra ninguém externo achar o staging.)

### 4.5 Scripts locais (PowerShell, Windows) — substituem o build-na-VPS

- `scripts/deploy/deploy-fast.ps1` — **caminho rápido, sem git** (recomendado pro dia a dia):
  builda as imagens na MÁQUINA LOCAL (`docker build` de `back/` e `web/`), `docker push`
  pro GHCR (incremental: só as camadas que mudaram sobem) e chama o `deploy-pull.ps1`.
  `-Environment staging|prod`, `-Tag` (default `local-<timestamp>`), `-Service both|api|web`.
  npm: `deploy:fast` / `deploy:fast:prod`. É o equivalente Omni ao incremental do crow-php —
  a diferença é que o build fica na sua máquina (RAM de sobra), não na VPS (o Nuxt pede 4GB).
- `scripts/deploy/deploy-pull.ps1` — parametrizado por `-Environment staging|prod`:
  1. resolve o SHA alvo (default: `git rev-parse HEAD` já buildado, ou parâmetro `-Tag <sha>`),
  2. escreve `IMAGE_TAG=<sha>` no `.env.<env>` remoto,
  3. `docker login ghcr.io` (se imagens privadas),
  4. `docker compose --env-file .env.<env> -f docker-compose.prod.yml pull api web`,
  5. backup do Postgres (se `-BackupDatabase`),
  6. `up -d --no-build api web` + `ps`,
  7. smoke tests no domínio do ambiente.
- `scripts/deploy/staging-up.ps1` / `staging-down.ps1` — sobe/derruba a stack `omni-staging` (sob demanda).
- `scripts/deploy/promote.ps1` — pega o `:staging` atual e promove pra prod (escreve o mesmo SHA no `.env.production`, pull, up). Recusa promover SHA que não passou por staging.

### 4.6 Workflow de deploy existente
- `deploy-vps.yml` evolui de "build na VPS" para "pull do GHCR". Os gates de teste continuam; o passo `up -d --build` vira `pull` + `up --no-build`. Inputs: `image_tag` (SHA), `environment` (staging/prod), `backup_database`.

## 5. Fluxo de promoção (o que tira o medo)

```
1. push da branch         → CI builda e publica ghcr…/omni-{api,web}:<sha>
2. deploy-pull -Environment staging -Tag <sha>   → staging.lista.<dom> roda <sha>
3. testar em staging      (login, /tasks, /operacao, /erp, WS, smoke do release)
4. promote.ps1            → produção passa a rodar EXATAMENTE o <sha> testado
5. se quebrar             → deploy-pull -Environment prod -Tag <sha-anterior>  (rollback instantâneo)
```

Produção recebe o **mesmo artefato** validado em staging — não uma recompilação que "deveria" ser igual.

## 6. Banco do staging (isolado de verdade)

- Volume próprio (`omni-staging_postgres_data`) — **nunca** o de prod.
- Seed inicial: restaurar um **backup sanitizado** da prod (mascarar/baralhar PII, e-mails, senhas) OU rodar `bootstrap-owner` com um owner de teste. Decidir no detalhamento; default recomendado = bootstrap de teste limpo (sem dado real de cliente em staging).
- A imagem da api roda `migrate up` no boot → **staging vira ensaio real das migrations** antes da prod. (Mantém a regra: migration destrutiva exige backup + checagem de divergência ANTES de promover — ver §10.)

## 7. Infra / não-sobrecarregar a VPS

- **Build sai da VPS** (vai pro CI) — elimina o pico de 4GB do Nuxt no host de produção. Este é o maior ganho.
- **Staging sob demanda** — `staging-down.ps1` derruba a stack quando não está testando; volume persiste. Idle (se ficar no ar) é leve porque **não builda** (postgres + api + web ~ poucas centenas de MB).
- **`mem_limit`** opcional nos serviços de staging pra ele nunca competir agressivamente com prod.
- **Limpeza de imagens** na VPS: `docker image prune` periódico (as tags por SHA acumulam). Documentar no runbook.
- **Disco**: 72GB livres na auditoria; imagens GHCR cacheadas + tags antigas cabem folgado, mas o prune evita crescer sem fim.

## 8. Segurança

- Imagens **privadas** no GHCR (contêm o código compilado). VPS autentica com um **PAT fine-grained read-only** (`read:packages`) guardado só no `.env` remoto, OU `docker login` com `GITHUB_TOKEN` no CI e PAT dedicado na VPS. (Alternativa: tornar públicas — descartada por expor o build.)
- **Segredos nunca entram na imagem** — só em runtime via `.env.<env>` (já é o padrão). Confirmar que nenhum `COPY` arrasta `.env*`.
- `.env.staging` e `.env.production` ficam **só na VPS** (git-ignored), com segredos distintos por ambiente.
- Caddy do staging com matriz de headers de segurança igual à de prod (+ basic auth opcional).

## 9. Migrations — o único ponto sensível (igual hoje)

Trocar o mecanismo de deploy **não muda** a disciplina de migrations:

- A api roda `migrate up` no boot da imagem. Promover imagem nova = rodar as migrations dela.
- Staging roda as migrations primeiro (ensaio). Se passar limpo em staging, prod tende a passar.
- Migration destrutiva (DROP/rename/view-swap) continua exigindo: **backup antes**, checagem de divergência de dados, confirmação explícita. Ver `docs/DEPLOY_VPS.md` (seção do release multitenant) e a regra de senha/dados em `AGENT_RULES.md`.

## 10. Plano de implementação por fases (independentes ⇒ paralelizáveis)

| Fase | Escopo | Depende de | Paralelizável? |
|---|---|---|---|
| **D1** | `build-images.yml` (CI buildx + push GHCR + cache gha) | — | sim (trilha CI) |
| **D2** | `docker-compose.prod.yml` com `image` parametrizada por env + `.env.production.example` (API_IMAGE/WEB_IMAGE/IMAGE_TAG) | — | sim (trilha compose) |
| **D3** | `.env.staging.example` + decisão de seed do banco staging | D2 | sim |
| **D4** | Bloco Caddy `staging.lista…` + DNS `A staging.lista` | — | sim (precisa do usuário p/ DNS) |
| **D5** | `deploy-pull.ps1` + `staging-up/down.ps1` + `promote.ps1` | D1, D2 | depois de D1+D2 |
| **D6** | `deploy-vps.yml` migra build→pull; inputs image_tag/environment | D1, D2 | depois de D1+D2 |
| **D7** | Runbook (atualizar `DEPLOY_VPS.md` + `DEPLOY_CHECKLIST.md` + `scripts/deploy/AGENT.md`) + panorama HTML | todas | por último |

D1, D2, D3, D4 são independentes entre si → candidatos a rodar em paralelo (ex.: 2 subagentes: um na trilha CI/imagens, outro na trilha compose/staging/caddy). D5/D6 dependem de D1+D2. **Implementação só começa após "ok, implementa".**

## 11. Notas de Deploy (env vars / secrets / DNS / ordem)

Variáveis novas (lembrar: declarar em `.env.<env>` **e** referenciar no `docker-compose.prod.yml`):

- `API_IMAGE=ghcr.io/mikewade2k16/omni-api`
- `WEB_IMAGE=ghcr.io/mikewade2k16/omni-web`
- `IMAGE_TAG=<git-sha>` (por ambiente)
- Staging: `COMPOSE_PROJECT_NAME=omni-staging`, `PROXY_API_ALIAS=lista-staging-api`, `PROXY_WEB_ALIAS=lista-staging-web`, URLs/secrets próprios.

Secrets:
- GHCR PAT read-only na VPS (`~/.docker/config.json` via `docker login`, ou var no `.env`).
- CI: usa `GITHUB_TOKEN` (permissão `packages: write` no workflow) — sem secret novo se o repo permitir.

DNS (manual, fora da VPS):
- `A staging.lista → 85.31.62.33` (ou `CNAME` pro host).

Ordem do primeiro go-live do pipeline:
1. CI publica as duas imagens (`:<sha>`). 2. DNS do staging. 3. Bloco Caddy do staging + reload. 4. `.env.staging` na VPS. 5. `staging-up` → valida. 6. Migrar `deploy-vps.yml`/scripts. 7. Primeiro `promote` pra prod com backup.

## 12. Riscos e mitigações

| Risco | Mitigação |
|---|---|
| Nuxt OOM no CI | runner do GitHub tem 7GB+; `--max-old-space-size=4096` já cabe |
| Imagens grandes / disco da VPS | `image prune` periódico; tags por SHA + retenção curta no GHCR |
| Staging competindo RAM com prod | sob demanda (down quando ocioso) + `mem_limit` opcional |
| PAT do GHCR vazar | fine-grained read-only `packages`, só na VPS, rotacionável |
| Migration destrutiva | backup + ensaio em staging + confirmação (regra existente) |
| Esquecer de referenciar var nova no compose | checklist da regra "feature flag em 2 lugares" |

## 13. Decisões tomadas (2026-06-15)

- **Build/deploy:** Registry (GHCR). DOIS caminhos para a mesma esteira de imagens:
  1. **Rápido, sem git (dia a dia):** `deploy-fast.ps1` builda na MÁQUINA LOCAL → `docker push` → VPS `pull`. É o equivalente Omni ao incremental do crow-php (sem git, sem CI), com o build na máquina local em vez da VPS.
  2. **Completo/rastreável (release formal):** `build-images.yml` builda no GitHub Actions com gate de testes + tag por SHA do git → VPS `pull`.
  Em ambos a **VPS nunca compila** (o build do Nuxt pede 4GB; não cabe na VPS de ~6GB). Descartado: porte literal do crow-php (build na VPS).
- **Staging:** mesma VPS, **projeto Compose separado, sob demanda** (sobe pra testar, derruba depois). (Descartado: VPS separada — custo; sempre-no-ar — RAM.)

## 14. Referências

- Inspiração: `C:\xampp\htdocs\crow-php\scripts\deploy\deploy-vps-incremental.ps1` (rsync à mão; bom pra PHP, não pra Go/Nuxt).
- Estado atual: `docs/DEPLOY_VPS.md`, `docs/DEPLOY_CHECKLIST.md`, `scripts/deploy/AGENT.md`, `.github/workflows/deploy-vps.yml`, `docker-compose.prod.yml`.
- Princípios: `AGENT_RULES.md` (Deploy), `docs/ENGINEERING_PRINCIPLES.md`.
