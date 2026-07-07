# AC-11 — Compose: limites de memória/CPU + healthchecks + restart/depends_on

> Spec de implementação — série ac-fixes-2026-07. Cobre o AC-11 (P1, esforço S, impacto alto)
> e a parte "compose" do AC-16 (healthchecks como base mínima de monitoração).
> Quem implementa recebe SÓ este arquivo: ele é autossuficiente.

## 1. Contexto

**Achado canônico AC-11** (fatos.json, `achados_canonicos.AC-11`): *"Compose sem limites de
memória e healthchecks faltando (web, redis, waha, n8n)"* — evidência: `grep mem_limit|deploy.resources`
= 0 matches nos 2 compose; healthcheck só em postgres+api.

Estado atual verificado em 2026-07-02 (leitura integral dos dois arquivos):

- `docker-compose.yml` (dev, 285 linhas, 8 serviços): healthcheck **só** em `postgres` (linhas 14-23)
  e `api` (linhas 135-144). `web`, `crow-nuxt`, `redis`, `waha`, `n8n`, `meta-ads-assistant` sem
  healthcheck. **Nenhum** serviço tem limite de memória/CPU. `restart:` só em `crow-nuxt` (linha 189).
  `crow-nuxt` depende de `api` sem condition (linhas 201-202); `n8n` depende de `redis` sem condition
  (linhas 255-256).
- `docker-compose.prod.yml` (prod, 255 linhas, 6 serviços): todos `restart: unless-stopped`, todos em
  `127.0.0.1`; healthcheck só em `postgres` (linhas 13-22) e `api` (linhas 116-125). `web` (que o Caddy
  externo roteia), `redis`, `waha`, `n8n` sem healthcheck e sem limites. `n8n` depende de `redis` sem
  condition (linhas 233-234).

Por que importa: a VPS de produção (85.31.62.33, Ubuntu 24.04, **~6 GB de RAM**, compartilhada com a
stack alheia omnichannel-mvp cujo Caddy roteia o painel) não tem nenhum teto de consumo — um vazamento
na WAHA (engine GOWS, o vilão histórico de RAM) ou no n8n pode derrubar a VPS inteira por OOM do host,
levando junto o painel. E sem healthcheck, `docker ps`, o smoke do deploy e qualquer monitor futuro
(AC-16) não enxergam serviço zumbi (processo vivo, HTTP morto).

**Fatos verificados ao vivo nesta sessão (2026-07-02), nos containers dev rodando:**

| Verificação | Resultado |
|---|---|
| `docker compose version` local | **v2.38.2-desktop.1** — suporta `mem_limit`/`mem_reservation`/`cpus` service-level (compose spec, modo não-swarm) |
| WAHA `devlikeapro/waha:gows-2026.5.1` | tem `/usr/bin/curl` e node (Debian bookworm). `GET /ping` = **200**; `/health` = 422 (feature Plus); `/api/health` = 404 |
| n8n `n8nio/n8n:2.23.2` | Alpine; tem `wget` (sem curl). `GET /healthz` = `{"status":"ok"}` |
| web `node:24.11.1-bookworm-slim` | **sem curl/wget**; Node 24 tem `fetch` global. `GET http://127.0.0.1:3003/` = 200 (não existe /healthz no Nitro — grep confirmou) |
| crow-nuxt | node presente; `GET http://127.0.0.1:3000/` = 200 |
| redis `redis:8-alpine` | `redis-cli --no-auth-warning -a "$REDIS_PASSWORD" ping` = **PONG** (env `REDIS_PASSWORD` já existe no serviço dev; **não existe no prod** — precisa ser adicionada) |
| meta-ads-assistant | `node:22-alpine`; `GET /healthz` sem auth (`meta-ads-assistant/src/server.mjs:230`) |

## 2. Objetivo e não-objetivos

**Objetivo:** editar `docker-compose.yml` e `docker-compose.prod.yml` para (a) limitar memória de todos
os serviços (e CPU no prod), (b) adicionar healthcheck a todo serviço que não tem, (c) revisar
`restart:` e `depends_on` com `condition` onde faz sentido. Nada além disso.

