<script setup>
import FinishStepClient from '~/components/operation/finish/FinishStepClient.vue'
import FinishStepNotes from '~/components/operation/finish/FinishStepNotes.vue'
import FinishStepOutcome from '~/components/operation/finish/FinishStepOutcome.vue'
import FinishStepProduct from '~/components/operation/finish/FinishStepProduct.vue'
import { useFinishModalController } from '~/components/operation/finish/useFinishModalController'
import { useOperationsStore } from '~/stores/operations'
import { useUiStore } from '~/stores/ui'
const props = defineProps({
  state: {
    type: Object,
    required: true,
  },
})
const operationsStore = useOperationsStore()
const ui = useUiStore()
const {
  PRODUCT_SEARCH_MIN_CHARS,
  modalConfig,
  service,
  hasRestoredDraft,
  clearCurrentDraft,
  closeModal,
  step,
  step1JustificationsRevealed,
  step2JustificationsRevealed,
  modalTitle,
  serviceDisplayName,
  form,
  showCustomerSection,
  customerSectionLabel,
  showExistingCustomerField,
  existingCustomerLabel,
  showCustomerNameField,
  customerNameLabel,
  showCustomerPhoneField,
  customerPhoneLabel,
  showEmailField,
  customerEmailLabel,
  showProfessionField,
  customerProfessionLabel,
  professionPickerOptions,
  professionSelectedItems,
  updateProfessionSelectedItems,
  showVisitReasonField,
  visitReasonLabel,
  visitReasonPickerOptions,
  visitReasonSelectedItems,
  isVisitReasonMultiple,
  visitReasonDetailsEnabled,
  visitReasonPickerDetailMode,
  updateVisitReasonSelectedItems,
  showCustomerSourceField,
  customerSourceLabel,
  customerSourcePickerOptions,
  customerSourceSelectedItems,
  isCustomerSourceMultiple,
  customerSourceDetailsEnabled,
  customerSourcePickerDetailMode,
  updateCustomerSourceSelectedItems,
  formatPhoneMask,
  syncSelectedDetails,
  shouldUsePurchaseCodeField,
  purchaseCodeLabel,
  purchaseCodePlaceholder,
  shouldUseLegacyClosedProductField,
  closedProductLabel,
  closedProductHelperText,
  productsClosedPickerOptions,
  productsClosedSearch,
  productsClosedEmptyLabel,
  showProductSeenField,
  productSeenLabel,
  productsSeenPickerOptions,
  productsSeenSearch,
  productsSeenEmptyLabel,
  productSeenPlaceholder,
  allowProductSeenNone,
  showProductSeenNotesField,
  productSeenDetailMap,
  productSeenNotesLabel,
  productSeenNotesPlaceholder,
  isProductSeenNoneSelected,
  isProductSeenNotesRequired,
  isProductSeenNotesValid,
  productSeenNotesHelperText,
  trimmedProductSeenNotes,
  productSeenNotesMinChars,
  step1MissingJustifications,
  isFieldJustificationValid,
  getFieldJustificationCharCount,
  formStep1Quality,
  isStep1Ready,
  requirePurchaseCodeField,
  requireProductClosedField,
  requireProductSeenField,
  hasInvalidStep1Justifications,
  updateProductsClosed,
  updateProductsSeen,
  updateProductSeenDetails,
  updateProductsSeenNone,
  goToStep2,
  showQueueJumpReasonField,
  queueJumpReasonLabel,
  queueJumpReasonPickerOptions,
  queueJumpReasonSelectedItems,
  queueJumpReasonPlaceholder,
  updateQueueJumpReasonSelectedItems,
  showLossReasonField,
  lossReasonLabel,
  lossReasonPickerOptions,
  lossReasonSelectedItems,
  isLossReasonMultiple,
  lossReasonDetailsEnabled,
  lossReasonPickerDetailMode,
  lossReasonPlaceholder,
  updateLossReasonSelectedItems,
  showNotesField,
  notesLabel,
  notesPlaceholder,
  step2MissingJustifications,
  formatCurrency,
  closedTotal,
  step2QualityTone,
  formQuality,
  step2QualityLabel,
  requireCustomerNameField,
  requireCustomerPhoneField,
  requireVisitReasonField,
  requireCustomerSourceField,
  requireLossReasonField,
  requireQueueJumpReasonField,
  requireEmailField,
  requireProfessionField,
  requireNotesField,
  hasInvalidStep2Justifications,
  goToStep1,
  submitForm,
} = useFinishModalController(props, operationsStore, ui)
</script>
<template>
  <Teleport to="body">
    <div
      v-if="service"
      class="modal-backdrop"
      data-testid="operation-finish-modal-backdrop"
      @click.self.prevent
    >
      <div
        class="finish-modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="finish-modal-title"
        data-testid="operation-finish-modal"
      >
        <div class="finish-modal__header">
          <div>
            <h2 id="finish-modal-title" class="finish-modal__title">{{ modalTitle }}</h2>
            <p class="finish-modal__subtitle">
              {{ serviceDisplayName(service) }}
            </p>
          </div>
          <div class="finish-modal__header-actions">
            <button
              v-if="hasRestoredDraft"
              class="finish-modal__draft-clear"
              type="button"
              data-testid="operation-finish-clear-draft"
              @click="clearCurrentDraft"
            >
              Limpar modal
            </button>
            <button
              class="finish-modal__close"
              type="button"
              aria-label="Fechar"
              data-testid="operation-finish-close"
              @click="closeModal"
            >
              X
            </button>
          </div>
        </div>
        <div class="finish-modal__steps">
          <div class="finish-modal__step">
            <span
              class="finish-modal__step-dot"
              :class="{ 'is-active': step === 1, 'is-done': step > 1 }"
            >
              1
            </span>
            <span class="finish-modal__step-label" :class="{ 'is-active': step === 1 }">
              Atendimento
            </span>
          </div>
          <div class="finish-modal__step-line" :class="{ 'is-done': step > 1 }"></div>
          <div class="finish-modal__step">
            <span class="finish-modal__step-dot" :class="{ 'is-active': step === 2 }">2</span>
            <span class="finish-modal__step-label" :class="{ 'is-active': step === 2 }">
              Cliente
            </span>
          </div>
        </div>
        <form class="finish-form" @submit.prevent="submitForm">
          <template v-if="step === 1">
            <FinishStepOutcome
              :outcome="form.outcome"
              :should-use-purchase-code-field="shouldUsePurchaseCodeField"
              :purchase-code-label="purchaseCodeLabel"
              :purchase-code="form.purchaseCode"
              :purchase-code-placeholder="purchaseCodePlaceholder"
              @update:outcome="form.outcome = $event"
              @update:purchase-code="form.purchaseCode = $event"
            />

            <FinishStepProduct
              :justifications-revealed="step1JustificationsRevealed"
              :should-use-legacy-closed-product-field="shouldUseLegacyClosedProductField"
              :closed-product-label="closedProductLabel"
              :closed-product-helper-text="closedProductHelperText"
              :products-closed-picker-options="productsClosedPickerOptions"
              :products-closed="form.productsClosed"
              :product-closed-placeholder="
                modalConfig.productClosedPlaceholder || 'Digite 3 primeiros digitos do codigo/SKU'
              "
              :products-closed-empty-label="productsClosedEmptyLabel"
              :products-closed-search-pending="productsClosedSearch.state.pending"
              :show-product-seen-field="showProductSeenField"
              :product-seen-label="productSeenLabel"
              :products-seen-picker-options="productsSeenPickerOptions"
              :products-seen="form.productsSeen"
              :products-seen-none="form.productsSeenNone"
              :product-seen-placeholder="
                productSeenPlaceholder || 'Digite 3 primeiros digitos do codigo/SKU'
              "
              :products-seen-empty-label="productsSeenEmptyLabel"
              :allow-product-seen-none="allowProductSeenNone"
              :show-product-seen-notes-field="showProductSeenNotesField"
              :product-seen-detail-map="productSeenDetailMap"
              :product-seen-notes-label="productSeenNotesLabel"
              :product-seen-notes-placeholder="productSeenNotesPlaceholder"
              :products-seen-search-pending="productsSeenSearch.state.pending"
              :product-search-min-chars="PRODUCT_SEARCH_MIN_CHARS"
              :is-product-seen-none-selected="isProductSeenNoneSelected"
              :product-seen-notes="form.productSeenNotes"
              :is-product-seen-notes-required="isProductSeenNotesRequired"
              :is-product-seen-notes-valid="isProductSeenNotesValid"
              :product-seen-notes-helper-text="productSeenNotesHelperText"
              :product-seen-notes-length="trimmedProductSeenNotes.length"
              :product-seen-notes-min-chars="productSeenNotesMinChars"
              :step1-missing-justifications="step1MissingJustifications"
              :field-justifications="form.fieldJustifications"
              :is-field-justification-valid="isFieldJustificationValid"
              :get-field-justification-char-count="getFieldJustificationCharCount"
              :form-step1-quality="formStep1Quality"
              :is-step1-ready="isStep1Ready"
              :should-use-purchase-code-field="shouldUsePurchaseCodeField"
              :require-purchase-code-field="requirePurchaseCodeField"
              :require-product-closed-field="requireProductClosedField"
              :require-product-seen-field="requireProductSeenField"
              :has-invalid-step1-justifications="hasInvalidStep1Justifications"
              @search-products-closed="productsClosedSearch.search"
              @update:products-closed="updateProductsClosed"
              @search-products-seen="productsSeenSearch.search"
              @update:products-seen="updateProductsSeen"
              @update:product-seen-details="updateProductSeenDetails"
              @update:products-seen-none="updateProductsSeenNone"
              @update:product-seen-notes="form.productSeenNotes = $event"
              @update:field-justification="form.fieldJustifications[$event.key] = $event.value"
              @next="$event === 'cancel' ? closeModal() : goToStep2()"
            />
          </template>
          <template v-if="step === 2">
            <FinishStepClient
              :show-customer-section="showCustomerSection"
              :customer-section-label="customerSectionLabel"
              :show-existing-customer-field="showExistingCustomerField"
              :existing-customer-label="existingCustomerLabel"
              :is-existing-customer="form.isExistingCustomer"
              :show-customer-name-field="showCustomerNameField"
              :customer-name-label="customerNameLabel"
              :customer-name="form.customerName"
              :show-customer-phone-field="showCustomerPhoneField"
              :customer-phone-label="customerPhoneLabel"
              :customer-phone="form.customerPhone"
              :show-email-field="showEmailField"
              :customer-email-label="customerEmailLabel"
              :customer-email="form.customerEmail"
              :show-profession-field="showProfessionField"
              :customer-profession-label="customerProfessionLabel"
              :profession-picker-options="professionPickerOptions"
              :profession-selected-items="professionSelectedItems"
              :show-visit-reason-field="showVisitReasonField"
              :visit-reason-label="visitReasonLabel"
              :visit-reason-picker-options="visitReasonPickerOptions"
              :visit-reason-selected-items="visitReasonSelectedItems"
              :is-visit-reason-multiple="isVisitReasonMultiple"
              :visit-reason-details-enabled="visitReasonDetailsEnabled"
              :visit-reason-picker-detail-mode="visitReasonPickerDetailMode"
              :visit-reason-details="form.visitReasonDetails"
              :visit-reason-not-informed="form.visitReasonNotInformed"
              :show-customer-source-field="showCustomerSourceField"
              :customer-source-label="customerSourceLabel"
              :customer-source-picker-options="customerSourcePickerOptions"
              :customer-source-selected-items="customerSourceSelectedItems"
              :is-customer-source-multiple="isCustomerSourceMultiple"
              :customer-source-details-enabled="customerSourceDetailsEnabled"
              :customer-source-picker-detail-mode="customerSourcePickerDetailMode"
              :customer-source-details="form.customerSourceDetails"
              :customer-source-not-informed="form.customerSourceNotInformed"
              :format-phone-mask="formatPhoneMask"
              @update:is-existing-customer="form.isExistingCustomer = $event"
              @update:customer-name="form.customerName = $event"
              @update:customer-phone="form.customerPhone = $event"
              @update:customer-email="form.customerEmail = $event"
              @update:profession-selected-items="updateProfessionSelectedItems"
              @update:visit-reason-selected-items="updateVisitReasonSelectedItems"
              @update:visit-reason-details="
                form.visitReasonDetails = syncSelectedDetails(form.visitReasonIds, $event)
              "
              @update:visit-reason-not-informed="form.visitReasonNotInformed = $event"
              @update:customer-source-selected-items="updateCustomerSourceSelectedItems"
              @update:customer-source-details="
                form.customerSourceDetails = syncSelectedDetails(form.customerSourceIds, $event)
              "
              @update:customer-source-not-informed="form.customerSourceNotInformed = $event"
            />
            <FinishStepNotes
              :justifications-revealed="step2JustificationsRevealed"
              :is-queue-jump-service="service.startMode === 'queue-jump'"
              :show-queue-jump-reason-field="showQueueJumpReasonField"
              :queue-jump-reason-label="queueJumpReasonLabel"
              :queue-jump-reason-picker-options="queueJumpReasonPickerOptions"
              :queue-jump-reason-selected-items="queueJumpReasonSelectedItems"
              :queue-jump-reason-placeholder="queueJumpReasonPlaceholder"
              :show-loss-reason-section="form.outcome === 'nao-compra' && showLossReasonField"
              :loss-reason-label="lossReasonLabel"
              :loss-reason-picker-options="lossReasonPickerOptions"
              :loss-reason-selected-items="lossReasonSelectedItems"
              :is-loss-reason-multiple="isLossReasonMultiple"
              :loss-reason-details-enabled="lossReasonDetailsEnabled"
              :loss-reason-picker-detail-mode="lossReasonPickerDetailMode"
              :loss-reason-details="form.lossReasonDetails"
              :loss-reason-placeholder="lossReasonPlaceholder"
              :show-notes-field="showNotesField"
              :notes-label="notesLabel"
              :notes="form.notes"
              :notes-placeholder="notesPlaceholder"
              :step2-missing-justifications="step2MissingJustifications"
              :field-justifications="form.fieldJustifications"
              :is-field-justification-valid="isFieldJustificationValid"
              :get-field-justification-char-count="getFieldJustificationCharCount"
              :should-use-legacy-closed-product-field="shouldUseLegacyClosedProductField"
              :formatted-closed-total="formatCurrency(closedTotal)"
              :step2-quality-tone="step2QualityTone"
              :form-quality="formQuality"
              :step2-quality-label="step2QualityLabel"
              :show-customer-name-field="showCustomerNameField"
              :require-customer-name-field="requireCustomerNameField"
              :show-customer-phone-field="showCustomerPhoneField"
              :require-customer-phone-field="requireCustomerPhoneField"
              :should-use-purchase-code-field="shouldUsePurchaseCodeField"
              :require-purchase-code-field="requirePurchaseCodeField"
              :require-product-closed-field="requireProductClosedField"
              :show-product-seen-field="showProductSeenField"
              :require-product-seen-field="requireProductSeenField"
              :is-product-seen-notes-required="isProductSeenNotesRequired"
              :show-visit-reason-field="showVisitReasonField"
              :require-visit-reason-field="requireVisitReasonField"
              :show-customer-source-field="showCustomerSourceField"
              :require-customer-source-field="requireCustomerSourceField"
              :is-loss-outcome="form.outcome === 'nao-compra'"
              :show-loss-reason-field="showLossReasonField"
              :require-loss-reason-field="requireLossReasonField"
              :require-queue-jump-reason-field="requireQueueJumpReasonField"
              :show-email-field="showEmailField"
              :require-email-field="requireEmailField"
              :show-profession-field="showProfessionField"
              :require-profession-field="requireProfessionField"
              :require-notes-field="requireNotesField"
              :has-invalid-step2-justifications="hasInvalidStep2Justifications"
              @update:queue-jump-reason-selected-items="updateQueueJumpReasonSelectedItems"
              @update:loss-reason-selected-items="updateLossReasonSelectedItems"
              @update:loss-reason-details="
                form.lossReasonDetails = syncSelectedDetails(form.lossReasonIds, $event)
              "
              @update:notes="form.notes = $event"
              @update:field-justification="form.fieldJustifications[$event.key] = $event.value"
              @previous="goToStep1"
              @submit="submitForm"
            />
          </template>
        </form>
      </div>
    </div>
  </Teleport>
</template>
<style scoped>
.finish-form__justification-list {
  display: grid;
  gap: 12px;
}
.finish-form__justification-item {
  display: grid;
  gap: 8px;
}
</style>
