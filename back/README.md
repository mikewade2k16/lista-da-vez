# Back

Base inicial da API em Go para o modulo de fila de atendimento.

## Objetivo desta fase

Subir um backend solido desde o inicio, com:

- modulo de auth/autorizacao real
- arquitetura modular dentro de `internal/`
- base pronta para multitenancy
- transporte HTTP organizado para futura entrada de websocket, PostgreSQL e filas de eventos

## Modelo de tenant adotado

Nesta primeira fase, o sistema ja nasce preparado para multitenancy com a seguinte regra:

- `owner` representa o dono do cliente, ou seja, o futuro tenant owner
- `consultant`, `manager`, `store_terminal` e `marketing` pertencem a um `tenant_id`
- `platform_admin` fica acima dos tenants e serve para areas internas da plataforma

Hoje usamos um tenant demo (`tenant-demo`) para nao travar a entrega inicial, mas o `tenant_id` ja existe no usuario autenticado e no token.

Agora o auth ja usa PostgreSQL real para `users`, `user_platform_roles`, `user_tenant_roles` e `user_store_roles`.

## Roles iniciais

- `consultant`
  - escopo de loja
- `store_terminal`
  - escopo de loja, operacao completa da unidade com acesso apenas a telas seguras de leitura fora de `/operacao`
- `manager`
  - escopo de loja com visao acima dos consultores da loja
- `marketing`
  - escopo de tenant para campanhas e inteligencia cross-store
- `owner`
  - escopo de tenant com visao total do cliente/grupo
- `platform_admin`
  - escopo de plataforma para times internos/dev

Observacao de modelagem:

- `platform_admin` continua sendo o root global da plataforma e o ponto de acesso total entre clientes
- um tenant de desenvolvimento como `Omni` pode existir como cliente comum para teste, mas nao substitui o papel global de `platform_admin`

## Estrutura atual

- `cmd/api`
  - bootstrap da aplicacao
- `cmd/migrate`
  - aplica migrations no PostgreSQL
- `internal/platform/config`
  - leitura de envs e configuracao base
- `internal/platform/httpapi`
  - helpers JSON e middlewares HTTP
- `internal/platform/server`
  - configuracao do `http.Server`
- `internal/platform/database`
  - pool PostgreSQL e runner de migrations
- `internal/platform/app`
  - composicao dos modulos e montagem do handler principal
- `internal/modules/auth`
  - roles, usuarios, token, middleware e handlers do auth
- `internal/modules/consultants`
  - roster administrativo de consultores por loja, com conta autenticada 1:1 do consultor
- `internal/modules/settings`
  - bundle configuravel da operacao por loja
- `internal/modules/realtime`
  - WebSocket autenticado para sincronizacao operacional por loja
- `internal/modules/reports`
  - leituras analiticas e gerenciais server-side sobre o historico operacional
- `internal/modules/analytics`
  - agregados server-side para `ranking`, `dados` e `inteligencia`
- `database`
  - documentacao humana do schema e regras de banco

## Endpoints iniciais

- `GET /healthz`
- `GET /v1/auth/roles`
- `POST /v1/auth/login`
- `GET /v1/auth/me`
- `GET /v1/auth/invitations/{token}`
- `POST /v1/auth/invitations/accept`
- `GET /v1/me/context`
- `GET /v1/tenants`
- `GET /v1/stores`
- `POST /v1/stores`
- `PATCH /v1/stores/{id}`
- `GET /v1/consultants`
- `POST /v1/consultants`
- `PATCH /v1/consultants/{id}`
- `POST /v1/consultants/{id}/archive`
- `GET /v1/settings`
- `PUT /v1/settings`
- `PATCH /v1/settings/operation`
- `PATCH /v1/settings/modal`
- `POST /v1/settings/options/{group}`
- `PATCH /v1/settings/options/{group}/{itemId}`
- `DELETE /v1/settings/options/{group}/{itemId}`
- `PUT /v1/settings/options/{group}`
- `POST /v1/settings/products`
- `PATCH /v1/settings/products/{id}`
- `DELETE /v1/settings/products/{id}`
- `PUT /v1/settings/products`
- `GET /v1/operations/snapshot`
- `POST /v1/operations/queue`
- `POST /v1/operations/pause`
- `POST /v1/operations/resume`
- `POST /v1/operations/start`
- `POST /v1/operations/finish`
- `GET /v1/realtime/operations`
- `GET /v1/reports/overview`
- `GET /v1/reports/results`
- `GET /v1/reports/recent-services`
- `GET /v1/analytics/ranking`
- `GET /v1/analytics/data`
- `GET /v1/analytics/intelligence`
- `GET /v1/users`
- `POST /v1/users`
- `PATCH /v1/users/{id}`
- `POST /v1/users/{id}/invite`
- `POST /v1/users/{id}/reset-password`
- `POST /v1/users/{id}/archive`
- `GET /v1/dev/ping`
  - protegido para `platform_admin`

