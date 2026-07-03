<script setup lang="ts">
import { computed } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import type { CalendarClient, CalendarView } from '~/utils/calendar'

const props = defineProps<{
  periodTitle: string
  clients: CalendarClient[]
  selectedClientId: string
  view: CalendarView
}>()

const emit = defineEmits<{
  today: []
  'update:client': [clientId: string]
  'update:view': [view: CalendarView]
  'new-item': []
  config: []
  ai: []
}>()

const clientOptions = computed(() => [
  { value: '', label: 'Todos os clientes' },
  ...props.clients.map((client) => ({ value: client.id, label: client.name })),
])
</script>

<template>
  <div class="calendar-controls">
    <h2 class="calendar-controls__title">{{ periodTitle }}</h2>

    <button
      type="button"
      class="calendar-controls__gear"
      aria-label="IA do mês"
      title="Gerar plano de conteúdo com IA"
      @click="emit('ai')"
    >
      <UIcon name="i-lucide-sparkles" aria-hidden="true" />
    </button>

    <button
      type="button"
      class="calendar-controls__gear"
      aria-label="Configurações do calendário"
      title="Configurações (responsáveis, feriados)"
      @click="emit('config')"
    >
      <UIcon name="i-lucide-settings" aria-hidden="true" />
    </button>

    <AppSelectField
      class="calendar-controls__client"
      :model-value="selectedClientId"
      :options="clientOptions"
      placeholder="Todos os clientes"
      :show-leading-icon="false"
      compact
      @update:model-value="emit('update:client', $event)"
    />

    <div class="calendar-controls__seg" role="tablist" aria-label="Visão do calendário">
      <button
        type="button"
        class="calendar-controls__seg-btn"
        :class="{ 'is-active': view === 'month' }"
        role="tab"
        :aria-selected="view === 'month' ? 'true' : 'false'"
        @click="emit('update:view', 'month')"
      >
        Mês
      </button>
      <button
        type="button"
        class="calendar-controls__seg-btn"
        :class="{ 'is-active': view === 'week' }"
        role="tab"
        :aria-selected="view === 'week' ? 'true' : 'false'"
        @click="emit('update:view', 'week')"
      >
        Semana
      </button>
    </div>

    <AppPanelButton variant="ghost" class="calendar-controls__today" @click="emit('today')">
      Hoje
    </AppPanelButton>
    <AppPanelButton variant="primary" class="calendar-controls__new" @click="emit('new-item')">
      <UIcon name="i-lucide-plus" class="calendar-controls__new-icon" aria-hidden="true" />
      Novo
    </AppPanelButton>
  </div>
</template>
