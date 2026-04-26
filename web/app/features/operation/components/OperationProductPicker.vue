<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useUiStore } from "~/stores/ui";

const props = defineProps({
  label: {
    type: String,
    required: true
  },
  helperText: {
    type: String,
    default: ""
  },
  options: {
    type: Array,
    default: () => []
  },
  selectedItems: {
    type: Array,
    default: () => []
  },
  mode: {
    type: String,
    default: "default"
  },
  multiple: {
    type: Boolean,
    default: true
  },
  allowNone: {
    type: Boolean,
    default: false
  },
  allowNoneDetails: {
    type: Boolean,
    default: false
  },
  noneSelected: {
    type: Boolean,
    default: false
  },
  noneLabel: {
    type: String,
    default: "Nenhum"
  },
  noneStateLabel: {
    type: String,
    default: ""
  },
  nonePlacement: {
    type: String,
    default: "inline"
  },
  searchPlaceholder: {
    type: String,
    default: "Busque e selecione"
  },
  triggerLabel: {
    type: String,
    default: "Selecionar"
  },
  emptySelectedLabel: {
    type: String,
    default: "Nenhum item selecionado"
  },
  emptySearchLabel: {
    type: String,
    default: "Nenhum item encontrado para a busca atual."
  },
  allowCustom: {
    type: Boolean,
    default: false
  },
  customOptionLabel: {
    type: String,
    default: "Item nao cadastrado"
  },
  customCodePlaceholder: {
    type: String,
    default: "Codigo do produto *"
  },
  customNamePlaceholder: {
    type: String,
    default: "Nome do item *"
  },
  customPricePlaceholder: {
    type: String,
    default: "Valor R$ *"
  },
  enableItemDetails: {
    type: Boolean,
    default: false
  },
  itemDetails: {
    type: Object,
    default: () => ({})
  },
  itemDetailMode: {
    type: String,
    default: "shared"
  },
  itemDetailLabel: {
    type: String,
    default: "Detalhe"
  },
  itemDetailPlaceholder: {
    type: String,
    default: "Digite um detalhe"
  },
  itemDetailTestid: {
    type: String,
    default: ""
  },
  testidPrefix: {
    type: String,
    default: ""
  }
});

const emit = defineEmits(["update:selectedItems", "update:noneSelected", "update:itemDetails"]);
const ui = useUiStore();

const searchInputRef = ref(null);
const containerRef = ref(null);
const dropdownStyle = ref({});
const dropdownOpen = ref(false);
const customOpen = ref(false);
const searchTerm = ref("");
const customCode = ref("");
const itemDetailEditorId = ref("");
const itemDetailDraft = ref("");
const SHARED_DETAIL_KEY = "__shared__";
const NONE_DETAIL_KEY = "__none__";

