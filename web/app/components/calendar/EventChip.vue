<script setup lang="ts">
import { computed } from 'vue'
import {
  NEUTRAL_COLOR,
  eventTypeMeta,
  hexToTriplet,
  isHexColor,
  rgba,
  type CalendarClient,
  type CalendarEvent,
} from '~/utils/calendar'

const props = defineProps<{
  event: CalendarEvent
  client?: CalendarClient
  /** Override de cor por tipo (config `typeColors[type]` = `#rrggbb`); vazio = cor do cliente. */
  typeColor?: string
}>()

defineEmits<{ select: [event: CalendarEvent] }>()

const typeIcon = computed(() => eventTypeMeta(props.event.type).icon)

// WAVE 5 (E3): evento-espelho nascido de uma task (source='task') ganha um marcador visual
// distinto (icone de task + borda tracejada) para diferenciar do evento manual.
const isMirror = computed(() => props.event.source === 'task')

// WAVE 11: item ESPECIAL de midia (source='media', upload avulso que virou tarefa) NAO mostra
// o titulo na visao geral do calendario — so um icone de midia (o fundo do dia ja exibe a arte;
// o titulo = nome do arquivo aparece na task e no drawer).
const isMediaItem = computed(() => props.event.source === 'media')

const chipStyle = computed(() => {
  // A cor do tipo (config) vence a do cliente quando setada; senao cor do cliente;
  // senao cinza neutro. Cor e DADO -> aplicada via rgba() no ponto de uso.
  const override = props.typeColor
  const color =
    override && isHexColor(override)
      ? hexToTriplet(override)
      : (props.client?.color ?? NEUTRAL_COLOR)
  return {
    background: rgba(color, 0.14),
    borderLeftColor: rgba(color, 1),
    color: rgba(color, 1),
  }
})
</script>

<template>
  <button
    type="button"
    class="calendar-chip"
    :class="{ 'calendar-chip--mirror': isMirror, 'calendar-chip--media': isMediaItem }"
    :style="chipStyle"
    :title="`${event.time ? event.time + ' · ' : ''}${event.title}${isMirror ? ' (espelho de task)' : ''}`"
    @click.stop="$emit('select', event)"
  >
    <UIcon
      :name="isMediaItem ? 'i-lucide-image' : typeIcon"
      class="calendar-chip__icon"
      aria-hidden="true"
    />
    <span v-if="!isMediaItem" class="calendar-chip__title">{{ event.title }}</span>
    <UIcon
      v-if="isMirror"
      name="i-lucide-square-check-big"
      class="calendar-chip__mirror"
      aria-hidden="true"
    />
  </button>
</template>

<style scoped>
/* Evento-espelho de task (WAVE 5): borda tracejada + iconezinho de task ao fim do chip. */
.calendar-chip--mirror {
  border-left-style: dashed;
}

.calendar-chip__mirror {
  margin-left: auto;
  width: 0.8rem;
  height: 0.8rem;
  flex: 0 0 auto;
  opacity: 0.7;
}
</style>
