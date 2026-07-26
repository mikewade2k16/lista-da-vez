<script setup lang="ts">
import { ref } from 'vue'

import AttendanceTranscriptionCard from './AttendanceTranscriptionCard.vue'
import type {
  AttendanceTranscription,
  AttendanceTranscriptionStoreGroup,
} from '~/domain/operation/attendance-transcriptions'

const props = defineProps<{
  group: AttendanceTranscriptionStoreGroup
  initiallyOpen?: boolean
  audioLoadingId?: string
  transcribingId?: string
  analyzingId?: string
}>()

const emit = defineEmits<{
  details: [item: AttendanceTranscription]
  listen: [item: AttendanceTranscription]
  transcribe: [item: AttendanceTranscription]
  analyze: [item: AttendanceTranscription]
}>()

const open = ref(Boolean(props.initiallyOpen))

function onToggle(event: Event) {
  open.value = (event.currentTarget as HTMLDetailsElement).open
}
</script>

<template>
  <details class="transcription-store" :open="open" @toggle="onToggle">
    <summary class="transcription-store__summary">
      <span class="transcription-store__identity">
        <UIcon name="i-lucide-store" aria-hidden="true" />
        <strong>{{ group.name }}</strong>
        <span>{{ group.items.length }} atendimento{{ group.items.length === 1 ? '' : 's' }}</span>
      </span>
      <UIcon class="transcription-store__chevron" name="i-lucide-chevron-down" aria-hidden="true" />
    </summary>

    <div class="transcription-store__grid">
      <AttendanceTranscriptionCard
        v-for="item in group.items"
        :key="item.id"
        :item="item"
        :audio-loading="audioLoadingId === item.id"
        :transcribing="transcribingId === item.id"
        :analyzing="analyzingId === item.id"
        @details="emit('details', $event)"
        @listen="emit('listen', $event)"
        @transcribe="emit('transcribe', $event)"
        @analyze="emit('analyze', $event)"
      />
    </div>
  </details>
</template>

<style scoped>
.transcription-store {
  overflow: clip;
  border: 1px solid rgb(var(--border));
  border-radius: 12px;
  background: rgb(var(--surface) / 0.5);
}

.transcription-store__summary {
  display: flex;
  min-height: 2.75rem;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0.65rem 0.8rem;
  cursor: pointer;
  list-style: none;
}

.transcription-store__summary::-webkit-details-marker {
  display: none;
}

.transcription-store__identity {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.45rem;
}

.transcription-store__identity > svg {
  flex: 0 0 auto;
  color: rgb(var(--primary));
}

.transcription-store__identity strong {
  overflow: hidden;
  color: var(--text-main);
  font-size: 0.83rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.transcription-store__identity span {
  color: rgb(var(--muted));
  font-size: 0.7rem;
}

.transcription-store__chevron {
  color: rgb(var(--muted));
  transition: transform 160ms ease;
}

.transcription-store[open] .transcription-store__chevron {
  transform: rotate(180deg);
}

.transcription-store__grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(100%, 16rem), 1fr));
  gap: 0.6rem;
  padding: 0 0.65rem 0.65rem;
}
</style>
