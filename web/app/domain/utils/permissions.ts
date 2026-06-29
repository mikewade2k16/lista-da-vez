const ROLE_ALIAS = {
  admin: 'platform_admin',
}

const ROLE_LABELS = {
  consultant: 'Consultor',
  store_terminal: 'Acesso da loja',
  manager: 'Gerente',
  marketing: 'Marketing',
  director: 'Diretoria',
  owner: 'Proprietario',
  platform_admin: 'Admin da plataforma',
}

// Label amigavel do papel para exibicao (perfil, cabecalhos). normalizeAppRole e
// hoisted (declaracao de funcao abaixo), entao pode ser referenciada aqui.
export function getRoleLabel(role) {
  const normalized = normalizeAppRole(role)
  return ROLE_LABELS[normalized] || ROLE_LABELS[role] || String(role || '').trim() || 'Sem papel'
}

export const WORKSPACE_ACCESS_DEFINITIONS = [
  {
    id: 'operacao',
    label: 'Operacao',
    description: 'Fila, snapshot e comandos operacionais.',
    viewPermission: 'workspace.operacao.view',
    editPermission: 'workspace.operacao.edit',
  },
  {
    id: 'consultor',
    label: 'Consultor',
    description: 'Painel individual do consultor.',
    viewPermission: 'workspace.consultor.view',
    editPermission: '',
  },
  {
    id: 'tasks',
    label: 'Tasks',
    description: 'Quadros e tarefas do modulo notion-like.',
    viewPermission: '',
    editPermission: '',
  },
  {
    id: 'automation',
    label: 'Automacao',
    description: 'Assistente de WhatsApp/IA (n8n + WAHA). Conectar numero e ligar/desligar.',
    viewPermission: '',
    editPermission: '',
  },
  {
    id: 'meta_ads',
    label: 'Meta Ads',
    description:
      'Conexao, relatorios e gestao de campanhas de trafego pago da Meta (Facebook/Instagram).',
    viewPermission: 'meta_ads.view',
    editPermission: 'meta_ads.manage',
  },
  {
    id: 'ranking',
    label: 'Ranking',
    description: 'Leitura de performance comercial.',
    viewPermission: 'workspace.ranking.view',
    editPermission: '',
  },
  {
    id: 'dados',
    label: 'Dados',
    description: 'Leitura operacional detalhada.',
    viewPermission: 'workspace.dados.view',
    editPermission: '',
  },
  {
    id: 'inteligencia',
    label: 'Inteligencia',
    description: 'Diagnostico e leitura automatica.',
    viewPermission: 'workspace.inteligencia.view',
    editPermission: '',
  },
  {
    id: 'relatorios',
    label: 'Relatorios',
    description: 'Relatorios consolidados e filtros analiticos.',
    viewPermission: 'workspace.relatorios.view',
    editPermission: '',
  },
  {
    id: 'campanhas',
    label: 'Campanhas',
    description: 'Regras comerciais e cadastro de campanhas.',
    viewPermission: 'workspace.campanhas.view',
    editPermission: 'workspace.campanhas.edit',
  },
  {
    id: 'site',
    label: 'Site',
    description: 'Produtos publicados, leads captados e configuracoes do site.',
    viewPermission: '',
    editPermission: '',
  },
  {
    id: 'site_produtos_web',
    label: 'Produtos do Site',
    description: 'Superficie canonica de produtos do site dentro do menu Site.',
    viewPermission: '',
    editPermission: '',
  },
  {
    id: 'site_leads_web',
    label: 'Leads do Site',
    description: 'Superficie canonica de leads do site dentro do menu Site.',
    viewPermission: '',
    editPermission: '',
  },
  {
    id: 'site_tracking_web',
    label: 'Tracking do Site',
    description: 'Listagem administrativa dos eventos brutos de tracking do site.',
    viewPermission: '',
    editPermission: '',
  },
  {
    id: 'site_bio_web',
    label: 'Bio',
    description: 'Paginas de link-in-bio servidas pelo front bio publico.',
    viewPermission: '',
    editPermission: '',
  },
  {
    id: 'cardapio_web',
    label: 'Presence',
    description:
      'Presence — gestao do site (visual, cardapio, pedidos, dominios) de cada estabelecimento.',
    viewPermission: 'cardapio.view',
    editPermission: 'cardapio.manage',
  },
  {
    id: 'clientes',
    label: 'Clientes',
    description: 'Clientes, agencias e status do grupo atendido.',
    viewPermission: 'workspace.clientes.view',
    editPermission: 'workspace.clientes.edit',
  },
  {
    id: 'clientes_web',
    label: 'Clientes Web',
    description:
      'Pagina administrativa de clientes trazida do web-reference para validacao visual.',
    viewPermission: '',
    editPermission: '',
  },
  {
    id: 'erp',
    label: 'ERP',
    description: 'Sync ERP/FTP, status dos lotes e busca de produtos consolidada.',
    viewPermission: 'workspace.erp.view',
    editPermission: 'workspace.erp.edit',
  },
  {
    id: 'bi',
    label: 'BI',
    description: 'Proxy e diagnostico da API Perola BI.',
    viewPermission: '',
    editPermission: '',
  },
  {
    id: 'crm',
    label: 'CRM',
    description: 'Painel comercial cruzando ERP com metas cadastradas no sistema.',
    viewPermission: 'workspace.erp.view',
    editPermission: 'workspace.erp.edit',
  },
  {
    id: 'multiloja',
    label: 'Multi-loja',
    description: 'Consolidado e administracao de lojas.',
    viewPermission: 'workspace.multiloja.view',
    editPermission: 'workspace.multiloja.edit',
  },
  {
    id: 'usuarios',
    label: 'Usuarios',
    description: 'Usuarios, overrides e matriz de acesso.',
    viewPermission: 'workspace.usuarios.view',
    editPermission: 'workspace.usuarios.edit',
  },
  {
    id: 'usuarios_admin',
    label: 'Usuarios Admin',
    description: 'Visao cross-account de todos os users em core.users (so platform_admin).',
    viewPermission: 'workspace.usuarios_admin.view',
    editPermission: 'workspace.usuarios_admin.edit',
  },
  {
    id: 'organizations_admin',
    label: 'Organizations',
    description:
      'Gerencia agencias (core.organizations) e vinculos com accounts (so platform_admin).',
    viewPermission: 'workspace.organizations_admin.view',
    editPermission: 'workspace.organizations_admin.edit',
  },
  {
    id: 'manage',
    label: 'Manage',
    description: 'Agrupa rotas administrativas internas do painel.',
    viewPermission: 'workspace.manage.view',
    editPermission: '',
  },
  {
    id: 'configuracoes',
    label: 'Configuracoes',
    description: 'Templates, catalogos e parametros operacionais.',
    viewPermission: 'workspace.configuracoes.view',
    editPermission: 'workspace.configuracoes.edit',
  },
  {
    id: 'menu_layout',
    label: 'Menu',
    description: 'Config global do menu: posicao de cada item entre header e sidebar.',
    viewPermission: '',
    editPermission: '',
  },
  {
    id: 'themes',
    label: 'Temas',
    description: 'Theme Studio centralizado do tenant.',
    viewPermission: 'workspace.themes.view',
    editPermission: '',
  },
  {
    id: 'alertas',
    label: 'Alertas',
    description: 'Incidentes operacionais em realtime, acknowledge e regras do modulo.',
    viewPermission: 'workspace.alertas.view',
    editPermission: 'workspace.alertas.edit',
  },
  {
    id: 'feedback',
    label: 'Feedback',
    description: 'Sugestoes, duvidas e problemas dos usuarios.',
    viewPermission: 'workspace.feedback.view',
    editPermission: 'workspace.feedback.edit',
  },
  {
    id: 'tools',
    label: 'Tools',
    description: 'Ferramentas auxiliares internas, como editor e utilitarios beta.',
    viewPermission: 'workspace.tools.view',
    editPermission: '',
  },
  {
    id: 'banco',
    label: 'Banco',
    description:
      'Estrutura do banco de dados — tabelas, campos, relacionamentos e status de migracao.',
    viewPermission: '',
    editPermission: '',
  },
  {
    id: 'roadmap',
    label: 'Roadmap',
    description: 'Acompanhamento das fases da reestruturacao multi-tenant.',
    viewPermission: '',
    editPermission: '',
  },
  {
    id: 'performance',
    label: 'Performance',
    description:
      'Resultados da auditoria de performance de navegacao (T1/T2/T3 por rota, in-app e cold).',
    viewPermission: '',
    editPermission: '',
  },
]

