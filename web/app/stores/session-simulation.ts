import { defineStore } from 'pinia'

export type SessionSimulationUserType = 'admin' | 'client'
export type SessionSimulationUserLevel =
  | 'admin'
  | 'consultant'
  | 'manager'
  | 'marketing'
  | 'finance'
  | 'viewer'

export interface SessionSimulationClientOption {
  label: string
  value: number
  coreTenantId?: string
  moduleCodes?: string[]
}

const DEFAULT_CLIENT_OPTIONS: SessionSimulationClientOption[] = [
  {
    label: 'crow',
    value: 106,
    coreTenantId: '106',
    moduleCodes: ['core_panel', 'atendimento', 'fila-atendimento'],
  },
  { label: 'Perola', value: 105, coreTenantId: '105', moduleCodes: ['core_panel', 'atendimento'] },
  { label: 'Dr Antonio Tavares', value: 104, coreTenantId: '104', moduleCodes: ['core_panel'] },
  { label: 'Zen as Fuck', value: 103, coreTenantId: '103', moduleCodes: ['core_panel'] },
  { label: 'Duby', value: 102, coreTenantId: '102', moduleCodes: ['core_panel'] },
  { label: 'sdfsodifho', value: 101, coreTenantId: '101', moduleCodes: ['core_panel'] },
]

function normalizeClientId(value: unknown) {
  const parsed = Number.parseInt(String(value ?? '').trim(), 10)
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return 0
  }

  return parsed
}

function normalizeModuleCodes(value: unknown) {
  const source = Array.isArray(value) ? value : []
  const dedupe = new Set<string>()
  const normalized: string[] = []

  for (const entry of source) {
    const code = String(entry ?? '')
      .trim()
      .toLowerCase()
      .replace(/\s+/g, '_')
    if (!code || dedupe.has(code)) {
      continue
    }

    dedupe.add(code)
    normalized.push(code)
  }

  return normalized
}

function dedupeClientOptions(options: SessionSimulationClientOption[]) {
  const deduped = new Map<number, SessionSimulationClientOption>()

  for (const option of options) {
    const clientId = normalizeClientId(option.value)
    if (clientId <= 0) {
      continue
    }

    deduped.set(clientId, {
      value: clientId,
      label: String(option.label || `Cliente #${clientId}`).trim() || `Cliente #${clientId}`,
      coreTenantId: String(option.coreTenantId || clientId).trim() || String(clientId),
      moduleCodes: normalizeModuleCodes(option.moduleCodes),
    })
  }

  return [...deduped.values()].sort((a, b) => a.label.localeCompare(b.label, 'pt-BR'))
}

export const useSessionSimulationStore = defineStore('sessionSimulation', () => {
  const auth = useAuthStore()
  const initialized = ref(false)
  const clientOptionsSynced = ref(false)
  const userType = ref<SessionSimulationUserType>('admin')
  const userLevel = ref<SessionSimulationUserLevel>('admin')
  const clientId = ref<number>(106)
  const clientOptions = ref<SessionSimulationClientOption[]>([...DEFAULT_CLIENT_OPTIONS])

  const isAdmin = computed(() => userType.value === 'admin')
  const activeClient = computed(
    () => clientOptions.value.find((option) => option.value === clientId.value) || null,
  )
  const activeClientLabel = computed(
    () => activeClient.value?.label || `Cliente #${clientId.value}`,
  )
  const activeClientCoreTenantId = computed(() =>
    String(activeClient.value?.coreTenantId || '').trim(),
  )
  const effectiveClientId = computed(() => normalizeClientId(clientId.value))

  const requestHeaders = computed<Record<string, string>>(() => {
    const headers: Record<string, string> = {
      'x-user-type': userType.value,
      'x-user-level': userLevel.value,
    }

    if (effectiveClientId.value > 0) {
      headers['x-client-id'] = String(effectiveClientId.value)
    }

    if (activeClientCoreTenantId.value) {
      headers['x-tenant-id'] = activeClientCoreTenantId.value
    }

    return headers
  })

  const requestContextHash = computed(() =>
    JSON.stringify({
      userType: userType.value,
      userLevel: userLevel.value,
      clientId: effectiveClientId.value,
      coreTenantId: activeClientCoreTenantId.value,
      optionCount: clientOptions.value.length,
    }),
  )

  function initialize() {
    userType.value = auth.role === 'platform_admin' ? 'admin' : 'client'
    userLevel.value = auth.role === 'platform_admin' ? 'admin' : 'viewer'

    if (!clientOptions.value.length) {
      clientOptions.value = [...DEFAULT_CLIENT_OPTIONS]
    }

    if (!clientOptions.value.some((option) => option.value === clientId.value)) {
      clientId.value = clientOptions.value[0]?.value ?? 106
    }

    clientOptionsSynced.value = true
    initialized.value = true
  }

  async function refreshClientOptions() {
    initialize()
    clientOptionsSynced.value = true
  }

  function replaceClientOptions(options: SessionSimulationClientOption[]) {
    const nextOptions = dedupeClientOptions(options)
    clientOptions.value = nextOptions.length > 0 ? nextOptions : [...DEFAULT_CLIENT_OPTIONS]

    if (!clientOptions.value.some((option) => option.value === clientId.value)) {
      clientId.value = clientOptions.value[0]?.value ?? 106
    }

    clientOptionsSynced.value = true
  }

  function setClientId(value: number) {
    const nextClientId = normalizeClientId(value)
    if (nextClientId <= 0) {
      return
    }

    clientId.value = nextClientId
  }

  return {
    initialized,
    clientOptionsSynced,
    userType,
    userLevel,
    clientId,
    clientOptions,
    isAdmin,
    activeClientLabel,
    activeClientCoreTenantId,
    effectiveClientId,
    requestHeaders,
    requestContextHash,
    initialize,
    refreshClientOptions,
    replaceClientOptions,
    setClientId,
  }
})
