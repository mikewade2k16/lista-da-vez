import { computed } from 'vue'
import { useAuthStore } from '~/stores/auth'

export function useCan(permissionKey: string) {
	const auth = useAuthStore()
	const normalizedPermissionKey = String(permissionKey || '').trim()
	return computed(() => auth.permissionKeys.includes(normalizedPermissionKey))
}