export const WORKSPACES = [
  { id: "operacao", label: "Operacao", icon: "pending_actions", path: "/operacao" },
  { id: "consultor", label: "Consultor", icon: "person", path: "/operacao/consultor" },
  { id: "ranking", label: "Ranking", icon: "leaderboard", path: "/operacao/ranking" },
  { id: "dados", label: "Dados", icon: "bar_chart", path: "/operacao/dados" },
  { id: "inteligencia", label: "Inteligencia", icon: "psychology", path: "/operacao/inteligencia" },
  { id: "relatorios", label: "Relatorios", icon: "description", path: "/operacao/relatorios" },
  { id: "campanhas", label: "Campanhas", icon: "campaign", path: "/operacao/campanhas" },
  { id: "clientes", label: "Clientes", icon: "apartment", path: "/operacao/clientes" },
  { id: "erp", label: "ERP", icon: "inventory_2", path: "/operacao/erp" },
  { id: "crm", label: "CRM", icon: "insights", path: "/operacao/crm" },
  { id: "multiloja", label: "Multi-loja", icon: "store", path: "/operacao/multiloja" },
  { id: "usuarios", label: "Usuarios", icon: "group", path: "/operacao/usuarios" },
  { id: "configuracoes", label: "Config", icon: "tune", path: "/operacao/configuracoes" },
  { id: "alertas", label: "Alertas", icon: "warning", path: "/operacao/alertas" },
  { id: "feedback", label: "Feedback", icon: "chat_bubble", path: "/operacao/feedback" },
  { id: "tasks", label: "Tasks", icon: "task_alt", path: "/tasks" },
  { id: "themes", label: "Temas", icon: "palette", path: "/themes" },
  { id: "banco", label: "Banco", icon: "storage", path: "/operacao/banco" }
];

const workspaceById = new Map(WORKSPACES.map((workspace) => [workspace.id, workspace]));

export const VALID_WORKSPACE_IDS = new Set(WORKSPACES.map((workspace) => workspace.id));

/** IDs que vivem APENAS no workspace nav (não devem aparecer no header/sidebar) */
export const QUEUE_ONLY_WORKSPACE_IDS = new Set(
  WORKSPACES
    .filter(w => !["operacao", "tasks", "themes"].includes(w.id))
    .map(w => w.id)
);

export function getWorkspaceLabel(workspaceId) {
  return workspaceById.get(workspaceId)?.label || "";
}

export function getWorkspacePath(workspaceId) {
  return workspaceById.get(workspaceId)?.path || "/operacao";
}
