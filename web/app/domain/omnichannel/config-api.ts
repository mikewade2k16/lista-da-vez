// Camada de acesso a dados das telas de config do omnichannel (F10). I/O puro sobre o
// apiRequest — separada dos componentes para mantê-los < 450 linhas e isolar a
// construcao de URL/parse da orquestracao de UI (page -> composable/fetch -> back).
//
// A F10 CONSOME as rotas ja entregues por F2/F4 (instancias/sessao/credencial), F8
// (setores/filas/regras) e F9 (agente/versao/simulate). NAO recria contrato.
//
// account_id vai SEMPRE no header X-Account-Id (injetado pelo api-client), nunca no body.
// A credencial NUNCA volta crua: so o status {set,last4}.
import type { createApiRequest } from '~/utils/api-client'
import type {
  OmniAgent,
  OmniAgentInput,
  OmniAgentVersion,
  OmniAiKnowledgeBinding,
  OmniAiToolBinding,
  OmniAiToolBindingInput,
  OmniAiToolApproval,
  OmniAiToolRun,
  OmniCollectField,
  OmniCredentialStatus,
  OmniDepartment,
  OmniDepartmentInput,
  OmniInstance,
  OmniInstanceManagement,
  OmniInstanceWriteInput,
  OmniHandoffPolicy,
  OmniHandoffPolicyInput,
  OmniMediaAnalysis,
  OmniKnowledgeBase,
  OmniKnowledgeChunkInput,
  OmniKnowledgeDocument,
  OmniQueue,
  OmniQueueInput,
  OmniQueueMember,
  OmniProviderKeyStatusView,
  OmniRoutingRule,
  OmniRoutingRuleInput,
  OmniSession,
  OmniSimulateInput,
  OmniSimulateResult,
} from '~/domain/omnichannel/config-types'

export type ApiRequest = ReturnType<typeof createApiRequest>

const BASE = '/v1/omnichannel'
const WA = `${BASE}/tenant/whatsapp`
// ============================================================================
// Numeros / instancias / providers (perm omnichannel.instances.manage)
// ============================================================================

export function fetchInstances(api: ApiRequest): Promise<OmniInstanceManagement> {
  return api(`${WA}/instances`) as Promise<OmniInstanceManagement>
}

export function createInstance(
  api: ApiRequest,
  input: OmniInstanceWriteInput,
): Promise<OmniInstance> {
  return api(`${WA}/instances`, { method: 'POST', body: input }) as Promise<OmniInstance>
}

export function updateInstance(
  api: ApiRequest,
  id: string,
  input: OmniInstanceWriteInput,
): Promise<OmniInstance> {
  return api(`${WA}/instances/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: input,
  }) as Promise<OmniInstance>
}

export function setInstanceUsers(
  api: ApiRequest,
  id: string,
  userIds: string[],
): Promise<OmniInstance> {
  return api(`${WA}/instances/${encodeURIComponent(id)}/users`, {
    method: 'PUT',
    body: { userIds },
  }) as Promise<OmniInstance>
}

// Credencial write-only. Vazio nao e permitido pelo back (apiKey obrigatorio); a tela
// so chama quando ha valor. A resposta e o status mascarado {set,last4}. Keyed por
// instanceName (nao id) — ver http_session.go.
export function setInstanceCredentials(
  api: ApiRequest,
  instanceName: string,
  apiKey: string,
): Promise<OmniCredentialStatus> {
  return api(`${WA}/instances/${encodeURIComponent(instanceName)}/credentials`, {
    method: 'PUT',
    body: { apiKey },
  }) as Promise<OmniCredentialStatus>
}

// Sessao/QR por instanceName (query param). Usadas na conexao/pareamento do numero.
export function fetchSessionStatus(api: ApiRequest, instanceName: string): Promise<OmniSession> {
  const q = new URLSearchParams({ instanceName }).toString()
  return api(`${WA}/status?${q}`, { dedupe: false }) as Promise<OmniSession>
}

export function fetchQrCode(api: ApiRequest, instanceName: string): Promise<OmniSession> {
  const q = new URLSearchParams({ instanceName }).toString()
  return api(`${WA}/qrcode?${q}`, { dedupe: false }) as Promise<OmniSession>
}

export function connectSession(api: ApiRequest, instanceName: string): Promise<OmniSession> {
  return api(`${WA}/connect`, {
    method: 'POST',
    body: { instanceName },
  }) as Promise<OmniSession>
}

export function logoutSession(api: ApiRequest, instanceName: string): Promise<OmniSession> {
  return api(`${WA}/logout`, { method: 'POST', body: { instanceName } }) as Promise<OmniSession>
}

// ============================================================================
// Setores / filas / membros / regras (perm omnichannel.settings.manage)
// ============================================================================

const SETTINGS = `${BASE}/settings`

export function fetchDepartments(api: ApiRequest): Promise<OmniDepartment[]> {
  return api(`${SETTINGS}/departments`) as Promise<OmniDepartment[]>
}

export function createDepartment(
  api: ApiRequest,
  input: OmniDepartmentInput,
): Promise<OmniDepartment> {
  return api(`${SETTINGS}/departments`, {
    method: 'POST',
    body: input,
  }) as Promise<OmniDepartment>
}

export function updateDepartment(
  api: ApiRequest,
  id: string,
  patch: Partial<Pick<OmniDepartment, 'name' | 'isDefault' | 'isActive'>>,
): Promise<OmniDepartment> {
  return api(`${SETTINGS}/departments/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: patch,
  }) as Promise<OmniDepartment>
}

