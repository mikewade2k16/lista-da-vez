<script setup lang="ts">
import { computed } from 'vue'
import { CalendarDays, RefreshCw } from 'lucide-vue-next'

import type { AnalyticsRange } from '~/domain/cardapio/analytics'

// Barra de periodo do dashboard (F4). AppDatePicker em modo range (model-value =
// inicio ISO, end-date = fim ISO) + chips de preset (Hoje/7d/30d/Mes) + botao
// Atualizar. NAO faz fetch: emite update:range (novo {from,to}) e refresh; o
// orquestrador repassa ao composable. Default do periodo (7 dias) e definido pelo
// pai e refletido aqui (sem estado proprio — fonte unica no pai).

const props = defineProps<{ range: AnalyticsRange; loading?: boolean }>()

const emit = defineEmits<{
  'update:range': [value: AnalyticsRange]
  refresh: []
}>()

// --- Helpers de data (YYYY-MM-DD, local) ---
function toIso(date: Date): string {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

function daysAgo(count: number): Date {
  const date = new Date()
  date.setDate(date.getDate() - count)
  return date
}

function startOfMonth(): Date {
  const now = new Date()
  return new Date(now.getFullYear(), now.getMonth(), 1)
}

type PresetKey = 'today' | '7d' | '30d' | 'month'

const PRESETS: Array<{ key: PresetKey; label: string }> = [
  { key: 'today', label: 'Hoje' },
  { key: '7d', label: '7 dias' },
  { key: '30d', label: '30 dias' },
  { key: 'month', label: 'Mes' },
]

function rangeForPreset(key: PresetKey): AnalyticsRange {
  const today = new Date()
  switch (key) {
    case 'today':
      return { from: toIso(today), to: toIso(today) }
    case '7d':
      return { from: toIso(daysAgo(6)), to: toIso(today) }
    case '30d':
      return { from: toIso(daysAgo(29)), to: toIso(today) }
    case 'month':
      return { from: toIso(startOfMonth()), to: toIso(today) }
    default:
      return { from: toIso(daysAgo(6)), to: toIso(today) }
  }
}

// Marca o chip ativo quando o range atual bate com o preset (comparacao de strings).
const activePreset = computed<PresetKey | ''>(() => {
  for (const preset of PRESETS) {
    const candidate = rangeForPreset(preset.key)
    if (candidate.from === props.range.from && candidate.to === props.range.to) {
      return preset.key
    }
  }
  return ''
})

function applyPreset(key: PresetKey) {
  emit('update:range', rangeForPreset(key))
}

// AppDatePicker emite ISO YYYY-MM-DD (slice de 10) em update:model-value/endDate.
// Mantem o outro lado do range; so atualiza quando o valor existe (limpar = no-op
// para nunca mandar from/to vazio ao back).
function onStartChange(value: string) {
  const from = String(value || '').slice(0, 10)
  if (!from) {
    return
  }
  emit('update:range', { from, to: props.range.to })
}

function onEndChange(value: string) {
  const to = String(value || '').slice(0, 10)
  if (!to) {
    return
  }
  emit('update:range', { from: props.range.from, to })
}
</script>

<template>
  <div class="cardapio-analytics-toolbar">
    <div class="cardapio-analytics-toolbar__period">
      <AppDatePicker
        :model-value="range.from"
        :end-date="range.to"
        @update:model-value="onStartChange"
        @update:end-date="onEndChange"
      >
        <template #default="{ label }">
          <button type="button" class="cardapio-analytics-toolbar__date">
            <CalendarDays :size="14" aria-hidden="true" />
            <span>{{ label || 'Selecionar periodo' }}</span>
          </button>
        </template>
      </AppDatePicker>
    </div>

    <div class="cardapio-analytics-toolbar__presets" role="group" aria-label="Atalhos de periodo">
      <button
        v-for="preset in PRESETS"
        :key="preset.key"
        type="button"
        class="cardapio-analytics-toolbar__chip"
        :class="{ 'cardapio-analytics-toolbar__chip--active': activePreset === preset.key }"
        @click="applyPreset(preset.key)"
      >
        {{ preset.label }}
      </button>
    </div>

    <button
      type="button"
      class="cardapio-analytics-toolbar__refresh"
      :disabled="loading"
      @click="emit('refresh')"
    >
      <RefreshCw
        :size="14"
        aria-hidden="true"
        :class="{ 'cardapio-analytics-toolbar__refresh-icon--spin': loading }"
      />
      <span>{{ loading ? 'Atualizando...' : 'Atualizar' }}</span>
    </button>
  </div>
</template>

<style scoped>
.cardapio-analytics-toolbar {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.cardapio-analytics-toolbar__period {
  min-width: 0;
}

.cardapio-analytics-toolbar__date {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  min-height: 2.4rem;
  padding: 0 0.85rem;
  border-radius: var(--radius-sm);
  border: 1px solid var(--line-soft);
  background: rgb(var(--surface) / 0.9);
  color: var(--text-main);
  font-size: 0.86rem;
  font-weight: 600;
  cursor: pointer;
  white-space: nowrap;
}

.cardapio-analytics-toolbar__presets {
  display: flex;
  gap: 0.35rem;
  flex-wrap: wrap;
}

.cardapio-analytics-toolbar__chip {
  min-height: 2.4rem;
  padding: 0 0.7rem;
  border-radius: var(--radius-sm);
  border: 1px solid var(--line-soft);
  background: transparent;
  color: var(--text-muted);
  font-size: 0.82rem;
  font-weight: 600;
  cursor: pointer;
  transition:
    background 0.13s ease,
    color 0.13s ease,
    border-color 0.13s ease;
}

.cardapio-analytics-toolbar__chip:hover {
  background: rgb(var(--surface-2) / 0.6);
  color: var(--text-main);
}

.cardapio-analytics-toolbar__chip--active {
  border-color: rgb(var(--primary) / 0.55);
  background: rgb(var(--primary) / 0.15);
  color: rgb(var(--primary));
}

.cardapio-analytics-toolbar__refresh {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  min-height: 2.4rem;
  margin-left: auto;
  padding: 0 0.95rem;
  border-radius: var(--radius-sm);
  border: none;
  background: rgb(var(--primary));
  color: rgb(255 255 255);
  font-size: 0.85rem;
  font-weight: 700;
  cursor: pointer;
}

.cardapio-analytics-toolbar__refresh:disabled {
  opacity: 0.7;
  cursor: wait;
}

.cardapio-analytics-toolbar__refresh-icon--spin {
  animation: cardapio-analytics-toolbar-spin 0.8s linear infinite;
}

@keyframes cardapio-analytics-toolbar-spin {
  to {
    transform: rotate(360deg);
  }
}

@media (max-width: 880px) {
  .cardapio-analytics-toolbar__refresh {
    margin-left: 0;
  }
}
</style>
