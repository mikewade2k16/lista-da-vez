import { computed } from "vue";
import { useCoreAccountStore } from "../stores/account";

export function useNav() {
  // useNavStore é auto-importado pelo Nuxt a partir de app/stores/nav.ts
  const navStore = useNavStore();
  const accountStore = useCoreAccountStore();

  const enabledModules = computed(() => new Set(accountStore.enabledModules));

  const sections = computed(() =>
    navStore.sections.filter(
      (s) => s.moduleId === "legacy" || s.moduleId === "core" || enabledModules.value.has(s.moduleId)
    )
  );

  return { sections };
}
