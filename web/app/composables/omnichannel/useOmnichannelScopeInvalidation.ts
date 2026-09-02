import { computed } from 'vue'
import type { ApiRequest } from '~/domain/omnichannel/config-api'
import {
  resetInstanceHistory,
  type OmniInstanceHistoryResetInput,
  type OmniInstanceHistoryResetResult,
} from '~/domain/omnichannel/instance-admin-api'
import type { OmniInstance } from '~/domain/omnichannel/config-types'
import { useAuthStore } from '~/stores/auth'
import { useUiStore } from '~/stores/ui'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import { useCoreAccountStore } from '../../../layers/core/stores/account'

export const HISTORY_RESET_ACTION_LABEL = 'Limpar histórico visível desta conexão'

export type OmnichannelInvalidationReason =
  | 'message_changed'
  | 'history_reset'
  | 'access_scope_changed'

export interface OmnichannelScopeInvalidationEvent {
  eventId: string
  reason: OmnichannelInvalidationReason
  occurredAt: string
  accountId: string
  source: 'local' | 'realtime'
  instanceId?: string
  instanceScopeKey?: string
  hiddenBefore?: string
  resetRevision?: number
}

interface OmnichannelScopeInvalidationState {
  sequence: number
  scopeEpoch: number
  dataEpoch: number
  lastEvent: OmnichannelScopeInvalidationEvent | null
  seenEventKeys: string[]
  busyResetKeys: string[]
  refetchDepthByAccount: Record<string, number>
}

export interface RealtimeInvalidationPayload {
  eventId: string
  reason: OmnichannelInvalidationReason
  occurredAt: string
}

export interface ParsedRealtimeInvalidation extends RealtimeInvalidationPayload {
  accountId: string
}

export type InstanceHistoryResetActionResult =
  | { status: 'success'; result: OmniInstanceHistoryResetResult }
  | {
      status:
        | 'cancelled'
        | 'invalid_confirmation'
        | 'busy'
        | 'forbidden'
        | 'conflict'
        | 'scope_changed'
        | 'error'
    }

interface ResetPromptOptions {
  title: string
  message: string
  inputLabel: string
  inputPlaceholder: string
  confirmLabel: string
  cancelLabel: string
  initialValue: string
  required: boolean
}

interface InstanceHistoryResetActionDependencies {
  accountId: () => string
  accountLabel: () => string
  scopeGeneration: () => number
  prompt: (options: ResetPromptOptions) => Promise<{ confirmed?: boolean; value?: unknown }>
  reset: (
    id: string,
    input: OmniInstanceHistoryResetInput,
  ) => Promise<OmniInstanceHistoryResetResult>
  tryStart: (accountId: string, instanceId: string) => boolean
  finish: (accountId: string, instanceId: string) => void
  publish: (
    accountId: string,
    instanceId: string,
    instanceScopeKey: string,
    result: OmniInstanceHistoryResetResult,
  ) => void
  success: (message: string) => void
  error: (message: string) => void
  statusCode: (cause: unknown) => number
  errorMessage: (cause: unknown, fallback: string) => string
}

const MAX_SEEN_EVENTS = 200
const ALLOWED_INVALIDATION_REASONS = new Set<OmnichannelInvalidationReason>([
  'message_changed',
  'history_reset',
  'access_scope_changed',
])

function normalizeRequiredText(value: unknown): string {
  return typeof value === 'string' ? value.trim() : ''
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value)
}

export function canResetInstanceHistory(instance: OmniInstance | null | undefined): boolean {
  return instance?.myCapabilities?.resetHistory === true
}

export function rememberInvalidationEvent(
  seenEventKeys: string[],
  eventKey: string,
  maxEntries = MAX_SEEN_EVENTS,
): { duplicate: boolean; nextSeenEventKeys: string[] } {
  if (seenEventKeys.includes(eventKey)) {
    return { duplicate: true, nextSeenEventKeys: seenEventKeys }
  }
  return {
    duplicate: false,
    nextSeenEventKeys: [...seenEventKeys, eventKey].slice(-Math.max(1, maxEntries)),
  }
}

