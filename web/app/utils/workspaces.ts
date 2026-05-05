export const WORKSPACES = [
  { id: "operacao", label: "Operacao", icon: "pending_actions", path: "/operacao" },
  { id: "consultor", label: "Consultor", icon: "person", path: "/consultor" },
  { id: "ranking", label: "Ranking", icon: "leaderboard", path: "/ranking" },
  { id: "dados", label: "Dados", icon: "bar_chart", path: "/dados" },
  { id: "inteligencia", label: "Inteligencia", icon: "psychology", path: "/inteligencia" },
  { id: "relatorios", label: "Relatorios", icon: "description", path: "/relatorios" },
  { id: "campanhas", label: "Campanhas", icon: "campaign", path: "/campanhas" },
  { id: "clientes", label: "Clientes", icon: "apartment", path: "/clientes" },
  { id: "erp", label: "ERP", icon: "inventory_2", path: "/erp" },
  { id: "multiloja", label: "Multi-loja", icon: "store", path: "/multiloja" },
  { id: "usuarios", label: "Usuarios", icon: "group", path: "/usuarios" },
  { id: "configuracoes", label: "Config", icon: "tune", path: "/configuracoes" },
  { id: "alertas", label: "Alertas", icon: "warning", path: "/alertas" },
  { id: "feedback", label: "Feedback", icon: "chat_bubble", path: "/feedback" },
  { id: "banco", label: "Banco", icon: "storage", path: "/banco" }
];

const workspaceById = new Map(WORKSPACES.map((workspace) => [workspace.id, workspace]));

export const VALID_WORKSPACE_IDS = new Set(WORKSPACES.map((workspace) => workspace.id));

export function getWorkspaceLabel(workspaceId) {
  return workspaceById.get(workspaceId)?.label || "";
}

export function getWorkspacePath(workspaceId) {
  return workspaceById.get(workspaceId)?.path || "/operacao";
}
