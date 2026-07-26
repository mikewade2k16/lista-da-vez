import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { buildNickname } from '~/domain/utils/person-display'
import { getApiErrorMessage } from '~/utils/api-client'

const FINISH_MODAL_DRAFT_STORAGE_KEY = 'ldv_finish_modal_drafts_v1'
const FINISH_MODAL_DRAFT_MAX_AGE_MS = 1000 * 60 * 60 * 24
const PRODUCT_SEEN_NONE_DETAIL_KEY = '__none__'
const PRODUCT_CATALOG_SOURCE_KEY = 'erp_current'
const PRODUCT_SEARCH_MIN_CHARS = 3
const PRODUCT_SEARCH_LIMIT = 12
const FIELD_JUSTIFICATION_KEYS = [
  'customerName',
  'customerPhone',
  'email',
  'profession',
  'existingCustomer',
  'notes',
  'productSeen',
  'productSeenNotes',
  'productClosed',
  'purchaseCode',
  'visitReason',
  'customerSource',
  'queueJumpReason',
  'lossReason',
]

function serviceDisplayName(service) {
  return buildNickname(service?.name || '')
}

function readDraftStorage() {
  if (import.meta.server) {
    return {}
  }

  try {
    const parsed = JSON.parse(window.sessionStorage.getItem(FINISH_MODAL_DRAFT_STORAGE_KEY) || '{}')
    return parsed &&
      typeof parsed === 'object' &&
      parsed.drafts &&
      typeof parsed.drafts === 'object'
      ? parsed.drafts
      : {}
  } catch {
    return {}
  }
}

function writeDraftStorage(drafts) {
  if (import.meta.server) {
    return
  }

  const now = Date.now()
  const normalizedDrafts = Object.fromEntries(
    Object.entries(drafts || {}).filter(
      ([, entry]) =>
        entry &&
        typeof entry === 'object' &&
        now - Number(entry.updatedAt || 0) <= FINISH_MODAL_DRAFT_MAX_AGE_MS,
    ),
  )

  if (Object.keys(normalizedDrafts).length === 0) {
    window.sessionStorage.removeItem(FINISH_MODAL_DRAFT_STORAGE_KEY)
    return
  }

  window.sessionStorage.setItem(
    FINISH_MODAL_DRAFT_STORAGE_KEY,
    JSON.stringify({
      version: 1,
      drafts: normalizedDrafts,
    }),
  )
}

function removeStoredDraft(draftKey) {
  const normalizedKey = String(draftKey || '').trim()

  if (!normalizedKey) {
    return
  }

  const drafts = readDraftStorage()
  delete drafts[normalizedKey]
  writeDraftStorage(drafts)
}

function createEmptyFieldJustifications() {
  return Object.fromEntries(FIELD_JUSTIFICATION_KEYS.map((key) => [key, '']))
}

function normalizeFieldJustifications(values) {
  const defaults = createEmptyFieldJustifications()

  if (!values || typeof values !== 'object') {
    return defaults
  }

  return Object.fromEntries(
    FIELD_JUSTIFICATION_KEYS.map((key) => [key, String(values?.[key] || '')]),
  )
}

function createEmptyForm() {
  return {
    outcome: '',
    isExistingCustomer: false,
    purchaseCode: '',
    productsSeen: [],
    productsClosed: [],
    productsSeenNone: false,
    productSeenNotes: '',
    customerName: '',
    customerPhone: '',
    customerEmail: '',
    customerProfessionId: '',
    visitReasonIds: [],
    visitReasonNotInformed: false,
    visitReasonDetails: {},
    customerSourceIds: [],
    customerSourceNotInformed: false,
    customerSourceDetails: {},
    queueJumpReasonId: '',
    lossReasonIds: [],
    lossReasonDetails: {},
    notes: '',
    fieldJustifications: createEmptyFieldJustifications(),
  }
}

function normalizeIdList(values = []) {
  return [
    ...new Set(
      (Array.isArray(values) ? values : [])
        .map((value) => String(value || '').trim())
        .filter(Boolean),
    ),
  ]
}

function syncSelectedDetails(itemIds = [], details = {}) {
  return Object.fromEntries(
    normalizeIdList(itemIds).map((itemId) => [itemId, String(details?.[itemId] || '')]),
  )
}

function findOptionByLabel(options, label) {
  const normalizedLabel = String(label || '')
    .trim()
    .toLowerCase()

  if (!normalizedLabel) {
    return null
  }

  return (
    (options || []).find(
      (item) =>
        String(item?.label || '')
          .trim()
          .toLowerCase() === normalizedLabel,
    ) || null
  )
}

function normalizeProducts(items = []) {
  return (Array.isArray(items) ? items : []).map((item, index) => ({
    id: String(item?.id || `${item?.name || 'produto'}-${index}`),
    name: String(item?.name || '').trim(),
    label: String(item?.label || item?.name || '').trim(),
    price: Math.max(0, Number(item?.price ?? item?.basePrice ?? 0) || 0),
    code: String(item?.code || '').trim(),
    isCustom: Boolean(item?.isCustom),
  }))
}

function getProductIdentity(product) {
  const code = String(product?.code || '')
    .trim()
    .toLowerCase()
  const id = String(product?.id || '')
    .trim()
    .toLowerCase()
  const name = String(product?.name || product?.label || '')
    .trim()
    .toLowerCase()

  return code ? `code:${code}` : id ? `id:${id}` : name ? `name:${name}` : ''
}

function mergeProductEntries(...groups) {
  const seen = new Set()
  const merged = []

  groups.flat().forEach((product) => {
    const normalized = normalizeProducts([product])[0]
    const identity = getProductIdentity(normalized)

    if (!identity || seen.has(identity)) {
      return
    }

    seen.add(identity)
    merged.push(normalized)
  })

  return merged
}

function normalizeProductPickerOptions(items = []) {
  return normalizeProducts(items)
    .map((item) => ({
      ...item,
      label:
        String(item?.label || item?.name || item?.code || '').trim() ||
        String(item?.code || '').trim(),
      name: String(item?.name || item?.label || item?.code || '').trim(),
      code: String(item?.code || '')
        .trim()
        .toUpperCase(),
      price: Math.max(0, Number(item?.price || 0) || 0),
    }))
    .filter((item) => item.id || item.name || item.code)
}

function buildProductSearchEmptyLabel(searchState) {
  if (String(searchState?.error || '').trim()) {
    return searchState.error
  }

  if (searchState?.pending) {
    return 'Buscando produtos...'
  }

  if (String(searchState?.term || '').trim().length < PRODUCT_SEARCH_MIN_CHARS) {
    return `Digite pelo menos ${PRODUCT_SEARCH_MIN_CHARS} digitos do codigo/SKU.`
  }

  return 'Nenhum produto encontrado para a busca atual.'
}

