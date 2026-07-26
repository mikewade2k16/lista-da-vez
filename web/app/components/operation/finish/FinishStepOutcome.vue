<script setup>
defineProps({
  isPendingValidation: {
    type: Boolean,
    default: false,
  },
  validationReason: {
    type: String,
    default: '',
  },
  outcome: {
    type: String,
    default: '',
  },
  shouldUsePurchaseCodeField: {
    type: Boolean,
    default: false,
  },
  purchaseCodeLabel: {
    type: String,
    default: '',
  },
  purchaseCode: {
    type: String,
    default: '',
  },
  purchaseCodePlaceholder: {
    type: String,
    default: '',
  },
})

const emit = defineEmits(['update:validation-reason', 'update:outcome', 'update:purchase-code'])
</script>

<template>
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

  <section class="finish-form__section">
    <strong class="finish-form__label">Como terminou</strong>
    <div class="finish-form__options">
      <label class="modal-radio">
        <input
          :checked="outcome === 'reserva'"
          type="radio"
          name="finish-outcome"
          value="reserva"
          data-testid="operation-outcome-reserva"
          @change="emit('update:outcome', $event.target.value)"
        />
        <span>Reserva</span>
      </label>
      <label class="modal-radio">
        <input
          :checked="outcome === 'compra'"
          type="radio"
          name="finish-outcome"
          value="compra"
          data-testid="operation-outcome-compra"
          @change="emit('update:outcome', $event.target.value)"
        />
        <span>Compra</span>
      </label>
      <label class="modal-radio">
        <input
          :checked="outcome === 'nao-compra'"
          type="radio"
          name="finish-outcome"
          value="nao-compra"
          data-testid="operation-outcome-nao-compra"
          @change="emit('update:outcome', $event.target.value)"
        />
        <span>Nao compra</span>
      </label>
    </div>
  </section>

  <section v-if="shouldUsePurchaseCodeField" class="finish-form__section">
    <label class="finish-form__field">
      <span class="finish-form__label">{{ purchaseCodeLabel }}</span>
      <input
        :value="purchaseCode"
        class="finish-form__input"
        type="text"
        :placeholder="purchaseCodePlaceholder"
        data-testid="operation-purchase-code"
        @input="emit('update:purchase-code', $event.target.value)"
      />
    </label>
  </section>
</template>
