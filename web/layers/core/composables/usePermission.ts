import { computed } from 'vue'
import { useCoreAccountStore } from '../stores/account'

/**
 * Fornece consultas de permissao de alto nivel sobre o store central de conta.
 *
 * Serve como fachada minima para componentes e layouts do layer core checarem chaves individuais,
 * conjuntas ou alternativas sem depender diretamente da implementacao do store.
 *
 * @returns Helpers `has`, `hasAll`, `hasAny` e a lista reativa de permissoes atuais.
 *
 * @example
 * ```ts
 * const { hasAny } = usePermission()
 * const canManageUsers = hasAny('users:write', 'tenant:admin')
 * ```
 */
export function usePermission() {
  const accountStore = useCoreAccountStore()

  function has(key: string): boolean {
    return accountStore.hasPermission(key)
  }

  function hasAll(...keys: string[]): boolean {
    return keys.every((k) => accountStore.hasPermission(k))
  }

  function hasAny(...keys: string[]): boolean {
    return keys.some((k) => accountStore.hasPermission(k))
  }

  const permissions = computed(() => accountStore.permissions)

  return { has, hasAll, hasAny, permissions }
}
