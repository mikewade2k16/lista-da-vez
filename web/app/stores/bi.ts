import { computed, reactive, ref } from 'vue'
import { defineStore } from 'pinia'

import { useAuthStore } from '~/stores/auth'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'

export interface BiMetric {
  key: string
  label: string
  value: string
  detail?: string
  tone?: string
}

export interface BiSource {
  key: string
  label: string
  endpoint: string
  ok: boolean
  pending?: boolean
  upstreamStatus: number
  fetched: number
  total: number
  durationMs: number
  truncated?: boolean
  error?: string
}

export interface BiInsight {
  title: string
  body: string
  tone?: string
}

export interface BiIntelligenceItem {
  label: string
  value: string
  detail?: string
  tone?: string
}

export interface BiIntelligenceSection {
  key: string
  title: string
  summary: string
  tone?: string
  items: BiIntelligenceItem[]
}

export interface BiTableColumn {
  id: string
  label: string
  width?: string
  align?: string
  locked?: boolean
  defaultVisible?: boolean
  description?: string
}

export interface BiTableFilter {
  key: string
  label: string
  placeholder?: string
  options: string[]
}

export interface BiDataTable {
  key: string
  label: string
  description: string
  pending?: boolean
  total: number
  fetched: number
  columns: BiTableColumn[]
  filters: BiTableFilter[]
  rows: Array<Record<string, unknown>>
}

export interface BiOverview {
  ok: boolean
  generatedAt: string
  cnpjEmpresa: string
  metrics: BiMetric[]
  sources: BiSource[]
  insights: BiInsight[]
  sections: BiIntelligenceSection[]
  tables: BiDataTable[]
}

export interface BiManualConfig {
  companyKey: string
  cnpjEmpresa: string
  login: string
  pass: string
}

interface PerolaLoginResponse {
  ok?: boolean
  token?: string
  upstreamStatusText?: string
}

function normalizeOverview(response: Partial<BiOverview> | null | undefined): BiOverview {
  return {
    ok: Boolean(response?.ok),
    generatedAt: String(response?.generatedAt || ''),
    cnpjEmpresa: String(response?.cnpjEmpresa || ''),
    metrics: Array.isArray(response?.metrics) ? response.metrics : [],
    sources: Array.isArray(response?.sources) ? response.sources : [],
    insights: Array.isArray(response?.insights) ? response.insights : [],
    sections: Array.isArray(response?.sections) ? response.sections : [],
    tables: Array.isArray(response?.tables) ? response.tables : [],
  }
}

function normalizeText(value: unknown) {
  return String(value || '').trim()
}

function normalizeBearer(value: unknown) {
  const token = normalizeText(value)
  const parts = token.split(/\s+/)
  if (parts.length === 2 && parts[0].toLowerCase() === 'bearer') {
    return parts[1].trim()
  }
  return token
}

