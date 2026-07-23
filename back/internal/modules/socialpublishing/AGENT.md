# Modulo `socialpublishing`

## Escopo

Agendamento, publicacao e analytics de posts em contas profissionais do
Instagram. O schema autoritativo e `social_publishing`; o modulo nao consulta
nem altera tabelas de Calendar, Omnichannel ou Meta Ads.

## Contratos estaveis

- Module ID: `social_publishing`.
- API autenticada: `/v1/social-publishing`.
- Runtime somente leitura: `/v1/runtime/social-publishing/context`.
- `account_id` vem exclusivamente de `auth.Principal.AccountID`, preenchido por
  `RequireAuthWithAccount`.
- Permissoes: `social_publishing.view`, `social_publishing.manage`,
  `social_publishing.connect` e `social_publishing.analytics`.
- PostgreSQL e a fonte de verdade. O fluxo e `handler -> service -> repository`.
- Toda consulta e mutacao do repository repete o filtro por `account_id`.
- Recurso inexistente ou de outra conta retorna 404.

## Instagram e segredos

- Somente contas profissionais (`BUSINESS`, `CREATOR` ou `MEDIA_CREATOR`) sao
  aceitas.
- O access token e validado em `/me`, cifrado com `platform/secretbox` e nunca
  retorna na API nem aparece em logs/erros.
- `SOCIAL_PUBLISHING_GRAPH_BASE` configura a Graph; o default e
  `https://graph.instagram.com/v23.0`.
- Publicacao de imagem usa `/{ig_user_id}/media` e
  `/{ig_user_id}/media_publish`.
- URL da imagem deve ser HTTPS. O modulo nao baixa nem faz proxy do arquivo.

## Estados e idempotencia

- Post: `draft`, `scheduled`, `publishing`, `published`, `failed`, `cancelled`.
- `source_type + source_ref`, quando `source_ref` existe, e unico por conta.
- Agendar incrementa `schedule_revision`; o job carrega `{postId, revision}`.
- Job com revisao antiga e no-op.
- A criacao/agendamento e a insercao na outbox acontecem na mesma transacao.
- Kinds: `social.publish` e `social.analytics.refresh`.
- FIFO de publicacao por conexao; analytics usa ordering key por post.

## Tabelas esperadas

- `social_publishing.connections`
- `social_publishing.posts`
- `social_publishing.post_analytics`
- `social_publishing.analytics_snapshots`
- `social_publishing.outbox` (contrato exato de `platform/jobs`)

## Operacao

- O worker pertence ao Handle do modulo, inicia no Build e para em `Close`.
- Erros persistidos sao codigos/mensagens sanitizados; payload, token e resposta
  bruta da Graph nunca sao persistidos em `last_error`.
- O runtime usa `AUTOMATION_RUNTIME_TOKEN`, comparado em tempo constante. Token
  ausente deixa a rota indisponivel; nunca abre acesso anonimo.

## Validacao

No diretorio `back`:

```bash
gofmt -w internal/modules/socialpublishing
go test ./internal/modules/socialpublishing
```