function buildInitialForm(state, draft) {
  const currentDraft = draft || {}
  const selectedVisitReasonIds = normalizeIdList(
    currentDraft.visitReasonIds || currentDraft.visitReasons,
  )
  const selectedSourceIds = normalizeIdList(
    Array.isArray(currentDraft.customerSourceIds) || Array.isArray(currentDraft.customerSources)
      ? currentDraft.customerSourceIds || currentDraft.customerSources
      : currentDraft.customerSource
        ? [currentDraft.customerSource]
        : [],
  )
  const selectedProfession =
    (state.professionOptions || []).find(
      (option) => option.id === String(currentDraft.customerProfessionId || ''),
    ) || findOptionByLabel(state.professionOptions, currentDraft.customerProfession)
  const selectedQueueJumpReason =
    (state.queueJumpReasonOptions || []).find(
      (option) => option.id === String(currentDraft.queueJumpReasonId || ''),
    ) || findOptionByLabel(state.queueJumpReasonOptions, currentDraft.queueJumpReason)
  const selectedLossReasonIds = normalizeIdList(
    Array.isArray(currentDraft.lossReasonIds) || Array.isArray(currentDraft.lossReasons)
      ? currentDraft.lossReasonIds || currentDraft.lossReasons
      : currentDraft.lossReasonId
        ? [currentDraft.lossReasonId]
        : [],
  )
  const selectedLossReason = selectedLossReasonIds[0]
    ? (state.lossReasonOptions || []).find((option) => option.id === selectedLossReasonIds[0]) ||
      null
    : findOptionByLabel(state.lossReasonOptions, currentDraft.lossReason)
  const resolvedLossReasonIds = selectedLossReasonIds.length
    ? selectedLossReasonIds
    : selectedLossReason?.id
      ? [selectedLossReason.id]
      : []

  return {
    outcome: String(currentDraft.outcome || ''),
    isExistingCustomer: Boolean(currentDraft.isExistingCustomer),
    purchaseCode: String(currentDraft.purchaseCode || ''),
    productsSeen: normalizeProducts(currentDraft.productsSeen),
    productsClosed: normalizeProducts(currentDraft.productsClosed),
    productsSeenNone: Boolean(currentDraft.productsSeenNone),
    productSeenNotes: String(
      currentDraft.productSeenNotes ||
        (Array.isArray(currentDraft.productsSeen) && currentDraft.productsSeen.length
          ? ''
          : currentDraft.productSeen) ||
        '',
    ),
    customerName: String(currentDraft.customerName || ''),
    customerPhone: String(currentDraft.customerPhone || ''),
    customerEmail: String(currentDraft.customerEmail || ''),
    customerProfessionId: selectedProfession?.id || '',
    visitReasonIds: selectedVisitReasonIds,
    visitReasonNotInformed:
      Boolean(currentDraft.visitReasonsNotInformed) && selectedVisitReasonIds.length === 0,
    visitReasonDetails: syncSelectedDetails(
      selectedVisitReasonIds,
      currentDraft.visitReasonDetails,
    ),
    customerSourceIds: selectedSourceIds,
    customerSourceNotInformed:
      Boolean(currentDraft.customerSourcesNotInformed) && selectedSourceIds.length === 0,
    customerSourceDetails: syncSelectedDetails(
      selectedSourceIds,
      currentDraft.customerSourceDetails && typeof currentDraft.customerSourceDetails === 'object'
        ? currentDraft.customerSourceDetails
        : selectedSourceIds[0]
          ? { [selectedSourceIds[0]]: String(currentDraft.customerSourceDetail || '') }
          : {},
    ),
    queueJumpReasonId: selectedQueueJumpReason?.id || '',
    lossReasonIds: String(currentDraft.outcome || '') === 'nao-compra' ? resolvedLossReasonIds : [],
    lossReasonDetails: syncSelectedDetails(
      resolvedLossReasonIds,
      currentDraft.lossReasonDetails && typeof currentDraft.lossReasonDetails === 'object'
        ? currentDraft.lossReasonDetails
        : {},
    ),
    notes: String(currentDraft.notes || ''),
    fieldJustifications: normalizeFieldJustifications(currentDraft.fieldJustifications),
  }
}

function formatCurrency(value) {
  return Number(value || 0).toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' })
}

function formatPhoneMask(value) {
  const digits = String(value || '')
    .replace(/\D/g, '')
    .slice(0, 11)

  if (!digits) {
    return ''
  }

  if (digits.length <= 2) {
    return `(${digits}`
  }

  if (digits.length <= 6) {
    return `(${digits.slice(0, 2)}) ${digits.slice(2)}`
  }

  if (digits.length <= 10) {
    return `(${digits.slice(0, 2)}) ${digits.slice(2, 6)}-${digits.slice(6)}`
  }

  return `(${digits.slice(0, 2)}) ${digits.slice(2, 7)}-${digits.slice(7)}`
}

function mapOptionToPickerItem(option, meta = '') {
  return {
    id: String(option?.id || ''),
    label: String(option?.label || option?.name || '').trim(),
    meta: String(meta || '').trim(),
  }
}

function resolveModalText(value, fallback) {
  const normalizedValue = String(value || '').trim()
  return normalizedValue || fallback
}

function resolveModalBoolean(value, fallback = false) {
  return typeof value === 'boolean' ? value : fallback
}

function resolveModalNumber(value, fallback = 0, minimum = 0) {
  return Math.max(minimum, Number(value ?? fallback) || fallback || 0)
}

function hasTrimmedText(value) {
  return String(value || '').trim().length > 0
}

function countCharsIgnoringWhitespace(value) {
  return String(value || '').replace(/\s+/g, '').length
}