**Não-objetivos (explicitamente NÃO fazer):**
- **NÃO mudar nenhuma porta** (regra dura: api 9091 no host via `.env`, web 3003, postgres 5432; prod tudo em 127.0.0.1 com os mapeamentos atuais). Não tocar em `ports:` de nenhum serviço.
- NÃO mudar imagens, tags, `command:` (exceto zero mudanças — nem o `command` do redis), `networks:`, `aliases`, `profiles`, `volumes:`, variáveis de ambiente existentes (única adição permitida: `REDIS_PASSWORD` no redis do prod, ver passo 4).
- NÃO adicionar rota /healthz no Nitro/web (o `GET /` resolve; criar rota é código de app, fora do escopo).
- NÃO instalar autoheal/watchtower nem exporters (isso é o resto do AC-16, outra frente).
- NÃO mexer nos healthchecks existentes de postgres e api (já funcionam).
- NÃO rodar nada na VPS (aplicação em prod é manual pelo usuário — seção 6).
- NÃO usar `deploy.resources` (formato swarm); usar `mem_limit`/`mem_reservation`/`cpus` service-level, que o compose v2.38 aplica em modo não-swarm.

## 3. Mudanças

### 3.1 Tabela de valores decididos (não recalcular, usar exatamente estes)

| Serviço | dev `mem_limit` | dev `mem_reservation` | prod `mem_limit` | prod `mem_reservation` | prod `cpus` |
|---|---|---|---|---|---|
| postgres | `1g` | `256m` | `1g` | `256m` | `2.0` |
| api | `512m` | `128m` | `512m` | `128m` | `2.0` |
| web | `4g` (nuxt dev compila; heap do build já usa 4096 MB — `web/Dockerfile:13`) | `512m` | `768m` (serve só `.output`) | `128m` | `1.0` |
| crow-nuxt | `512m` | `64m` | — (não existe no prod) | — | — |
| redis | `256m` | `32m` | `256m` | `32m` | `0.5` |
| waha | `1g` | `256m` | `1g` | `256m` | `1.5` |
| n8n | `768m` | `192m` | `768m` | `192m` | `1.0` |
| meta-ads-assistant | `512m` | `64m` | — (não existe no prod) | — | — |

Racional (para o comentário no YAML): soma dos limits de prod = **4,25 GiB** (core 2,25 + automação 2,0)
numa VPS de ~6 GB — limits são TETO, não alocação; a soma das reservations é ~0,97 GiB, sobrando folga
para SO, Caddy e a stack alheia. **Não definir `cpus` no dev** (compile do Vite/Nuxt precisa de burst na
máquina local). Se um serviço estourar o limit, o kernel OOM-killa (exit 137) e o `restart: unless-stopped`
ressuscita — comportamento desejado (auto-recuperação em vez de derrubar a VPS).

### 3.2 Healthchecks novos (dev E prod, salvo indicação)

Usar exatamente estes blocos. Atenção ao `$$` no redis (escapa a interpolação do compose para que a
variável seja lida do ambiente do CONTAINER em runtime, não do `.env` no host).

**web** — forma exec `CMD` com node (imagem não tem curl/wget); endpoint validado `GET /` = 200:

```yaml
    healthcheck:
      test: ["CMD", "node", "-e", "fetch('http://127.0.0.1:3003/').then((r)=>process.exit(r.ok?0:1)).catch(()=>process.exit(1))"]
      interval: 30s
      timeout: 10s
      retries: 5
      start_period: 180s   # dev: nuxt dev demora a compilar. No PROD usar start_period: 30s
```

**crow-nuxt** (só existe no dev) — mesmo padrão, porta interna 3000, `start_period: 30s`:

```yaml
    healthcheck:
      test: ["CMD", "node", "-e", "fetch('http://127.0.0.1:3000/').then((r)=>process.exit(r.ok?0:1)).catch(()=>process.exit(1))"]
      interval: 30s
      timeout: 10s
      retries: 5
      start_period: 30s
```

**redis** (dev e prod) — validado ao vivo (PONG):

```yaml
    healthcheck:
      test: ["CMD-SHELL", "redis-cli --no-auth-warning -a \"$$REDIS_PASSWORD\" ping | grep -q PONG"]
      interval: 15s
      timeout: 5s
      retries: 5
      start_period: 10s
```

**waha** (dev e prod) — curl existe na imagem; `/ping` é o endpoint Core (`/health` é WAHA Plus → 422; `/api/health` → 404):

