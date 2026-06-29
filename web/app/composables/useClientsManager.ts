import type {
  AccountFieldKey,
  AccountItem,
  AccountModuleAccess,
  AccountStore,
} from '~/types/accounts'
import { useInlineEditManager } from '~/composables/useInlineEditManager'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'

// Mapeia campo frontend → campo do PATCH (backend AdminUpdateAccountInput).
// `status` mapeia para `active` (bool). Campos read-only (userCount, etc.) e
// `modules` (que usa endpoint dedicado /modules) NÃO entram aqui.
const FIELD_TO_PATCH: Partial<Record<AccountFieldKey, string>> = {
  name: 'name',
  slug: 'slug',
  organizationId: 'organizationId',
  billingMode: 'billingMode',
  monthlyPaymentAmount: 'monthlyPaymentAmount',
  paymentDueDay: 'paymentDueDay',
  logo: 'logoPath',
  webhookEnabled: 'webhookEnabled',
  contactPhone: 'contactPhone',
  contactSite: 'contactSite',
  contactAddress: 'contactAddress',
  requireUserStoreLink: 'requireUserStoreLink',
  requireUserRegistration: 'requireUserRegistration',
}

function normalizeModules(raw: unknown): AccountModuleAccess[] {
  if (!Array.isArray(raw)) return []
  return raw.map((m: Record<string, unknown>) => ({
    code: String(m.moduleId ?? ''),
    name: String(m.label ?? m.moduleId ?? ''),
    status: m.enabled ? 'active' : 'inactive',
  }))
}

function normalizeStores(raw: unknown): AccountStore[] {
  if (!Array.isArray(raw)) return []
  return raw.map((s: Record<string, unknown>) => ({
    id: String(s.id ?? ''),
    code: String(s.code ?? ''),
    name: String(s.name ?? ''),
    city: String(s.city ?? ''),
    active: Boolean(s.active),
    amount: Number(s.billingAmount ?? 0) || 0,
  }))
}

function normalizeAccount(raw: Record<string, unknown>): AccountItem {
  const modules = normalizeModules(raw.modules)
  return {
    id: String(raw.id ?? ''),
    slug: String(raw.slug ?? ''),
    name: String(raw.name ?? ''),
    status: raw.active ? 'active' : 'inactive',
    planCode: String(raw.planCode ?? ''),
    isAgency: Boolean(raw.isAgency),
    billingMode: raw.billingMode === 'per_store' ? 'per_store' : 'single',
    monthlyPaymentAmount: Number(raw.monthlyPaymentAmount ?? 0) || 0,
    paymentDueDay: raw.paymentDueDay != null ? Number(raw.paymentDueDay) : null,
    webhookEnabled: Boolean(raw.webhookEnabled),
    webhookKey: String(raw.webhookKey ?? ''),
    contactPhone: String(raw.contactPhone ?? ''),
    contactSite: String(raw.contactSite ?? ''),
    contactAddress: String(raw.contactAddress ?? ''),
    logo: String(raw.logoPath ?? ''),
    organizationId: String(raw.organizationId ?? ''),
    requireUserStoreLink: Boolean(raw.requireUserStoreLink),
    requireUserRegistration: Boolean(raw.requireUserRegistration),
    userCount: Number(raw.userCount ?? 0) || 0,
    userNicks: String(raw.userNicks ?? ''),
    projectCount: Number(raw.projectCount ?? 0) || 0,
    projectSegments: String(raw.projectSegments ?? ''),
    modules,
    moduleCodes: modules.filter((m) => m.status === 'active').map((m) => m.code),
    stores: normalizeStores(raw.stores),
    createdAt: String(raw.createdAt ?? ''),
    updatedAt: String(raw.updatedAt ?? ''),
  }
}

