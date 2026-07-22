# Operation Docker Bug Log

## Objetivo

Registrar os bugs encontrados na fase `frontend + API Go + PostgreSQL + Docker`, o impacto observado e a decisao de arquitetura usada para resolver cada um.

## Bugs mapeados

### 1. Loop infinito entre `/operacao` e `/auth/login`

Sintoma:

- ao abrir ou recarregar `/operacao`, a pagina entrava em ciclo de redirecionamento e nao estabilizava

Causa raiz:

- a rota protegida estava sendo prerenderizada como HTML estatico
- o HTML antigo fazia `refresh` para `/auth/login`
- com sessao ativa, o login redirecionava de volta para `/operacao`

Resolucao:

- remover prerender de rotas protegidas no Nuxt

Arquivos:

- `web/nuxt.config.ts`

### 2. `/operacao` travando em `Carregando operacao...`

Sintoma:

- depois do `F5`, a tela mostrava a mensagem de sincronizacao e nao saia mais dela

Causa raiz:

- o runtime do client estava sobrescrevendo o estado SSR/autenticado com o mock local ao hidratar

Resolucao:

- preservar o estado SSR ja carregado
- usar mock apenas como fallback quando nao existir estado valido

Arquivos:

- `web/app/stores/app-runtime.ts`

### 3. Loja ativa resetando no reload

Sintoma:

- ao trocar de loja e recarregar a pagina, a UI voltava para a loja padrao

Causa raiz:

- a loja ativa nao era persistida no contexto autenticado do frontend

Resolucao:

- persistir `activeStoreId` em cookie
- preferir esse valor ao reconstruir a sessao

Arquivos:

- `web/app/stores/auth.ts`

### 4. `invalid_json` ao finalizar atendimento

Sintoma:

- o `POST /v1/operations/finish` respondia `400 invalid_json`
- o atendimento nao finalizava
- o consultor nao voltava para a fila

Causa raiz:

- o frontend enviava `productsSeen` e `productsClosed` com campos extras do componente de UI
- o backend usa `DisallowUnknownFields`, o que rejeitou o payload

Resolucao:

- normalizar os produtos antes de enviar para a API
- manter apenas `id`, `name`, `code`, `price` e `isCustom`

Arquivos:

- `web/app/stores/operations.ts`
- `back/internal/platform/httpapi/json.go`

### 5. Resposta de `finish` devolvendo o snapshot inteiro da loja

Sintoma:

- ao finalizar um atendimento, a resposta do comando trazia toda a fila, atendimentos ativos, sessoes e historico da loja
- isso confundia o debug e aumentava o payload da mutacao

Causa raiz:

- os comandos de `operations` devolviam `MutationAck` com `snapshot` completo embutido

Resolucao:

- `GET /v1/operations/snapshot` continua sendo a leitura completa da loja
- comandos `POST /v1/operations/*` agora devolvem apenas `ack` minimo
- apos mutacao bem-sucedida, o frontend revalida somente o snapshot operacional

Arquivos:

- `back/internal/modules/operations/model.go`
- `back/internal/modules/operations/service.go`
- `web/app/stores/operations.ts`
- `back/internal/modules/operations/AGENT.md`
- `docs/operacao/operations.md`

### 6. `IPC connection closed` recorrente no dev do Nuxt

Sintoma:

- o `web` em modo dev derrubava o processo com `Error / An error has occurred / IPC connection closed`
- o erro aparecia de forma intermitente, principalmente no fluxo Docker-first com bind mount no Windows

Causa raiz:

- o Nuxt estava inicializando `@nuxt/devtools` no ambiente de desenvolvimento
- ao mesmo tempo, o watcher do bind mount reagia a arquivos gerados dentro de `.output` e `dist`
- essa combinacao aumentava o churn do builder e podia encerrar o processo filho do `vite-node`, fechando o canal IPC

Resolucao:

- desabilitar `devtools` por padrao e deixar reativacao apenas por `NUXT_DEVTOOLS=true`
- manter watcher por polling no fluxo oficial e no fallback local do Windows
- ignorar `.output` e `dist` no watcher do Vite para evitar rebuild sobre artefatos gerados
- desabilitar SSR nas rotas autenticadas do dashboard, que nao precisam de SEO e eram exatamente o caminho que disparava o worker `vite-node` no dev

Arquivos:

- `web/nuxt.config.ts`
- `docker-compose.yml`
- `scripts/dev/start-web-local.sh`

### 7. Dev do web inviavel no Docker Desktop/WSL2 (boot de minutos, pagina nunca abria)

Sintoma (2026-07-03):

- `nuxt dev` no container levava 5+ minutos SEM terminar o boot; `localhost:3003` recusava conexao
- troca de pagina levava ate ~10 minutos; paliativos (polling + warmup do grafo inteiro) PIORARAM

Causa raiz:

- o bind mount `./web:/app` de caminho Windows atravessa a ponte 9P/gRPC-FUSE do WSL2 — cada stat/read de arquivo fica ~100x mais lento, e o nuxt dev toca milhares de arquivos
- os proprios paliativos multiplicaram o I/O nessa ponte: polling do watcher (varre a arvore a cada 350ms) + warmup de TODAS as paginas no boot

Resolucao:

- bind mount REMOVIDO; o codigo e copiado no build da imagem (target dev) e vive no overlayfs nativo do container
- `docker compose watch` (develop.watch, action sync) sincroniza edicoes host -> container; inotify real dentro do container, polling desligado por padrao
- package.json/package-lock.json usam `sync+restart` (ensure-node-modules faz `npm ci` no volume)
- ATENCAO: o watch NAO faz sync inicial (verificado empiricamente no Compose v2.38) — por isso o fluxo oficial roda `docker compose up -d --build --force-recreate --no-deps web` antes de ligar o watch (`npm run dev:watch` / scripts/dev/watch-web.ps1). O `--force-recreate` e necessario porque o Compose pode manter um container com o bundle antigo mesmo apos build; `npm run dev` = `up --build --watch`
- resultado medido: boot completo do nuxt dev em ~60s, GET / em ~0.04s, sync de edicao chega em ~2s

Arquivos:

- `docker-compose.yml`
- `scripts/dev/watch-web.ps1`
- `package.json` (scripts dev/dev:watch)
- `AGENT.md`, `web/AGENT.md`, `scripts/dev/AGENT.md`

## Regra arquitetural consolidada

Para `operations`, seguimos agora esta separacao:

- leitura completa: `GET /v1/operations/snapshot`
- mutacao: `POST /v1/operations/*` com `ack` enxuto
- sincronizacao da UI: revalidar o snapshot depois da mutacao

Isso reduz acoplamento entre leitura e comando e prepara melhor a entrada futura de websocket.