export function fetchQueues(api: ApiRequest, departmentId?: string): Promise<OmniQueue[]> {
  const suffix = departmentId ? `?${new URLSearchParams({ departmentId }).toString()}` : ''
  return api(`${SETTINGS}/queues${suffix}`) as Promise<OmniQueue[]>
}

export function createQueue(api: ApiRequest, input: OmniQueueInput): Promise<OmniQueue> {
  return api(`${SETTINGS}/queues`, { method: 'POST', body: input }) as Promise<OmniQueue>
}

export function updateQueue(
  api: ApiRequest,
  id: string,
  patch: Partial<Pick<OmniQueue, 'name' | 'isDefault' | 'isActive'>>,
): Promise<OmniQueue> {
  return api(`${SETTINGS}/queues/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: patch,
  }) as Promise<OmniQueue>
}

export function fetchQueueMembers(api: ApiRequest, queueId: string): Promise<OmniQueueMember[]> {
  return api(`${SETTINGS}/queues/${encodeURIComponent(queueId)}/members`) as Promise<
    OmniQueueMember[]
  >
}

// Membros = incremental (add/remove um a um). A tela faz o diff e dispara N chamadas —
// nunca um PUT de conjunto (lost update entre supervisores).
export function addQueueMember(
  api: ApiRequest,
  queueId: string,
  userId: string,
): Promise<OmniQueueMember> {
  return api(`${SETTINGS}/queues/${encodeURIComponent(queueId)}/members`, {
    method: 'POST',
    body: { userId },
  }) as Promise<OmniQueueMember>
}

export function removeQueueMember(api: ApiRequest, queueId: string, userId: string): Promise<void> {
  return api(
    `${SETTINGS}/queues/${encodeURIComponent(queueId)}/members/${encodeURIComponent(userId)}`,
    {
      method: 'DELETE',
    },
  ) as Promise<void>
}

export function fetchRoutingRules(api: ApiRequest): Promise<OmniRoutingRule[]> {
  return api(`${SETTINGS}/routing-rules`) as Promise<OmniRoutingRule[]>
}

export function createRoutingRule(
  api: ApiRequest,
  input: OmniRoutingRuleInput,
): Promise<OmniRoutingRule> {
  return api(`${SETTINGS}/routing-rules`, {
    method: 'POST',
    body: input,
  }) as Promise<OmniRoutingRule>
}

export function updateRoutingRule(
  api: ApiRequest,
  id: string,
  patch: Partial<OmniRoutingRuleInput>,
): Promise<OmniRoutingRule> {
  return api(`${SETTINGS}/routing-rules/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: patch,
  }) as Promise<OmniRoutingRule>
}

