# Staging do Omni — setup (mesma VPS, sob demanda)

> Frente: `deploy-registry-staging` (fase **D4**). Doc canonico do plano:
> `docs/deploy/REGISTRY_STAGING_DEPLOY_PLAN.md` (secoes 3, 4.3, 4.4, 6, 7).
>
> Este documento e' o setup especifico do ambiente de **staging**. O runbook
> consolidado (prod + staging) fica em `docs/DEPLOY_VPS.md` e
> `docs/DEPLOY_CHECKLIST.md` — NAO editar aqui; este arquivo e' aditivo.

## 1. Ideia central

Staging roda na **mesma VPS** que prod, com o **mesmo** `docker-compose.prod.yml`.
O que muda e' so:

- `--env-file .env.staging` (em vez de `.env.production`);
- `COMPOSE_PROJECT_NAME=omni-staging` — isso ja' da namespace proprio a
  containers, **volumes** (`omni-staging_postgres_data`, `omni-staging_api_uploads`,
  ...) e a stack inteira. Staging **nunca** toca dado/volume de prod.

Sobe sob demanda pra testar um SHA candidato, valida, e derruba quando ocioso.
A VPS **nunca compila** — as imagens vem prontas do GHCR (`docker pull`).

## 2. DNS (passo manual do usuario, fora da VPS)

Criar o registro:

```
A   preview   ->   85.31.62.33
```

Criar na **zona AUTORITATIVA do dominio** (ver §7.4 — erro comum). O preview usa
`whenthelightsdie.com`, cuja zona e' gerenciada no painel do `dns-parking`/Hostinger
(NS `ns1/ns2.dns-parking.com`) — mesmo dominio do prod (`lista.whenthelightsdie.com`).
Sem o DNS resolvendo, o Caddy nao consegue emitir o certificado TLS.

## 3. Bloco Caddy — `preview.whenthelightsdie.com`

Adicionar no `/opt/omnichannel/Caddyfile` (mesmo arquivo que ja' tem o bloco de
prod `lista.whenthelightsdie.com`). Mesmo desenho do bloco de prod, trocando o
host e os upstreams para os aliases do staging (`PROXY_API_ALIAS=lista-staging-api`
e `PROXY_WEB_ALIAS=lista-staging-web`, definidos no `.env.staging`):

```caddy
preview.whenthelightsdie.com {
  # Mesma matriz de headers de seguranca do bloco de prod. O connect-src do CSP
  # aponta pro WSS do PROPRIO host de staging.
  header {
    Strict-Transport-Security "max-age=31536000; includeSubDomains"
    X-Content-Type-Options "nosniff"
    X-Frame-Options "SAMEORIGIN"
    Referrer-Policy "strict-origin-when-cross-origin"
    Permissions-Policy "geolocation=(), microphone=(), camera=()"
    Content-Security-Policy "default-src 'self'; img-src 'self' data: https:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self' wss://preview.whenthelightsdie.com;"
  }

  # OPCIONAL: basic auth no edge pra ninguem externo achar/usar o staging.
  # Gere o hash com:  docker run --rm caddy caddy hash-password --plaintext 'SENHA'
  # e descomente o bloco abaixo (cobre todo o host, inclusive a API):
  # basicauth {
  #   staging JDJhJDE0J...hash-gerado...
  # }

  handle /v1/* {
    reverse_proxy lista-staging-api:8080
  }

  handle /uploads/* {
    reverse_proxy lista-staging-api:8080
  }

  handle /healthz {
    reverse_proxy lista-staging-api:8080
  }

  handle {
    reverse_proxy lista-staging-web:3003
  }
}
```

Depois de salvar, recarregar o Caddy (validar antes):

```bash
docker exec <container-caddy> caddy validate --config /etc/caddy/Caddyfile
docker exec <container-caddy> caddy reload   --config /etc/caddy/Caddyfile
```

Observacoes (iguais as do bloco de prod em `docs/DEPLOY_VPS.md`):

- `Strict-Transport-Security` so faz sentido depois de HTTPS estavel no host.
- O CSP assume frontend, API e WebSocket no MESMO host de staging.
- Se algum recurso legitimo do Nuxt parar de carregar, valide no navegador qual
  diretiva bloqueou e ajuste a policy antes de endurecer mais.

## 4. Subir / derrubar sob demanda

> Apenas DOCUMENTACAO — comandos a rodar na VPS pelo usuario. Pre-requisitos:
> DNS (secao 2) + bloco Caddy (secao 3) + `.env.staging` na VPS (a partir do
> `.env.staging.example`, com segredos PROPRIOS) + as imagens ja' publicadas no
> GHCR com o SHA candidato em `IMAGE_TAG`.