const isClosedMode = computed(() => props.mode === "closed");
const isSharedDetailMode = computed(() => props.itemDetailMode !== "per-item");
const useInlineNoneAction = computed(() => props.nonePlacement !== "dropdown");
const normalizedOptions = computed(() => (Array.isArray(props.options) ? props.options : []).map(normalizeOption));
const normalizedSelectedItems = computed(() =>
  (Array.isArray(props.selectedItems) ? props.selectedItems : []).map(normalizeOption)
);
const normalizedItemDetails = computed(() =>
  props.itemDetails && typeof props.itemDetails === "object" ? props.itemDetails : {}
);
const selectedCount = computed(() => normalizedSelectedItems.value.length);
const hasVisualSelection = computed(() => selectedCount.value > 0 || props.noneSelected);
const canEditSharedDetail = computed(() =>
  props.enableItemDetails
  && isSharedDetailMode.value
  && (
    (selectedCount.value > 0 && !props.noneSelected)
    || (props.allowNoneDetails && props.noneSelected && selectedCount.value === 0)
  )
);
const total = computed(() =>
  normalizedSelectedItems.value.reduce((sum, item) => sum + (Number(item.price) || 0), 0)
);
const activeDetailItem = computed(() =>
  isSharedDetailMode.value
    ? null
    : normalizedSelectedItems.value.find((item) => item.id === itemDetailEditorId.value) || null
);
const sharedDetailValue = computed(() => {
  if (props.noneSelected && selectedCount.value === 0) {
    return String(normalizedItemDetails.value?.[NONE_DETAIL_KEY] || "").trim();
  }

  const detailValues = normalizedSelectedItems.value
    .map((item) => String(normalizedItemDetails.value?.[item.id] || "").trim())
    .filter(Boolean);

  if (!detailValues.length) {
    return "";
  }

  return new Set(detailValues).size === 1 ? detailValues[0] : "";
});
const hasSharedDetail = computed(() => sharedDetailValue.value.length > 0);
const isSharedDetailEditorOpen = computed(() => itemDetailEditorId.value === SHARED_DETAIL_KEY);
const activeDetailTitle = computed(() => {
  if (isSharedDetailMode.value) {
    if (props.noneSelected && selectedCount.value === 0) {
      return props.noneStateLabel || props.noneLabel || props.itemDetailLabel;
    }

    if (selectedCount.value === 1) {
      return normalizedSelectedItems.value[0]?.label || props.itemDetailLabel;
    }

    return `${selectedCount.value} itens selecionados`;
  }

  return activeDetailItem.value?.label || props.itemDetailLabel;
});
const activeDetailCaption = computed(() => {
  if (isSharedDetailMode.value) {
    return hasSharedDetail.value ? "Texto compartilhado salvo" : "Texto compartilhado";
  }

  return activeDetailItem.value && hasItemDetail(activeDetailItem.value.id)
    ? `${props.itemDetailLabel} salvo`
    : props.itemDetailLabel;
});
const filteredOptions = computed(() => {
  const normalizedSearch = normalizeSearch(searchTerm.value);
  const selectedIds = new Set(normalizedSelectedItems.value.map((item) => item.id));

  return normalizedOptions.value.filter((item) => {
    if (selectedIds.has(item.id)) {
      return false;
    }

    if (!normalizedSearch) {
      return true;
    }

    return normalizeSearch([item.label, item.meta, item.code, item.searchText].filter(Boolean).join(" "))
      .includes(normalizedSearch);
  });
});

function formatCurrency(value) {
  return Number(value || 0).toLocaleString("pt-BR", { style: "currency", currency: "BRL" });
}

function normalizeSearch(value) {
  return String(value || "")
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .trim()
    .toLowerCase();
}

function normalizeOption(item) {
  const price = Math.max(0, Number(item?.price ?? item?.basePrice ?? 0) || 0);
  const label = String(item?.label ?? item?.name ?? "").trim();
  const code = String(item?.code || "").trim();
  const metaParts = [];

  if (String(item?.meta || "").trim()) {
    metaParts.push(String(item.meta).trim());
  } else {
    if (String(item?.description || "").trim()) {
      metaParts.push(String(item.description).trim());
    }

    if (String(item?.category || "").trim()) {
      metaParts.push(String(item.category).trim());
    }

    if (code) {
      metaParts.push(code);
    }
  }

  if (isClosedMode.value && price > 0) {
    metaParts.push(formatCurrency(price));
  }

  return {
    ...item,
    id: String(item?.id || label || `item-${Math.random().toString(36).slice(2, 8)}`),
    label,
    name: String(item?.name ?? label),
    meta: metaParts.filter(Boolean).join(" | "),
    code,
    price,
    searchText: String(item?.searchText || metaParts.join(" ")).trim(),
    isCustom: Boolean(item?.isCustom)
  };
}

function emitSelectedItems(nextItems) {
  emit("update:selectedItems", nextItems);
}

function emitItemDetails(nextDetails) {
  emit("update:itemDetails", nextDetails);
}