## Credenciais demo

Todos os usuarios abaixo usam a senha `dev123456`:

- `consultor@demo.local`
- `gerente@demo.local`
- `marketing@demo.local`
- `proprietario@demo.local`
- `plataforma@demo.local`

Para o seed MVP atual do root de desenvolvimento da plataforma:

- `mikewade2k16@gmail.com`
- senha: `Mvp@2026!`

No fluxo Docker local, a migration `0033_seed_dev_platform_admin_password.sql` regrava essa senha para o `platform_admin` seedado.
Ela entra apenas em ambientes nao-producao.

## Onboarding por convite

Fluxo atual:

1. admin cria o usuario sem senha em `POST /v1/users`
2. a API gera `user_invitations` com token em hash e devolve um `inviteUrl`
3. o usuario abre `/auth/convite/:token` no Nuxt
4. `POST /v1/auth/invitations/accept` grava a primeira senha e devolve sessao valida

Observacoes:

- `users.password_hash` pode ficar nulo enquanto o convite estiver pendente
- `POST /v1/auth/login` responde `onboarding_required` se a conta ainda nao definiu a primeira senha
- `POST /v1/users/{id}/invite` regenera o convite de um usuario ainda nao onboarded

## Politica atual para consultores

- todo consultor novo nasce como:
  - registro em `consultants`
  - usuario real em `users`
  - papel `consultant` no escopo da loja
- o email padrao e gerado automaticamente por nome + loja + dominio configurado
- a senha inicial padrao vem de `AUTH_CONSULTANT_DEFAULT_PASSWORD`
- no ambiente atual de desenvolvimento:
  - dominio padrao: `acesso.omni.local`
  - senha inicial padrao: `Omni@123`
- o consultor nasce com `must_change_password = true`
- o consultor deve trocar a senha em `/perfil` no primeiro acesso

## Analytics server-side

As telas abaixo ja nao devem depender de recalculo pesado no browser:

- `/ranking`
- `/dados`
- `/inteligencia`

Regra atual:

- `GET /v1/analytics/ranking` entrega ranking mensal, ranking diario e alertas prontos
- `GET /v1/analytics/data` entrega agregados de produtos, motivos, origens, profissoes e horarios
- `GET /v1/analytics/intelligence` entrega diagnosticos, severidade, score e acoes recomendadas

Objetivo:

- reduzir custo de processamento no frontend
- evitar trafegar historico bruto quando a tela so precisa de agregados
- centralizar a regra analitica no backend para manter consistencia entre dispositivos e futuras exportacoes

## Variaveis de ambiente

- `APP_NAME`
- `APP_ENV`
- `APP_ADDR`
- `DATABASE_URL`
- `DATABASE_MIN_CONNS`
- `DATABASE_MAX_CONNS`
- `WEB_APP_URL`
- `CORS_ALLOWED_ORIGINS`
- `AUTH_TOKEN_SECRET`
- `AUTH_TOKEN_TTL`
- `AUTH_INVITE_TTL`
- `AUTH_PASSWORD_RESET_TTL`
- `SMTP_HOST`
- `SMTP_PORT`
- `SMTP_USERNAME`
- `SMTP_PASSWORD`
- `SMTP_FROM_EMAIL`
- `SMTP_FROM_NAME`
- `SMTP_TLS_MODE`
- `SMTP_INSECURE_SKIP_VERIFY`
- `SMTP_TIMEOUT`
- `AUTH_BCRYPT_COST`
- `AUTH_CONSULTANT_EMAIL_DOMAIN`
- `AUTH_CONSULTANT_DEFAULT_PASSWORD`

Arquivo base:

- `.env.example`

Observacao sobre entrega de email:

- sem `SMTP_HOST`, o backend continua no modo dev e escreve o codigo de reset no log da API
- com SMTP configurado, `POST /v1/auth/password-reset/request` envia o codigo por email real
- `SMTP_TLS_MODE=starttls` cobre o caso mais comum de porta `587`; para porta `465`, usar `SMTP_TLS_MODE=tls`

## Matriz de versoes

- Go do modulo: `1.24.0`
- toolchain Go: `1.24.3`
- PostgreSQL alvo: `16`
- imagem Docker do banco: `postgres:16-alpine`
- imagem Docker do backend: `golang:1.24.0-bookworm`

## Fluxo oficial com Docker

O backend agora roda por padrao dentro do Compose da raiz.

Pela raiz do repositorio:

```bash
npm run dev
```

Ou apenas a stack em background:

```bash
npm run dev:detach
```

Servicos relevantes:

- `postgres`
- `api`

O container da API:

