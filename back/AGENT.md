# AGENT

## Escopo

Estas instrucoes valem para todo o backend dentro de `back/`.

## Objetivo atual

Construir uma API Go solida, modular e pronta para evoluir de:

- auth simples no inicio
- tenant demo no bootstrap
- HTTP como fonte autoritativa
- realtime com WebSocket para invalidacao leve e sincronizacao de contexto/operacao

Sem precisar redesenhar a base quando o produto virar multi-tenant de verdade.

## Matriz de versoes

- Docker Compose oficial: `v2`
- Go do modulo: `1.24.0`
- Toolchain Go: `1.24.3`
- Nuxt do frontend integrado: `4.4.2`
- Pinia do frontend integrado: `3.0.4`
- Node do frontend containerizado: `24.11.1`
- Driver PostgreSQL no backend: `pgx/v5 5.7.6`
- PostgreSQL alvo do backend: `16`
- Imagem oficial do banco: `postgres:16-alpine`
- Imagem oficial do backend: `golang:1.24.0-bookworm`

## Workflow

O backend agora deve ser pensado com Docker como fluxo principal de execucao local.

Pela raiz:

```bash
npm run dev
```

Isso sobe `postgres + api + web`.

Se precisar atuar so no backend:

```bash
docker compose up --build postgres api
docker compose logs -f api
```

## Modelo de arquitetura

- `cmd/api`
  - bootstrap da aplicacao
- `internal/platform`
  - infraestrutura compartilhada do backend
- `internal/modules`
  - modulos de dominio e aplicacao

Regra:

- `platform` nao contem regra de negocio do produto
- cada dominio entra como modulo em `internal/modules/<modulo>`
- cada modulo deve conseguir evoluir para repositorio real, service, handlers HTTP e realtime sem vazar detalhes internos

## Modelo de tenant adotado

O backend ja nasce com esta regra:

- `owner` representa o cliente dono do tenant
- `consultant`, `manager`, `store_terminal`, `marketing` e `director` pertencem a um `tenant_id`
- `platform_admin` fica acima dos tenants

Hoje existe um `tenant-demo` para destravar a implementacao inicial, mas `tenant_id` ja e parte do principal autenticado e do token.

## Roles atuais

- `consultant`
- `store_terminal`
- `manager`
- `marketing`
- `director`
- `owner`
- `platform_admin`

### Diretriz de acesso operacional

- todo consultor agora e conta real do sistema; cadastro operacional e identidade nao devem seguir soltos
- `consultant`
  - conta individual de quem participa da operacao
  - deve ficar vinculada a uma unica loja
  - agora ja nasce automaticamente quando o consultor e criado no roster
  - o ciclo de vida dessa conta pertence ao modulo `consultants`, nao ao CRUD generico de `users`
- `store_terminal`
  - conta fixa do computador da loja
  - deve ficar vinculada a uma unica loja
  - pode operar fila, pausa, inicio e encerramento apenas na propria loja
- `manager`
  - conta individual com escopo de uma unica loja
  - nao entra automaticamente na lista da vez so por existir como usuario
- `marketing` e `director`
  - contas tenant-wide com leitura cross-store da operacao
  - podem alternar loja e abrir a visao integrada, mas nao executam comandos operacionais
- `owner` e `platform_admin`
  - podem alternar loja no select e auditar outras unidades
  - agora tambem podem abrir a operacao em modo integrado cross-store
- backlog futuro:
  - seguranca de terminal por loja/dispositivo
  - notificacao operacional estruturada e acao remota por tarefa/reuniao

## Modulos atuais

- `auth`
  - login, token, principal, roles e middleware
- `tenants`
  - leitura do escopo de tenant acessivel e ciclo administrativo basico de clientes/grupos
- `stores`
  - leitura e administracao basica das lojas acessiveis
- `access`
  - catalogo de permissoes, matriz padrao por papel e overrides por usuario
- `catalog`
  - busca operacional de catalogo de produtos com fontes trocaveis e contrato estavel
- `consultants`
  - roster administrativo por loja
- `settings`
  - leitura em bundle + gravacao por secao da configuracao operacional por loja
- `realtime`
  - WebSocket autenticado por loja para invalidacao e sincronizacao operacional em tempo real
  - WebSocket autenticado por tenant para sincronizacao administrativa de contexto, lojas e usuarios
  - o frontend agora tambem usa o realtime da operacao em modo multi-loja para revalidar a visao integrada
