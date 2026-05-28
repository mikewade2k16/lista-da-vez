<script setup>
import OperationProductPicker from '~/components/operation/OperationProductPicker.vue'

const props = defineProps({
  showCustomerSection: {
    type: Boolean,
    default: false,
  },
  customerSectionLabel: {
    type: String,
    default: '',
  },
  showExistingCustomerField: {
    type: Boolean,
    default: false,
  },
  existingCustomerLabel: {
    type: String,
    default: '',
  },
  isExistingCustomer: {
    type: Boolean,
    default: false,
  },
  showCustomerNameField: {
    type: Boolean,
    default: false,
  },
  customerNameLabel: {
    type: String,
    default: '',
  },
  customerName: {
    type: String,
    default: '',
  },
  showCustomerPhoneField: {
    type: Boolean,
    default: false,
  },
  customerPhoneLabel: {
    type: String,
    default: '',
  },
  customerPhone: {
    type: String,
    default: '',
  },
  showEmailField: {
    type: Boolean,
    default: false,
  },
  customerEmailLabel: {
    type: String,
    default: '',
  },
  customerEmail: {
    type: String,
    default: '',
  },
  showProfessionField: {
    type: Boolean,
    default: false,
  },
  customerProfessionLabel: {
    type: String,
    default: '',
  },
  professionPickerOptions: {
    type: Array,
    default: () => [],
  },
  professionSelectedItems: {
    type: Array,
    default: () => [],
  },
  showVisitReasonField: {
    type: Boolean,
    default: false,
  },
  visitReasonLabel: {
    type: String,
    default: '',
  },
  visitReasonPickerOptions: {
    type: Array,
    default: () => [],
  },
  visitReasonSelectedItems: {
    type: Array,
    default: () => [],
  },
  isVisitReasonMultiple: {
    type: Boolean,
    default: false,
  },
  visitReasonDetailsEnabled: {
    type: Boolean,
    default: false,
  },
  visitReasonPickerDetailMode: {
    type: String,
    default: 'shared',
  },
  visitReasonDetails: {
    type: Object,
    default: () => ({}),
  },
  visitReasonNotInformed: {
    type: Boolean,
    default: false,
  },
  showCustomerSourceField: {
    type: Boolean,
    default: false,
  },
  customerSourceLabel: {
    type: String,
    default: '',
  },
  customerSourcePickerOptions: {
    type: Array,
    default: () => [],
  },
  customerSourceSelectedItems: {
    type: Array,
    default: () => [],
  },
  isCustomerSourceMultiple: {
    type: Boolean,
    default: false,
  },
  customerSourceDetailsEnabled: {
    type: Boolean,
    default: false,
  },
  customerSourcePickerDetailMode: {
    type: String,
    default: 'shared',
  },
  customerSourceDetails: {
    type: Object,
    default: () => ({}),
  },
  customerSourceNotInformed: {
    type: Boolean,
    default: false,
  },
  formatPhoneMask: {
    type: Function,
    default: (value) => String(value || ''),
  },
})

const emit = defineEmits([
  'update:is-existing-customer',
  'update:customer-name',
  'update:customer-phone',
  'update:customer-email',
  'update:profession-selected-items',
  'update:visit-reason-selected-items',
  'update:visit-reason-details',
  'update:visit-reason-not-informed',
  'update:customer-source-selected-items',
  'update:customer-source-details',
  'update:customer-source-not-informed',
])

function handleCustomerPhoneInput(event) {
  const maskedValue = props.formatPhoneMask(event?.target?.value || props.customerPhone)
  emit('update:customer-phone', maskedValue)

  if (event?.target) {
    event.target.value = maskedValue
  }
}
</script>

