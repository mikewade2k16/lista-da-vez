<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from "vue";
import { useUiStore } from "~/stores/ui";

const props = defineProps({
  label: {
    type: String,
    required: true
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
    default: "Codigo (opcional)"
  },
  customNamePlaceholder: {
    type: String,
    default: "Nome do item *"
  },
  customPricePlaceholder: {
    type: String,
    default: "Valor R$"
  },
  testidPrefix: {
    type: String,
    default: ""
  }
});

const emit = defineEmits(["update:selectedItems", "update:noneSelected"]);
const ui = useUiStore();

const searchInputRef = ref(null);
const containerRef = ref(null);
const dropdownStyle = ref({});
const dropdownOpen = ref(false);
const customOpen = ref(false);
const searchTerm = ref("");
const customCode = ref("");
const customName = ref("");
const customPrice = ref("");

const isClosedMode = computed(() => props.mode === "closed");
const normalizedOptions = computed(() => (Array.isArray(props.options) ? props.options : []).map(normalizeOption));
const normalizedSelectedItems = computed(() =>
  (Array.isArray(props.selectedItems) ? props.selectedItems : []).map(normalizeOption)
);
const selectedCount = computed(() => normalizedSelectedItems.value.length);
const total = computed(() =>
  normalizedSelectedItems.value.reduce((sum, item) => sum + (Number(item.price) || 0), 0)
);
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

function clearCustomForm() {
  customCode.value = "";
  customName.value = "";
  customPrice.value = "";
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
    return;
  }

  emitSelectedItems([]);
  emit("update:noneSelected", true);
  closeDropdown();
}

function selectOption(item) {
  const nextItems = props.multiple ? [...normalizedSelectedItems.value, item] : [item];
  emitSelectedItems(nextItems);
  emit("update:noneSelected", false);
  closeDropdown();
}

function addCustomItem() {
  const label = customName.value.trim();

  if (!label) {
    void ui.alert("Informe o nome do item.");
    return;
  }

  const nextItem = normalizeOption({
    id: `__custom__${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
    label,
    name: label,
    code: customCode.value.trim(),
    price: Math.max(0, Number(customPrice.value || 0)),
    isCustom: true
  });

  const nextItems = props.multiple ? [...normalizedSelectedItems.value, nextItem] : [nextItem];
  emitSelectedItems(nextItems);
  emit("update:noneSelected", false);
  closeDropdown();
}

function removeSelectedItem(itemId) {
  emitSelectedItems(normalizedSelectedItems.value.filter((item) => item.id !== itemId));
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

function handleEscape(event) {
  if (event.key === "Escape") {
    closeDropdown();
  }
}

onMounted(() => {
  document.addEventListener("keydown", handleEscape);
});

onBeforeUnmount(() => {
  document.removeEventListener("keydown", handleEscape);
});
</script>

<template>
  <section class="finish-form__section operation-select-picker">
    <label class="finish-form__label">{{ label }}</label>

    <div ref="containerRef" class="product-pick">
      <div class="product-pick__inline-row">
        <button
          class="product-pick__add-btn"
          :class="{
            'is-open': dropdownOpen,
            'product-pick__add-btn--empty': selectedCount === 0 && !noneSelected
          }"
          type="button"
          :aria-expanded="dropdownOpen ? 'true' : 'false'"
          :data-testid="testidPrefix ? `${testidPrefix}-trigger` : null"
          @click="toggleDropdown"
        >
          <span class="material-icons-round">add</span>
          <span v-if="selectedCount === 0 && !noneSelected">{{ triggerLabel }}</span>
        </button>

        <button
          v-if="allowNone"
          class="product-pick__none-btn product-pick__none-btn--icon"
          :class="{ 'is-active': noneSelected }"
          type="button"
          :title="noneLabel"
          :aria-label="noneLabel"
          :data-testid="testidPrefix ? `${testidPrefix}-none` : null"
          @click="toggleNone"
        >
          <span class="material-icons-round">do_not_disturb_on</span>
        </button>

        <span v-if="noneSelected && selectedCount === 0" class="product-pick__tag product-pick__tag--muted">
          {{ noneStateLabel || noneLabel }}
        </span>

        <span
          v-for="item in normalizedSelectedItems"
          :key="item.id"
          class="product-pick__tag"
        >
          <span class="product-pick__tag-label">
            {{ item.label }}<small v-if="isClosedMode && item.code" class="product-pick__closed-code"> ({{ item.code }})</small>
          </span>
          <span v-if="isClosedMode && item.price > 0" class="product-pick__tag-price">{{ formatCurrency(item.price) }}</span>
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
                <input
                  v-model="customCode"
                  type="text"
                  class="finish-form__input"
                  :placeholder="customCodePlaceholder"
                  :data-testid="testidPrefix ? `${testidPrefix}-custom-code` : null"
                >
                <input
                  v-model="customName"
                  type="text"
                  class="finish-form__input"
                  :placeholder="customNamePlaceholder"
                  :data-testid="testidPrefix ? `${testidPrefix}-custom-name` : null"
                >
                <input
                  v-model="customPrice"
                  type="number"
                  class="finish-form__input"
                  :placeholder="customPricePlaceholder"
                  min="0"
                  step="0.01"
                  :data-testid="testidPrefix ? `${testidPrefix}-custom-price` : null"
                >
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
