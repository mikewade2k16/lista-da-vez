const ROLE_ALIAS = {
  admin: "platform_admin"
};

const ROLE_LABELS = {
  consultant: "Consultor",
  store_terminal: "Acesso da loja",
  manager: "Gerente",
  marketing: "Marketing",
  director: "Diretoria",
  owner: "Proprietario",
  platform_admin: "Admin da plataforma",
  admin: "Admin da plataforma"
};

export const WORKSPACE_ACCESS_DEFINITIONS = [
  {
    id: "operacao",
    label: "Operacao",
    description: "Fila, snapshot e comandos operacionais.",
    viewPermission: "workspace.operacao.view",
    editPermission: "workspace.operacao.edit"
  },
  {
    id: "consultor",
    label: "Consultor",
    description: "Painel individual do consultor.",
    viewPermission: "workspace.consultor.view",
    editPermission: ""
  },
  {
    id: "ranking",
    label: "Ranking",
    description: "Leitura de performance comercial.",
    viewPermission: "workspace.ranking.view",
    editPermission: ""
  },
  {
    id: "dados",
    label: "Dados",
    description: "Leitura operacional detalhada.",
    viewPermission: "workspace.dados.view",
    editPermission: ""
  },
  {
    id: "inteligencia",
    label: "Inteligencia",
    description: "Diagnostico e leitura automatica.",
    viewPermission: "workspace.inteligencia.view",
    editPermission: ""
  },
  {
    id: "relatorios",
    label: "Relatorios",
    description: "Relatorios consolidados e filtros analiticos.",
    viewPermission: "workspace.relatorios.view",
    editPermission: ""
  },
  {
    id: "campanhas",
    label: "Campanhas",
    description: "Regras comerciais e cadastro de campanhas.",
    viewPermission: "workspace.campanhas.view",
    editPermission: "workspace.campanhas.edit"
  },
  {
    id: "clientes",
    label: "Clientes",
    description: "Clientes, agencias e status do grupo atendido.",
    viewPermission: "workspace.clientes.view",
    editPermission: "workspace.clientes.edit"
  },
  {
    id: "erp",
    label: "ERP",
    description: "Sync ERP/FTP, status dos lotes e busca de produtos consolidada.",
    viewPermission: "workspace.erp.view",
    editPermission: "workspace.erp.edit"
  },
  {
    id: "multiloja",
    label: "Multi-loja",
    description: "Consolidado e administracao de lojas.",
    viewPermission: "workspace.multiloja.view",
    editPermission: "workspace.multiloja.edit"
  },
  {
    id: "usuarios",
    label: "Usuarios",
    description: "Usuarios, overrides e matriz de acesso.",
    viewPermission: "workspace.usuarios.view",
    editPermission: "workspace.usuarios.edit"
  },
  {
    id: "configuracoes",
    label: "Configuracoes",
    description: "Templates, catalogos e parametros operacionais.",
    viewPermission: "workspace.configuracoes.view",
    editPermission: "workspace.configuracoes.edit"
  },
  {
    id: "alertas",
    label: "Alertas",
    description: "Incidentes operacionais em realtime, acknowledge e regras do modulo.",
    viewPermission: "workspace.alertas.view",
    editPermission: "workspace.alertas.edit"
  },
  {
    id: "feedback",
    label: "Feedback",
    description: "Sugestoes, duvidas e problemas dos usuarios.",
    viewPermission: "workspace.feedback.view",
    editPermission: "workspace.feedback.edit"
  },
  {
    id: "banco",
    label: "Banco",
    description: "Estrutura do banco de dados — tabelas, campos, relacionamentos e status de migracao.",
    viewPermission: "",
    editPermission: ""
  }
];

export const ADVANCED_ACCESS_DEFINITIONS = [
  {
    key: "users.password.manage",
    label: "Resetar senha administrativa",
    description: "Permite redefinir senha manual pelo painel."
  },
  {
    key: "access.role_defaults.manage",
    label: "Editar padrao por perfil",
    description: "Permite editar a matriz padrao de acesso por papel."
  },
  {
    key: "alerts.rules.manage",
    label: "Editar regras de alertas",
    description: "Permite alterar thresholds e canais internos do modulo de alertas."
  },
  {
    key: "alerts.actions.manage",
    label: "Executar acoes de alertas",
    description: "Permite acknowledge e resolucao manual de alertas operacionais."
  }
];

