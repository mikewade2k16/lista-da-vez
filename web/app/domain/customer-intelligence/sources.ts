import type { createApiRequest } from '~/utils/api-client'

type SourceApi = ReturnType<typeof createApiRequest>

export type SourceConfigFieldType =
  | 'integer'
  | 'boolean'
  | 'select'
  | 'safe_key'
  | 'string_list'
  | 'uuid'

export type SourceConfigValue = string | number | boolean | string[]
export type ObservationRetentionAction = 'tombstone' | 'crypto_shred'

export interface SourceConfigField {
  key: string
  label: string
  type: SourceConfigFieldType
  required?: boolean
  min?: number
  max?: number
  options: string[]
  elementKeys: string[]
}

interface BackendSourceDescriptor {
  key: string
  label: string
  ownerPackage?: string
  capabilities?: string[]
  modes?: string[]
  requiresModule?: string
  allowedConfigKeys?: string[]
  allowedFields?: string[]
  purposeKeys?: string[]
  configSchema?: Array<{
    key: string
    label: string
    type: SourceConfigFieldType
    required?: boolean
    min?: number
    max?: number
    options?: string[]
    elementKeys?: string[]
  }>
}

export interface IntelligenceSourceDescriptor {
  sourceKey: string
  name: string
  description: string
  ownerPackage: string
  moduleId: string
  category: string
  modes: string[]
  allowedConfigKeys: string[]
  allowedFields: string[]
  capabilities: string[]
  dataClasses: string[]
  purposeKeys: string[]
  configSchema: SourceConfigField[]
}

interface BackendSourceConfig {
  id: string
  clientAccountId: string
  sourceKey: string
  connectionKey: string
  status: string
  mode: string
  purposeKey: string
  fieldAllowlist?: string[]
  freshnessSeconds: number
  retentionPolicyKey?: string
  retentionPolicyVersionId?: string
  retentionPolicyVersion?: number
  snapshotTtlSeconds?: number
  onExpiry?: ObservationRetentionAction
  config?: Record<string, unknown>
  revision: number
  lastHealthStatus?: string
  updatedAt?: string
}

export interface IntelligenceSourceConfig extends BackendSourceConfig {
  retentionPolicyKey: string
  retentionPolicyVersionId: string
  retentionPolicyVersion: number
  snapshotTtlSeconds: number
  onExpiry: ObservationRetentionAction
  enabled: boolean
  lastSuccessAt?: string | null
  lagSeconds?: number | null
  healthReasonCode?: string
}

export interface IntelligenceSourceDraft {
  connectionKey: string
  status: string
  mode: string
  purposeKey: string
  fieldAllowlist: string[]
  freshnessSeconds: number
  retentionPolicyKey: string
  snapshotTtlSeconds: number
  onExpiry: ObservationRetentionAction
  config: Record<string, SourceConfigValue>
}

export interface IntelligenceSourceWriteInput {
  clientAccountId: string
  sourceKey: string
  connectionKey: string
  status: string
  mode: string
  purposeKey: string
  fieldAllowlist: string[]
  freshnessSeconds: number
  retentionPolicyKey: string
  snapshotTtlSeconds: number
  onExpiry: ObservationRetentionAction
  config: Record<string, SourceConfigValue>
  expectedRevision: number
}

export interface IntelligenceSourceDraftValidation {
  valid: boolean
  errors: Record<string, string>
}

export const SOURCE_STATUS_OPTIONS = [
  { value: 'disabled', label: 'Desabilitada' },
  { value: 'draft', label: 'Rascunho' },
  { value: 'enabled', label: 'Habilitada' },
  { value: 'error', label: 'Bloqueada por erro' },
] as const

const SOURCE_STATUSES = new Set(SOURCE_STATUS_OPTIONS.map((item) => item.value))
const SAFE_KEY_PATTERN = /^[a-z][a-z0-9]*(?:[._-][a-z0-9]+)*$/
const UUID_PATTERN =
  /^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$/
const DEFAULT_FRESHNESS_SECONDS = 3600
const MAX_FRESHNESS_SECONDS = 31_536_000
const DEFAULT_RETENTION_POLICY_KEY = 'customer_profile.default'
const DEFAULT_SNAPSHOT_TTL_SECONDS = 7_776_000
const MIN_SNAPSHOT_TTL_SECONDS = 86_400
const MAX_SNAPSHOT_TTL_SECONDS = 315_360_000

