# AGENT

## Escopo

Estas instrucoes valem para `back/internal/modules/feedback`.

## Responsabilidade do modulo

O modulo `feedback` cuida do canal de comunicacao onde usuarios enviam sugestoes, duvidas e relatos de problemas. Administradores acessam uma tela dedicada para visualizar, classificar e responder essas mensagens.

Hoje ele deve responder por:

- receber feedback de usuarios autenticados
- persistir feedback com informacoes do usuario e loja
- listar feedbacks para administradores com filtros por tipo e status
- permitir que administradores atualizem status e adicionem notas internas

Ele nao deve cuidar de:

- notificacoes automaticas para administradores
- envio de respostas por email ou chat
- analise automatica de sentimento
- integracao com sistemas de ticketing externos

## Contrato atual

- `POST /v1/feedback` — qualquer usuario autenticado cria feedback
- `GET /v1/feedback?kind=&status=&since=` — administradores listam com filtros
- `GET /v1/feedback/me?kind=&status=&since=` — usuario lista os proprios chamados
- `PATCH /v1/feedback/{id}` — administradores atualizam status e notas
- `GET /v1/feedback/{id}/messages?after=` / `POST /v1/feedback/{id}/messages` — thread do chamado
- `POST /v1/feedback/{id}/read` — marca o chamado como lido para o viewer

### Agregados na listagem (ToListView)

Os dois endpoints de list devolvem, por feedback, alem dos campos do feedback:

- `unread_count` (int, sempre presente no list) — mensagens nao lidas pela perspectiva do viewer. Computado por linha no SQL: se o viewer e o dono do feedback, conta respostas de terceiros (`author <> user_feedback.user_id`); se e admin, conta mensagens do criador (`author = user_feedback.user_id`); sempre `created_at > read_at` do viewer. Espelha `getUnreadCount`/`isUnreadForViewer` do front.
- `last_message_body` + `last_message_at` (omitempty) — preview da ultima mensagem da conversa. Como o backend marca o feedback como lido ao responder (CreateMessage chama MarkRead do autor), a ultima mensagem e o sinal certo de novidade quando `unread_count > 0`.

So o list popula esses campos (via `ToListView`); as respostas de mutacao usam `ToView` e os omitem (ponteiros nil), e o front preserva o ultimo valor do list em vez de zerar. Isso elimina o fan-out em que o sino/lista baixavam mensagens de cada feedback so para contar nao-lidos.

## Regras de acesso

- criacao: qualquer usuario autenticado
- leitura: `owner`, `manager`, `platform_admin`
- atualizacao: `owner`, `manager`, `platform_admin`

## Regras de dados

- feedback e criado dentro do escopo do tenant e loja do usuario autenticado
- cada feedback registra o nome e ID do usuario criador
- administrador pode adicionar notas internas sem visibilidade para o usuario criador
- tipos de feedback: `suggestion`, `question`, `problem`
- status possiveis: `open`, `in_progress`, `resolved`, `closed`
- status padrao ao criar: `open`

## Observacoes de integracao

- o modulo nao depende de outros modulos alem de `auth`
- pode ser usado como base para expandir para chat/suporte futuro

## Evolucao futura mapeada (resolver ao mexer no modulo)

- **Empurrar por WebSocket em vez de polling.** O `unread_count` + `last_message_*` no list (feito em 2026-06-25) ja eliminou o fan-out de mensagens; o sino/lista agora so chamam o list periodicamente (60s o sino, 30s o workspace) e o chat aberto faz poll de 15s. O passo final e o modulo `realtime` (que ja tem canais de operacao/tasks/presence) publicar evento de feedback novo / resposta nova e o front assinar, zerando o polling. Mapeado no roadmap (fase-7, task `feedback-realtime`).
