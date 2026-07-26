<script setup lang="ts">
import { computed } from 'vue'

import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import {
  attendanceTranscriptionStatusLabel,
  formatAttendanceAudioSize,
  formatAttendanceTranscriptionDate,
  type AttendanceTranscription,
} from '~/domain/operation/attendance-transcriptions'

const props = defineProps<{
  item: AttendanceTranscription
  audioLoading?: boolean
  transcribing?: boolean
  analyzing?: boolean
}>()

const emit = defineEmits<{
  details: [item: AttendanceTranscription]
  listen: [item: AttendanceTranscription]
  transcribe: [item: AttendanceTranscription]
  analyze: [item: AttendanceTranscription]
}>()

const statusTone = computed(() => {
  if (props.item.transcriptionStatus === 'failed' || props.item.recordingStatus === 'failed') {
    return 'error'
  }
  if (
    props.item.recordingStatus === 'recording' ||
    props.item.transcriptionStatus === 'processing' ||
    (props.item.transcriptionRequested && props.item.transcriptionStatus === 'pending')
  ) {
    return 'working'
  }
  return 'ready'
})

const compactMessage = computed(() => {
  if (props.item.summaryText) return props.item.summaryText
  if (props.item.transcriptLive) {
    return 'A transcrição quase ao vivo já começou. Abra os detalhes para acompanhar.'
  }
  if (props.item.analysisError) return props.item.analysisError
  if (props.item.transcriptionStatus === 'completed') {
    return 'Transcrição concluída. O resumo está sendo preparado.'
  }
  if (props.item.recordingStatus === 'recording') {
    return 'Recebendo áudio em blocos seguros de 5 segundos.'
  }
  return 'Áudio aguardando processamento.'
})

const canRequestTranscription = computed(
  () =>
    props.item.hasAudio &&
    props.item.transcriptionStatus !== 'completed' &&
    !(props.item.transcriptionRequested && props.item.transcriptionStatus === 'pending'),
)

const canRequestAnalysis = computed(
  () =>
    props.item.transcriptionStatus === 'completed' &&
    props.item.analysisStatus !== 'completed' &&
    props.item.analysisStatus !== 'pending',
)
</script>

<template>
  <article class="transcription-mini-card">
    <header class="transcription-mini-card__header">
      <div>
        <strong>{{ item.consultantName }}</strong>
        <span>{{ formatAttendanceTranscriptionDate(item.startedAt) }}</span>
      </div>
      <span class="transcription-mini-card__status" :data-tone="statusTone">
        {{ item.transcriptLive ? 'Ao vivo' : attendanceTranscriptionStatusLabel(item) }}
      </span>
    </header>

    <p class="transcription-mini-card__summary">{{ compactMessage }}</p>

    <div class="transcription-mini-card__meta">
      <span>{{ item.chunkCount }} blocos</span>
      <span>{{ formatAttendanceAudioSize(item.sizeBytes) }}</span>
      <span v-if="item.finishOutcome">{{ item.finishOutcome }}</span>
    </div>

    <footer class="transcription-mini-card__actions">
      <AppPanelButton variant="secondary" @click="emit('details', item)">
        <UIcon name="i-lucide-file-text" aria-hidden="true" />
        Detalhes
      </AppPanelButton>
      <AppPanelButton
        variant="secondary"
        :disabled="!item.hasAudio || audioLoading"
        @click="emit('listen', item)"
      >
        <UIcon name="i-lucide-play" aria-hidden="true" />
        {{ audioLoading ? 'Carregando' : 'Ouvir' }}
      </AppPanelButton>
      <AppPanelButton
        v-if="canRequestTranscription"
        variant="secondary"
        :disabled="transcribing || item.transcriptionStatus === 'processing'"
        @click="emit('transcribe', item)"
      >
        <UIcon name="i-lucide-audio-lines" aria-hidden="true" />
        {{ transcribing ? 'Enfileirando' : 'Transcrever' }}
      </AppPanelButton>
      <AppPanelButton
        v-if="canRequestAnalysis"
        variant="secondary"
        :disabled="analyzing || item.analysisStatus === 'processing'"
        @click="emit('analyze', item)"
      >
        <UIcon name="i-lucide-sparkles" aria-hidden="true" />
        {{ analyzing ? 'Enfileirando' : 'Resumir' }}
      </AppPanelButton>
    </footer>
  </article>