export function deleteRoutingRule(api: ApiRequest, id: string): Promise<void> {
  return api(`${SETTINGS}/routing-rules/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  }) as Promise<void>
}

// Reordenar = uma transacao (tudo ou nada). Envia a ordem completa de ids; o back
// reescreve as prioridades e devolve a lista ja ordenada.
export function reorderRoutingRules(
  api: ApiRequest,
  ruleIds: string[],
): Promise<OmniRoutingRule[]> {
  return api(`${SETTINGS}/routing-rules/order`, {
    method: 'PUT',
    body: { ruleIds },
  }) as Promise<OmniRoutingRule[]>
}

export function fetchHandoffPolicies(api: ApiRequest): Promise<OmniHandoffPolicy[]> {
  return api(`${SETTINGS}/handoff-policies`) as Promise<OmniHandoffPolicy[]>
}

export function createHandoffPolicy(
  api: ApiRequest,
  input: OmniHandoffPolicyInput,
): Promise<OmniHandoffPolicy> {
  return api(`${SETTINGS}/handoff-policies`, {
    method: 'POST',
    body: input,
  }) as Promise<OmniHandoffPolicy>
}

export function updateHandoffPolicy(
  api: ApiRequest,
  id: string,
  patch: Partial<OmniHandoffPolicyInput>,
): Promise<OmniHandoffPolicy> {
  return api(`${SETTINGS}/handoff-policies/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: patch,
  }) as Promise<OmniHandoffPolicy>
}

export function deleteHandoffPolicy(api: ApiRequest, id: string): Promise<void> {
  return api(`${SETTINGS}/handoff-policies/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  }) as Promise<void>
}

// ============================================================================
// Agente de IA (perm omnichannel.agents.manage)
// ============================================================================

const AGENTS = `${BASE}/agents`

export function fetchMediaAnalyses(
  api: ApiRequest,
  conversationId: string,
  messageId: string,
): Promise<OmniMediaAnalysis[]> {
  return api(
    `${BASE}/conversations/${encodeURIComponent(conversationId)}/messages/${encodeURIComponent(messageId)}/media/analyses`,
    { dedupe: false },
  ) as Promise<OmniMediaAnalysis[]>
}

export function fetchAgents(api: ApiRequest): Promise<OmniAgent[]> {
  return api(AGENTS) as Promise<OmniAgent[]>
}

export function createAgent(api: ApiRequest, input: OmniAgentInput): Promise<OmniAgent> {
  return api(AGENTS, { method: 'POST', body: input }) as Promise<OmniAgent>
}

export function fetchAgent(api: ApiRequest, id: string): Promise<OmniAgent> {
  return api(`${AGENTS}/${encodeURIComponent(id)}`) as Promise<OmniAgent>
}

// Patch do agente. providerKey nao-nil grava/limpa a chave CIFRADA; a resposta so
// devolve {set,last4}. A chave crua nunca vai a log nem a localStorage.
export function updateAgent(
  api: ApiRequest,
  id: string,
  patch: { name?: string; enabled?: boolean; providerKey?: string },
): Promise<OmniAgent> {
  return api(`${AGENTS}/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: patch,
  }) as Promise<OmniAgent>
}

export function fetchAgentProviderKeys(
  api: ApiRequest,
  id: string,
): Promise<OmniProviderKeyStatusView> {
  return api(`${AGENTS}/${encodeURIComponent(id)}/provider-keys`, {
    dedupe: false,
  }) as Promise<OmniProviderKeyStatusView>
}

export function putAgentProviderKey(
  api: ApiRequest,
  id: string,
  provider: string,
  apiKey: string,
): Promise<OmniProviderKeyStatusView> {
  return api(`${AGENTS}/${encodeURIComponent(id)}/provider-keys/${encodeURIComponent(provider)}`, {
    method: 'PUT',
    body: { apiKey },
  }) as Promise<OmniProviderKeyStatusView>
}

export function clearAgentProviderKey(
  api: ApiRequest,
  id: string,
  provider: string,
): Promise<OmniProviderKeyStatusView> {
  return api(`${AGENTS}/${encodeURIComponent(id)}/provider-keys/${encodeURIComponent(provider)}`, {
    method: 'DELETE',
  }) as Promise<OmniProviderKeyStatusView>
}

export function fetchAgentVersions(api: ApiRequest, id: string): Promise<OmniAgentVersion[]> {
  return api(`${AGENTS}/${encodeURIComponent(id)}/versions`) as Promise<OmniAgentVersion[]>
}

