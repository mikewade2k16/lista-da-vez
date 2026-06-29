<script setup lang="ts">
import CoreErrorState from '../../../../layers/core/components/CoreErrorState.vue'

// Shell de bloco do dashboard (F4). Padroniza loading/erro/vazio + cabecalho dos
// blocos para nao duplicar markup em cada um. NAO faz fetch: recebe os estados por
// prop e emite retry (o pai re-chama o refresh do bloco no composable).
//
// Estados (precedencia): error > loading > empty > slot default.
//  - loading: placeholder pulsante (sem dado ainda).
//  - error:   CoreErrorState compacto (o mesmo que CoreAsyncError reusa) com @retry
//             real — NAO recarrega a pagina (perderia o escopo do editor).
//  - empty:   "Sem dados neste periodo." (vazio honesto; nunca mock).

withDefaults(
  defineProps<{
    title: string
    subtitle?: string
    loading?: boolean
    error?: string
    empty?: boolean
    emptyLabel?: string
  }>(),
  {
    subtitle: '',
    loading: false,
    error: '',
    empty: false,
    emptyLabel: 'Sem dados neste periodo.',
  },
)

const emit = defineEmits<{ (e: 'retry'): void }>()
</script>

<template>
  <section class="cardapio-analytics-card" :aria-label="title">
    <header class="cardapio-analytics-card__head">
      <div class="cardapio-analytics-card__heading">
        <h3 class="cardapio-analytics-card__title">{{ title }}</h3>
        <p v-if="subtitle" class="cardapio-analytics-card__subtitle">{{ subtitle }}</p>
      </div>
      <div v-if="$slots.actions" class="cardapio-analytics-card__actions">
        <slot name="actions"></slot>
      </div>
    </header>

    <div class="cardapio-analytics-card__body">
      <CoreErrorState
        v-if="error"
        compact
        :message="error"
        retry-label="Tentar de novo"
        @retry="emit('retry')"
      />

      <div
        v-else-if="loading"
        class="cardapio-analytics-card__placeholder"
        aria-hidden="true"
      ></div>

      <p v-else-if="empty" class="cardapio-analytics-card__empty">{{ emptyLabel }}</p>

      <slot v-else></slot>
    </div>
  </section>
</template>

<style scoped>
.cardapio-analytics-card {
  display: flex;
  flex-direction: column;
  gap: 0.9rem;
  padding: 1.25rem 1.4rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface) / 0.7);
  box-shadow: var(--shadow-card);
  min-width: 0;
}

.cardapio-analytics-card__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.cardapio-analytics-card__heading {
  min-width: 0;
}

.cardapio-analytics-card__title {
  font-size: 1.02rem;
  font-weight: 700;
  color: var(--text-main);
}

.cardapio-analytics-card__subtitle {
  margin-top: 0.15rem;
  font-size: 0.8rem;
  color: var(--text-muted);
}

.cardapio-analytics-card__actions {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.cardapio-analytics-card__body {
  min-width: 0;
}

.cardapio-analytics-card__placeholder {
  height: 220px;
  border-radius: var(--radius-soft);
  background: rgb(var(--surface-2) / 0.5);
  animation: cardapio-analytics-card-pulse 1.2s ease-in-out infinite;
}

.cardapio-analytics-card__empty {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 120px;
  padding: 1rem;
  text-align: center;
  font-size: 0.86rem;
  color: var(--text-muted);
  border: 1px dashed var(--line-soft);
  border-radius: var(--radius-soft);
  background: rgb(var(--surface-2) / 0.4);
}

@keyframes cardapio-analytics-card-pulse {
  0%,
  100% {
    opacity: 0.5;
  }
  50% {
    opacity: 1;
  }
}
</style>
