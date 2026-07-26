import type { createApiRequest } from '~/utils/api-client'
import type {
  PromptCatalogView,
  PromptDraftInput,
  PromptProcessDefinition,
  PromptProcessView,
  PromptVersionView,
} from './prompt-types'

type PromptApi = ReturnType<typeof createApiRequest>

interface BackendProcessDefinition {
  id: string
  key: string
  label: string
  description: string
  status: string
  schemaVersion: string
  allowedVariables?: string[]
}

interface BackendPromptVersion {
  id: string
  processKey: string
  layer: string
  version: number
  status: string
  content?: string
  variables?: string[]
  revision: number
  createdAt?: string
  publishedAt?: string
}

interface BackendPromptBinding {
  id: string
  processKey: string
  processPromptVersionId: string
  agentVersionId: string
  status: string
  revision: number
}

interface BackendAgent {
  id: string
  name: string
  status: string
  activeVersionId?: string
}

interface BackendPromptEvaluation {
  id: string
  status: string
  scores?: Record<string, unknown>
  reasonCodes?: string[]
  createdAt?: string
}

function scopeQuery(clientAccountId: string, processKey = ''): string {
  const query = new URLSearchParams()
  if (clientAccountId.trim()) query.set('clientAccountId', clientAccountId.trim())
  if (processKey.trim()) query.set('processKey', processKey.trim())
  return query.toString()
}

function normalizeProcess(item: BackendProcessDefinition): PromptProcessDefinition {
  return {
    processKey: item.key,
    definitionId: item.id,
    name: item.label,
    description: item.description,
    owner: 'customer_intelligence',
    status: item.status,
    inputSchemaVersion: item.schemaVersion,
    outputSchemaVersion: item.schemaVersion,
    allowedVariables: (item.allowedVariables ?? []).map((key) => ({
      key,
      type: 'registered',
      classification: 'server_allowlisted',
      source: 'process_definition',
    })),
    // A API real ainda nao publica policySchema tipado. Policy nao e editada
    // como JSON/texto livre pelo frontend.
    policySchema: [],
  }
}

function normalizeVersion(item: BackendPromptVersion): PromptVersionView {
  return {
    id: item.id,
    version: item.version,
    status: item.status,
    revision: item.revision,
    promptText: item.content ?? '',
    config: {},
    checksum: `${item.layer}:${item.id}`,
    createdAt: item.createdAt,
    publishedAt: item.publishedAt ?? null,
  }
}

export async function fetchPromptCatalog(
  api: PromptApi,
  _clientAccountId: string,
  signal?: AbortSignal,
): Promise<PromptCatalogView> {
  const processes = (await api('/v1/customer-intelligence/processes', {
    signal,
    dedupe: false,
  })) as BackendProcessDefinition[]
  return {
    processes: Array.isArray(processes) ? processes.map(normalizeProcess) : [],
    // O backend atual nao registra GET /pipelines nem um descriptor de
    // capabilities legadas. O Studio deixa esses blocos vazios e mostra o gap.
    pipelines: [],
    legacyManagedCapabilities: [],
  }
}

