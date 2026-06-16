<script setup lang="ts">
// Shell visual de uma secao do editor: titulo + descricao opcional + slot de
// campos. Mantem as secoes consistentes e curtas (cada uma so descreve os
// campos do seu bloco do contrato).

defineProps<{
  title: string
  description?: string
}>()
</script>

<template>
  <section class="bio-section-card">
    <header class="bio-section-card__head">
      <h2 class="bio-section-card__title">{{ title }}</h2>
      <p v-if="description" class="bio-section-card__desc">{{ description }}</p>
    </header>
    <div class="bio-section-card__body">
      <slot></slot>
    </div>
  </section>
</template>

<style scoped>
.bio-section-card {
  display: flex;
  flex-direction: column;
  gap: 0.85rem;
}

.bio-section-card__head {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.bio-section-card__title {
  font-size: 1.05rem;
  font-weight: 700;
  color: var(--text-main);
}

.bio-section-card__desc {
  margin: 0;
  font-size: 0.85rem;
  color: var(--text-muted);
  line-height: 1.5;
}

.bio-section-card__body {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}
</style>
