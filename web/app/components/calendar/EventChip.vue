<script setup lang="ts">
import { computed } from 'vue'
import {
  EVENT_TYPE_META,
  NEUTRAL_COLOR,
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

const typeIcon = computed(() => EVENT_TYPE_META[props.event.type].icon)

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
    :style="chipStyle"
    :title="`${event.time ? event.time + ' · ' : ''}${event.title}`"
    @click.stop="$emit('select', event)"
  >
    <UIcon :name="typeIcon" class="calendar-chip__icon" aria-hidden="true" />
    <span class="calendar-chip__title">{{ event.title }}</span>
  </button>
</template>
