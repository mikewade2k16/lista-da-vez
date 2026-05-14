import { SIDEBAR_NAV_SECTIONS } from "~/utils/sidebar-nav";
import { useNavStore } from "~/stores/nav";
import type { NavSection } from "~/stores/nav";

export default defineNuxtPlugin(() => {
  const navStore = useNavStore();

  // 1. Legado com prioridade baixa — será sobrescrito por qualquer layer que
  //    declare a mesma section id. Removido quando todos os layers estiverem prontos.
  navStore.register(
    SIDEBAR_NAV_SECTIONS.map((s) => ({ ...s, moduleId: "legacy" })) as NavSection[]
  );

  // 2. Layers declarativos (maior prioridade — sobrescrevem o legado pelo id da section).
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
});
