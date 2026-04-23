# ERD

## Visao atual do banco

```mermaid
erDiagram
    USERS {
        uuid id PK
        text email
        text display_name
        text password_hash
        boolean must_change_password
      text employee_code
      text job_title
        text avatar_path
        boolean is_active
        timestamptz created_at
        timestamptz updated_at
    }

    USER_INVITATIONS {
        uuid id PK
        uuid user_id FK
        text email
        uuid invited_by_user_id FK
        text token_hash
        text status
        timestamptz expires_at
        timestamptz accepted_at
        timestamptz revoked_at
        timestamptz created_at
        timestamptz updated_at
    }

    TENANTS {
        uuid id PK
        text slug
        text name
        boolean is_active
        timestamptz created_at
        timestamptz updated_at
    }

    STORES {
        uuid id PK
        uuid tenant_id FK
        text code
        text name
        text city
        boolean is_active
        text default_template_id
        numeric monthly_goal
        numeric weekly_goal
        numeric avg_ticket_goal
        numeric conversion_goal
        numeric pa_goal
        timestamptz created_at
        timestamptz updated_at
    }

    USER_PLATFORM_ROLES {
        uuid user_id PK,FK
        text role
        timestamptz created_at
    }

    USER_TENANT_ROLES {
        uuid id PK
        uuid user_id FK
        uuid tenant_id FK
        text role
        timestamptz created_at
    }

    USER_STORE_ROLES {
        uuid id PK
        uuid user_id FK
        uuid store_id FK
        text role
        timestamptz created_at
    }

    ACCESS_PERMISSIONS {
      text key PK
      text scope
      text description
      timestamptz created_at
    }

    ACCESS_ROLE_PERMISSIONS {
      text role PK
      text permission_key PK,FK
      timestamptz created_at
    }

    USER_ACCESS_OVERRIDES {
      uuid id PK
      uuid user_id FK
      text permission_key FK
      text effect
      uuid tenant_id FK
      uuid store_id FK
      uuid created_by_user_id FK
      text note
      boolean is_active
      timestamptz created_at
      timestamptz updated_at
    }

    STORE_TERMINALS {
      uuid id PK
      uuid tenant_id FK
      uuid store_id FK
      uuid user_id FK
      text code
      text device_label
      text device_slug
      text access_mode
      boolean is_active
      timestamptz last_seen_at
      timestamptz created_at
      timestamptz updated_at
    }

    CONSULTANTS {
        uuid id PK
        uuid tenant_id FK
        uuid store_id FK
        uuid user_id FK
        text name
        text role_label
        text initials
        text color
        numeric monthly_goal
        numeric commission_rate
        numeric conversion_goal
        numeric avg_ticket_goal
        numeric pa_goal
        boolean is_active
        timestamptz created_at
        timestamptz updated_at
    }

    STORE_OPERATION_SETTINGS {
        uuid store_id PK,FK
        text selected_operation_template_id
        integer max_concurrent_services
        integer timing_fast_close_minutes
        integer timing_long_service_minutes
        numeric timing_low_sale_amount
        boolean test_mode_enabled
        boolean auto_fill_finish_modal
        numeric alert_min_conversion_rate
        numeric alert_max_queue_jump_rate
        numeric alert_min_pa_score
        numeric alert_min_ticket_average
        text title
        text product_seen_label
        text product_seen_placeholder
        text product_closed_label
        text product_closed_placeholder
        text notes_label
        text notes_placeholder
        text queue_jump_reason_label
        text queue_jump_reason_placeholder
        text loss_reason_label
        text loss_reason_placeholder
        text customer_section_label
        boolean show_email_field
        boolean show_profession_field
        boolean show_notes_field
        text visit_reason_selection_mode
        text visit_reason_detail_mode
        text loss_reason_selection_mode
        text loss_reason_detail_mode
        text customer_source_selection_mode
        text customer_source_detail_mode
        boolean require_product
        boolean require_visit_reason
        boolean require_customer_source
        boolean require_customer_name_phone
        timestamptz created_at
        timestamptz updated_at
    }

    STORE_SETTING_OPTIONS {
        uuid store_id PK,FK
        text kind PK
        text option_id PK
        text label
        integer sort_order
        timestamptz created_at
        timestamptz updated_at
    }

    STORE_CATALOG_PRODUCTS {
        uuid store_id PK,FK
        text product_id PK
        text name
        text code
        text category
        numeric base_price
        integer sort_order
        timestamptz created_at
        timestamptz updated_at
    }

    OPERATION_QUEUE_ENTRIES {
        uuid store_id PK,FK
        uuid consultant_id PK,FK
        integer sort_order
        bigint queue_joined_at
        timestamptz created_at
    }

    OPERATION_ACTIVE_SERVICES {
        uuid store_id PK,FK
        uuid consultant_id PK,FK
        text service_id
        bigint service_started_at
        bigint queue_joined_at
        bigint queue_wait_ms
        integer queue_position_at_start
        text start_mode
        jsonb skipped_people_json
        timestamptz created_at
        timestamptz updated_at
    }

    OPERATION_PAUSED_CONSULTANTS {
        uuid store_id PK,FK
        uuid consultant_id PK,FK
        text reason
        text kind
        bigint started_at
        timestamptz created_at
        timestamptz updated_at
    }

    OPERATION_CURRENT_STATUS {
        uuid store_id PK,FK
        uuid consultant_id PK,FK
        text status
        bigint started_at
        timestamptz created_at
        timestamptz updated_at
    }

    OPERATION_STATUS_SESSIONS {
        uuid id PK
        uuid store_id FK
        uuid consultant_id FK
        text status
        bigint started_at
        bigint ended_at
        bigint duration_ms
        timestamptz created_at
    }

    OPERATION_SERVICE_HISTORY {
        uuid id PK
        uuid store_id FK
        text service_id
        uuid person_id FK
        text person_name
        bigint started_at
        bigint finished_at
        bigint duration_ms
        text finish_outcome
        text start_mode
        integer queue_position_at_start
        bigint queue_wait_ms
        jsonb skipped_people_json
        integer skipped_count
        boolean is_window_service
        boolean is_gift
        text product_seen
        text product_closed
        text product_details
        jsonb products_seen_json
        jsonb products_closed_json
        boolean products_seen_none
        boolean visit_reasons_not_informed
        boolean customer_sources_not_informed
        text customer_name
        text customer_phone
        text customer_email
        boolean is_existing_customer
        jsonb visit_reasons_json
        jsonb visit_reason_details_json
        jsonb customer_sources_json
        jsonb customer_source_details_json
        jsonb loss_reasons_json
        jsonb loss_reason_details_json
        text loss_reason_id
        text loss_reason
        numeric sale_amount
        text customer_profession
        text queue_jump_reason
        text notes
        jsonb campaign_matches_json
        numeric campaign_bonus_total
        timestamptz created_at
    }

    TENANTS ||--o{ STORES : owns
    USERS ||--o| USER_PLATFORM_ROLES : has
    USERS ||--o{ USER_TENANT_ROLES : has
    USERS ||--o{ USER_STORE_ROLES : has
    USERS ||--o{ USER_INVITATIONS : onboarding
    USERS ||--o{ USER_ACCESS_OVERRIDES : overrides
    TENANTS ||--o{ USER_TENANT_ROLES : scopes
    STORES ||--o{ USER_STORE_ROLES : scopes
    TENANTS ||--o{ STORE_TERMINALS : owns
    STORES ||--|| STORE_TERMINALS : device_access
    USERS ||--|| STORE_TERMINALS : terminal_login
    ACCESS_PERMISSIONS ||--o{ ACCESS_ROLE_PERMISSIONS : granted_to_role
    ACCESS_PERMISSIONS ||--o{ USER_ACCESS_OVERRIDES : overridden
    TENANTS ||--o{ CONSULTANTS : scopes
    STORES ||--o{ CONSULTANTS : roster
    STORES ||--|| STORE_OPERATION_SETTINGS : config
    STORES ||--o{ STORE_SETTING_OPTIONS : catalogs
    STORES ||--o{ STORE_CATALOG_PRODUCTS : catalog
    STORES ||--o{ OPERATION_QUEUE_ENTRIES : queue
    STORES ||--o{ OPERATION_ACTIVE_SERVICES : active_services
    STORES ||--o{ OPERATION_PAUSED_CONSULTANTS : paused
    STORES ||--o{ OPERATION_CURRENT_STATUS : current_status
    STORES ||--o{ OPERATION_STATUS_SESSIONS : status_history
    STORES ||--o{ OPERATION_SERVICE_HISTORY : service_history
    CONSULTANTS ||--o{ OPERATION_QUEUE_ENTRIES : queue_member
    CONSULTANTS ||--o{ OPERATION_ACTIVE_SERVICES : serves
    CONSULTANTS ||--o{ OPERATION_PAUSED_CONSULTANTS : pauses
    CONSULTANTS ||--o{ OPERATION_CURRENT_STATUS : current_status
    CONSULTANTS ||--o{ OPERATION_STATUS_SESSIONS : status_sessions
    CONSULTANTS ||--o{ OPERATION_SERVICE_HISTORY : closes
```

