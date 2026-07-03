<script setup lang="ts">
import { computed } from 'vue'
import type { CalendarMember } from '~/utils/calendar'

// Secao Responsaveis: quais usuarios da conta aparecem na lista de responsaveis.
// Nenhum marcado = todos. Migrado do antigo CalendarConfigModal (SPEC-F3).
const props = defineProps<{
  modelValue: string[]
  members: CalendarMember[]
}>()

const emit = defineEmits<{ 'update:modelValue': [ids: string[]] }>()

const selected = computed(() => new Set(props.modelValue || []))

function toggle(id: string): void {
  const next = new Set(selected.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  emit('update:modelValue', [...next])
}
</script>

<template>
  <section class="calendar-config__section">
    <h3 class="calendar-config__section-title">Responsáveis</h3>
    <p class="calendar-config__hint">
      Quais usuários da conta aparecem na lista de responsáveis. Nenhum marcado = todos.
    </p>
    <div v-if="members.length" class="calendar-config__members">
      <label v-for="member in members" :key="member.id" class="calendar-config__check">
        <input type="checkbox" :checked="selected.has(member.id)" @change="toggle(member.id)" />
        <span>{{ member.name }}</span>
      </label>
    </div>
    <p v-else class="calendar-config__empty">Nenhum usuário na conta.</p>
  </section>
</template>
