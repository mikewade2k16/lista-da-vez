<script setup lang="ts">
// Resultado do plano de IA (contrato C4.content): summary + pilares + por cliente
// os dias (data, tipo, ideia, copy). Componente presentational; as acoes (aplicar
// nas notas / criar eventos) sobem por emit para o modal orquestrar (mantem cada
// arquivo < 450 linhas). SPEC-F5.
import { computed } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import { EVENT_TYPE_META, planTypeToEventType, type CalendarAiPlan } from '~/utils/calendar'

const props = defineProps<{
  plan: CalendarAiPlan
  applying: boolean
}>()

const emit = defineEmits<{
  'apply-notes': []
  'create-events': []
}>()

const isApplied = computed(() => props.plan.status === 'applied')
const totalDays = computed(() =>
  props.plan.content.clients.reduce((acc, client) => acc + client.days.length, 0),
)

function typeLabel(type: string): string {
  return EVENT_TYPE_META[planTypeToEventType(type)].label
}
</script>

<template>
  <div class="calendar-ai-result">
    <p v-if="plan.content.summary" class="calendar-ai-result__summary">
      {{ plan.content.summary }}
    </p>

    <section v-if="plan.content.pillars.length" class="calendar-ai-result__block">
      <h4 class="calendar-ai-result__block-title">Pilares de conteúdo</h4>
      <ul class="calendar-ai-result__pillars">
        <li v-for="(pillar, i) in plan.content.pillars" :key="i" class="calendar-ai-result__pillar">
          <strong>{{ pillar.name }}</strong>
          <span v-if="pillar.proportion" class="calendar-ai-result__pillar-prop">
            {{ pillar.proportion }}
          </span>
          <span v-if="pillar.rationale" class="calendar-ai-result__pillar-why">
            {{ pillar.rationale }}
          </span>
        </li>
      </ul>
    </section>

    <section
      v-for="client in plan.content.clients"
      :key="client.clientId"
      class="calendar-ai-result__block"
    >
      <h4 class="calendar-ai-result__block-title">{{ client.clientName || 'Cliente' }}</h4>
      <p v-if="client.strategy" class="calendar-ai-result__strategy">{{ client.strategy }}</p>
      <ul v-if="client.days.length" class="calendar-ai-result__days">
        <li v-for="(day, i) in client.days" :key="i" class="calendar-ai-result__day">
          <span class="calendar-ai-result__day-date">{{ day.date }}</span>
          <span class="calendar-ai-result__day-type">{{ typeLabel(day.type) }}</span>
          <span class="calendar-ai-result__day-idea">{{ day.idea }}</span>
          <span v-if="day.copy" class="calendar-ai-result__day-copy">{{ day.copy }}</span>
        </li>
      </ul>
    </section>

    <div class="calendar-ai-result__actions">
      <span v-if="isApplied" class="calendar-ai-result__applied">
        <UIcon name="i-lucide-check-circle" aria-hidden="true" />
        Plano já aplicado
      </span>
      <span class="calendar-ai-result__spacer"></span>
      <AppPanelButton variant="ghost" :disabled="applying" @click="emit('apply-notes')">
        Aplicar nas notas
      </AppPanelButton>
      <AppPanelButton
        variant="primary"
        :disabled="applying || totalDays === 0"
        @click="emit('create-events')"
      >
        Criar eventos ({{ totalDays }})
      </AppPanelButton>
    </div>
  </div>
</template>
