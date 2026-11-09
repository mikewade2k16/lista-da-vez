<script setup lang="ts">
import AccountingEvidence from './AccountingEvidence.vue'
import { computed, ref, watch } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import type { useCrowPlus } from '../../composables/useCrowPlus'
import { useAccountingWorkspace } from '../../composables/accounting/useAccountingWorkspace'
import { useFinanceDocumentsStore } from '~/stores/finance-documents'
import { useAuthStore } from '~/stores/auth'
import { confirmFiscalAction } from '../../utils/fiscal-confirm'
const props = defineProps<{
  controller: ReturnType<typeof useCrowPlus>
  readonly?: boolean
  enterpriseOnly?: boolean
}>()
const emit = defineEmits<{ personal: [] }>()
const { state } = props.controller
const company = useAccountingWorkspace()
const documentsStore = useFinanceDocumentsStore(),
  auth = useAuthStore()
const sources = ref<Array<{ id: string; name: string }>>([]),
  sourcesBusy = ref(false)
let sourceGeneration = 0
watch(
  () => `${props.controller.accountId.value}:${auth.user?.id}:${state.issuerId}`,
  () => {
    sourceGeneration++
    sources.value = []
    draftId.value = ''
    sourcesBusy.value = false
  },
  { flush: 'sync' },
)
async function loadSources() {
  if (state.issuerId || !state.enabled || sourcesBusy.value) return
  const generation = ++sourceGeneration,
    accountId = props.controller.accountId.value
  sourcesBusy.value = true
  try {
    const values = await documentsStore.list(accountId)
    if (generation === sourceGeneration)
      sources.value = values.map((item) => ({ id: item.id, name: item.sourceName || item.id }))
  } catch {
    if (generation === sourceGeneration) state.error = 'Não foi possível listar seus comprovantes.'
  } finally {
    if (generation === sourceGeneration) sourcesBusy.value = false
  }
}
const draftId = ref('')
const companyOptions = computed(() => [
  {
    value: '',
    label: props.enterpriseOnly ? 'Escolha uma empresa' : 'Somente Finance / perfil pessoal',
  },
  ...company.state.issuers.map((item) => ({ value: item.id, label: item.id })),
])
async function changeIssuer(id: string) {
  if (id === state.issuerId) return
  if (state.draft || state.busy) {
    const confirmed = await confirmFiscalAction(
      'Trocar de contexto inicia outra conversa. O resultado de uma operação em andamento ficará no histórico de origem.',
      () => `${props.controller.accountId.value}:${auth.user?.id}:${state.issuerId}`,
    )
    if (!confirmed) return
  }
  draftId.value = ''
  await props.controller.chooseIssuer(id)
}
async function mayChangeConversation() {
  return (
    !state.draft.trim() ||
    (await confirmFiscalAction(
      'Descartar a mensagem ainda não enviada?',
      () => `${props.controller.accountId.value}:${auth.user?.id}:${state.issuerId}`,
    ))
  )
}
async function openHistory(id: string) {
  if (await mayChangeConversation()) await props.controller.open(id)
}
async function freshConversation() {
  if (await mayChangeConversation()) await props.controller.fresh()
}