export function parseRealtimeInvalidationEnvelope(
  envelope: unknown,
): ParsedRealtimeInvalidation | null {
  if (!isRecord(envelope) || envelope.type !== 'omnichannel.invalidate') return null

  const accountId = normalizeRequiredText(envelope.accountId)
  const payload = isRecord(envelope.payload) ? envelope.payload : null
  if (!accountId || !payload) return null

  const payloadKeys = Object.keys(payload).sort()
  if (payloadKeys.join(',') !== 'eventId,occurredAt,reason') return null

  const eventId = normalizeRequiredText(payload.eventId)
  const occurredAt = normalizeRequiredText(payload.occurredAt)
  const reason = normalizeRequiredText(payload.reason) as OmnichannelInvalidationReason
  if (
    !eventId ||
    !occurredAt ||
    !Number.isFinite(Date.parse(occurredAt)) ||
    !ALLOWED_INVALIDATION_REASONS.has(reason)
  ) {
    return null
  }

  return { accountId, eventId, occurredAt, reason }
}

export function buildHistoryResetPromptMessage(
  accountLabel: string,
  instance: Pick<
    OmniInstance,
    'instanceName' | 'displayName' | 'phoneNumber' | 'provider'
  >,
): string {
  return [
    `Conta ativa: ${accountLabel || 'não identificada'}.`,
    `instanceName: ${instance.instanceName}.`,
    `Nome de exibição: ${instance.displayName || 'não informado'}.`,
    `Telefone: ${instance.phoneNumber || 'não informado'}.`,
    `Provider: ${instance.provider}.`,
    'A sessão não será desconectada e os contatos serão preservados.',
    `Digite exatamente ${instance.instanceName} para confirmar.`,
  ].join(' ')
}

export function createInstanceHistoryResetAction(
  dependencies: InstanceHistoryResetActionDependencies,
) {
  return async function requestInstanceHistoryReset(
    instance: OmniInstance,
    options: { rehydrate?: () => Promise<void> | void } = {},
  ): Promise<InstanceHistoryResetActionResult> {
    if (!canResetInstanceHistory(instance)) {
      dependencies.error('Você não tem permissão para limpar o histórico desta conexão.')
      return { status: 'forbidden' }
    }

    const snapshot = Object.freeze({
      accountId: dependencies.accountId(),
      accountLabel: dependencies.accountLabel(),
      scopeGeneration: dependencies.scopeGeneration(),
      id: instance.id,
      instanceName: instance.instanceName.trim(),
      displayName: instance.displayName,
      phoneNumber: instance.phoneNumber,
      provider: instance.provider,
      historyResetRevision: instance.historyResetRevision,
    })
    const isSameScope = () =>
      dependencies.accountId() === snapshot.accountId &&
      dependencies.scopeGeneration() === snapshot.scopeGeneration

    if (!dependencies.tryStart(snapshot.accountId, snapshot.id)) return { status: 'busy' }

    try {
      const answer = await dependencies.prompt({
        title: HISTORY_RESET_ACTION_LABEL,
        message: buildHistoryResetPromptMessage(snapshot.accountLabel, snapshot),
        inputLabel: 'Digite o instanceName exato',
        inputPlaceholder: snapshot.instanceName,
        confirmLabel: HISTORY_RESET_ACTION_LABEL,
        cancelLabel: 'Cancelar',
        initialValue: '',
        required: true,
      })
      if (!isSameScope()) return { status: 'scope_changed' }
      if (!answer.confirmed) return { status: 'cancelled' }

      const expectedConfirmation = snapshot.instanceName
      const confirmation = String(answer.value ?? '').trim()
      if (confirmation !== expectedConfirmation) {
        dependencies.error('O instanceName digitado não corresponde exatamente à conexão.')
        return { status: 'invalid_confirmation' }
      }

      if (!isSameScope()) return { status: 'scope_changed' }
      const result = await dependencies.reset(snapshot.id, {
        confirmation: expectedConfirmation,
        expectedRevision: snapshot.historyResetRevision,
      })
      if (!isSameScope()) return { status: 'scope_changed' }

      dependencies.publish(snapshot.accountId, snapshot.id, snapshot.instanceName, result)
      const publishedGeneration = dependencies.scopeGeneration()
      const isSamePublishedScope = () =>
        dependencies.accountId() === snapshot.accountId &&
        dependencies.scopeGeneration() === publishedGeneration
      let rehydrateFailed = false
      if (isSamePublishedScope()) {
        try {
          await options.rehydrate?.()
        } catch {
          rehydrateFailed = true
          dependencies.error(
            'O histórico foi limpo, mas não foi possível atualizar os dados da conexão.',
          )
        }
      }
      if (!rehydrateFailed && isSamePublishedScope()) {
        dependencies.success('O histórico visível desta conexão foi limpo.')
      }
      return { status: 'success', result }
    } catch (cause) {
      if (!isSameScope()) return { status: 'scope_changed' }

      if (dependencies.statusCode(cause) === 409) {
        let rehydrated = false
        if (isSameScope()) {
          try {
            await options.rehydrate?.()
            rehydrated = true
          } catch {
            // A mutation não é repetida; o próximo carregamento autoritativo tentará novamente.
          }
        }
        if (isSameScope()) {
          dependencies.error(
            rehydrated
              ? 'A conexão foi atualizada por outra ação. Os dados foram recarregados; confirme novamente.'
              : 'A conexão foi atualizada por outra ação. Atualize os dados e confirme novamente.',
          )
        }
        return { status: 'conflict' }
      }

      dependencies.error(
        dependencies.errorMessage(cause, 'Não foi possível limpar o histórico desta conexão.'),
      )
      return { status: 'error' }
    } finally {
      dependencies.finish(snapshot.accountId, snapshot.id)
    }
  }
}