function buildFilteredItemDetails(selectedItems = [], details = normalizedItemDetails.value) {
  if (props.noneSelected && (!Array.isArray(selectedItems) || selectedItems.length === 0)) {
    const noneDetail = String(details?.[NONE_DETAIL_KEY] || "").trim();
    return noneDetail ? { [NONE_DETAIL_KEY]: noneDetail } : {};
  }

  return Object.fromEntries(
    (Array.isArray(selectedItems) ? selectedItems : [])
      .map((item) => String(item?.id || "").trim())
      .filter(Boolean)
      .map((itemId) => [itemId, String(details?.[itemId] || "").trim()])
      .filter(([, value]) => value)
  );
}

function syncItemDetails(selectedItems = [], details = normalizedItemDetails.value) {
  if (!props.enableItemDetails) {
    return;
  }

  emitItemDetails(buildFilteredItemDetails(selectedItems, details));
}

function clearCustomForm() {
  customCode.value = "";
}

function closeItemDetailEditor() {
  itemDetailEditorId.value = "";
  itemDetailDraft.value = "";
}

function closeDropdown() {
  dropdownOpen.value = false;
  customOpen.value = false;
  searchTerm.value = "";
  clearCustomForm();
}

function updateDropdownPosition() {
  if (!containerRef.value) return;
  const rect = containerRef.value.getBoundingClientRect();
  dropdownStyle.value = {
    top: `${rect.bottom + 4}px`,
    left: `${rect.left}px`,
    width: `${rect.width}px`
  };
}

function openDropdown() {
  closeItemDetailEditor();
  dropdownOpen.value = true;
  updateDropdownPosition();
  nextTick(() => {
    searchInputRef.value?.focus();
  });
}

function toggleDropdown() {
  if (dropdownOpen.value) {
    closeDropdown();
    return;
  }

  openDropdown();
}

function toggleNone() {
  if (!props.allowNone) {
    return;
  }

  if (props.noneSelected) {
    emit("update:noneSelected", false);
    emitItemDetails({});
    closeItemDetailEditor();
    return;
  }

  emitSelectedItems([]);
  emit("update:noneSelected", true);
  emitItemDetails({});
  closeItemDetailEditor();
  closeDropdown();
}

function selectOption(item) {
  const nextItems = props.multiple ? [...normalizedSelectedItems.value, item] : [item];
  emitSelectedItems(nextItems);
  emit("update:noneSelected", false);
  syncItemDetails(nextItems);
  closeDropdown();
}

