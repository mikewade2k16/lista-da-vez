import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { useAuthStore } from '~/stores/auth'
import { useUiStore } from '~/stores/ui'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'

// Posicao de cada item do menu na config GLOBAL editavel pelo platform_admin.
// `both` (default) = aparece no header e na sidebar. `hidden` = ocultado pelo
// admin (distinto do flag `hidden:true` do dev no nav.config, que sempre esconde).
export type MenuPlacement = 'header' | 'sidebar' | 'both' | 'hidden'

export interface MenuLayoutSectionConfig {
  id: string
  order: number
}

export interface MenuLayoutItemConfig {
  placement: MenuPlacement
  order: number
}

export interface MenuLayout {
  version: number
  sections: MenuLayoutSectionConfig[]
  items: Record<string, MenuLayoutItemConfig>
}

interface MenuLayoutResponse {
  layout: MenuLayout
  updatedAt: string
  updatedBy: string
}

const DEFAULT_PLACEMENT: MenuPlacement = 'both'

export function createEmptyLayout(): MenuLayout {
  return { version: 1, sections: [], items: {} }
}

function normalizeLayout(raw: unknown): MenuLayout {
  const candidate = (raw || {}) as Partial<MenuLayout>
  const sections = Array.isArray(candidate.sections)
    ? candidate.sections
        .map((section) => ({
          id: String(section?.id || '').trim(),
          order: Number(section?.order ?? 0),
        }))
        .filter((section) => section.id.length > 0)
    : []
  const items: Record<string, MenuLayoutItemConfig> = {}
  if (candidate.items && typeof candidate.items === 'object') {
    for (const [key, value] of Object.entries(candidate.items)) {
      const id = String(key || '').trim()
      if (!id) continue
      const placement = String((value as MenuLayoutItemConfig)?.placement || '').trim()
      items[id] = {
        placement: (['header', 'sidebar', 'both', 'hidden'].includes(placement)
          ? placement
          : DEFAULT_PLACEMENT) as MenuPlacement,
        order: Number((value as MenuLayoutItemConfig)?.order ?? 0),
      }
    }
  }
  return {
    version: Number(candidate.version ?? 1) || 1,
    sections,
    items,
  }
}

/**
 * Store da config GLOBAL do menu (header vs sidebar). NAO e tenant-scoped: as
 * rotas /v1/platform/menu-layout sao da plataforma e nao recebem tenantId na
 * URL. Apenas o platform_admin edita; todos consomem o layout para dividir o
 * header e a sidebar (ver useDashboardNav).
 *
 * @see ~/composables/useDashboardNav
 */
export const useMenuLayoutStore = defineStore('menuLayout', () => {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const ui = useUiStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  const layout = ref<MenuLayout>(createEmptyLayout())
  const loaded = ref(false)
  const loading = ref(false)
  const saving = ref(false)

  const sectionOrderIndex = computed(() => {
    const index = new Map<string, number>()
    for (const section of layout.value.sections) index.set(section.id, section.order)
    return index
  })

  function placementOf(id: string): MenuPlacement {
    const config = layout.value.items[id]
    return config?.placement || DEFAULT_PLACEMENT
  }

  function orderOf(id: string): number {
    const config = layout.value.items[id]
    return Number.isFinite(config?.order) ? Number(config?.order) : Number.MAX_SAFE_INTEGER
  }

  function sectionOrderOf(id: string): number {
    const order = sectionOrderIndex.value.get(id)
    return Number.isFinite(order) ? Number(order) : Number.MAX_SAFE_INTEGER
  }

  // Carrega o layout uma unica vez (client-only, idempotente). Falha de rede ou
  // ausencia do endpoint deixa o layout vazio = default 'both' (comportamento
  // de hoje: tudo aparece nos dois). Nao bloqueia a renderizacao do shell.
  async function load() {
    if (!import.meta.client || loaded.value || loading.value) return
    loading.value = true
    try {
      await auth.ensureSession()
      if (!auth.accessToken) {
        loaded.value = true
        return
      }
      const response = (await apiRequest('/v1/platform/menu-layout')) as MenuLayoutResponse
      layout.value = normalizeLayout(response?.layout)
      loaded.value = true
    } catch (err) {
      // Sem layout salvo ainda (ou backend indisponivel): segue com o default.
      // Marca loaded para nao repetir a tentativa a cada navegacao.
      loaded.value = true
      console.warn('menuLayout.load falhou; usando layout padrao', getApiErrorMessage(err, ''))
    } finally {
      loading.value = false
    }
  }

  // Salva (PATCH) de forma otimista: aplica no estado antes da resposta e mostra
  // toast no fim. Em erro, reverte para o snapshot anterior.
  async function save(next: MenuLayout) {
    const normalizedNext = normalizeLayout(next)
    const previous = layout.value
    layout.value = normalizedNext
    saving.value = true
    try {
      const response = (await apiRequest('/v1/platform/menu-layout', {
        method: 'PATCH',
        body: { layout: normalizedNext },
      })) as MenuLayoutResponse
      layout.value = normalizeLayout(response?.layout)
      loaded.value = true
      ui.success('Layout do menu salvo.')
      return layout.value
    } catch (err) {
      layout.value = previous
      ui.error(getApiErrorMessage(err, 'Nao foi possivel salvar o layout do menu.'))
      throw err
    } finally {
      saving.value = false
    }
  }

  return {
    layout,
    loaded,
    loading,
    saving,
    placementOf,
    orderOf,
    sectionOrderOf,
    load,
    save,
  }
})
