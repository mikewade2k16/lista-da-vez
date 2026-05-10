export type SchemaStatus = "implemented" | "building" | "planned";

export interface SchemaField {
  name: string;
  type: string;
  nullable?: boolean;
  primaryKey?: boolean;
  unique?: boolean;
  foreignKey?: { schema: string; table: string; onDelete?: "cascade" | "set null" | "restrict" };
  default?: string;
  description?: string;
}

export interface SchemaTable {
  name: string;
  description: string;
  status: SchemaStatus;
  phase?: string;
  fields: SchemaField[];
  indexes?: string[];
}

export interface DatabaseSchema {
  id: string;
  label: string;
  description: string;
  status: SchemaStatus;
  phase: string;
  tables: SchemaTable[];
}

export const DATABASE_SCHEMAS: DatabaseSchema[] = [
  {
    id: "core",
    label: "core",
    description: "Plataforma multi-tenant: identidade global, accounts (substitui tenants), organizations, RBAC dinâmico e Module Registry.",
    status: "building",
    phase: "Fase 1 (parcial — Fase 3 conclui RBAC)",
    tables: [
      {
        name: "organizations",
        description: "Agência (opcional). Agrupa accounts.",
        status: "implemented",
        phase: "Fase 1",
        fields: [
          { name: "id", type: "uuid", primaryKey: true, default: "gen_random_uuid()" },
          { name: "slug", type: "text", unique: true },
          { name: "name", type: "text" },
          { name: "is_active", type: "boolean", default: "true" },
          { name: "created_at", type: "timestamptz", default: "now()" },
          { name: "updated_at", type: "timestamptz", default: "now()" }
        ],
        indexes: ["core_organizations_slug_uidx (lower(slug))"]
      },
      {
        name: "accounts",
        description: "Cliente do SaaS. Substitui public.tenants.",
        status: "implemented",
        phase: "Fase 1",
        fields: [
          { name: "id", type: "uuid", primaryKey: true, default: "gen_random_uuid()" },
          { name: "organization_id", type: "uuid", nullable: true, foreignKey: { schema: "core", table: "organizations", onDelete: "set null" } },
          { name: "slug", type: "text", unique: true },
          { name: "name", type: "text" },
          { name: "is_active", type: "boolean", default: "true" },
          { name: "plan_code", type: "text", default: "'standard'" },
          { name: "created_at", type: "timestamptz", default: "now()" },
          { name: "updated_at", type: "timestamptz", default: "now()" }
        ],
        indexes: ["core_accounts_slug_uidx (lower(slug))", "core_accounts_organization_id_idx"]
      },
      {
        name: "users",
        description: "Identidade global. 1 e-mail = 1 user. Sem account_id.",
        status: "implemented",
        phase: "Fase 1",
        fields: [
          { name: "id", type: "uuid", primaryKey: true, default: "gen_random_uuid()" },
          { name: "email", type: "text", unique: true },
          { name: "display_name", type: "text" },
          { name: "password_hash", type: "text", nullable: true, description: "null durante convite pendente" },
          { name: "must_change_password", type: "boolean", default: "false" },
          { name: "avatar_path", type: "text", default: "''" },
          { name: "is_platform_admin", type: "boolean", default: "false" },
          { name: "is_active", type: "boolean", default: "true" },
          { name: "created_at", type: "timestamptz", default: "now()" },
          { name: "updated_at", type: "timestamptz", default: "now()" }
        ],
        indexes: ["core_users_email_lower_uidx (lower(email))"]
      },
      {
        name: "account_users",
        description: "Membership user ↔ account. Chave composta.",
        status: "implemented",
        phase: "Fase 1",
        fields: [
          { name: "account_id", type: "uuid", primaryKey: true, foreignKey: { schema: "core", table: "accounts", onDelete: "cascade" } },
          { name: "user_id", type: "uuid", primaryKey: true, foreignKey: { schema: "core", table: "users", onDelete: "cascade" } },
          { name: "is_active", type: "boolean", default: "true" },
          { name: "invited_by_user_id", type: "uuid", nullable: true, foreignKey: { schema: "core", table: "users", onDelete: "set null" } },
          { name: "joined_at", type: "timestamptz", default: "now()" }
        ],
        indexes: ["core_account_users_user_id_idx"]
      },
      {
        name: "organization_users",
        description: "Membership user ↔ organization (modo agência).",
        status: "implemented",
        phase: "Fase 1",
        fields: [
          { name: "organization_id", type: "uuid", primaryKey: true, foreignKey: { schema: "core", table: "organizations", onDelete: "cascade" } },
          { name: "user_id", type: "uuid", primaryKey: true, foreignKey: { schema: "core", table: "users", onDelete: "cascade" } },
          { name: "org_role", type: "text", description: "agency_owner | agency_member" },
          { name: "joined_at", type: "timestamptz", default: "now()" }
        ],
        indexes: ["core_organization_users_user_id_idx"]
      },
      {
        name: "user_sessions",
        description: "Sessões ativas do user. JWT carrega sessionId; revoked_at faz logout funcionar.",
        status: "implemented",
        phase: "Fase 1",
        fields: [
          { name: "id", type: "uuid", primaryKey: true, default: "gen_random_uuid()" },
          { name: "user_id", type: "uuid", foreignKey: { schema: "core", table: "users", onDelete: "cascade" } },
          { name: "revoked_at", type: "timestamptz", nullable: true },
          { name: "last_seen_at", type: "timestamptz", default: "now()" },
          { name: "user_agent", type: "text", default: "''" },
          { name: "ip", type: "text", default: "''" },
          { name: "created_at", type: "timestamptz", default: "now()" }
        ],
        indexes: ["core_user_sessions_user_id_idx", "core_user_sessions_active_idx (where revoked_at is null)"]
      },
      {
        name: "modules",
        description: "Catálogo de módulos disponíveis na plataforma. Populado pelo SyncCatalog no boot (Fase 2).",
        status: "implemented",
        phase: "Fase 1 (estrutura) — Fase 2 popula",
        fields: [
          { name: "id", type: "text", primaryKey: true, description: "ex: 'queue', 'finance', 'contacts'" },
          { name: "schema_name", type: "text" },
          { name: "label", type: "text" },
          { name: "description", type: "text", default: "''" },
          { name: "is_core", type: "boolean", default: "false" },
          { name: "requires_modules", type: "text[]", default: "{}" },
          { name: "optional_modules", type: "text[]", default: "{}" },
          { name: "sort_order", type: "integer", default: "100" },
          { name: "created_at", type: "timestamptz", default: "now()" },
          { name: "updated_at", type: "timestamptz", default: "now()" }
        ]
      },
      {
        name: "account_modules",
        description: "Módulos habilitados por Account. Controla rotas e menu.",
        status: "implemented",
        phase: "Fase 1 (estrutura) — Fase 2 popula",
        fields: [
          { name: "account_id", type: "uuid", primaryKey: true, foreignKey: { schema: "core", table: "accounts", onDelete: "cascade" } },
          { name: "module_id", type: "text", primaryKey: true, foreignKey: { schema: "core", table: "modules", onDelete: "restrict" } },
          { name: "enabled", type: "boolean", default: "true" },
          { name: "enabled_at", type: "timestamptz", default: "now()" },
          { name: "config", type: "jsonb", default: "'{}'", description: "toggles específicos do módulo na account" }
        ],
        indexes: ["core_account_modules_module_id_idx"]
      },
      {
        name: "permissions",
        description: "Catálogo de permissões declaradas pelos módulos. Sync no boot. Removidas marcam deprecated_at — nunca DELETE auto.",
        status: "implemented",
        phase: "Fase 1 (estrutura) — Fase 2 popula",
        fields: [
          { name: "key", type: "text", primaryKey: true, description: "ex: 'finance.invoices.read'" },
          { name: "module_id", type: "text", foreignKey: { schema: "core", table: "modules", onDelete: "cascade" } },
          { name: "label", type: "text" },
          { name: "description", type: "text", default: "''" },
          { name: "scope", type: "text", description: "account | store | platform", default: "'account'" },
          { name: "deprecated_at", type: "timestamptz", nullable: true },
          { name: "created_at", type: "timestamptz", default: "now()" },
          { name: "updated_at", type: "timestamptz", default: "now()" }
        ],
        indexes: ["core_permissions_module_id_idx"]
      },
      {
        name: "role_templates",
        description: "Templates de cargo declarados pelos módulos (Owner, Admin, Operacional, ...).",
        status: "implemented",
        phase: "Fase 1 (estrutura) — Fase 2 popula",
        fields: [
          { name: "id", type: "text", primaryKey: true, description: "ex: 'core.owner', 'finance.financial'" },
          { name: "module_id", type: "text", foreignKey: { schema: "core", table: "modules", onDelete: "cascade" } },
          { name: "label", type: "text" },
          { name: "description", type: "text", default: "''" },
          { name: "is_system", type: "boolean", default: "true" },
          { name: "sort_order", type: "integer", default: "100" },
          { name: "created_at", type: "timestamptz", default: "now()" },
          { name: "updated_at", type: "timestamptz", default: "now()" }
        ]
      },
      {
        name: "role_template_permissions",
        description: "Matriz template → permissões. Imutável após template criado.",
        status: "implemented",
        phase: "Fase 1 (estrutura) — Fase 2 popula",
        fields: [
          { name: "role_template_id", type: "text", primaryKey: true, foreignKey: { schema: "core", table: "role_templates", onDelete: "cascade" } },
          { name: "permission_key", type: "text", primaryKey: true, foreignKey: { schema: "core", table: "permissions", onDelete: "cascade" } }
        ]
      },
      {
        name: "roles",
        description: "Cargos efetivos da Account (clones editáveis dos templates). 'Owner' é is_locked.",
        status: "implemented",
        phase: "Fase 1 (estrutura) — Fase 3 popula",
        fields: [
          { name: "id", type: "uuid", primaryKey: true, default: "gen_random_uuid()" },
          { name: "account_id", type: "uuid", foreignKey: { schema: "core", table: "accounts", onDelete: "cascade" } },
          { name: "cloned_from_template_id", type: "text", nullable: true, foreignKey: { schema: "core", table: "role_templates", onDelete: "set null" } },
          { name: "code", type: "text", unique: true, description: "único por (account_id, code)" },
          { name: "label", type: "text" },
          { name: "description", type: "text", default: "''" },
          { name: "is_default", type: "boolean", default: "false" },
          { name: "is_locked", type: "boolean", default: "false" },
          { name: "created_at", type: "timestamptz", default: "now()" },
          { name: "updated_at", type: "timestamptz", default: "now()" }
        ],
        indexes: ["core_roles_account_id_idx"]
      },
      {
        name: "role_permissions",
        description: "Permissões efetivas por role. Editado pelo cliente (validado contra catálogo).",
        status: "implemented",
        phase: "Fase 1 (estrutura) — Fase 3 popula",
        fields: [
          { name: "role_id", type: "uuid", primaryKey: true, foreignKey: { schema: "core", table: "roles", onDelete: "cascade" } },
          { name: "permission_key", type: "text", primaryKey: true, foreignKey: { schema: "core", table: "permissions", onDelete: "cascade" } }
        ]
      },
      {
        name: "user_role_assignments",
        description: "Atribuição de cargo a usuário, sempre dentro de uma account.",
        status: "implemented",
        phase: "Fase 1 (estrutura) — Fase 3 popula",
        fields: [
          { name: "id", type: "uuid", primaryKey: true, default: "gen_random_uuid()" },
          { name: "account_id", type: "uuid", foreignKey: { schema: "core", table: "accounts", onDelete: "cascade" } },
          { name: "user_id", type: "uuid", foreignKey: { schema: "core", table: "users", onDelete: "cascade" } },
          { name: "role_id", type: "uuid", foreignKey: { schema: "core", table: "roles", onDelete: "cascade" } },
          { name: "created_at", type: "timestamptz", default: "now()" }
        ],
        indexes: ["core_user_role_assignments_user_idx", "core_user_role_assignments_account_user_idx"]
      },
      {
        name: "user_permission_overrides",
        description: "Allow/deny por usuário. Convive com role.",
        status: "implemented",
        phase: "Fase 1 (estrutura) — Fase 3 popula",
        fields: [
          { name: "id", type: "uuid", primaryKey: true, default: "gen_random_uuid()" },
          { name: "account_id", type: "uuid", foreignKey: { schema: "core", table: "accounts", onDelete: "cascade" } },
          { name: "user_id", type: "uuid", foreignKey: { schema: "core", table: "users", onDelete: "cascade" } },
          { name: "permission_key", type: "text", foreignKey: { schema: "core", table: "permissions", onDelete: "cascade" } },
          { name: "effect", type: "text", description: "allow | deny" },
          { name: "note", type: "text", default: "''" },
          { name: "is_active", type: "boolean", default: "true" },
          { name: "created_by_user_id", type: "uuid", nullable: true, foreignKey: { schema: "core", table: "users", onDelete: "set null" } },
          { name: "created_at", type: "timestamptz", default: "now()" },
          { name: "updated_at", type: "timestamptz", default: "now()" }
        ],
        indexes: [
          "core_user_permission_overrides_lookup_idx (user_id, account_id) WHERE is_active",
          "core_user_permission_overrides_unique_active_uidx (account_id, user_id, permission_key) WHERE is_active"
        ]
      }
    ]
  },
  {
    id: "queue",
    label: "queue",
    description: "Ex-fila-atendimento (atual produto único). Tabelas atualmente em public.* serão movidas para queue.* na Fase 4.",
    status: "planned",
    phase: "Fase 4 (4A → 4D)",
    tables: [
      { name: "stores", description: "Lojas físicas. Hoje em public.stores. Move na Fase 4A.", status: "planned", phase: "Fase 4A", fields: [] },
      { name: "consultants", description: "Roster de consultores. Hoje em public.consultants. Move na Fase 4A.", status: "planned", phase: "Fase 4A", fields: [] },
      { name: "settings_*", description: "Config operacional por tenant. Move na Fase 4A.", status: "planned", phase: "Fase 4A", fields: [] },
      { name: "catalog_*", description: "Produtos/categorias. Move na Fase 4A.", status: "planned", phase: "Fase 4A", fields: [] },
      { name: "operations_*", description: "Fila operacional. Hoje em public. Move na Fase 4B.", status: "planned", phase: "Fase 4B", fields: [] },
      { name: "feedback_*", description: "Canal de feedback. Move na Fase 4B.", status: "planned", phase: "Fase 4B", fields: [] },
      { name: "alerts_*", description: "Regras e incidentes. Move na Fase 4C.", status: "planned", phase: "Fase 4C", fields: [] },
      { name: "analytics_*", description: "Leituras gerenciais. Move na Fase 4C.", status: "planned", phase: "Fase 4C", fields: [] },
      { name: "reports_*", description: "Histórico analítico. Move na Fase 4C.", status: "planned", phase: "Fase 4C", fields: [] },
      { name: "erp_*", description: "Ingestão FTP/ERP. Move na Fase 4C.", status: "planned", phase: "Fase 4C", fields: [] }
    ]
  },
  {
    id: "contacts",
    label: "contacts",
    description: "Core opcional. Quando habilitado, vira fonte de verdade de 'cliente do cliente' para Finance, Omni, etc.",
    status: "planned",
    phase: "Fase 6",
    tables: [
      { name: "contacts", description: "Pessoas e empresas referenciadas pelos outros módulos.", status: "planned", phase: "Fase 6", fields: [] },
      { name: "contact_tags", description: "Tags livres por contato.", status: "planned", phase: "Fase 6", fields: [] },
      { name: "contact_fields", description: "Campos custom por contato.", status: "planned", phase: "Fase 6", fields: [] }
    ]
  },
  {
    id: "finance",
    label: "finance",
    description: "Módulo satélite. Faturamento, recebíveis, comissões. Quando contacts está ativo, usa contact_id; senão, mantém customers locais.",
    status: "planned",
    phase: "Fase 6",
    tables: [
      { name: "customers", description: "Fallback local quando contacts não habilitado.", status: "planned", phase: "Fase 6", fields: [] },
      { name: "invoices", description: "Faturas emitidas/recebidas.", status: "planned", phase: "Fase 6", fields: [] },
      { name: "payments", description: "Recebimentos e baixas.", status: "planned", phase: "Fase 6", fields: [] },
      { name: "commissions", description: "Comissões de vendedores. Pode ser disparado por evento queue.service_finished.", status: "planned", phase: "Fase 6", fields: [] }
    ]
  },
  {
    id: "tasks",
    label: "tasks",
    description: "Módulo satélite (notion-like). Boards, tarefas, sub-tarefas, atribuições.",
    status: "planned",
    phase: "Fase 6",
    tables: [
      { name: "boards", description: "Quadros de tarefas.", status: "planned", phase: "Fase 6", fields: [] },
      { name: "tasks", description: "Tarefas individuais.", status: "planned", phase: "Fase 6", fields: [] }
    ]
  },
  {
    id: "omni",
    label: "omni",
    description: "Módulo satélite (omnichannel WhatsApp/Instagram). Canais, conversas, mensagens.",
    status: "planned",
    phase: "Fase 6",
    tables: [
      { name: "channels", description: "Canais conectados (WhatsApp, IG, etc).", status: "planned", phase: "Fase 6", fields: [] },
      { name: "conversations", description: "Conversas com leads/clientes.", status: "planned", phase: "Fase 6", fields: [] },
      { name: "messages", description: "Mensagens trocadas.", status: "planned", phase: "Fase 6", fields: [] }
    ]
  },
  {
    id: "site",
    label: "site",
    description: "Módulo satélite — landing pages e formulários.",
    status: "planned",
    phase: "Fase 6",
    tables: []
  },
  {
    id: "bio",
    label: "bio",
    description: "Módulo satélite — bio link / link-na-bio para redes sociais.",
    status: "planned",
    phase: "Fase 6",
    tables: []
  }
];