## Leitura rapida

- `users`
  - identidade base da pessoa autenticada
  - `password_hash` pode nascer nulo durante onboarding por convite
  - `employee_code` guarda matricula quando houver
  - `job_title` guarda o cargo exibivel do acesso
  - `avatar_path` guarda apenas o caminho publico da foto; o arquivo vive no volume do backend
- `user_invitations`
- `users.must_change_password`
  - trilha de convite/onboarding e aceite inicial de senha
- `tenants`
  - cliente/dono do grupo
- `stores`
  - lojas pertencentes a um tenant, incluindo template padrao e metas administrativas
- `user_platform_roles`
  - acesso interno de plataforma, hoje para `platform_admin`
- `user_tenant_roles`
  - papeis no escopo do tenant, hoje `marketing`, `director` e `owner`
- `user_store_roles`
  - papeis no escopo da loja, hoje `consultant`, `manager` e `store_terminal`
- `access_permissions`
  - catalogo central de capacidades por escopo
- `access_role_permissions`
  - grants default por papel para preparar visibilidade configuravel no produto
- `user_access_overrides`
  - excecoes allow/deny por usuario em nivel de tenant ou loja
- `store_terminals`
  - identidade fixa dos computadores das lojas, 1:1 com store e login tecnico read-only
- `consultants`
  - roster administrativo por loja para a operacao
  - no seed MVP cada consultor ja nasce com vinculo 1:1 em `users`