```yaml
    healthcheck:
      test: ["CMD-SHELL", "curl --fail --silent http://127.0.0.1:3000/ping > /dev/null || exit 1"]
      interval: 30s
      timeout: 10s
      retries: 5
      start_period: 60s
```

**n8n** (dev e prod) — imagem Alpine só tem wget; `/healthz` validado (`{"status":"ok"}`):

```yaml
    healthcheck:
      test: ["CMD-SHELL", "wget -q -O /dev/null http://127.0.0.1:5678/healthz || exit 1"]
      interval: 30s
      timeout: 10s
      retries: 5
      start_period: 60s
```

**meta-ads-assistant** (só dev, profile próprio) — node:22-alpine tem fetch global; `/healthz` sem auth:

```yaml
    healthcheck:
      test: ["CMD", "node", "-e", "fetch('http://127.0.0.1:8765/healthz').then((r)=>process.exit(r.ok?0:1)).catch(()=>process.exit(1))"]
      interval: 30s
      timeout: 10s
      retries: 5
      start_period: 20s
```

### 3.3 Passo a passo — `docker-compose.yml` (dev)

1. **postgres** (bloco linhas 4-23): adicionar `restart: unless-stopped`, `mem_limit: 1g`,
   `mem_reservation: 256m`. Manter healthcheck existente intocado.
2. **api** (bloco linhas 25-144): adicionar `restart: unless-stopped`, `mem_limit: 512m`,
   `mem_reservation: 128m`. Manter healthcheck existente intocado.
3. **web** (bloco linhas 146-173): adicionar `restart: unless-stopped`, `mem_limit: 4g`,
   `mem_reservation: 512m` e o healthcheck da seção 3.2 (com `start_period: 180s`). Adicionar
   comentário de 1 linha: `# mem 4g: nuxt dev + Vite compilam no container (heap do build ja e 4096MB)`.
4. **crow-nuxt** (bloco linhas 186-202): já tem `restart: unless-stopped`. Adicionar `mem_limit: 512m`,
   `mem_reservation: 64m`, o healthcheck da seção 3.2, e trocar o `depends_on` de forma curta
   (`- api`) para:
   ```yaml
    depends_on:
      api:
        condition: service_healthy
   ```
5. **redis** (bloco linhas 209-218): adicionar `restart: unless-stopped`, `mem_limit: 256m`,
   `mem_reservation: 32m` e o healthcheck da seção 3.2. **Não** mexer no `command` nem no env
   `REDIS_PASSWORD` (já existe, linha 214).
6. **waha** (bloco linhas 220-234): adicionar `restart: unless-stopped`, `mem_limit: 1g`,
   `mem_reservation: 256m`, o healthcheck da seção 3.2 e ordering de boot:
   ```yaml
    depends_on:
      n8n:
        condition: service_started
   ```
   (só ordena o boot — a WAHA posta webhooks no n8n; NÃO usar `service_healthy` aqui para não impedir
   o uso do QR/dashboard da WAHA caso o n8n esteja quebrado — features coexistem).
7. **n8n** (bloco linhas 236-256): adicionar `restart: unless-stopped`, `mem_limit: 768m`,
   `mem_reservation: 192m`, o healthcheck da seção 3.2, e trocar `depends_on: - redis` para:
   ```yaml
    depends_on:
      redis:
        condition: service_healthy
   ```
8. **meta-ads-assistant** (bloco linhas 263-274): adicionar `restart: unless-stopped`,
   `mem_limit: 512m`, `mem_reservation: 64m` e o healthcheck da seção 3.2.

### 3.4 Passo a passo — `docker-compose.prod.yml`

Comentário de topo (2-3 linhas, acima de `services:`): registrar que os `mem_limit` somam 4,25 GiB
de TETO numa VPS de ~6 GB e que OOM-kill + `restart: unless-stopped` = auto-recuperação.

1. **postgres** (linhas 4-24): adicionar `mem_limit: 1g`, `mem_reservation: 256m`, `cpus: 2.0`.
   `restart` e healthcheck já existem — não mexer.
2. **api** (linhas 26-130): adicionar `mem_limit: 512m`, `mem_reservation: 128m`, `cpus: 2.0`.
3. **web** (linhas 132-160): adicionar `mem_limit: 768m`, `mem_reservation: 128m`, `cpus: 1.0` e o
   healthcheck da seção 3.2 **com `start_period: 30s`** (prod serve `.output`, sobe rápido).
