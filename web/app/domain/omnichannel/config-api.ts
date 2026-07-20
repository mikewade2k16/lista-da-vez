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
  OmniAgentVersionInput,
  OmniCapabilities,
  OmniCollectField,
  OmniCredentialStatus,
  OmniDepartment,
  OmniDepartmentInput,
  OmniInstance,
  OmniInstanceManagement,
  OmniInstanceWriteInput,
  OmniQueue,
  OmniQueueInput,
  OmniQueueMember,
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

// Capabilities por numero. GAP conhecido: o back ainda NAO expoe este endpoint
// (needsWiring). Ate existir, a tela DEGRADA: capability desconhecida = ausente. Por
// isso qualquer falha (404 incluso) devolve null, e a UI nao oferece o que nao confirma.
export function fetchCapabilities(api: ApiRequest, id: string): Promise<OmniCapabilities | null> {
  return (
    api(`${WA}/instances/${encodeURIComponent(id)}/capabilities`, {
      dedupe: false,
    }) as Promise<OmniCapabilities>
  )
    .then((caps) => caps)
    .catch(() => null)
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

// ============================================================================
// Agente de IA (perm omnichannel.agents.manage)
// ============================================================================

const AGENTS = `${BASE}/agents`

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

export function fetchAgentVersions(api: ApiRequest, id: string): Promise<OmniAgentVersion[]> {
  return api(`${AGENTS}/${encodeURIComponent(id)}/versions`) as Promise<OmniAgentVersion[]>
}

export function createAgentVersion(
  api: ApiRequest,
  id: string,
  input: OmniAgentVersionInput,
): Promise<OmniAgentVersion> {
  return api(`${AGENTS}/${encodeURIComponent(id)}/versions`, {
    method: 'POST',
    body: input,
  }) as Promise<OmniAgentVersion>
}

// Publish: versao no path (POST /versions/{v}/publish). Torna a versao ativa e imutavel.
export function publishAgentVersion(
  api: ApiRequest,
  id: string,
  version: number,
): Promise<OmniAgent> {
  return api(`${AGENTS}/${encodeURIComponent(id)}/versions/${version}/publish`, {
    method: 'POST',
  }) as Promise<OmniAgent>
}

// Rollback: repointa active_version_id para uma versao ja publicada.
export function rollbackAgent(api: ApiRequest, id: string, versionId: string): Promise<OmniAgent> {
  return api(`${AGENTS}/${encodeURIComponent(id)}/rollback`, {
    method: 'POST',
    body: { versionId },
  }) as Promise<OmniAgent>
}

export function fetchCollectFields(api: ApiRequest, id: string): Promise<OmniCollectField[]> {
  return api(`${AGENTS}/${encodeURIComponent(id)}/collect-fields`) as Promise<OmniCollectField[]>
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
