<script setup lang="ts">
import { computed } from 'vue'
import type { CalendarHolidayFlags } from '~/utils/calendar'

// Secao Feriados & datas comemorativas: quais conjuntos aparecem marcados no
// calendario. Migrado do antigo CalendarConfigModal (SPEC-F3).
const props = defineProps<{ modelValue: CalendarHolidayFlags }>()
const emit = defineEmits<{ 'update:modelValue': [flags: CalendarHolidayFlags] }>()

const options = [
  { key: 'brNational', label: 'Feriados nacionais (Brasil)' },
  { key: 'sergipe', label: 'Sergipe (estadual)' },
  { key: 'aracaju', label: 'Aracaju (municipal)' },
  { key: 'luxuryIntl', label: 'Internacionais (marcas de luxo)' },
] as const

const flags = computed(() => props.modelValue)

function toggle(key: keyof CalendarHolidayFlags): void {
  emit('update:modelValue', { ...flags.value, [key]: !flags.value[key] })
}
</script>

<template>
  <section class="calendar-config__section">
    <h3 class="calendar-config__section-title">Feriados & datas comemorativas</h3>
    <p class="calendar-config__hint">Quais conjuntos aparecem marcados no calendário.</p>
    <label v-for="opt in options" :key="opt.key" class="calendar-config__check">
      <input type="checkbox" :checked="flags[opt.key]" @change="toggle(opt.key)" />
      <span>{{ opt.label }}</span>
    </label>
  </section>
</template>
