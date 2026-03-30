<script setup>
const props = defineProps({
  item: {
    type: Object,
    required: true
  }
});

function getSeverityLabel(level) {
  if (level === "critical") {
    return "Critico";
  }

  if (level === "attention") {
    return "Atencao";
  }

  return "Saudavel";
}
</script>

<template>
  <article class="insight-card intel-card">
    <header class="intel-card__header">
      <h3 class="insight-card__title">{{ item.title }}</h3>
      <span :class="`intel-badge intel-badge--${item.level}`">{{ getSeverityLabel(item.level) }}</span>
    </header>
    <p class="intel-card__text"><strong>Leitura:</strong> {{ item.reading }}</p>
    <p class="intel-card__text"><strong>O que pode estar acontecendo:</strong> {{ item.hypothesis }}</p>
    <p class="intel-card__text"><strong>Acao recomendada:</strong> {{ item.action }}</p>
  </article>
</template>
