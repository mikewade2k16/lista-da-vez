<script setup>
function formatPercent(value) {
  return `${Number(value || 0).toFixed(1)}%`;
}

defineProps({
  quality: {
    type: Object,
    required: true
  }
});
</script>

<template>
  <article class="settings-card">
    <header class="settings-card__header">
      <h3 class="settings-card__title">Qualidade do preenchimento</h3>
      <p class="settings-card__text">
        Quem preenche bem, quem adiciona observacoes e quem ainda deixa o fechamento incompleto.
      </p>
    </header>

    <section class="metric-grid metric-grid--tight">
      <article class="metric-card metric-card--soft">
        <span class="metric-card__label">Preenchimento correto</span>
        <strong class="metric-card__value">{{ formatPercent(quality.completeRate) }}</strong>
        <span class="metric-card__text">{{ quality.completeCount }} atendimentos completos</span>
      </article>
      <article class="metric-card metric-card--soft">
        <span class="metric-card__label">Completo + observacao</span>
        <strong class="metric-card__value">{{ formatPercent(quality.excellentRate) }}</strong>
        <span class="metric-card__text">{{ quality.excellentCount }} atendimentos com observacoes</span>
      </article>
      <article class="metric-card metric-card--soft">
        <span class="metric-card__label">Incompletos</span>
        <strong class="metric-card__value">{{ formatPercent(quality.incompleteRate) }}</strong>
        <span class="metric-card__text">{{ quality.incompleteCount }} atendimentos com falhas de preenchimento</span>
      </article>
      <article class="metric-card metric-card--soft">
        <span class="metric-card__label">Observacoes</span>
        <strong class="metric-card__value">{{ formatPercent(quality.notesRate) }}</strong>
        <span class="metric-card__text">{{ quality.notesCount }} atendimentos com anotacoes</span>
      </article>
    </section>

    <div class="insight-table-wrap">
      <table class="insight-table">
        <thead>
          <tr>
            <th>Consultor</th>
            <th>Atendimentos</th>
            <th>Completo</th>
            <th>Completo + obs</th>
            <th>Incompleto</th>
            <th>Observacoes</th>
            <th>Nivel</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!quality.byConsultant.length">
            <td colspan="7">Sem dados suficientes para avaliar preenchimento.</td>
          </tr>
          <tr v-for="item in quality.byConsultant" :key="item.consultantId">
            <td>{{ item.consultantName }}</td>
            <td>{{ item.totalAttendances }}</td>
            <td>{{ formatPercent(item.completeRate) }}</td>
            <td>{{ formatPercent(item.excellentRate) }}</td>
            <td>{{ formatPercent(item.incompleteRate) }}</td>
            <td>{{ formatPercent(item.notesRate) }}</td>
            <td>
              <span :class="`report-quality-badge report-quality-badge--${item.qualityLevelKey}`">
                {{ item.qualityLevelLabel }}
              </span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </article>
</template>
