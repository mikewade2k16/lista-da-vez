<script setup>
import { computed } from "vue";

const FILTER_GROUPS = [
  { id: "consultantIds", label: "Consultor" },
  { id: "outcomes", label: "Desfecho" },
  { id: "sourceIds", label: "Origem" },
  { id: "visitReasonIds", label: "Motivo" },
  { id: "campaignIds", label: "Campanha" },
  { id: "startModes", label: "Tipo" },
  { id: "existingCustomerModes", label: "Cliente" },
  { id: "completionLevels", label: "Preenchimento" },
  { id: "advanced", label: "Periodo e busca" }
];

const props = defineProps({
  filters: {
    type: Object,
    required: true
  },
  roster: {
    type: Array,
    default: () => []
  },
  visitReasonOptions: {
    type: Array,
    default: () => []
  },
  customerSourceOptions: {
    type: Array,
    default: () => []
  },
  campaigns: {
    type: Array,
    default: () => []
  },
  filtersExpanded: {
    type: Boolean,
    default: false
  },
  expandedGroup: {
    type: String,
    default: null
  }
});

defineEmits([
  "toggle-filters",
  "toggle-group",
  "toggle-value",
  "update-filter",
  "clear-filter",
  "reset-filters",
  "export-csv",
  "export-pdf"
]);

function formatCurrency(value) {
  return new Intl.NumberFormat("pt-BR", {
    style: "currency",
    currency: "BRL"
  }).format(Number(value || 0));
}

function hasActiveValue(value) {
  return Array.isArray(value) ? value.length > 0 : String(value || "").trim().length > 0;
}

function hasActiveGroup(groupId) {
  if (groupId === "advanced") {
    return Boolean(
      props.filters.dateFrom ||
        props.filters.dateTo ||
        props.filters.minSaleAmount ||
        props.filters.maxSaleAmount ||
        props.filters.search
    );
  }

  return hasActiveValue(props.filters[groupId]);
}

const activeChips = computed(() => {
  const consultantMap = new Map((props.roster || []).map((consultant) => [consultant.id, consultant.name]));
  const visitReasonMap = new Map((props.visitReasonOptions || []).map((item) => [item.id, item.label]));
  const customerSourceMap = new Map((props.customerSourceOptions || []).map((item) => [item.id, item.label]));
  const outcomeMap = new Map([
    ["compra", "Compra"],
    ["reserva", "Reserva"],
    ["nao-compra", "Nao compra"]
  ]);
  const startModeMap = new Map([
    ["queue", "Na vez"],
    ["queue-jump", "Fora da vez"]
  ]);
  const existingCustomerMap = new Map([
    ["yes", "Recorrente"],
    ["no", "Novo cliente"]
  ]);
  const completionMap = new Map([
    ["excellent", "Completo + observacao"],
    ["complete", "Completo"],
    ["incomplete", "Incompleto"]
  ]);
  const chips = [];

  (props.filters.consultantIds || []).forEach((value) => {
    chips.push({
      filterId: "consultantIds",
      filterValue: value,
      label: `Consultor: ${consultantMap.get(value) || value}`
    });
  });

  (props.filters.outcomes || []).forEach((value) => {
    chips.push({
      filterId: "outcomes",
      filterValue: value,
      label: `Desfecho: ${outcomeMap.get(value) || value}`
    });
  });

  (props.filters.sourceIds || []).forEach((value) => {
    chips.push({
      filterId: "sourceIds",
      filterValue: value,
      label: `Origem: ${customerSourceMap.get(value) || value}`
    });
  });

  (props.filters.visitReasonIds || []).forEach((value) => {
    chips.push({
      filterId: "visitReasonIds",
      filterValue: value,
      label: `Motivo: ${visitReasonMap.get(value) || value}`
    });
  });

  (props.filters.startModes || []).forEach((value) => {
    chips.push({
      filterId: "startModes",
      filterValue: value,
      label: `Tipo: ${startModeMap.get(value) || value}`
    });
  });

  (props.filters.existingCustomerModes || []).forEach((value) => {
    chips.push({
      filterId: "existingCustomerModes",
      filterValue: value,
      label: `Cliente: ${existingCustomerMap.get(value) || value}`
    });
  });

  (props.filters.completionLevels || []).forEach((value) => {
    chips.push({
      filterId: "completionLevels",
      filterValue: value,
      label: `Preenchimento: ${completionMap.get(value) || value}`
    });
  });

  const campaignMap = new Map((props.campaigns || []).map((c) => [c.id, c.name || c.id]));

  (props.filters.campaignIds || []).forEach((value) => {
    chips.push({
      filterId: "campaignIds",
      filterValue: value,
      label: `Campanha: ${campaignMap.get(value) || value}`
    });
  });

  if (props.filters.dateFrom) {
    chips.push({
      filterId: "dateFrom",
      label: `De: ${props.filters.dateFrom}`
    });
  }

  if (props.filters.dateTo) {
    chips.push({
      filterId: "dateTo",
      label: `Ate: ${props.filters.dateTo}`
    });
  }

  if (props.filters.minSaleAmount) {
    chips.push({
      filterId: "minSaleAmount",
      label: `Min: ${formatCurrency(props.filters.minSaleAmount)}`
    });
  }

  if (props.filters.maxSaleAmount) {
    chips.push({
      filterId: "maxSaleAmount",
      label: `Max: ${formatCurrency(props.filters.maxSaleAmount)}`
    });
  }

  if (props.filters.search) {
    chips.push({
      filterId: "search",
      label: `Busca: ${props.filters.search}`
    });
  }

  return chips;
});