function clientQuery(clientAccountId: string): string {
  const query = new URLSearchParams()
  if (clientAccountId.trim()) query.set('clientAccountId', clientAccountId.trim())
  return query.toString()
}

function descriptorCategory(capabilities: string[]): string {
  if (capabilities.includes('portfolio_aggregate')) return 'agregado'
  if (capabilities.includes('business_context')) return 'contexto do cliente'
  return 'dados do relacionamento'
}

export function normalizeIntelligenceSourceDescriptor(
  item: BackendSourceDescriptor,
): IntelligenceSourceDescriptor {
  const capabilities = Array.isArray(item.capabilities) ? [...item.capabilities] : []
  const ownerPackage = String(item.ownerPackage || 'customerintelligence')
  return {
    sourceKey: item.key,
    name: item.label,
    description: item.requiresModule
      ? `Requer o modulo ${item.requiresModule}.`
      : `Fonte mantida por ${ownerPackage}.`,
    ownerPackage,
    moduleId: item.requiresModule || ownerPackage,
    category: descriptorCategory(capabilities),
    modes: Array.isArray(item.modes) ? [...item.modes] : [],
    allowedConfigKeys: Array.isArray(item.allowedConfigKeys) ? [...item.allowedConfigKeys] : [],
    allowedFields: Array.isArray(item.allowedFields) ? [...item.allowedFields] : [],
    capabilities,
    dataClasses: Array.isArray(item.allowedFields) ? [...item.allowedFields] : [],
    purposeKeys: Array.isArray(item.purposeKeys) ? [...item.purposeKeys] : [],
    configSchema: Array.isArray(item.configSchema)
      ? item.configSchema.map((field) => ({
          ...field,
          options: Array.isArray(field.options) ? [...field.options] : [],
          elementKeys: Array.isArray(field.elementKeys) ? [...field.elementKeys] : [],
        }))
      : [],
  }
}

function normalizeConfig(item: BackendSourceConfig): IntelligenceSourceConfig {
  return {
    ...item,
    fieldAllowlist: item.fieldAllowlist ?? [],
    retentionPolicyKey: item.retentionPolicyKey || DEFAULT_RETENTION_POLICY_KEY,
    retentionPolicyVersionId: item.retentionPolicyVersionId || '',
    retentionPolicyVersion: item.retentionPolicyVersion ?? 0,
    snapshotTtlSeconds: item.snapshotTtlSeconds ?? DEFAULT_SNAPSHOT_TTL_SECONDS,
    onExpiry: item.onExpiry ?? 'tombstone',
    config: item.config ?? {},
    enabled: item.status === 'enabled',
    healthReasonCode: item.lastHealthStatus || undefined,
  }
}

function preferredDescriptorValue(values: string[], preferred: string): string {
  return values.includes(preferred) ? preferred : (values[0] ?? '')
}

function cloneConfigValue(value: SourceConfigValue): SourceConfigValue {
  return Array.isArray(value) ? [...value] : value
}

function validConfigValue(field: SourceConfigField, value: unknown): value is SourceConfigValue {
  switch (field.type) {
    case 'integer':
      return (
        typeof value === 'number' &&
        Number.isInteger(value) &&
        (field.min === undefined || value >= field.min) &&
        (field.max === undefined || value <= field.max)
      )
    case 'boolean':
      return typeof value === 'boolean'
    case 'select':
      return typeof value === 'string' && field.options.includes(value)
    case 'safe_key':
      return typeof value === 'string' && value.length <= 120 && SAFE_KEY_PATTERN.test(value)
    case 'uuid':
      return typeof value === 'string' && UUID_PATTERN.test(value)
    case 'string_list':
      return (
        Array.isArray(value) &&
        value.every((item): item is string => typeof item === 'string') &&
        new Set(value).size === value.length &&
        value.every((item) => field.elementKeys.includes(item))
      )
    default:
      return false
  }
}

