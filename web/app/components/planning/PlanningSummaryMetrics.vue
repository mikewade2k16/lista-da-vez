<script setup lang="ts">
defineProps<{
  scheduledHours: number
  contractedHours: number
  coveragePercent: number
  coveredDayCount: number
  openDayCount: number
  hardIssueCount: number
  warningCount: number
  staffCount: number
  locationLabel: string
}>()
</script>

<template>
  <div class="planning-metrics">
    <article class="omni-glass">
      <span>Horas escaladas</span>
      <strong>{{ scheduledHours.toFixed(1) }}h</strong>
      <small>de {{ contractedHours.toFixed(0) }}h contratadas</small>
    </article>
    <article class="omni-glass">
      <span>Cobertura semanal</span>
      <strong>{{ coveragePercent }}%</strong>
      <small>{{ coveredDayCount }} de {{ openDayCount }} dias abertos</small>
    </article>
    <article class="omni-glass" :class="{ 'has-danger': hardIssueCount > 0 }">
      <span>Restrições</span>
      <strong>{{ hardIssueCount }}</strong>
      <small>{{ warningCount }} alerta(s) adicional(is)</small>
    </article>
    <article class="omni-glass">
      <span>Equipe elegível</span>
      <strong>{{ staffCount }}</strong>
      <small>{{ locationLabel }}</small>
    </article>
  </div>
</template>

<style scoped>
.planning-metrics {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 0.65rem;
}
.planning-metrics article {
  display: grid;
  gap: 0.18rem;
  border: 1px solid rgb(var(--border) / 0.68);
  border-radius: var(--radius-card);
  padding: 0.75rem 0.85rem;
  background: rgb(var(--surface) / 0.8);
}
.planning-metrics span,
.planning-metrics small {
  color: var(--text-muted);
  font-size: 0.68rem;
}
.planning-metrics strong {
  color: var(--text-main);
  font-size: 1.28rem;
}
.planning-metrics article.has-danger strong {
  color: rgb(var(--danger));
}
@media (max-width: 1020px) {
  .planning-metrics {
    grid-template-columns: 1fr 1fr;
  }
}
@media (max-width: 620px) {
  .planning-metrics {
    grid-template-columns: 1fr;
  }
}
</style>