- `store_operation_settings`
  - configuracao escalar da loja para a operacao e o modal
- `store_setting_options`
  - catalogos configuraveis da loja, tipados por `kind`
- `store_catalog_products`
  - catalogo de produtos configuravel da loja
- `operation_queue_entries`
  - fila corrente por loja
- `operation_active_services`
  - atendimentos em andamento
- `operation_paused_consultants`
  - pausas correntes por consultor
- `operation_current_status`
  - status atual resumido por consultor
- `operation_status_sessions`
  - trilha append-only das transicoes de status
- `operation_service_history`
  - historico append-only do fechamento operacional

## Seeds atuais

A migration de seed cria:

- `tenant-demo`
- 4 lojas operacionais (`Riomar`, `Jardins`, `Garcia`, `Treze`)
- 29 acessos reais de MVP entre consultores, gerentes, marketing, diretoria, terminais e dev
- memberships coerentes com os papeis atuais do auth
- terminais fixos por loja com login tecnico dedicado

## Observacoes de modelagem

- `settings` deixou de viver em um JSON gigante e foi normalizado por tabela
- `operations` usa tabelas correntes para snapshot rapido e tabelas append-only para historico
- `reports` le o historico principalmente por `store_id` + `finished_at`, com indices dedicados para tempo, consultor e desfecho
- `user_invitations` guarda o token em hash, nunca o token aberto
- onboarding inicial funciona assim:
  - admin cria usuario sem senha
  - backend gera convite com expiracao
  - usuario aceita o convite e define a primeira senha
- a base agora tambem possui catalogo de permissoes e overrides por usuario para permitir evolucao de visibilidade dentro da plataforma sem redesenhar auth
- alguns campos do historico continuam em `jsonb` por serem listas e mapas variaveis do fechamento, como:
  - `products_seen_json`
  - `products_closed_json`
  - `visit_reasons_json`
  - `visit_reason_details_json`
  - `customer_sources_json`
  - `customer_source_details_json`
  - `loss_reasons_json`
  - `loss_reason_details_json`
- para agregacao e filtros, o dado estruturado em `jsonb` deve ser tratado como fonte de verdade antes dos campos escalares legados
  - `campaign_matches_json`

## Proxima camada que deve entrar aqui

- websocket/outbox de eventos por loja
- campanhas server-side
- relatorios e analytics server-side
- endurecimento do modelo de identidade operacional:
  - consultor como conta real obrigatoria
  - terminal de loja como conta fixa com operacao completa da propria unidade
  - futuras amarras de dispositivo/origem por loja
  - editor de grants/overrides aproveitando `access_permissions` e `user_access_overrides`
