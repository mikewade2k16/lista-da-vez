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
- `GET /v1/feedback?kind=&status=` — administradores listam com filtros
- `PATCH /v1/feedback/{id}` — administradores atualizam status e notas

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
