<script setup>
import { computed, reactive, watch } from 'vue'

import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import OmniEntityDrawer from '~/components/ui/OmniEntityDrawer.vue'
import { normalizeCampaign } from '~/domain/utils/campaigns'

const props = defineProps({
  open: { type: Boolean, default: false },
  item: { type: Object, default: null },
  campaignType: { type: String, required: true },
  customerSourceOptions: { type: Array, default: () => [] },
  visitReasonOptions: { type: Array, default: () => [] },
  productOptions: { type: Array, default: () => [] },
  saving: { type: Boolean, default: false },
})

const emit = defineEmits(['update:open', 'save'])

const draft = reactive(normalizeCampaign({ campaignType: props.campaignType }))
const targetOutcomeOptions = [
  { value: 'compra-reserva', label: 'Compra ou reserva' },
  { value: 'compra', label: 'Compra' },
  { value: 'reserva', label: 'Reserva' },
  { value: 'nao-compra', label: 'Não compra' },
  { value: 'qualquer', label: 'Qualquer desfecho' },
]
const existingCustomerOptions = [
  { value: 'all', label: 'Todos os clientes' },
  { value: 'yes', label: 'Somente clientes recorrentes' },
  { value: 'no', label: 'Somente clientes novos' },
]

const entityLabel = computed(() => (props.campaignType === 'interna' ? 'corridinha' : 'campanha'))
const canSave = computed(() => !props.saving && Boolean(draft.name.trim()))

function resetDraft() {
  Object.assign(
    draft,
    normalizeCampaign({
      ...(props.item || {}),
      campaignType: props.campaignType,
      productCodes: [...(props.item?.productCodes || [])],
      sourceIds: [...(props.item?.sourceIds || [])],
      reasonIds: [...(props.item?.reasonIds || [])],
    }),
  )
}

function toggleListValue(field, value) {
  const values = Array.isArray(draft[field]) ? draft[field] : []
  draft[field] = values.includes(value)
    ? values.filter((candidate) => candidate !== value)
    : [...values, value]
}

function submit() {
  if (!canSave.value) return
  emit(
    'save',
    normalizeCampaign({
      ...draft,
      campaignType: props.campaignType,
    }),
  )
}

watch(
  () => [props.open, props.item?.id, props.campaignType],
  ([open]) => {
    if (open) resetDraft()
  },
  { immediate: true },
)
</script>

<template>
  <OmniEntityDrawer
    :model-value="open"
    :title="item ? `Editar ${entityLabel}` : `Nova ${entityLabel}`"
    :subtitle="
      campaignType === 'interna'
        ? 'Configure o período, as regras e a premiação do incentivo.'
        : 'Configure o período e as condições comerciais da campanha.'
    "
    @update:model-value="emit('update:open', $event)"
  >
    <form class="campaign-editor" @submit.prevent="submit">
      <section class="campaign-editor__section">
        <header>
          <strong>Informações principais</strong>
          <span>Identificação e período de validade.</span>
        </header>
        <div class="campaign-editor__grid">
          <label class="campaign-editor__field campaign-editor__field--wide">
            <span>Nome</span>
            <input v-model="draft.name" type="text" maxlength="160" />
          </label>
          <label class="campaign-editor__field campaign-editor__field--wide">
            <span>Descrição</span>
            <input v-model="draft.description" type="text" maxlength="300" />
          </label>
          <label class="campaign-editor__field">
            <span>Início</span>
            <input v-model="draft.startsAt" type="date" />
          </label>
          <label class="campaign-editor__field">
            <span>Fim</span>
            <input v-model="draft.endsAt" type="date" />
          </label>
        </div>
        <div class="campaign-editor__toggles">
          <label>
            <input v-model="draft.isActive" type="checkbox" />
            <span>Ativa</span>
          </label>
          <label>
            <input v-model="draft.queueJumpOnly" type="checkbox" />
            <span>Somente atendimentos fora da vez</span>
          </label>
        </div>
      </section>

      <section class="campaign-editor__section">
        <header>
          <strong>Regras para pontuar</strong>
          <span>O atendimento precisa respeitar todos os filtros preenchidos.</span>
        </header>
        <div class="campaign-editor__grid">
          <AppSelectField
            v-model="draft.targetOutcome"
            label="Desfecho alvo"
            :options="targetOutcomeOptions"
          />
          <AppSelectField
            v-model="draft.existingCustomerFilter"
            label="Tipo de cliente"
            :options="existingCustomerOptions"
          />
          <label class="campaign-editor__field">
            <span>Venda mínima (R$)</span>
            <input v-model.number="draft.minSaleAmount" type="number" min="0" step="0.01" />
          </label>
          <label class="campaign-editor__field">
            <span>Duração máxima (min)</span>
            <input v-model.number="draft.maxServiceMinutes" type="number" min="0" step="1" />
          </label>
        </div>
      </section>

      <section class="campaign-editor__section">
        <header>
          <strong>{{ campaignType === 'interna' ? 'Premiação' : 'Bonificação' }}</strong>
          <span>Valor fixo e/ou percentual calculado sobre a venda.</span>
        </header>
        <div class="campaign-editor__grid">
          <label class="campaign-editor__field">
            <span>Valor fixo (R$)</span>
            <input v-model.number="draft.bonusFixed" type="number" min="0" step="0.01" />
          </label>
          <label class="campaign-editor__field">
            <span>Percentual (decimal)</span>
            <input v-model.number="draft.bonusRate" type="number" min="0" max="1" step="0.001" />
          </label>
        </div>
      </section>

      <section
        v-if="customerSourceOptions.length || visitReasonOptions.length || productOptions.length"
        class="campaign-editor__section"
      >
        <header>
          <strong>Público e produtos</strong>
          <span>Sem seleção, a regra considera todas as opções.</span>
        </header>

        <div v-if="customerSourceOptions.length" class="campaign-editor__choices">
          <span>Origens</span>
          <div>
            <label v-for="option in customerSourceOptions" :key="option.id">
              <input
                type="checkbox"
                :checked="draft.sourceIds.includes(option.id)"
                @change="toggleListValue('sourceIds', option.id)"
              />
              <span>{{ option.label || option.name }}</span>
            </label>
          </div>
        </div>

        <div v-if="visitReasonOptions.length" class="campaign-editor__choices">
          <span>Motivos da visita</span>
          <div>
            <label v-for="option in visitReasonOptions" :key="option.id">
              <input
                type="checkbox"
                :checked="draft.reasonIds.includes(option.id)"
                @change="toggleListValue('reasonIds', option.id)"
              />
              <span>{{ option.label || option.name }}</span>
            </label>
          </div>
        </div>

        <div v-if="productOptions.length" class="campaign-editor__choices">
          <span>Produtos</span>
          <div>
            <label v-for="option in productOptions" :key="option.value">
              <input
                type="checkbox"
                :checked="draft.productCodes.includes(option.value)"
                @change="toggleListValue('productCodes', option.value)"
              />
              <span>{{ option.label }}</span>
            </label>
          </div>
        </div>
      </section>
    </form>

    <template #footer>
      <div class="campaign-editor__footer">
        <AppPanelButton variant="ghost" @click="emit('update:open', false)">
          Cancelar
        </AppPanelButton>
        <AppPanelButton :disabled="!canSave" @click="submit">
          {{ saving ? 'Salvando…' : item ? 'Salvar alterações' : `Criar ${entityLabel}` }}
        </AppPanelButton>
      </div>
    </template>
  </OmniEntityDrawer>
