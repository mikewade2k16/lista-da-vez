import { SIDEBAR_NAV_SECTIONS } from "~/utils/sidebar-nav";
import { useNavStore } from "~/stores/nav";
import type { NavSection } from "~/stores/nav";

export default defineNuxtPlugin(() => {
  const navStore = useNavStore();

  // Carrega nav.config.ts de todos os layers declarados em web/layers/*/
  const layerConfigs = import.meta.glob("../../layers/*/nav.config.ts", { eager: true }) as Record<
    string,
    { default: { moduleId: string; sections: Omit<NavSection, "moduleId">[] } }
  >;

  for (const mod of Object.values(layerConfigs)) {
    const config = mod.default;
    if (!config?.moduleId || !config.sections?.length) continue;
    navStore.register(
      config.sections.map((s) => ({ ...s, moduleId: config.moduleId }))
    );
  }

  // Fallback legado: injeta sidebar-nav.ts estático como módulo "legacy"
  // Permanece enquanto o layer queue não estiver criado.
  navStore.register(
    SIDEBAR_NAV_SECTIONS.map((s) => ({ ...s, moduleId: "legacy" })) as NavSection[]
  );
});
