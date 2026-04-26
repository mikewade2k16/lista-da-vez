# AGENT

## Escopo

Estas instrucoes valem para tudo que for banco de dados no backend:

- schema
- migrations
- convencoes SQL
- modelagem tenant/store
- documentacao estrutural do banco

## Versoes padrao desta base

- Go: `1.24.0`
- Toolchain Go: `1.24.3`
- Nuxt no frontend integrado: `4.4.2`
- Pinia no frontend integrado: `3.0.4`
- Driver PostgreSQL no backend: `pgx/v5 5.7.6`
- PostgreSQL alvo: `16`
- Tag recomendada para Docker futuro: `postgres:16-alpine`

## Localizacao atual

- migrations SQL: `back/internal/platform/database/migrations`
- pool/conexao: `back/internal/platform/database/pool.go`
- runner de migrations: `back/internal/platform/database/migrator.go`
- comando de migration: `back/cmd/migrate/main.go`
- visualizacao humana: `back/database/ERD.md`
- visualizacao local no Windows: `back/scripts/postgres/open-pgadmin.ps1`

## Regras de modelagem

### 1. Multi-tenant

- toda entidade de negocio futura deve considerar `tenant_id` quando fizer sentido
- tudo que for operacional por loja deve considerar tambem `store_id`
- `platform_admin` fica fora de tenant
- contas de loja/operacao com escopo de store devem tender a uma unica loja por usuario
  - hoje isso vale para `consultant`, `manager` e `store_terminal`

### 2. IDs

- usar `uuid` como PK nas tabelas principais
- evitar PK numerica incremental como identificador externo

### 3. Datas

- usar `timestamptz`
- toda tabela principal deve ter pelo menos `created_at`
- tabelas mutaveis devem ter `updated_at`

### 4. Soft delete vs archive

- preferir `is_active`/`archived_at` quando a regra pedir historico preservado
- nao apagar dado operacional importante sem motivo forte

### 5. Email e unicidade

- email deve ser unico case-insensitive
- no PostgreSQL atual estamos tratando isso com indice em `lower(email)`
- onboarding de usuario por convite deve usar tabela dedicada
  - `user_invitations`
  - token persistido apenas em hash
  - `users.password_hash` pode ficar nulo enquanto o convite nao for aceito
- senhas temporarias administrativas devem usar flag explicita
  - `users.must_change_password`
  - nao depender de comparar hash ou tentar inferir pela senha padrao
- o modelo de acesso operacional deve suportar:
  - conta individual do consultor
  - conta fixa do terminal da loja
  - conta gerencial por loja
  - conta tenant-wide para `owner` e `marketing`
- foto de perfil do usuario:
  - o banco guarda apenas `users.avatar_path`
  - o arquivo binario nao deve ir para coluna blob nesta fase
  - no Docker, o storage local deve ficar em volume dedicado para nao perder arquivo no recreate do container

### 6. Regra de payload e mutacao

- banco nao deve ser tratado como destino de bundles gigantes para alteracoes pequenas
- uma alteracao pequena deve gerar:
  - payload pequeno na API
  - SQL pequeno e previsivel
  - escrita focada apenas nas linhas/colunas afetadas
- campos opcionais ou nao aplicaveis devem ser omitidos no JSON sempre que o backend puder assumir zero-value com seguranca
- evitar estrategia de `delete all + insert all` quando a intencao do usuario for adicionar, editar ou remover um unico item
- manter endpoints bulk apenas para cenarios de importacao, seed controlada, template ou substituicao total intencional
- sempre considerar custo de I/O, lock, WAL, rede e observabilidade ao desenhar mutacoes
- exclusoes administrativas precisam validar dependencias de negocio antes de executar `delete`
- `on delete cascade` deve ser tratado como protecao de integridade, nao como politica primaria de remocao
- leituras administrativas expandidas devem ficar separadas das leituras operacionais normais para manter o bootstrap leve
- seguranca por loja/dispositivo ainda e backlog, mas a modelagem deve preservar espaco para:
  - vinculo entre conta de terminal e loja
  - auditoria de sessao por dispositivo
  - futuras restricoes de login por origem/estacao
- para leitura analitica, preferir indices e consultas coerentes com o recorte real da tela:
  - `store_id`
  - intervalo de tempo
  - consultor
  - desfecho
- para visao operacional integrada multi-loja:
  - consultas precisam continuar baratas por `tenant_id + store_id`
  - o dado devolvido ao frontend deve carregar identificacao visual minima da loja (`store_id`, `store_name`, opcionalmente `store_code`)
  - a tabela `operation_paused_consultants` deve preservar o tipo do afastamento (`pause` vs `assignment`)
  - a configuracao operacional vive em tabelas `tenant_*` desde a migration `0024_tenant_operation_settings.sql`:
    - `tenant_operation_settings`
    - `tenant_setting_options`
    - `tenant_catalog_products`
  - a tabela `tenant_setting_options` precisa manter `kind` alinhado com os catalogos vivos da UI:
    - `visit_reason`
    - `customer_source`
    - `pause_reason`
    - `queue_jump_reason`
    - `loss_reason`
    - `profession`
  - as tabelas legadas `store_operation_settings`, `store_setting_options` e `store_catalog_products` permanecem no banco apenas como fonte de backfill durante a transicao; novas escritas vao para o escopo do tenant
- quando o historico tiver campos estruturados em JSON, a agregacao deve priorizar a colecao estruturada como fonte de verdade
  - exemplo: `productsClosed[]` antes de `productClosed`

### 7. Resiliencia futura

- a modelagem deve facilitar:
  - reprocessamento idempotente
  - fila de sincronizacao offline
  - auditoria de mutacoes
  - recuperacao apos falha de rede ou de servico
- isso ainda nao e a prioridade de implementacao, mas ja e uma restricao arquitetural da base

## Regras de migrations

- migrations sao append-only
- nao editar migration ja aplicada em ambiente compartilhado
- nomear com prefixo ordenavel:
  - `0001_init.sql`
  - `0002_seed_demo_auth.sql`
- migration deve ser idempotente quando possivel para facilitar setup local

## Comandos uteis

```bash
go run ./cmd/migrate up
go run ./cmd/migrate status
```

```powershell
.\scripts\postgres\status-local.ps1
.\scripts\postgres\open-pgadmin.ps1
```

Ambiente esperado:

- `DATABASE_URL`
- opcionalmente `DATABASE_MIN_CONNS`
- opcionalmente `DATABASE_MAX_CONNS`

## O que consultar antes de mexer no schema

1. `back/PLAN.md`
2. `back/README.md`
3. `back/database/ERD.md`
4. `../web/app/pages/operacao/operations.md`
5. `../docs/NUXT_FULL_REFERENCE.md`

## Proxima evolucao esperada

- campos administrativos completos por loja
  - template operacional padrao
  - metas por loja
- consolidacao de leitura para multiloja e usuarios/acessos
- convites e onboarding de usuarios ja modelados com `user_invitations`
- vinculo 1:1 entre consultor operacional e conta autenticada
- politica de primeiro login e senha temporaria ja usa `users.must_change_password`
- conta `store_terminal` por loja para computador fixo da unidade
- visao integrada da operacao ja exige que o banco sustente leitura cross-store leve para `owner` e `platform_admin`
- sessoes persistidas de auth
- auditoria/eventos para realtime e resiliencia offline
- estrategia futura de backup automatizado, restore testado e redundancia do banco