<template>
  <section v-if="showCustomerSection" class="finish-form__section">
    <strong class="finish-form__label">{{ customerSectionLabel }}</strong>
  </section>

  <section v-if="showExistingCustomerField" class="finish-form__section finish-form__grid">
    <label class="modal-checkbox">
      <input
        :checked="isExistingCustomer"
        type="checkbox"
        @change="emit('update:is-existing-customer', $event.target.checked)"
      />
      <span>{{ existingCustomerLabel }}</span>
    </label>
  </section>

  <section class="finish-form__section finish-form__grid finish-form__grid--customer">
    <label v-if="showCustomerNameField" class="finish-form__field">
      <span class="finish-form__label">{{ customerNameLabel }}</span>
      <input
        :value="customerName"
        class="finish-form__input"
        type="text"
        placeholder="Nome Completo"
        data-testid="operation-customer-name"
        @input="emit('update:customer-name', $event.target.value)"
      />
    </label>
    <label v-if="showCustomerPhoneField" class="finish-form__field">
      <span class="finish-form__label">{{ customerPhoneLabel }}</span>
      <input
        :value="customerPhone"
        class="finish-form__input"
        type="tel"
        placeholder="(11) 99999-9999"
        data-testid="operation-customer-phone"
        @input="handleCustomerPhoneInput"
      />
    </label>
    <label v-if="showEmailField" class="finish-form__field">
      <span class="finish-form__label">{{ customerEmailLabel }}</span>
      <input
        :value="customerEmail"
        class="finish-form__input"
        type="email"
        placeholder="E-mail"
        data-testid="operation-customer-email"
        @input="emit('update:customer-email', $event.target.value)"
      />
    </label>
  </section>

  <div class="operation-modal__select-grid">
    <section v-if="showProfessionField" class="finish-form__section operation-modal__picker-cell">
      <OperationProductPicker
        :label="customerProfessionLabel"
        :options="professionPickerOptions"
        :selected-items="professionSelectedItems"
        :multiple="false"
        trigger-label="Selecionar profissão"
        search-placeholder="Busque e selecione a profissão"
        empty-selected-label="Nenhuma profissão selecionada"
        testid-prefix="operation-customer-profession"
        @update:selected-items="emit('update:profession-selected-items', $event)"
      />
    </section>

    <section v-if="showVisitReasonField" class="finish-form__section operation-modal__picker-cell">
      <OperationProductPicker
        :label="visitReasonLabel"
        :options="visitReasonPickerOptions"
        :selected-items="visitReasonSelectedItems"
        :multiple="isVisitReasonMultiple"
        :enable-item-details="visitReasonDetailsEnabled"
        :item-detail-mode="visitReasonPickerDetailMode"
        :item-details="visitReasonDetails"
        item-detail-label="Descricao"
        item-detail-placeholder="Digite a descricao que deseja salvar"
        item-detail-testid="operation-visit-reason-detail"
        :none-selected="visitReasonNotInformed"
        allow-none
        none-label="Nao informado"
        none-state-label="Nao informado"
        trigger-label="Selecionar motivo"
        search-placeholder="Busque e selecione o motivo"
        empty-selected-label="Nenhum motivo selecionado"
        testid-prefix="operation-visit-reason"
        @update:selected-items="emit('update:visit-reason-selected-items', $event)"
        @update:item-details="emit('update:visit-reason-details', $event)"
        @update:none-selected="emit('update:visit-reason-not-informed', $event)"
      />
    </section>

    <section
      v-if="showCustomerSourceField"
      class="finish-form__section operation-modal__picker-cell"
    >
      <OperationProductPicker
        :label="customerSourceLabel"
        :options="customerSourcePickerOptions"
        :selected-items="customerSourceSelectedItems"
        :multiple="isCustomerSourceMultiple"
        :enable-item-details="customerSourceDetailsEnabled"
        :item-detail-mode="customerSourcePickerDetailMode"
        :item-details="customerSourceDetails"
        item-detail-label="Descricao"
        item-detail-placeholder="Digite a descricao da origem"
        item-detail-testid="operation-customer-source-detail"
        :none-selected="customerSourceNotInformed"
        allow-none
        none-label="Nao informado"
        none-state-label="Nao informado"
        trigger-label="Selecionar origem"
        search-placeholder="Busque e selecione a origem"
        empty-selected-label="Nenhuma origem selecionada"
        testid-prefix="operation-customer-source"
        @update:selected-items="emit('update:customer-source-selected-items', $event)"
        @update:item-details="emit('update:customer-source-details', $event)"
        @update:none-selected="emit('update:customer-source-not-informed', $event)"
      />
    </section>
  </div>
</template>