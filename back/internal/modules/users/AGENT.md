# AGENT

## Escopo

Estas instrucoes valem para `back/internal/modules/users`.

## Responsabilidade do modulo

O modulo `users` cuida da administracao de usuarios da plataforma dentro do modelo multitenant atual.

Hoje ele deve responder por:

- listar usuarios acessiveis
- criar usuario com papel, escopo e onboarding por convite
- criar usuario com senha inicial definida pelo admin quando esse for o fluxo desejado
- atualizar dados basicos, papel e escopo
- reenviar/gerar convite inicial quando aplicavel
- redefinir senha temporaria de forma administrativa
- inativar usuario

Ele nao deve cuidar de:

- login e emissao de token
- leitura operacional da fila
- configuracoes da loja

## Contrato atual

- `GET /v1/users`
- `POST /v1/users`
- `PATCH /v1/users/{id}`
- `POST /v1/users/{id}/invite`
- `POST /v1/users/{id}/reset-password`
- `POST /v1/users/{id}/archive`

## Regras de escopo

- `platform_admin` pode administrar usuarios de qualquer tenant, inclusive outros `platform_admin`
- `owner` pode administrar usuarios do proprio tenant
- `owner` nao pode criar nem editar `platform_admin`
- `manager`, `consultant`, `marketing` e `director` nao administram usuarios
- `store_terminal` nao administra usuarios

## Regras de modelagem

- o sistema trabalha com um papel efetivo por usuario
- papeis de tenant usam `user_tenant_roles`
- papeis de loja usam `user_store_roles`
  - `consultant`
  - `manager`
  - `store_terminal`
- `platform_admin` usa `user_platform_roles`
- mutacoes devem limpar atribuicoes antigas e regravar apenas o escopo valido para o novo papel
- papeis de loja devem ficar vinculados a uma unica loja por usuario nesta fase
- criar usuario sem senha deve preferir convite, nao senha placeholder
- criar usuario com senha manual nao deve gerar convite
- definicao manual e reset administrativo de senha ficam restritos a `platform_admin`; `owner` segue no fluxo de convite
- criar usuario com senha manual deve marcar a conta com senha temporaria quando o papel for individual
- convite so deve ser gerado para usuario ativo e sem senha definida
- se o admin definir senha manualmente ou inativar a conta, convites pendentes devem ser revogados
- reset administrativo de senha deve marcar `must_change_password = true`, exceto para papeis de terminal fixo quando essa regra nao se aplicar
- o CRUD administrativo de usuarios deve viver em area propria do frontend, separado de `multiloja`
- autoedicao do proprio perfil nao pertence a este modulo; fica em `auth`
- consultores nao devem nascer por este modulo; o fluxo correto e `consultants`
- contas com papel `consultant` e vinculo de roster nao devem ser editadas, convidadas nem inativadas por este modulo
- para contas de consultor, este modulo pode apenas listar e executar reset administrativo de senha
- `platform_admin` pode usar override administrativo para manutencao/debug de contas `consultant`, inclusive mudanca de papel por PATCH quando isso for explicitamente necessario
- esse override administrativo nao cria roster; ele apenas altera o acesso do usuario e deixa o sincronismo do consultor vinculado agir quando houver `consultants.user_id`
