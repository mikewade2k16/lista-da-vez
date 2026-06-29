<script setup lang="ts">
import { computed } from 'vue'

import CardapioAnalyticsCard from './CardapioAnalyticsCard.vue'
import CardapioAnalyticsDonutList from './CardapioAnalyticsDonutList.vue'
import type { AnalyticsDeviceItem, AnalyticsDevices } from '~/domain/cardapio/analytics'

// Bloco Dispositivos — mapeia `devices`. 3 donuts/listas lado a lado:
// tipo de dispositivo, navegador e sistema. Reusa CardapioAnalyticsDonutList.
// NAO faz fetch: recebe o breakdown por prop e emite retry.

const props = defineProps<{
  data: AnalyticsDevices | null
  loading?: boolean
  error?: string
}>()

const emit = defineEmits<{ (e: 'retry'): void }>()

// value vazio do back -> "(desconhecido)" (vazio honesto, nao inventa categoria).
function toSlices(items: AnalyticsDeviceItem[] | undefined) {
  return (items ?? []).map((item) => ({
    label: item.value || '(desconhecido)',
    value: item.sessions,
  }))
}

const deviceTypeSlices = computed(() => toSlices(props.data?.deviceType))
const browserSlices = computed(() => toSlices(props.data?.browser))
const osSlices = computed(() => toSlices(props.data?.os))

const isEmpty = computed(
  () =>
    deviceTypeSlices.value.length === 0 &&
    browserSlices.value.length === 0 &&
    osSlices.value.length === 0,
)
</script>

<template>
  <CardapioAnalyticsCard
    title="Dispositivos"
    subtitle="Sessoes por tipo, navegador e sistema"
    :loading="loading"
    :error="error"
    :empty="!loading && !error && isEmpty"
    @retry="emit('retry')"
  >
    <div class="cardapio-analytics-devices__grid">
      <CardapioAnalyticsDonutList title="Dispositivo" :slices="deviceTypeSlices" />
      <CardapioAnalyticsDonutList title="Navegador" :slices="browserSlices" />
      <CardapioAnalyticsDonutList title="Sistema" :slices="osSlices" />
    </div>
  </CardapioAnalyticsCard>
</template>

<style scoped>
.cardapio-analytics-devices__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 1.4rem;
}
</style>
