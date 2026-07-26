import { ref } from 'vue'
import {
  applyChannelClientBindingRepair,
  createChannelClientBinding,
  createChannelClientBindingRepairPreview,
  endChannelClientBinding,
  fetchChannelClientBindingExceptions,
  fetchChannelClientBindingPolicy,
  fetchChannelClientBindings,
  reassignChannelClientBinding,
  updateChannelClientBindingPolicy,
  type ChannelBindingChannel,
  type ChannelBindingMode,
  type ChannelClientBinding,
  type ChannelClientBindingException,
  type ChannelClientBindingPolicy,
  type ChannelClientBindingRepairJob,
  type CustomerIntelligenceFailurePolicy,
  type CustomerIntelligenceMode,
} from '~/domain/omnichannel/channel-client-bindings-api'
import { fetchInstagramAccounts } from '~/domain/omnichannel/instagram-api'
import type { OmniInstagramAccount } from '~/domain/omnichannel/config-types'
import { useAuthStore } from '~/stores/auth'
import { useUiStore } from '~/stores/ui'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'

function mutationKey(prefix: string): string {
  const random =
    typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function'
      ? crypto.randomUUID()
      : `${Date.now()}-${Math.random().toString(16).slice(2)}`
  return `${prefix}:${random}`
}

export function useChannelClientBindings() {
  const auth = useAuthStore()
  const ui = useUiStore()
  const runtimeConfig = useRuntimeConfig()
  const api = createApiRequest(runtimeConfig, () => auth.accessToken)

  const bindings = ref<ChannelClientBinding[]>([])
  const exceptions = ref<ChannelClientBindingException[]>([])
  const policy = ref<ChannelClientBindingPolicy | null>(null)
  const instagramAccounts = ref<OmniInstagramAccount[]>([])
  const lastRepair = ref<ChannelClientBindingRepairJob | null>(null)
  const loading = ref(false)
  const saving = ref(false)
  const error = ref('')

  async function load(): Promise<void> {
    loading.value = true
    error.value = ''
    try {
      const [page, exceptionPage, currentPolicy, instagram] = await Promise.all([
        fetchChannelClientBindings(api),
        fetchChannelClientBindingExceptions(api),
        fetchChannelClientBindingPolicy(api),
        fetchInstagramAccounts(api),
      ])
      bindings.value = page.items || []
      exceptions.value = exceptionPage.items || []
      policy.value = currentPolicy
      instagramAccounts.value = instagram
    } catch (cause) {
      error.value = getApiErrorMessage(
        cause,
        'Não foi possível carregar os vínculos de clientes por canal.',
      )
    } finally {
      loading.value = false
    }
  }

  async function createBinding(input: {
    clientAccountId: string
    channel: ChannelBindingChannel
    channelResourceId: string
    reason: string
  }): Promise<boolean> {
    if (saving.value) return false
    saving.value = true
    try {
      await createChannelClientBinding(api, {
        ...input,
        idempotencyKey: mutationKey('channel-binding:create'),
      })
      await load()
      ui.success('Canal vinculado ao cliente.')
      return true
    } catch (cause) {
      ui.error(getApiErrorMessage(cause, 'Não foi possível vincular este canal.'))
      return false
    } finally {
      saving.value = false
    }
  }

  async function reassignBinding(
    binding: ChannelClientBinding,
    targetClientAccountId: string,
    reason: string,
  ): Promise<boolean> {
    if (saving.value) return false
    saving.value = true
    try {
      await reassignChannelClientBinding(api, binding.id, {
        targetClientAccountId,
        effectiveAt: new Date().toISOString(),
        reason,
        expectedRevision: binding.revision,
        idempotencyKey: mutationKey('channel-binding:reassign'),
      })
      await load()
      ui.success('Canal reatribuído sem alterar o histórico.')
      return true
    } catch (cause) {
      ui.error(getApiErrorMessage(cause, 'Não foi possível reatribuir este canal.'))
      return false
    } finally {
      saving.value = false
    }
  }

  async function endBinding(binding: ChannelClientBinding, reason: string): Promise<boolean> {
    if (saving.value) return false
    saving.value = true
    try {
      await endChannelClientBinding(api, binding.id, {
        effectiveAt: new Date().toISOString(),
        reason,
        expectedRevision: binding.revision,
        idempotencyKey: mutationKey('channel-binding:end'),
      })
      await load()
      ui.success('Vínculo encerrado.')
      return true
    } catch (cause) {
      ui.error(
        getApiErrorMessage(
          cause,
          'Desative o recurso ou crie um sucessor antes de encerrar o vínculo.',
        ),
      )
      return false
    } finally {
      saving.value = false
    }
  }

  async function savePolicy(
    channelBindingMode: ChannelBindingMode,
    customerIntelligenceMode: CustomerIntelligenceMode,
    customerIntelligenceFailurePolicy: CustomerIntelligenceFailurePolicy,
  ): Promise<boolean> {
    if (!policy.value || saving.value) return false
    saving.value = true
    try {
      policy.value = await updateChannelClientBindingPolicy(api, {
        channelBindingMode,
        customerIntelligenceMode,
        customerIntelligenceFailurePolicy,
        expectedRevision: policy.value.revision,
      })
      ui.success('Política de integração salva.')
      return true
    } catch (cause) {
      ui.error(
        getApiErrorMessage(
          cause,
          'Não foi possível salvar. Confira módulos, vínculos ativos e revisão.',
        ),
      )
      return false
    } finally {
      saving.value = false
    }
  }

  async function repairBinding(binding: ChannelClientBinding, reason: string): Promise<boolean> {
    if (saving.value) return false
    saving.value = true
    try {
      const preview = await createChannelClientBindingRepairPreview(api, {
        bindingId: binding.id,
        watermark: new Date().toISOString(),
        reason,
        idempotencyKey: mutationKey('channel-binding:repair-preview'),
        includeClosed: false,
        confirmNoRetroactiveMove: true,
      })
      lastRepair.value = preview
      if (preview.eligibleCount === 0) {
        ui.success('Preview concluído: nenhuma conversa elegível para reparo.')
        return true
      }
      const applied = await applyChannelClientBindingRepair(api, {
        previewId: preview.id,
        previewChecksum: preview.previewChecksum,
        reason,
        idempotencyKey: mutationKey('channel-binding:repair-apply'),
        confirm: true,
      })
      lastRepair.value = applied
      await load()
      ui.success(`Reparo concluído: ${applied.repairedCount} conversa(s) atualizada(s).`)
      return true
    } catch (cause) {
      ui.error(getApiErrorMessage(cause, 'Não foi possível executar o reparo assistido.'))
      return false
    } finally {
      saving.value = false
    }
  }

  return {
    bindings,
    exceptions,
    policy,
    instagramAccounts,
    lastRepair,
    loading,
    saving,
    error,
    load,
    createBinding,
    reassignBinding,
    endBinding,
    savePolicy,
    repairBinding,
  }
}
