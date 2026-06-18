import { computed, ref, watch } from 'vue'

import { useNavStore } from '~/stores/nav'
import type { NavItem, NavSection } from '~/stores/nav'
import { createEmptyLayout, useMenuLayoutStore } from '~/stores/menuLayout'
import type { MenuLayout, MenuPlacement } from '~/stores/menuLayout'

export interface MenuLayoutEditorItem {
  id: string
  label: string
  icon: string
  hidden: boolean
  children: MenuLayoutEditorItem[]
}

export interface MenuLayoutEditorSection {
  id: string
  label: string
  items: MenuLayoutEditorItem[]
}

// Ids dos nos sugeridos para o header no layout enxuto. Itens fora dessa lista
// vao para a sidebar. Grupos (com filhos) ficam na sidebar para nao estourar o
// header; os filhos relevantes (fila/tasks/crm) estao na raiz da secao.
const LEAN_HEADER_IDS = new Set(['fila', 'tasks', 'tracking', 'crm', 'automation'])

function toEditorItem(item: NavItem): MenuLayoutEditorItem {
  return {
    id: item.id,
    label: item.label,
    icon: item.icon,
    hidden: Boolean(item.hidden),
    children: Array.isArray(item.children) ? item.children.map(toEditorItem) : [],
  }
}

/**
 * Controla a edicao da config global do menu: monta a arvore canonica
 * (declarada em nav.config), mantem um draft local de placements/ordens e
 * deriva o layout final para preview e save.
 *
 * @see ~/stores/menuLayout
 */