const ROLE_WORKSPACES = {
  platform_admin: ["operacao", "consultor", "ranking", "dados", "inteligencia", "relatorios", "campanhas", "clientes", "erp", "multiloja", "usuarios", "configuracoes", "alertas", "feedback", "banco"],
  owner: ["operacao", "consultor", "ranking", "dados", "inteligencia", "relatorios", "campanhas", "clientes", "erp", "multiloja", "usuarios", "configuracoes", "alertas", "feedback"],
  marketing: ["operacao", "erp"],
  director: ["operacao", "erp"],
  manager: ["operacao", "erp", "alertas", "feedback"],
  store_terminal: ["operacao", "consultor", "ranking", "dados", "inteligencia", "relatorios", "alertas"],
  consultant: ["operacao"],
  admin: ["operacao", "consultor", "ranking", "dados", "inteligencia", "relatorios", "campanhas", "clientes", "erp", "multiloja", "usuarios", "configuracoes", "alertas", "feedback"]
};

export function normalizeAppRole(role) {
  const normalized = String(role || "").trim();
  return ROLE_ALIAS[normalized] || normalized || "consultant";
}

export function normalizePermissionKeys(permissionKeys = []) {
  return Array.isArray(permissionKeys)
    ? permissionKeys.map((permissionKey) => String(permissionKey || "").trim()).filter(Boolean)
    : [];
}

export function hasPermission(permissionKeys, permissionKey) {
  const normalizedPermission = String(permissionKey || "").trim();
  if (!normalizedPermission) {
    return false;
  }

  return normalizePermissionKeys(permissionKeys).includes(normalizedPermission);
}

export function getWorkspaceAccessDefinition(workspaceId) {
  return WORKSPACE_ACCESS_DEFINITIONS.find((workspace) => workspace.id === String(workspaceId || "").trim()) || null;
}

export function getWorkspaceAccessOptions(workspaceDefinition, { includeInherit = false } = {}) {
  const options = [];

  if (includeInherit) {
    options.push({ value: "inherit", label: "Herdar padrao" });
  }

  options.push({ value: "none", label: "Sem acesso" });
  options.push({ value: "view", label: "Somente ver" });

  if (String(workspaceDefinition?.editPermission || "").trim()) {
    options.push({ value: "edit", label: "Ver e editar" });
  }

  return options;
}

export function readWorkspaceAccessState(workspaceDefinition, permissionKeys, fallbackState = "none") {
  const viewPermission = String(workspaceDefinition?.viewPermission || "").trim();
  const editPermission = String(workspaceDefinition?.editPermission || "").trim();
  if (!viewPermission) {
    return fallbackState;
  }

  if (!hasPermission(permissionKeys, viewPermission)) {
    return fallbackState;
  }

  if (editPermission && hasPermission(permissionKeys, editPermission)) {
    return "edit";
  }

  return "view";
}

export function writeWorkspaceAccessState(workspaceDefinition, permissionKeys, nextState) {
  const viewPermission = String(workspaceDefinition?.viewPermission || "").trim();
  const editPermission = String(workspaceDefinition?.editPermission || "").trim();
  const nextPermissions = normalizePermissionKeys(permissionKeys).filter(
    (permissionKey) => permissionKey !== viewPermission && permissionKey !== editPermission
  );

  switch (String(nextState || "").trim()) {
    case "edit":
      if (viewPermission) {
        nextPermissions.push(viewPermission);
      }
      if (editPermission) {
        nextPermissions.push(editPermission);
      }
      break;
    case "view":
      if (viewPermission) {
        nextPermissions.push(viewPermission);
      }
      break;
    case "none":
    case "inherit":
    default:
      break;
  }

  return normalizePermissionKeys(nextPermissions);
}

export function getAllowedWorkspaces(role, permissionKeys = [], permissionsResolved = false) {
  const normalizedRole = normalizeAppRole(role);
  const roleDefaults = new Set(ROLE_WORKSPACES[normalizedRole] || ROLE_WORKSPACES.consultant);

  if (!permissionsResolved) {
    return [...roleDefaults];
  }

  // Com permissoes resolvidas via banco:
  // - workspace COM viewPermission: exige a chave no token (controle fino por usuario)
  // - workspace SEM viewPermission: segue ROLE_WORKSPACES sem necessidade de migration
  return WORKSPACE_ACCESS_DEFINITIONS
    .filter((workspace) => {
      const viewPermission = String(workspace.viewPermission || "").trim();
      if (!viewPermission) {
        return roleDefaults.has(workspace.id);
      }
      return hasPermission(permissionKeys, viewPermission);
    })
    .map((workspace) => workspace.id);
}

