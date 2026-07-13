<script setup>
import OperationProductPicker from '~/components/operation/OperationProductPicker.vue'

defineProps({
  isQueueJumpService: {
    type: Boolean,
    default: false,
  },
  showQueueJumpReasonField: {
    type: Boolean,
    default: false,
  },
  queueJumpReasonLabel: {
    type: String,
    default: '',
  },
  queueJumpReasonPickerOptions: {
    type: Array,
    default: () => [],
  },
  queueJumpReasonSelectedItems: {
    type: Array,
    default: () => [],
  },
  queueJumpReasonPlaceholder: {
    type: String,
    default: '',
  },
  showLossReasonSection: {
    type: Boolean,
    default: false,
  },
  lossReasonLabel: {
    type: String,
    default: '',
  },
  lossReasonPickerOptions: {
    type: Array,
    default: () => [],
  },
  lossReasonSelectedItems: {
    type: Array,
    default: () => [],
  },
  isLossReasonMultiple: {
    type: Boolean,
    default: false,
  },
  lossReasonDetailsEnabled: {
    type: Boolean,
    default: false,
  },
  lossReasonPickerDetailMode: {
    type: String,
    default: 'shared',
  },
  lossReasonDetails: {
    type: Object,
    default: () => ({}),
  },
  lossReasonPlaceholder: {
    type: String,
    default: '',
  },
  showNotesField: {
    type: Boolean,
    default: false,
  },
  // Encerramento de pendencia (auto-encerramento 2h): exibe o bloco de
  // justificativa OBRIGATORIA de por que o consultor nao encerrou na hora.
  isPendingValidation: {
    type: Boolean,
    default: false,
  },
  validationReason: {
    type: String,
    default: '',
  },
  notesLabel: {
    type: String,
    default: '',
  },
  notes: {
    type: String,
    default: '',
  },
  notesPlaceholder: {
    type: String,
    default: '',
  },
  justificationsRevealed: {
    type: Boolean,
    default: false,
  },
  step2MissingJustifications: {
    type: Array,
    default: () => [],
  },
  fieldJustifications: {
    type: Object,
    default: () => ({}),
  },
  isFieldJustificationValid: {
    type: Function,
    required: true,
  },
  getFieldJustificationCharCount: {
    type: Function,
    required: true,
  },
  shouldUseLegacyClosedProductField: {
    type: Boolean,
    default: false,
  },
  formattedClosedTotal: {
    type: String,
    default: '',
  },
  step2QualityTone: {
    type: String,
    default: 'incomplete',
  },
  formQuality: {
    type: Object,
    required: true,
  },
  step2QualityLabel: {
    type: String,
    default: '',
  },
  showCustomerNameField: {
    type: Boolean,
    default: false,
  },
  requireCustomerNameField: {
    type: Boolean,
    default: false,
  },
  showCustomerPhoneField: {
    type: Boolean,
    default: false,
  },
  requireCustomerPhoneField: {
    type: Boolean,
    default: false,
  },
  shouldUsePurchaseCodeField: {
    type: Boolean,
    default: false,
  },
  requirePurchaseCodeField: {
    type: Boolean,
    default: false,
  },
  requireProductClosedField: {
    type: Boolean,
    default: false,
  },
  showProductSeenField: {
    type: Boolean,
    default: false,
  },
  requireProductSeenField: {
    type: Boolean,
    default: false,
  },
  isProductSeenNotesRequired: {
    type: Boolean,
    default: false,
  },
  showVisitReasonField: {
    type: Boolean,
    default: false,
  },
  requireVisitReasonField: {
    type: Boolean,
    default: false,
  },
  showCustomerSourceField: {
    type: Boolean,
    default: false,
  },
  requireCustomerSourceField: {
    type: Boolean,
    default: false,
  },
  isLossOutcome: {
    type: Boolean,
    default: false,
  },
  showLossReasonField: {
    type: Boolean,
    default: false,
  },
  requireLossReasonField: {
    type: Boolean,
    default: false,
  },
  requireQueueJumpReasonField: {
    type: Boolean,
    default: false,
  },
  showEmailField: {
    type: Boolean,
    default: false,
  },
  requireEmailField: {
    type: Boolean,
    default: false,
  },
  showProfessionField: {
    type: Boolean,
    default: false,
  },
  requireProfessionField: {
    type: Boolean,
    default: false,
  },
  requireNotesField: {
    type: Boolean,
    default: false,
  },
  hasInvalidStep2Justifications: {
    type: Boolean,
    default: false,
  },
})

