const ROLE_WORKSPACES = {
  admin: ["operacao", "consultor", "ranking", "dados", "inteligencia", "relatorios", "campanhas", "multiloja", "configuracoes"],
  manager: ["operacao", "consultor", "ranking", "dados", "inteligencia", "relatorios", "campanhas", "multiloja"],
  consultant: ["operacao", "consultor", "dados"]
};

export function getAllowedWorkspaces(role) {
  return ROLE_WORKSPACES[role] || ROLE_WORKSPACES.consultant;
}

export function canManageSettings(role) {
  return role === "admin";
}

export function canManageConsultants(role) {
  return role === "admin";
}

export function canAccessReports(role) {
  return role === "admin" || role === "manager";
}

export function canManageCampaigns(role) {
  return role === "admin";
}

export function canManageStores(role) {
  return role === "admin";
}

export function canAccessMultiStore(role) {
  return role === "admin" || role === "manager";
}
