import { computed } from 'vue'
import { useCoreAccountStore } from '../stores/account'

/**
 * Filtra as secoes de navegacao do core conforme os modulos habilitados para a conta atual.
 *
 * O resultado preserva apenas secoes `legacy`, `core` e os modulos presentes em
 * `accountStore.enabledModules`, permitindo que o shell monte menus coerentes com a licenca ativa.
 *
 * @returns Lista reativa de secoes habilitadas para a sessao atual.
 *
 * @example
 * ```ts
 * const { sections } = useNav()
 * ```
 */
export function useNav() {
  // useNavStore é auto-importado pelo Nuxt a partir de app/stores/nav.ts
  const navStore = useNavStore()
  const accountStore = useCoreAccountStore()

  const enabledModules = computed(() => new Set(accountStore.enabledModules))

  const sections = computed(() =>
    navStore.sections.filter(
      (s) =>
        s.moduleId === 'legacy' || s.moduleId === 'core' || enabledModules.value.has(s.moduleId),
    ),
  )

  return { sections }
}