4. **redis** (linhas 172-182): adicionar `mem_limit: 256m`, `mem_reservation: 32m`, `cpus: 0.5`,
   o healthcheck da seção 3.2 **e** o bloco de environment que hoje não existe no prod:
   ```yaml
    environment:
      REDIS_PASSWORD: ${AUTOMATION_REDIS_PASSWORD}
   ```
   (mesma var que o `command` já usa — nenhuma env nova no `.env` da VPS; sem ela o healthcheck
   não autentica).
5. **waha** (linhas 184-205): adicionar `mem_limit: 1g`, `mem_reservation: 256m`, `cpus: 1.5`,
   o healthcheck da seção 3.2 e:
   ```yaml
    depends_on:
      n8n:
        condition: service_started
   ```
6. **n8n** (linhas 207-239): adicionar `mem_limit: 768m`, `mem_reservation: 192m`, `cpus: 1.0`,
   o healthcheck da seção 3.2, e trocar `depends_on: - redis` para condition `service_healthy`
   (igual ao dev, passo 3.3-7).

### 3.5 Documentação (mesmo commit lógico)

1. **`automation/AGENT.md`**: adicionar subseção curta (≤10 linhas) na parte do stack
   (redis/waha/n8n), título `### Limites e healthchecks (AC-11, 2026-07)`, registrando: limites de
   memória por serviço, endpoints de health usados (`waha /ping` — `/health` é Plus; `n8n /healthz`;
   `redis-cli ping` autenticado), e que unhealthy NÃO reinicia sozinho (Docker sem swarm não
   auto-restarta por health; restart só em OOM/crash).
2. **`docs/MULTITENANT_COMPLETION_PLAN.md`**: na seção global `## Notas de Deploy` (linha 587),
   adicionar AO FINAL da seção uma subseção `### AC-11 (data de hoje) — limites de memória +
   healthchecks no compose` com o comando de aplicação em prod (ver seção 6 desta spec).
3. `docs/LEGADO.md`: **não aplicável** (nenhum legado/mock tocado).

### 3.6 O que fica intocado (checklist negativo)

Portas e mapeamentos; healthchecks de postgres/api; `build:`; `image:`; `command:` do redis;
todas as envs existentes; redes `app`/`proxy` e aliases; volumes; profiles; comentários existentes
(preservar todos).

## 4. Critérios de aceite

1. `docker compose config --quiet` sai com exit 0 (dev).
2. `docker compose -f docker-compose.prod.yml config --quiet` sai com exit 0 (warnings de env vazia são aceitáveis; erro de sintaxe não).
3. No render do config (dev e prod), TODOS os serviços têm `mem_limit` e `mem_reservation` com os valores da tabela 3.1; no prod, `cpus` presente nos 6 serviços.
4. Healthcheck presente em 8/8 serviços do dev e 6/6 do prod, com os testes exatos da seção 3.2.
5. `docker ps` após recreate do dev mostra `(healthy)` em postgres, api, web, crow-nuxt e — com profile automation ativo — redis, waha, n8n.
6. Nenhuma linha de `ports:` alterada em nenhum dos dois arquivos (diff limpo nessas linhas).
7. `depends_on` com condition: dev `crow-nuxt→api healthy`, `n8n→redis healthy`, `waha→n8n started`; prod `n8n→redis healthy`, `waha→n8n started`; os existentes (`api→postgres healthy`, `web→api healthy`) preservados.
8. Redis do prod tem env `REDIS_PASSWORD: ${AUTOMATION_REDIS_PASSWORD}`; healthcheck usa `$$REDIS_PASSWORD` (dois cifrões) nos DOIS arquivos.
9. `automation/AGENT.md` e `docs/MULTITENANT_COMPLETION_PLAN.md` atualizados conforme 3.5.
10. Ambos os compose continuam abaixo de 450 linhas.

## 5. Validação

