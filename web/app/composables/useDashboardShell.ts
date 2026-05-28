import { computed, onMounted, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useAuthStore } from '~/stores/auth'
import { useWorkspaceStore } from '~/stores/workspace'
import { getWorkspaceLabel, getWorkspacePath } from '~/utils/workspaces'

export function useDashboardState() {
  const workspace = useWorkspaceStore()
  const { state } = storeToRefs(workspace)

  return {
    state,
  }
}

/**
 * Orquestra o shell do painel administrativo: sessao, workspace ativo, titulo da pagina e redirects.
 *
 * O fluxo principal garante sessao valida, carrega o estado do workspace, aplica fallback quando a
 * rota pede um workspace nao permitido e mantem o store alinhado ao path atual.
 *
 * @returns Estado do workspace, identificador ativo derivado da rota e helpers para trocar perfil e loja.
 *
 * @example
 * ```ts
 * const { pageLabel, activeWorkspaceId } = useDashboardShell()
 * ```
 *
 * @see ~/stores/workspace
 * @see ~/utils/workspaces
 */
export function useDashboardShell() {
  const route = useRoute()
  const auth = useAuthStore()
  const workspace = useWorkspaceStore()
  const { activeRole, allowedWorkspaces } = storeToRefs(workspace)
  const { state } = useDashboardState()

  const routeWorkspaceId = computed(() => {
    if (Object.prototype.hasOwnProperty.call(route.meta, 'workspaceId')) {
      return String(route.meta.workspaceId || '').trim()
    }

    return null
  })

  const activeWorkspaceId = computed(() =>
    routeWorkspaceId.value === null
      ? String(state.value?.activeWorkspace || 'operacao')
      : routeWorkspaceId.value,
  )
  const pageLabel = computed(() => {
    if (routeWorkspaceId.value === '') {
      return String(route.meta.pageLabel || 'Painel').trim() || 'Painel'
    }

    return getWorkspaceLabel(activeWorkspaceId.value) || 'Painel'
  })

  useHead(() => ({
    title: `${pageLabel.value} | ${state.value?.brandName || 'Fila Atendimento'}`,
  }))

  async function syncWorkspaceState() {
    await auth.ensureSession()

    if (!auth.isAuthenticated) {
      return
    }

    await workspace.ensure()

    if (routeWorkspaceId.value === '') {
      return
    }

    const allowed = allowedWorkspaces.value
    const fallbackWorkspace = allowed[0] || 'operacao'
    const nextWorkspace = allowed.includes(activeWorkspaceId.value)
      ? activeWorkspaceId.value
      : fallbackWorkspace

    if (nextWorkspace !== activeWorkspaceId.value) {
      await navigateTo(getWorkspacePath(nextWorkspace), { replace: true })
      return
    }

    if (state.value.activeWorkspace !== nextWorkspace) {
      await workspace.setWorkspace(nextWorkspace)
    }
  }

  onMounted(async () => {
    await syncWorkspaceState()
  })

  watch([activeWorkspaceId, activeRole, () => auth.isAuthenticated], ([, , isAuthenticated]) => {
    if (import.meta.client) {
      if (!isAuthenticated) {
        return
      }

      void syncWorkspaceState()
    }
  })

  return {
    state,
    activeWorkspaceId,
    allowedWorkspaces,
    pageLabel,
    setActiveProfile(profileId) {
      return workspace.setActiveProfile(profileId)
    },
    setActiveStore(storeId) {
      return workspace.setActiveStore(storeId)
    },
  }
}
