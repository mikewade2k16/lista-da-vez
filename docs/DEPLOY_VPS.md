# Deploy na VPS

Este e o unico playbook de deploy que vale para este repositorio.
Os arquivos em `docs_depoy/` vieram de outro projeto, foram mantidos apenas como apontadores curtos e nao sao fonte de verdade daqui para frente.

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

- `COMPOSE_PROJECT_NAME=listaatendimento` separa containers, rede e volumes
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

- `COMPOSE_PROJECT_NAME=listaatendimento`
- `POSTGRES_DB=listaatendimento`
- `POSTGRES_USER=listaatendimento`
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
`.env.production`. Por padrao, o sync manual fica desligado em producao.

### Carga ERP por dump de banco

Quando os consolidados ja foram importados no Postgres local, nao subir a pasta
`Controlle10 - ftp` para a VPS. Gere e transfira apenas um dump comprimido das
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
  -U lista_da_vez -d lista_da_vez \
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
docker cp lista-da-vez-postgres-1:/tmp/erp_data.dump ./tmp/erp_data.dump
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
  psql -U listaatendimento -d listaatendimento -v ON_ERROR_STOP=1 <<'SQL'
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
  pg_restore -U listaatendimento -d listaatendimento \
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
- exclui a pasta local `Controlle10 - ftp` do payload antes do envio
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

### Configuracoes operacionais agora sao tenant-wide

Migration `0024_tenant_operation_settings.sql` cria as tabelas
`tenant_operation_settings`, `tenant_setting_options` e
`tenant_catalog_products` e move o escopo de configuracao da operacao de
loja para tenant. As tabelas legadas `store_operation_settings`,
`store_setting_options` e `store_catalog_products` continuam no banco
durante a transicao para servirem de fonte de backfill.

Antes de subir este release:

1. **Backup completo do banco antes da migration 0024.**
   Use `npm run prod:deploy:vps -- -BackupDatabase` ou rode o backup manual
   do bloco `Se o release tocar schema, migrations ou dados`.
2. **Aplicar a migration 0024 no host.**
   O backfill embutido escolhe a config da loja mais antiga de cada tenant
   e faz uniao deduplicada das opcoes/produtos. Em ambiente local isso
   resolve, mas em producao a regra final de uniao precisa ser confirmada
   por humano.
3. **Conferir o backfill manualmente em producao antes de liberar acesso.**
   Para cada tenant, comparar `tenant_setting_options` e
   `tenant_catalog_products` com o que existia na loja-fonte e nas demais
   lojas. Se faltar item conhecido, completar manualmente via SQL ou
   reaplicar via UI.
4. **Avisar o owner que a UI de Configuracoes virou tenant-wide.**
   O seletor de loja do header deixou de afetar a area de Configuracoes.
   A propria tela mostra um banner reforcando isso, mas vale comunicar para
   evitar duvida no primeiro acesso.
5. **Nao dropar as tabelas `store_*` nesse deploy.**
   Elas devem ser mantidas como historico/backfill ate o release seguinte
   confirmar que o tenant-wide esta estavel.

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
