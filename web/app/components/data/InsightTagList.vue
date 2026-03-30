<script setup>
defineProps({
  title: {
    type: String,
    required: true
  },
  items: {
    type: Array,
    default: () => []
  }
});
</script>

<template>
  <article class="insight-card">
    <h3 class="insight-card__title">{{ title }}</h3>
    <div class="insight-tags">
      <template v-if="items.length">
        <span v-for="item in items" :key="`${title}-${item.label}`" class="insight-tag">
          {{ item.label }}
          <strong>{{ item.count }}</strong>
        </span>
      </template>
      <span v-else class="insight-empty">Sem dados.</span>
    </div>
  </article>
</template>