const consultantOptions = computed(() =>
  (props.roster || []).map((consultant) => ({
    value: consultant.id,
    label: consultant.name
  }))
);
const campaignOptions = computed(() =>
  (props.campaigns || []).map((c) => ({
    value: c.id,
    label: c.name || c.id
  }))
);
const sourceOptions = computed(() =>
  (props.customerSourceOptions || []).map((option) => ({
    value: option.id,
    label: option.label
  }))
);
const visitReasonOptions = computed(() =>
  (props.visitReasonOptions || []).map((option) => ({
    value: option.id,
    label: option.label
  }))
);

function getGroupOptions(groupId) {
  if (groupId === "consultantIds") {
    return consultantOptions.value;
  }

  if (groupId === "outcomes") {
    return [
      { value: "compra", label: "Compra" },
      { value: "reserva", label: "Reserva" },
      { value: "nao-compra", label: "Nao compra" }
    ];
  }

  if (groupId === "sourceIds") {
    return sourceOptions.value;
  }

  if (groupId === "visitReasonIds") {
    return visitReasonOptions.value;
  }

  if (groupId === "startModes") {
    return [
      { value: "queue", label: "Na vez" },
      { value: "queue-jump", label: "Fora da vez" }
    ];
  }

  if (groupId === "existingCustomerModes") {
    return [
      { value: "yes", label: "Recorrente" },
      { value: "no", label: "Novo cliente" }
    ];
  }

  if (groupId === "completionLevels") {
    return [
      { value: "excellent", label: "Completo + observacao" },
      { value: "complete", label: "Completo" },
      { value: "incomplete", label: "Incompleto" }
    ];
  }

  if (groupId === "campaignIds") {
    return campaignOptions.value;
  }

  return [];
}
</script>

