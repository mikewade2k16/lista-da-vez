export const WORKSPACES = [
  { id: 'operacao', label: 'Operacao', icon: 'pending_actions', path: '/operacao' },
  { id: 'consultor', label: 'Consultor', icon: 'person', path: '/consultor' },
  { id: 'tasks', label: 'Tasks', icon: 'task_alt', path: '/tasks' },
  { id: 'ranking', label: 'Ranking', icon: 'leaderboard', path: '/ranking' },
  { id: 'dados', label: 'Dados', icon: 'bar_chart', path: '/dados' },
  { id: 'inteligencia', label: 'Inteligencia', icon: 'psychology', path: '/inteligencia' },
  { id: 'relatorios', label: 'Relatorios', icon: 'description', path: '/relatorios' },
  { id: 'campanhas', label: 'Campanhas', icon: 'campaign', path: '/campanhas' },
  { id: 'clientes', label: 'Clientes', icon: 'apartment', path: '/clientes' },
  { id: 'erp', label: 'ERP', icon: 'inventory_2', path: '/erp' },
  { id: 'crm', label: 'CRM', icon: 'insights', path: '/crm' },
  { id: 'multiloja', label: 'Multi-loja', icon: 'store', path: '/multiloja' },
  { id: 'usuarios', label: 'Usuarios', icon: 'group', path: '/usuarios' },
  { id: 'manage', label: 'Manage', icon: 'layout_panel_left', path: '/manage/users' },
  { id: 'configuracoes', label: 'Config', icon: 'tune', path: '/configuracoes' },
  { id: 'themes', label: 'Temas', icon: 'palette', path: '/themes' },
  { id: 'alertas', label: 'Alertas', icon: 'warning', path: '/alertas' },
  { id: 'feedback', label: 'Feedback', icon: 'chat_bubble', path: '/feedback' },
  { id: 'tools', label: 'Tools', icon: 'build', path: '/editor' },
  { id: 'banco', label: 'Banco', icon: 'storage', path: '/banco' },
]

export const QUEUE_WORKSPACES = [
  { id: 'operacao', label: 'Operacao', icon: 'pending_actions', path: '/operacao' },
  { id: 'consultor', label: 'Consultor', icon: 'person', path: '/operacao/consultor' },
  { id: 'ranking', label: 'Ranking', icon: 'leaderboard', path: '/operacao/ranking' },
  { id: 'dados', label: 'Dados', icon: 'bar_chart', path: '/operacao/dados' },
  { id: 'inteligencia', label: 'Inteligencia', icon: 'psychology', path: '/operacao/inteligencia' },
  { id: 'relatorios', label: 'Relatorios', icon: 'description', path: '/operacao/relatorios' },
  { id: 'campanhas', label: 'Campanhas', icon: 'campaign', path: '/operacao/campanhas' },
  { id: 'clientes', label: 'Clientes', icon: 'apartment', path: '/operacao/clientes' },
  { id: 'erp', label: 'ERP', icon: 'inventory_2', path: '/operacao/erp' },
  { id: 'crm', label: 'CRM', icon: 'insights', path: '/operacao/crm' },
  { id: 'multiloja', label: 'Multi-loja', icon: 'store', path: '/operacao/multiloja' },
  { id: 'configuracoes', label: 'Config', icon: 'tune', path: '/operacao/configuracoes' },
  { id: 'alertas', label: 'Alertas', icon: 'warning', path: '/operacao/alertas' },
  { id: 'feedback', label: 'Feedback', icon: 'chat_bubble', path: '/operacao/feedback' },
]

const AUXILIARY_WORKSPACES = [{ id: 'bi', label: 'BI', icon: 'bar_chart', path: '/bi' }]

export const QUEUE_ONLY_WORKSPACE_IDS = new Set(QUEUE_WORKSPACES.map((workspace) => workspace.id))

const workspaceById = new Map(
  [...WORKSPACES, ...AUXILIARY_WORKSPACES].map((workspace) => [workspace.id, workspace]),
)

export const VALID_WORKSPACE_IDS = new Set(
  [...WORKSPACES, ...AUXILIARY_WORKSPACES].map((workspace) => workspace.id),
)

export function getWorkspaceLabel(workspaceId) {
  return workspaceById.get(workspaceId)?.label || ''
}

export function getWorkspacePath(workspaceId) {
  return workspaceById.get(workspaceId)?.path || '/operacao'
}
