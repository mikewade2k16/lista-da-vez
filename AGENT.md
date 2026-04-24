# AGENT

## Escopo

Estas instrucoes valem para o repositorio inteiro.

## Workflow oficial

O fluxo padrao do projeto agora e Docker-first.

Suba a stack completa pela raiz com:

```bash
npm run dev
```

O `web` roda em modo dev dentro do container, com bind mount e hot reload.
Mudancas de layout, componentes e CSS devem refletir sem rebuild completo.

Isso sobe:

- `postgres` em `localhost:5432`
- `api` em `localhost:8080`
- `web` em `localhost:3003`

Comandos principais:

- `npm run dev`
- `npm run dev:detach`
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
- `web/AGENTS.md`
- `back/AGENT.md`
- `docs/NUXT_4_STORE_ARCHITECTURE.md`

## Validacao minima

- frontend: `npm --prefix web run build`
- backend: `go test ./...` em `back/`
- compose: `docker compose config`