function addCustomItem() {
  const code = customCode.value.trim().toUpperCase();

  if (!code) {
    void ui.alert("Informe o codigo do item.");
    return;
  }

  const nextItem = normalizeOption({
    id: `__custom__${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
    label: code,
    name: code,
    code,
    price: 0,
    isCustom: true
  });

  const nextItems = props.multiple ? [...normalizedSelectedItems.value, nextItem] : [nextItem];
  emitSelectedItems(nextItems);
  emit("update:noneSelected", false);
  syncItemDetails(nextItems);
  closeDropdown();
}

function removeSelectedItem(itemId) {
  const nextItems = normalizedSelectedItems.value.filter((item) => item.id !== itemId);
  emitSelectedItems(nextItems);
  syncItemDetails(nextItems);

  if (itemDetailEditorId.value === itemId) {
    closeItemDetailEditor();
  }
}

function toggleCustomForm() {
  if (!props.allowCustom) {
    return;
  }

  if (!dropdownOpen.value) {
    openDropdown();
  }

  customOpen.value = !customOpen.value;

  if (!customOpen.value) {
    clearCustomForm();
    nextTick(() => {
      searchInputRef.value?.focus();
    });
  }
}

function hasItemDetail(itemId) {
  return String(normalizedItemDetails.value?.[itemId] || "").trim().length > 0;
}

function isItemDetailEditorOpen(itemId) {
  return !isSharedDetailMode.value && itemDetailEditorId.value === itemId;
}

function openSharedDetailEditor() {
  if (!props.enableItemDetails) {
    return;
  }

  closeDropdown();
  itemDetailEditorId.value = SHARED_DETAIL_KEY;
  itemDetailDraft.value = sharedDetailValue.value;
}

function openItemDetailEditor(itemId) {
  if (!props.enableItemDetails) {
    return;
  }

  closeDropdown();
  itemDetailEditorId.value = String(itemId || "");
  itemDetailDraft.value = String(normalizedItemDetails.value?.[itemId] || "");
}

function saveItemDetail() {
  const nextDetails = {
    ...normalizedItemDetails.value
  };
  const trimmedDetail = itemDetailDraft.value.trim();

  if (isSharedDetailMode.value) {
    if (props.noneSelected && selectedCount.value === 0) {
      if (trimmedDetail) {
        nextDetails[NONE_DETAIL_KEY] = trimmedDetail;
      } else {
        delete nextDetails[NONE_DETAIL_KEY];
      }
    } else {
      normalizedSelectedItems.value.forEach((item) => {
        if (trimmedDetail) {
          nextDetails[item.id] = trimmedDetail;
        } else {
          delete nextDetails[item.id];
        }
      });
    }
  } else {
    const itemId = String(itemDetailEditorId.value || "").trim();

    if (!itemId) {
      return;
    }

    if (trimmedDetail) {
      nextDetails[itemId] = trimmedDetail;
    } else {
      delete nextDetails[itemId];
    }
  }

  emitItemDetails(buildFilteredItemDetails(normalizedSelectedItems.value, nextDetails));
  closeItemDetailEditor();
}

function handleEscape(event) {
  if (event.key === "Escape") {
    if (itemDetailEditorId.value) {
      closeItemDetailEditor();
      return;
    }

    closeDropdown();
  }
}

watch(() => normalizedSelectedItems.value.map((item) => item.id), (nextIds) => {
  if (!nextIds.length && !props.noneSelected) {
    closeItemDetailEditor();
    return;
  }

  if (!isSharedDetailMode.value && itemDetailEditorId.value && !nextIds.includes(itemDetailEditorId.value)) {
    closeItemDetailEditor();
  }
});

watch(() => props.selectedItems, () => {
  closeItemDetailEditor();
});

watch(() => props.enableItemDetails, (nextValue) => {
  if (!nextValue) {
    closeItemDetailEditor();
  }
});

watch(() => props.noneSelected, (nextValue) => {
  if (!nextValue) {
    closeItemDetailEditor();
  }
});

watch(() => props.itemDetailMode, () => {
  closeItemDetailEditor();
});

onMounted(() => {
  document.addEventListener("keydown", handleEscape);
});

onBeforeUnmount(() => {
  document.removeEventListener("keydown", handleEscape);
});
</script>

<template>
  <section class="finish-form__section operation-select-picker">
    <div class="product-pick__header">
      <div class="product-pick__copy">
        <label class="finish-form__label">{{ label }}</label>
        <span v-if="helperText" class="product-pick__helper">{{ helperText }}</span>
      </div>

      <button
        v-if="hasVisualSelection"
        class="product-pick__add-btn product-pick__add-btn--header"
        :class="{ 'is-open': dropdownOpen }"
        type="button"
        :aria-expanded="dropdownOpen ? 'true' : 'false'"
        :data-testid="testidPrefix ? `${testidPrefix}-trigger` : null"
        @click="toggleDropdown"
      >
        <span class="material-icons-round">add</span>
      </button>
    </div>

    <div ref="containerRef" class="product-pick">
      <div class="product-pick__inline-row">
        <button
          v-if="!hasVisualSelection"
          class="product-pick__add-btn"
          :class="{
            'is-open': dropdownOpen,
            'product-pick__add-btn--empty': !hasVisualSelection
          }"
          type="button"
          :aria-expanded="dropdownOpen ? 'true' : 'false'"
          :data-testid="testidPrefix ? `${testidPrefix}-trigger` : null"
          @click="toggleDropdown"
        >
          <span class="material-icons-round">add</span>
          <span>{{ triggerLabel }}</span>
        </button>

        <button
          v-if="allowNone && useInlineNoneAction && !hasVisualSelection"
          class="product-pick__none-btn product-pick__none-btn--icon"
          type="button"
          :title="noneLabel"
          :aria-label="noneLabel"
          :data-testid="testidPrefix ? `${testidPrefix}-none` : null"
          @click="toggleNone"
        >
          <span class="material-icons-round">do_not_disturb_on</span>
        </button>

        <span v-if="noneSelected && selectedCount === 0" class="product-pick__tag">
          <span class="product-pick__tag-content">
            <span class="product-pick__tag-label">{{ noneStateLabel || noneLabel }}</span>
          </span>
          <button
            type="button"
            class="product-pick__tag-remove"
            title="Remover"
            :data-testid="testidPrefix ? `${testidPrefix}-none-remove` : null"
            @click="toggleNone"
          >
            <span class="material-icons-round">close</span>
          </button>
        </span>

        <span
          v-for="item in normalizedSelectedItems"
          :key="item.id"
          class="product-pick__tag"
          :class="{ 'product-pick__tag--closed': isClosedMode }"
        >
          <span class="product-pick__tag-content">
            <small v-if="isClosedMode && item.code" class="product-pick__closed-code">{{ item.code }}</small>
            <span class="product-pick__tag-label">{{ item.label }}</span>
          </span>
          <span v-if="isClosedMode && item.price > 0" class="product-pick__tag-price">{{ formatCurrency(item.price) }}</span>
          <button
            v-if="enableItemDetails && !isSharedDetailMode"
            type="button"
            class="product-pick__tag-detail-btn"
            :class="{ 'is-filled': hasItemDetail(item.id), 'is-open': isItemDetailEditorOpen(item.id) }"
            :title="hasItemDetail(item.id) ? `Editar ${itemDetailLabel.toLowerCase()}` : `Adicionar ${itemDetailLabel.toLowerCase()}`"
            :aria-label="hasItemDetail(item.id) ? `Editar ${itemDetailLabel.toLowerCase()}` : `Adicionar ${itemDetailLabel.toLowerCase()}`"
            :data-testid="testidPrefix ? `${testidPrefix}-detail-trigger-${item.id}` : null"
            @click.stop="openItemDetailEditor(item.id)"
          >
            <span class="material-icons-round">{{ hasItemDetail(item.id) ? "edit_note" : "note_add" }}</span>
          </button>
          <button
            type="button"
            class="product-pick__tag-remove"
            title="Remover"
            :data-testid="testidPrefix ? `${testidPrefix}-remove-${item.id}` : null"
            @click="removeSelectedItem(item.id)"
          >
            <span class="material-icons-round">close</span>
          </button>
        </span>

        <button
          v-if="canEditSharedDetail"
          type="button"
          class="product-pick__shared-detail-btn"
          :class="{ 'is-filled': hasSharedDetail, 'is-open': isSharedDetailEditorOpen }"
          :title="hasSharedDetail ? `Editar ${itemDetailLabel.toLowerCase()} compartilhada` : `Adicionar ${itemDetailLabel.toLowerCase()} compartilhada`"
          :aria-label="hasSharedDetail ? `Editar ${itemDetailLabel.toLowerCase()} compartilhada` : `Adicionar ${itemDetailLabel.toLowerCase()} compartilhada`"
          :data-testid="testidPrefix ? `${testidPrefix}-shared-detail-trigger` : null"
          @click.stop="openSharedDetailEditor"
        >
          <span class="material-icons-round">{{ hasSharedDetail ? "edit_note" : "note_add" }}</span>
        </button>
      </div>

      <div
        v-if="enableItemDetails && ((isSharedDetailMode && isSharedDetailEditorOpen) || (!isSharedDetailMode && activeDetailItem))"
        class="product-pick__detail-popover"
      >
        <div class="product-pick__detail-head">
          <div class="product-pick__detail-copy">
            <span class="product-pick__detail-title">{{ activeDetailTitle }}</span>
            <span class="product-pick__detail-caption">
              {{ activeDetailCaption }}
            </span>
          </div>

          <button
            type="button"
            class="product-pick__detail-close"
            aria-label="Fechar editor de detalhe"
            @click="closeItemDetailEditor"
          >
            <span class="material-icons-round">close</span>
          </button>
        </div>

        <textarea
          v-model="itemDetailDraft"
          class="finish-form__textarea product-pick__detail-textarea"
          rows="3"
          :placeholder="itemDetailPlaceholder"
          :data-testid="itemDetailTestid || (testidPrefix ? `${testidPrefix}-detail` : null)"
        />

        <div class="product-pick__detail-actions">
          <button
            type="button"
            class="product-pick__detail-action product-pick__detail-action--ghost"
            @click="closeItemDetailEditor"
          >
            Cancelar
          </button>
          <button
            type="button"
            class="product-pick__detail-action product-pick__detail-action--primary"
            @click="saveItemDetail"
          >
            Salvar
          </button>
        </div>
      </div>

      <Teleport to="body">
        <template v-if="dropdownOpen">
          <button
            class="product-pick__scrim"
            type="button"
            tabindex="-1"
            aria-label="Fechar seletor"
            @click="closeDropdown"
          />

          <div
            class="product-pick__dropdown is-open"
            :style="dropdownStyle"
            :data-testid="testidPrefix ? `${testidPrefix}-dropdown` : null"
          >
            <label class="catalog-picker__search">
              <span class="material-icons-round">search</span>
              <input
                ref="searchInputRef"
                v-model="searchTerm"
                class="catalog-picker__search-input"
                type="search"
                :placeholder="searchPlaceholder"
                :data-testid="testidPrefix ? `${testidPrefix}-search` : null"
              >
            </label>

            <button
              v-if="allowNone && !useInlineNoneAction"
              class="product-pick__option product-pick__option--special"
              :class="{ 'product-pick__option--active': noneSelected }"
              type="button"
              :data-testid="testidPrefix ? `${testidPrefix}-none` : null"
              @click="toggleNone"
            >
              <span class="material-icons-round">do_not_disturb_on</span>
              <span>{{ noneSelected ? (noneStateLabel || noneLabel) : noneLabel }}</span>
            </button>

            <button
              v-if="allowCustom"
              class="product-pick__option product-pick__option--special"
              type="button"
              :data-testid="testidPrefix ? `${testidPrefix}-custom-option` : null"
              @click="toggleCustomForm"
            >
              <span class="material-icons-round">add_circle</span>
              <span>{{ customOptionLabel }}</span>
            </button>

            <div v-if="allowCustom" class="product-pick__custom-form" :class="{ 'is-open': customOpen }">
              <div class="product-pick__custom-fields">
                <div class="product-pick__custom-field product-pick__custom-field--code">
                  <input
                    v-model="customCode"
                    type="text"
                    class="finish-form__input"
                    :placeholder="customCodePlaceholder"
                    :data-testid="testidPrefix ? `${testidPrefix}-custom-code` : null"
                  >
                </div>
              </div>
              <div class="product-pick__custom-actions">
                <button
                  class="column-action column-action--secondary"
                  type="button"
                  @click="toggleCustomForm"
                >
                  Cancelar
                </button>
                <button
                  class="column-action column-action--primary"
                  type="button"
                  :disabled="!customCode.trim()"
                  :data-testid="testidPrefix ? `${testidPrefix}-custom-confirm` : null"
                  @click="addCustomItem"
                >
                  Confirmar
                </button>
              </div>
            </div>

            <div class="product-pick__results">
              <button
                v-for="item in filteredOptions"
                :key="item.id"
                class="product-pick__option"
                type="button"
                :data-testid="testidPrefix ? `${testidPrefix}-option-${item.id}` : null"
                @click="selectOption(item)"
              >
                <span class="product-pick__option-name">{{ item.label }}</span>
                <span v-if="item.meta" class="product-pick__option-meta">{{ item.meta }}</span>
              </button>

              <div v-if="filteredOptions.length === 0" class="product-pick__empty">
                {{ emptySearchLabel }}
              </div>
            </div>
          </div>
        </template>
      </Teleport>

      <div v-if="isClosedMode && total > 0" class="product-pick__total">
        <span>Total:</span>
        <strong>{{ formatCurrency(total) }}</strong>
      </div>
    </div>
  </section>
</template>