```powershell
# 1) Lint dos dois arquivos (na raiz do repo)
docker compose config --quiet
docker compose -f docker-compose.prod.yml config --quiet

# 2) Conferir limites renderizados
docker compose config | Select-String -Pattern "mem_limit|mem_reservation|cpus"

# 3) Recreate do dev — AVISAR O USUÁRIO ANTES: recria o web dev (nuxt recompila ~1-3 min)
docker compose up -d
docker compose --profile automation up -d

# 4) Aguardar e conferir health (esperar ~2-3 min pelo start_period do web)
docker compose ps

# 5) Provas pontuais (mesmos testes já validados nesta sessão)
docker exec omni-waha-1 sh -c "curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:3000/ping"   # 200
docker exec omni-n8n-1 sh -c "wget -q -O - http://127.0.0.1:5678/healthz"                            # {"status":"ok"}
docker exec omni-redis-1 sh -c 'redis-cli --no-auth-warning -a "$REDIS_PASSWORD" ping'               # PONG

# 6) Conferir que o limite foi aplicado de fato (bytes; 1073741824 = 1g)
docker inspect omni-waha-1 --format "{{.HostConfig.Memory}}"
```

Sem alteração em `back/` → **não** precisa `--build api`. Sem npm/vitest no host. O passo 3 reinicia o
stack dev do usuário — executar em momento combinado (containers atuais têm 32 h de uptime).

## 6. Notas de Deploy

- **Nenhuma migration. Nenhuma env var nova** (`AUTOMATION_REDIS_PASSWORD` já existe no `.env` da VPS; só passa a ser exposta também como env do container redis).
- **Nenhum rebuild de imagem** (mudança é só de configuração de container).
- Aplicar em prod (USUÁRIO roda, manualmente, na VPS — `/home/deploy/lista-atendimento`):
  ```bash
  docker compose -f docker-compose.prod.yml config --quiet          # lint com o .env real
  docker compose -f docker-compose.prod.yml up -d                   # recria postgres→api→web (ordem via depends_on)
  docker compose -f docker-compose.prod.yml --profile automation up -d   # recria redis→n8n→waha
  docker compose -f docker-compose.prod.yml ps                      # conferir (healthy)
  ```
- **Janela de restart curta** (~30-60 s no core): o `up -d` recria os containers com a nova config
  respeitando as conditions (postgres healthy → api healthy → web). Dados do postgres ficam no volume.
- Sessão WhatsApp da WAHA persiste no volume `automation_waha_sessions` — re-pareamento de QR **não**
  deve ser necessário; conferir status no painel após o restart.
- **NUNCA parar o Caddy da stack omnichannel-mvp** (ele roteia o painel). Este procedimento não o toca.
- Expectativa correta: healthcheck `unhealthy` NÃO reinicia o container sozinho (sem swarm/autoheal);
  ele alimenta `docker ps`, `depends_on` e a monitoração futura (resto do AC-16). Quem reinicia em
  OOM/crash é o `restart: unless-stopped` + `mem_limit`.

## 7. Arquivos tocados

- `docker-compose.yml` (editar)
- `docker-compose.prod.yml` (editar)
- `automation/AGENT.md` (editar — subseção nova, ≤10 linhas)
- `docs/MULTITENANT_COMPLETION_PLAN.md` (editar — entrada nova ao final de `## Notas de Deploy`)

## Regras de execução (obrigatórias para o implementador)

1. **NENHUM comando git** (sessão multi-agente — só o usuário roda git).
2. **NÃO rodar npm/build/generate** sem aprovação do usuário. Aqui `back/` não muda, então nem o
   `docker compose up -d --build api` é necessário; a validação usa `docker compose config` e o
   recreate da seção 5 (avisar o usuário antes do recreate — derruba o dev por 1-3 min).
3. Máx 450 linhas por arquivo (os dois compose ficam bem abaixo).
4. Não remover funcionalidade existente; healthchecks/limits são ADIÇÕES — preservar todos os
   comentários, envs, portas e comportamentos atuais.
5. Zero mock/legado novo; `docs/LEGADO.md` não aplicável nesta spec.
6. Sem migrations nesta spec; se por algum motivo surgir SQL, seria plano/idempotente, sem
   `-- +goose Down`, numeração a partir de 0187.
7. **Portas fixas: api 9091 (host, via `.env`), web 3003, postgres 5432 — proibido alterar
   qualquer `ports:`.**
8. Nunca sobrescrever password_hash/dados de usuário (não aplicável aqui — nenhum dado tocado).
9. Atualizar `automation/AGENT.md` ao final (passo 3.5).
10. Design system/front: não aplicável (sem mudança de front).
