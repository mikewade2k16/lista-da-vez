<script setup lang="ts">
// Estados honestos de uma aba: carregando, erro (com nova tentativa), vazio com
// orientacao de como gerar dados. Com dados, renderiza o slot.
defineProps<{
  loading: boolean
  error: string
  empty: boolean
  emptyTitle?: string
  emptyHint?: string
}>()

defineEmits<{ retry: [] }>()
</script>

<template>
  <div v-if="loading && empty" class="obs-state">
    <UIcon name="i-lucide-loader-circle" class="animate-spin" />
    Carregando...
  </div>
  <div v-else-if="error" class="obs-state obs-state--error">
    <UIcon name="i-lucide-alert-triangle" />
    <span>{{ error }}</span>
    <UButton
      label="Tentar de novo"
      size="xs"
      color="neutral"
      variant="soft"
      @click="$emit('retry')"
    />
  </div>
  <div v-else-if="empty" class="obs-state">
    <UIcon name="i-lucide-inbox" />
    <div>
      <strong>{{ emptyTitle || 'Sem dados neste periodo.' }}</strong>
      <p v-if="emptyHint">{{ emptyHint }}</p>
    </div>
  </div>
  <slot v-else></slot>
</template>
