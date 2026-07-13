// Opcoes de conta dona (dropdown do modal) compartilhadas entre o encurtador e o
// QR. So platform_admin escolhe a conta; o resto fica travado na conta ativa
// (o back ignora accountId de nao-admin). Fonte da lista: useTenantsStore
// (/v1/tenants = core.accounts). Ver docs/tools/PLANO_MODULO_TOOLS.md.
import { useCoreAccountStore } from '../../layers/core/stores/account'

export function useToolsClientOptions() {
  const auth = useAuthStore()
  const account = useCoreAccountStore()
  const tenantsStore = useTenantsStore()

  const isAdmin = computed(() => auth.role === 'platform_admin')
  const canChooseClient = computed(() => isAdmin.value)
  const viewerUserType = computed<'admin' | 'client'>(() => (isAdmin.value ? 'admin' : 'client'))

  const clientOptions = computed<Array<{ label: string; value: string }>>(() =>
    (tenantsStore.tenants || []).map((tenant) => ({
      label: tenant.name || `Conta ${String(tenant.id).slice(0, 8)}`,
      value: tenant.id,
    })),
  )

  const activeClientLabel = computed(() => account.activeAccount?.name || 'Conta ativa')
  const activeClientId = computed(() => account.activeAccountId || '')

  async function ensureClientOptions() {
    if (canChooseClient.value) {
      await tenantsStore.ensureLoaded()
    }
  }

  function clientLabelForValue(value: string) {
    const found = clientOptions.value.find((option) => option.value === value)
    return found?.label ?? activeClientLabel.value
  }

  return {
    isAdmin,
    canChooseClient,
    viewerUserType,
    clientOptions,
    activeClientLabel,
    activeClientId,
    ensureClientOptions,
    clientLabelForValue,
  }
}