export function fetchCollectFields(api: ApiRequest, id: string): Promise<OmniCollectField[]> {
  return api(`${AGENTS}/${encodeURIComponent(id)}/collect-fields`) as Promise<OmniCollectField[]>
}

export function fetchAiToolBindings(
  api: ApiRequest,
  agentId: string,
): Promise<OmniAiToolBinding[]> {
  return api(`${AGENTS}/${encodeURIComponent(agentId)}/tool-bindings`) as Promise<
    OmniAiToolBinding[]
  >
}

export function createAiToolBinding(
  api: ApiRequest,
  agentId: string,
  input: OmniAiToolBindingInput,
): Promise<OmniAiToolBinding> {
  return api(`${AGENTS}/${encodeURIComponent(agentId)}/tool-bindings`, {
    method: 'POST',
    body: input,
  }) as Promise<OmniAiToolBinding>
}

export function updateAiToolBinding(
  api: ApiRequest,
  agentId: string,
  bindingId: string,
  patch: Partial<OmniAiToolBindingInput>,
): Promise<OmniAiToolBinding> {
  return api(
    `${AGENTS}/${encodeURIComponent(agentId)}/tool-bindings/${encodeURIComponent(bindingId)}`,
    {
      method: 'PATCH',
      body: patch,
    },
  ) as Promise<OmniAiToolBinding>
}

export function disableAiToolBinding(
  api: ApiRequest,
  agentId: string,
  bindingId: string,
): Promise<void> {
  return api(
    `${AGENTS}/${encodeURIComponent(agentId)}/tool-bindings/${encodeURIComponent(bindingId)}`,
    {
      method: 'DELETE',
    },
  ) as Promise<void>
}

export function fetchAiToolRuns(
  api: ApiRequest,
  agentId: string,
  options: { status?: OmniAiToolRun['status']; limit?: number } = {},
): Promise<OmniAiToolRun[]> {
  const query = new URLSearchParams()
  if (options.status) query.set('status', options.status)
  if (options.limit) query.set('limit', String(options.limit))
  const suffix = query.toString() ? `?${query.toString()}` : ''
  return api(`${AGENTS}/${encodeURIComponent(agentId)}/tool-runs${suffix}`, {
    dedupe: false,
  }) as Promise<OmniAiToolRun[]>
}

export function fetchAiToolApprovals(
  api: ApiRequest,
  agentId: string,
  limit = 30,
): Promise<OmniAiToolApproval[]> {
  const query = new URLSearchParams({ limit: String(limit) }).toString()
  return api(`${AGENTS}/${encodeURIComponent(agentId)}/tool-approvals?${query}`, {
    dedupe: false,
  }) as Promise<OmniAiToolApproval[]>
}

export function approveAiToolApproval(
  api: ApiRequest,
  agentId: string,
  approvalId: string,
): Promise<OmniAiToolApproval> {
  return api(
    `${AGENTS}/${encodeURIComponent(agentId)}/tool-approvals/${encodeURIComponent(approvalId)}/approve`,
    { method: 'POST' },
  ) as Promise<OmniAiToolApproval>
}

export function rejectAiToolApproval(
  api: ApiRequest,
  agentId: string,
  approvalId: string,
): Promise<OmniAiToolApproval> {
  return api(
    `${AGENTS}/${encodeURIComponent(agentId)}/tool-approvals/${encodeURIComponent(approvalId)}/reject`,
    { method: 'POST' },
  ) as Promise<OmniAiToolApproval>
}

export function fetchKnowledgeBases(api: ApiRequest): Promise<OmniKnowledgeBase[]> {
  return api(`${BASE}/knowledge-bases`) as Promise<OmniKnowledgeBase[]>
}

export function createKnowledgeBase(
  api: ApiRequest,
  input: { name: string; isEnabled?: boolean; searchConfig?: Record<string, unknown> },
): Promise<OmniKnowledgeBase> {
  return api(`${BASE}/knowledge-bases`, {
    method: 'POST',
    body: input,
  }) as Promise<OmniKnowledgeBase>
}

