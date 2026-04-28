# Deploy Producao Checklist

Este arquivo agora e o guia curto de operacao pelo terminal para este repositorio.
A fonte de verdade mais completa continua sendo:

- `docs/DEPLOY_VPS.md`

## 1. O que precisa existir uma vez

Antes do primeiro deploy automatizado, confirme estes pontos:

1. a VPS responde por SSH com o usuario `deploy`
2. a chave local existe em `C:/Users/Mike/.ssh/gh_actions_omnichannel_vps`
3. o diretorio remoto existe em `/home/deploy/lista-atendimento`
4. o arquivo remoto `.env.production` ja existe na VPS
5. o dominio `https://lista.whenthelightsdie.com` ja aponta para a VPS
6. o Caddy da outra stack ja tem o bloco de `lista.whenthelightsdie.com`

Por que isso importa:

- sem a chave SSH o script nao autentica
- sem `.env.production` o compose remoto nao sobe
- sem DNS e proxy o deploy pode concluir mas o site nao abre publicamente

## 2. Deploy rapido direto do terminal local

Este e o caminho mais rapido para subir da sua maquina para a VPS.

### 2.1. Entrar na pasta do projeto

```bash
cd ~/Documents/Projects/fila-atendimento
```

O que faz:

- garante que os comandos rodem na raiz do repositorio

Por que fazer:

- o script usa arquivos da raiz, como `package.json`, `docker-compose.prod.yml` e `scripts/deploy/deploy-vps-fast.ps1`

### 2.2. Conferir branch e alteracoes antes do deploy

```bash
git status -sb
```

O que faz:

- mostra a branch atual e se ha alteracoes locais

Por que fazer:

- evita subir algo local por acidente quando voce queria outra branch ou estado
- a pasta local `Controlle10 - ftp` fica fora de git e fora do payload oficial de deploy

### 2.3. Rodar o deploy normal

```bash
npm run prod:deploy:vps
```

O que faz:

1. empacota o workspace local por `tar`
2. exclui a pasta local `Controlle10 - ftp` do pacote antes do envio
3. conecta na VPS por SSH
4. limpa o diretorio remoto preservando `.env.production` e `backups`
5. envia o codigo atualizado
6. roda `docker compose config`
7. sobe `api` e `web` com `up -d --build`
8. executa smoke tests em `https://lista.whenthelightsdie.com` e `/healthz`

Por que fazer:

- este e o caminho oficial e mais rapido para redeploy normal

### 2.4. Rodar o deploy com backup antes

```bash
npm run prod:deploy:vps:backup
```

O que faz:

- faz tudo do deploy normal e antes gera um dump gzip do PostgreSQL remoto em `/home/deploy/lista-atendimento/backups/`

Por que fazer:

- use quando o release tocar migrations, importacao de dados ou qualquer mudanca sensivel no banco

### 2.5. Subir so um servico especifico

```bash
npm run prod:deploy:vps -- -Services api
npm run prod:deploy:vps -- -Services web
```

O que faz:

- sobe apenas os servicos informados

Por que fazer:

- reduz tempo e escopo quando a mudanca esta isolada

### 2.6. Forcar recreate dos containers

```bash
npm run prod:deploy:vps -- -ForceRecreate
```

O que faz:

- adiciona `--force-recreate` no `docker compose up`

Por que fazer:

- use quando quiser reinicio completo dos containers selecionados

### 2.7. Combinar backup com recreate

```bash
npm run prod:deploy:vps -- -BackupDatabase -ForceRecreate
```

O que faz:

- gera backup remoto e sobe os servicos com recreate forcado

Por que fazer:

- e o comando mais seguro para release com mais risco operacional

## 3. Validar depois do deploy

### 3.1. Testar a home publica

```bash
curl -I https://lista.whenthelightsdie.com
```

O que faz:

- verifica se o frontend publico respondeu

Por que fazer:

- confirma que o proxy e o container `web` estao servindo a aplicacao

### 3.2. Testar o healthcheck da API