</template>

<style scoped>
.transcription-mini-card {
  display: grid;
  min-width: 0;
  gap: 0.65rem;
  padding: 0.75rem;
  border: 1px solid rgb(var(--border));
  border-radius: 12px;
  background: rgb(var(--surface));
}

.transcription-mini-card__header,
.transcription-mini-card__meta,
.transcription-mini-card__actions {
  display: flex;
  align-items: center;
}

.transcription-mini-card__actions :deep(.app-panel-button) {
  min-height: 1.9rem;
  gap: 0.4rem;
  padding: 0 0.55rem;
  border: 1px solid rgb(var(--border));
  border-radius: 8px;
  background: rgb(var(--surface-2));
  color: var(--text-main);
  font-size: 0.68rem;
  cursor: pointer;
  box-shadow: var(--shadow-xs);
  transition:
    color 140ms ease,
    border-color 140ms ease,
    background 140ms ease,
    box-shadow 140ms ease,
    transform 100ms ease;
}

.transcription-mini-card__actions :deep(.app-panel-button:hover:not(.is-disabled)) {
  border-color: rgb(var(--primary) / 0.55);
  background: rgb(var(--primary) / 0.12);
  color: rgb(var(--primary));
  box-shadow: var(--shadow-sm);
  transform: translateY(-1px);
}

.transcription-mini-card__actions :deep(.app-panel-button:active:not(.is-disabled)) {
  box-shadow: none;
  transform: translateY(1px);
}

.transcription-mini-card__actions :deep(.app-panel-button:focus-visible) {
  outline: 2px solid rgb(var(--primary) / 0.75);
  outline-offset: 2px;
}

.transcription-mini-card__actions :deep(.app-panel-button.is-disabled) {
  cursor: not-allowed;
  box-shadow: none;
}

.transcription-mini-card__header {
  justify-content: space-between;
  gap: 0.5rem;
}

.transcription-mini-card__header > div {
  display: grid;
  min-width: 0;
}

.transcription-mini-card__header strong {
  overflow: hidden;
  color: var(--text-main);
  font-size: 0.82rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.transcription-mini-card__header div span,
.transcription-mini-card__meta {
  color: rgb(var(--muted));
  font-size: 0.68rem;
}

.transcription-mini-card__status {
  flex: 0 0 auto;
  max-width: 8.5rem;
  overflow: hidden;
  padding: 0.2rem 0.45rem;
  border-radius: 999px;
  background: rgb(var(--surface-2));
  color: rgb(var(--muted));
  font-size: 0.62rem;
  font-weight: 800;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.transcription-mini-card__status[data-tone='ready'] {
  background: rgb(var(--success) / 0.12);
  color: rgb(var(--success));
}

.transcription-mini-card__status[data-tone='working'] {
  background: rgb(var(--primary) / 0.12);
  color: rgb(var(--primary));
}

.transcription-mini-card__status[data-tone='error'] {
  background: rgb(var(--danger) / 0.12);
  color: rgb(var(--danger));
}

.transcription-mini-card__summary {
  display: -webkit-box;
  min-height: 2.6rem;
  margin: 0;
  overflow: hidden;
  color: var(--text-main);
  font-size: 0.76rem;
  line-height: 1.35;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 3;
}

.transcription-mini-card__meta {
  flex-wrap: wrap;
  gap: 0.35rem 0.65rem;
}

.transcription-mini-card__actions {
  flex-wrap: wrap;
  gap: 0.25rem;
  padding-top: 0.1rem;
  border-top: 1px solid rgb(var(--border) / 0.65);
}
</style>
