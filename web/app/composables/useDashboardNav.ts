import { computed } from 'vue'
import type { ComputedRef } from 'vue'
import {
  AlertTriangle,
  BarChart3,
  Blocks,
  Boxes,
  BrainCircuit,
  Building2,
  CalendarDays,
  ClipboardList,
  Code2,
  Database,
  FileBarChart,
  FileText,
  FormInput,
  Gauge,
  Landmark,
  LayoutPanelLeft,
  Link2,
  ListChecks,
  ListTodo,
  Megaphone,
  MessageCircle,
  MessagesSquare,
  MonitorCog,
  PackageCheck,
  Palette,
  QrCode,
  SearchCheck,
  Settings,
  ShieldCheck,
  Store,
  Users,
  Wrench,
} from 'lucide-vue-next'
import type { NavItem } from '~/stores/nav'
import { useNavStore } from '~/stores/nav'
import { useCoreAccountStore } from '../../layers/core/stores/account'
import { QUEUE_ONLY_WORKSPACE_IDS } from '~/utils/workspaces'

export const NAV_ICON_MAP: Record<string, unknown> = {
  alert: AlertTriangle,
  audit: ShieldCheck,
  boxes: Boxes,
  brain: BrainCircuit,
  building: Building2,
  calendar: CalendarDays,
  chart: BarChart3,
  database: Database,
  feedback: MessageCircle,
  finance: Landmark,
  forms: FormInput,
  indicators: Gauge,
  integration: Blocks,
  link: Link2,
  manage: LayoutPanelLeft,
  megaphone: Megaphone,
  messages: MessagesSquare,
  monitoring: MonitorCog,
  page: FileText,
  palette: Palette,
  qr: QrCode,
  queue: ListChecks,
  ranking: FileBarChart,
  reports: ClipboardList,
  script: Code2,
  settings: Settings,
  site: PackageCheck,
  stores: Store,
  tasks: ListTodo,
  team: Users,
  tools: Wrench,
  tracking: SearchCheck,
  user: Users,
  users: Users,
}

/**
 * Deriva a navegacao visivel do painel a partir do workspace atual, permissoes e store de menu.
 *
 * O composable filtra itens nao permitidos, remove workspaces exclusivos da operacao/queue do menu
 * administrativo e expõe helpers para icones e estados ativos de itens e grupos.
 *
 * @param activeWorkspace Workspace atualmente resolvido pelo shell.
 * @param allowedWorkspaces Lista reativa de workspaces permitidos para a sessao.
 * @returns Secoes visiveis, itens para header e resolvers de estado ativo/icone.
 *
 * @example
 * ```ts
 * const { visibleSections, isItemActive } = useDashboardNav(activeWorkspaceId, allowedWorkspaces)
 * ```
 *
 * @see ~/stores/nav
 * @see ~/utils/workspaces
 */
export function useDashboardNav(
  activeWorkspace: ComputedRef<string>,
  allowedWorkspaces: ComputedRef<readonly unknown[]>,
) {
  const navStore = useNavStore()
  const route = useRoute()
  const accountStore = useCoreAccountStore()

  const allowedWorkspaceSet = computed(() => new Set(allowedWorkspaces.value || []))
  const enabledModulesSet = computed(() => new Set(accountStore.enabledModules))
  const currentPath = computed(() => normalizePath(route.path))

  function normalizePath(path: string) {
    return String(path || '').replace(/\/+$/, '') || '/'
  }

  // Conjunto de todos os paths declarados no menu (itens + filhos). Usado para decidir quando a
  // rota atual corresponde a um item especifico — nesse caso o match por workspaceId nao deve
  // acender outros itens do mesmo workspace (ex.: Tasks e Tracking compartilham workspaceId 'tasks').
  const knownPaths = computed(() => {
    const paths = new Set<string>()
    const collect = (items?: NavItem[]) => {
      for (const item of items || []) {
        if (item.path) paths.add(normalizePath(item.path))
        collect(item.children)
      }
    }
    for (const section of navStore.sections || []) collect(section.items)
    return paths
  })

  const routeMatchesKnownPath = computed(() => {
    const current = currentPath.value
    for (const path of knownPaths.value) {
      if (current === path || current.startsWith(`${path}/`)) return true
    }
    return false
  })

  function isItemAllowed(item: NavItem): boolean {
    if (item.hidden) return false
    // C11: filtro por modulo habilitado. Se item declara moduleId e o modulo
    // nao esta em useCoreAccountStore().enabledModules, item some do menu.
    // Quando enabledModulesSet esta vazio (sem account ativa ainda), nao filtra
    // por modulo — deixa permissoes (role/workspace) decidirem. Evita "menu
    // vazio" durante o hidrate inicial. Modulo `core` nunca e filtrado.
    const moduleId = String(item.moduleId || '').trim()
    if (moduleId && moduleId !== 'core' && enabledModulesSet.value.size > 0) {
      if (!enabledModulesSet.value.has(moduleId)) return false
    }
    const workspaceId = String(item.workspaceId || '').trim()
    if (!workspaceId) return true
    if (!allowedWorkspaceSet.value.has(workspaceId)) return false
    if (QUEUE_ONLY_WORKSPACE_IDS.has(workspaceId)) {
      return workspaceId === 'operacao' && normalizePath(item.path || '') === '/operacao'
    }
    return true
  }

  function filterItem(item: NavItem): NavItem | null {
    if (!isItemAllowed(item)) return null
    if (!Array.isArray(item.children)) return item
    const children = item.children.filter(isItemAllowed)
    if (!children.length) return null
    return { ...item, children }
  }

  function isItemActive(item: NavItem): boolean {
    const itemPath = normalizePath(item.path || '')
    // Path exato/prefixo vence — distingue itens que compartilham workspaceId.
    if (
      item.path &&
      (currentPath.value === itemPath || currentPath.value.startsWith(`${itemPath}/`))
    ) {
      return true
    }
    // Fallback por workspace: so quando a rota atual NAO corresponde a nenhum item do menu
    // (ex.: rotas-alias cujo path difere). Evita acender dois itens do mesmo workspace.
    const workspaceId = String(item.workspaceId || '').trim()
    return Boolean(
      workspaceId && activeWorkspace.value === workspaceId && !routeMatchesKnownPath.value,
    )
  }

  function isGroupActive(item: NavItem): boolean {
    return Array.isArray(item.children) && item.children.some(isItemActive)
  }

  function resolveIcon(icon: string) {
    return NAV_ICON_MAP[icon] || LayoutPanelLeft
  }

  const visibleSections = computed(() =>
    navStore.sections
      .filter((section) => !section.hidden)
      .map((section) => ({
        ...section,
        items: (section.items || []).map(filterItem).filter((i): i is NavItem => i !== null),
      }))
      .filter((section) => section.items.length > 0),
  )

  const headerItems = computed(() =>
    visibleSections.value.flatMap((section) =>
      section.items.map((item) => ({ ...item, sectionLabel: section.label })),
    ),
  )

  return { visibleSections, headerItems, resolveIcon, isItemActive, isGroupActive }
}
