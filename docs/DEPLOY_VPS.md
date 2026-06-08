# Deploy na VPS

Este e o playbook completo de deploy. Para o checklist operacional curto do dia-a-dia (comandos prontos, ordem recomendada), veja [DEPLOY_CHECKLIST.md](DEPLOY_CHECKLIST.md).

> Historico: a pasta `docs_depoy/` foi consolidada em 2026-05-18. Conteudo vivo migrou para este arquivo e para `DEPLOY_CHECKLIST.md`.

## O que faz sentido para este projeto

Este repositorio sobe sozinho, com stack propria:

- `postgres`
- `api`
- `web`

O deploy de producao deste projeto usa:

- `docker-compose.prod.yml`
- `.env.production`
- um diretorio dedicado na VPS, neste ambiente auditado: `/home/deploy/lista-atendimento`
- o mesmo acesso SSH e o mesmo Docker Engine que voce ja usa no outro projeto
- o proxy reverso que ja existe na VPS, desde que ele encaminhe as rotas deste app para `web` e `api`

## Estado real auditado da VPS

Auditoria feita em `2026-04-23` via SSH na VPS `85.31.62.33`:

- host: `srv1507028`
- sistema: Ubuntu `24.04.4 LTS`
- usuario de deploy validado: `deploy`
- Docker Engine instalado e operacional
- stack atual em producao: `omnichannel-mvp`
- proxy atual: container `omnichannel-mvp-caddy-1`
- arquivo de configuracao do proxy atual: `/opt/omnichannel/Caddyfile`
- rede Docker usada pelo proxy e pelos servicos atuais: `omnichannel-mvp_default`
- espaco livre em disco: cerca de `72 GB`
- memoria disponivel no momento da auditoria: cerca de `6.2 GiB`

Conclusao pratica:

- nao devemos subir outro proxy neste repositorio
- nao devemos disputar `80/443`
- o encaixe correto deste app e entrar na rede `omnichannel-mvp_default` so para proxy, mantendo uma rede privada propria para `postgres`

## O que dos docs antigos nao se aplica aqui

Nada disso faz parte deste repositorio:

- `redis`
- `plataforma-api`
- `painel-web`
- `atendimento-online-api`
- `worker`
- `retencao-worker`
- `whatsapp-evolution-gateway`
- `caddy` como servico deste compose
- deploy automatico de `main` por GitHub Actions
- rota hospedada em `/admin/fila-atendimento`
- banco compartilhado com schema dentro de outra plataforma

Se esse app for subir na mesma VPS, ele sobe como stack separada e isolada.

## Dominio recomendado

Para este codigo, o desenho mais simples e direto e um unico host publico:

- app e api: `https://lista.whenthelightsdie.com`

Motivo tecnico:

- o frontend chama a API com caminhos absolutos como `/v1/...`
- o frontend monta o WebSocket em `/v1/realtime/...`
- a API ja expõe as rotas publicas tecnicas em prefixos proprios: `/v1/*`, `/uploads/*` e `/healthz`

Entao o proxy pode encaminhar:

- `/v1/*` -> API
- `/uploads/*` -> API
- `/healthz` -> API
- todo o resto -> frontend

Isso evita CORS cross-origin e nao exige segundo subdominio publico para a API.

## DNS que voce precisa criar

No painel DNS do dominio:

- registro `A` para `lista` -> `85.31.62.33`

O registro `@` atual apontando para outro IP nao precisa ser alterado para este deploy.

## O que fica isolado do outro app

- `COMPOSE_PROJECT_NAME=omni` separa containers, rede e volumes
- banco proprio deste app
- volume proprio do banco
- volume proprio de uploads
- rede privada propria do app
- aliases proprios na rede compartilhada do proxy
- portas locais proprias no host para debug e curl do host
- arquivo `.env.production` proprio

O que nao deve ser compartilhado:

- container de PostgreSQL
- volume de banco
- segredo JWT
- credenciais SMTP
- volumes de upload

## Arquivos de producao deste repo

- `docker-compose.prod.yml`
- `.env.production`
- `.env.production.example`

## Variaveis principais

Use `.env.production.example` como base.

As variaveis mais importantes sao:

- `COMPOSE_PROJECT_NAME=omni`
- `POSTGRES_DB=omni`
- `POSTGRES_USER=omni`
- `POSTGRES_PASSWORD=<senha-forte>`
- `PROXY_NETWORK_NAME=omnichannel-mvp_default`
- `PROXY_API_ALIAS=lista-api`
- `PROXY_WEB_ALIAS=lista-web`
- `WEB_APP_URL=https://lista.whenthelightsdie.com`
- `NUXT_PUBLIC_API_BASE=https://lista.whenthelightsdie.com`
- `NUXT_PUBLIC_API_WS_BASE=wss://lista.whenthelightsdie.com`
- `NUXT_API_INTERNAL_BASE=http://api:8080`
- `CORS_ALLOWED_ORIGINS=https://lista.whenthelightsdie.com`
- `AUTH_TOKEN_SECRET=<segredo-longo-e-aleatorio>`

