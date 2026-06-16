import { useNavStore } from '~/stores/nav'
import type { NavSection } from '~/stores/nav'

export default defineNuxtPlugin(() => {
  const navStore = useNavStore()

  // Menu vem 100% dos layers declarativos (web/layers/*/nav.config.ts). O antigo
  // fallback estatico (utils/sidebar-nav.ts) foi removido — o nav.config.ts da
  // layer queue ja cobre todas as sections (service, tools, team-site,
  // indicators, manage) e mais.
  const layerConfigs = import.meta.glob('../../layers/*/nav.config.ts', { eager: true }) as Record<
    string,
    { default: { moduleId: string; sections: Omit<NavSection, 'moduleId'>[] } }
  >

  for (const mod of Object.values(layerConfigs)) {
    const config = mod.default
    if (!config?.moduleId || !config.sections?.length) continue
    navStore.register(config.sections.map((s) => ({ ...s, moduleId: config.moduleId })))
  }
})
