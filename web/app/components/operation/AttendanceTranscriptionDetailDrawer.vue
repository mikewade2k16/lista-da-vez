<script setup lang="ts">
import { computed, ref } from 'vue'

import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import OmniEntityDrawer from '~/components/ui/OmniEntityDrawer.vue'
import {
  attendanceTranscriptionStatusLabel,
  formatAttendanceAudioSize,
  formatAttendanceTranscriptionDate,
  type AttendanceTranscription,
} from '~/domain/operation/attendance-transcriptions'

const props = defineProps<{
  open: boolean
  item: AttendanceTranscription | null
  audioLoading?: boolean
  audioUrl?: string
  transcribing?: boolean
  analyzing?: boolean
}>()

const emit = defineEmits<{
  'update:open': [value: boolean]
  'load-audio': [item: AttendanceTranscription]
  transcribe: [item: AttendanceTranscription]
  analyze: [item: AttendanceTranscription]
}>()

const drawerMode = ref<'side' | 'center' | 'fullscreen'>('center')
const openModel = computed({
  get: () => props.open,
  set: (value: boolean) => emit('update:open', value),
})

const reportRows = computed(() => {
  const report = props.item?.analysisReport
  if (!report) return []
  return [
    { label: 'Intenção', value: report.customerIntent },
    { label: 'Necessidades', value: report.needs?.join(' · ') },
    { label: 'Produtos', value: report.products?.join(' · ') },
    { label: 'Objeções', value: report.objections?.join(' · ') },
    { label: 'Compromissos', value: report.commitments?.join(' · ') },
    { label: 'Próximos passos', value: report.nextSteps?.join(' · ') },
    { label: 'Oportunidades', value: report.opportunities?.join(' · ') },
    { label: 'Alertas', value: report.alerts?.join(' · ') },
    { label: 'Sentimento', value: report.sentiment },
  ].filter((row) => Boolean(row.value))
})

const canRequestTranscription = computed(
  () =>
    Boolean(props.item?.hasAudio) &&
    props.item?.transcriptionStatus !== 'completed' &&
    !(props.item?.transcriptionRequested && props.item?.transcriptionStatus === 'pending'),
)

const canRequestAnalysis = computed(
  () =>
    props.item?.transcriptionStatus === 'completed' &&
    props.item?.analysisStatus !== 'completed' &&
    props.item?.analysisStatus !== 'pending',
)
</script>

<template>
  <OmniEntityDrawer
    v-model="openModel"
    v-model:mode="drawerMode"
    :title="item ? `Atendimento de ${item.consultantName}` : 'Atendimento'"
    :subtitle="
      item ? `${item.storeName} · ${formatAttendanceTranscriptionDate(item.startedAt)}` : ''
    "
  >
    <div v-if="item" class="transcription-detail">
      <section class="transcription-detail__audio">
        <header>
          <div>
            <strong>Áudio completo</strong>
            <span>
              {{ item.chunkCount }} blocos · {{ formatAttendanceAudioSize(item.sizeBytes) }}
            </span>
          </div>
          <AppPanelButton
            v-if="!audioUrl"
            variant="secondary"
            :disabled="!item.hasAudio || audioLoading"
            @click="emit('load-audio', item)"
          >
            <UIcon name="i-lucide-play" aria-hidden="true" />
            {{ audioLoading ? 'Carregando áudio' : 'Carregar áudio' }}
          </AppPanelButton>
        </header>
        <audio
          v-if="audioUrl"
          class="transcription-detail__player"
          controls
          autoplay
          preload="metadata"
          :src="audioUrl"
        ></audio>
        <p v-else-if="!item.hasAudio">
          O áudio completo ficará disponível assim que a gravação for encerrada.
        </p>
      </section>

      <section class="transcription-detail__section">
        <header class="transcription-detail__section-heading">
          <strong>Resumo</strong>
          <span>{{ item.analysisStatus === 'completed' ? 'Concluído' : 'Em preparação' }}</span>
        </header>
        <p v-if="item.summaryText" class="transcription-detail__summary">
          {{ item.summaryText }}
        </p>
        <p v-else class="transcription-detail__muted">
          O resumo será gerado depois da transcrição definitiva.
        </p>
        <dl v-if="reportRows.length" class="transcription-detail__report">
          <div v-for="row in reportRows" :key="row.label">
            <dt>{{ row.label }}</dt>
            <dd>{{ row.value }}</dd>
          </div>
        </dl>
      </section>

      <section class="transcription-detail__section">
        <header class="transcription-detail__section-heading">
          <strong>Transcrição completa</strong>
          <span :data-live="item.transcriptLive">
            {{ item.transcriptLive ? 'Quase ao vivo' : attendanceTranscriptionStatusLabel(item) }}
          </span>
        </header>
        <div v-if="item.transcriptText" class="transcription-detail__transcript">
          {{ item.transcriptText }}
        </div>
        <p v-else-if="item.transcriptError" class="transcription-detail__error">
          {{ item.transcriptError }}
        </p>
        <p v-else class="transcription-detail__muted">
          {{
            item.recordingStatus === 'recording'
              ? 'A primeira janela aparecerá após cerca de 25 segundos de áudio.'
              : 'Nenhuma fala foi identificada até agora.'
          }}
        </p>
      </section>
    </div>

    <template #footer>
      <div v-if="item" class="transcription-detail__footer">
        <span>Atendimento {{ item.serviceId }}</span>
        <div>
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
            variant="primary"
            :disabled="analyzing || item.analysisStatus === 'processing'"
            @click="emit('analyze', item)"
          >
            <UIcon name="i-lucide-sparkles" aria-hidden="true" />
            {{ analyzing ? 'Enfileirando' : 'Gerar resumo' }}
          </AppPanelButton>
        </div>
      </div>
    </template>
  </OmniEntityDrawer>
