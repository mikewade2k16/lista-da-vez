<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import type { AutomationClientRef } from '~/domain/omnichannel/automation-api'
import type { OmniInstance } from '~/domain/omnichannel/config-types'
import type {
  ChannelBindingChannel,
  ChannelBindingMode,
  ChannelClientBinding,
  CustomerIntelligenceFailurePolicy,
  CustomerIntelligenceMode,
} from '~/domain/omnichannel/channel-client-bindings-api'
import { useChannelClientBindings } from '~/composables/omnichannel/useChannelClientBindings'

const props = defineProps<{
  clients: AutomationClientRef[]
  instances: OmniInstance[]
  canManage: boolean
}>()

const {
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
} = useChannelClientBindings()

const clientAccountId = ref('')
const channel = ref<ChannelBindingChannel>('WHATSAPP')
const channelResourceId = ref('')
const reason = ref('')
const selectedBindingId = ref('')
const targetClientAccountId = ref('')
const actionReason = ref('')
const channelBindingMode = ref<ChannelBindingMode>('shadow')
const customerIntelligenceMode = ref<CustomerIntelligenceMode>('off')
const customerIntelligenceFailurePolicy =
  ref<CustomerIntelligenceFailurePolicy>('retry_then_handoff')

const selectedBinding = computed(
  () => bindings.value.find((item) => item.id === selectedBindingId.value) || null,
)
const resourceOptions = computed(() =>
  channel.value === 'WHATSAPP'
    ? props.instances.map((item) => ({
        id: item.id,
        label: item.displayName || item.instanceName,
        active: item.isActive,
      }))
    : instagramAccounts.value.map((item) => ({
        id: item.id,
        label: item.displayName || item.username || item.igUserId,
        active: item.isActive,
      })),
)
const selectedResourceActive = computed(() => {
  const selected = selectedBinding.value
  if (!selected) return false
  if (selected.channel === 'WHATSAPP') {
    return (
      props.instances.find((item) => item.id === selected.channelResource.id)?.isActive ?? false
    )
  }
  return (
    instagramAccounts.value.find((item) => item.id === selected.channelResource.id)?.isActive ??
    false
  )
})
const canCreate = computed(
  () =>
    props.canManage &&
    Boolean(clientAccountId.value && channelResourceId.value && reason.value.trim()) &&
    !saving.value,
)

watch(channel, () => {
  channelResourceId.value = ''
})
watch(
  policy,
  (value) => {
    if (!value) return
    channelBindingMode.value = value.channelBindingMode
    customerIntelligenceMode.value = value.customerIntelligenceMode
    customerIntelligenceFailurePolicy.value = value.customerIntelligenceFailurePolicy
  },
  { immediate: true },
)

function selectBinding(binding: ChannelClientBinding): void {
  selectedBindingId.value = binding.id
  targetClientAccountId.value = binding.clientAccountId
  actionReason.value = ''
}

async function submitCreate(): Promise<void> {
  if (!canCreate.value) return
  const created = await createBinding({
    clientAccountId: clientAccountId.value,
    channel: channel.value,
    channelResourceId: channelResourceId.value,
    reason: reason.value.trim(),
  })
  if (created) {
    channelResourceId.value = ''
    reason.value = ''
  }
}

async function submitReassign(): Promise<void> {
  const binding = selectedBinding.value
  if (!binding || !targetClientAccountId.value || !actionReason.value.trim()) return
  const changed = await reassignBinding(
    binding,
    targetClientAccountId.value,
    actionReason.value.trim(),
  )
  if (changed) selectedBindingId.value = ''
}

async function submitEnd(): Promise<void> {
  const binding = selectedBinding.value
  if (!binding || !actionReason.value.trim()) return
  if (selectedResourceActive.value) return
  if (!window.confirm('Encerrar este vínculo? O histórico será preservado.')) return
  const ended = await endBinding(binding, actionReason.value.trim())
  if (ended) selectedBindingId.value = ''
}

async function submitRepair(): Promise<void> {
  const binding = selectedBinding.value
  if (!binding || !actionReason.value.trim()) return
  if (
    !window.confirm(
      'Gerar preview e aplicar somente conversas elegíveis, sem resposta humana e sem mover vínculos já resolvidos?',
    )
  ) {
    return
  }
  await repairBinding(binding, actionReason.value.trim())
}