const labels: Record<string, string> = {
  completed: 'Concluído',
  missing_data: 'Dados pendentes',
  capability_unavailable: 'Capacidade indisponível',
  provider_unavailable: 'Provedor indisponível',
  insufficient_evidence: 'Evidência insuficiente',
  awaiting_authorization: 'Aguardando autorização',
}
function totals(value: unknown): Array<[string, string]> {
  if (!value || typeof value !== 'object') return []
  const data = value as Record<string, unknown>
  const titles: Record<string, string> = {
    effectiveIn: 'Entradas efetivadas',
    effectiveOut: 'Saídas efetivadas',
    effectiveBalance: 'Saldo efetivado',
    expectedIn: 'Entradas esperadas',
    expectedOut: 'Saídas esperadas',
  }
  return Object.entries(titles).flatMap(([key, title]) =>
    typeof data[key] === 'string' ? [[title, data[key] as string] as [string, string]] : [],
  )
}
function pendingItems(value: unknown): string[] {
  return Array.isArray(value)
    ? value.filter((item): item is string => typeof item === 'string')
    : []
}
function diagnosisCards(value: unknown): Array<{ id: string; pending: string[] }> {
  if (!Array.isArray(value)) return []
  return value.flatMap((item) =>
    item && typeof item === 'object' && typeof item.id === 'string'
      ? [{ id: item.id, pending: pendingItems(item.pending) }]
      : [],
  )
}
</script>
<template>
  <section class="crow-plus" aria-label="Crow Assistant Plus">
    <p v-if="readonly">
      Histórico anterior, somente leitura. Para novos pedidos, volte à conversa.
    </p>
    <p>
      Conta {{ controller.accountId.value }} · Finance ·
      {{ state.issuerId ? `Emitente ${state.issuerId}` : 'Perfil pessoal / consultas financeiras' }}
    </p>
    <AppSelectField
      label="Contexto da conversa"
      :model-value="state.issuerId"
      :options="companyOptions"
      @update:model-value="changeIssuer"
    />
    <p>
      Trocar emitente inicia outra conversa. Histórico legado e empresarial permanecem separados.
    </p>
    <AppSelectField
      label="Histórico Plus deste contexto"
      :model-value="state.conversationId"
      :disabled="state.busy"
      :options="
        state.conversations.map((item) => ({ value: item.id, label: item.title || item.id }))
      "
      @update:model-value="openHistory"
    />
    <div class="crow-plus__actions">
      <AppPanelButton variant="secondary" :disabled="state.busy" @click="controller.load">
        Atualizar histórico
      </AppPanelButton>
      <AppPanelButton
        v-if="!readonly"
        variant="secondary"
        :disabled="state.busy"
        @click="freshConversation"
      >
        Nova conversa
      </AppPanelButton>
    </div>
    <p v-if="state.error" role="alert">{{ state.error }}</p>
    <div class="crow-plus__messages" aria-live="polite">
      <article v-for="message in state.messages" :key="message.id" class="crow-plus__message">
        <strong>{{ message.role === 'user' ? 'Você' : 'Crow Plus' }}</strong>
        <p>{{ message.content }}</p>
        <template v-if="message.plusEnvelope">
          <small>{{ labels[message.plusEnvelope.status] || message.plusEnvelope.status }}</small>
          <dl v-if="message.plusEnvelope.data?.totals">
            <template
              v-for="[label, value] in totals(message.plusEnvelope.data.totals)"
              :key="label"
            >
              <dt>{{ label }}</dt>
              <dd>{{ value }}</dd>
            </template>
          </dl>
          <ul v-if="pendingItems(message.plusEnvelope.data?.pending).length">
            <li v-for="item in pendingItems(message.plusEnvelope.data.pending)" :key="item">
              {{ item }}
            </li>
          </ul>
          <article
            v-for="item in diagnosisCards(message.plusEnvelope.data?.diagnoses)"
            :key="item.id"
          >
            <strong>Diagnóstico {{ item.id }}</strong>
            <ul>
              <li v-for="pending in item.pending" :key="pending">{{ pending }}</li>
            </ul>
            <p v-if="!item.pending.length">Sem pendências registradas.</p>
          </article>
          <AccountingEvidence
            :evidence="message.plusEnvelope.evidence"
            :data="message.plusEnvelope.data"
          />
          <p v-if="message.plusEnvelope.data?.potentialConflict">
            Múltiplas fontes; revisar possíveis conflitos.
          </p>
          <p v-if="message.plusEnvelope.data?.truncated">
            Resultado parcial; totais informados pelo servidor.
          </p>
          <AppPanelButton
            v-if="message.plusEnvelope.data?.action"
            variant="secondary"
            @click="emit('personal')"
          >
            Abrir formulário pessoal protegido
          </AppPanelButton>
        </template>
      </article>
    </div>
    <p v-if="!state.messages.length">
      Selecione uma planilha no Finance e peça “resumo” ou “o que paguei”. Para a empresa, escolha o
      emitente e peça “pendências”.
    </p>
    <p>
      Preparação não autoriza envio. Credenciais e anexos são tratados no fluxo pessoal protegido.
    </p>
    <template v-if="!readonly && !state.issuerId">
      <AppPanelButton
        variant="secondary"
        :disabled="sourcesBusy || state.busy"
        @click="loadSources"
      >
        Escolher comprovante pessoal já salvo
      </AppPanelButton>
      <AppSelectField
        v-if="sources.length"
        v-model="draftId"
        label="Comprovante para preparar (seleção explícita)"
        :options="[
          { value: '', label: 'Nenhum' },
          ...sources.map((item) => ({ value: item.id, label: item.name })),
        ]"
        :disabled="state.busy"
      />
    </template>
    <form v-if="!readonly" class="crow-plus__form" @submit.prevent="controller.send({ draftId })">
      <label for="crow-plus-message">Mensagem</label>
      <textarea
        id="crow-plus-message"
        v-model="state.draft"
        class="omni-input"
        rows="3"
        maxlength="4000"
        :disabled="
          state.busy || !state.enabled || state.blocked || (enterpriseOnly && !state.issuerId)
        "
        placeholder="Consultar dados, pendências ou preparar um rascunho selecionado"
      ></textarea>
      <AppPanelButton
        type="submit"
        :disabled="
          state.busy ||
          !state.enabled ||
          state.blocked ||
          (enterpriseOnly && !state.issuerId) ||
          !state.draft.trim()
        "
      >
        {{ state.busy ? 'Processando...' : 'Enviar' }}
      </AppPanelButton>
    </form>
  </section>
</template>
<style scoped>
.crow-plus {
  display: grid;
  gap: 0.625rem;
  padding: 0.75rem;
  min-width: 0;
  overflow-wrap: anywhere;
}
.crow-plus__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}
.crow-plus__messages {
  display: grid;
  gap: 0.5rem;
}
.crow-plus__message {
  padding: 0.625rem;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
}
.crow-plus__message p {
  white-space: pre-wrap;
}
.crow-plus__form {
  display: grid;
  gap: 0.5rem;
}
</style>