</template>

<style scoped>
.transcription-detail {
  display: grid;
  gap: 0.75rem;
  padding: 0.85rem;
}

.transcription-detail__audio,
.transcription-detail__section {
  padding: 0.85rem;
  border: 1px solid rgb(var(--border));
  border-radius: 12px;
  background: rgb(var(--surface));
}

.transcription-detail__audio header,
.transcription-detail__audio header > div,
.transcription-detail__section-heading {
  display: flex;
}

.transcription-detail__audio header,
.transcription-detail__section-heading {
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.transcription-detail__audio header > div {
  min-width: 0;
  flex-direction: column;
}

.transcription-detail__audio strong,
.transcription-detail__section-heading strong {
  color: var(--text-main);
  font-size: 0.82rem;
}

.transcription-detail__audio span,
.transcription-detail__audio p,
.transcription-detail__muted,
.transcription-detail__section-heading span {
  color: rgb(var(--muted));
  font-size: 0.72rem;
}

.transcription-detail__section-heading span[data-live='true'] {
  color: rgb(var(--primary));
  font-weight: 800;
}

.transcription-detail__player {
  width: 100%;
  margin-top: 0.75rem;
}

.transcription-detail__summary {
  margin: 0.65rem 0 0;
  color: var(--text-main);
  font-size: 0.82rem;
  line-height: 1.55;
}

.transcription-detail__report {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.5rem;
  margin: 0.75rem 0 0;
}

.transcription-detail__report div {
  padding: 0.6rem;
  border-radius: 9px;
  background: rgb(var(--surface-2) / 0.65);
}

.transcription-detail__report dt {
  color: rgb(var(--muted));
  font-size: 0.64rem;
  font-weight: 800;
  text-transform: uppercase;
}

.transcription-detail__report dd {
  margin: 0.2rem 0 0;
  color: var(--text-main);
  font-size: 0.76rem;
}

.transcription-detail__transcript {
  max-height: 48vh;
  margin-top: 0.65rem;
  overflow-y: auto;
  color: var(--text-main);
  font-size: 0.82rem;
  line-height: 1.6;
  white-space: pre-wrap;
}

.transcription-detail__error {
  color: rgb(var(--danger));
}

.transcription-detail__footer,
.transcription-detail__footer > div {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.transcription-detail__audio :deep(.app-panel-button),
.transcription-detail__footer :deep(.app-panel-button) {
  gap: 0.42rem;
}

.transcription-detail__footer {
  width: 100%;
  justify-content: space-between;
}

.transcription-detail__footer > span {
  overflow: hidden;
  color: rgb(var(--muted));
  font-size: 0.68rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

@media (max-width: 640px) {
  .transcription-detail__report {
    grid-template-columns: 1fr;
  }
}
</style>