onMounted(() => void load())
</script>

<template>
  <section class="binding-config">
    <header>
      <div>
        <p class="binding-config__eyebrow">Ownership operacional</p>
        <h3>Clientes por canal</h3>
        <p>
          O vínculo é independente da IA. Trocas criam um novo intervalo e preservam as conversas
          antigas.
        </p>
      </div>
      <button type="button" class="binding-config__ghost" :disabled="loading" @click="load">
        Atualizar
      </button>
    </header>

    <div class="binding-config__ownership-guide">
      <article>
        <strong>Número próprio do cliente</strong>
        <span>
          É criado na conta do cliente, recebe vínculo padrão com essa mesma conta e pode ser
          operado no portal do cliente.
        </span>
      </article>
      <article>
        <strong>Número da agência dedicado ao cliente</strong>
        <span>
          Continua pertencendo à conta da agência. O vínculo organiza CRM e roteamento, mas não
          concede acesso ao portal do cliente.
        </span>
      </article>
    </div>

    <p v-if="error" class="binding-config__error">{{ error }}</p>

    <div v-if="policy" class="binding-config__policy">
      <label>
        Validação do vínculo
        <select v-model="channelBindingMode" :disabled="!canManage || saving">
          <option value="legacy">Legado</option>
          <option value="shadow">Shadow — observa sem bloquear</option>
          <option value="enforced">Enforced — IA exige vínculo resolvido</option>
        </select>
      </label>
      <label>
        Customer Intelligence
        <select v-model="customerIntelligenceMode" :disabled="!canManage || saving">
          <option value="off">Desligada</option>
          <option value="shadow">Shadow — compara sem efeito</option>
          <option value="on">Ativa — decisão passa pelo Omnichannel</option>
        </select>
      </label>
      <label>
        Se a Inteligência falhar
        <select v-model="customerIntelligenceFailurePolicy" :disabled="!canManage || saving">
          <option value="retry_then_handoff">Tentar novamente e transferir</option>
          <option value="immediate_handoff">Transferir imediatamente</option>
          <option value="legacy_fallback">Usar motor legado</option>
        </select>
      </label>
      <button
        type="button"
        :disabled="!canManage || saving"
        @click="
          savePolicy(
            channelBindingMode,
            customerIntelligenceMode,
            customerIntelligenceFailurePolicy,
          )
        "
      >
        Salvar política
      </button>
      <small>Revisão {{ policy.revision }}. Prompts não podem alterar estes gates.</small>
    </div>

    <form class="binding-config__form" @submit.prevent="submitCreate">
      <label>
        Cliente
        <select v-model="clientAccountId" :disabled="!canManage || saving">
          <option value="">Selecione</option>
          <option v-for="client in clients" :key="client.id" :value="client.id">
            {{ client.name }}
          </option>
        </select>
      </label>
      <label>
        Canal
        <select v-model="channel" :disabled="!canManage || saving">
          <option value="WHATSAPP">WhatsApp</option>
          <option value="INSTAGRAM">Instagram</option>
        </select>
      </label>
      <label>
        Recurso
        <select v-model="channelResourceId" :disabled="!canManage || saving">
          <option value="">Selecione</option>
          <option
            v-for="resource in resourceOptions"
            :key="resource.id"
            :value="resource.id"
            :disabled="!resource.active"
          >
            {{ resource.label }}{{ resource.active ? '' : ' (inativo)' }}
          </option>
        </select>
      </label>
      <label class="binding-config__reason">
        Motivo auditável
        <input
          v-model="reason"
          maxlength="500"
          placeholder="Ex.: número dedicado ao cliente"
          :disabled="!canManage || saving"
        />
      </label>
      <button type="submit" :disabled="!canCreate">Criar vínculo</button>
    </form>

    <div class="binding-config__list">
      <p v-if="loading">Carregando vínculos…</p>
      <p v-else-if="bindings.length === 0">Nenhum vínculo ativo.</p>
      <template v-else>
        <article v-for="binding in bindings" :key="binding.id">
          <div>
            <strong>{{ binding.channelResource.label }}</strong>
            <span>
              {{ binding.channel }} · {{ binding.clientAccountName || binding.clientAccountId }}
            </span>
            <small>Desde {{ new Date(binding.effectiveFrom).toLocaleString('pt-BR') }}</small>
          </div>
          <button type="button" @click="selectBinding(binding)">Gerenciar</button>
        </article>
      </template>
    </div>

    <div v-if="selectedBinding" class="binding-config__actions">
      <h4>Gerenciar {{ selectedBinding.channelResource.label }}</h4>
      <label>
        Cliente sucessor
        <select v-model="targetClientAccountId" :disabled="saving">
          <option v-for="client in clients" :key="client.id" :value="client.id">
            {{ client.name }}
          </option>
        </select>
      </label>
      <label>
        Motivo
        <input v-model="actionReason" maxlength="500" :disabled="saving" />
      </label>
      <div>
        <button type="button" :disabled="saving || !actionReason.trim()" @click="submitReassign">
          Reatribuir
        </button>
        <button type="button" :disabled="saving || !actionReason.trim()" @click="submitRepair">
          Preview + reparar órfãos
        </button>
        <button
          type="button"
          class="binding-config__danger"
          :disabled="saving || selectedResourceActive || !actionReason.trim()"
          @click="submitEnd"
        >
          Encerrar
        </button>
      </div>
      <small v-if="selectedResourceActive">
        Para encerrar sem sucessor, desative primeiro o recurso de canal.
      </small>
      <small v-if="lastRepair">
        Último reparo: {{ lastRepair.status }} · {{ lastRepair.repairedCount }} reparada(s),
        {{ lastRepair.skippedCount }} ignorada(s).
      </small>
    </div>

    <div v-if="exceptions.length" class="binding-config__exceptions">
      <h4>Exceções que não recebem IA no modo enforced</h4>
      <p
        v-for="item in exceptions"
        :key="`${item.channel}:${item.channelResourceId}:${item.bindingState}`"
      >
        {{ item.channel }} · {{ item.reasonCode }} — {{ item.conversationCount }} conversa(s),
        {{ item.touchpointCount }} touchpoint(s)
      </p>
    </div>
  </section>
