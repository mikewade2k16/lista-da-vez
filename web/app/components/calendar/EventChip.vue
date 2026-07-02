<script setup lang="ts">
import { computed } from 'vue'
import { EVENT_TYPE_META, rgba, type CalendarClient, type CalendarEvent } from '~/utils/calendar'

const props = defineProps<{
  event: CalendarEvent
  client?: CalendarClient
}>()

defineEmits<{ select: [event: CalendarEvent] }>()

const typeIcon = computed(() => EVENT_TYPE_META[props.event.type].icon)

const chipStyle = computed(() => {
  const color = props.client?.color ?? ([148, 163, 184] as const)
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
