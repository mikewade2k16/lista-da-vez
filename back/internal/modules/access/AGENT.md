# AGENT

## Escopo

Estas instrucoes valem para `back/internal/modules/access`.

## Responsabilidade do modulo

O modulo `access` centraliza o catalogo de permissoes do painel, os grants padrao por papel e os overrides por usuario.

Ele deve cuidar de:

- catalogo de permissoes reutilizavel pelo backend e pelo painel administrativo
- grants padrao por papel
- merge entre grants de papel e overrides por usuario
- services/handlers para editar a matriz de acesso
- resolver permissoes efetivas usadas por `auth.Principal`

Ele nao deve cuidar de:

- autenticacao ou emissao de token
- CRUD generico de usuarios
- regras operacionais da fila

## Contrato atual

- `GET /v1/access/roles`
- `PUT /v1/access/roles/{roleId}`
- `GET /v1/access/users/{userId}`
- `PUT /v1/access/users/{userId}/overrides`

## Regras

- o catalogo precisa continuar explicito e legivel; nao esconder permissoes em strings soltas pelo codigo
- defaults por papel devem refletir o baseline atual do produto antes de qualquer override
- override por usuario sempre deve conseguir `allow` ou `deny` sobre o padrao do papel
- grants de role podem ser globais; overrides de usuario continuam podendo carregar contexto de tenant/loja quando necessario
- o modulo deve devolver tanto `basePermissionKeys` quanto `effectivePermissionKeys` para permitir auditoria no painel
- permissoes efetivas devem ser calculadas com precedencia dos overrides ativos sobre o grant padrao do papel

## Permissoes atuais

O catalogo cobre workspaces (`operacao`, `consultor`, `ranking`, `dados`, `inteligencia`, `relatorios`, `campanhas`, `clientes`, `multiloja`, `usuarios`, `configuracoes`) e acoes administrativas de plataforma (`users.password.manage`, `access.role_defaults.manage`).

WebSocket ainda nao usa permissoes `realtime.*` no catalogo Go atual; a conexao operacional valida `workspace.operacao.view` quando o principal ja vem com permissoes resolvidas.

Alteracoes em grants padrao por papel e overrides por usuario precisam publicar `context.updated` com `resource = access` no canal de contexto do tenant, para que outras sessoes revalidem `GET /v1/me/context` e a UI administrativa de acessos sem depender de reload manual.
