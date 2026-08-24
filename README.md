# Omni

Repositorio principal do Omni, com frontend em Nuxt 4 dentro de `web/` e backend em Go dentro de `back/`.

## Estrutura

- `web/`
  - app Nuxt 4, stores Pinia, paginas e componentes
- `back/`
  - API Go, auth, contexto de tenant/loja, settings, consultores e operacao
- `docs/`
  - backlog, arquitetura e referencias funcionais
- `scripts/dev/`
  - entrada padrao de desenvolvimento local em Git Bash

## Stack atual

- Nuxt `4.4.2`
- Vue `3.5.30`
- Pinia `3.0.4`
- Node `24.11.1`
- Go `1.26.5`
- PostgreSQL `16`

## Fluxo oficial

O projeto agora trabalha em Docker por padrao.

Comando principal pela raiz:

```bash
npm run dev
```

Esse fluxo:

1. sobe o PostgreSQL em `http://localhost:5432`
2. sobe a API Go em `http://localhost:8080`
3. sobe o Nuxt em `http://localhost:3003`

Ao subir o container `api`, a imagem executa `migrate up` antes de iniciar o servidor.
Se o volume do banco vier de uma stack mais antiga e voce suspeitar de drift de schema,
recrie a API ou rode `docker compose exec api migrate up` para reaplicar o estado esperado
antes de depurar erros em `/v1/settings` ou `/v1/operations`.

No Compose, o `web` roda em modo dev com hot reload.
Mudancas de UI em `web/` devem atualizar sem rebuild do container.
Dependencias do frontend em `web/package.json` e `web/package-lock.json` sao sincronizadas automaticamente quando o container `web` sobe.

Arquivo opcional para customizar portas e credenciais do Compose:

```bash
cp .env.docker.example .env
```

### Variaveis e segredos de integracoes

- `.env.docker.example` e `.env.production.example` sao somente modelos versionados; nunca recebem
  tokens ou chaves reais.
- Docker local le os valores reais do `.env` ignorado pelo Git na raiz.
- Producao le o `.env.production` protegido que ja existe na VPS; os scripts de deploy preservam
  esse arquivo e variaveis novas nao viajam dentro da imagem.
- `back/.env` e usado apenas no fallback em que a API Go roda diretamente fora do Docker.

No Cloudflare R2, `R2_ACCOUNT_ID`, `R2_BUCKET`, `R2_ACCESS_KEY_ID` e
`R2_SECRET_ACCESS_KEY` ficam nesses envs privados. Os limites de consumo nao sao segredos nem env:
sao editados no painel e persistidos em `storage.settings`.

## Scripts principais

- `npm run dev`
  - sobe stack Docker completa
- `npm run dev:detach`
  - sobe stack Docker em background
- `npm run dev:build`
  - rebuilda as imagens quando houver mudanca de Dockerfile, imagem base ou dependencia do sistema
- `npm run dev:logs`
  - acompanha logs dos containers
- `npm run dev:ps`
  - lista os servicos e portas
- `npm run dev:down`
  - para a stack Docker preservando containers e volumes
- `npm run dev:remove-containers -- -Confirmation REMOVE_OMNI_CONTAINERS`
  - remove deliberadamente os containers locais, preservando os volumes
- `npm run dev:down:volumes`
  - bloqueado por padrao; para apagar containers e dados locais, exige
    `npm run dev:down:volumes -- -Confirmation DELETE_OMNI_LOCAL_DATA`
- `npm run build`
  - build do frontend

## Fallback local

O fluxo sem Docker continua disponivel so como contingencia:

- `npm run dev:local`
- `npm run dev:local:db`
- `npm run dev:local:api`
- `npm run dev:local:api:status`
- `npm run dev:local:api:stop`

## Login demo

- `proprietario@demo.local`
- `consultor@demo.local`
- senha: `dev123456`

Para o root local da plataforma no seed MVP use:

- `mikewade2k16@gmail.com`
- senha de teste local: `123123456`

Credenciais existentes nunca devem ser alteradas para viabilizar testes. Se a senha local nao estiver
disponivel ou nao funcionar, interrompa o teste e pergunte ao responsavel antes de qualquer reset.

## Quando rebuildar

- mudancas em `web/Dockerfile`
- mudancas no backend Go se a imagem da API precisar ser refeita
- alteracoes de imagem base, dependencia do sistema ou configuracao de build

## Onboarding de usuarios

O onboarding inicial agora funciona por convite:

- admin cria o usuario em `multiloja`
- a API devolve um link `http://localhost:3003/auth/convite/:token`
- o usuario define a primeira senha nesse link e entra com sessao real

## Referencias

- guia de backend local: `back/START_LOCAL.md`
- regras do repositorio: `AGENT.md`
- backend: `back/README.md`
- arquitetura do frontend: `docs/NUXT_4_STORE_ARCHITECTURE.md`
- backlog: `docs/BACKLOG.md`
- roadmap multi-tenant: `docs/ROADMAP.md`
- estado atual do projeto: `docs/ESTADO_ATUAL.md` (visual: `docs/estado-atual.html`)
- plano de refatoracao em andamento: `docs/PLANO_REFATORACAO.md` (visual: `docs/plano-refatoracao.html`)
- paralelizacao de agentes (Claude/Codex/Copilot): `docs/PARALELIZACAO.md` + `docs/agents/`
- planos e atas arquivados: `docs/historico/`