Path remoto sugerido (separado de prod): `/home/deploy/lista-atendimento-staging`.

Pull do SHA candidato e subir:

```bash
docker compose --env-file .env.staging -f docker-compose.prod.yml pull api web
docker compose --env-file .env.staging -f docker-compose.prod.yml up -d --no-build api web
docker compose --env-file .env.staging -f docker-compose.prod.yml ps
```

Logs / status:

```bash
docker compose --env-file .env.staging -f docker-compose.prod.yml logs -f api
```

Derrubar quando ocioso (preserva os volumes `omni-staging_*`):

```bash
docker compose --env-file .env.staging -f docker-compose.prod.yml down
```

Ligar o profile `automation` (so se precisar testar o bot; desligado por padrao):

```bash
docker compose --env-file .env.staging -f docker-compose.prod.yml --profile automation up -d
```

Smoke test no host de staging:

```bash
curl -I https://preview.whenthelightsdie.com
curl -I https://preview.whenthelightsdie.com/healthz
```

## 5. Isolamento (nunca tocar dados de prod)

- `COMPOSE_PROJECT_NAME=omni-staging` => containers, rede e **volumes** ganham o
  prefixo `omni-staging_` (`omni-staging_postgres_data`, `omni-staging_api_uploads`,
  `omni-staging_api_erp_storage`, e os `automation_*` se o profile subir). Prod
  continua em `omni_*`. Zero compartilhamento.
- Banco de staging comeca vazio; a api roda `migrate up` no boot (staging vira
  ensaio real das migrations antes da prod). Seed recomendado: bootstrap de owner
  de **teste** limpo (sem PII real). Backup SANITIZADO da prod e' opcao — nunca o
  dump bruto. Ver `.env.staging.example` (topo) e plano secao 6.
- Portas de host distintas (`API_PORT=18081`, `WEB_PORT=13004`) pra nao colidir
  com prod (`18080`/`13003`) na mesma VPS.
- Segredos (`POSTGRES_PASSWORD`, `AUTH_TOKEN_SECRET`, tokens do automation)
  **proprios** do staging — NUNCA os de prod.

## 6. Notas de Deploy desta frente (D2-D4)

Variaveis novas (declaradas no `.example` E referenciadas no
`docker-compose.prod.yml`):