const emit = defineEmits([
  'update:queue-jump-reason-selected-items',
  'update:loss-reason-selected-items',
  'update:loss-reason-details',
  'update:notes',
  'update:validation-reason',
  'update:field-justification',
  'previous',
  'submit',
])
</script>

<template>
  <!-- Encerramento de pendencia (auto-encerramento 2h): justificativa OBRIGATORIA
       de por que o consultor nao encerrou na hora — base das metricas de cobranca. -->
  <section v-if="isPendingValidation" class="finish-form__section finish-form__section--pending">
    <label class="finish-form__label" for="operation-validation-reason">
      Por que este atendimento nao foi encerrado pelo consultor? *
    </label>
    <textarea
      id="operation-validation-reason"
      :value="validationReason"
      class="finish-form__textarea"
      rows="2"
      placeholder="Ex.: consultor esqueceu de encerrar; cliente saiu sem aviso"
      data-testid="operation-validation-reason"
      @input="emit('update:validation-reason', $event.target.value)"
    ></textarea>
  </section>

  <section
    v-if="isQueueJumpService && showQueueJumpReasonField"
    class="finish-form__section operation-modal__picker-cell"
  >
    <OperationProductPicker
      :label="queueJumpReasonLabel"
      :options="queueJumpReasonPickerOptions"
      :selected-items="queueJumpReasonSelectedItems"
      :multiple="false"
      trigger-label="Selecionar motivo"
      :search-placeholder="queueJumpReasonPlaceholder"
      empty-selected-label="Nenhum motivo selecionado"
      testid-prefix="operation-queue-jump-reason"
      @update:selected-items="emit('update:queue-jump-reason-selected-items', $event)"
    />
  </section>

  <section v-if="showLossReasonSection" class="finish-form__section operation-modal__picker-cell">
    <OperationProductPicker
      :label="lossReasonLabel"
      :options="lossReasonPickerOptions"
      :selected-items="lossReasonSelectedItems"
      :multiple="isLossReasonMultiple"
      :enable-item-details="lossReasonDetailsEnabled"
      :item-detail-mode="lossReasonPickerDetailMode"
      :item-details="lossReasonDetails"
      item-detail-label="Descricao"
      item-detail-placeholder="Digite a descricao do motivo da perda"
      item-detail-testid="operation-loss-reason-detail"
      trigger-label="Selecionar motivo"
      :search-placeholder="lossReasonPlaceholder"
      empty-selected-label="Nenhum motivo selecionado"
      testid-prefix="operation-loss-reason"
      @update:selected-items="emit('update:loss-reason-selected-items', $event)"
      @update:item-details="emit('update:loss-reason-details', $event)"
    />
  </section>

  <section v-if="showNotesField" class="finish-form__section">
    <label class="finish-form__label" for="finish-notes">{{ notesLabel }}</label>
    <textarea
      id="finish-notes"
      :value="notes"
      class="finish-form__textarea"
      rows="3"
      :placeholder="notesPlaceholder"
      data-testid="operation-notes"
      @input="emit('update:notes', $event.target.value)"
    ></textarea>
  </section>

  <section
    v-if="justificationsRevealed && step2MissingJustifications.length"
    class="finish-form__section"
  >
    <strong class="finish-form__label">Justificativas pendentes</strong>
    <div class="finish-form__justification-list">
      <div
        v-for="item in step2MissingJustifications"
        :key="item.key"
        class="finish-form__justification-item"
      >
        <label class="finish-form__label" :for="`finish-justification-${item.key}`">
          Justifique: {{ item.label }}
        </label>
        <textarea
          :id="`finish-justification-${item.key}`"
          :value="fieldJustifications[item.key] || ''"
          class="finish-form__textarea"
          rows="3"
          :placeholder="`Explique por que ${item.label.toLowerCase()} ficou em branco`"
          :data-testid="`operation-justification-${item.key}`"
          @input="emit('update:field-justification', { key: item.key, value: $event.target.value })"
        ></textarea>
        <div
          class="finish-form__field-note"
          :class="{
            'finish-form__field-note--error': !isFieldJustificationValid(item.key, item.minChars),
          }"
        >
          <span>
            Campo opcional sem preenchimento. Informe pelo menos
            {{ item.minChars }} caracteres sem contar espaços.
          </span>
          <strong>
            {{ getFieldJustificationCharCount(item.key) }}/{{ item.minChars }} sem espaços
          </strong>
        </div>
      </div>
    </div>
  </section>

  <section
    v-if="shouldUseLegacyClosedProductField"
    class="finish-form__section operation-modal__summary"
  >
    <span class="finish-form__label">Valor da venda derivado dos produtos fechados</span>
    <strong>{{ formattedClosedTotal }}</strong>
  </section>

  <div class="finish-form__quality" :class="`finish-form__quality--${step2QualityTone}`">
    <div class="finish-form__quality-dots">
      <span
        v-if="showCustomerNameField && requireCustomerNameField"
        class="finish-form__quality-dot"
        :class="{ 'is-filled': formQuality.checks.customerName }"
        title="Nome"
      ></span>
      <span
        v-if="showCustomerPhoneField && requireCustomerPhoneField"
        class="finish-form__quality-dot"
        :class="{ 'is-filled': formQuality.checks.customerPhone }"
        title="Telefone"
      ></span>
      <span
        v-if="shouldUsePurchaseCodeField && requirePurchaseCodeField"
        class="finish-form__quality-dot"
        :class="{ 'is-filled': formQuality.checks.purchaseCode }"
        title="Codigo da compra"
      ></span>
      <span
        v-if="shouldUseLegacyClosedProductField && requireProductClosedField"
        class="finish-form__quality-dot"
        :class="{ 'is-filled': formQuality.checks.productClosed }"
        title="Compra / reserva"
      ></span>
      <span
        v-if="showProductSeenField && requireProductSeenField"
        class="finish-form__quality-dot"
        :class="{ 'is-filled': formQuality.checks.productSeen }"
        title="Interesses do cliente"
      ></span>
      <span
        v-if="isProductSeenNotesRequired"
        class="finish-form__quality-dot finish-form__quality-dot--notes"
        :class="{ 'is-filled': formQuality.checks.productSeenNotes }"
        title="Detalhes dos interesses"
      ></span>
      <span
        v-if="showVisitReasonField && requireVisitReasonField"
        class="finish-form__quality-dot"
        :class="{ 'is-filled': formQuality.checks.visitReasons }"
        title="Motivo da visita"
      ></span>
      <span
        v-if="showCustomerSourceField && requireCustomerSourceField"
        class="finish-form__quality-dot"
        :class="{ 'is-filled': formQuality.checks.customerSources }"
        title="Origem do cliente"
      ></span>
      <span
        v-if="isLossOutcome && showLossReasonField && requireLossReasonField"
        class="finish-form__quality-dot"
        :class="{ 'is-filled': formQuality.checks.lossReason }"
        title="Motivo da perda"
      ></span>
      <span
        v-if="isQueueJumpService && showQueueJumpReasonField && requireQueueJumpReasonField"
        class="finish-form__quality-dot"
        :class="{ 'is-filled': formQuality.checks.queueJumpReason }"
        title="Motivo fora da vez"
      ></span>
      <span
        v-if="showEmailField && requireEmailField"
        class="finish-form__quality-dot"
        :class="{ 'is-filled': formQuality.checks.customerEmail }"
        title="E-mail"
      ></span>
      <span
        v-if="showProfessionField && requireProfessionField"
        class="finish-form__quality-dot"
        :class="{ 'is-filled': formQuality.checks.customerProfession }"
        title="Profissão"
      ></span>
      <span
        v-if="showNotesField && requireNotesField"
        class="finish-form__quality-dot finish-form__quality-dot--notes"
        :class="{ 'is-filled': formQuality.checks.notes }"
        title="Observações"
      ></span>
      <span
        v-if="justificationsRevealed && step2MissingJustifications.length"
        class="finish-form__quality-dot finish-form__quality-dot--notes"
        :class="{ 'is-filled': !hasInvalidStep2Justifications }"
        title="Justificativas pendentes"
      ></span>
    </div>
    <span class="finish-form__quality-text">
      {{ formQuality.coreFilledCount }}/{{ formQuality.coreTotal }} obrigatorios ·
      {{ step2QualityLabel }}
    </span>
  </div>

  <div class="finish-form__actions">
    <button
      class="column-action column-action--secondary"
      type="button"
      data-testid="operation-step-back"
      @click="emit('previous')"
    >
      ← Voltar
    </button>
    <button
      class="column-action column-action--primary"
      type="submit"
      data-testid="operation-finish-submit"
      @click.prevent="emit('submit')"
    >
      Salvar e encerrar
    </button>
  </div>
</template>