- espera o PostgreSQL ficar saudavel
- aplica `migrate up`
- sobe a API em `:8080`

Se aparecer erro SQL em rota nova mesmo com a API saudavel, trate primeiro como drift entre schema e volume local.
Exemplo real desta base: settings tenant-wide passaram a depender de coluna nova em `tenant_operation_settings`; nesse caso o check correto e confirmar `migrate status` dentro do container e nao criar um novo tenant para mascarar a falta de coluna.

Arquivo opcional para customizacao do Compose:

- `../.env.docker.example`

Validacao minima do Compose:

```bash
docker compose config
docker compose ps
```

## Banco e migrations

Versao alvo de PostgreSQL:

- `16`
- imagem recomendada para Docker futuro: `postgres:16-alpine`

Comandos:

```bash
go run ./cmd/migrate up
go run ./cmd/migrate status
```

## Setup local no Windows sem Docker

Guia rapido:

- `START_LOCAL.md`

Esse fluxo agora e apenas fallback quando Docker nao estiver disponivel.

Scripts de apoio:

- `scripts/postgres/init-local.ps1`
- `scripts/postgres/start-local.ps1`
- `scripts/postgres/stop-local.ps1`
- `scripts/postgres/status-local.ps1`
- `scripts/postgres/open-pgadmin.ps1`
- `scripts/api/start-local.ps1`
- `scripts/api/status-local.ps1`
- `scripts/api/stop-local.ps1`

Fluxo recomendado:

```powershell
.\scripts\postgres\status-local.ps1
.\scripts\postgres\init-local.ps1
.\scripts\postgres\open-pgadmin.ps1
```

O script:

- reutiliza o servico `postgresql-x64-16` quando ele existir
- cai para um cluster manual em `%LOCALAPPDATA%\lista-da-vez\postgres16\data` quando o servico nao existir
- sobe o servidor na porta `5432`
- cria a role `lista_da_vez`
- cria o banco `lista_da_vez`
- aplica as migrations do projeto

Credenciais padrao da aplicacao:

- host: `localhost`
- porta: `5432`
- database: `lista_da_vez`
- user: `lista_da_vez`
- password: `lista_da_vez_dev`

Observacao de CORS local:

- a API aceita `localhost`, `127.0.0.1` e `[::1]` em qualquer porta local
- isso evita `Failed to fetch` quando o Nuxt sobe em `3000` ou `3001`

Visualizacao local:

- `pgAdmin 4` instalado em `C:\Program Files\PostgreSQL\16\pgAdmin 4\runtime\pgAdmin4.exe`
- atalho do projeto: `.\scripts\postgres\open-pgadmin.ps1`

Visualizacao do schema:

- `database/ERD.md`

## Estado atual da operacao

Hoje `operations` ja esta integrado em:

- frontend Nuxt
- API Go
- PostgreSQL

O snapshot operacional por loja e persistido e lido de:

- `operation_queue_entries`
- `operation_active_services`
- `operation_paused_consultants`
- `operation_current_status`
- `operation_status_sessions`
- `operation_service_history`

O frontend ainda usa o `app-runtime` como camada de compatibilidade visual, mas a fonte de verdade da fila e do historico agora vem da API.

Contrato atual de leitura e mutacao:

- `GET /v1/operations/snapshot` devolve o snapshot completo da loja
- `POST /v1/operations/queue|pause|resume|start|finish` devolvem apenas `ack` minimo
- apos cada mutacao, o Nuxt revalida o snapshot operacional por `GET /v1/operations/snapshot`
- `GET /v1/realtime/operations?storeId=...&access_token=...` entrega eventos leves de invalidacao para a UI revalidar a loja ativa sem refresh

Regra atual de payload no backend:

- leitura pode devolver bundle/snapshot quando isso for o proprio caso de uso
- escrita deve ser granular por padrao
- adicionar, editar ou excluir um item nao deve trafegar a colecao inteira
- endpoints bulk ficam reservados para importacao, template ou substituicao total intencional

Primeiro contrato de `reports`:

- `GET /v1/reports/overview` entrega agregados pequenos para dashboards
- `GET /v1/reports/results` entrega linhas paginadas para a tabela de resultados
- `GET /v1/reports/recent-services` entrega leitura administrativa dos ultimos atendimentos
- produtos fechados passam a considerar `productsClosed[]` como fonte de verdade, com `productClosed` apenas como fallback legado

## Proxima etapa recomendada

1. evoluir o hub local de realtime para broker externo quando houver multiplas replicas da API
2. ligar a futura tela de ultimos atendimentos ao endpoint `GET /v1/reports/recent-services`
3. sincronizar `settings.updated` e `campaigns.updated` sem refresh
4. avancar para analytics, multiloja e campanhas server-side
5. adicionar replay/resume e metricas de conexao do realtime