export function updateKnowledgeBase(
  api: ApiRequest,
  id: string,
  patch: Partial<{ name: string; isEnabled: boolean; searchConfig: Record<string, unknown> }>,
): Promise<OmniKnowledgeBase> {
  return api(`${BASE}/knowledge-bases/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: patch,
  }) as Promise<OmniKnowledgeBase>
}

export function fetchKnowledgeDocuments(
  api: ApiRequest,
  baseId: string,
): Promise<OmniKnowledgeDocument[]> {
  return api(`${BASE}/knowledge-bases/${encodeURIComponent(baseId)}/documents`) as Promise<
    OmniKnowledgeDocument[]
  >
}

export function createKnowledgeDocument(
  api: ApiRequest,
  baseId: string,
  input: {
    sourceRef: string
    title?: string
    checksum: string
    version?: number
    metadata?: Record<string, unknown>
  },
): Promise<OmniKnowledgeDocument> {
  return api(`${BASE}/knowledge-bases/${encodeURIComponent(baseId)}/documents`, {
    method: 'POST',
    body: input,
  }) as Promise<OmniKnowledgeDocument>
}

export function updateKnowledgeDocument(
  api: ApiRequest,
  baseId: string,
  documentId: string,
  patch: Partial<{
    title: string
    status: OmniKnowledgeDocument['status']
    metadata: Record<string, unknown>
    error: string
  }>,
): Promise<OmniKnowledgeDocument> {
  return api(
    `${BASE}/knowledge-bases/${encodeURIComponent(baseId)}/documents/${encodeURIComponent(documentId)}`,
    { method: 'PATCH', body: patch },
  ) as Promise<OmniKnowledgeDocument>
}

export function replaceKnowledgeDocumentChunks(
  api: ApiRequest,
  baseId: string,
  documentId: string,
  chunks: OmniKnowledgeChunkInput[],
): Promise<void> {
  return api(
    `${BASE}/knowledge-bases/${encodeURIComponent(baseId)}/documents/${encodeURIComponent(documentId)}/chunks`,
    {
      method: 'POST',
      body: { chunks },
    },
  ) as Promise<void>
}

export function fetchAiKnowledgeBindings(
  api: ApiRequest,
  agentId: string,
): Promise<OmniAiKnowledgeBinding[]> {
  return api(`${AGENTS}/${encodeURIComponent(agentId)}/knowledge-bindings`) as Promise<
    OmniAiKnowledgeBinding[]
  >
}

export function createAiKnowledgeBinding(
  api: ApiRequest,
  agentId: string,
  input: { knowledgeBaseId: string; isEnabled?: boolean; topK?: number; minScore?: number },
): Promise<OmniAiKnowledgeBinding> {
  return api(`${AGENTS}/${encodeURIComponent(agentId)}/knowledge-bindings`, {
    method: 'POST',
    body: input,
  }) as Promise<OmniAiKnowledgeBinding>
}

export function updateAiKnowledgeBinding(
  api: ApiRequest,
  agentId: string,
  bindingId: string,
  patch: Partial<{ isEnabled: boolean; topK: number; minScore: number }>,
): Promise<OmniAiKnowledgeBinding> {
  return api(
    `${AGENTS}/${encodeURIComponent(agentId)}/knowledge-bindings/${encodeURIComponent(bindingId)}`,
    { method: 'PATCH', body: patch },
  ) as Promise<OmniAiKnowledgeBinding>
}

export function disableAiKnowledgeBinding(
  api: ApiRequest,
  agentId: string,
  bindingId: string,
): Promise<void> {
  return api(
    `${AGENTS}/${encodeURIComponent(agentId)}/knowledge-bindings/${encodeURIComponent(bindingId)}`,
    { method: 'DELETE' },
  ) as Promise<void>
}

// Simulador (C9.7). Grava ai_runs e consome o limite mensal — chama o modelo de verdade
// (o custo aparece em `usage`). Limite estourado => 409 acionavel. NAO envia mensagem.
export function simulateAgent(
  api: ApiRequest,
  id: string,
  input: OmniSimulateInput,
): Promise<OmniSimulateResult> {
  return api(`${AGENTS}/${encodeURIComponent(id)}/simulate`, {
    method: 'POST',
    body: input,
  }) as Promise<OmniSimulateResult>
}