export async function fetchPromptProcessView(
  api: PromptApi,
  processKey: string,
  clientAccountId: string,
  signal?: AbortSignal,
): Promise<PromptProcessView> {
  const query = scopeQuery(clientAccountId, processKey)
  const [processes, versions, bindings, agents] = (await Promise.all([
    api('/v1/customer-intelligence/processes', { signal, dedupe: false }),
    api(`/v1/customer-intelligence/prompts?${query}`, {
      signal,
      dedupe: false,
    }),
    api(`/v1/customer-intelligence/prompt-bindings?${query}`, {
      signal,
      dedupe: false,
    }),
    api(`/v1/customer-intelligence/agents?${scopeQuery(clientAccountId)}`, {
      signal,
      dedupe: false,
    }),
  ])) as [
    BackendProcessDefinition[],
    BackendPromptVersion[],
    BackendPromptBinding[],
    BackendAgent[],
  ]
  const definition = processes.find((item) => item.key === processKey)
  if (!definition) {
    throw Object.assign(new Error('Processo nao registrado.'), { statusCode: 404 })
  }
  const normalizedVersions = (Array.isArray(versions) ? versions : [])
    .map(normalizeVersion)
    .sort((left, right) => right.version - left.version)
  // A validated version remains the editable lifecycle candidate until it is
  // published; treating it as absent made the publish action unreachable.
  const draft =
    normalizedVersions.find((item) => item.status === 'draft' || item.status === 'validated') ??
    null
  const effective = (Array.isArray(bindings) ? bindings : []).find(
    (item) => item.processKey === processKey && item.status === 'published',
  )
  const publishedVersions = normalizedVersions.filter((item) => item.status === 'published')
  const published =
    publishedVersions.find((item) => item.id === effective?.processPromptVersionId) ??
    publishedVersions[0] ??
    null
  const rollbackTarget =
    publishedVersions.find((item) => item.id !== effective?.processPromptVersionId) ?? null
  const publishAgents = (Array.isArray(agents) ? agents : [])
    .filter(
      (item) =>
        item.status === 'enabled' &&
        typeof item.activeVersionId === 'string' &&
        item.activeVersionId.trim().length > 0,
    )
    .map((item) => ({
      agentId: item.id,
      agentVersionId: item.activeVersionId!.trim(),
      label: item.name,
    }))
  const evaluationVersion = draft ?? published
  const evaluations = evaluationVersion
    ? ((await api(
        `/v1/customer-intelligence/prompt-versions/${encodeURIComponent(evaluationVersion.id)}/evaluations`,
        { signal, dedupe: false },
      )) as BackendPromptEvaluation[])
    : []
  return {
    process: normalizeProcess(definition),
    draft,
    published,
    versions: normalizedVersions,
    evaluations: (Array.isArray(evaluations) ? evaluations : []).map((item) => ({
      id: item.id,
      status: item.status,
      qualityScore:
        typeof item.scores?.functional === 'number'
          ? item.scores.functional
          : typeof item.scores?.structural === 'number'
            ? item.scores.structural
            : null,
      safetyScore: typeof item.scores?.safety === 'number' ? item.scores.safety : null,
      schemaScore:
        typeof item.scores?.schema === 'number'
          ? item.scores.schema
          : typeof item.scores?.structural === 'number'
            ? item.scores.structural
            : null,
      violations: Array.isArray(item.reasonCodes) ? item.reasonCodes : [],
    })),
    effectiveBinding: effective
      ? {
          id: effective.id,
          revision: effective.revision,
          scope: clientAccountId ? 'client' : 'account',
          activeVersionId: effective.processPromptVersionId,
          rolloutMode: 'full',
          agentVersionId: effective.agentVersionId,
        }
      : null,
    publishAgents,
    rollbackTargetVersionId: rollbackTarget?.id ?? null,
    canEdit: Boolean(draft),
    canTest: Boolean(draft),
    canPublish: draft?.status === 'validated' && publishAgents.length > 0,
    canRollback: Boolean(effective && rollbackTarget),
  }
}

export function createPromptDraft(
  api: PromptApi,
  processKey: string,
  clientAccountId: string,
  promptText: string,
  basedOnVersionId = '',
): Promise<PromptVersionView> {
  return api(`/v1/customer-intelligence/prompts/${encodeURIComponent(processKey)}/drafts`, {
    method: 'POST',
    body: {
      clientAccountId,
      layer: 'process_prompt',
      content: promptText,
      ...(basedOnVersionId ? { basedOnVersionId } : {}),
    },
  }) as Promise<PromptVersionView>
}

export function publishPromptVersion(
  api: PromptApi,
  versionId: string,
  clientAccountId: string,
  agentVersionId: string,
): Promise<unknown> {
  return api(`/v1/customer-intelligence/prompt-versions/${encodeURIComponent(versionId)}/publish`, {
    method: 'POST',
    body: {
      clientAccountId,
      agentVersionId,
      sourcePolicy: [],
      toolPolicy: [],
      knowledgePolicy: [],
      runtimePolicy: {},
    },
  })
}

export function rollbackPromptBinding(
  api: PromptApi,
  bindingId: string,
  targetPromptVersionId: string,
): Promise<unknown> {
  return api(
    `/v1/customer-intelligence/prompt-bindings/${encodeURIComponent(bindingId)}/rollback`,
    {
      method: 'POST',
      body: {
        targetPromptVersionId,
        reasonCode: 'panel_rollback',
      },
    },
  )
}

export function updatePromptDraft(
  api: PromptApi,
  versionId: string,
  input: PromptDraftInput,
): Promise<PromptVersionView> {
  return api(`/v1/customer-intelligence/prompt-versions/${encodeURIComponent(versionId)}`, {
    method: 'PATCH',
    body: {
      content: input.promptText,
      expectedRevision: input.expectedRevision,
    },
  }) as Promise<PromptVersionView>
}

export function runPromptVersionAction(
  api: PromptApi,
  versionId: string,
  action: 'validate' | 'test',
): Promise<PromptVersionView> {
  return api(
    `/v1/customer-intelligence/prompt-versions/${encodeURIComponent(versionId)}/${action}`,
    action === 'test' ? { method: 'POST', body: { fixture: {} } } : { method: 'POST' },
  ) as Promise<PromptVersionView>
}
