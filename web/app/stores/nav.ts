import { defineStore } from 'pinia'
import { ref } from 'vue'

export interface NavItem {
  id: string
  label: string
  icon: string
  path?: string
  workspaceId?: string
  children?: NavItem[]
  hidden?: boolean
  beta?: boolean
  // C11: id do modulo (`core`, `queue`, `tasks`, `crm`, `site`, `roadmap`,
  // `notifications`). Quando preenchido, o item so aparece se o modulo esta
  // habilitado em useCoreAccountStore().enabledModules. Sem moduleId = sempre
  // visivel (itens do `core` ou utilitarios).
  moduleId?: string
  // Itens de admin-global (Manage da plataforma): so aparecem quando a conta
  // ativa do switcher e a agencia (activeAccount.isAgency). Diferente de moduleId
  // (gating por modulo contratado) — agencyOnly nao depende de modulo, e sim de
  // a conta ser o workspace da agencia. Ver useDashboardNav.isItemAllowed.
  agencyOnly?: boolean
}

export interface NavSection {
  id: string
  label: string
  moduleId: string
  items: NavItem[]
  hidden?: boolean
}

export const useNavStore = defineStore('nav', () => {
  const sections = ref<NavSection[]>([])

  function register(incoming: NavSection[]) {
    for (const section of incoming) {
      const idx = sections.value.findIndex((s) => s.id === section.id)
      if (idx >= 0) {
        sections.value[idx] = section
      } else {
        sections.value.push(section)
      }
    }
  }

  function reset() {
    sections.value = []
  }

  return { sections, register, reset }
})
