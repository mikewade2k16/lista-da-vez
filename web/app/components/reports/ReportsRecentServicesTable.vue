<script setup>
defineProps({
  rows: {
    type: Array,
    default: () => []
  },
  total: {
    type: Number,
    default: 0
  }
});
</script>

<template>
  <article class="insight-card insight-card--wide">
    <header class="intel-card__header">
      <h3 class="insight-card__title">Ultimos atendimentos</h3>
      <span class="insight-tag">{{ total || rows.length }} no periodo</span>
    </header>

    <div class="insight-table-wrap">
      <table class="insight-table">
        <thead>
          <tr>
            <th>Loja</th>
            <th>Data/Hora</th>
            <th>Consultor</th>
            <th>Desfecho</th>
            <th>Cliente</th>
            <th>Produto fechado</th>
            <th>Origem</th>
            <th>Motivo fora da vez</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!rows.length">
            <td colspan="8">Nenhum atendimento recente para os filtros selecionados.</td>
          </tr>
          <tr v-for="row in rows" :key="`${row.serviceId}-${row.finishedAt}`">
            <td>{{ row.storeName }}</td>
            <td>{{ row.finishedAtLabel }}</td>
            <td>{{ row.consultantName }}</td>
            <td>{{ row.outcomeLabel }}</td>
            <td>{{ row.customerName }}</td>
            <td>{{ row.productClosed }}</td>
            <td>{{ row.customerSourcesLabel }}</td>
            <td>{{ row.queueJumpReason }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </article>
</template>