export function useMenuLayoutEditor() {
  const navStore = useNavStore()
  const menuLayoutStore = useMenuLayoutStore()

  // Arvore canonica: TODAS as secoes/itens declarados (inclusive `hidden:true`
  // do dev), pois o admin configura o conjunto completo, nao a visao filtrada.
  const sections = computed<MenuLayoutEditorSection[]>(() =>
    (navStore.sections as NavSection[]).map((section) => ({
      id: section.id,
      label: section.label,
      items: (section.items || []).map(toEditorItem),
    })),
  )

  // Ordem dos ids de secao no draft (reordenavel por drag).
  const sectionOrder = ref<string[]>([])
  // Placement por id de item (inclui filhos).
  const placements = ref<Record<string, MenuPlacement>>({})
  // Ordem dos ids de item dentro de cada secao/grupo (reordenavel por drag).
  const itemOrder = ref<Record<string, string[]>>({})

  function collectChildOrder(item: MenuLayoutEditorItem, target: Record<string, string[]>) {
    if (item.children.length) {
      target[item.id] = item.children.map((child) => child.id)
      for (const child of item.children) collectChildOrder(child, target)
    }
  }

  function applyItemPlacement(item: MenuLayoutEditorItem, layout: MenuLayout) {
    const config = layout.items[item.id]
    placements.value[item.id] = config?.placement || 'both'
    for (const child of item.children) applyItemPlacement(child, layout)
  }

  // Inicializa o draft a partir do layout salvo (ou do default). Reaplicado
  // quando as secoes carregam ou quando o layout remoto chega.
  function resetDraft() {
    const layout = menuLayoutStore.layout
    const nextOrder: Record<string, string[]> = {}
    const declaredSectionIds = sections.value.map((section) => section.id)

    for (const section of sections.value) {
      nextOrder[section.id] = section.items.map((item) => item.id)
      for (const item of section.items) collectChildOrder(item, nextOrder)
      for (const item of section.items) applyItemPlacement(item, layout)
    }

    // Ordena ids de secao pela config salva; secoes sem override seguem a ordem
    // declarada (anexadas ao fim, ordem estavel).
    const savedSectionOrder = [...layout.sections]
      .sort((a, b) => a.order - b.order)
      .map((entry) => entry.id)
      .filter((id) => declaredSectionIds.includes(id))
    const remainingSections = declaredSectionIds.filter((id) => !savedSectionOrder.includes(id))
    sectionOrder.value = [...savedSectionOrder, ...remainingSections]

    // Ordena itens de cada secao/grupo pela ordem salva.
    for (const [scopeId, ids] of Object.entries(nextOrder)) {
      const ordered = [...ids].sort(
        (a, b) => menuLayoutStore.orderOf(a) - menuLayoutStore.orderOf(b),
      )
      nextOrder[scopeId] = ordered
    }
    itemOrder.value = nextOrder
  }

  watch(
    () => [sections.value, menuLayoutStore.layout] as const,
    () => resetDraft(),
    { immediate: true, deep: true },
  )

  function placementFor(id: string): MenuPlacement {
    return placements.value[id] || 'both'
  }

  function setPlacement(id: string, placement: MenuPlacement) {
    placements.value = { ...placements.value, [id]: placement }
  }

  // Reordena ids dentro de um escopo (secao ou grupo), movendo `sourceId` para
  // a posicao de `targetId`.
  function reorderItems(scopeId: string, sourceId: string, targetId: string) {
    if (sourceId === targetId) return
    const current = [...(itemOrder.value[scopeId] || [])]
    const sourceIdx = current.indexOf(sourceId)
    const targetIdx = current.indexOf(targetId)
    if (sourceIdx < 0 || targetIdx < 0) return
    current.splice(sourceIdx, 1)
    current.splice(targetIdx, 0, sourceId)
    itemOrder.value = { ...itemOrder.value, [scopeId]: current }
  }

  function reorderSections(sourceId: string, targetId: string) {
    if (sourceId === targetId) return
    const current = [...sectionOrder.value]
    const sourceIdx = current.indexOf(sourceId)
    const targetIdx = current.indexOf(targetId)
    if (sourceIdx < 0 || targetIdx < 0) return
    current.splice(sourceIdx, 1)
    current.splice(targetIdx, 0, sourceId)
    sectionOrder.value = current
  }

  // Secoes ordenadas pelo draft, com itens ordenados pelo draft (recursivo).
  const orderedSections = computed<MenuLayoutEditorSection[]>(() => {
    const byId = new Map(sections.value.map((section) => [section.id, section]))
    const orderItems = (scopeId: string, items: MenuLayoutEditorItem[]): MenuLayoutEditorItem[] => {
      const order = itemOrder.value[scopeId] || items.map((item) => item.id)
      const itemById = new Map(items.map((item) => [item.id, item]))
      const ordered = order
        .map((id) => itemById.get(id))
        .filter((item): item is MenuLayoutEditorItem => Boolean(item))
      for (const item of items) if (!order.includes(item.id)) ordered.push(item)
      return ordered.map((item) =>
        item.children.length ? { ...item, children: orderItems(item.id, item.children) } : item,
      )
    }
    return sectionOrder.value
      .map((id) => byId.get(id))
      .filter((section): section is MenuLayoutEditorSection => Boolean(section))
      .map((section) => ({ ...section, items: orderItems(section.id, section.items) }))
  })

  // Layout final derivado do draft, pronto para preview e save.
  const draftLayout = computed<MenuLayout>(() => {
    const layout = createEmptyLayout()
    orderedSections.value.forEach((section, index) => {
      layout.sections.push({ id: section.id, order: index })
    })
    const writeItem = (item: MenuLayoutEditorItem, order: number) => {
      layout.items[item.id] = { placement: placementFor(item.id), order }
      item.children.forEach((child, childIndex) => writeItem(child, childIndex))
    }
    for (const section of orderedSections.value) {
      section.items.forEach((item, index) => writeItem(item, index))
    }
    return layout
  })

  // Pre-preenche um split enxuto no DRAFT (nao salva): itens fila/tasks/crm no
  // header, resto na sidebar. O admin revisa e clica em salvar.
  function suggestLeanLayout() {
    const nextPlacements: Record<string, MenuPlacement> = {}
    const visit = (item: MenuLayoutEditorItem, isTopLevel: boolean) => {
      if (item.children.length) {
        // Grupos vao para a sidebar (header so com itens diretos curados).
        nextPlacements[item.id] = 'sidebar'
        for (const child of item.children) visit(child, false)
        return
      }
      if (isTopLevel && LEAN_HEADER_IDS.has(item.id)) {
        nextPlacements[item.id] = 'header'
      } else {
        nextPlacements[item.id] = 'sidebar'
      }
    }
    for (const section of sections.value) {
      for (const item of section.items) visit(item, true)
    }
    placements.value = nextPlacements
  }

  const saving = computed(() => menuLayoutStore.saving)

  async function save() {
    await menuLayoutStore.save(draftLayout.value)
  }

  return {
    sections,
    orderedSections,
    sectionOrder,
    draftLayout,
    saving,
    placementFor,
    setPlacement,
    reorderItems,
    reorderSections,
    suggestLeanLayout,
    resetDraft,
    save,
  }
}