export const ADVANCED_ACCESS_DEFINITIONS = [
  {
    key: 'users.password.manage',
    label: 'Resetar senha administrativa',
    description: 'Permite redefinir senha manual pelo painel.',
  },
  {
    key: 'access.role_defaults.manage',
    label: 'Editar padrao por perfil',
    description: 'Permite editar a matriz padrao de acesso por papel.',
  },
  {
    key: 'alerts.rules.manage',
    label: 'Editar regras de alertas',
    description: 'Permite alterar thresholds e canais internos do modulo de alertas.',
  },
  {
    key: 'alerts.actions.manage',
    label: 'Executar acoes de alertas',
    description: 'Permite acknowledge e resolucao manual de alertas operacionais.',
  },
]

const ROLE_WORKSPACES = {
  platform_admin: [
    'operacao',
    'consultor',
    'tasks',
    'ranking',
    'dados',
    'inteligencia',
    'relatorios',
    'campanhas',
    'site',
    'site_produtos_web',
    'site_leads_web',
    'site_tracking_web',
    'site_bio_web',
    'cardapio_web',
    'clientes',
    'clientes_web',
    'erp',
    'bi',
    'crm',
    'multiloja',
    'usuarios',
    'usuarios_admin',
    'organizations_admin',
    'manage',
    'configuracoes',
    'menu_layout',
    'themes',
    'alertas',
    'feedback',
    'tools',
    'banco',
    'roadmap',
    'automation',
    'meta_ads',
    'performance',
  ],
  owner: [
    'operacao',
    'consultor',
    'tasks',
    'ranking',
    'dados',
    'inteligencia',
    'relatorios',
    'campanhas',
    'site',
    'site_produtos_web',
    'site_leads_web',
    'site_tracking_web',
    'site_bio_web',
    'cardapio_web',
    'clientes',
    'erp',
    'bi',
    'crm',
    'multiloja',
    'usuarios',
    'manage',
    'configuracoes',
    'themes',
    'alertas',
    'feedback',
    'tools',
    'roadmap',
  ],
  marketing: [
    'operacao',
    'campanhas',
    'site',
    'site_produtos_web',
    'site_leads_web',
    'site_tracking_web',
    'site_bio_web',
    'cardapio_web',
    'erp',
    'bi',
    'crm',
    'multiloja',
  ],
  director: [
    'operacao',
    'site',
    'site_produtos_web',
    'site_leads_web',
    'site_tracking_web',
    'site_bio_web',
    'cardapio_web',
    'erp',
    'bi',
    'crm',
    'multiloja',
    'configuracoes',
  ],
  manager: [
    'operacao',
    'site',
    'site_produtos_web',
    'site_leads_web',
    'site_tracking_web',
    'site_bio_web',
    'cardapio_web',
    'erp',
    'bi',
    'crm',
    'multiloja',
    'alertas',
    'feedback',
  ],
  store_terminal: [
    'operacao',
    'consultor',
    'ranking',
    'dados',
    'inteligencia',
    'relatorios',
    'alertas',
  ],
  consultant: ['operacao'],
}