- `API_IMAGE` (default `ghcr.io/mikewade2k16/omni-api`)
- `WEB_IMAGE` (default `ghcr.io/mikewade2k16/omni-web`)
- `IMAGE_TAG` (recebe o SHA do git; em staging fica vazio ate' escolher o candidato)

Staging (`.env.staging`): `COMPOSE_PROJECT_NAME=omni-staging`,
`PROXY_API_ALIAS=lista-staging-api`, `PROXY_WEB_ALIAS=lista-staging-web`,
`API_PORT=18082`, `WEB_PORT=13005`, URLs/CORS em
`preview.whenthelightsdie.com`, segredos proprios.

Passos manuais: DNS `A preview -> 85.31.62.33` (na zona autoritativa!); bloco Caddy
(secao 3) + restart; criar `.env.staging` na VPS a partir do `.example`.

## 7. Caminho VALIDADO em 2026-06-16 (o que funcionou) + armadilhas

Subimos o preview de ponta a ponta neste dia. O caminho que funcionou e as 3
armadilhas que custaram tempo (registradas para nao repetir):

### 7.1 Deploy rapido SEM registry (build local -> SSH -> up), foi o que funcionou

Nao exigiu `docker login` no GHCR em lado nenhum. Da maquina local (Docker Desktop):

```bash
# 1. build local das 2 imagens (tag = o que o compose espera):
docker build -t ghcr.io/mikewade2k16/omni-api:preview ./back
docker build -t ghcr.io/mikewade2k16/omni-web:preview ./web

# 2. manda as imagens prontas pra VPS por SSH (save -> load), sem registry:
docker save ghcr.io/mikewade2k16/omni-api:preview ghcr.io/mikewade2k16/omni-web:preview \
  | gzip | ssh -i ~/.ssh/gh_actions_omnichannel_vps deploy@85.31.62.33 'gunzip | docker load'

# 3. na VPS: IMAGE_TAG=preview no .env.staging, envia o compose, up SEM build e SEM pull:
scp -i ~/.ssh/gh_actions_omnichannel_vps docker-compose.prod.yml \
  deploy@85.31.62.33:/home/deploy/lista-atendimento-staging/docker-compose.prod.yml
ssh -i ~/.ssh/gh_actions_omnichannel_vps deploy@85.31.62.33 \
  "cd /home/deploy/lista-atendimento-staging && \
   sed -i 's|^IMAGE_TAG=.*|IMAGE_TAG=preview|' .env.staging && \
   docker compose --env-file .env.staging -f docker-compose.prod.yml up -d --no-build api web && \
   docker compose --env-file .env.staging -f docker-compose.prod.yml ps"
```

Trade-off vs. GHCR: o `save`/`load` manda a imagem inteira (sem dedup de camada entre
deploys). Para iterar muito, o caminho GHCR (`deploy-fast.ps1`, exige `docker login
ghcr.io` no usuario `deploy` da VPS uma vez) sobe so as camadas que mudaram. Os dois
NAO buildam na VPS. Validacao: `curl http://127.0.0.1:18082/healthz` na VPS = 200.

### 7.2 Armadilha — colisao de porta com outras stacks da VPS

`18081/13004` (escolha inicial do staging) ja estavam tomadas pela stack `omni-crow`
-> a api ficava em `Created` com `port is already allocated`, e o healthz dava 200
batendo na stack ERRADA (falso positivo). Conferir SEMPRE antes:
`docker ps --format '{{.Names}} {{.Ports}}' | grep -oE '127.0.0.1:[0-9]+' | sort -u`.
Preview passou a usar `18082/13005`.

### 7.3 Armadilha — Caddyfile e bind-mount de arquivo unico

`/opt/omnichannel/Caddyfile` e' montado como arquivo unico em `/etc/caddy/Caddyfile`.
`sed -i`/`cp`/editores que renomeiam TROCAM o inode; o container fica preso no inode
antigo e NAO ve a edicao (o `caddy validate` dentro dele continua mostrando o erro
velho). Regras:
- depois de editar o Caddyfile, **`docker restart omnichannel-mvp-caddy-1`** (nao adianta
  so editar nem `caddy reload` via exec — eles leem o inode preso);
- diretiva com bloco precisa ser MULTI-LINHA (`handle /x { \n reverse_proxy ... \n }`);
  uma linha so (`handle /x { ... }`) da `Unexpected next token after '{' on same line`;
- valida o arquivo do host num caddy descartavel COM as envs do caddy real (senao
  `${DOMAIN}` expande vazio e da falso erro `subject does not qualify ... 'lista.'`):
  ```bash
  docker inspect omnichannel-mvp-caddy-1 --format '{{range .Config.Env}}{{println .}}{{end}}' > /tmp/caddy.env
  IMG=$(docker inspect omnichannel-mvp-caddy-1 --format '{{.Config.Image}}')
  docker run --rm --env-file /tmp/caddy.env --entrypoint caddy \
    -v /opt/omnichannel/Caddyfile:/etc/caddy/Caddyfile:ro "$IMG" \
    validate --config /etc/caddy/Caddyfile --adapter caddyfile
  ```

### 7.4 Armadilha — registro DNS na zona ERRADA (o que mais custou tempo)

Tentamos primeiro `preview.crowvisuals.com.br`, mas a zona de `crowvisuals.com.br` e'
autoritativa na **HostGator** (`ns20/ns21.hostgator.com.br`) e o registro foi criado em
OUTRO painel, entao dava `NXDOMAIN` (o Caddy nao emitia o cert: erro
`challenge failed ... NXDOMAIN looking up A`). Lição: **registro DNS so vale na zona
autoritativa do dominio** — confira de quem e' a zona com
`nslookup -type=NS <dominio>` e teste o registro direto no NS autoritativo
(`nslookup <fqdn> <ns-autoritativo>`), nao so num resolver publico (cache).

**Resolucao:** trocamos para `preview.whenthelightsdie.com` — mesmo dominio do prod
(`lista.whenthelightsdie.com`), cuja zona e' gerenciada no `dns-parking`/Hostinger
(`ns1/ns2.dns-parking.com`), painel que o usuario controla facil. Resolveu na hora e o
cert foi emitido. Conferir: `nslookup preview.whenthelightsdie.com ns1.dns-parking.com`
(tem que dar `85.31.62.33`). Os identificadores INTERNOS continuam "staging"
(`COMPOSE_PROJECT_NAME=omni-staging`, `.env.staging`, aliases `lista-staging-*`); so o
host publico e' `preview.whenthelightsdie.com`.
