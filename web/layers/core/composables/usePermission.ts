import { computed } from "vue";
import { useCoreAccountStore } from "../stores/account";

export function usePermission() {
  const accountStore = useCoreAccountStore();

  function has(key: string): boolean {
    return accountStore.hasPermission(key);
  }

  function hasAll(...keys: string[]): boolean {
    return keys.every((k) => accountStore.hasPermission(k));
  }

  function hasAny(...keys: string[]): boolean {
    return keys.some((k) => accountStore.hasPermission(k));
  }

  const permissions = computed(() => accountStore.permissions);

  return { has, hasAll, hasAny, permissions };
}