</template>

<style scoped>
.campaign-editor,
.campaign-editor__section {
  display: grid;
  gap: 0.85rem;
}

.campaign-editor__section {
  padding: 0.9rem;
  border: 1px solid var(--line-soft);
  border-radius: 13px;
  background: rgb(var(--surface-2) / 0.24);
}

.campaign-editor__section > header {
  display: grid;
  gap: 0.18rem;
}

.campaign-editor__section > header strong {
  color: var(--text-main);
  font-size: 0.82rem;
}

.campaign-editor__section > header span,
.campaign-editor__choices > span {
  color: rgb(var(--muted));
  font-size: 0.69rem;
}

.campaign-editor__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.7rem;
}

.campaign-editor__field {
  display: grid;
  gap: 0.35rem;
  color: rgb(var(--muted));
  font-size: 0.72rem;
  font-weight: 700;
}

.campaign-editor__field--wide {
  grid-column: 1 / -1;
}

.campaign-editor__field input {
  width: 100%;
  min-height: 2.45rem;
  padding: 0 0.72rem;
  border: 1px solid var(--line-soft);
  border-radius: 10px;
  outline: none;
  background: rgb(var(--surface-2) / 0.55);
  color: var(--text-main);
  font: inherit;
}

.campaign-editor__field input:focus {
  border-color: rgb(var(--primary) / 0.65);
  box-shadow: 0 0 0 3px rgb(var(--primary) / 0.1);
}

.campaign-editor__toggles,
.campaign-editor__toggles label,
.campaign-editor__choices label {
  display: flex;
  align-items: center;
  gap: 0.48rem;
}

.campaign-editor__toggles {
  flex-wrap: wrap;
  gap: 0.65rem 1rem;
}

.campaign-editor__toggles label,
.campaign-editor__choices label {
  color: var(--text-main);
  font-size: 0.73rem;
  font-weight: 700;
  cursor: pointer;
}

.campaign-editor__choices {
  display: grid;
  gap: 0.45rem;
}

.campaign-editor__choices > div {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.4rem;
}

.campaign-editor__choices label {
  min-height: 2.2rem;
  padding: 0.42rem 0.55rem;
  border: 1px solid var(--line-soft);
  border-radius: 9px;
  background: rgb(var(--surface-2) / 0.42);
}

.campaign-editor__footer {
  display: flex;
  justify-content: flex-end;
  gap: 0.5rem;
  width: 100%;
}

@media (max-width: 640px) {
  .campaign-editor__grid,
  .campaign-editor__choices > div {
    grid-template-columns: 1fr;
  }
}
</style>
