<script setup lang="ts">
import type { SegmentMaterializationView } from '~/domain/customer-data/segment-types'

defineProps<{ materializations: SegmentMaterializationView[] }>()
</script>

<template>
  <section class="segment-materializations">
    <h3>Materializacoes</h3>
    <p class="segment-materializations__notice" role="status">
      Exportacao ainda indisponivel: os snapshots abaixo sao somente para consulta ate a API
      governada ser implementada.
    </p>
    <p v-if="!materializations.length">Nenhum snapshot materializado.</p>
    <article v-for="item in materializations" :key="item.id">
      <div>
        <strong>{{ item.status }}</strong>
        <span>{{ item.countBucket || 'contagem protegida' }}</span>
        <span>{{ item.freshnessStatus }} · {{ new Date(item.asOf).toLocaleString('pt-BR') }}</span>
      </div>
      <span v-if="item.status === 'current'" class="segment-materializations__unavailable">
        Exportacao indisponivel
      </span>
    </article>
  </section>
</template>

<style scoped>
.segment-materializations {
  display: grid;
  gap: 0.65rem;
  padding: 1rem;
  border: 1px solid rgb(var(--border) / 0.8);
  border-radius: 0.9rem;
}

.segment-materializations h3,
.segment-materializations p {
  margin: 0;
}

.segment-materializations article,
.segment-materializations article div {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.55rem;
  flex-wrap: wrap;
}

.segment-materializations span,
.segment-materializations p {
  color: rgb(var(--muted));
  font-size: 0.72rem;
}

.segment-materializations__notice {
  padding: 0.65rem;
  border: 1px solid rgb(var(--border) / 0.8);
  border-radius: 0.65rem;
}

.segment-materializations__unavailable {
  font-weight: 600;
}
</style>