export function useClientsManager() {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  const clients = ref<AccountItem[]>([])
  const filters = reactive({ q: '', status: '' as '' | 'active' | 'inactive' })
  const loading = ref(false)
  const creating = ref(false)
  const deletingId = ref<string | null>(null)
  const errorMessage = ref('')
  const canResetFilters = computed(() => Boolean(filters.q || filters.status))

  // Mecanica de edicao inline compartilhada (savingMap/setSaving/rowIsSaving +
  // debounce + cleanup). Comportamento identico ao anterior.
  const { savingMap, setSaving, rowIsSaving, schedulePatch } = useInlineEditManager()

  function applyPatch(id: string, raw: Record<string, unknown>) {
    const idx = clients.value.findIndex((a) => a.id === id)
    if (idx >= 0) clients.value[idx] = normalizeAccount(raw)
  }

  function patchLocal(id: string, field: AccountFieldKey, value: unknown) {
    const idx = clients.value.findIndex((a) => a.id === id)
    if (idx < 0) return
    ;(clients.value[idx] as Record<string, unknown>)[field] = value
  }

  async function fetchClients() {
    loading.value = true
    errorMessage.value = ''
    try {
      const resp = await apiRequest('/v1/admin/accounts')
      clients.value = (resp.accounts as Record<string, unknown>[]).map(normalizeAccount)
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao carregar contas.')
    } finally {
      loading.value = false
    }
  }

  async function persistPatch(id: string, fieldKey: string, patch: Record<string, unknown>) {
    const key = `${id}:${fieldKey}`
    setSaving(key, true)
    try {
      const resp = await apiRequest(`/v1/admin/accounts/${encodeURIComponent(id)}`, {
        method: 'PATCH',
        body: patch,
      })
      applyPatch(id, resp as Record<string, unknown>)
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao salvar.')
    } finally {
      setSaving(key, false)
    }
  }

  async function persistModules(id: string, nextCodes: string[]) {
    const key = `${id}:modules`
    setSaving(key, true)
    try {
      const account = clients.value.find((a) => a.id === id)
      const currentEnabled = (account?.modules ?? [])
        .filter((m) => m.status === 'active')
        .map((m) => m.code)
      const nextSet = new Set(nextCodes)
      const currentSet = new Set(currentEnabled)
      const enable = [...nextSet].filter((c) => !currentSet.has(c))
      const disable = [...currentSet].filter((c) => !nextSet.has(c))
      if (enable.length === 0 && disable.length === 0) return

      await apiRequest(`/v1/admin/accounts/${encodeURIComponent(id)}/modules`, {
        method: 'PUT',
        body: { enable, disable },
      })
      // Re-fetch account para pegar lista atualizada com novos enabled flags.
      const resp = await apiRequest(`/v1/admin/accounts/${encodeURIComponent(id)}`)
      applyPatch(id, resp as Record<string, unknown>)
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao atualizar modulos.')
    } finally {
      setSaving(key, false)
    }
  }

  function updateField(
    id: string,
    field: AccountFieldKey,
    value: unknown,
    opts?: { immediate?: boolean },
  ) {
    // 'modules' NAO passa por patchLocal: `value` e string[] (codes), mas
    // account.modules e {code,name,status}[]. Patchar corromperia o tipo e
    // zeraria o currentEnabled do persistModules -> o diff de `disable` nunca
    // dispararia (adicionar funciona, remover nao). persistModules le os modules
    // intactos, calcula o diff enable/disable e re-busca o account.
    if (field === 'modules') {
      const codes = Array.isArray(value) ? (value as string[]) : []
      void persistModules(id, codes)
      return
    }

    patchLocal(id, field, value)

    // status → active (bool no backend)
    const patchKey = field === 'status' ? 'active' : FIELD_TO_PATCH[field]
    if (!patchKey) return

    const patchValue = field === 'status' ? value === 'active' : value
    const patch = { [patchKey]: patchValue }
    schedulePatch(`${id}:${field}`, () => void persistPatch(id, field, patch), {
      immediate: opts?.immediate,
    })
  }

  async function saveContactAndLogo(
    id: string,
    payload: { logo: string; contactPhone: string; contactSite: string; contactAddress: string },
  ) {
    await persistPatch(id, 'contactPhone', {
      logoPath: payload.logo,
      contactPhone: payload.contactPhone,
      contactSite: payload.contactSite,
      contactAddress: payload.contactAddress,
    })
  }

  async function saveWebhookEnabled(id: string, enabled: boolean) {
    await persistPatch(id, 'webhookEnabled', { webhookEnabled: enabled })
  }

  async function rotateWebhookKey(id: string) {
    const key = `${id}:webhookKey`
    setSaving(key, true)
    try {
      const resp = await apiRequest(`/v1/admin/accounts/${encodeURIComponent(id)}/webhook/rotate`, {
        method: 'POST',
      })
      const idx = clients.value.findIndex((a) => a.id === id)
      if (idx >= 0)
        clients.value[idx]!.webhookKey = String((resp as { webhookKey?: string }).webhookKey ?? '')
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao rotacionar chave.')
    } finally {
      setSaving(key, false)
    }
  }

  async function saveStores(id: string, stores: AccountStore[]) {
    const key = `${id}:stores`
    setSaving(key, true)
    try {
      await apiRequest(`/v1/admin/accounts/${encodeURIComponent(id)}/stores`, {
        method: 'PUT',
        body: { stores: stores.map((s) => ({ id: s.id, amount: s.amount })) },
      })
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao salvar lojas.')
    } finally {
      setSaving(key, false)
    }
  }

  // Criação de account via POST /v1/admin/accounts (C10). O adminEmail e OPCIONAL:
  // vazio cria um cliente sem dono (controle interno, sem acesso). Quando informado,
  // o backend clona roles de template + vincula o usuario (ja existente) como owner.
  async function createClient(input?: {
    name: string
    slug: string
    planCode?: string
    adminEmail?: string
  }): Promise<string | null> {
    if (!input) {
      errorMessage.value = 'Informe ao menos o nome para criar a conta.'
      return null
    }
    const name = input.name.trim()
    const slug = input.slug.trim().toLowerCase()
    const adminEmail = (input.adminEmail ?? '').trim().toLowerCase()
    const planCode = (input.planCode ?? 'standard').trim() || 'standard'
    if (!name || !slug) {
      errorMessage.value = 'Nome e slug sao obrigatorios.'
      return null
    }
    creating.value = true
    errorMessage.value = ''
    try {
      const raw = await apiRequest('/v1/admin/accounts', {
        method: 'POST',
        body: { slug, name, planCode, adminEmail },
      })
      const account = normalizeAccount(raw as Record<string, unknown>)
      clients.value = [account, ...clients.value.filter((a) => a.id !== account.id)]
      return account.id
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao criar conta.')
      return null
    } finally {
      creating.value = false
    }
  }

  async function deleteClient(id: string) {
    deletingId.value = id
    setSaving(`${id}:delete`, true)
    errorMessage.value = ''
    try {
      await apiRequest(`/v1/admin/accounts/${encodeURIComponent(id)}`, { method: 'DELETE' })
      clients.value = clients.value.filter((a) => a.id !== id)
    } catch (e) {
      errorMessage.value = getApiErrorMessage(e, 'Falha ao excluir conta.')
    } finally {
      deletingId.value = null
      setSaving(`${id}:delete`, false)
    }
  }

  function resetFilters() {
    filters.q = ''
    filters.status = ''
  }

  return {
    clients,
    filters,
    loading,
    creating,
    deletingId,
    errorMessage,
    savingMap,
    canResetFilters,
    rowIsSaving,
    fetchClients,
    updateField,
    saveContactAndLogo,
    saveWebhookEnabled,
    rotateWebhookKey,
    saveStores,
    createClient,
    deleteClient,
    resetFilters,
  }
}
