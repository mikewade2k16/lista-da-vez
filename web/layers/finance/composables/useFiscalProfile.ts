import { computed, onMounted, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { useAuthStore } from '~/stores/auth'
import { useFinanceAccountContext } from './finance-account-context'
import { getApiErrorMessage } from '~/utils/api-client'
import { useFiscalProfileStore } from '~/stores/fiscal-profile'
import { emptyFiscalProfile, emptyFiscalTemplate } from '../types/fiscal'
import type {
  FiscalPortalCheck,
  FiscalPortalSession,
  FiscalProfileView,
  FiscalTemplate,
} from '../types/fiscal'

export function useFiscalProfile() {
  const auth = useAuthStore()
  const account = useFinanceAccountContext()
  const store = useFiscalProfileStore()
  const enabled = computed(
    () =>
      Boolean(account.activeAccountId && auth.user?.id) &&
      account.enabledModules.includes('finance') &&
      (auth.role === 'platform_admin' ||
        account.permissions.includes('finance.sheets.view') ||
        (auth.role === 'owner' && account.permissions.includes('core.account.manage'))),
  )
  const scopeKey = computed(() => `${account.activeAccountId}:${auth.user?.id || ''}`)
  const mounted = ref(false)
  const reloadKey = ref(0)
  const loading = ref(false)
  const saving = ref(false)
  const error = ref('')
  const message = ref('')
  const sessionResult = ref<FiscalPortalSession | null>(null)
  const portalResult = ref<FiscalPortalCheck | null>(null)
  const view = ref<FiscalProfileView | null>(null)
  const draft = reactive(emptyFiscalProfile())
  const profileDirty = ref(false)
  const templates = ref<FiscalTemplate[]>([])
  const templateDraft = reactive(emptyFiscalTemplate())
  const editingId = ref('')
  const editingVersion = ref('')
  const templateDirty = ref(false)
  const previewId = ref('')
  const preview = computed(() => templates.value.find((t) => t.id === previewId.value) || null)
  let generation = 0

  function clearTemplate() {
    editingId.value = ''
    editingVersion.value = ''
    Object.assign(templateDraft, emptyFiscalTemplate())
    templateDirty.value = false
  }
  function reset() {
    sessionResult.value = null
    portalResult.value = null
    view.value = null
    Object.assign(draft, emptyFiscalProfile())
    templates.value = []
    clearTemplate()
    profileDirty.value = false
    previewId.value = ''
    error.value = ''
    message.value = ''
    saving.value = false
    loading.value = false
  }
  function hydrate(data: FiscalProfileView) {
    if (portalResult.value?.profileVersion !== data.updatedAt) portalResult.value = null
    view.value = data
    Object.assign(draft, data.profile)
    profileDirty.value = false
  }
  async function load() {
    if (!enabled.value || !mounted.value) return
    const ticket = ++generation
    const accountId = account.activeAccountId
    loading.value = true
    error.value = ''
    message.value = ''
    try {
      const [profile, items] = await Promise.all([
        store.getProfile(accountId),
        store.listTemplates(accountId),
      ])
      if (ticket !== generation) return
      reloadKey.value++
      hydrate(profile)
      templates.value = items
      previewId.value = items.find((t) => t.isDefault)?.id || items[0]?.id || ''
      clearTemplate()
    } catch (cause) {
      if (ticket === generation)
        error.value = getApiErrorMessage(cause, 'Não foi possível carregar seu cadastro fiscal.')
    } finally {
      if (ticket === generation) loading.value = false
    }
  }
  async function mutate<T>(
    operation: (accountId: string) => Promise<T>,
    accept: (data: T) => void,
    success: string,
  ) {
    if (saving.value || loading.value || !enabled.value || !view.value) return false
    const ticket = generation
    const accountId = account.activeAccountId
    saving.value = true
    error.value = ''
    message.value = ''
    try {
      const data = await operation(accountId)
      if (ticket !== generation) return false
      accept(data)
      message.value = success
      return true
    } catch (cause) {
      if (ticket === generation)
        error.value = getApiErrorMessage(
          cause,
          'Não foi possível salvar. Recarregue em caso de conflito.',
        )
      return false
    } finally {
      if (ticket === generation) saving.value = false
    }
  }
  async function saveProfile() {
    const body = { profile: { ...draft }, expectedUpdatedAt: view.value?.updatedAt || '' }
    return mutate(
      (accountId) => store.saveProfile(accountId, body),
      hydrate,
      'Dados fiscais salvos no seu perfil.',
    )
  }
  async function saveCredentials(username: string, password: string) {
    if (profileDirty.value) return false
    const body = { username, password, expectedUpdatedAt: view.value?.updatedAt || '' }
    return mutate(
      (accountId) => store.saveCredentials(accountId, body),
      hydrate,
      'Acesso ao portal salvo. Nenhum login foi realizado.',
    )
  }
  async function deleteCredentials() {
    if (profileDirty.value) return false
    const body = { expectedUpdatedAt: view.value?.updatedAt || '' }
    return mutate(
      (accountId) => store.deleteCredentials(accountId, body),
      hydrate,
      'Acesso salvo removido.',
    )
  }
  async function loadPortalStatus() {
    if (!view.value || !enabled.value) return
    const ticket = generation
    const version = view.value.updatedAt
    try {
      const [result, session] = await Promise.all([
        store.portalStatus(account.activeAccountId),
        store.portalSessionStatus(account.activeAccountId),
      ])
      if (ticket === generation && view.value?.updatedAt === version) {
        portalResult.value = result
        sessionResult.value = session
      }
    } catch (cause) {
      if (ticket === generation)
        error.value = getApiErrorMessage(cause, 'Não foi possível consultar o teste de acesso.')
    }
  }
  async function connectPortalSession() {
    if (profileDirty.value) return false
    const version = view.value?.updatedAt || ''
    return mutate(
      (id) => store.connectPortal(id, version),
      (result) => {
        sessionResult.value = result
      },
      'Conexão verificada. Confira o estado da sessão.',
    )
  }
  async function testPortalAccess() {
    if (profileDirty.value) return false
    const version = view.value?.updatedAt || ''
    return mutate(
      (id) => store.testPortal(id, version),
      (result) => {
        portalResult.value = result
      },
      'Teste de acesso concluído. Confira o resultado.',
    )
  }
  function editTemplate(template: FiscalTemplate) {
    editingId.value = template.id
    editingVersion.value = template.updatedAt
    Object.assign(templateDraft, {
      name: template.name,
      isDefault: template.isDefault,
      fields: { ...template.fields },
    })
    templateDirty.value = false
  }
  async function saveTemplate() {
    const scope = scopeKey.value
    const id = editingId.value
    const body = {
      name: templateDraft.name,
      fields: { ...templateDraft.fields },
      isDefault: templateDraft.isDefault,
      expectedUpdatedAt: editingVersion.value,
    }
    return mutate(
      async (accountId) => {
        const saved = await store.saveTemplate(accountId, id, body)
        return {
          saved,
          items: scope === scopeKey.value ? await store.listTemplates(accountId) : [],
        }
      },
      ({ saved, items }) => {
        templates.value = items
        previewId.value = saved.id
        clearTemplate()
      },
      'Template salvo.',
    )
  }
  async function deleteTemplate(template: FiscalTemplate) {
    const scope = scopeKey.value
    const body = { expectedUpdatedAt: template.updatedAt }
    return mutate(
      async (accountId) => {
        await store.deleteTemplate(accountId, template.id, body)
        return scope === scopeKey.value ? store.listTemplates(accountId) : []
      },
      (items) => {
        templates.value = items
        previewId.value = items.find((t) => t.isDefault)?.id || items[0]?.id || ''
        if (editingId.value === template.id) clearTemplate()
      },
      'Template removido.',
    )
  }
  watch(
    [scopeKey, enabled, mounted],
    () => {
      generation++
      reset()
      if (mounted.value && enabled.value) void load()
    },
    { flush: 'sync' },
  )
  onMounted(() => {
    mounted.value = true
  })
  onBeforeUnmount(() => {
    generation++
  })
  return {
    sessionResult,
    connectPortalSession,
    portalResult,
    loadPortalStatus,
    testPortalAccess,
    account,
    enabled,
    scopeKey,
    reloadKey,
    loading,
    saving,
    error,
    message,
    view,
    draft,
    profileDirty,
    templates,
    templateDraft,
    editingId,
    templateDirty,
    previewId,
    preview,
    load,
    saveProfile,
    saveCredentials,
    deleteCredentials,
    editTemplate,
    clearTemplate,
    saveTemplate,
    deleteTemplate,
  }
}
export type FiscalProfileEditor = ReturnType<typeof useFiscalProfile>