- `reports`
  - leituras analiticas e gerenciais server-side sobre o historico operacional
- `analytics`
  - agregados server-side para `ranking`, `dados` e `inteligencia`, evitando recalculo pesado no browser
- `users`
  - administracao de contas, papeis, escopo e convite/onboarding por tenant/loja

## Plugabilidade do core

O backend agora deve ser pensado em dois niveis:

- modulos de plataforma/host:
  - `auth`
  - `access`
  - `tenants`
  - `stores`
  - `users`
- modulos de core reutilizavel:
  - `operations`
  - `realtime`
  - `reports`
  - `analytics`
  - `settings` operacional quando fizer sentido

Regra:

- o core nao deve depender do projeto host inteiro
- o core deve depender de contratos minimos e adapters pequenos
- quando um modulo precisar de auth/loja, documentar:
  - contexto minimo de acesso
  - interface minima consumida
  - shape minimo de dados esperado

Referencia principal desta direcao:

- `CORE_MODULES_PORTABILITY.md`

## Infraestruturas atuais

- `database`
  - PostgreSQL via `pgx`
  - migrations SQL
  - runner em `cmd/migrate`
  - visualizacao humana do schema em `database/ERD.md`

Sempre que um novo modulo nascer, ele deve ganhar seu proprio `AGENT.md`.

## Regras de implementacao

### 1. Modulos

- Cada modulo deve ter fronteira clara.
- Preferir este shape quando fizer sentido:
  - `model.go`
  - `service.go`
  - `http.go`
  - `repository_*.go`
  - `middleware.go` se o modulo expuser middlewares
- Nao importar detalhes internos de outro modulo sem passar por interface pequena e explicita.
- quando um modulo do core depender de auth, tenant ou lojas, preferir:
  - contexto proprio do modulo
  - adapter fino na borda HTTP/app
  - interface pequena para leitura de escopo

### 2. Tenant e loja

- Toda regra operacional futura deve considerar `tenant_id` e, quando aplicavel, `store_id`.
- Nao confiar em `tenant_id` ou `store_id` vindo do client sem cruzar com o principal autenticado.
- `platform_admin` pode operar cross-tenant; os demais perfis nao.
- papeis de loja (`consultant`, `manager`, `store_terminal`) devem ficar com escopo de uma unica loja
- leitura operacional e mutacao operacional nao devem ser tratadas como a mesma permissao
  - `marketing` e `director` seguem como caso base dessa separacao: leem contexto mais amplo, mas nao executam comandos operacionais
  - `store_terminal` segue com mutacao restrita a propria loja, sem ganhar escopo administrativo ou cross-store

### 3. HTTP

- Handlers devem ficar finos.
- Validacao de payload deve acontecer no boundary HTTP e/ou no service.
- Respostas JSON devem ser consistentes.
- Middleware global deve ficar em `internal/platform/httpapi`.
- Qualquer wrapper de `http.ResponseWriter` precisa preservar interfaces do writer original quando existirem, especialmente:
  - `http.Hijacker` para websocket
  - `http.Flusher` para streaming
  - `http.Pusher` quando aplicavel
- Nunca quebrar upgrade de protocolo por causa de middleware transversal.

### 4. Persistencia

- O store em memoria e temporario.
- A fonte de verdade alvo e PostgreSQL.
- Quando repositorio real entrar, manter o contrato do modulo, trocando a implementacao por baixo.
- Nunca enviar colecoes completas quando a intencao for criar, editar ou remover um unico item.
- Preferir mutacoes granulares:
  - adicionar um item -> enviar apenas esse item
  - editar um item -> enviar apenas esse item ou apenas os campos alterados
  - remover um item -> enviar apenas o identificador e o escopo necessario
- Em comandos e patches, omitir campos nao aplicaveis, vazios ou default quando a ausencia carregar a mesma semantica no backend.
- No contrato Go/JSON desta base, ausencia de campo deve ser preferida a mandar `null`, string vazia, array vazio ou mapa vazio sem necessidade.
- Respostas de mutacao devem ser enxutas; snapshot/bundle completo e responsabilidade de endpoints de leitura.
- arquivos enviados pelo usuario devem seguir a mesma regra:
  - o banco persiste metadado/caminho
  - storage binario fica fora do PostgreSQL quando o caso de uso nao exigir blob transacional
