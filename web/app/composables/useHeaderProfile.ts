import { computed } from 'vue'
import { storeToRefs } from 'pinia'
import { useAuthStore } from '~/stores/auth'
import { getRoleLabel } from '~/domain/utils/permissions'
import { getApiBase } from '~/utils/api-client'

/**
 * Dados de exibicao do usuario logado para o header (nome, e-mail, papel, URL do
 * avatar e inicial de fallback). Usado pelo menu de perfil e pelo rodape do
 * drawer — uma fonte so para os dois.
 */
export function useHeaderProfile() {
  const auth = useAuthStore()
  const { user, role } = storeToRefs(auth)
  const runtimeConfig = useRuntimeConfig()

  const displayName = computed(() => String(user.value?.displayName || '').trim())
  const profileEmail = computed(() => String(user.value?.email || '').trim())
  const profileRoleLabel = computed(() => getRoleLabel(role.value))
  const avatarUrl = computed(() => {
    const avatarPath = String(user.value?.avatarPath || '').trim()
    if (!avatarPath) return ''
    return new URL(avatarPath, getApiBase(runtimeConfig)).toString()
  })
  const profileInitial = computed(() => displayName.value.charAt(0).toUpperCase() || 'U')

  return { displayName, profileEmail, profileRoleLabel, avatarUrl, profileInitial }
}
