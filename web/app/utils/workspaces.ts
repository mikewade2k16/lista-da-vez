export const WORKSPACES = [
  { id: 'operacao', label: 'Operacao', icon: 'pending_actions', path: '/operacao' },
  { id: 'transcricoes', label: 'Transcrições', icon: 'mic', path: '/transcricoes' },
  { id: 'campanhas', label: 'Campanhas', icon: 'campaign', path: '/campanhas' },
  // ATENCAO: `icon` aqui e ligature do Material Icons Round (o
  // DashboardWorkspaceNav renderiza <span class="material-icons-round">{{ icon }}</span>),
  // NAO chave do NAV_ICON_MAP. Usar 'messages' (a chave do nav.config.ts) renderiza
  // o texto cru. 'forum' e ligature valida — vizinhos: task_alt, calendar_month.
  { id: 'omnichannel', label: 'Omnichannel', icon: 'forum', path: '/omnichannel' },
  { id: 'consultor', label: 'Consultor', icon: 'person', path: '/consultor' },
  { id: 'tasks', label: 'Tasks', icon: 'task_alt', path: '/tasks' },
  { id: 'calendar', label: 'Calendario', icon: 'calendar_month', path: '/calendario' },
  {
    id: 'social_publishing',
    label: 'Agendamento de postagens',
    icon: 'schedule_send',
    path: '/postagens',
  },
  { id: 'ranking', label: 'Ranking', icon: 'leaderboard', path: '/ranking' },
  { id: 'dados', label: 'Dados', icon: 'bar_chart', path: '/dados' },
  { id: 'inteligencia', label: 'Inteligencia', icon: 'psychology', path: '/inteligencia' },
  {
    id: 'customer_intelligence',
    label: 'Inteligencia de clientes',
    icon: 'psychology',
    path: '/inteligencia-clientes',
  },
  { id: 'relatorios', label: 'Relatorios', icon: 'description', path: '/relatorios' },
  {
    id: 'site_produtos_web',
    label: 'Produtos do Site',
    icon: 'dashboard_customize',
    path: '/site/produtos',
  },
  {
    id: 'site_leads_web',
    label: 'Leads do Site',
    icon: 'dashboard_customize',
    path: '/site/leads',
  },
  {
    id: 'site_tracking_web',
    label: 'Tracking do Site',
    icon: 'dashboard_customize',
    path: '/site/tracking',
  },
  {
    id: 'site_bio_web',
    label: 'Bio',
    icon: 'dashboard_customize',
    path: '/site/bio',
  },
  {
    id: 'cardapio_web',
    label: 'Presence',
    icon: 'dashboard_customize',
    path: '/cardapio',
  },
  {
    id: 'clientes_web',
    label: 'Clientes Web',
    icon: 'dashboard_customize',
    path: '/manage/clientes-web',
  },
  { id: 'erp', label: 'ERP', icon: 'inventory_2', path: '/erp' },
  { id: 'crm', label: 'CRM', icon: 'insights', path: '/crm' },
  { id: 'multiloja', label: 'Multi-loja', icon: 'store', path: '/multiloja' },
  {
    id: 'planejamento',
    label: 'Planejamento',
    icon: 'calendar_month',
    path: '/planejamento/metas',
  },
  { id: 'usuarios', label: 'Usuarios', icon: 'group', path: '/operacao/usuarios' },
  { id: 'usuarios_admin', label: 'Usuarios Admin', icon: 'manage_accounts', path: '/manage/users' },
  {
    id: 'organizations_admin',
    label: 'Organizations',
    icon: 'corporate_fare',
    path: '/manage/organizations',
  },
  {
    id: 'role_templates_admin',
    label: 'Papeis-padrao',
    icon: 'admin_panel_settings',
    path: '/manage/role-templates',
  },
  { id: 'storage_admin', label: 'Storage R2', icon: 'cloud', path: '/manage/storage' },
  { id: 'manage', label: 'Manage', icon: 'layout_panel_left', path: '/manage/users' },
  { id: 'configuracoes', label: 'Config', icon: 'tune', path: '/configuracoes' },
  { id: 'menu_layout', label: 'Organização do Menu', icon: 'tune', path: '/manage/menu-layout' },
  { id: 'themes', label: 'Temas', icon: 'palette', path: '/themes' },
  { id: 'alertas', label: 'Alertas', icon: 'warning', path: '/alertas' },
  { id: 'feedback', label: 'Suporte', icon: 'chat_bubble', path: '/suporte' },
  { id: 'tools', label: 'Tools', icon: 'build', path: '/editor' },
  { id: 'banco', label: 'Banco', icon: 'storage', path: '/banco' },
  { id: 'automation', label: 'Automacao', icon: 'psychology', path: '/automation' },
  { id: 'meta_ads', label: 'Meta Ads', icon: 'campaign', path: '/meta-ads' },
  { id: 'finance', label: 'Finance', icon: 'payments', path: '/finance' },
  { id: 'performance', label: 'Performance', icon: 'speed', path: '/performance' },
]

export const QUEUE_WORKSPACES = [
  { id: 'operacao', label: 'Operacao', icon: 'pending_actions', path: '/operacao' },
  { id: 'transcricoes', label: 'Transcrições', icon: 'mic', path: '/transcricoes' },
  { id: 'campanhas', label: 'Campanhas', icon: 'campaign', path: '/campanhas' },
  { id: 'consultor', label: 'Consultor', icon: 'person', path: '/operacao/consultor' },
  { id: 'ranking', label: 'Ranking', icon: 'leaderboard', path: '/operacao/ranking' },
  { id: 'erp', label: 'ERP', icon: 'inventory_2', path: '/operacao/erp' },
  { id: 'crm', label: 'CRM', icon: 'insights', path: '/operacao/crm' },
  { id: 'multiloja', label: 'Multi-loja', icon: 'store', path: '/operacao/multiloja' },
  {
    id: 'planejamento',
    label: 'Planejamento',
    icon: 'calendar_month',
    path: '/planejamento/metas',
  },
  { id: 'configuracoes', label: 'Config', icon: 'tune', path: '/operacao/configuracoes' },
  { id: 'alertas', label: 'Alertas', icon: 'warning', path: '/operacao/alertas' },
  { id: 'feedback', label: 'Suporte', icon: 'chat_bubble', path: '/suporte' },
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