<template>
  <article class="settings-card report-filters-card">
    <header class="settings-card__header report-filters-card__header">
      <div class="report-filters-card__title-row">
        <h3 class="settings-card__title">Filtros</h3>
        <button
          type="button"
          class="report-filter-toggle"
          :aria-expanded="filtersExpanded ? 'true' : 'false'"
          :aria-label="filtersExpanded ? 'Esconder filtros' : 'Abrir filtros'"
          :title="filtersExpanded ? 'Esconder filtros' : 'Abrir filtros'"
          data-testid="reports-filter-toggle"
          @click="$emit('toggle-filters')"
        >
          <span class="material-icons-round">filter_alt</span>
        </button>
      </div>
    </header>

    <div v-if="activeChips.length" class="report-active-filters">
      <div class="report-active-filters__list">
        <button
          v-for="chip in activeChips"
          :key="`${chip.filterId}-${chip.filterValue || chip.label}`"
          type="button"
          class="report-active-chip"
          title="Remover filtro"
          @click="$emit('clear-filter', chip.filterId, chip.filterValue || null)"
        >
          <span class="report-active-chip__label">{{ chip.label }}</span>
          <span class="report-active-chip__remove material-icons-round">close</span>
        </button>
      </div>

      <button
        type="button"
        class="report-icon-action report-icon-action--subtle"
        aria-label="Limpar filtros"
        title="Limpar filtros"
        @click="$emit('reset-filters')"
      >
        <span class="material-icons-round">filter_alt_off</span>
      </button>
    </div>

    <template v-if="filtersExpanded">
      <div class="report-filter-groups">
        <button
          v-for="group in FILTER_GROUPS"
          :key="group.id"
          type="button"
          :class="[
            'report-filter-group-btn',
            { 'is-active': expandedGroup === group.id, 'has-value': hasActiveGroup(group.id) }
          ]"
          @click="$emit('toggle-group', group.id)"
        >
          {{ group.label }}
        </button>
      </div>

      <div v-if="expandedGroup" class="report-filter-panel">
        <template v-if="expandedGroup === 'advanced'">
          <div class="report-filter-grid">
            <label class="settings-field">
              <span>Data inicial</span>
              <input type="date" :value="filters.dateFrom" @input="$emit('update-filter', 'dateFrom', $event.target.value)">
            </label>
            <label class="settings-field">
              <span>Data final</span>
              <input type="date" :value="filters.dateTo" @input="$emit('update-filter', 'dateTo', $event.target.value)">
            </label>
            <label class="settings-field">
              <span>Valor minimo (R$)</span>
              <input
                type="number"
                min="0"
                step="1"
                :value="filters.minSaleAmount"
                @input="$emit('update-filter', 'minSaleAmount', $event.target.value)"
              >
            </label>
            <label class="settings-field">
              <span>Valor maximo (R$)</span>
              <input
                type="number"
                min="0"
                step="1"
                :value="filters.maxSaleAmount"
                @input="$emit('update-filter', 'maxSaleAmount', $event.target.value)"
              >
            </label>
            <label class="settings-field report-filter-grid__search">
              <span>Busca livre</span>
              <input
                type="text"
                :value="filters.search"
                placeholder="ID, cliente, telefone, produto..."
                @input="$emit('update-filter', 'search', $event.target.value)"
              >
            </label>
          </div>
        </template>

        <template v-else>
          <div class="report-option-cloud">
            <button
              v-for="option in getGroupOptions(expandedGroup)"
              :key="`${expandedGroup}-${option.value}`"
              type="button"
              :class="[
                'report-option-chip',
                { 'is-active': (filters[expandedGroup] || []).includes(option.value) }
              ]"
              @click="$emit('toggle-value', expandedGroup, option.value)"
            >
              {{ option.label }}
            </button>
          </div>
        </template>
      </div>
    </template>

    <div class="report-actions">
      <button type="button" class="report-icon-action" title="Exportar CSV" data-testid="reports-export-csv" @click="$emit('export-csv')">
        <span class="material-icons-round">table_view</span>
      </button>
      <button type="button" class="report-icon-action" title="Exportar PDF" data-testid="reports-export-pdf" @click="$emit('export-pdf')">
        <span class="material-icons-round">picture_as_pdf</span>
      </button>
    </div>
  </article>
</template>
