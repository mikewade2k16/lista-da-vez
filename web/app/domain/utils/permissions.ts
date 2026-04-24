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

const ROLE_WORKSPACES = {
  platform_admin: ["operacao", "consultor", "ranking", "dados", "inteligencia", "relatorios", "campanhas", "multiloja", "usuarios", "configuracoes"],
  owner: ["operacao", "consultor", "ranking", "dados", "inteligencia", "relatorios", "campanhas", "multiloja", "usuarios", "configuracoes"],
  marketing: ["operacao"],
  director: ["operacao"],
  manager: ["operacao"],
  store_terminal: ["operacao", "consultor", "ranking", "dados", "inteligencia", "relatorios"],
  consultant: ["operacao"],
  admin: ["operacao", "consultor", "ranking", "dados", "inteligencia", "relatorios", "campanhas", "multiloja", "usuarios", "configuracoes"]
};

export function normalizeAppRole(role) {
  const normalized = String(role || "").trim();
  return ROLE_ALIAS[normalized] || normalized || "consultant";
}

export function getAllowedWorkspaces(role) {
  return ROLE_WORKSPACES[normalizeAppRole(role)] || ROLE_WORKSPACES.consultant;
}

export function canManageSettings(role) {
  const normalized = normalizeAppRole(role);
  return normalized === "platform_admin" || normalized === "owner";
}

export function canManageConsultants(role) {
  const normalized = normalizeAppRole(role);
  return normalized === "platform_admin" || normalized === "owner";
}

export function canAccessReports(role) {
  const normalized = normalizeAppRole(role);
  return normalized === "platform_admin" || normalized === "owner" || normalized === "director" || normalized === "marketing" || normalized === "manager" || normalized === "store_terminal";
}

export function canMutateOperations(role) {
  const normalized = normalizeAppRole(role);
  return normalized === "platform_admin" || normalized === "owner" || normalized === "manager" || normalized === "consultant" || normalized === "store_terminal";
}

export function canManageCampaigns(role) {
  const normalized = normalizeAppRole(role);
  return normalized === "platform_admin" || normalized === "owner" || normalized === "marketing";
}

export function canManageStores(role) {
  const normalized = normalizeAppRole(role);
  return normalized === "platform_admin" || normalized === "owner";
}

export function canManageUsers(role) {
  const normalized = normalizeAppRole(role);
  return normalized === "platform_admin" || normalized === "owner";
}

export function canManageUserPasswords(role) {
  const normalized = normalizeAppRole(role);
  return normalized === "platform_admin";
}

export function canAccessMultiStore(role) {
  const normalized = normalizeAppRole(role);
  return normalized === "platform_admin" || normalized === "owner" || normalized === "director" || normalized === "marketing";
}

export function getRoleLabel(role) {
  return ROLE_LABELS[normalizeAppRole(role)] || "Consultor";
}
