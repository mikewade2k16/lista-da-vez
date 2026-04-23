# AGENT

## Escopo

Estas instrucoes valem para `back/internal/modules/auth`.

## Versoes herdadas desta base

- Go: `1.24.0`
- Toolchain Go: `1.24.3`
- Nuxt integrado no frontend: `4.4.2`
- PostgreSQL alvo: `16`

## Responsabilidade do modulo

O modulo `auth` e a porta de entrada de identidade e autorizacao da plataforma.

Ele deve cuidar de:

- login
- aceite de convite e primeira senha
- recuperacao de senha por codigo
- emissao e leitura de token
- principal autenticado
- catalogo de roles
- middleware de auth/role
- autoatendimento do usuario autenticado:
  - editar nome
  - editar email
  - trocar senha
  - enviar foto de perfil

Ele nao deve cuidar de:

- regra operacional da fila
- CRUD de loja
- CRUD de tenant
- websocket de fila

## Como o auth funciona hoje

### Persistencia

- usuarios vivem no PostgreSQL
- memberships tenant/store sao lidas de:
  - `users`
  - `user_invitations`
  - `user_platform_roles`
  - `user_tenant_roles`
  - `user_store_roles`
- a migration demo continua semeando usuarios para smoke local

### Token

- token assinado via HMAC
- `tenant_id`, `store_ids` e `role` ja vao no token
- ainda nao existe refresh token
- ainda nao existe sessao persistida por dispositivo

### Roles atuais

- `consultant`
  - escopo de loja
- `store_terminal`
  - escopo de loja
  - operacao completa da propria unidade
- `manager`
  - escopo de loja
- `marketing`
  - escopo de tenant
- `director`
  - escopo de tenant
- `owner`
  - escopo de tenant
- `platform_admin`
  - escopo de plataforma

### Endpoints atuais

- `GET /v1/auth/roles`
- `POST /v1/auth/login`
- `POST /v1/auth/password-reset/request`
- `POST /v1/auth/password-reset/confirm`
- `GET /v1/auth/me`
- `PATCH /v1/auth/me/profile`
- `PATCH /v1/auth/me/password`
- `POST /v1/auth/me/avatar`
- `GET /v1/auth/invitations/{token}`
- `POST /v1/auth/invitations/accept`

## Invariantes

- email deve ser tratado normalizado em lowercase
- usuario inativo nao pode autenticar
- usuario sem `password_hash` deve receber `onboarding_required` no login
- usuario com `must_change_password = true` pode autenticar, mas deve ser conduzido ao fluxo de troca de senha no frontend
- token invalido ou expirado gera `401`
- role fora do catalogo e erro de modelagem
- `platform_admin` pode atravessar limites de tenant; os demais nao
- `consultant`, `manager` e `store_terminal` devem carregar uma unica loja no escopo efetivo
- token de convite deve ser persistido apenas em hash
- codigo de recuperacao de senha deve ser persistido apenas em hash
- foto de perfil nao deve ser salva em blob no PostgreSQL desta base
  - o banco guarda apenas `users.avatar_path`
  - o arquivo fica em disco/volume montado no backend
- convite aceito deve:
  - gravar a primeira senha
  - limpar `must_change_password`
  - revogar convites pendentes restantes do usuario
  - devolver sessao valida para entrar sem login extra
- troca de senha em `/v1/auth/me/password` deve limpar `must_change_password`
- atualizacao de perfil do usuario autenticado deve refletir o nome do consultor no roster quando houver vinculo `consultants.user_id`

## Regras para evolucao

Quando este modulo crescer, a ordem certa e:

1. adicionar refresh token e sessao por dispositivo
2. auditar login/logout e revogacao
3. expor autorizacao reutilizavel para websocket handshake
4. separar permissoes por grant para reduzir dependencia de role fixa
5. evoluir convite para entrega por email real
6. endurecer sessao de `store_terminal` por loja/dispositivo
7. auditar melhor resets de senha e primeiro login por dispositivo

## Cuidados ao integrar com o Nuxt

- o front deve usar `POST /v1/auth/login` e `GET /v1/me/context` antes de qualquer realtime
- `GET /v1/auth/me` continua util para leitura simples de identidade, mas o contexto de tenant/loja agora vem do endpoint composto
- o dropdown de perfil de teste do frontend deve ficar oculto quando houver sessao real
- workspaces visiveis no front devem derivar do principal autenticado, nao de mock local
- a loja ativa do runtime local deve respeitar `store_ids` do principal autenticado

## Arquivos atuais

- `model.go`
- `roles.go`
- `service.go`
- `tokens.go`
- `middleware.go`
- `http.go`
- `store_postgres.go`
- `store_memory.go`
- `passwords.go`
- `errors.go`

## Auth como host, nao como dependencia obrigatoria do core

Quando outro projeto quiser reaproveitar o core Omni, este modulo pode ser substituido pelo auth do sistema host.

Nesse caso, o host precisa apenas conseguir entregar para os modulos do core:

- um contexto equivalente a `user_id + tenant_id + role + store_ids[]`
- um handshake autenticado para websocket quando usar realtime

Referencia:

- `../../CORE_MODULES_PORTABILITY.md`
