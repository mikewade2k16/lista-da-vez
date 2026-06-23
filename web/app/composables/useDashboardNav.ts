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
import { useMenuLayoutStore } from '~/stores/menuLayout'
import type { MenuPlacement } from '~/stores/menuLayout'
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
  const menuLayoutStore = useMenuLayoutStore()

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
    // Modo super-admin/dev (platformView): revela TUDO, inclusive itens `hidden`
    // (modulos/telas em desenvolvimento nao liberados nem para a conta-agencia).
    // So o platform_admin entra nesse modo (switcher > "Plataforma (dev)").
    if (accountStore.platformView) return true
    if (item.hidden) return false
    // Admin-global (Manage da plataforma): so aparece quando a conta ativa do
    // switcher e a agencia. Em qualquer conta-cliente (view-as) esses itens
    // somem. Espelhado no module-enabled.global.ts (AGENCY_ONLY_PATHS).
    if (item.agencyOnly && !accountStore.activeAccount?.isAgency) return false
    // C11 / view-as: filtro por modulo contratado pela conta ativa do switcher.
    // O switcher e ferramenta do admin para "ver como o cliente" — ao selecionar
    // uma conta, o menu reflete SO os modulos que ela contratou (igual o cliente
    // veria), inclusive para platform_admin.
    //
    // Gate de "conta carregada" (fail-closed): para item de modulo (moduleId
    // nao-core), tres casos:
    //  1. Ha activeAccount resolvida → filtra: esconde se o modulo nao esta
    //     habilitado. Conta com 0 modulos → some tudo de modulo (sobra core/Manage).
    //  2. Sem activeAccount MAS o contexto ja resolveu (accountsLoaded) → fecha:
    //     `return false`. Sem conta ativa (sem membership, /v2/me/accounts vazio
    //     ou erro) nao deve revelar itens de modulo so pelo papel.
    //  3. Sem activeAccount e ainda hidratando (accountsLoaded false) → NAO filtra:
    //     evita flash de menu vazio; permissoes (role/workspace) decidem por ora.
    // Itens sem moduleId ou com `core` nunca sao filtrados (Manage, perfil, etc.).
    const moduleId = String(item.moduleId || '').trim()
    if (moduleId && moduleId !== 'core') {
      if (accountStore.activeAccount) {
        if (!enabledModulesSet.value.has(moduleId)) return false
      } else if (accountStore.accountsLoaded) {
        return false
      }
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

  // Posicao efetiva de um item segundo a config GLOBAL do menu. Sem layout
  // salvo o store devolve 'both' (default) — nesse caso o comportamento e o
  // de hoje: o item aparece no header e na sidebar.
  function effectivePlacement(item: NavItem): MenuPlacement {
    return menuLayoutStore.placementOf(item.id)
  }

  // Ordena itens pela ordem da config global; empate cai na ordem declarada
  // (estavel, pois orderOf devolve MAX_SAFE_INTEGER quando nao ha override).
  function sortByConfiguredOrder(items: NavItem[]): NavItem[] {
    return items
      .map((item, index) => ({ item, index }))
      .sort((a, b) => {
        const delta = menuLayoutStore.orderOf(a.item.id) - menuLayoutStore.orderOf(b.item.id)
        return delta !== 0 ? delta : a.index - b.index
      })
      .map((entry) => entry.item)
  }

  // Mantem itens visiveis na SIDEBAR: placement sidebar/both. Aplica
  // recursivamente aos filhos e ordena pela config global. Grupos sem filho
  // elegivel sao descartados.
  function keepForSidebar(item: NavItem): NavItem | null {
    if (Array.isArray(item.children)) {
      const children = sortByConfiguredOrder(
        item.children.filter((child) => {
          const placement = effectivePlacement(child)
          return placement === 'sidebar' || placement === 'both'
        }),
      )
      if (!children.length) return null
      return { ...item, children }
    }
    const placement = effectivePlacement(item)
    if (placement !== 'sidebar' && placement !== 'both') return null
    return item
  }

  // Mantem itens visiveis no HEADER: placement header/both. Um grupo entra se o
  // proprio grupo for header/both E tiver ao menos um filho elegivel.
  function keepForHeader(item: NavItem): NavItem | null {
    const placement = effectivePlacement(item)
    if (placement !== 'header' && placement !== 'both') return null
    if (Array.isArray(item.children)) {
      const children = sortByConfiguredOrder(
        item.children.filter((child) => {
          const childPlacement = effectivePlacement(child)
          return childPlacement === 'header' || childPlacement === 'both'
        }),
      )
      if (!children.length) return null
      return { ...item, children }
    }
    return item
  }

  // Secoes filtradas por permissao/modulo/agencyOnly/hidden (visao canonica do
  // usuario), antes de aplicar o split header/sidebar.
  const allowedSections = computed(() =>
    navStore.sections
      .filter((section) => !section.hidden)
      .map((section) => ({
        ...section,
        items: (section.items || []).map(filterItem).filter((i): i is NavItem => i !== null),
      }))
      .filter((section) => section.items.length > 0),
  )

  // Ordena as secoes pela config global (sectionOrderOf); empate mantem a ordem
  // de registro.
  function sortSectionsByConfiguredOrder<T extends { id: string }>(sections: T[]): T[] {
    return sections
      .map((section, index) => ({ section, index }))
      .sort((a, b) => {
        const delta =
          menuLayoutStore.sectionOrderOf(a.section.id) -
          menuLayoutStore.sectionOrderOf(b.section.id)
        return delta !== 0 ? delta : a.index - b.index
      })
      .map((entry) => entry.section)
  }

  const visibleSections = computed(() => {
    const sections = allowedSections.value
      .map((section) => {
        const items = sortByConfiguredOrder(
          section.items.map(keepForSidebar).filter((i): i is NavItem => i !== null),
        )
        return { ...section, items }
      })
      .filter((section) => section.items.length > 0)
    return sortSectionsByConfiguredOrder(sections)
  })

  const headerItems = computed(() => {
    const sections = sortSectionsByConfiguredOrder(allowedSections.value)
    return sections.flatMap((section) =>
      sortByConfiguredOrder(
        section.items.map(keepForHeader).filter((i): i is NavItem => i !== null),
      ).map((item) => ({ ...item, sectionLabel: section.label })),
    )
  })

  return { visibleSections, headerItems, resolveIcon, isItemActive, isGroupActive }
}