function statusCodeOf(cause: unknown): number {
  if (!isRecord(cause)) return 0
  const response = isRecord(cause.response) ? cause.response : null
  const value = Number(cause.statusCode ?? cause.status ?? response?.status ?? 0)
  return Number.isFinite(value) ? value : 0
}

export function useOmnichannelScopeInvalidation() {
  const auth = useAuthStore()
  const accountStore = useCoreAccountStore()
  const ui = useUiStore()
  const runtimeConfig = useRuntimeConfig()
  const api = createApiRequest(runtimeConfig, () => auth.accessToken) as ApiRequest
  const state = useState<OmnichannelScopeInvalidationState>(
    'omnichannel-scope-invalidation-v1',
    () => ({
      sequence: 0,
      scopeEpoch: 0,
      dataEpoch: 0,
      lastEvent: null,
      seenEventKeys: [],
      busyResetKeys: [],
      refetchDepthByAccount: {},
    }),
  )

  const accountId = computed(
    () => accountStore.activeAccountId || String(auth.activeTenantId || '').trim(),
  )
  const accountLabel = computed(
    () => accountStore.activeAccount?.name || accountStore.activeAccountId || 'Conta ativa',
  )
  const sequence = computed(() => state.value.sequence)
  const lastEvent = computed(() => state.value.lastEvent)
  const scopeGeneration = computed(() => state.value.scopeEpoch ?? 0)
  const dataGeneration = computed(() => state.value.dataEpoch ?? 0)
  const isScopeRefetching = computed(
    () => (state.value.refetchDepthByAccount[accountId.value] ?? 0) > 0,
  )

  function busyKey(busyAccountId: string, instanceId: string): string {
    return `${busyAccountId}:${instanceId}`
  }

  function isResettingInstance(instanceId: string): boolean {
    return state.value.busyResetKeys.includes(busyKey(accountId.value, instanceId))
  }

  function tryStartReset(busyAccountId: string, instanceId: string): boolean {
    const key = busyKey(busyAccountId, instanceId)
    if (state.value.busyResetKeys.includes(key)) return false
    state.value = {
      ...state.value,
      busyResetKeys: [...state.value.busyResetKeys, key],
    }
    return true
  }

  function finishReset(busyAccountId: string, instanceId: string): void {
    const key = busyKey(busyAccountId, instanceId)
    state.value = {
      ...state.value,
      busyResetKeys: state.value.busyResetKeys.filter((entry) => entry !== key),
    }
  }

  function publish(event: OmnichannelScopeInvalidationEvent): boolean {
    const eventKey = `${event.accountId}:${event.eventId}`
    const remembered = rememberInvalidationEvent(state.value.seenEventKeys, eventKey)
    if (remembered.duplicate) return false

    state.value = {
      ...state.value,
      sequence: state.value.sequence + 1,
      dataEpoch: (state.value.dataEpoch ?? 0) + 1,
      lastEvent: event,
      seenEventKeys: remembered.nextSeenEventKeys,
    }
    return true
  }

  function publishRealtimeInvalidation(event: ParsedRealtimeInvalidation): boolean {
    return publish({ ...event, source: 'realtime' })
  }

  function publishLocalHistoryReset(
    eventAccountId: string,
    instanceId: string,
    instanceScopeKey: string,
    result: OmniInstanceHistoryResetResult,
  ): boolean {
    return publish({
      accountId: eventAccountId,
      eventId: `local-history-reset:${instanceId}:${result.resetRevision}`,
      reason: 'history_reset',
      occurredAt: result.hiddenBefore,
      source: 'local',
      instanceId,
      instanceScopeKey,
      hiddenBefore: result.hiddenBefore,
      resetRevision: result.resetRevision,
    })
  }

  function publishLocalAccessChange(
    eventAccountId: string,
    instanceId: string,
    accessRevision: number,
  ): boolean {
    return publish({
      accountId: eventAccountId,
      eventId: `local-access-change:${instanceId}:${accessRevision}`,
      reason: 'access_scope_changed',
      occurredAt: new Date().toISOString(),
      source: 'local',
      instanceId,
    })
  }

  function beginScopeRefetch(): string {
    const key = accountId.value
    state.value = {
      ...state.value,
      refetchDepthByAccount: {
        ...state.value.refetchDepthByAccount,
        [key]: (state.value.refetchDepthByAccount[key] ?? 0) + 1,
      },
    }
    return key
  }

  function advanceScopeGeneration(): void {
    state.value = {
      ...state.value,
      scopeEpoch: (state.value.scopeEpoch ?? 0) + 1,
      dataEpoch: (state.value.dataEpoch ?? 0) + 1,
    }
  }

  function advanceDataGeneration(): void {
    state.value = {
      ...state.value,
      dataEpoch: (state.value.dataEpoch ?? 0) + 1,
    }
  }

  function finishScopeRefetch(refetchAccountId = accountId.value): void {
    const key = refetchAccountId
    state.value = {
      ...state.value,
      refetchDepthByAccount: {
        ...state.value.refetchDepthByAccount,
        [key]: Math.max(0, (state.value.refetchDepthByAccount[key] ?? 0) - 1),
      },
    }
  }

  function isCurrentScopeEvent(event: OmnichannelScopeInvalidationEvent | null): boolean {
    return Boolean(event?.accountId && event.accountId === accountId.value)
  }

  const requestInstanceHistoryReset = createInstanceHistoryResetAction({
    accountId: () => accountId.value,
    accountLabel: () => accountLabel.value,
    scopeGeneration: () => scopeGeneration.value,
    prompt: (options) => ui.prompt(options),
    reset: (id, input) => resetInstanceHistory(api, id, input),
    tryStart: tryStartReset,
    finish: finishReset,
    publish: (eventAccountId, instanceId, instanceScopeKey, result) => {
      publishLocalHistoryReset(eventAccountId, instanceId, instanceScopeKey, result)
    },
    success: (message) => {
      ui.success(message)
    },
    error: (message) => {
      ui.error(message)
    },
    statusCode: statusCodeOf,
    errorMessage: (cause, fallback) => getApiErrorMessage(cause, fallback),
  })

  return {
    accountId,
    accountLabel,
    sequence,
    lastEvent,
    scopeGeneration,
    dataGeneration,
    isScopeRefetching,
    isResettingInstance,
    requestInstanceHistoryReset,
    publishRealtimeInvalidation,
    publishLocalAccessChange,
    advanceScopeGeneration,
    advanceDataGeneration,
    beginScopeRefetch,
    finishScopeRefetch,
    isCurrentScopeEvent,
  }
}