const SUPERUSER_ROLES = new Set(['platform_admin'])

export function normalizeAppRole(role) {
  const normalized = String(role || '').trim()
  return ROLE_ALIAS[normalized] || normalized || 'consultant'
}

export function normalizePermissionKeys(permissionKeys = []) {
  return Array.isArray(permissionKeys)
    ? permissionKeys.map((permissionKey) => String(permissionKey || '').trim()).filter(Boolean)
    : []
}

export function hasPermission(permissionKeys, permissionKey) {
  const normalizedPermission = String(permissionKey || '').trim()
  if (!normalizedPermission) {
    return false
  }

  return normalizePermissionKeys(permissionKeys).includes(normalizedPermission)
}

function hasAnyOperationAccessPermission(permissionKeys = []) {
  return (
    hasPermission(permissionKeys, 'workspace.operacao.view') ||
    hasPermission(permissionKeys, 'workspace.operacao.edit') ||
    hasPermission(permissionKeys, 'queue.dashboard.read') ||
    hasPermission(permissionKeys, 'queue.operations.manage')
  )
}

function hasAnyAlertAccessPermission(permissionKeys = []) {
  return (
    hasPermission(permissionKeys, 'workspace.alertas.view') ||
    hasPermission(permissionKeys, 'workspace.alertas.edit') ||
    hasPermission(permissionKeys, 'alerts.actions.manage') ||
    hasPermission(permissionKeys, 'alerts.rules.manage') ||
    hasPermission(permissionKeys, 'queue.alerts.manage')
  )
}