export function createIntelligenceSourceDraft(
  descriptor: IntelligenceSourceDescriptor,
  source: IntelligenceSourceConfig | null,
): IntelligenceSourceDraft {
  const config: Record<string, SourceConfigValue> = {}
  for (const field of descriptor.configSchema) {
    const value = source?.config?.[field.key]
    if (validConfigValue(field, value)) config[field.key] = cloneConfigValue(value)
  }
  return {
    connectionKey: source?.connectionKey ?? 'default',
    status: source?.status ?? 'disabled',
    mode:
      source?.mode ??
      preferredDescriptorValue(
        descriptor.modes,
        descriptor.modes.includes('on_demand') ? 'on_demand' : 'manual',
      ),
    purposeKey:
      source?.purposeKey ?? preferredDescriptorValue(descriptor.purposeKeys, 'customer_profile'),
    fieldAllowlist: (source?.fieldAllowlist ?? []).filter((field, index, values) => {
      return descriptor.allowedFields.includes(field) && values.indexOf(field) === index
    }),
    freshnessSeconds: source?.freshnessSeconds ?? DEFAULT_FRESHNESS_SECONDS,
    retentionPolicyKey: source?.retentionPolicyKey ?? DEFAULT_RETENTION_POLICY_KEY,
    snapshotTtlSeconds: source?.snapshotTtlSeconds ?? DEFAULT_SNAPSHOT_TTL_SECONDS,
    onExpiry: source?.onExpiry ?? 'tombstone',
    config,
  }
}

export function validateIntelligenceSourceDraft(
  descriptor: IntelligenceSourceDescriptor,
  draft: IntelligenceSourceDraft,
): IntelligenceSourceDraftValidation {
  const errors: Record<string, string> = {}
  if (
    draft.connectionKey.length < 1 ||
    draft.connectionKey.length > 120 ||
    !SAFE_KEY_PATTERN.test(draft.connectionKey)
  ) {
    errors.connectionKey =
      'Use uma chave segura: minusculas, numeros e separadores ponto, hifen ou sublinhado.'
  }
  if (!SOURCE_STATUSES.has(draft.status as (typeof SOURCE_STATUS_OPTIONS)[number]['value'])) {
    errors.status = 'Selecione um status registrado.'
  }
  if (!descriptor.modes.includes(draft.mode)) {
    errors.mode = 'Selecione um modo permitido para esta fonte.'
  }
  if (!descriptor.purposeKeys.includes(draft.purposeKey)) {
    errors.purposeKey = 'Selecione uma finalidade permitida para esta fonte.'
  }
  if (
    new Set(draft.fieldAllowlist).size !== draft.fieldAllowlist.length ||
    draft.fieldAllowlist.some((field) => !descriptor.allowedFields.includes(field))
  ) {
    errors.fieldAllowlist = 'A lista contem um campo nao registrado para esta fonte.'
  }
  if (
    !Number.isInteger(draft.freshnessSeconds) ||
    draft.freshnessSeconds < 0 ||
    draft.freshnessSeconds > MAX_FRESHNESS_SECONDS
  ) {
    errors.freshnessSeconds = 'Informe de 0 a 31536000 segundos.'
  }
  if (
    draft.retentionPolicyKey.length < 1 ||
    draft.retentionPolicyKey.length > 160 ||
    !SAFE_KEY_PATTERN.test(draft.retentionPolicyKey)
  ) {
    errors.retentionPolicyKey = 'A chave da policy de retencao e invalida.'
  }
  if (
    !Number.isInteger(draft.snapshotTtlSeconds) ||
    draft.snapshotTtlSeconds < MIN_SNAPSHOT_TTL_SECONDS ||
    draft.snapshotTtlSeconds > MAX_SNAPSHOT_TTL_SECONDS
  ) {
    errors.snapshotTtlSeconds = 'O prazo de retencao deve ficar entre 1 e 3650 dias.'
  }
  if (draft.onExpiry !== 'tombstone' && draft.onExpiry !== 'crypto_shred') {
    errors.onExpiry = 'Selecione uma acao de expiracao registrada.'
  }

  const fieldsByKey = new Map(descriptor.configSchema.map((field) => [field.key, field]))
  for (const key of Object.keys(draft.config)) {
    if (!fieldsByKey.has(key)) {
      errors.config = 'A configuracao contem um campo que nao pertence ao descriptor.'
    }
  }
  for (const field of descriptor.configSchema) {
    const value = draft.config[field.key]
    if (value === undefined) {
      if (field.required) errors[`config.${field.key}`] = `${field.label} e obrigatorio.`
      continue
    }
    if (!validConfigValue(field, value)) {
      errors[`config.${field.key}`] = `${field.label} possui um valor invalido.`
    }
  }
  return { valid: Object.keys(errors).length === 0, errors }
}