Observacao importante para a Fase 4:

- o path remoto `/home/deploy/lista-atendimento` pode continuar igual nesta janela para evitar churn extra de filesystem;
- o rename do banco em producao fica detalhado em [deploy/db-rename.md](deploy/db-rename.md).

## Portas locais desta stack

No compose de producao atual:

- frontend publicado em `127.0.0.1:13003`
- api publicada em `127.0.0.1:18080`
- postgres nao e publicado externamente

Isso evita colisao direta com a outra stack da VPS.

## Proxy reverso

Este repositorio nao sobe proxy proprio. Ele assume que a VPS ja tem um proxy central.

No ambiente real auditado, esse proxy central e um `caddy` em container no outro projeto.
Por isso, o `docker-compose.prod.yml` deste repositorio agora conecta `web` e `api` na rede externa `omnichannel-mvp_default` com aliases dedicados:

- `lista-web`
- `lista-api`

### Se o proxy atual roda no host

Exemplo de Nginx:

```nginx
server {
    server_name lista.whenthelightsdie.com;

    location /v1/ {
        proxy_pass http://127.0.0.1:18080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }

    location /uploads/ {
        proxy_pass http://127.0.0.1:18080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location = /healthz {
        proxy_pass http://127.0.0.1:18080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location / {
        proxy_pass http://127.0.0.1:13003;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

### Proxy real desta VPS

O `Caddyfile` atual do outro projeto ja publica hosts como:

- `app.${DOMAIN}` -> `painel-web:3000`
- `api.${DOMAIN}` -> `atendimento-online-api:4000`
- `evo.${DOMAIN}` -> `whatsapp-evolution-gateway:8080`

Para este repositorio, a integracao correta e adicionar um novo bloco no arquivo `/opt/omnichannel/Caddyfile`:

```caddy
lista.whenthelightsdie.com {
    handle /v1/* {
        reverse_proxy lista-api:8080
    }

    handle /uploads/* {
        reverse_proxy lista-api:8080
    }

    handle /healthz {
        reverse_proxy lista-api:8080
    }

    handle {
        reverse_proxy lista-web:3003
    }
}
```

### Headers de seguranca

Dentro do mesmo host `lista.whenthelightsdie.com`, adicione a matriz base abaixo antes dos blocos `handle`:

```caddy
header {
  Strict-Transport-Security "max-age=31536000; includeSubDomains"
  X-Content-Type-Options "nosniff"
  X-Frame-Options "SAMEORIGIN"
  Referrer-Policy "strict-origin-when-cross-origin"
  Permissions-Policy "geolocation=(), microphone=(), camera=()"
  Content-Security-Policy "default-src 'self'; img-src 'self' data: https:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self' wss://lista.whenthelightsdie.com;"
}
```

Exemplo completo com o bloco `header` ja encaixado:

```caddy
lista.whenthelightsdie.com {
  header {
    Strict-Transport-Security "max-age=31536000; includeSubDomains"
    X-Content-Type-Options "nosniff"
    X-Frame-Options "SAMEORIGIN"
    Referrer-Policy "strict-origin-when-cross-origin"
    Permissions-Policy "geolocation=(), microphone=(), camera=()"
    Content-Security-Policy "default-src 'self'; img-src 'self' data: https:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self' wss://lista.whenthelightsdie.com;"
  }

  handle /v1/* {
    reverse_proxy lista-api:8080
  }

  handle /uploads/* {
    reverse_proxy lista-api:8080
  }

  handle /healthz {
    reverse_proxy lista-api:8080
  }

  handle {
    reverse_proxy lista-web:3003
  }
}
```

Observacoes praticas:

- `Strict-Transport-Security` so faz sentido depois de HTTPS estavel no host publico.
- o `Content-Security-Policy` acima assume frontend, API e WebSocket no mesmo host `lista.whenthelightsdie.com`;
- se algum recurso legitimo do Nuxt parar de carregar depois da mudanca, valide no navegador qual diretiva bloqueou e ajuste a policy antes de endurecer mais.

Esse desenho mantem tudo em `lista.whenthelightsdie.com`, inclusive API e WebSocket, sem abrir outro subdominio publico.

## O que eu consigo fazer so por SSH

Consigo fazer por SSH:

- inspecionar a VPS
- confirmar containers, portas e redes atuais
- clonar este repo em um diretorio dedicado
- criar `.env.production`
- subir os containers deste projeto
- ajustar o proxy reverso que ja existe na VPS
- validar logs, healthcheck e acesso HTTP/HTTPS

Nao consigo fazer so por SSH, a menos que voce me de acesso ao provedor DNS:

- criar os registros DNS no painel do dominio

Entao, na pratica:

- DNS e a parte que normalmente fica manual fora da VPS
- o resto eu consigo tocar pela VPS sim

## Preparacao minima da VPS

Se a VPS ja sobe o outro projeto por Docker, provavelmente quase tudo ja existe.

Ainda assim, precisamos confirmar:

- `docker`
- `docker compose`
- `git`
- permissao do usuario SSH para rodar Docker
- um diretorio para este repo, neste ambiente: `/home/deploy/lista-atendimento`

Observacao real da VPS auditada:

- `/srv` existe, mas exige `sudo` para preparar o diretorio
- o usuario `deploy` tem escrita em `/home/deploy`
- para o primeiro deploy manual sem `git`, o alvo correto e `/home/deploy/lista-atendimento`

## Bootstrap inicial sem Git

Para este primeiro deploy, o caminho oficial validado neste ambiente e sincronizar o workspace local por `tar` sobre SSH, sem depender de branch, commit ou clone remoto.

Motivo pratico:

- `scp -r` falhou neste Windows com este workspace
- `rsync` nao estava disponivel no fluxo real usado aqui
- `tar` sobre SSH funcionou de ponta a ponta e preservou o `.env.production` remoto porque ele nao vai no pacote local

Preparacao do diretorio remoto:

```bash
mkdir -p /home/deploy/lista-atendimento
```

Sincronizacao a partir da maquina local:

```bash
tar -czf - \
    --exclude='.git' \
    --exclude='.env' \
    --exclude='.env.production' \
    --exclude='node_modules' \
    --exclude='web/node_modules' \
    --exclude='web/.nuxt' \
    --exclude='web/.output' \
    --exclude='web/dist' \
    --exclude='back/.logs' \
    --exclude='qa-bot/.venv' \
    --exclude='qa-bot/artifacts' \
    --exclude='Controlle10 - ftp' \
    --exclude='tmp' \
    . | ssh -i c:/Users/Mike/.ssh/gh_actions_omnichannel_vps \
    -o StrictHostKeyChecking=accept-new \
    deploy@85.31.62.33 \
    "mkdir -p /home/deploy/lista-atendimento && \
    find /home/deploy/lista-atendimento -mindepth 1 -maxdepth 1 ! -name '.env.production' ! -name 'backups' -exec rm -rf {} + && \
    tar -xzf - -C /home/deploy/lista-atendimento"
```

Subida na VPS:

```bash
cd /home/deploy/lista-atendimento
cp .env.production.example .env.production
docker compose --env-file .env.production -f docker-compose.prod.yml config
docker compose --env-file .env.production -f docker-compose.prod.yml up -d --build
docker compose --env-file .env.production -f docker-compose.prod.yml ps
```

## Primeiro acesso sem seed demo

Em producao, o backend agora pula as migrations de seed demo.
Isso evita subir usuarios, consultores e senhas de exemplo no ambiente real.

Em troca, o primeiro acesso precisa de um bootstrap explicito do owner inicial do tenant.

Depois do `up -d`, rode uma vez:

```bash
cd /home/deploy/lista-atendimento
docker compose --env-file .env.production -f docker-compose.prod.yml run --rm \
    -e BOOTSTRAP_TENANT_SLUG=whenthelightsdie \
    -e BOOTSTRAP_TENANT_NAME='When The Lights Die' \
    -e BOOTSTRAP_STORE_CODE=MATRIZ \
    -e BOOTSTRAP_STORE_NAME='Loja Matriz' \
    -e BOOTSTRAP_STORE_CITY='Aracaju' \
    -e BOOTSTRAP_OWNER_NAME='Owner Inicial' \
    -e BOOTSTRAP_OWNER_EMAIL='seu-email@whenthelightsdie.com' \
    -e BOOTSTRAP_OWNER_PASSWORD='troque-essa-senha-agora' \
    api sh -lc 'migrate bootstrap-owner'
```

Esse comando cria ou atualiza de forma idempotente:

- o tenant inicial
- a primeira loja
- o usuario owner inicial com senha definida

Com isso, o primeiro deploy sobe sem seed demo e com acesso inicial controlado por voce.

### Bootstrap automatico da loja ERP 184

A workspace ERP MVP consulta sempre a loja raiz `184`. Em producao, o migrator pula
o seed `0036_seed_dev_erp_store_184.sql`, entao o container da API roda um passo
idempotente no startup: `migrate up && migrate bootstrap-erp-store && api`.

Com `ERP_BOOTSTRAP_STORE_CODE=184`, o deploy cria ou reativa a loja ERP no tenant
definido por `ERP_BOOTSTRAP_TENANT_SLUG` ou `ERP_BOOTSTRAP_TENANT_ID`. Se nenhum
dos dois estiver preenchido, o comando usa automaticamente o unico tenant ativo;
com zero ou multiplos tenants ativos, ele apenas registra skip no log e deixa a API
subir.

No primeiro go-live, se o `bootstrap-owner` for executado depois que a API ja
subiu, reinicie a API para rodar o bootstrap ERP:

```bash
docker compose --env-file .env.production -f docker-compose.prod.yml restart api
```

Para conferir:

```bash
docker compose --env-file .env.production -f docker-compose.prod.yml exec -T postgres \
  sh -lc 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB"' <<'SQL'
select t.slug, s.code, s.name, s.is_active
from stores s
join tenants t on t.id = s.tenant_id
where s.code = '184';
SQL
```

Se o botao de bootstrap/importacao manual for usado em producao, a pasta dos
consolidados tambem precisa existir no host configurado por
`ERP_SOURCE_HOST_DIR` e `ERP_ALLOW_MANUAL_SYNC` precisa estar `true` no
`.env.production`. No baseline operacional atual (`2026-05-08`), o ERP ficou
com sync manual ligado e scheduler FTP diario ativo em producao.

### Carga ERP por dump de banco

Quando os consolidados ja foram importados no Postgres local, nao subir a pasta
`erp-source-local` (antiga `Controlle10 - ftp`) para a VPS. Gere e transfira apenas um dump comprimido das
tabelas `erp_*`. Em 2026-04-29, os markdowns tinham cerca de 430 MB e o dump
custom das tabelas ERP ficou com cerca de 111 MB.

Tabelas do dump:

- `erp_sync_runs`
- `erp_sync_files`
- `erp_item_raw`
- `erp_customer_raw`
- `erp_employee_raw`
- `erp_order_raw`
- `erp_order_canceled_raw`
- `erp_item_current`
- `erp_export_outbox`

Fluxo usado em producao:

1. Gerar o dump local no container Postgres:

```bash
mkdir -p tmp
docker compose --env-file .env.docker exec -T postgres pg_dump \
  -U omni -d omni \
  -Fc --data-only --no-owner --no-privileges \
  -t public.erp_sync_runs \
  -t public.erp_sync_files \
  -t public.erp_item_raw \
  -t public.erp_customer_raw \
  -t public.erp_employee_raw \
  -t public.erp_order_raw \
  -t public.erp_order_canceled_raw \
  -t public.erp_item_current \
  -t public.erp_export_outbox \
  -f /tmp/erp_data.dump
docker cp omni-postgres-1:/tmp/erp_data.dump ./tmp/erp_data.dump
```

2. Enviar o dump para a VPS:

```bash
scp -i c:/Users/Mike/.ssh/gh_actions_omnichannel_vps \
  ./tmp/erp_data.dump \
  deploy@85.31.62.33:/home/deploy/lista-atendimento/tmp/erp_data.dump
```

3. Antes de restaurar, criar backup completo remoto:

```bash
cd /home/deploy/lista-atendimento
backup="backups/pre_erp_restore_$(date +%Y%m%d_%H%M%S).sql.gz"
docker compose --env-file .env.production -f docker-compose.prod.yml exec -T postgres \
  sh -lc 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB"' | gzip > "$backup"
ls -lh "$backup"
```

4. Confirmar que a loja `184` remota esta alinhada ao dump local.
   No snapshot de 2026-04-29, o dump local usa:

```text
tenant_id = aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa
store_id  = bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbb0184
storeCode = 184
```

Se a VPS tiver criado a loja `184` com outro UUID e ela nao tiver referencias em
outras tabelas, alinhar antes do restore:

```sql
update stores
set id = 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbb0184'::uuid,
    updated_at = now()
where tenant_id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'::uuid
  and code = '184';
```

5. Limpar somente as tabelas ERP e restaurar:

```bash
cd /home/deploy/lista-atendimento
container=$(docker compose --env-file .env.production -f docker-compose.prod.yml ps -q postgres)
docker cp tmp/erp_data.dump "$container:/tmp/erp_data.dump"

docker compose --env-file .env.production -f docker-compose.prod.yml exec -T postgres \
  psql -U omni -d omni -v ON_ERROR_STOP=1 <<'SQL'
truncate table
  erp_export_outbox,
  erp_item_current,
  erp_order_canceled_raw,
  erp_order_raw,
  erp_employee_raw,
  erp_customer_raw,
  erp_item_raw,
  erp_sync_files,
  erp_sync_runs;
SQL

docker compose --env-file .env.production -f docker-compose.prod.yml exec -T postgres \
  pg_restore -U omni -d omni \
  --data-only --no-owner --no-privileges \
  --single-transaction --exit-on-error /tmp/erp_data.dump
```

6. Validar os contadores:

```sql
select
  (select count(*) from erp_sync_runs) as runs,
  (select count(*) from erp_sync_files) as files,
  (select count(*) from erp_item_raw) as item_raw,
  (select count(*) from erp_item_current) as item_current,
  (select count(*) from erp_customer_raw) as customer_raw,
  (select count(*) from erp_employee_raw) as employee_raw,
  (select count(*) from erp_order_raw) as order_raw,
  (select count(*) from erp_order_canceled_raw) as order_canceled_raw;
```

Resultado esperado do restore feito em 2026-04-29:

```text
runs=11
files=4255
item_raw=1101126
item_current=355088
customer_raw=221764
employee_raw=10219
order_raw=376044
order_canceled_raw=21648
```

7. Apagar os dumps temporarios depois da validacao:

```bash
rm -f /home/deploy/lista-atendimento/tmp/erp_data.dump
docker compose --env-file .env.production -f docker-compose.prod.yml exec -T postgres \
  rm -f /tmp/erp_data.dump
rm -f ./tmp/erp_data.dump
```

Resultado validado do restore feito em `2026-05-08`:

```text
runs=54
files=7227
item_raw=1526022
item_current=357619
customer_raw=339174
employee_raw=20695
order_raw=757617
order_canceled_raw=43775
```

Estado final do ERP em producao apos esse restore:

- API em `ERP_SOURCE_KIND=ftp`
- `ERP_SOURCE_RECURSIVE=false`, mantendo a rotina diaria olhando apenas a raiz `extract_files`
- `ERP_ALLOW_MANUAL_SYNC=true`, permitindo disparo pelo painel `/erp`
- `ERP_SYNC_AUTOMATIC_ENABLED=true`, `ERP_SYNC_INTERVAL=24h`, `ERP_SYNC_HOUR_UTC=4`
- scheduler confirmado em log por `erp_sync_scheduler_started`
- sync manual em producao validado com sucesso depois da ativacao
- dump temporario apagado do host local, do host remoto e do container Postgres remoto apos a validacao

## Integracao do Caddy atual

Depois que os containers deste repo estiverem no ar, adicione o bloco de `lista.whenthelightsdie.com` em `/opt/omnichannel/Caddyfile` e reaplique o proxy do outro projeto:

```bash
cd /opt/omnichannel
docker compose -f docker-compose.yml -f docker-compose.prod.yml --profile channels --env-file .env.prod up -d caddy
```

## Atualizacao manual

```bash
tar -czf - \
    --exclude='.git' \
    --exclude='.env' \
    --exclude='.env.production' \
    --exclude='node_modules' \
    --exclude='web/node_modules' \
    --exclude='web/.nuxt' \
    --exclude='web/.output' \
    --exclude='web/dist' \
    --exclude='back/.logs' \
    --exclude='qa-bot/.venv' \
    --exclude='qa-bot/artifacts' \
    --exclude='Controlle10 - ftp' \
    --exclude='tmp' \
    . | ssh -i c:/Users/Mike/.ssh/gh_actions_omnichannel_vps \
    -o StrictHostKeyChecking=accept-new \
    deploy@85.31.62.33 \
    "find /home/deploy/lista-atendimento -mindepth 1 -maxdepth 1 ! -name '.env.production' ! -name 'backups' -exec rm -rf {} + && \
    tar -xzf - -C /home/deploy/lista-atendimento"

ssh -i c:/Users/Mike/.ssh/gh_actions_omnichannel_vps \
    -o StrictHostKeyChecking=accept-new \
    deploy@85.31.62.33 \
    "cd /home/deploy/lista-atendimento && \
    docker compose --env-file .env.production -f docker-compose.prod.yml config && \
    docker compose --env-file .env.production -f docker-compose.prod.yml up -d --build && \
    docker compose --env-file .env.production -f docker-compose.prod.yml ps"
```

## Redeploy rapido validado

Para o proximo deploy normal, a maior parte do trabalho pesado ja passou.
Se nao houver troca de dominio, mudanca no proxy central ou restauracao/importacao de dados, o fluxo fica reduzido a:

1. sincronizar o codigo para `/home/deploy/lista-atendimento`
2. validar o compose com o `.env.production` que ja esta na VPS
3. subir `api` e `web` com rebuild
4. fazer smoke test HTTP e healthcheck

Na pratica, o proximo deploy tende a levar minutos, nao a janela inteira do primeiro go-live.
O que normalmente nao precisa mais repetir:

- DNS do subdominio
- bloco do `lista.whenthelightsdie.com` no Caddy, salvo se o proxy mudar
- `bootstrap-owner`
- importacao manual de usuarios
- restauracao completa do banco

Sequencia curta recomendada:

```bash
tar -czf - \
    --exclude='.git' \
    --exclude='.env' \
    --exclude='.env.production' \
    --exclude='node_modules' \
    --exclude='web/node_modules' \
    --exclude='web/.nuxt' \
    --exclude='web/.output' \
    --exclude='web/dist' \
    --exclude='back/.logs' \
    --exclude='qa-bot/.venv' \
    --exclude='qa-bot/artifacts' \
    --exclude='Controlle10 - ftp' \
    --exclude='tmp' \
    . | ssh -i c:/Users/Mike/.ssh/gh_actions_omnichannel_vps \
    -o StrictHostKeyChecking=accept-new \
    deploy@85.31.62.33 \
    "find /home/deploy/lista-atendimento -mindepth 1 -maxdepth 1 ! -name '.env.production' ! -name 'backups' -exec rm -rf {} + && \
    tar -xzf - -C /home/deploy/lista-atendimento"

ssh -i c:/Users/Mike/.ssh/gh_actions_omnichannel_vps \
    -o StrictHostKeyChecking=accept-new \
    deploy@85.31.62.33 \
    "cd /home/deploy/lista-atendimento && \
    docker compose --env-file .env.production -f docker-compose.prod.yml up -d --build && \
    docker compose --env-file .env.production -f docker-compose.prod.yml ps && \
    curl -I https://lista.whenthelightsdie.com && \
    curl -I https://lista.whenthelightsdie.com/healthz"
```

Se o release tocar schema, migrations ou dados, faca backup antes:

```bash
ssh -i c:/Users/Mike/.ssh/gh_actions_omnichannel_vps \
    -o StrictHostKeyChecking=accept-new \
    deploy@85.31.62.33 \
    "mkdir -p /home/deploy/lista-atendimento/backups && \
    cd /home/deploy/lista-atendimento && \
    docker compose --env-file .env.production -f docker-compose.prod.yml exec -T postgres \
    sh -lc 'pg_dump -U \"$POSTGRES_USER\" -d \"$POSTGRES_DB\"' | gzip > \
    backups/backup_$(date +%Y%m%d_%H%M%S).sql.gz"
```

### Script local recomendado

Para o fluxo diario na sua maquina Windows, a entrada recomendada agora e:

```bash
npm run prod:deploy:vps
```

Esse script usa o metodo validado neste ambiente:

- empacota o workspace local por `tar`
- exclui a pasta local `erp-source-local` (e mantém `Controlle10 - ftp` como defesa) do payload antes do envio
- limpa o diretorio remoto preservando `.env.production` e `backups`
- envia o codigo por SSH
- valida `docker compose`
- sobe `api` e `web` com rebuild
- executa smoke tests publicos

Comandos uteis:

```bash
npm run prod:deploy:vps
npm run prod:deploy:vps -- -Services api
npm run prod:deploy:vps -- -ForceRecreate
npm run prod:deploy:vps -- -BackupDatabase
```

Arquivo do script:

- `scripts/deploy/deploy-vps-fast.ps1`

### Workflow manual por GitHub Actions

Tambem existe um workflow manual por Git/SSH neste repositorio:

- `.github/workflows/deploy-vps.yml`

Ele nao faz bootstrap da VPS nem cria `.env.production` pela primeira vez.
Ele assume que estes itens ja existem no host remoto e serve para redeploy e rollback controlado por `git_ref`.

Secret necessario no GitHub:

- `DEPLOY_VPS_SSH_KEY`

Inputs disponiveis no `workflow_dispatch`:

- `git_ref`
- `services`
- `backup_database`
- `force_recreate`
- `skip_smoke_tests`

O workflow reutiliza exatamente o mesmo fluxo do script local:

- sync por `tar` + SSH
- limpeza remota preservando `.env.production` e `backups`
- `docker compose config`
- `docker compose up -d --build`
- smoke tests em `https://lista.whenthelightsdie.com`

## Pendencias para o proximo deploy

Itens que precisam ser revistos antes de subir o release atual para a VPS.
Atualizar esta lista a cada release que toca em schema ou regra que precisa
de tratamento manual no momento do deploy.

### Release multitenant-complete (branch refactor/multi-tenant-complete) — 2026-06-08

Este e' o release mais critico ja feito: 37 novas migrations (0100-0136), reescrita
de FKs e DROP de tabelas publicas. Leia inteiro antes de executar qualquer passo.

#### Risco principal: migration 0136 e' IRREVERSIVEL sem backup

`0136_drop_public_tenant_tables.sql` dropa `public.tenants`, `public.users`,
`public.stores` e `public.consultants` depois de repontrar todas as FKs para `core.*`.
**Sem backup pre-deploy nao ha rollback.**

#### Armadilha 0124 — core.account_modules fica vazio no primeiro boot

`0124_core_account_modules_seed.sql` usa `CROSS JOIN core.modules` — mas
`core.modules` e' populado pelo `SyncCatalog` que roda DEPOIS que as migrations
completam. Resultado: a migration insere 0 linhas. O guard de modulos (`RequireModuleByPath`)
vai bloquear tudo ate que o reseed seja feito manualmente (passo 7 abaixo).

#### Sequencia obrigatoria de deploy

**Passo 1 — Confirmar que as migrations anteriores (0001-0059) ja existem na VPS.**
A VPS deve ter o historico ate 0059 (baseline pre-multitenant). Verificar:

```bash
ssh -i ~/.ssh/gh_actions_omnichannel_vps -o StrictHostKeyChecking=accept-new \
  deploy@85.31.62.33 \
  "cd /home/deploy/lista-atendimento && \
   docker compose --env-file .env.production -f docker-compose.prod.yml exec -T postgres \
   psql -U omni -d omni -c 'select version from schema_migrations order by version desc limit 5;'"
```

Esperado: ultima versao e' `0059` ou superior (mas menor que `0100`).

**Passo 2 — Backup completo do banco ANTES de qualquer coisa.**

```bash
npm run prod:deploy:vps -- -BackupDatabase -SkipComposeConfig -SkipSmokeTests -Services postgres
```

Ou manualmente via SSH:

```bash
ssh -i ~/.ssh/gh_actions_omnichannel_vps -o StrictHostKeyChecking=accept-new \
  deploy@85.31.62.33 \
  "cd /home/deploy/lista-atendimento && \
   mkdir -p backups && \
   docker compose --env-file .env.production -f docker-compose.prod.yml exec -T postgres \
   sh -lc 'pg_dump -U \"\$POSTGRES_USER\" -d \"\$POSTGRES_DB\" -Fc' \
   > backups/backup_pre_multitenant_\$(date +%Y%m%d_%H%M%S).dump && \
   ls -lh backups/"
```

Confirmar que o arquivo `.dump` foi criado e tem tamanho > 0.

**Passo 3 — Conferir divergencia de password_hash entre public.users e core.users.**

Se a seed 0101 ja rodou na VPS em algum momento anterior, `core.users` pode ter
`password_hash` stale (congelado no seed). A migration 0136 vai transformar
`public.users` em view de `core.users` — se o hash do `core.users` for diferente
do `public.users`, o owner fica trancado fora.

```bash
ssh -i ~/.ssh/gh_actions_omnichannel_vps -o StrictHostKeyChecking=accept-new \
  deploy@85.31.62.33 \
  "cd /home/deploy/lista-atendimento && \
   docker compose --env-file .env.production -f docker-compose.prod.yml exec -T postgres \
   psql -U omni -d omni -c \
   'select p.id, p.email,
           left(p.password_hash,20) as pub_hash,
           left(c.password_hash,20) as core_hash,
           (p.password_hash = c.password_hash) as match
    from public.users p
    join core.users c on c.id = p.id
    where p.password_hash != c.password_hash;'"
```

Se retornar linhas: **reconciliar antes de prosseguir** com:

```sql
update core.users c
set password_hash = p.password_hash
from public.users p
where p.id = c.id
  and p.password_hash != c.password_hash;
```

**Passo 4 — Sincronizar o codigo para a VPS (sem subir ainda).**

```bash
npm run prod:deploy:vps -- -SkipSmokeTests -Services api
```

Isso faz o sync do codigo mas o compose up vai rodar as migrations automaticamente
ao subir a API. Aguardar o passo seguinte antes.

Ou, se preferir separar sync de deploy:

```bash
# So sync (sem up):
cd ~/Documents/Projects/fila-atendimento
tar -czf - \
    --exclude='.git' --exclude='.claude' --exclude='.env' --exclude='.env.production' \
    --exclude='node_modules' --exclude='web/node_modules' \
    --exclude='web/.nuxt' --exclude='web/.output' --exclude='web/dist' \
    --exclude='back/.logs' --exclude='qa-bot/.venv' \
    --exclude='erp-source-local' --exclude='Controlle10 - ftp' --exclude='tmp' \
    . | ssh -i ~/.ssh/gh_actions_omnichannel_vps -o StrictHostKeyChecking=accept-new \
    deploy@85.31.62.33 \
    "find /home/deploy/lista-atendimento -mindepth 1 -maxdepth 1 \
     ! -name '.env.production' ! -name 'backups' -exec rm -rf {} + && \
     tar -xzf - -C /home/deploy/lista-atendimento"
```

**Passo 5 — Subir a API (migrations rodam automaticamente no startup).**

```bash
ssh -i ~/.ssh/gh_actions_omnichannel_vps -o StrictHostKeyChecking=accept-new \
  deploy@85.31.62.33 \
  "cd /home/deploy/lista-atendimento && \
   docker compose --env-file .env.production -f docker-compose.prod.yml up -d --build api"
```

**Passo 6 — Acompanhar o log da API ate as migrations completarem.**

```bash
ssh -i ~/.ssh/gh_actions_omnichannel_vps -o StrictHostKeyChecking=accept-new \
  deploy@85.31.62.33 \
  "cd /home/deploy/lista-atendimento && \
   docker compose --env-file .env.production -f docker-compose.prod.yml logs -f --tail=100 api"
```

Aguardar mensagem indicando que as migrations rodaram e a API esta ouvindo.
Sinais de sucesso: `migrations applied`, `server listening`, `healthz ok`.

**Passo 7 — Reseed obrigatorio de core.account_modules (armadilha 0124).**

Depois que a API esta saudavel (`/healthz` retorna 200), o `SyncCatalog` ja
populou `core.modules`. Rodar o reseed:

```bash
ssh -i ~/.ssh/gh_actions_omnichannel_vps -o StrictHostKeyChecking=accept-new \
  deploy@85.31.62.33 \
  "cd /home/deploy/lista-atendimento && \
   docker compose --env-file .env.production -f docker-compose.prod.yml exec -T postgres \
   psql -U omni -d omni -c \
   'insert into core.account_modules (account_id, module_id, enabled)
    select a.id, m.id, true
    from core.accounts a
    cross join core.modules m
    where a.is_active = true
    on conflict (account_id, module_id) do nothing;
    select count(*) as rows_inserted from core.account_modules;'"
```

Esperado: `rows_inserted` > 0. Se retornar 0 na segunda query, verificar se
`core.modules` esta' populado: `select count(*) from core.modules;`.

**Passo 8 — Subir o frontend.**

```bash
ssh -i ~/.ssh/gh_actions_omnichannel_vps -o StrictHostKeyChecking=accept-new \
  deploy@85.31.62.33 \
  "cd /home/deploy/lista-atendimento && \
   docker compose --env-file .env.production -f docker-compose.prod.yml up -d --build web"
```

**Passo 9 — Smoke tests finais.**

```bash
# Publico
curl -I https://lista.whenthelightsdie.com
curl -I https://lista.whenthelightsdie.com/healthz

# Headers de seguranca
curl -sI https://lista.whenthelightsdie.com \
  | grep -Ei 'strict-transport|x-content-type|x-frame|referrer|permissions|content-security'
```

**Passo 10 — Smoke de browser (voce faz manualmente):**
- Login com owner da Pérola → navegar em /tasks, /operacao, /erp → tudo carrega (200, sem 403)
- Verificar console DevTools: zero erros de CORS ou 403

#### Se algo der errado: rollback

```bash
ssh -i ~/.ssh/gh_actions_omnichannel_vps -o StrictHostKeyChecking=accept-new \
  deploy@85.31.62.33 \
  "cd /home/deploy/lista-atendimento && \
   docker compose --env-file .env.production -f docker-compose.prod.yml stop api web && \
   backup_file=$(ls -t backups/*.dump | head -1) && \
   docker compose --env-file .env.production -f docker-compose.prod.yml exec -T postgres \
   sh -lc 'dropdb -U \"\$POSTGRES_USER\" \"\$POSTGRES_DB\" && createdb -U \"\$POSTGRES_USER\" \"\$POSTGRES_DB\"' && \
   docker compose --env-file .env.production -f docker-compose.prod.yml exec -T postgres \
   pg_restore -U omni -d omni --no-owner \"\$backup_file\""
```

Nota: rollback so e' possivel enquanto o backup existir e as migrations 0136 nao
tiverem apagado dados que nao estao no backup.

### Itens recorrentes a checar antes de subir

- conferir migrations pendentes com `go run ./cmd/migrate status`
- rodar `npm run build` e `go build ./...` localmente antes do deploy
- registrar nesta secao qualquer release que precise de passo manual extra

## Validacao pos-deploy

Checks minimos:

```bash
docker compose --env-file .env.production -f docker-compose.prod.yml ps
docker compose --env-file .env.production -f docker-compose.prod.yml logs --tail=100 api
docker compose --env-file .env.production -f docker-compose.prod.yml logs --tail=100 web
curl -I https://lista.whenthelightsdie.com
curl -I https://lista.whenthelightsdie.com/healthz
```

Checks de headers de seguranca:

```bash
curl -I https://lista.whenthelightsdie.com | grep -Ei 'strict-transport-security|x-content-type-options|x-frame-options|referrer-policy|permissions-policy|content-security-policy'
curl -I https://lista.whenthelightsdie.com/healthz | grep -Ei 'strict-transport-security|x-content-type-options|x-frame-options|referrer-policy|permissions-policy|content-security-policy'
```

Check externo complementar:

- abrir `https://securityheaders.com/` e testar `https://lista.whenthelightsdie.com`;
- comparar o resultado com o bloco `header` aplicado no Caddy para confirmar que o proxy publico esta devolvendo a matriz esperada.

Checks funcionais:

1. abrir `https://lista.whenthelightsdie.com`
2. fazer login
3. validar carregamento do dashboard
4. validar uma operacao que use API autenticada
5. validar WebSocket em operacao, se essa tela fizer parte do go-live
6. validar no DevTools que `GET /v1/settings?tenantId={activeTenantId}` retorna `200` apos login, especialmente com usuario `platform_admin`
7. quando o release tocar settings tenant-wide, confirmar que o usuario global sem `tenantId` no token usa o `activeTenantId` retornado por `/v1/me/context`; sem `tenantId`, a API so deve cair no fallback quando existir exatamente um tenant acessivel

Checks administrativos para o primeiro bootstrap:

1. fazer login com o owner inicial criado no `bootstrap-owner`
2. abrir a area de usuarios
3. validar criacao manual dos primeiros usuarios reais
4. validar que nenhum usuario `@demo.local` existe no ambiente

## Backup minimo

- backup do volume do PostgreSQL desta stack
- backup do volume `api_uploads`
- backup do arquivo `.env.production`

## Dados que eu preciso quando voce quiser que eu execute o deploy

- host ou IP da VPS
- porta SSH
- usuario SSH
- forma de autenticacao: senha ou chave
- caminho do clone do outro projeto na VPS, se o proxy estiver la
- confirmacao se o proxy atual roda no host ou em container

## Proximo passo natural

Depois do primeiro deploy manual estabilizado, o melhor segundo passo e criar um deploy por Git para este repositorio reutilizando o mesmo acesso SSH da VPS. Mas isso vem depois de validar o bootstrap produtivo sem seed demo.