function hasAnyReportsAccessPermission(permissionKeys = []) {
  return (
    hasPermission(permissionKeys, 'workspace.relatorios.view') ||
    hasPermission(permissionKeys, 'queue.reports.read')
  )
}

function hasWorkspaceAccessAlias(
  workspaceId,
  permissionKeys = [],
  roleDefaults = new Set(),
  normalizedRole = '',
) {
  switch (String(workspaceId || '').trim()) {
    case 'operacao':
      return hasAnyOperationAccessPermission(permissionKeys)
    case 'cardapio_web':
      // owner da conta sempre ve o cardapio (espelha o gate do back: platform_admin
      // + owner + agency_owner + cardapio.view/manage). Demais papeis caem no
      // viewPermission ('cardapio.view') resolvido por permissao logo abaixo.
      return normalizedRole === 'owner'
    case 'consultor':
      return (
        hasPermission(permissionKeys, 'workspace.consultor.view') ||
        (roleDefaults.has('consultor') && hasPermission(permissionKeys, 'queue.consultants.manage'))
      )
    case 'ranking':
    case 'dados':
    case 'inteligencia':
      return hasPermission(permissionKeys, 'queue.analytics.read')
    case 'relatorios':
      return hasAnyReportsAccessPermission(permissionKeys)
    case 'multiloja':
      return (
        hasPermission(permissionKeys, 'workspace.multiloja.view') ||
        hasPermission(permissionKeys, 'workspace.multiloja.edit') ||
        roleDefaults.has('multiloja')
      )
    case 'configuracoes':
      return (
        hasPermission(permissionKeys, 'workspace.configuracoes.view') ||
        hasPermission(permissionKeys, 'workspace.configuracoes.edit') ||
        (roleDefaults.has('configuracoes') &&
          (hasPermission(permissionKeys, 'queue.settings.manage') ||
            hasPermission(permissionKeys, 'workspace.operacao.view')))
      )
    case 'alertas':
      return hasAnyAlertAccessPermission(permissionKeys)
    case 'feedback':
      return (
        hasPermission(permissionKeys, 'workspace.feedback.view') ||
        hasPermission(permissionKeys, 'workspace.feedback.edit') ||
        hasPermission(permissionKeys, 'queue.feedback.read')
      )
    default:
      return false
  }
}

export function getWorkspaceAccessDefinition(workspaceId) {
  return (
    WORKSPACE_ACCESS_DEFINITIONS.find(
      (workspace) => workspace.id === String(workspaceId || '').trim(),
    ) || null
  )
}

export function getWorkspaceAccessOptions(workspaceDefinition, { includeInherit = false } = {}) {
  const options = []

  if (includeInherit) {
    options.push({ value: 'inherit', label: 'Herdar padrao' })
  }

  options.push({ value: 'none', label: 'Sem acesso' })
  options.push({ value: 'view', label: 'Somente ver' })

  if (String(workspaceDefinition?.editPermission || '').trim()) {
    options.push({ value: 'edit', label: 'Ver e editar' })
  }

  return options
}

export function readWorkspaceAccessState(
  workspaceDefinition,
  permissionKeys,
  fallbackState = 'none',
) {
  const viewPermission = String(workspaceDefinition?.viewPermission || '').trim()
  const editPermission = String(workspaceDefinition?.editPermission || '').trim()
  if (!viewPermission) {
    return fallbackState
  }

  if (!hasPermission(permissionKeys, viewPermission)) {
    return fallbackState
  }

  if (editPermission && hasPermission(permissionKeys, editPermission)) {
    return 'edit'
  }

  return 'view'
}

export function writeWorkspaceAccessState(workspaceDefinition, permissionKeys, nextState) {
  const viewPermission = String(workspaceDefinition?.viewPermission || '').trim()
  const editPermission = String(workspaceDefinition?.editPermission || '').trim()
  const nextPermissions = normalizePermissionKeys(permissionKeys).filter(
    (permissionKey) => permissionKey !== viewPermission && permissionKey !== editPermission,
  )

  switch (String(nextState || '').trim()) {
    case 'edit':
      if (viewPermission) {
        nextPermissions.push(viewPermission)
      }
      if (editPermission) {
        nextPermissions.push(editPermission)
      }
      break
    case 'view':
      if (viewPermission) {
        nextPermissions.push(viewPermission)
      }
      break
    case 'none':
    case 'inherit':
    default:
      break
  }

  return normalizePermissionKeys(nextPermissions)
}

