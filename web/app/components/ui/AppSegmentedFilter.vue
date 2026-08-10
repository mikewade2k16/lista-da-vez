<script setup lang="ts">
// Controle segmentado (pills) generico para filtro de visao. Cada opcao tem um
// `value` NAO-VAZIO (regra do projeto: nunca usar value=""; usar sentinela tipo
// 'all'). O consumidor converte a sentinela na borda. Emite `update:modelValue`
// com o value da opcao clicada. Opcionalmente mostra a contagem de itens por
// opcao (badge) para o usuario saber quantos caem em cada filtro antes de clicar.

interface SegmentOption {
  value: string
  label: string
  // Contagem opcional exibida como badge ao lado do label.
  count?: number
}

defineProps<{
  modelValue: string
  options: SegmentOption[]
  ariaLabel?: string
  disabled?: boolean
}>()

const emit = defineEmits<{ 'update:modelValue': [string] }>()
</script>

<template>
  <div class="app-segmented" role="group" :aria-label="ariaLabel">
    <button
      v-for="option in options"
      :key="option.value"
      type="button"
      class="app-segmented__btn"
      :class="{ 'app-segmented__btn--active': modelValue === option.value }"
      :aria-pressed="modelValue === option.value"
      :disabled="disabled"
      @click="emit('update:modelValue', option.value)"
    >
      <span class="app-segmented__label">{{ option.label }}</span>
      <span v-if="typeof option.count === 'number'" class="app-segmented__count">
        {{ option.count }}
      </span>
    </button>
  </div>
</template>

<style scoped>
.app-segmented {
  display: inline-flex;
  align-items: center;
  min-height: 2rem;
  padding: 0.15rem;
  overflow-x: auto;
  border-radius: 999px;
  border: 1px solid rgb(var(--ring) / 0.14);
  background: rgb(var(--surface-2) / 0.72);
}

.app-segmented__btn {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  min-height: 1.7rem;
  padding: 0 0.55rem;
  border: 0;
  border-radius: 999px;
  background: transparent;
  color: rgb(var(--muted));
  font-size: 0.68rem;
  font-weight: 800;
  white-space: nowrap;
  cursor: pointer;
}

.app-segmented__btn:hover {
  color: rgb(var(--text));
}

.app-segmented__btn--active {
  background: rgb(var(--primary) / 0.16);
  color: rgb(var(--primary));
}

.app-segmented__btn:disabled {
  cursor: not-allowed;
  opacity: 0.45;
}

.app-segmented__count {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 1.1rem;
  padding: 0 0.3rem;
  border-radius: 999px;
  background: rgb(var(--border) / 0.55);
  color: rgb(var(--muted));
  font-size: 0.66rem;
  font-weight: 700;
}

.app-segmented__btn--active .app-segmented__count {
  background: rgb(var(--primary) / 0.16);
  color: rgb(var(--primary-600));
}
</style>
