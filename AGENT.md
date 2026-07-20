# AGENT

## Escopo

Estas instrucoes valem para o repositorio inteiro.

## Workflow oficial

O fluxo padrao do projeto agora e Docker-first.

Suba a stack completa pela raiz com:

```bash
npm run dev        # docker compose up --build --watch
```

O `web` roda em modo dev dentro do container SEM bind mount (2026-07-03): o
codigo e copiado no build e o `docker compose watch` sincroniza as edicoes do
host pra dentro (sync). Motivo: bind mount de caminho Windows atravessa a
ponte 9P do WSL2 (~100x mais lento por arquivo) — boot e troca de pagina
levavam minutos. Hot reload continua funcionando (inotify real, sem polling).

REGRA DE OURO: desenvolva sempre com o watch LIGADO (`npm run dev`, ou
`npm run dev:watch` se a stack ja estiver de pe). O watch NAO faz sync
inicial: edicao feita com watch desligado so entra com rebuild
(`docker compose up -d --build web` — e exatamente o que o dev:watch faz
antes de ligar o watch).

NOTA: os scripts `npm run dev*` da raiz sao SO ATALHOS para `docker compose`
— nenhum Node/npm do app roda no host (o npm que serve o painel e o de dentro
do container). Sem npm, os equivalentes diretos sao:

```bash
docker compose up --build --watch                                  # = npm run dev
docker compose up -d --build web && docker compose watch --no-up web  # = npm run dev:watch
```

Dar "play" no container web pelo Docker Desktop sobe o painel para USO, mas
NAO liga o watch — edicoes de codigo nao entram ate rodar um dos comandos
acima num terminal (o watch nao existe na UI do Docker Desktop).

Isso sobe:

- `postgres` em `localhost:5432`
- `api` em `localhost:8080`
- `web` em `localhost:3003`

Comandos principais:

- `npm run dev` (stack completa + watch/sync do web)
- `npm run dev:watch` (so reconcilia o web + liga o watch; stack ja de pe)
- `npm run dev:detach` (sobe SEM watch — edicoes no web nao chegam!)
- `npm run dev:build`
- `npm run dev:logs`
- `npm run dev:ps`
- `npm run dev:down`
- `npm run dev:down:volumes`

Quando rebuild ainda e necessario:

- mudanca em `web/package.json` ou `web/Dockerfile`
- mudanca de codigo no backend Go que precise reempacotar a imagem
- alteracao de imagem base, dependencias do sistema ou configuracao de build

O fluxo local sem Docker continua existindo apenas como fallback:

- `npm run dev:local`
- `npm run dev:local:db`
- `npm run dev:local:api`

## Matriz de versoes

- Docker Compose: `v2`
- PostgreSQL: `16`
- imagem PostgreSQL: `postgres:16-alpine`
- Go do backend: `1.24.0`
- toolchain Go: `1.24.3`
- imagem base do backend: `golang:1.24.0-bookworm`
- Nuxt: `4.4.2`
- Vue: `3.5.30`
- Pinia: `3.0.4`
- Node.js do frontend containerizado: `24.11.1`
- imagem base do frontend: `node:24.11.1-bookworm-slim`

## Organizacao

- `web/`
  - frontend Nuxt 4
- `back/`
  - API Go modular
- `docs/`
  - backlog, arquitetura e referencias
- `scripts/dev/`
  - fallback local para Windows/Git Bash

## Regras gerais

- Skills pessoais Codex deste projeto:
  - `$principios-engenharia`: usar antes de qualquer implementação, refactor, migration,
    deploy ou revisão de código;
  - `$omnichannel-hibrido`: usar sempre que a tarefa tocar atendimento, CRM multicanal,
    canais, IA, n8n, filas ou handoff;
  - `$revisao-dia`: usar para balanço consultivo; é somente leitura.
  As fontes Claude originais continuam em `.claude/skills`, mas as versões nativas e
  implicitamente invocáveis ficam em `~/.codex/skills`.
- n8n tem ownership por módulo: antes de editar/importar/exportar/ativar/desativar/remover,
  identificar o owner pelo mapa em `automation/AGENT.md`. A tarefa só pode tocar workflows
  do módulo explicitamente em escopo; scripts compartilhados preservam os demais ids e estados.
- Todo novo trabalho de produto deve considerar `web + back + banco` como stack integrada.
- `web` fala com a API por `NUXT_PUBLIC_API_BASE` no browser e `NUXT_API_INTERNAL_BASE` no SSR/container.
- `back` deve continuar modular, com um `AGENT.md` proprio por modulo em `internal/modules/<modulo>`.
- Mudancas de schema exigem migration SQL e atualizacao de `back/database/ERD.md`.
- Evitar reintroduzir fonte de verdade em `localStorage`.
- Onboarding de usuario agora segue convite real:
  - usuario pode nascer sem senha
  - API devolve link `/auth/convite/:token`
  - primeira senha e criada no aceite do convite
- O modelo de acesso operacional agora precisa seguir esta direcao:
  - todo consultor ja nasce como conta real do sistema vinculada ao roster operacional
  - existe conta `store_terminal` para o computador fixo da loja
  - `store_terminal` visualiza apenas a operacao da propria unidade
  - seguranca por loja/dispositivo entra como proxima camada de hardening

## Documentos principais

- `README.md`
- `back/README.md`
- `back/PLAN.md`
- `back/CORE_MODULES_PORTABILITY.md`
- `back/START_LOCAL.md`
- `web/AGENT.md`
- `back/AGENT.md`
- `docs/NUXT_4_STORE_ARCHITECTURE.md`

## Pre-commit hook (Fase 6.3 do PLANO_REFATORACAO)

Configurado a partir de 2026-05-18. Toda vez que voce roda `git commit`, o Husky dispara `npx lint-staged` que despacha pelos wrappers:

- `scripts/dev/lint-web-staged.sh` -> `eslint --fix` nos `.vue`/`.ts`/`.js`/`.mjs` staged
- `scripts/dev/format-web-staged.sh` -> `prettier --write` nos arquivos web staged
- `scripts/dev/lint-go-staged.sh` -> `gofmt -w` + `golangci-lint run --new-from-rev=HEAD` em **escopo de pacote** (nao arquivo isolado)

### Decisao tecnica importante

O wrapper Go nao roda golangci-lint em arquivos isolados — extrai os **pacotes** dos arquivos staged e roda no escopo `./pkg/...`. Sem isso, linters como `unused`, `errcheck` e `staticcheck` (que precisam do pacote inteiro) gerariam falsos positivos/negativos. A flag `--new-from-rev=HEAD` evita bloquear commit em pacote que ja tinha divida na baseline registrada.

### Bypass de emergencia

Quando MESMO precisar pular o hook (ex.: hotfix urgente):

```bash
git commit --no-verify -m "..."
```

Use com parcimonia — o ponto do hook e evitar regressao silenciosa.

## Validacao minima

- frontend: `npm --prefix web run build` + `npm --prefix web run lint`
- backend: `go test ./...` + `golangci-lint run ./...` em `back/`
- compose: `docker compose config`
