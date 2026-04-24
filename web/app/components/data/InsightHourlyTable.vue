<script setup>
import { formatCurrencyBRL } from "~/domain/utils/admin-metrics";

defineProps({
  items: {
    type: Array,
    default: () => []
  }
});
</script>

<template>
  <article class="insight-card">
    <h3 class="insight-card__title">Horarios com mais venda</h3>
    <div class="insight-table-wrap">
      <table class="insight-table">
        <thead>
          <tr>
            <th>Horario</th>
            <th>Vendas</th>
            <th>Valor</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!items.length">
            <td colspan="3">Sem dados.</td>
          </tr>
          <tr v-for="item in items" :key="item.label">
            <td>{{ item.label }}</td>
            <td>{{ item.count }}</td>
            <td>{{ formatCurrencyBRL(item.value) }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </article>
</template>