export function getAllowedWorkspaces(role, permissionKeys = [], permissionsResolved = false) {
  const normalizedRole = normalizeAppRole(role)
  const roleDefaults = new Set(ROLE_WORKSPACES[normalizedRole] || ROLE_WORKSPACES.consultant)

  if (SUPERUSER_ROLES.has(normalizedRole)) {
    return [...roleDefaults]
  }

  if (!permissionsResolved) {
    return [...roleDefaults]
  }

  // Com permissoes resolvidas via banco:
  // - workspace COM viewPermission: exige a chave no token (controle fino por usuario)
  // - workspace SEM viewPermission: segue ROLE_WORKSPACES sem necessidade de migration
  return WORKSPACE_ACCESS_DEFINITIONS.filter((workspace) => {
    const viewPermission = String(workspace.viewPermission || '').trim()
    if (!viewPermission) {
      return roleDefaults.has(workspace.id)
    }
    if (hasWorkspaceAccessAlias(workspace.id, permissionKeys, roleDefaults, normalizedRole)) {
      return true
    }
    return hasPermission(permissionKeys, viewPermission)
  }).map((workspace) => workspace.id)
}

export function canManageSettings(role, permissionKeys = [], permissionsResolved = false) {
  const normalized = normalizeAppRole(role)
  if (SUPERUSER_ROLES.has(normalized)) {
    return true
  }

  if (permissionsResolved) {
    return (
      hasPermission(permissionKeys, 'workspace.configuracoes.edit') ||
      ((normalized === 'platform_admin' || normalized === 'owner') &&
        hasPermission(permissionKeys, 'queue.settings.manage'))
    )
  }

  return normalized === 'platform_admin' || normalized === 'owner'
}

export function canManageCrmCommercialPolicy(
  role,
  _permissionKeys = [],
  _permissionsResolved = false,
) {
  const normalized = normalizeAppRole(role)
  return normalized === 'platform_admin' || normalized === 'director'
}

export function canManageConsultants(role, permissionKeys = [], permissionsResolved = false) {
  const normalized = normalizeAppRole(role)
  if (SUPERUSER_ROLES.has(normalized)) {
    return true
  }

  if (permissionsResolved) {
    return (
      hasPermission(permissionKeys, 'workspace.configuracoes.edit') ||
      ((normalized === 'platform_admin' || normalized === 'owner') &&
        hasPermission(permissionKeys, 'queue.consultants.manage'))
    )
  }

  return normalized === 'platform_admin' || normalized === 'owner'
}

export function canViewConsultants(role, permissionKeys = [], permissionsResolved = false) {
  const normalized = normalizeAppRole(role)
  if (SUPERUSER_ROLES.has(normalized)) {
    return true
  }

  if (permissionsResolved) {
    return (
      hasPermission(permissionKeys, 'workspace.consultor.view') ||
      hasPermission(permissionKeys, 'workspace.configuracoes.view') ||
      hasPermission(permissionKeys, 'queue.consultants.manage')
    )
  }

  return (
    normalized === 'platform_admin' || normalized === 'owner' || normalized === 'store_terminal'
  )
}

export function canViewAlerts(role, permissionKeys = [], permissionsResolved = false) {
  const normalized = normalizeAppRole(role)
  if (SUPERUSER_ROLES.has(normalized)) {
    return true
  }

  if (permissionsResolved) {
    return hasAnyAlertAccessPermission(permissionKeys)
  }

  return (
    normalized === 'platform_admin' ||
    normalized === 'owner' ||
    normalized === 'manager' ||
    normalized === 'store_terminal'
  )
}

export function canAccessReports(role, permissionKeys = [], permissionsResolved = false) {
  const normalized = normalizeAppRole(role)
  if (SUPERUSER_ROLES.has(normalized)) {
    return true
  }

  if (permissionsResolved) {
    return hasAnyReportsAccessPermission(permissionKeys)
  }

  return (
    normalized === 'platform_admin' ||
    normalized === 'owner' ||
    normalized === 'director' ||
    normalized === 'marketing' ||
    normalized === 'manager' ||
    normalized === 'store_terminal'
  )
}

export function canMutateOperations(role, permissionKeys = [], permissionsResolved = false) {
  const normalized = normalizeAppRole(role)
  if (SUPERUSER_ROLES.has(normalized)) {
    return true
  }

  if (permissionsResolved) {
    return hasPermission(permissionKeys, 'workspace.operacao.edit')
  }

  return (
    normalized === 'platform_admin' ||
    normalized === 'owner' ||
    normalized === 'manager' ||
    normalized === 'consultant' ||
    normalized === 'store_terminal'
  )
}