export function buildIntelligenceSourceWriteInput(
  descriptor: IntelligenceSourceDescriptor,
  source: IntelligenceSourceConfig | null,
  clientAccountId: string,
  draft: IntelligenceSourceDraft,
): IntelligenceSourceWriteInput {
  const validation = validateIntelligenceSourceDraft(descriptor, draft)
  if (!validation.valid) {
    throw new Error(Object.values(validation.errors)[0] || 'Configuracao de fonte invalida.')
  }
  const config: Record<string, SourceConfigValue> = {}
  for (const field of descriptor.configSchema) {
    const value = draft.config[field.key]
    if (value !== undefined) config[field.key] = cloneConfigValue(value)
  }
  return {
    clientAccountId,
    sourceKey: descriptor.sourceKey,
    connectionKey: draft.connectionKey,
    status: draft.status,
    mode: draft.mode,
    purposeKey: draft.purposeKey,
    fieldAllowlist: [...draft.fieldAllowlist],
    freshnessSeconds: draft.freshnessSeconds,
    retentionPolicyKey: draft.retentionPolicyKey,
    snapshotTtlSeconds: draft.snapshotTtlSeconds,
    onExpiry: draft.onExpiry,
    config,
    expectedRevision: source?.revision ?? 0,
  }
}

export async function fetchIntelligenceSourceCatalog(
  api: SourceApi,
  clientAccountId: string,
  signal?: AbortSignal,
): Promise<IntelligenceSourceDescriptor[]> {
  const response = (await api(
    `/v1/customer-intelligence/sources/catalog?${clientQuery(clientAccountId)}`,
    { signal, dedupe: false },
  )) as BackendSourceDescriptor[]
  return Array.isArray(response) ? response.map(normalizeIntelligenceSourceDescriptor) : []
}

export async function fetchIntelligenceSources(
  api: SourceApi,
  clientAccountId: string,
  signal?: AbortSignal,
): Promise<IntelligenceSourceConfig[]> {
  const response = (await api(`/v1/customer-intelligence/sources?${clientQuery(clientAccountId)}`, {
    signal,
    dedupe: false,
  })) as BackendSourceConfig[]
  return Array.isArray(response) ? response.map(normalizeConfig) : []
}

export async function saveIntelligenceSource(
  api: SourceApi,
  input: IntelligenceSourceWriteInput,
): Promise<IntelligenceSourceConfig> {
  const response = (await api('/v1/customer-intelligence/sources', {
    method: 'PUT',
    body: input,
  })) as BackendSourceConfig
  return normalizeConfig(response)
}

export function syncIntelligenceSource(
  api: SourceApi,
  sourceId: string,
  clientAccountId: string,
): Promise<{ id: string; status: string }> {
  return api(`/v1/customer-intelligence/sources/${encodeURIComponent(sourceId)}/sync`, {
    method: 'POST',
    body: {
      clientAccountId,
      trigger: 'manual',
      idempotencyKey: globalThis.crypto?.randomUUID?.() ?? `panel-${Date.now()}`,
    },
  }) as Promise<{ id: string; status: string }>
}

export function setIntelligenceSourceEnabled(
  api: SourceApi,
  source: IntelligenceSourceConfig,
  enabled: boolean,
): Promise<IntelligenceSourceConfig> {
  return saveIntelligenceSource(api, {
    clientAccountId: source.clientAccountId,
    sourceKey: source.sourceKey,
    connectionKey: source.connectionKey,
    status: enabled ? 'enabled' : 'disabled',
    mode: source.mode,
    purposeKey: source.purposeKey,
    fieldAllowlist: source.fieldAllowlist ?? [],
    freshnessSeconds: source.freshnessSeconds,
    retentionPolicyKey: source.retentionPolicyKey,
    snapshotTtlSeconds: source.snapshotTtlSeconds,
    onExpiry: source.onExpiry,
    config: source.config as Record<string, SourceConfigValue>,
    expectedRevision: source.revision,
  })
}
