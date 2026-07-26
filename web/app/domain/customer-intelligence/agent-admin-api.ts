import type { createApiRequest } from '~/utils/api-client'
import type {
  IntelligenceAgent,
  IntelligenceAgentCreateInput,
  IntelligenceAgentPatchInput,
  IntelligenceAgentVersion,
  IntelligenceAgentVersionWriteInput,
  IntelligenceAIProvider,
  IntelligenceCredential,
  IntelligenceCredentialWriteInput,
  IntelligenceModel,
  IntelligenceModelStatus,
  IntelligenceModelWriteInput,
} from './agent-admin-types'

type AgentAdminApi = ReturnType<typeof createApiRequest>
type UnknownRecord = Record<string, unknown>

const PROVIDERS = new Set<IntelligenceAIProvider>(['openai', 'gemini', 'glm'])

function record(value: unknown): UnknownRecord {
  return value && typeof value === 'object' && !Array.isArray(value) ? (value as UnknownRecord) : {}
}

function text(value: unknown): string {
  return String(value ?? '').trim()
}

function number(value: unknown): number {
  const normalized = Number(value)
  return Number.isFinite(normalized) ? normalized : 0
}

function provider(value: unknown): IntelligenceAIProvider {
  const normalized = text(value).toLowerCase() as IntelligenceAIProvider
  return PROVIDERS.has(normalized) ? normalized : 'openai'
}

function modelStatus(value: unknown): IntelligenceModelStatus {
  return text(value) === 'disabled' ? 'disabled' : 'enabled'
}

function normalizeModel(value: unknown): IntelligenceModel {
  const item = record(value)
  return {
    id: text(item.id),
    provider: provider(item.provider),
    model: text(item.model),
    baseUrl: text(item.baseUrl),
    status: modelStatus(item.status),
    config: record(item.config),
    revision: number(item.revision),
  }
}

function normalizeCredential(value: unknown): IntelligenceCredential {
  const item = record(value)
  const secret = record(item.secret)
  return {
    id: text(item.id),
    provider: provider(item.provider),
    label: text(item.label),
    // A leitura deliberadamente descarta qualquer campo de segredo inesperado.
    secret: {
      set: secret.set === true,
      last4: text(secret.last4),
    },
    updatedAt: text(item.updatedAt),
  }
}

function normalizeAgent(value: unknown): IntelligenceAgent {
  const item = record(value)
  return {
    id: text(item.id),
    name: text(item.name),
    purpose: text(item.purpose),
    status: text(item.status) === 'enabled' ? 'enabled' : 'disabled',
    activeVersionId: text(item.activeVersionId),
    updatedAt: text(item.updatedAt),
    revision: number(item.revision),
  }
}

function normalizeVersion(value: unknown): IntelligenceAgentVersion {
  const item = record(value)
  return {
    id: text(item.id),
    agentId: text(item.agentId),
    version: number(item.version),
    status: text(item.status),
    modelId: text(item.modelId),
    credentialId: text(item.credentialId),
    temperature: number(item.temperature),
    maxOutputTokens: number(item.maxOutputTokens),
    timeoutMs: number(item.timeoutMs),
    promptOverride: text(item.promptOverride),
    config: record(item.config),
  }
}

function normalizeArray<T>(value: unknown, normalize: (item: unknown) => T): T[] {
  return Array.isArray(value) ? value.map(normalize) : []
}

export async function fetchIntelligenceModels(
  api: AgentAdminApi,
  signal?: AbortSignal,
): Promise<IntelligenceModel[]> {
  const response = await api('/v1/customer-intelligence/models', {
    signal,
    dedupe: false,
  })
  return normalizeArray(response, normalizeModel)
}

export async function configureIntelligenceModel(
  api: AgentAdminApi,
  input: IntelligenceModelWriteInput,
): Promise<IntelligenceModel> {
  const response = await api('/v1/customer-intelligence/models', {
    method: 'PUT',
    body: input,
  })
  return normalizeModel(response)
}

export async function fetchIntelligenceCredentials(
  api: AgentAdminApi,
  signal?: AbortSignal,
): Promise<IntelligenceCredential[]> {
  const response = await api('/v1/customer-intelligence/credentials', {
    signal,
    dedupe: false,
  })
  return normalizeArray(response, normalizeCredential)
}

export async function configureIntelligenceCredential(
  api: AgentAdminApi,
  input: IntelligenceCredentialWriteInput,
): Promise<IntelligenceCredential> {
  const response = await api('/v1/customer-intelligence/credentials', {
    method: 'PUT',
    body: input,
  })
  return normalizeCredential(response)
}

export function revokeIntelligenceCredential(
  api: AgentAdminApi,
  credentialId: string,
): Promise<void> {
  return api(`/v1/customer-intelligence/credentials/${encodeURIComponent(credentialId)}`, {
    method: 'DELETE',
  }) as Promise<void>
}

export async function fetchIntelligenceAgents(
  api: AgentAdminApi,
  clientAccountId: string,
  signal?: AbortSignal,
): Promise<IntelligenceAgent[]> {
  const query = new URLSearchParams({ clientAccountId: clientAccountId.trim() })
  const response = await api(`/v1/customer-intelligence/agents?${query.toString()}`, {
    signal,
    dedupe: false,
  })
  return normalizeArray(response, normalizeAgent)
}

export async function createIntelligenceAgent(
  api: AgentAdminApi,
  input: IntelligenceAgentCreateInput,
): Promise<IntelligenceAgent> {
  const response = await api('/v1/customer-intelligence/agents', {
    method: 'POST',
    body: input,
  })
  return normalizeAgent(response)
}

export async function updateIntelligenceAgent(
  api: AgentAdminApi,
  agentId: string,
  input: IntelligenceAgentPatchInput,
): Promise<IntelligenceAgent> {
  const response = await api(`/v1/customer-intelligence/agents/${encodeURIComponent(agentId)}`, {
    method: 'PATCH',
    body: input,
  })
  return normalizeAgent(response)
}

export async function createIntelligenceAgentVersion(
  api: AgentAdminApi,
  agentId: string,
  input: IntelligenceAgentVersionWriteInput,
): Promise<IntelligenceAgentVersion> {
  const response = await api(
    `/v1/customer-intelligence/agents/${encodeURIComponent(agentId)}/versions`,
    { method: 'POST', body: input },
  )
  return normalizeVersion(response)
}

export async function publishIntelligenceAgentVersion(
  api: AgentAdminApi,
  versionId: string,
): Promise<IntelligenceAgentVersion> {
  const response = await api(
    `/v1/customer-intelligence/agent-versions/${encodeURIComponent(versionId)}/publish`,
    { method: 'POST' },
  )
  return normalizeVersion(response)
}
