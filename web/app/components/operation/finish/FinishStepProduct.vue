<script setup>
import OperationProductPicker from '~/components/operation/OperationProductPicker.vue'

defineProps({
  shouldUseLegacyClosedProductField: {
    type: Boolean,
    default: false,
  },
  closedProductLabel: {
    type: String,
    default: '',
  },
  closedProductHelperText: {
    type: String,
    default: '',
  },
  productsClosedPickerOptions: {
    type: Array,
    default: () => [],
  },
  productsClosed: {
    type: Array,
    default: () => [],
  },
  productClosedPlaceholder: {
    type: String,
    default: '',
  },
  productsClosedEmptyLabel: {
    type: String,
    default: '',
  },
  productsClosedSearchPending: {
    type: Boolean,
    default: false,
  },
  showProductSeenField: {
    type: Boolean,
    default: false,
  },
  productSeenLabel: {
    type: String,
    default: '',
  },
  productsSeenPickerOptions: {
    type: Array,
    default: () => [],
  },
  productsSeen: {
    type: Array,
    default: () => [],
  },
  productsSeenNone: {
    type: Boolean,
    default: false,
  },
  productSeenPlaceholder: {
    type: String,
    default: '',
  },
  productsSeenEmptyLabel: {
    type: String,
    default: '',
  },
  allowProductSeenNone: {
    type: Boolean,
    default: false,
  },
  showProductSeenNotesField: {
    type: Boolean,
    default: false,
  },
  productSeenDetailMap: {
    type: Object,
    default: () => ({}),
  },
  productSeenNotesLabel: {
    type: String,
    default: '',
  },
  productSeenNotesPlaceholder: {
    type: String,
    default: '',
  },
  productsSeenSearchPending: {
    type: Boolean,
    default: false,
  },
  productSearchMinChars: {
    type: Number,
    default: 3,
  },
  isProductSeenNoneSelected: {
    type: Boolean,
    default: false,
  },
  productSeenNotes: {
    type: String,
    default: '',
  },
  isProductSeenNotesRequired: {
    type: Boolean,
    default: false,
  },
  isProductSeenNotesValid: {
    type: Boolean,
    default: true,
  },
  productSeenNotesHelperText: {
    type: String,
    default: '',
  },
  productSeenNotesLength: {
    type: Number,
    default: 0,
  },
  productSeenNotesMinChars: {
    type: Number,
    default: 0,
  },
  step1MissingJustifications: {
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
  formStep1Quality: {
    type: Object,
    required: true,
  },
  isStep1Ready: {
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
  requireProductSeenField: {
    type: Boolean,
    default: false,
  },
  hasInvalidStep1Justifications: {
    type: Boolean,
    default: false,
  },
})

const emit = defineEmits([
  'search-products-closed',
  'update:products-closed',
  'search-products-seen',
  'update:products-seen',
  'update:product-seen-details',
  'update:products-seen-none',
  'update:product-seen-notes',
  'update:field-justification',
  'next',
])
</script>

<template>
  <OperationProductPicker
    v-if="shouldUseLegacyClosedProductField"
    key="products-closed-picker"
    :label="closedProductLabel"
    :helper-text="closedProductHelperText"
    :options="productsClosedPickerOptions"
    :selected-items="productsClosed"
    :search-placeholder="productClosedPlaceholder"
    trigger-label="Selecionar item"
    empty-selected-label="Nenhum item selecionado"
    :empty-search-label="productsClosedEmptyLabel"
    allow-custom
    mode="closed"
    remote-search
    :remote-search-loading="productsClosedSearchPending"
    :remote-search-min-chars="productSearchMinChars"
    :remote-search-idle-label="`Digite pelo menos ${productSearchMinChars} digitos do codigo/SKU.`"
    remote-search-loading-label="Buscando produtos..."
    testid-prefix="operation-products-closed"
    @search="emit('search-products-closed', $event)"
    @update:selected-items="emit('update:products-closed', $event)"
  />

  <OperationProductPicker
    v-if="showProductSeenField"
    key="products-seen-picker"
    :label="productSeenLabel"
    helper-text=""
    :options="productsSeenPickerOptions"
    :selected-items="productsSeen"
    :none-selected="productsSeenNone"
    :search-placeholder="productSeenPlaceholder"
    trigger-label="Selecionar interesse"
    empty-selected-label="Nenhum interesse selecionado"
    :empty-search-label="productsSeenEmptyLabel"
    :allow-none="allowProductSeenNone"
    none-placement="dropdown"
    none-label="Nenhum interesse identificado"
    none-state-label="Nenhum interesse identificado"
    :enable-item-details="showProductSeenNotesField"
    item-detail-mode="shared"
    :item-details="productSeenDetailMap"
    :item-detail-label="productSeenNotesLabel"
    :item-detail-placeholder="productSeenNotesPlaceholder"
    item-detail-testid="operation-product-seen-notes"
    remote-search
    :remote-search-loading="productsSeenSearchPending"
    :remote-search-min-chars="productSearchMinChars"
    :remote-search-idle-label="`Digite pelo menos ${productSearchMinChars} digitos do codigo/SKU.`"
    remote-search-loading-label="Buscando produtos..."
    testid-prefix="operation-products-seen"
    @search="emit('search-products-seen', $event)"
    @update:selected-items="emit('update:products-seen', $event)"
    @update:item-details="emit('update:product-seen-details', $event)"
    @update:none-selected="emit('update:products-seen-none', $event)"
  />

  <section v-if="isProductSeenNoneSelected" class="finish-form__section">
    <label class="finish-form__label" for="finish-product-seen-notes">
      {{ productSeenNotesLabel }}
    </label>
    <textarea
      id="finish-product-seen-notes"
      :value="productSeenNotes"
      class="finish-form__textarea"
      rows="3"
      :placeholder="productSeenNotesPlaceholder"
      data-testid="operation-product-seen-notes"
      @input="emit('update:product-seen-notes', $event.target.value)"
    ></textarea>
    <div
      class="finish-form__field-note"
      :class="{
        'finish-form__field-note--error': isProductSeenNotesRequired && !isProductSeenNotesValid,
      }"
    >
      <span>{{ productSeenNotesHelperText }}</span>
      <strong>{{ productSeenNotesLength }}/{{ productSeenNotesMinChars }} caracteres</strong>
    </div>
  </section>

  <section v-if="step1MissingJustifications.length" class="finish-form__section">
    <strong class="finish-form__label">Justificativas pendentes</strong>
    <div class="finish-form__justification-list">
      <div
        v-for="item in step1MissingJustifications"
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

  <div
    class="finish-form__quality"
    :class="isStep1Ready ? 'finish-form__quality--complete' : 'finish-form__quality--incomplete'"
  >
    <div class="finish-form__quality-dots">
      <span
        class="finish-form__quality-dot"
        :class="{ 'is-filled': formStep1Quality.checks.outcome }"
        title="Como terminou"
      ></span>
      <span
        v-if="shouldUsePurchaseCodeField && requirePurchaseCodeField"
        class="finish-form__quality-dot"
        :class="{ 'is-filled': formStep1Quality.checks.purchaseCode }"
        title="Codigo da compra"
      ></span>
      <span
        v-if="shouldUseLegacyClosedProductField && requireProductClosedField"
        class="finish-form__quality-dot"
        :class="{ 'is-filled': formStep1Quality.checks.productClosed }"
        title="Compra / reserva"
      ></span>
      <span
        v-if="showProductSeenField && requireProductSeenField"
        class="finish-form__quality-dot"
        :class="{ 'is-filled': formStep1Quality.checks.productSeen }"
        title="Interesses do cliente"
      ></span>
      <span
        v-if="isProductSeenNotesRequired"
        class="finish-form__quality-dot finish-form__quality-dot--notes"
        :class="{ 'is-filled': formStep1Quality.checks.productSeenNotes }"
        title="Detalhes dos interesses"
      ></span>
      <span
        v-if="step1MissingJustifications.length"
        class="finish-form__quality-dot finish-form__quality-dot--notes"
        :class="{ 'is-filled': !hasInvalidStep1Justifications }"
        title="Justificativas pendentes"
      ></span>
    </div>
    <span class="finish-form__quality-text">
      {{ formStep1Quality.filled }}/{{ formStep1Quality.total }} obrigatorios ·
      {{ isStep1Ready ? 'Pronto para avançar' : 'Preencha antes de continuar' }}
    </span>
  </div>

  <div class="finish-form__actions">
    <button
      class="column-action column-action--secondary"
      type="button"
      data-testid="operation-finish-cancel"
      @click="$emit('next', 'cancel')"
    >
      Cancelar
    </button>
    <button
      class="column-action column-action--primary"
      type="button"
      data-testid="operation-step-next"
      @click="emit('next')"
    >
      Próximo
    </button>
  </div>
</template>