```bash
curl -I https://lista.whenthelightsdie.com/healthz
```

O que faz:

- verifica o endpoint tecnico da API

Por que fazer:

- confirma que o container `api` esta saudavel atras do proxy

### 3.3. Se precisar, olhar a execucao do workflow no GitHub CLI

```bash
gh run list --repo mikewade2k16/lista-da-vez --workflow deploy-vps.yml --limit 5
gh run view <RUN_ID> --repo mikewade2k16/lista-da-vez --log
gh run watch <RUN_ID> --repo mikewade2k16/lista-da-vez
```

O que faz:

- lista as runs, mostra logs e acompanha execucao ao vivo

Por que fazer:

- ajuda a confirmar o que foi executado quando voce usar o caminho via GitHub Actions

## 4. Deploy automatizado via GitHub pelo terminal

Use este caminho quando quiser disparar o workflow sem abrir o navegador.

### 4.1. Fazer login no GitHub CLI

```bash
gh auth status
```

O que faz:

- confirma se o `gh` esta autenticado

Por que fazer:

- sem autenticacao o terminal nao consegue criar secrets nem disparar workflows

### 4.2. Cadastrar o secret da chave SSH no repositorio

```bash
gh secret set DEPLOY_VPS_SSH_KEY --repo mikewade2k16/lista-da-vez < "$HOME/.ssh/gh_actions_omnichannel_vps"
gh secret list --repo mikewade2k16/lista-da-vez
```

O que faz:

- grava a chave privada como secret do repositorio e depois lista os secrets cadastrados

Por que fazer:

- o workflow usa esse secret para autenticar na VPS

### 4.3. Garantir que o workflow esta no GitHub

```bash
git push -u origin migracao/nuxt
gh pr create --repo mikewade2k16/lista-da-vez --base main --head migracao/nuxt --fill
```

O que faz:

- sobe sua branch e abre a PR para levar o workflow e o restante das mudancas ate a `main`

Por que fazer:

- o GitHub so mostra e executa o workflow se o arquivo estiver publicado na branch remota correspondente

### 4.4. Disparar o workflow manual pelo terminal

```bash
gh workflow run deploy-vps.yml \
	--repo mikewade2k16/lista-da-vez \
	--ref main \
	-f git_ref=main \
	-f services="api web" \
	-f backup_database=false \
	-f force_recreate=false \
	-f skip_smoke_tests=false
```

O que faz:

- manda o GitHub executar o workflow `Deploy VPS` usando o codigo da `main`

Por que fazer:

- esse e o jeito de fazer deploy automatizado sem entrar na aba `Actions`

### 4.5. Acompanhar a run do workflow

```bash
gh run list --repo mikewade2k16/lista-da-vez --workflow deploy-vps.yml --limit 1
gh run watch <RUN_ID> --repo mikewade2k16/lista-da-vez
```

O que faz:

- encontra a ultima run e acompanha a execucao no terminal

Por que fazer:

- voce ve se as etapas `Sync workspace to VPS`, `Deploy selected services` e `Smoke tests` passaram

## 5. Ordem recomendada no dia a dia

Para deploy normal e mais rapido:

1. `git status -sb`
2. `npm run prod:deploy:vps`
3. `curl -I https://lista.whenthelightsdie.com`
4. `curl -I https://lista.whenthelightsdie.com/healthz`

Para deploy com risco de banco:

1. `git status -sb`
2. `npm run prod:deploy:vps:backup`
3. `curl -I https://lista.whenthelightsdie.com`
4. `curl -I https://lista.whenthelightsdie.com/healthz`

Para deploy automatizado pelo GitHub CLI:

1. `gh workflow run deploy-vps.yml --repo mikewade2k16/lista-da-vez --ref main -f git_ref=main -f services="api web" -f backup_database=false -f force_recreate=false -f skip_smoke_tests=false`
2. `gh run list --repo mikewade2k16/lista-da-vez --workflow deploy-vps.yml --limit 1`
3. `gh run watch <RUN_ID> --repo mikewade2k16/lista-da-vez`