export function useFinishModalController(props, operationsStore, ui) {
  const form = reactive(createEmptyForm())
  const step = ref(1)
  const customProducts = ref([])
  const restoredDraftKey = ref('')
  // Justificativas de campo nao-preenchido so aparecem na TENTATIVA de avancar
  // (passo 1) ou finalizar (passo 2) — uma flag por passo para nao revelar o
  // passo 2 cedo demais ao avancar.
  const step1JustificationsRevealed = ref(false)
  const step2JustificationsRevealed = ref(false)
  // Justificativa OBRIGATORIA do encerramento de pendencia (auto-encerramento 2h):
  // por que o consultor nao encerrou na hora. Ref proprio (fora do form) para nao
  // entrar no draft persistido; resetado quando o modal fecha/troca de servico.
  const validationReason = ref('')
  let isApplyingDraft = false

  function createProductCatalogSearchState() {
    const state = reactive({
      term: '',
      options: [],
      pending: false,
      error: '',
      requestToken: 0,
    })

    async function search(term) {
      const normalizedTerm = String(term || '')
        .trim()
        .toUpperCase()
      state.term = normalizedTerm
      state.error = ''

      if (normalizedTerm.length < PRODUCT_SEARCH_MIN_CHARS) {
        state.requestToken += 1
        state.pending = false
        state.options = []
        return
      }

      const normalizedStoreId = String(productCatalogStoreId.value || '').trim()
      if (!normalizedStoreId) {
        state.requestToken += 1
        state.pending = false
        state.options = []
        state.error = 'Loja ativa indisponivel para buscar produtos.'
        return
      }

      const requestToken = state.requestToken + 1
      state.requestToken = requestToken
      state.pending = true

      try {
        const response = await operationsStore.searchCatalogProducts({
          storeId: normalizedStoreId,
          sourceKey: PRODUCT_CATALOG_SOURCE_KEY,
          term: normalizedTerm,
          limit: PRODUCT_SEARCH_LIMIT,
        })

        if (requestToken !== state.requestToken) {
          return
        }

        state.options = normalizeProductPickerOptions(response?.items)
      } catch (error) {
        if (requestToken !== state.requestToken) {
          return
        }

        state.options = []
        state.error = getApiErrorMessage(error, 'Nao foi possivel buscar produtos agora.')
      } finally {
        if (requestToken === state.requestToken) {
          state.pending = false
        }
      }
    }

    function clear() {
      state.requestToken += 1
      state.term = ''
      state.options = []
      state.pending = false
      state.error = ''
    }

    return {
      state,
      search,
      clear,
    }
  }

  const productsClosedSearch = createProductCatalogSearchState()
  const productsSeenSearch = createProductCatalogSearchState()

  const modalConfig = computed(() => props.state.modalConfig || {})
  const finishFlowMode = computed(() =>
    String(modalConfig.value.finishFlowMode || '').trim() === 'erp-reconciliation'
      ? 'erp-reconciliation'
      : 'legacy',
  )
  const isERPReconciliationFlow = computed(() => finishFlowMode.value === 'erp-reconciliation')

  const activeServiceMatch = computed(
    () =>
      (props.state.activeServices || []).find(
        (item) => item.serviceId === props.state.finishModalServiceId,
      ) || null,
  )
  // Pendencia de auto-encerramento (2h): o servico ja saiu de activeServices, mas a
  // gestao encerra pelo MESMO modal. Resolve um service-like a partir da pendencia
  // (timer congelado via effectiveFinishedAt; startMode queue evita o passo de
  // motivo fora-da-vez). O submit vai para POST /validate em vez de /finish.
  const pendingValidationMatch = computed(() => {
    const pendingFromList = (props.state.pendingValidations || []).find(
      (item) => item.serviceId === props.state.finishModalServiceId,
    )
    if (pendingFromList) {
      return pendingFromList
    }

    const transientPending = props.state.finishModalPendingValidation
    return transientPending?.serviceId === props.state.finishModalServiceId
      ? transientPending
      : null
  })
  const isPendingValidation = computed(
    () => !activeServiceMatch.value && Boolean(pendingValidationMatch.value),
  )
  const service = computed(() => {
    if (activeServiceMatch.value) {
      return activeServiceMatch.value
    }

    const pending = pendingValidationMatch.value
    if (!pending) {
      return null
    }

    return {
      serviceId: pending.serviceId,
      id: pending.personId,
      name: pending.personName,
      storeId: pending.storeId,
      storeName: pending.storeName || '',
      serviceStartedAt: Number(pending.startedAt || 0),
      stoppedAt: Number(pending.finishedAt || 0),
      effectiveFinishedAt: Number(pending.finishedAt || 0),
      startMode: 'queue',
      skippedPeople: [],
    }
  })
  const productCatalogStoreId = computed(() =>
    String(service.value?.storeId || props.state.activeStoreId || '').trim(),
  )
  const draft = computed(() => props.state.finishModalDraft || null)
  const requestedServiceDraftKey = computed(() => {
    const storeId = String(props.state.activeStoreId || '').trim()
    const serviceId = String(props.state.finishModalServiceId || '').trim()

    return storeId && serviceId ? `${storeId}:${serviceId}` : ''
  })
  const serviceDraftKey = computed(() => {
    const currentService = service.value
    const storeId = String(props.state.activeStoreId || '').trim()
    const serviceId = String(currentService?.serviceId || '').trim()

    return storeId && serviceId ? `${storeId}:${serviceId}` : ''
  })
  const hasRestoredDraft = computed(() =>
    Boolean(restoredDraftKey.value && restoredDraftKey.value === serviceDraftKey.value),
  )

  const isClosedOutcome = computed(() => form.outcome === 'compra' || form.outcome === 'reserva')
  const isPurchaseOutcome = computed(() => form.outcome === 'compra')
  const showCustomerNameField = computed(
    () =>
      !isPurchaseOutcome.value &&
      resolveModalBoolean(modalConfig.value.showCustomerNameField, true),
  )
  const showCustomerPhoneField = computed(
    () =>
      !isPurchaseOutcome.value &&
      resolveModalBoolean(modalConfig.value.showCustomerPhoneField, true),
  )
  const showEmailField = computed(
    () => !isPurchaseOutcome.value && resolveModalBoolean(modalConfig.value.showEmailField, true),
  )
  const showProfessionField = computed(() =>
    resolveModalBoolean(modalConfig.value.showProfessionField, true),
  )
  const showNotesField = computed(() => resolveModalBoolean(modalConfig.value.showNotesField, true))
  const showProductSeenField = computed(() =>
    resolveModalBoolean(modalConfig.value.showProductSeenField, true),
  )
  const showProductSeenNotesField = computed(() =>
    resolveModalBoolean(modalConfig.value.showProductSeenNotesField, true),
  )
  const showProductClosedField = computed(
    () =>
      !isPurchaseOutcome.value &&
      resolveModalBoolean(modalConfig.value.showProductClosedField, true),
  )
  const showPurchaseCodeField = computed(() =>
    resolveModalBoolean(modalConfig.value.showPurchaseCodeField, true),
  )
  const showVisitReasonField = computed(() =>
    resolveModalBoolean(modalConfig.value.showVisitReasonField, true),
  )
  const showCustomerSourceField = computed(() =>
    resolveModalBoolean(modalConfig.value.showCustomerSourceField, true),
  )
  const showExistingCustomerField = computed(
    () =>
      !isPurchaseOutcome.value &&
      resolveModalBoolean(modalConfig.value.showExistingCustomerField, true),
  )
  const showQueueJumpReasonField = computed(() =>
    resolveModalBoolean(modalConfig.value.showQueueJumpReasonField, true),
  )
  const showLossReasonField = computed(() =>
    resolveModalBoolean(modalConfig.value.showLossReasonField, true),
  )

  const requireCustomerNameField = computed(() =>
    resolveModalBoolean(
      modalConfig.value.requireCustomerNameField,
      resolveModalBoolean(modalConfig.value.requireCustomerNamePhone, true),
    ),
  )
  const requireCustomerPhoneField = computed(() =>
    resolveModalBoolean(
      modalConfig.value.requireCustomerPhoneField,
      resolveModalBoolean(modalConfig.value.requireCustomerNamePhone, true),
    ),
  )
  const requireEmailField = computed(() =>
    resolveModalBoolean(modalConfig.value.requireEmailField, false),
  )
  const requireProfessionField = computed(() =>
    resolveModalBoolean(modalConfig.value.requireProfessionField, false),
  )
  const requireNotesField = computed(() =>
    resolveModalBoolean(modalConfig.value.requireNotesField, false),
  )
  const requireProductSeenField = computed(() =>
    resolveModalBoolean(
      modalConfig.value.requireProductSeenField,
      resolveModalBoolean(modalConfig.value.requireProduct, true),
    ),
  )
  const requireProductSeenNotesField = computed(() =>
    resolveModalBoolean(modalConfig.value.requireProductSeenNotesField, false),
  )
  const requireProductClosedField = computed(() =>
    resolveModalBoolean(
      modalConfig.value.requireProductClosedField,
      resolveModalBoolean(modalConfig.value.requireProduct, true),
    ),
  )
  const requirePurchaseCodeField = computed(() =>
    resolveModalBoolean(modalConfig.value.requirePurchaseCodeField, true),
  )
  const requireVisitReasonField = computed(() =>
    resolveModalBoolean(modalConfig.value.requireVisitReason, true),
  )
  const requireCustomerSourceField = computed(() =>
    resolveModalBoolean(modalConfig.value.requireCustomerSource, true),
  )
  const allowProductSeenNone = computed(() =>
    resolveModalBoolean(modalConfig.value.allowProductSeenNone, true),
  )
  const requireProductSeenNotesWhenNone = computed(() =>
    resolveModalBoolean(modalConfig.value.requireProductSeenNotesWhenNone, true),
  )
  const productSeenNotesMinChars = computed(() =>
    resolveModalNumber(modalConfig.value.productSeenNotesMinChars, 20, 1),
  )
  const requireQueueJumpReasonField = computed(() =>
    resolveModalBoolean(modalConfig.value.requireQueueJumpReasonField, true),
  )
  const requireLossReasonField = computed(() =>
    resolveModalBoolean(modalConfig.value.requireLossReasonField, true),
  )
  const showCustomerSection = computed(
    () =>
      showCustomerNameField.value ||
      showCustomerPhoneField.value ||
      showEmailField.value ||
      showProfessionField.value ||
      showExistingCustomerField.value ||
      showCustomerSourceField.value ||
      showNotesField.value,
  )

  const visitReasonSelectionMode = computed(() =>
    modalConfig.value.visitReasonSelectionMode === 'single' ? 'single' : 'multiple',
  )
  const lossReasonSelectionMode = computed(() =>
    modalConfig.value.lossReasonSelectionMode === 'multiple' ? 'multiple' : 'single',
  )
  const customerSourceSelectionMode = computed(() =>
    modalConfig.value.customerSourceSelectionMode === 'multiple' ? 'multiple' : 'single',
  )
  const isVisitReasonMultiple = computed(() => visitReasonSelectionMode.value === 'multiple')
  const isLossReasonMultiple = computed(() => lossReasonSelectionMode.value === 'multiple')
  const isCustomerSourceMultiple = computed(() => customerSourceSelectionMode.value === 'multiple')
  const visitReasonConfiguredDetailMode = computed(() => {
    const configuredMode = modalConfig.value.visitReasonDetailMode

    if (['off', 'shared', 'per-item'].includes(configuredMode)) {
      return configuredMode
    }

    return modalConfig.value.showVisitReasonDetails === false ? 'off' : 'shared'
  })
  const lossReasonConfiguredDetailMode = computed(() => {
    const configuredMode = modalConfig.value.lossReasonDetailMode

    if (['off', 'shared', 'per-item'].includes(configuredMode)) {
      return configuredMode
    }

    return 'off'
  })
  const customerSourceConfiguredDetailMode = computed(() => {
    const configuredMode = modalConfig.value.customerSourceDetailMode

    if (['off', 'shared', 'per-item'].includes(configuredMode)) {
      return configuredMode
    }

    return modalConfig.value.showCustomerSourceDetails === false ? 'off' : 'shared'
  })
  const visitReasonDetailsEnabled = computed(() => visitReasonConfiguredDetailMode.value !== 'off')
  const lossReasonDetailsEnabled = computed(() => lossReasonConfiguredDetailMode.value !== 'off')
  const customerSourceDetailsEnabled = computed(
    () => customerSourceConfiguredDetailMode.value !== 'off',
  )
  const visitReasonPickerDetailMode = computed(() =>
    visitReasonConfiguredDetailMode.value === 'per-item' ? 'per-item' : 'shared',
  )
  const lossReasonPickerDetailMode = computed(() =>
    lossReasonConfiguredDetailMode.value === 'per-item' ? 'per-item' : 'shared',
  )
  const customerSourcePickerDetailMode = computed(() =>
    customerSourceConfiguredDetailMode.value === 'per-item' ? 'per-item' : 'shared',
  )

  const shouldUsePurchaseCodeField = computed(
    () => isERPReconciliationFlow.value && isPurchaseOutcome.value && showPurchaseCodeField.value,
  )
  const shouldUseLegacyClosedProductField = computed(
    () =>
      isClosedOutcome.value &&
      showProductClosedField.value &&
      (!isERPReconciliationFlow.value || form.outcome === 'reserva'),
  )
  const trimmedPurchaseCode = computed(() => String(form.purchaseCode || '').trim())
  const trimmedProductSeenNotes = computed(() => String(form.productSeenNotes || '').trim())
  const productSeenNotesLabel = computed(() =>
    resolveModalText(modalConfig.value.productSeenNotesLabel, 'Observação dos interesses'),
  )
  const productSeenNotesPlaceholder = computed(() =>
    resolveModalText(
      modalConfig.value.productSeenNotesPlaceholder,
      'Descreva referência, pedido específico, contexto do cliente ou justificativa quando não houver interesse identificado.',
    ),
  )
  const closedProductLabel = computed(() => {
    const configuredLabel = String(modalConfig.value.productClosedLabel || '').trim()
    if (
      configuredLabel &&
      !['produto reservado/comprado', 'produto fechado'].includes(configuredLabel.toLowerCase())
    ) {
      return configuredLabel
    }

    if (form.outcome === 'compra') {
      return 'Compra'
    }

    if (form.outcome === 'reserva') {
      return 'Reserva'
    }

    return 'Fechamento'
  })
  const closedProductHelperText = computed(() => {
    if (form.outcome === 'compra') {
      return ''
    }

    if (form.outcome === 'reserva') {
      return ''
    }

    return 'Registre o item fechado quando o atendimento terminar em compra ou reserva.'
  })
  const purchaseCodeLabel = computed(() =>
    resolveModalText(modalConfig.value.purchaseCodeLabel, 'Codigo da compra'),
  )
  const purchaseCodePlaceholder = computed(() =>
    resolveModalText(
      modalConfig.value.purchaseCodePlaceholder,
      'Informe o codigo da compra para conciliacao posterior',
    ),
  )
  const selectedProfessionLabel = computed(
    () =>
      props.state.professionOptions.find((option) => option.id === form.customerProfessionId)
        ?.label || '',
  )
  const selectedVisitReasonIdSet = computed(() => new Set(normalizeIdList(form.visitReasonIds)))
  const selectedLossReasonIdSet = computed(() => new Set(normalizeIdList(form.lossReasonIds)))
  const selectedCustomerSourceIdSet = computed(
    () => new Set(normalizeIdList(form.customerSourceIds)),
  )
  const closedTotal = computed(() =>
    form.productsClosed.reduce((sum, product) => sum + (Number(product.price) || 0), 0),
  )

  const canUseProductSeenNotes = computed(
    () =>
      showProductSeenField.value &&
      showProductSeenNotesField.value &&
      (form.productsSeen.length > 0 || (allowProductSeenNone.value && form.productsSeenNone)),
  )
  const isProductSeenNoneSelected = computed(
    () =>
      showProductSeenField.value &&
      showProductSeenNotesField.value &&
      allowProductSeenNone.value &&
      form.productsSeenNone &&
      form.productsSeen.length === 0,
  )
  const productSeenNotesForPayload = computed(() =>
    canUseProductSeenNotes.value ? trimmedProductSeenNotes.value : '',
  )
  const isProductSeenNotesRequired = computed(
    () =>
      isProductSeenNoneSelected.value &&
      (requireProductSeenNotesField.value || requireProductSeenNotesWhenNone.value),
  )
  const isProductSeenNotesValid = computed(
    () =>
      !isProductSeenNotesRequired.value ||
      trimmedProductSeenNotes.value.length >= productSeenNotesMinChars.value,
  )
  const productSeenNotesHelperText = computed(() => {
    if (isProductSeenNotesRequired.value) {
      return `Obrigatório quando nenhum interesse for identificado. Informe pelo menos ${productSeenNotesMinChars.value} caracteres.`
    }

    return 'Use este campo para detalhar referência, gosto, pedido especial ou algo que ainda não existe em loja.'
  })
  const productSeenDetailMap = computed(() => {
    const note = trimmedProductSeenNotes.value

    if (!canUseProductSeenNotes.value || isProductSeenNoneSelected.value || !note) {
      return {}
    }

    return Object.fromEntries(
      normalizeProducts(form.productsSeen)
        .map((item) => String(item.id || '').trim())
        .filter(Boolean)
        .map((itemId) => [itemId, note]),
    )
  })

  const productsClosedPickerOptions = computed(() =>
    mergeProductEntries(productsClosedSearch.state.options, customProducts.value),
  )
  const productsSeenPickerOptions = computed(() =>
    mergeProductEntries(productsSeenSearch.state.options, customProducts.value),
  )
  const productsClosedEmptyLabel = computed(() =>
    buildProductSearchEmptyLabel(productsClosedSearch.state),
  )
  const productsSeenEmptyLabel = computed(() =>
    buildProductSearchEmptyLabel(productsSeenSearch.state),
  )
  const professionPickerOptions = computed(() =>
    (props.state.professionOptions || []).map((option) => mapOptionToPickerItem(option)),
  )
  const professionSelectedItems = computed({
    get: () =>
      professionPickerOptions.value.filter((option) => option.id === form.customerProfessionId),
    set: (items) => {
      form.customerProfessionId = items[0]?.id || ''
    },
  })
  const visitReasonPickerOptions = computed(() =>
    (props.state.visitReasonOptions || []).map((option) => mapOptionToPickerItem(option)),
  )
  const visitReasonSelectedItems = computed({
    get: () =>
      visitReasonPickerOptions.value.filter((option) =>
        selectedVisitReasonIdSet.value.has(option.id),
      ),
    set: (items) => {
      form.visitReasonIds = normalizeIdList(items.map((item) => item.id))
      form.visitReasonDetails = syncSelectedDetails(form.visitReasonIds, form.visitReasonDetails)
      form.visitReasonNotInformed = false
    },
  })
  const customerSourcePickerOptions = computed(() =>
    (props.state.customerSourceOptions || []).map((option) => mapOptionToPickerItem(option)),
  )
  const customerSourceSelectedItems = computed({
    get: () =>
      customerSourcePickerOptions.value.filter((option) =>
        selectedCustomerSourceIdSet.value.has(option.id),
      ),
    set: (items) => {
      form.customerSourceIds = normalizeIdList(items.map((item) => item.id))
      form.customerSourceDetails = syncSelectedDetails(
        form.customerSourceIds,
        form.customerSourceDetails,
      )
      form.customerSourceNotInformed = false
    },
  })
  const queueJumpReasonPickerOptions = computed(() =>
    (props.state.queueJumpReasonOptions || []).map((option) => mapOptionToPickerItem(option)),
  )
  const lossReasonPickerOptions = computed(() =>
    (props.state.lossReasonOptions || []).map((option) => mapOptionToPickerItem(option)),
  )
  const selectedQueueJumpReasonLabel = computed(
    () =>
      (props.state.queueJumpReasonOptions || []).find(
        (option) => option.id === form.queueJumpReasonId,
      )?.label || '',
  )
  const selectedLossReasonLabels = computed(() =>
    lossReasonPickerOptions.value
      .filter((option) => selectedLossReasonIdSet.value.has(option.id))
      .map((option) => option.label)
      .filter(Boolean),
  )
  const selectedLossReasonSummary = computed(() => selectedLossReasonLabels.value.join(', '))
  const modalTitle = computed(() => resolveModalText(modalConfig.value.title, 'Fechar atendimento'))
  const productSeenLabel = computed(() =>
    resolveModalText(modalConfig.value.productSeenLabel, 'Interesses do cliente'),
  )
  const productSeenPlaceholder = computed(() =>
    resolveModalText(
      modalConfig.value.productSeenPlaceholder,
      'Digite 3 primeiros digitos do codigo/SKU',
    ),
  )
  const customerSectionLabel = computed(() =>
    resolveModalText(modalConfig.value.customerSectionLabel, 'Dados do cliente'),
  )
  const customerNameLabel = computed(() =>
    resolveModalText(modalConfig.value.customerNameLabel, 'Nome do cliente'),
  )
  const customerPhoneLabel = computed(() =>
    resolveModalText(modalConfig.value.customerPhoneLabel, 'Telefone'),
  )
  const customerEmailLabel = computed(() =>
    resolveModalText(modalConfig.value.customerEmailLabel, 'E-mail'),
  )
  const customerProfessionLabel = computed(() =>
    resolveModalText(modalConfig.value.customerProfessionLabel, 'Profissão'),
  )
  const existingCustomerLabel = computed(() =>
    resolveModalText(modalConfig.value.existingCustomerLabel, 'Já era cliente'),
  )
  const visitReasonLabel = computed(() =>
    resolveModalText(modalConfig.value.visitReasonLabel, 'Motivo da visita'),
  )
  const customerSourceLabel = computed(() =>
    resolveModalText(modalConfig.value.customerSourceLabel, 'Origem do cliente'),
  )
  const notesLabel = computed(() => resolveModalText(modalConfig.value.notesLabel, 'Observações'))
  const notesPlaceholder = computed(() =>
    resolveModalText(modalConfig.value.notesPlaceholder, 'Detalhes adicionais do atendimento'),
  )
  const queueJumpReasonLabel = computed(() =>
    resolveModalText(modalConfig.value.queueJumpReasonLabel, 'Motivo do atendimento fora da vez'),
  )
  const queueJumpReasonPlaceholder = computed(() =>
    resolveModalText(
      modalConfig.value.queueJumpReasonPlaceholder,
      'Busque e selecione o motivo fora da vez',
    ),
  )
  const lossReasonLabel = computed(() =>
    resolveModalText(modalConfig.value.lossReasonLabel, 'Motivo da perda'),
  )
  const lossReasonPlaceholder = computed(() =>
    resolveModalText(
      modalConfig.value.lossReasonPlaceholder,
      'Busque e selecione o motivo da perda',
    ),
  )
  const queueJumpReasonSelectedItems = computed({
    get: () =>
      queueJumpReasonPickerOptions.value.filter((option) => option.id === form.queueJumpReasonId),
    set: (items) => {
      form.queueJumpReasonId = items[0]?.id || ''
    },
  })
  const lossReasonSelectedItems = computed({
    get: () =>
      lossReasonPickerOptions.value.filter((option) =>
        selectedLossReasonIdSet.value.has(option.id),
      ),
    set: (items) => {
      form.lossReasonIds = normalizeIdList(items.map((item) => item.id))
      form.lossReasonDetails = syncSelectedDetails(form.lossReasonIds, form.lossReasonDetails)
    },
  })

  function getFieldJustificationValue(key) {
    return String(form.fieldJustifications?.[key] || '')
  }

  function getFieldJustificationCharCount(key) {
    return countCharsIgnoringWhitespace(getFieldJustificationValue(key))
  }

  function isFieldJustificationValid(key, minChars) {
    return getFieldJustificationCharCount(key) >= minChars
  }

  const formStep1Quality = computed(() => {
    const checks = {
      outcome: !!form.outcome,
    }

    if (isPendingValidation.value) {
      checks.validationReason = hasTrimmedText(validationReason.value)
    }

    if (showProductSeenField.value && requireProductSeenField.value) {
      checks.productSeen = form.productsSeen.length > 0 || form.productsSeenNone
    }

    if (isProductSeenNotesRequired.value) {
      checks.productSeenNotes = isProductSeenNotesValid.value
    }

    if (shouldUsePurchaseCodeField.value && requirePurchaseCodeField.value) {
      checks.purchaseCode = trimmedPurchaseCode.value.length > 0
    }

    if (shouldUseLegacyClosedProductField.value && requireProductClosedField.value) {
      checks.productClosed = form.productsClosed.length > 0
    }

    const total = Object.keys(checks).length
    const filled = Object.values(checks).filter(Boolean).length
    const isComplete = filled === total

    return { checks, filled, total, isComplete }
  })

  const formQuality = computed(() => {
    const checks = {}

    if (showCustomerNameField.value && requireCustomerNameField.value) {
      checks.customerName = hasTrimmedText(form.customerName)
    }

    if (showCustomerPhoneField.value && requireCustomerPhoneField.value) {
      checks.customerPhone = hasTrimmedText(form.customerPhone)
    }

    if (showProductSeenField.value && requireProductSeenField.value) {
      checks.productSeen = form.productsSeen.length > 0 || form.productsSeenNone
    }

    if (isProductSeenNotesRequired.value) {
      checks.productSeenNotes = isProductSeenNotesValid.value
    }

    if (shouldUsePurchaseCodeField.value && requirePurchaseCodeField.value) {
      checks.purchaseCode = trimmedPurchaseCode.value.length > 0
    }

    if (shouldUseLegacyClosedProductField.value && requireProductClosedField.value) {
      checks.productClosed = form.productsClosed.length > 0
    }

    if (showVisitReasonField.value && requireVisitReasonField.value) {
      checks.visitReasons = form.visitReasonIds.length > 0 || form.visitReasonNotInformed
    }

    if (showCustomerSourceField.value && requireCustomerSourceField.value) {
      checks.customerSources = form.customerSourceIds.length > 0 || form.customerSourceNotInformed
    }

    if (
      service.value?.startMode === 'queue-jump' &&
      showQueueJumpReasonField.value &&
      requireQueueJumpReasonField.value
    ) {
      checks.queueJumpReason = Boolean(selectedQueueJumpReasonLabel.value)
    }

    if (
      form.outcome === 'nao-compra' &&
      showLossReasonField.value &&
      requireLossReasonField.value
    ) {
      checks.lossReason = form.lossReasonIds.length > 0
    }

    if (showEmailField.value && requireEmailField.value) {
      checks.customerEmail = hasTrimmedText(form.customerEmail)
    }

    if (showProfessionField.value && requireProfessionField.value) {
      checks.customerProfession = !!form.customerProfessionId
    }

    if (showNotesField.value && requireNotesField.value) {
      checks.notes = hasTrimmedText(form.notes)
    }

    const coreTotal = Object.keys(checks).length
    const coreFilledCount = Object.values(checks).filter(Boolean).length
    const hasNotes = hasTrimmedText(form.notes) && showNotesField.value
    const isCoreComplete = coreFilledCount === coreTotal
    const level = isCoreComplete ? (hasNotes ? 'excellent' : 'complete') : 'incomplete'
    const levelLabels = { excellent: 'Excelente', complete: 'Completo', incomplete: 'Incompleto' }

    return {
      checks,
      coreFilledCount,
      coreTotal,
      hasNotes,
      isCoreComplete,
      level,
      levelLabel: levelLabels[level],
    }
  })

  const step1MissingJustifications = computed(() => {
    const items = [
      {
        key: 'purchaseCode',
        label: purchaseCodeLabel.value,
        minChars: resolveModalNumber(modalConfig.value.purchaseCodeJustificationMinChars, 20, 1),
        requiresInput:
          shouldUsePurchaseCodeField.value &&
          !requirePurchaseCodeField.value &&
          resolveModalBoolean(modalConfig.value.requirePurchaseCodeJustification, false) &&
          !hasTrimmedText(form.purchaseCode),
      },
      {
        key: 'productClosed',
        label: closedProductLabel.value,
        minChars: resolveModalNumber(modalConfig.value.productClosedJustificationMinChars, 20, 1),
        requiresInput:
          shouldUseLegacyClosedProductField.value &&
          !requireProductClosedField.value &&
          resolveModalBoolean(modalConfig.value.requireProductClosedJustification, false) &&
          form.productsClosed.length === 0,
      },
      {
        key: 'productSeen',
        label: productSeenLabel.value,
        minChars: resolveModalNumber(modalConfig.value.productSeenJustificationMinChars, 20, 1),
        requiresInput:
          showProductSeenField.value &&
          !requireProductSeenField.value &&
          resolveModalBoolean(modalConfig.value.requireProductSeenJustification, false) &&
          form.productsSeen.length === 0 &&
          !form.productsSeenNone,
      },
      {
        key: 'productSeenNotes',
        label: productSeenNotesLabel.value,
        minChars: resolveModalNumber(
          modalConfig.value.productSeenNotesJustificationMinChars,
          20,
          1,
        ),
        requiresInput:
          canUseProductSeenNotes.value &&
          !isProductSeenNotesRequired.value &&
          resolveModalBoolean(modalConfig.value.requireProductSeenNotesJustification, false) &&
          !hasTrimmedText(form.productSeenNotes),
      },
    ]

    return items.filter((item) => item.requiresInput)
  })

  const step2MissingJustifications = computed(() => {
    const items = [
      {
        key: 'customerName',
        label: customerNameLabel.value,
        minChars: resolveModalNumber(modalConfig.value.customerNameJustificationMinChars, 20, 1),
        // Dado do cliente segue a premissa "preenche OU justifica": vazio na hora
        // de finalizar exige justificativa (escape para quando nao da pra pegar).
        requiresInput: showCustomerNameField.value && !hasTrimmedText(form.customerName),
      },
      {
        key: 'customerPhone',
        label: customerPhoneLabel.value,
        minChars: resolveModalNumber(modalConfig.value.customerPhoneJustificationMinChars, 20, 1),
        requiresInput: showCustomerPhoneField.value && !hasTrimmedText(form.customerPhone),
      },
      {
        key: 'email',
        label: customerEmailLabel.value,
        minChars: resolveModalNumber(modalConfig.value.emailJustificationMinChars, 20, 1),
        requiresInput: showEmailField.value && !hasTrimmedText(form.customerEmail),
      },
      {
        key: 'profession',
        label: customerProfessionLabel.value,
        minChars: resolveModalNumber(modalConfig.value.professionJustificationMinChars, 20, 1),
        requiresInput:
          showProfessionField.value &&
          !requireProfessionField.value &&
          resolveModalBoolean(modalConfig.value.requireProfessionJustification, false) &&
          !form.customerProfessionId,
      },
      {
        key: 'visitReason',
        label: visitReasonLabel.value,
        minChars: resolveModalNumber(modalConfig.value.visitReasonJustificationMinChars, 20, 1),
        requiresInput:
          showVisitReasonField.value &&
          !requireVisitReasonField.value &&
          resolveModalBoolean(modalConfig.value.requireVisitReasonJustification, false) &&
          form.visitReasonIds.length === 0 &&
          !form.visitReasonNotInformed,
      },
      {
        key: 'customerSource',
        label: customerSourceLabel.value,
        minChars: resolveModalNumber(modalConfig.value.customerSourceJustificationMinChars, 20, 1),
        requiresInput:
          showCustomerSourceField.value &&
          !requireCustomerSourceField.value &&
          resolveModalBoolean(modalConfig.value.requireCustomerSourceJustification, false) &&
          form.customerSourceIds.length === 0 &&
          !form.customerSourceNotInformed,
      },
      {
        key: 'queueJumpReason',
        label: queueJumpReasonLabel.value,
        minChars: resolveModalNumber(modalConfig.value.queueJumpReasonJustificationMinChars, 20, 1),
        requiresInput:
          service.value?.startMode === 'queue-jump' &&
          showQueueJumpReasonField.value &&
          !requireQueueJumpReasonField.value &&
          resolveModalBoolean(modalConfig.value.requireQueueJumpReasonJustification, false) &&
          !hasTrimmedText(selectedQueueJumpReasonLabel.value),
      },
      {
        key: 'lossReason',
        label: lossReasonLabel.value,
        minChars: resolveModalNumber(modalConfig.value.lossReasonJustificationMinChars, 20, 1),
        requiresInput:
          form.outcome === 'nao-compra' &&
          showLossReasonField.value &&
          !requireLossReasonField.value &&
          resolveModalBoolean(modalConfig.value.requireLossReasonJustification, false) &&
          form.lossReasonIds.length === 0,
      },
      {
        key: 'notes',
        label: notesLabel.value,
        minChars: resolveModalNumber(modalConfig.value.notesJustificationMinChars, 20, 1),
        requiresInput:
          showNotesField.value &&
          !requireNotesField.value &&
          resolveModalBoolean(modalConfig.value.requireNotesJustification, false) &&
          !hasTrimmedText(form.notes),
      },
    ]

    return items.filter((item) => item.requiresInput)
  })

  const hasInvalidStep1Justifications = computed(() =>
    step1MissingJustifications.value.some(
      (item) => !isFieldJustificationValid(item.key, item.minChars),
    ),
  )
  const hasInvalidStep2Justifications = computed(() =>
    step2MissingJustifications.value.some(
      (item) => !isFieldJustificationValid(item.key, item.minChars),
    ),
  )
  const isStep1Ready = computed(
    () => formStep1Quality.value.isComplete && !hasInvalidStep1Justifications.value,
  )
  const step2QualityTone = computed(() =>
    hasInvalidStep2Justifications.value ? 'incomplete' : formQuality.value.level,
  )
  const step2QualityLabel = computed(() =>
    hasInvalidStep2Justifications.value ? 'Justificativas pendentes' : formQuality.value.levelLabel,
  )

  async function validateFieldJustifications(items = []) {
    const firstInvalid = items.find((item) => !isFieldJustificationValid(item.key, item.minChars))

    if (!firstInvalid) {
      return true
    }

    await ui.alert(
      `Preencha a justificativa de "${firstInvalid.label}" com pelo menos ${firstInvalid.minChars} caracteres sem contar espaços.`,
    )
    return false
  }

  function updateProfessionSelectedItems(items) {
    professionSelectedItems.value = items
  }

  function updateVisitReasonSelectedItems(items) {
    visitReasonSelectedItems.value = items
  }

  function updateCustomerSourceSelectedItems(items) {
    customerSourceSelectedItems.value = items
  }

  function updateQueueJumpReasonSelectedItems(items) {
    queueJumpReasonSelectedItems.value = items
  }

  function updateLossReasonSelectedItems(items) {
    lossReasonSelectedItems.value = items
  }

  function buildDraftPayload() {
    return {
      outcome: form.outcome,
      isExistingCustomer: form.isExistingCustomer,
      purchaseCode: String(form.purchaseCode || '').trim(),
      productsSeen: normalizeProducts(form.productsSeen),
      productsClosed: normalizeProducts(form.productsClosed),
      productsSeenNone: form.productsSeenNone,
      productSeenNotes: productSeenNotesForPayload.value,
      customerName: form.customerName,
      customerPhone: form.customerPhone,
      customerEmail: form.customerEmail,
      customerProfessionId: form.customerProfessionId,
      customerProfession: selectedProfessionLabel.value,
      visitReasonIds: normalizeIdList(form.visitReasonIds),
      visitReasons: normalizeIdList(form.visitReasonIds),
      visitReasonsNotInformed: form.visitReasonNotInformed,
      visitReasonDetails: { ...form.visitReasonDetails },
      customerSourceIds: normalizeIdList(form.customerSourceIds),
      customerSources: normalizeIdList(form.customerSourceIds),
      customerSourcesNotInformed: form.customerSourceNotInformed,
      customerSourceDetails: { ...form.customerSourceDetails },
      queueJumpReasonId: form.queueJumpReasonId,
      queueJumpReason: selectedQueueJumpReasonLabel.value,
      lossReasonIds: normalizeIdList(form.lossReasonIds),
      lossReasons: normalizeIdList(form.lossReasonIds),
      lossReasonDetails: { ...form.lossReasonDetails },
      lossReasonId: normalizeIdList(form.lossReasonIds)[0] || '',
      lossReason: selectedLossReasonSummary.value,
      notes: form.notes,
      fieldJustifications: normalizeFieldJustifications(form.fieldJustifications),
    }
  }

  function hasDraftContent(payload, products = []) {
    if (!payload || typeof payload !== 'object') {
      return false
    }

    return Boolean(
      payload.outcome ||
      payload.isExistingCustomer ||
      payload.purchaseCode ||
      payload.productsSeen?.length ||
      payload.productsClosed?.length ||
      payload.productsSeenNone ||
      payload.productSeenNotes ||
      payload.customerName ||
      payload.customerPhone ||
      payload.customerEmail ||
      payload.customerProfessionId ||
      payload.visitReasonIds?.length ||
      payload.visitReasonsNotInformed ||
      Object.keys(payload.visitReasonDetails || {}).length ||
      payload.customerSourceIds?.length ||
      payload.customerSourcesNotInformed ||
      Object.keys(payload.customerSourceDetails || {}).length ||
      payload.queueJumpReasonId ||
      payload.lossReasonIds?.length ||
      Object.keys(payload.lossReasonDetails || {}).length ||
      Object.values(payload.fieldJustifications || {}).some((value) => hasTrimmedText(value)) ||
      payload.notes ||
      products.length,
    )
  }

  function loadStoredDraft(currentService) {
    const key = serviceDraftKey.value
    const currentStoreId = String(props.state.activeStoreId || '').trim()
    const currentStartedAt = Number(currentService?.serviceStartedAt || 0)

    if (!key || !currentService) {
      return null
    }

    const stored = readDraftStorage()[key]

    if (!stored || typeof stored !== 'object') {
      return null
    }

    if (
      stored.storeId !== currentStoreId ||
      stored.serviceId !== currentService.serviceId ||
      stored.personId !== currentService.id ||
      Number(stored.serviceStartedAt || 0) !== currentStartedAt
    ) {
      removeStoredDraft(key)
      return null
    }

    return stored
  }

  function saveActiveDraft() {
    if (isApplyingDraft || !service.value || !serviceDraftKey.value) {
      return
    }

    const payload = buildDraftPayload()
    const normalizedCustomProducts = normalizeProducts(customProducts.value).filter(
      (product) => product.isCustom,
    )

    if (!hasDraftContent(payload, normalizedCustomProducts)) {
      removeStoredDraft(serviceDraftKey.value)
      restoredDraftKey.value = ''
      return
    }

    const drafts = readDraftStorage()
    drafts[serviceDraftKey.value] = {
      version: 1,
      storeId: String(props.state.activeStoreId || '').trim(),
      serviceId: service.value.serviceId,
      personId: service.value.id,
      serviceStartedAt: Number(service.value.serviceStartedAt || 0),
      updatedAt: Date.now(),
      form: payload,
      customProducts: normalizedCustomProducts,
    }
    writeDraftStorage(drafts)
  }

  function registerCustomProducts(items = []) {
    const nextCustomProducts = normalizeProducts(items).filter((product) => product.isCustom)

    if (!nextCustomProducts.length) {
      return
    }

    customProducts.value = mergeProductEntries(customProducts.value, nextCustomProducts)
  }

  function updateProductsSeen(items) {
    const nextItems = normalizeProducts(items)
    const wasNoneSelected = form.productsSeenNone
    registerCustomProducts(nextItems)
    form.productsSeen = nextItems

    if (nextItems.length > 0) {
      form.productsSeenNone = false
      if (wasNoneSelected) {
        form.productSeenNotes = ''
      }
      return
    }

    form.productSeenNotes = ''
  }

  function updateProductsSeenNone(nextValue) {
    const normalizedValue = Boolean(nextValue)

    if (form.productsSeenNone === normalizedValue) {
      return
    }

    form.productsSeenNone = normalizedValue

    if (normalizedValue) {
      form.productsSeen = []
      form.productSeenNotes = ''
      return
    }

    form.productSeenNotes = ''
  }

  function updateProductSeenDetails(details = {}) {
    const normalizedDetails = details && typeof details === 'object' ? details : {}

    if (isProductSeenNoneSelected.value) {
      form.productSeenNotes = String(normalizedDetails[PRODUCT_SEEN_NONE_DETAIL_KEY] || '').trim()
      return
    }

    const selectedProductIds = normalizeProducts(form.productsSeen)
      .map((item) => String(item.id || '').trim())
      .filter(Boolean)

    form.productSeenNotes =
      selectedProductIds
        .map((itemId) => String(normalizedDetails[itemId] || '').trim())
        .find(Boolean) || ''
  }

  function updateProductsClosed(items) {
    const nextItems = normalizeProducts(items)
    registerCustomProducts(nextItems)
    form.productsClosed = nextItems
  }

  function clearCurrentDraft() {
    const key = serviceDraftKey.value

    if (key) {
      removeStoredDraft(key)
    }

    isApplyingDraft = true
    restoredDraftKey.value = ''
    customProducts.value = []
    productsClosedSearch.clear()
    productsSeenSearch.clear()
    step.value = 1
    step1JustificationsRevealed.value = false
    step2JustificationsRevealed.value = false
    Object.assign(form, createEmptyForm())
    normalizeFormForModalConfig()
    isApplyingDraft = false
  }

  function normalizeFormForModalConfig() {
    form.visitReasonIds = normalizeIdList(form.visitReasonIds)
    form.lossReasonIds = normalizeIdList(form.lossReasonIds)
    form.customerSourceIds = normalizeIdList(form.customerSourceIds)
    form.visitReasonDetails = syncSelectedDetails(form.visitReasonIds, form.visitReasonDetails)
    form.lossReasonDetails = syncSelectedDetails(form.lossReasonIds, form.lossReasonDetails)
    form.customerSourceDetails = syncSelectedDetails(
      form.customerSourceIds,
      form.customerSourceDetails,
    )

    if (!shouldUsePurchaseCodeField.value) {
      form.purchaseCode = ''
    }

    if (!shouldUseLegacyClosedProductField.value) {
      form.productsClosed = []
    }

    if (!allowProductSeenNone.value) {
      form.productsSeenNone = false
    }

    if (form.productsSeen.length) {
      form.productsSeenNone = false
    }

    if (!canUseProductSeenNotes.value) {
      form.productSeenNotes = ''
    }

    if (form.visitReasonIds.length) {
      form.visitReasonNotInformed = false
    }

    if (form.customerSourceIds.length) {
      form.customerSourceNotInformed = false
    }
  }

  function resetForm() {
    const currentService = service.value
    const storedDraft = loadStoredDraft(currentService)
    const initialDraft = storedDraft?.form || draft.value

    isApplyingDraft = true
    step.value = 1
    step1JustificationsRevealed.value = false
    step2JustificationsRevealed.value = false
    productsClosedSearch.clear()
    productsSeenSearch.clear()
    customProducts.value = mergeProductEntries(
      storedDraft?.customProducts || [],
      initialDraft?.customProducts || [],
    )
    restoredDraftKey.value = storedDraft ? serviceDraftKey.value : ''
    Object.assign(form, createEmptyForm(), buildInitialForm(props.state, initialDraft))
    normalizeFormForModalConfig()
    isApplyingDraft = false
  }

  function goToStep1() {
    step.value = 1
  }

  async function goToStep2() {
    // Tentativa de avancar revela as justificativas pendentes do passo 1.
    step1JustificationsRevealed.value = true

    if (isPendingValidation.value && !validationReason.value.trim()) {
      await ui.alert('Informe por que este atendimento nao foi encerrado pelo consultor.')
      globalThis.document?.getElementById('operation-validation-reason')?.focus()
      return
    }

    if (!form.outcome) {
      await ui.alert('Selecione como o atendimento terminou.')
      return
    }

    if (
      showProductSeenField.value &&
      requireProductSeenField.value &&
      form.productsSeen.length === 0 &&
      !form.productsSeenNone
    ) {
      await ui.alert('Selecione pelo menos um interesse do cliente ou use a opcao de nenhum.')
      return
    }

    if (isProductSeenNotesRequired.value && !isProductSeenNotesValid.value) {
      await ui.alert(
        `Preencha os detalhes dos interesses com pelo menos ${productSeenNotesMinChars.value} caracteres.`,
      )
      return
    }

    if (
      shouldUsePurchaseCodeField.value &&
      requirePurchaseCodeField.value &&
      !trimmedPurchaseCode.value
    ) {
      await ui.alert('Informe o codigo da compra para seguir.')
      return
    }

    if (
      shouldUseLegacyClosedProductField.value &&
      requireProductClosedField.value &&
      form.productsClosed.length === 0
    ) {
      await ui.alert('Selecione o item de compra ou reserva.')
      return
    }

    if (!(await validateFieldJustifications(step1MissingJustifications.value))) {
      return
    }

    step.value = 2
  }

  function closeModal() {
    void operationsStore.closeFinishModal()
  }

  async function submitForm() {
    if (step.value !== 2) {
      await goToStep2()
      return
    }

    // Tentativa de finalizar revela as justificativas pendentes dos dois passos,
    // mesmo que algum campo estritamente obrigatorio interrompa antes.
    step1JustificationsRevealed.value = true
    step2JustificationsRevealed.value = true

    if (!service.value?.id || !form.outcome) {
      await ui.alert('Selecione como o atendimento terminou.')
      return
    }

    if (
      showVisitReasonField.value &&
      requireVisitReasonField.value &&
      form.visitReasonIds.length === 0 &&
      !form.visitReasonNotInformed
    ) {
      await ui.alert("Selecione um motivo da visita ou marque 'Nao informado'.")
      return
    }

    if (
      showProductSeenField.value &&
      requireProductSeenField.value &&
      form.productsSeen.length === 0 &&
      !form.productsSeenNone
    ) {
      await ui.alert('Selecione pelo menos um interesse do cliente ou use a opcao de nenhum.')
      return
    }

    if (isProductSeenNotesRequired.value && !isProductSeenNotesValid.value) {
      await ui.alert(
        `Preencha os detalhes dos interesses com pelo menos ${productSeenNotesMinChars.value} caracteres.`,
      )
      return
    }

    if (
      shouldUsePurchaseCodeField.value &&
      requirePurchaseCodeField.value &&
      !trimmedPurchaseCode.value
    ) {
      await ui.alert('Informe o codigo da compra para concluir o atendimento.')
      return
    }

    if (
      shouldUseLegacyClosedProductField.value &&
      requireProductClosedField.value &&
      form.productsClosed.length === 0
    ) {
      await ui.alert('Selecione o item de compra ou reserva.')
      return
    }

    // Nome/telefone/e-mail nao travam mais com "obrigatorio": quando vazios, a
    // validacao de justificativas abaixo exige o motivo (preenche OU justifica).
    if (showProfessionField.value && requireProfessionField.value && !form.customerProfessionId) {
      await ui.alert('Selecione a profissao do cliente.')
      return
    }

    if (
      showCustomerSourceField.value &&
      requireCustomerSourceField.value &&
      form.customerSourceIds.length === 0 &&
      !form.customerSourceNotInformed
    ) {
      await ui.alert("Selecione uma origem do cliente ou marque 'Nao informado'.")
      return
    }

    if (showNotesField.value && requireNotesField.value && !form.notes.trim()) {
      await ui.alert('Observações são obrigatórias para concluir o atendimento.')
      return
    }

    if (
      service.value.startMode === 'queue-jump' &&
      showQueueJumpReasonField.value &&
      requireQueueJumpReasonField.value &&
      !selectedQueueJumpReasonLabel.value
    ) {
      if (!queueJumpReasonPickerOptions.value.length) {
        await ui.alert('Cadastre pelo menos um motivo de atendimento fora da vez em Configuracoes.')
        return
      }

      await ui.alert('Selecione o motivo do atendimento fora da vez.')
      return
    }

    if (
      form.outcome === 'nao-compra' &&
      showLossReasonField.value &&
      requireLossReasonField.value &&
      form.lossReasonIds.length === 0
    ) {
      if (!lossReasonPickerOptions.value.length) {
        await ui.alert('Cadastre pelo menos um motivo da perda em Configuracoes.')
        return
      }

      await ui.alert('Selecione o motivo da perda.')
      return
    }

    if (
      !(await validateFieldJustifications([
        ...step1MissingJustifications.value,
        ...step2MissingJustifications.value,
      ]))
    ) {
      return
    }

    // Encerramento de pendencia (auto-encerramento 2h): a justificativa de por que
    // o consultor nao encerrou na hora e OBRIGATORIA (base das metricas de cobranca).
    if (isPendingValidation.value && !validationReason.value.trim()) {
      await ui.alert('Informe por que este atendimento nao foi encerrado pelo consultor.')
      return
    }

    const currentService = service.value
    const closedProductsForPayload = shouldUseLegacyClosedProductField.value
      ? form.productsClosed
      : []
    const productSeenSummary = [
      form.productsSeen.length
        ? form.productsSeen
            .map((item) => item.name)
            .filter(Boolean)
            .join(', ')
        : '',
      form.productsSeenNone ? 'Nenhum interesse identificado' : '',
      productSeenNotesForPayload.value,
    ]
      .filter(Boolean)
      .join(' | ')
    const result = await operationsStore.finishService(
      currentService.serviceId,
      {
        outcome: form.outcome,
        productSeen: productSeenSummary,
        productClosed: closedProductsForPayload[0]?.name || '',
        purchaseCode: shouldUsePurchaseCodeField.value ? trimmedPurchaseCode.value : '',
        productsSeen: form.productsSeen,
        productsClosed: closedProductsForPayload,
        productsSeenNone: form.productsSeenNone,
        productSeenNotes: productSeenNotesForPayload.value,
        productDetails: closedProductsForPayload[0]?.name || '' || productSeenSummary || '',
        customerName: form.customerName.trim(),
        customerPhone: form.customerPhone.trim(),
        customerEmail: form.customerEmail.trim(),
        customerProfession: selectedProfessionLabel.value,
        isExistingCustomer: form.isExistingCustomer,
        visitReasons: normalizeIdList(form.visitReasonIds),
        visitReasonsNotInformed: form.visitReasonNotInformed,
        visitReasonDetails: visitReasonDetailsEnabled.value
          ? Object.fromEntries(
              normalizeIdList(form.visitReasonIds)
                .map((reasonId) => [
                  reasonId,
                  String(form.visitReasonDetails?.[reasonId] || '').trim(),
                ])
                .filter(([, detail]) => detail),
            )
          : {},
        customerSources: normalizeIdList(form.customerSourceIds),
        customerSourcesNotInformed: form.customerSourceNotInformed,
        customerSourceDetails: customerSourceDetailsEnabled.value
          ? Object.fromEntries(
              normalizeIdList(form.customerSourceIds)
                .map((sourceId) => [
                  sourceId,
                  String(form.customerSourceDetails?.[sourceId] || '').trim(),
                ])
                .filter(([, detail]) => detail),
            )
          : {},
        lossReasons: form.outcome === 'nao-compra' ? normalizeIdList(form.lossReasonIds) : [],
        lossReasonDetails:
          lossReasonDetailsEnabled.value && form.outcome === 'nao-compra'
            ? Object.fromEntries(
                normalizeIdList(form.lossReasonIds)
                  .map((reasonId) => [
                    reasonId,
                    String(form.lossReasonDetails?.[reasonId] || '').trim(),
                  ])
                  .filter(([, detail]) => detail),
              )
            : {},
        lossReasonId:
          form.outcome === 'nao-compra' ? normalizeIdList(form.lossReasonIds)[0] || '' : '',
        lossReason: form.outcome === 'nao-compra' ? selectedLossReasonSummary.value : '',
        saleAmount: closedProductsForPayload.length > 0 ? closedTotal.value : 0,
        queueJumpReason:
          service.value.startMode === 'queue-jump' ? selectedQueueJumpReasonLabel.value : '',
        notes: form.notes.trim(),
      },
      {
        service: currentService,
        storeId: currentService.storeId,
        storeName: currentService.storeName,
        // Pendencia: o submit vai para POST /validate (UPDATE da linha pendente no
        // historico) com a justificativa obrigatoria, em vez do /finish normal.
        validate: isPendingValidation.value,
        validationReason: validationReason.value.trim(),
      },
    )

    if (result?.ok === false) {
      ui.error(result.message || 'Nao foi possivel encerrar o atendimento.')
      return
    }

    removeStoredDraft(
      `${String(props.state.activeStoreId || '').trim()}:${currentService.serviceId}`,
    )
    restoredDraftKey.value = ''
    customProducts.value = []
    ui.success('Atendimento encerrado.')
  }

  function handleEscape(event) {
    if (event.key !== 'Escape') return
    if (!service.value) return
    if (document.querySelector('.product-pick__dropdown.is-open')) return
    if (document.querySelector('.product-pick__detail-popover')) return
    closeModal()
  }

  watch(
    serviceDraftKey,
    () => {
      resetForm()
    },
    { immediate: true },
  )

  watch(productCatalogStoreId, () => {
    productsClosedSearch.clear()
    productsSeenSearch.clear()
  })

  watch([requestedServiceDraftKey, service], ([draftKey, currentService]) => {
    if (!draftKey || currentService) {
      return
    }

    removeStoredDraft(draftKey)
    restoredDraftKey.value = ''
    customProducts.value = []
    void operationsStore.closeFinishModal()
  })

  watch(draft, () => {
    if (!hasRestoredDraft.value) {
      resetForm()
    }
  })

  watch(
    () => [...form.visitReasonIds],
    (nextValue) => {
      if (nextValue.length) {
        form.visitReasonNotInformed = false
      }

      form.visitReasonDetails = syncSelectedDetails(nextValue, form.visitReasonDetails)
    },
    { deep: true },
  )

  watch(
    () => [...form.customerSourceIds],
    (nextValue) => {
      if (nextValue.length) {
        form.customerSourceNotInformed = false
      }

      form.customerSourceDetails = syncSelectedDetails(nextValue, form.customerSourceDetails)
    },
    { deep: true },
  )

  watch(
    () => [...form.lossReasonIds],
    (nextValue) => {
      form.lossReasonDetails = syncSelectedDetails(nextValue, form.lossReasonDetails)
    },
    { deep: true },
  )

  watch(
    () => form.visitReasonNotInformed,
    (nextValue) => {
      if (!nextValue) {
        return
      }

      form.visitReasonIds = []
      form.visitReasonDetails = {}
    },
  )

  watch(
    () => form.customerSourceNotInformed,
    (nextValue) => {
      if (!nextValue) {
        return
      }

      form.customerSourceIds = []
      form.customerSourceDetails = {}
    },
  )

  watch([isVisitReasonMultiple, visitReasonDetailsEnabled], () => {
    normalizeFormForModalConfig()
  })

  watch([isLossReasonMultiple, lossReasonDetailsEnabled], () => {
    normalizeFormForModalConfig()
  })

  watch([isCustomerSourceMultiple, customerSourceDetailsEnabled], () => {
    normalizeFormForModalConfig()
  })

  watch([allowProductSeenNone, showProductSeenNotesField], () => {
    normalizeFormForModalConfig()
  })

  watch([isERPReconciliationFlow, showPurchaseCodeField, showProductClosedField], () => {
    normalizeFormForModalConfig()
  })

  watch(
    () => form.outcome,
    (nextValue) => {
      step1JustificationsRevealed.value = false
      step2JustificationsRevealed.value = false

      if (nextValue !== 'nao-compra') {
        form.lossReasonIds = []
        form.lossReasonDetails = {}
      }

      normalizeFormForModalConfig()
    },
  )

  watch(
    form,
    () => {
      saveActiveDraft()
    },
    { deep: true },
  )

  watch(
    customProducts,
    () => {
      saveActiveDraft()
    },
    { deep: true },
  )

  watch(
    () => props.state.finishModalServiceId,
    (nextValue) => {
      validationReason.value = ''

      if (String(nextValue || '').trim()) {
        return
      }

      productsClosedSearch.clear()
      productsSeenSearch.clear()
    },
  )

  onMounted(() => {
    document.addEventListener('keydown', handleEscape)
  })

  onBeforeUnmount(() => {
    document.removeEventListener('keydown', handleEscape)
  })

  return {
    PRODUCT_SEARCH_MIN_CHARS,
    modalConfig,
    service,
    isPendingValidation,
    validationReason,
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
  }
}