export const useBiStore = defineStore('bi', () => {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  const overview = ref<BiOverview | null>(null)
  const loading = ref(false)
  const inventoryLoading = ref(false)
  const loggingIn = ref(false)
  const error = ref('')
  const loginError = ref('')
  const manualToken = ref('')
  const manualConfig = reactive<BiManualConfig>({
    companyKey: '',
    cnpjEmpresa: '',
    login: '',
    pass: '',
  })

  const tables = computed(() => overview.value?.tables || [])
  const metrics = computed(() => overview.value?.metrics || [])
  const insights = computed(() => overview.value?.insights || [])
  const sections = computed(() => overview.value?.sections || [])
  const sources = computed(() => overview.value?.sources || [])
  const hasData = computed(() => tables.value.some((table) => table.rows.length > 0))
  const hasManualToken = computed(() => Boolean(manualToken.value))

  function tableByKey(key: string) {
    return tables.value.find((table) => table.key === key) || null
  }

  function updateManualConfig(input: Partial<BiManualConfig>) {
    if (typeof input.companyKey === 'string') manualConfig.companyKey = input.companyKey
    if (typeof input.cnpjEmpresa === 'string') manualConfig.cnpjEmpresa = input.cnpjEmpresa
    if (typeof input.login === 'string') manualConfig.login = input.login
    if (typeof input.pass === 'string') manualConfig.pass = input.pass
  }

  function setManualToken(value: string) {
    manualToken.value = normalizeBearer(value)
  }

  function clearManualSession() {
    manualToken.value = ''
    loginError.value = ''
  }

  function buildOverviewUrl(includeInventory: boolean) {
    const params = new URLSearchParams()
    const cnpjEmpresa = normalizeText(manualConfig.cnpjEmpresa)
    if (cnpjEmpresa) {
      params.set('cnpjEmpresa', cnpjEmpresa)
    }
    if (includeInventory) {
      params.set('includeInventory', '1')
    }
    const query = params.toString()
    return `/v1/bi/perola/overview${query ? `?${query}` : ''}`
  }

  function shouldLoadInventory(snapshot: BiOverview | null) {
    return Boolean(snapshot?.sources?.find((source) => source.key === 'inventario')?.pending)
  }

  function applyInventoryFailure(message: string) {
    if (!overview.value) {
      return
    }

    overview.value = {
      ...overview.value,
      sources: overview.value.sources.map((source) =>
        source.key === 'inventario'
          ? { ...source, pending: false, ok: false, error: message }
          : source,
      ),
      tables: overview.value.tables.map((table) =>
        table.key === 'inventario'
          ? { ...table, pending: false }
          : table,
      ),
    }
  }

  async function loginPerola() {
    const missing: string[] = []
    if (!normalizeText(manualConfig.companyKey)) missing.push('CompanyKey')
    if (!normalizeText(manualConfig.login)) missing.push('Login')
    if (!manualConfig.pass) missing.push('Pass')

    if (missing.length > 0) {
      const message = `Preencha antes de gerar token: ${missing.join(', ')}.`
      loginError.value = message
      return { ok: false, message }
    }

    try {
      loggingIn.value = true
      loginError.value = ''
      await auth.ensureSession()

      const response = (await apiRequest('/v1/bi/perola/login', {
        method: 'POST',
        body: {
          companyKey: normalizeText(manualConfig.companyKey),
          login: normalizeText(manualConfig.login),
          pass: manualConfig.pass,
        },
      })) as PerolaLoginResponse

      const token = normalizeBearer(response?.token)
      if (!response?.ok || !token) {
        const message = response?.upstreamStatusText || 'Login retornou sem token JWT.'
        loginError.value = message
        return { ok: false, message }
      }

      manualToken.value = token
      return { ok: true, token }
    } catch (err) {
      const message = getApiErrorMessage(err, 'Nao foi possivel autenticar na Perola BI.')
      loginError.value = message
      return { ok: false, message }
    } finally {
      loggingIn.value = false
    }
  }

  async function refreshOverview(options: { includeInventory?: boolean; background?: boolean } = {}) {
    const includeInventory = options.includeInventory === true
    const background = options.background === true

    try {
      if (background) {
        inventoryLoading.value = true
      } else {
        loading.value = true
        error.value = ''
      }
      await auth.ensureSession()

      const headers: Record<string, string> = {}
      const companyKey = normalizeText(manualConfig.companyKey)
      if (companyKey && manualToken.value) {
        headers['X-Perola-Company-Key'] = companyKey
      }
      if (manualToken.value) {
        headers['X-Perola-Token'] = manualToken.value
      }

      const response = (await apiRequest(buildOverviewUrl(includeInventory), {
        headers,
        dedupe: false,
      })) as BiOverview
      overview.value = normalizeOverview(response)

      return { ok: overview.value.ok, data: overview.value }
    } catch (err) {
      const message = getApiErrorMessage(err, 'Nao foi possivel carregar o BI da Perola.')
      if (background) {
        applyInventoryFailure(message)
      } else {
        error.value = message
      }
      return { ok: false, message }
    } finally {
      if (background) {
        inventoryLoading.value = false
      } else {
        loading.value = false
      }
    }
  }

  return {
    overview,
    loading,
    inventoryLoading,
    loggingIn,
    error,
    loginError,
    tables,
    metrics,
    insights,
    sections,
    sources,
    hasData,
    hasManualToken,
    manualToken,
    manualConfig,
    tableByKey,
    updateManualConfig,
    setManualToken,
    clearManualSession,
    loginPerola,
    refreshOverview,
  }
})
