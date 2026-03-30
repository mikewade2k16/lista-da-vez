<script setup>
import { computed } from "vue";

const props = defineProps({
  rows: {
    type: Array,
    default: () => []
  }
});

const limitedRows = computed(() => props.rows.slice(0, 200));
</script>

<template>
  <article class="insight-card insight-card--wide">
    <header class="intel-card__header">
      <h3 class="insight-card__title">Atendimentos filtrados</h3>
      <span class="insight-tag">{{ rows.length }} registros</span>
    </header>

    <div class="insight-table-wrap">
      <table class="insight-table">
        <thead>
          <tr>
            <th>Loja</th>
            <th>Data/Hora</th>
            <th>Consultor</th>
            <th>Desfecho</th>
            <th>Valor</th>
            <th>Duracao</th>
            <th>Espera fila</th>
            <th>Preenchimento</th>
            <th>Modo</th>
            <th>Flags</th>
            <th>Cliente</th>
            <th>Origem</th>
            <th>Campanhas</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!limitedRows.length">
            <td colspan="13">Sem dados para os filtros selecionados.</td>
          </tr>
          <tr v-for="row in limitedRows" :key="`${row.serviceId}-${row.finishedAt}`">
            <td>{{ row.storeName }}</td>
            <td>{{ row.finishedAtLabel }}</td>
            <td>{{ row.consultantName }}</td>
            <td>{{ row.outcomeLabel }}</td>
            <td>{{ row.saleAmountLabel }}</td>
            <td>{{ row.durationLabel }}</td>
            <td>{{ row.queueWaitLabel }}</td>
            <td>{{ row.completionLabel }}</td>
            <td>{{ row.startModeLabel }}</td>
            <td>
              <span v-if="row.isWindowService" class="insight-tag insight-tag--sm">Vitrine</span>
              <span v-if="row.isGift" class="insight-tag insight-tag--sm">Presente</span>
              <span v-if="row.isExistingCustomer" class="insight-tag insight-tag--sm">Já cliente</span>
              <span v-if="!row.isWindowService && !row.isGift && !row.isExistingCustomer">-</span>
            </td>
            <td>{{ row.customerName }}</td>
            <td>{{ row.customerSourcesLabel }}</td>
            <td>{{ row.campaignNamesLabel }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <p v-if="rows.length > limitedRows.length" class="settings-card__text">
      Mostrando os primeiros {{ limitedRows.length }} registros na tela.
    </p>
  </article>
</template>