</template>

<style scoped>
.binding-config {
  display: grid;
  gap: 1rem;
}
.binding-config > header,
.binding-config__policy,
.binding-config__form,
.binding-config__ownership-guide article,
.binding-config__actions,
.binding-config__exceptions,
.binding-config__list article {
  border: 1px solid var(--border-subtle);
  border-radius: 0.8rem;
  background: var(--surface-card);
  padding: 0.9rem;
}
.binding-config > header,
.binding-config__list article,
.binding-config__actions > div {
  display: flex;
  justify-content: space-between;
  gap: 0.75rem;
  align-items: center;
}
.binding-config h3,
.binding-config h4,
.binding-config p {
  margin: 0;
}
.binding-config__eyebrow,
.binding-config small,
.binding-config__list span {
  color: var(--text-muted);
  font-size: 0.78rem;
}
.binding-config__policy,
.binding-config__form,
.binding-config__ownership-guide {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.75rem;
}
.binding-config__ownership-guide article {
  display: grid;
  gap: 0.35rem;
}
.binding-config__ownership-guide span {
  color: var(--text-muted);
  font-size: 0.78rem;
  line-height: 1.45;
}
.binding-config label,
.binding-config__list article > div {
  display: grid;
  gap: 0.35rem;
}
.binding-config select,
.binding-config input {
  min-height: 2.5rem;
  border: 1px solid var(--border-subtle);
  border-radius: 0.55rem;
  background: var(--surface-input, var(--surface-card));
  color: var(--text-primary);
  padding: 0.55rem 0.7rem;
}
.binding-config button {
  min-height: 2.45rem;
  border: 0;
  border-radius: 0.55rem;
  padding: 0.55rem 0.85rem;
  cursor: pointer;
}
.binding-config button:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}
.binding-config__reason {
  grid-column: 1 / -1;
}
.binding-config__list {
  display: grid;
  gap: 0.55rem;
}
.binding-config__danger,
.binding-config__error {
  color: var(--color-danger, #ef4444);
}
.binding-config__exceptions {
  display: grid;
  gap: 0.4rem;
}
@media (max-width: 760px) {
  .binding-config__policy,
  .binding-config__form,
  .binding-config__ownership-guide {
    grid-template-columns: 1fr;
  }
  .binding-config > header,
  .binding-config__list article,
  .binding-config__actions > div {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>
