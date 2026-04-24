<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { Check, ChevronDown, Plus, Search } from "lucide-vue-next";

const props = defineProps({
  label: {
    type: String,
    default: ""
  },
  modelValue: {
    type: String,
    default: ""
  },
  options: {
    type: Array,
    default: () => []
  },
  placeholder: {
    type: String,
    default: "Selecionar opcao"
  },
  emptyLabel: {
    type: String,
    default: "Nenhuma opcao encontrada."
  },
  searchPlaceholder: {
    type: String,
    default: "Buscar opcao"
  },
  searchable: {
    type: Boolean,
    default: false
  },
  showLeadingIcon: {
    type: Boolean,
    default: true
  },
  compact: {
    type: Boolean,
    default: false
  },
  testid: {
    type: String,
    default: ""
  },
  disabled: {
    type: Boolean,
    default: false
  }
});

const emit = defineEmits(["update:modelValue", "change"]);

const containerRef = ref(null);
const searchInputRef = ref(null);
const dropdownOpen = ref(false);
const dropdownStyle = ref({});
const searchTerm = ref("");

const normalizedOptions = computed(() =>
  (Array.isArray(props.options) ? props.options : []).map((option) => ({
    value: String(option?.value ?? "").trim(),
    label: String(option?.label ?? "").trim(),
    meta: String(option?.meta ?? option?.description ?? "").trim()
  }))
);

const selectedValue = computed(() => String(props.modelValue || "").trim());
const selectedOption = computed(() =>
  normalizedOptions.value.find((option) => option.value === selectedValue.value) || null
);
const shouldShowSearch = computed(() => props.searchable || normalizedOptions.value.length >= 8);
const filteredOptions = computed(() => {
  const normalizedSearch = normalizeSearch(searchTerm.value);

  if (!normalizedSearch) {
    return normalizedOptions.value;
  }

  return normalizedOptions.value.filter((option) =>
    normalizeSearch(`${option.label} ${option.meta}`.trim()).includes(normalizedSearch)
  );
});

function normalizeSearch(value) {
  return String(value || "")
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .trim()
    .toLowerCase();
}

function updateDropdownPosition() {
  if (!containerRef.value) {
    return;
  }

  const rect = containerRef.value.getBoundingClientRect();
  dropdownStyle.value = {
    top: `${rect.bottom + 4}px`,
    left: `${rect.left}px`,
    width: `${rect.width}px`
  };
}

function closeDropdown() {
  dropdownOpen.value = false;
  searchTerm.value = "";
}

function openDropdown() {
  if (props.disabled) {
    return;
  }

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

function selectOption(value) {
  const nextValue = String(value || "").trim();
  emit("update:modelValue", nextValue);
  emit("change", nextValue);
  closeDropdown();
}

function handleEscape(event) {
  if (event.key === "Escape") {
    closeDropdown();
  }
}

function handleViewportChange() {
  if (dropdownOpen.value) {
    updateDropdownPosition();
  }
}

watch(
  () => props.modelValue,
  () => {
    if (!dropdownOpen.value) {
      return;
    }

    closeDropdown();
  }
);

onMounted(() => {
  document.addEventListener("keydown", handleEscape);
  window.addEventListener("resize", handleViewportChange);
  window.addEventListener("scroll", handleViewportChange, true);
});

onBeforeUnmount(() => {
  document.removeEventListener("keydown", handleEscape);
  window.removeEventListener("resize", handleViewportChange);
  window.removeEventListener("scroll", handleViewportChange, true);
});
</script>

<template>
  <div
    ref="containerRef"
    class="app-select-field"
    :class="{ 'app-select-field--compact': compact }"
    :data-testid="testid || null"
  >
    <span v-if="label" class="app-select-field__label">{{ label }}</span>

    <button
      class="product-pick__add-btn product-pick__add-btn--empty app-select-field__trigger"
      :class="{ 'is-open': dropdownOpen, 'is-disabled': disabled, 'is-filled': selectedOption }"
      type="button"
      :disabled="disabled"
      :aria-expanded="dropdownOpen ? 'true' : 'false'"
      @click="toggleDropdown"
    >
      <span class="app-select-field__trigger-main">
        <component :is="selectedOption ? Check : Plus" v-if="showLeadingIcon" class="app-select-field__trigger-icon" :size="16" :stroke-width="2.1" />
        <span class="app-select-field__trigger-text">
          {{ selectedOption?.label || placeholder }}
        </span>
      </span>
      <ChevronDown class="app-select-field__trigger-arrow" :size="16" :stroke-width="2.1" />
    </button>

    <Teleport to="body">
      <template v-if="dropdownOpen">
        <button
          class="product-pick__scrim"
          type="button"
          tabindex="-1"
          aria-label="Fechar seletor"
          @click="closeDropdown"
        />

        <div class="product-pick__dropdown is-open app-select-field__dropdown" :style="dropdownStyle">
          <label v-if="shouldShowSearch" class="catalog-picker__search">
            <Search class="app-select-field__search-icon" :size="14" :stroke-width="2.1" />
            <input
              ref="searchInputRef"
              v-model="searchTerm"
              class="catalog-picker__search-input"
              type="search"
              :placeholder="searchPlaceholder"
            >
          </label>

          <div class="product-pick__results">
            <button
              v-for="option in filteredOptions"
              :key="`${option.value}-${option.label}`"
              class="product-pick__option app-select-field__option"
              :class="{ 'is-selected': option.value === selectedValue }"
              type="button"
              @click="selectOption(option.value)"
            >
              <span class="app-select-field__option-row">
                <span class="product-pick__option-name">{{ option.label }}</span>
                <Check
                  v-if="option.value === selectedValue"
                  class="app-select-field__option-check"
                  :size="14"
                  :stroke-width="2.4"
                />
              </span>
              <span v-if="option.meta" class="product-pick__option-meta">{{ option.meta }}</span>
            </button>

            <div v-if="filteredOptions.length === 0" class="product-pick__empty">
              {{ emptyLabel }}
            </div>
          </div>
        </div>
      </template>
    </Teleport>
  </div>
</template>

<style scoped>
.app-select-field {
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
  min-width: 0;
}

.app-select-field__label {
  color: rgba(219, 226, 255, 0.74);
  font-size: 0.72rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.06em;
}

.app-select-field__trigger {
  width: 100%;
  justify-content: space-between;
  min-width: 0;
}

.app-select-field__trigger.is-filled {
  border-color: rgba(129, 140, 248, 0.24);
  background: rgba(129, 140, 248, 0.08);
}

.app-select-field__trigger.is-disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.app-select-field__trigger-main {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.app-select-field__trigger-icon,
.app-select-field__trigger-arrow,
.app-select-field__option-check,
.app-select-field__search-icon {
  width: 16px;
  height: 16px;
  flex-shrink: 0;
}

.app-select-field__search-icon {
  color: var(--text-muted);
}

.app-select-field__trigger-text {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.app-select-field__dropdown {
  max-height: min(320px, 60vh);
}

.app-select-field__option-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  width: 100%;
}

.app-select-field__option.is-selected {
  background: rgba(129, 140, 248, 0.12);
}

.app-select-field__option-check {
  color: #c7d2fe;
}

.app-select-field--compact {
  gap: 0;
}

.app-select-field--compact .app-select-field__trigger {
  min-height: 36px;
  padding: 0 12px;
  border-radius: 999px;
  font-size: 0.74rem;
}
</style>