export function canManageCampaigns(role, permissionKeys = [], permissionsResolved = false) {
  const normalized = normalizeAppRole(role)
  if (SUPERUSER_ROLES.has(normalized)) {
    return true
  }

  if (permissionsResolved) {
    return hasPermission(permissionKeys, 'workspace.campanhas.edit')
  }

  return normalized === 'platform_admin' || normalized === 'owner' || normalized === 'marketing'
}

export function canManageStores(role, permissionKeys = [], permissionsResolved = false) {
  const normalized = normalizeAppRole(role)
  if (SUPERUSER_ROLES.has(normalized)) {
    return true
  }

  if (permissionsResolved) {
    return hasPermission(permissionKeys, 'workspace.multiloja.edit')
  }

  return normalized === 'platform_admin' || normalized === 'owner'
}

export function canManageGoalTargets(role, permissionKeys = [], permissionsResolved = false) {
  const normalized = normalizeAppRole(role)
  if (SUPERUSER_ROLES.has(normalized)) {
    return true
  }

  if (permissionsResolved && hasPermission(permissionKeys, 'workspace.multiloja.edit')) {
    return true
  }

  return (
    normalized === 'platform_admin' ||
    normalized === 'owner' ||
    normalized === 'director' ||
    normalized === 'manager'
  )
}

export function canAccessClients(role, permissionKeys = [], permissionsResolved = false) {
  const normalized = normalizeAppRole(role)

  if (SUPERUSER_ROLES.has(normalized)) {
    return true
  }

  if (permissionsResolved) {
    return (
      hasPermission(permissionKeys, 'workspace.clientes.view') ||
      normalized === 'platform_admin' ||
      normalized === 'owner'
    )
  }

  return normalized === 'platform_admin' || normalized === 'owner'
}

export function canManageTenants(role, permissionKeys = [], permissionsResolved = false) {
  const normalized = normalizeAppRole(role)

  if (SUPERUSER_ROLES.has(normalized)) {
    return true
  }

  if (permissionsResolved) {
    return (
      hasPermission(permissionKeys, 'workspace.clientes.edit') ||
      normalized === 'platform_admin' ||
      normalized === 'owner'
    )
  }

  return normalized === 'platform_admin' || normalized === 'owner'
}

export function canManageUsers(role, permissionKeys = [], permissionsResolved = false) {
  const normalized = normalizeAppRole(role)
  if (SUPERUSER_ROLES.has(normalized)) {
    return true
  }

  if (permissionsResolved) {
    return hasPermission(permissionKeys, 'workspace.usuarios.edit')
  }

  return normalized === 'platform_admin' || normalized === 'owner'
}

export function canManageUserPasswords(role, permissionKeys = [], permissionsResolved = false) {
  const normalized = normalizeAppRole(role)
  if (SUPERUSER_ROLES.has(normalized)) {
    return true
  }

  if (permissionsResolved) {
    return hasPermission(permissionKeys, 'users.password.manage')
  }

  return normalized === 'platform_admin'
}

export function canManageRoleDefaults(role, permissionKeys = [], permissionsResolved = false) {
  const normalized = normalizeAppRole(role)
  if (SUPERUSER_ROLES.has(normalized)) {
    return true
  }

  if (permissionsResolved) {
    return hasPermission(permissionKeys, 'access.role_defaults.manage')
  }

  return normalized === 'platform_admin'
}

export function canUseAllStoresScope(storeIds = []) {
  const normalizedStoreIds = Array.isArray(storeIds)
    ? storeIds.map((storeId) => String(storeId || '').trim()).filter(Boolean)
    : []

  return new Set(normalizedStoreIds).size > 1
}

export function canAccessMultiStore(role, permissionKeys = [], permissionsResolved = false) {
  const normalized = normalizeAppRole(role)
  if (SUPERUSER_ROLES.has(normalized)) {
    return true
  }

  if (permissionsResolved && hasPermission(permissionKeys, 'workspace.multiloja.view')) {
    return true
  }

  return (
    normalized === 'platform_admin' ||
    normalized === 'owner' ||
    normalized === 'director' ||
    normalized === 'marketing' ||
    normalized === 'manager'
  )
}

export function filterPerolaERPWorkspaces(workspaces = []) {
  return Array.isArray(workspaces)
    ? workspaces.map((workspace) => String(workspace || '').trim()).filter(Boolean)
    : []
}
