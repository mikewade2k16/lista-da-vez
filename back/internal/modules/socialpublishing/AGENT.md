# Modulo `socialpublishing`

## Escopo

Agendamento, publicacao e analytics de posts em contas profissionais do
Instagram. O schema autoritativo e `social_publishing`; o modulo nao consulta
nem altera tabelas de Calendar, Omnichannel ou Meta Ads.

## Contratos estaveis

- Module ID: `social_publishing`.
- API autenticada: `/v1/social-publishing`.
- Nao existe rota de runtime/Crow nesta fase. `Service.RuntimeContext` e somente
  um seam interno reservado para a integracao futura explicitamente autorizada.
- `account_id` vem exclusivamente de `auth.Principal.AccountID`, preenchido por
  `RequireAuthWithAccount`.
- Permissoes: `social_publishing.view`, `social_publishing.manage`,
  `social_publishing.connect` e `social_publishing.analytics`.
- `GET /v1/social-publishing/scope` usa `.view` e devolve somente contas-cliente
  ativas, com o modulo habilitado e visiveis ao Principal. `platform_admin` ve
  todas; usuario numa conta-agencia ve apenas clientes da mesma organizacao em
  que seja `agency_owner` ou membro ativo; conta-cliente fica travada nela mesma.
  A lista ainda repete o RBAC `.view` em cada conta-alvo.
- `GET /v1/social-publishing/portfolio` usa `.analytics`, exige um scope
  selecionavel (agencia/platform) e deriva os IDs exclusivamente do scope
  resolvido no servidor. O agregado repete o RBAC `.analytics` por cliente e
  nunca aceita `accountIds` do frontend.
- PostgreSQL e a fonte de verdade. O fluxo e `handler -> service -> repository`.
- Toda consulta e mutacao do repository repete o filtro por `account_id`.
- Recurso inexistente ou de outra conta retorna 404.

## Instagram e segredos

- Somente contas profissionais (`BUSINESS`, `CREATOR` ou `MEDIA_CREATOR`) sao
  aceitas.
- O access token e validado em `/me`, cifrado com `platform/secretbox` e nunca
  retorna na API nem aparece em logs/erros.
- Reconectar revoga a conexao anterior e insere uma nova linha. Posts preservam
  o connection ID historico; uma credencial nova so pode operar o mesmo
  `ig_user_id`, nunca redirecionar um post para outra conta.
- `SOCIAL_PUBLISHING_GRAPH_BASE` configura a Graph; o default e
  `https://graph.instagram.com/v24.0`.
- Publicacao de imagem usa `/{ig_user_id}/media` e
  `/{ig_user_id}/media_publish`.
- URL da imagem deve ser HTTPS. O modulo nao baixa nem faz proxy do arquivo.

## Estados e idempotencia

- Post: `draft`, `scheduled`, `publishing`, `published`, `failed`, `cancelled`.
- `source_type + source_ref` e unico por conta. O POST publico exige
  `idempotencyKey` e sempre grava `source_type=manual`; `calendar` e
  `crow_assistant` ficam reservados para adapters futuros.
- Agendar incrementa `schedule_revision`; o job carrega `{postId, revision}`.
- Job com revisao antiga e no-op.
- A criacao/agendamento e a insercao em `publish_outbox` acontecem na mesma
  transacao.
- `publish_attempted_at` e persistido antes de `media_publish`. Uma execucao
  retomada depois desse marco nunca chama o provider automaticamente: vira
  `publish_outcome_unknown` e fica bloqueada para PATCH/schedule/retry ate uma
  reconciliacao futura, privilegiando at-most-once.
- Ao marcar `published`, os refreshes de analytics de 5m, 1h, 6h e 24h entram
  em `analytics_outbox` na mesma transacao.
- Cada snapshot grava `job_key=analytics_outbox.id` e possui unique
  `(account_id, post_id, job_key)`; retry depois de commit nunca duplica o
  snapshot.
- A projecao corrente de analytics so aceita uma captura com
  `captured_at >=` ao valor persistido; refresh atrasado nao regride metricas.
- Kinds: `social.publish` e `social.analytics.refresh`.
- Publicacao usa `ordering_key=publish:<postId>:v<revision>`, igual a chave de
  idempotencia. Nao existe FIFO global entre posts: cada revisao executa no seu
  `run_after` e uma agenda distante nunca bloqueia outra mais proxima.
  Analytics usa chave por post+stage (`5m|1h|6h|24h|manual`) pelo mesmo motivo.

## Tabelas esperadas

- `social_publishing.connections`
- `social_publishing.posts`
- `social_publishing.post_analytics`
- `social_publishing.analytics_snapshots`
- `social_publishing.publish_outbox` (somente `social.publish`)
- `social_publishing.analytics_outbox` (somente `social.analytics.refresh`)

## Operacao

- Dois workers pertencem ao Handle, iniciam no Build e param em `Close`.
  Publicacao e analytics usam Stores/tabelas distintas; uma rajada de insights
  nunca consome os claims nem o loop do worker de publicacao.
- Erros persistidos sao codigos/mensagens sanitizados; payload, token e resposta
  bruta da Graph nunca sao persistidos em `last_error`.
- Analytics detalhadas saem somente em `GET /analytics/posts`, sob
  `social_publishing.analytics`. As rotas de posts sob `.view` nao incluem
  metricas, e o overview publico retorna somente o agregado de analytics.
- O portfolio executa uma unica consulta agregada parametrizada para o conjunto
  autorizado. A resposta traz totais e linhas enxutas por cliente; token,
  ciphertext, connection ID e qualquer outro segredo ficam fora da projecao.
- `GET /posts` aceita o filtro legado `status` e a lista
  `statuses=scheduled,publishing,failed`; os dois sao unidos por OR,
  deduplicados e validados antes do SQL tenant-scoped com `ANY`. `order` e um
  enum fechado: o default `created` ordena por criacao decrescente e
  `order=scheduled` ordena globalmente por `scheduled_for asc nulls last`, com
  desempate por ID antes de aplicar `limit/offset`.

## Validacao

No diretorio `back`:

```bash
gofmt -w internal/modules/socialpublishing
go test ./internal/modules/socialpublishing
```