export function canManageSettings(role, permissionKeys = [], permissionsResolved = false) {
  if (permissionsResolved) {
    return hasPermission(permissionKeys, "workspace.configuracoes.edit");
  }

  const normalized = normalizeAppRole(role);
  return normalized === "platform_admin" || normalized === "owner";
}

export function canManageConsultants(role, permissionKeys = [], permissionsResolved = false) {
  if (permissionsResolved) {
    return hasPermission(permissionKeys, "workspace.configuracoes.edit");
  }

  const normalized = normalizeAppRole(role);
  return normalized === "platform_admin" || normalized === "owner";
}

export function canAccessReports(role, permissionKeys = [], permissionsResolved = false) {
  if (permissionsResolved) {
    return hasPermission(permissionKeys, "workspace.relatorios.view");
  }

  const normalized = normalizeAppRole(role);
  return normalized === "platform_admin" || normalized === "owner" || normalized === "director" || normalized === "marketing" || normalized === "manager" || normalized === "store_terminal";
}

export function canMutateOperations(role, permissionKeys = [], permissionsResolved = false) {
  if (permissionsResolved) {
    return hasPermission(permissionKeys, "workspace.operacao.edit");
  }

  const normalized = normalizeAppRole(role);
  return normalized === "platform_admin" || normalized === "owner" || normalized === "manager" || normalized === "consultant" || normalized === "store_terminal";
}

export function canManageCampaigns(role, permissionKeys = [], permissionsResolved = false) {
  if (permissionsResolved) {
    return hasPermission(permissionKeys, "workspace.campanhas.edit");
  }

  const normalized = normalizeAppRole(role);
  return normalized === "platform_admin" || normalized === "owner" || normalized === "marketing";
}

export function canManageStores(role, permissionKeys = [], permissionsResolved = false) {
  if (permissionsResolved) {
    return hasPermission(permissionKeys, "workspace.multiloja.edit");
  }

  const normalized = normalizeAppRole(role);
  return normalized === "platform_admin" || normalized === "owner";
}

export function canAccessClients(role, permissionKeys = [], permissionsResolved = false) {
  const normalized = normalizeAppRole(role);

  if (permissionsResolved) {
    return hasPermission(permissionKeys, "workspace.clientes.view") || normalized === "platform_admin" || normalized === "owner";
  }

  return normalized === "platform_admin" || normalized === "owner";
}

export function canManageTenants(role, permissionKeys = [], permissionsResolved = false) {
  const normalized = normalizeAppRole(role);

  if (permissionsResolved) {
    return hasPermission(permissionKeys, "workspace.clientes.edit") || normalized === "platform_admin" || normalized === "owner";
  }

  return normalized === "platform_admin" || normalized === "owner";
}

export function canManageUsers(role, permissionKeys = [], permissionsResolved = false) {
  if (permissionsResolved) {
    return hasPermission(permissionKeys, "workspace.usuarios.edit");
  }

  const normalized = normalizeAppRole(role);
  return normalized === "platform_admin" || normalized === "owner";
}

export function canManageUserPasswords(role, permissionKeys = [], permissionsResolved = false) {
  if (permissionsResolved) {
    return hasPermission(permissionKeys, "users.password.manage");
  }

  const normalized = normalizeAppRole(role);
  return normalized === "platform_admin";
}

export function canManageRoleDefaults(role, permissionKeys = [], permissionsResolved = false) {
  if (permissionsResolved) {
    return hasPermission(permissionKeys, "access.role_defaults.manage");
  }

  return normalizeAppRole(role) === "platform_admin";
}

export function canUseAllStoresScope(storeIds = []) {
  const normalizedStoreIds = Array.isArray(storeIds)
    ? storeIds.map((storeId) => String(storeId || "").trim()).filter(Boolean)
    : [];

  return new Set(normalizedStoreIds).size > 1;
}

export function canAccessMultiStore(role, permissionKeys = [], permissionsResolved = false) {
  if (permissionsResolved) {
    return hasPermission(permissionKeys, "workspace.multiloja.view");
  }

  const normalized = normalizeAppRole(role);
  return normalized === "platform_admin" || normalized === "owner" || normalized === "director" || normalized === "marketing";
}

export function getRoleLabel(role) {
  return ROLE_LABELS[normalizeAppRole(role)] || "Consultor";
}
