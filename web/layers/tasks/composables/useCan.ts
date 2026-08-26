import { computed } from 'vue'
import { useAuthStore } from '~/stores/auth'

export function useCan(permissionKey: string) {
  const auth = useAuthStore()
  const normalizedPermissionKey = String(permissionKey || '').trim()
  return computed(
    () =>
      auth.role === 'platform_admin' ||
      auth.effectivePermissionKeys.includes(normalizedPermissionKey),
  )
}