- Endpoints bulk devem existir apenas para importacao, bootstrap controlado ou acoes que de fato alteram a secao inteira.
- Toda decisao de payload deve considerar latencia, custo de infra, custo de banco e previsibilidade de manutencao.
- Endpoints de leitura devem ser desenhados por caso de uso:
  - agregados pequenos para dashboards
  - listas paginadas para tabelas
  - leituras administrativas focadas, como ultimos atendimentos
- Para `ranking`, `dados` e `inteligencia`, a regra atual e:
  - frontend nao recalcula historico bruto quando existir endpoint analitico dedicado
  - o backend deve devolver payload pronto para renderizacao e tomada de decisao
  - historico bruto fica restrito a casos operacionais, auditoria ou leitura detalhada
- No onboarding de usuarios:
  - criar usuario sem senha deve gerar convite, nao senha fake
  - criar usuario com senha definida pelo admin nao deve gerar convite
  - senha definida pelo admin para conta individual deve marcar `must_change_password`
  - `password_hash` pode ser nulo enquanto o convite estiver pendente
  - token de convite nunca deve ser salvo em texto aberto no banco
  - aceite do convite deve definir a primeira senha e liberar login real
  - troca de senha em autoatendimento deve limpar `must_change_password`
  - rotas de convite devem devolver apenas o necessario para UX e operacao do admin
  - reset dedicado de senha deve existir como mutacao propria, sem ficar escondido em edicao generica
- No caso de areas administrativas como multiloja e usuarios, preferir:
  - leitura consolidada por tabela/lista
  - mutacoes pequenas por entidade
  - refresh do contexto apenas quando necessario para reidratar o front
- o frontend agora deve manter `usuarios` em workspace propria e `perfil` como pagina separada de autoatendimento
- separar claramente leitura operacional e leitura administrativa expandida
  - exemplo atual: o contexto autenticado usa apenas lojas ativas
  - a tela `multiloja` usa `GET /v1/stores?includeInactive=true` para o ciclo administrativo
- para operacao multi-loja:
  - `GET /v1/operations/snapshot` continua sendo leitura da loja ativa
  - `GET /v1/operations/overview` passa a ser a leitura integrada para `owner` e `platform_admin`
  - cards integrados devem sempre carregar `storeId` e `storeName` para auditoria visual clara
- remocoes administrativas nunca devem depender apenas de `cascade`; o backend deve validar dependencias de negocio antes de excluir

### 5. Frontend Nuxt

- O Nuxt deve manter HTTP como leitura autoritativa.
- WebSocket ja existe para invalidacao leve da operacao e do contexto administrativo.
- Realtime usa os mesmos conceitos de auth, tenant e store definidos no HTTP.
- O frontend deve sempre partir de identidade, escopo e snapshot inicial consistentes antes de assinar canais realtime.

### 6. Resiliencia

- A plataforma deve continuar operando mesmo com falha temporaria de rede, API ou infraestrutura.
- Registrar desde ja como direcao:
  - cache local seguro para operacao offline limitada
  - fila de sincronizacao para subir mutacoes pendentes depois
  - notificacao de erro para observabilidade
  - containers e servicos com estrategia de recuperacao automatica
- Nesta fase, priorizar contratos e modulos que nao dificultem essa evolucao futura.

## Ordem recomendada

1. auth
2. tenants/stores
3. consultores e configuracoes base
4. fila e atendimento via HTTP
5. websocket e reconciliacao realtime
6. relatorios, analytics, multiloja e usuarios/acessos
7. campanhas server-side

## Validacao minima

Ao alterar codigo em `back/`:

- rodar `gofmt`
- rodar `go test ./...`
- quando mexer em handlers criticos, fazer smoke HTTP real
- quando mexer em schema, atualizar tambem `database/ERD.md` e `database/AGENT.md`

## Documentos de apoio

- `../AGENT.md`
- `README.md`
- `START_LOCAL.md`
- `PLAN.md`
- `CORE_MODULES_PORTABILITY.md`
- `database/AGENT.md`
- `database/ERD.md`
- `../docs/BACKLOG.md`
- `../docs/NUXT_MIGRATION_BLUEPRINT.md`
- `../docs/NUXT_FULL_REFERENCE.md`
- `../web/app/pages/operacao/operations.md`
