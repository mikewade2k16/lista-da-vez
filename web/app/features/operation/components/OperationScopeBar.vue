<script setup>
import { computed } from "vue";
import AppSelectField from "~/components/ui/AppSelectField.vue";

const props = defineProps({
  canSeeIntegrated: {
    type: Boolean,
    default: false
  },
  scopeMode: {
    type: String,
    default: "single"
  },
  stores: {
    type: Array,
    default: () => []
  },
  activeStoreId: {
    type: String,
    default: ""
  },
  integratedStoreId: {
    type: String,
    default: ""
  }
});

const emit = defineEmits(["scope-change", "active-store-change", "integrated-store-change"]);

const storeOptions = computed(() =>
  (Array.isArray(props.stores) ? props.stores : []).map((store) => ({
    value: String(store?.id || "").trim(),
    label: String(store?.name || "").trim()
  }))
);

const modeOptions = [
  { value: "single", label: "Loja ativa" },
  { value: "all", label: "Todas as lojas" }
];

const integratedFilterOptions = computed(() => [
  { value: "", label: "Todas as lojas" },
  ...storeOptions.value
]);

function handleScopeChange(value) {
  emit("scope-change", String(value || "single").trim() || "single");
}

function handleActiveStoreChange(value) {
  emit("active-store-change", String(value || "").trim());
}

function handleIntegratedStoreChange(value) {
  emit("integrated-store-change", String(value || "").trim());
}
</script>

<template>
  <section class="operation-scope-bar">
    <div class="operation-scope-bar__copy">
      <strong class="operation-scope-bar__title">Visao da operacao</strong>
      <span class="operation-scope-bar__text">
        {{ scopeMode === "all" ? "Acompanhamento integrado das lojas acessiveis." : "Acompanhamento da loja selecionada." }}
      </span>
    </div>

    <div class="operation-scope-bar__controls">
      <AppSelectField
        class="operation-scope-bar__field"
        label="Loja"
        :model-value="activeStoreId"
        :options="storeOptions"
        testid="operation-filter-store"
        @update:model-value="handleActiveStoreChange"
      />

      <AppSelectField
        v-if="canSeeIntegrated"
        class="operation-scope-bar__field"
        label="Modo"
        :model-value="scopeMode"
        :options="modeOptions"
        testid="operation-filter-scope"
        @update:model-value="handleScopeChange"
      />

      <AppSelectField
        v-if="canSeeIntegrated && scopeMode === 'all'"
        class="operation-scope-bar__field"
        label="Filtro por loja"
        :model-value="integratedStoreId"
        :options="integratedFilterOptions"
        testid="operation-filter-integrated-store"
        @update:model-value="handleIntegratedStoreChange"
      />
    </div>
  </section>
</template>

<style scoped>
.operation-scope-bar {
  display: flex;
  align-items: end;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.85rem 1rem;
  border: 1px solid rgba(125, 146, 255, 0.18);
  border-radius: 1rem;
  background: rgba(10, 16, 32, 0.72);
}

.operation-scope-bar__copy {
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
}

.operation-scope-bar__title {
  font-size: 0.95rem;
  font-weight: 700;
}

.operation-scope-bar__text {
  color: rgba(219, 226, 255, 0.72);
  font-size: 0.8rem;
}

.operation-scope-bar__controls {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 0.75rem;
}

.operation-scope-bar__field {
  min-width: 11rem;
}

@media (max-width: 900px) {
  .operation-scope-bar {
    align-items: stretch;
    flex-direction: column;
  }

  .operation-scope-bar__controls {
    justify-content: stretch;
  }

  .operation-scope-bar__field {
    min-width: 0;
    width: 100%;
  }
}
</style>
