import { computed, ref } from 'vue'
import {
  fetchInstanceAccess,
  putInstanceAccess,
  type ApiRequest,
} from '~/domain/omnichannel/config-api'
import type {
  OmniInstanceAccessAdmin,
  OmniInstanceAccessPolicy,
  OmniInstanceGrantLevel,
} from '~/domain/omnichannel/config-types'

export type InstanceAccessEditorStatus = 'idle' | 'loading' | 'ready' | 'saving' | 'error'
export type InstanceAccessSaveResult = 'saved' | 'busy' | 'invalid' | 'conflict' | 'error'

function statusCodeOf(cause: unknown): number {
  if (!cause || typeof cause !== 'object' || Array.isArray(cause)) return 0
  const candidate = cause as {
    status?: unknown
    statusCode?: unknown
    response?: { status?: unknown }
  }
  const value = Number(candidate.statusCode ?? candidate.status ?? candidate.response?.status ?? 0)
  return Number.isFinite(value) ? value : 0
}

export function useOmnichannelInstanceAccessEditor(options: {
  api: ApiRequest
  instanceId: () => string
}) {
  const status = ref<InstanceAccessEditorStatus>('idle')
  const authoritative = ref<OmniInstanceAccessAdmin | null>(null)
  const accessPolicy = ref<OmniInstanceAccessPolicy>('RESTRICTED')
  const responsibleUserId = ref('')
  const grantLevels = ref<Record<string, OmniInstanceGrantLevel>>({})
  const errorMessage = ref('')

  const activeGrantCount = computed(() => Object.keys(grantLevels.value).length)
  const managerCount = computed(
    () => Object.values(grantLevels.value).filter((level) => level === 'manage').length,
  )
  const responsibleHasManage = computed(
    () =>
      Boolean(responsibleUserId.value) &&
      grantLevels.value[responsibleUserId.value] === 'manage',
  )
  const validationError = computed(() => {
    if (!responsibleUserId.value) return 'Selecione o responsavel principal.'
    if (!responsibleHasManage.value) return 'O responsavel principal precisa ter nivel manage.'
    if (managerCount.value < 1) return 'A conexao precisa manter ao menos um gestor.'
    return ''
  })

  function hydrate(view: OmniInstanceAccessAdmin): void {
    authoritative.value = view
    accessPolicy.value = view.accessPolicy
    responsibleUserId.value = view.responsibleUserId || ''
    grantLevels.value = Object.fromEntries(
      (view.grants || [])
        .filter((grant) => grant.isActive)
        .map((grant) => [grant.userId, grant.accessLevel]),
    )
  }

  async function load(): Promise<boolean> {
    const instanceId = options.instanceId()
    status.value = 'loading'
    errorMessage.value = ''
    authoritative.value = null
    grantLevels.value = {}
    try {
      const view = await fetchInstanceAccess(options.api, instanceId)
      if (options.instanceId() !== instanceId) return false
      hydrate(view)
      status.value = 'ready'
      return true
    } catch {
      if (options.instanceId() !== instanceId) return false
      status.value = 'error'
      errorMessage.value = 'Nao foi possivel carregar os acessos desta conexao.'
      return false
    }
  }

  function setGrant(userId: string, level: OmniInstanceGrantLevel | ''): void {
    grantLevels.value = level
      ? { ...grantLevels.value, [userId]: level }
      : Object.fromEntries(Object.entries(grantLevels.value).filter(([id]) => id !== userId))
  }

  function setResponsible(userId: string): void {
    responsibleUserId.value = userId
    if (userId) setGrant(userId, 'manage')
  }

  async function save(): Promise<InstanceAccessSaveResult> {
    if (status.value === 'saving') return 'busy'
    if (!authoritative.value || validationError.value) return 'invalid'

    const instanceId = options.instanceId()
    status.value = 'saving'
    errorMessage.value = ''
    try {
      const view = await putInstanceAccess(options.api, instanceId, {
        accessRevision: authoritative.value.accessRevision,
        accessPolicy: accessPolicy.value,
        responsibleUserId: responsibleUserId.value,
        grants: Object.entries(grantLevels.value)
          .sort(([left], [right]) => left.localeCompare(right))
          .map(([userId, accessLevel]) => ({ userId, accessLevel })),
      })
      if (options.instanceId() !== instanceId) return 'error'
      hydrate(view)
      status.value = 'ready'
      return 'saved'
    } catch (cause) {
      if (options.instanceId() !== instanceId) return 'error'
      if (statusCodeOf(cause) === 409) {
        await load()
        errorMessage.value =
          'Os acessos foram alterados por outra acao. O estado atual foi recarregado; revise antes de salvar.'
        return 'conflict'
      }
      status.value = 'error'
      errorMessage.value = 'Nao foi possivel salvar os acessos desta conexao.'
      authoritative.value = null
      grantLevels.value = {}
      return 'error'
    }
  }

  return {
    status,
    authoritative,
    accessPolicy,
    responsibleUserId,
    grantLevels,
    errorMessage,
    activeGrantCount,
    managerCount,
    responsibleHasManage,
    validationError,
    load,
    save,
    setGrant,
    setResponsible,
  }
}
