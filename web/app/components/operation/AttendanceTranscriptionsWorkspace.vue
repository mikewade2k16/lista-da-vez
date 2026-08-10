<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'

import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import AppToggleSwitch from '~/components/ui/AppToggleSwitch.vue'
import AttendanceTranscriptionDetailDrawer from './AttendanceTranscriptionDetailDrawer.vue'
import AttendanceTranscriptionStoreSection from './AttendanceTranscriptionStoreSection.vue'
import AttendanceTranscriptionsConfigDrawer from './AttendanceTranscriptionsConfigDrawer.vue'
import AdminPageHeader from '../../../layers/core/components/admin/AdminPageHeader.vue'
import {
  groupAttendanceTranscriptions,
  type AttendanceTranscription,
} from '~/domain/operation/attendance-transcriptions'
import { useAuthStore } from '~/stores/auth'
import { useAttendanceTranscriptionsStore } from '~/stores/attendanceTranscriptions'
import { useAttendanceRecordingFeatureStore } from '~/stores/attendanceRecordingFeature'

const auth = useAuthStore()
const transcriptionsStore = useAttendanceTranscriptionsStore()
const recordingFeature = useAttendanceRecordingFeatureStore()
const ALL_STORES = '__all_stores__'
const ALL_CONSULTANTS = '__all_consultants__'
const {
  items,
  total,
  limit,
  offset,
  loading,
  errorMessage,
  audioLoadingId,
  transcribingId,
  analyzingId,
  config,
  configLoading,
  configSaving,
  configErrorMessage,
  transcriptionErrorMessage,
  audioErrorMessage,
  activeAudioId,
  activeAudioUrl,
} = storeToRefs(transcriptionsStore)

const configDrawerOpen = ref(false)
const detailDrawerOpen = ref(false)
const selectedRecordingId = ref('')
const selectedStoreId = ref(ALL_STORES)
const selectedConsultantId = ref(ALL_CONSULTANTS)
let pollTimer: number | null = null

const featureEnabled = computed(() => recordingFeature.loaded && recordingFeature.enabled)
const isPlatformAdmin = computed(() => auth.role === 'platform_admin')
const recordingModeStatus = computed(() => {
  if (recordingFeature.loading) return 'Carregando'
  if (recordingFeature.errorMessage) return 'Erro ao carregar'
  return featureEnabled.value ? 'Ativo' : 'Desativado'
})
const storeOptions = computed(() => [
  { value: ALL_STORES, label: 'Todas as lojas' },
  ...(auth.storeContext || []).map((store) => ({
    value: String(store.id || ''),
    label: String(store.name || store.code || 'Loja'),
  })),
])
const consultantOptions = computed(() => {
  const consultants = new Map<string, string>()
  items.value.forEach((item) => {
    if (item.consultantId) consultants.set(item.consultantId, item.consultantName)
  })
  return [
    { value: ALL_CONSULTANTS, label: 'Todos os consultores' },
    ...Array.from(consultants, ([value, label]) => ({ value, label })).sort((left, right) =>
      left.label.localeCompare(right.label, 'pt-BR'),
    ),
  ]
})
const groupedItems = computed(() => groupAttendanceTranscriptions(items.value))
const selectedItem = computed(
  () => items.value.find((item) => item.id === selectedRecordingId.value) || null,
)
const selectedAudioUrl = computed(() =>
  activeAudioId.value === selectedRecordingId.value ? activeAudioUrl.value : '',
)
const pageStart = computed(() => (total.value ? offset.value + 1 : 0))
const pageEnd = computed(() => Math.min(total.value, offset.value + items.value.length))
const hasPrevious = computed(() => offset.value > 0)
const hasNext = computed(() => offset.value + limit.value < total.value)

async function loadPage(nextOffset = 0, preserveAudio = false) {
  if (!preserveAudio) transcriptionsStore.clearAudio()
  await transcriptionsStore.load(
    selectedStoreId.value === ALL_STORES ? '' : selectedStoreId.value,
    selectedConsultantId.value === ALL_CONSULTANTS ? '' : selectedConsultantId.value,
    nextOffset,
  )
}

function openDetails(item: AttendanceTranscription) {
  selectedRecordingId.value = item.id
  detailDrawerOpen.value = true
}

async function listen(item: AttendanceTranscription) {
  openDetails(item)
  await transcriptionsStore.loadAudio(item.id)
}

async function requestTranscription(item: AttendanceTranscription) {
  if (await transcriptionsStore.requestTranscription(item.id)) {
    await loadPage(offset.value, true)
  }
}

async function requestAnalysis(item: AttendanceTranscription) {
  if (await transcriptionsStore.requestAnalysis(item.id)) {
    await loadPage(offset.value, true)
  }
}

async function saveAnalysisConfig(nextConfig: NonNullable<typeof config.value>) {
  await transcriptionsStore.saveConfig(nextConfig)
}

async function setAttendanceAudioRecording(value: boolean) {
  await recordingFeature.save(value)
}

async function reloadRecordingMode() {
  await recordingFeature.load(true)
}

function closeDetailDrawer(value: boolean) {
  detailDrawerOpen.value = value
  if (!value) {
    selectedRecordingId.value = ''
    transcriptionsStore.clearAudio()
  }
}

onMounted(async () => {
  await Promise.all([recordingFeature.load(), transcriptionsStore.loadConfig(), loadPage()])
  pollTimer = window.setInterval(() => {
    void loadPage(offset.value, true)
  }, 3_000)
})

watch(selectedStoreId, () => {
  selectedConsultantId.value = ALL_CONSULTANTS
  void loadPage()
})
watch(selectedConsultantId, () => void loadPage())

onBeforeUnmount(() => {
  if (pollTimer !== null) window.clearInterval(pollTimer)
  transcriptionsStore.clearAudio()
})
</script>

<template>
  <section class="transcriptions-workspace">
    <header class="transcriptions-workspace__header">
      <AdminPageHeader
        eyebrow="Fila de atendimento"
        title="Transcrições"
        description="Resumos compactos por loja, com áudio e transcrição completa sob demanda."
      />
    </header>

    <div class="transcriptions-workspace__toolbar">
      <div class="transcriptions-rollout">
        <div class="transcriptions-rollout__icon" aria-hidden="true">
          <UIcon :name="featureEnabled ? 'i-lucide-mic' : 'i-lucide-mic-off'" />
        </div>
        <div class="transcriptions-rollout__copy">
          <strong>Modo de gravação teste</strong>
          <span>
            {{
              isPlatformAdmin
                ? 'Controle da captura para este cliente.'
                : 'A ativação desta conta é controlada pela equipe de desenvolvimento.'
            }}
          </span>
          <small v-if="recordingFeature.errorMessage">{{ recordingFeature.errorMessage }}</small>
        </div>
      </div>
      <AppSelectField
        v-model="selectedStoreId"
        class="transcriptions-workspace__filter"
        label="Loja"
        :options="storeOptions"
      />
      <AppSelectField
        v-model="selectedConsultantId"
        class="transcriptions-workspace__filter"
        label="Consultor"
        :options="consultantOptions"
      />
      <AppPanelButton
        class="transcriptions-workspace__config-button"
        variant="ghost"
        @click="configDrawerOpen = true"
      >
        <UIcon name="i-lucide-settings-2" aria-hidden="true" />
        Configurar
      </AppPanelButton>
      <span
        class="transcriptions-rollout__status"
        :class="{ 'is-active': featureEnabled, 'is-error': recordingFeature.errorMessage }"
      >
        {{ recordingModeStatus }}
      </span>
      <AppToggleSwitch
        v-if="isPlatformAdmin"
        :model-value="featureEnabled"
        :disabled="
          !recordingFeature.loaded ||
          recordingFeature.loading ||
          recordingFeature.saving ||
          Boolean(recordingFeature.errorMessage)
        "
        compact
        label="Ativar gravação"
        @update:model-value="setAttendanceAudioRecording"
      />
      <AppPanelButton
        v-if="recordingFeature.errorMessage"
        variant="ghost"
        :disabled="recordingFeature.loading"
        @click="reloadRecordingMode"
      >
        Tentar novamente
      </AppPanelButton>
    </div>

    <div v-if="loading && items.length === 0" class="transcriptions-state">
      <UIcon class="transcriptions-state__spin" name="i-lucide-loader-circle" aria-hidden="true" />
      <strong>Carregando áudios...</strong>
    </div>

    <div v-else-if="errorMessage" class="transcriptions-state transcriptions-state--error">
      <UIcon name="i-lucide-circle-alert" aria-hidden="true" />
      <strong>{{ errorMessage }}</strong>
      <AppPanelButton variant="ghost" @click="loadPage()">Tentar novamente</AppPanelButton>
    </div>

    <div v-else-if="items.length === 0" class="transcriptions-state">
      <UIcon name="i-lucide-file-audio" aria-hidden="true" />
      <strong>Nenhum áudio salvo ainda</strong>
      <p v-if="featureEnabled">Inicie um atendimento na Fila para criar o primeiro registro.</p>
      <p v-else>A gravação está desativada; os registros anteriores continuam disponíveis aqui.</p>
      <NuxtLink to="/operacao">Ir para a Fila</NuxtLink>
    </div>

    <div v-else class="transcriptions-list">
      <AttendanceTranscriptionStoreSection
        v-for="(group, index) in groupedItems"
        :key="group.id"
        :group="group"
        :initially-open="index === 0"
        :audio-loading-id="audioLoadingId"
        :transcribing-id="transcribingId"
        :analyzing-id="analyzingId"
        @details="openDetails"
        @listen="listen"
        @transcribe="requestTranscription"
        @analyze="requestAnalysis"
      />

      <div
        v-if="audioErrorMessage || transcriptionErrorMessage"
        class="transcriptions-workspace__error"
      >
        {{ audioErrorMessage || transcriptionErrorMessage }}
      </div>

      <footer class="transcriptions-pagination">
        <span>{{ pageStart }}–{{ pageEnd }} de {{ total }}</span>
        <div>
          <AppPanelButton
            variant="ghost"
            :disabled="!hasPrevious || loading"
            @click="loadPage(offset - limit)"
          >
            Anterior
          </AppPanelButton>
          <AppPanelButton
            variant="ghost"
            :disabled="!hasNext || loading"
            @click="loadPage(offset + limit)"
          >
            Próxima
          </AppPanelButton>
        </div>
      </footer>
    </div>

    <AttendanceTranscriptionDetailDrawer
      :open="detailDrawerOpen"
      :item="selectedItem"
      :audio-loading="audioLoadingId === selectedRecordingId"
      :audio-url="selectedAudioUrl"
      :transcribing="transcribingId === selectedRecordingId"
      :analyzing="analyzingId === selectedRecordingId"
      @update:open="closeDetailDrawer"
      @load-audio="transcriptionsStore.loadAudio($event.id)"
      @transcribe="requestTranscription"
      @analyze="requestAnalysis"
    />

    <AttendanceTranscriptionsConfigDrawer
      v-model:open="configDrawerOpen"
      :config="config"
      :loading="configLoading"
      :saving="configSaving"
      :error-message="configErrorMessage"
      @reload="transcriptionsStore.loadConfig()"
      @save="saveAnalysisConfig"
    />
  </section>
</template>

<style scoped>
.transcriptions-workspace {
  display: flex;
  min-height: 0;
  flex: 1;
  flex-direction: column;
  gap: 0.75rem;
  padding: 0.75rem 1rem 1rem;
  overflow-y: auto;
}

.transcriptions-workspace__header {
  display: flex;
  flex: 0 0 auto;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.transcriptions-workspace__toolbar {
  display: flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 0.65rem;
  padding: 0.65rem 0.75rem;
  border: 1px solid rgb(var(--border) / 0.85);
  border-radius: 12px;
  background: rgb(var(--surface-2) / 0.52);
  overflow-x: auto;
}

.transcriptions-rollout {
  display: flex;
  min-width: 19rem;
  flex: 1 1 22rem;
  align-items: center;
  gap: 0.75rem;
}

.transcriptions-rollout__icon {
  display: grid;
  width: 2rem;
  height: 2rem;
  flex: 0 0 auto;
  place-items: center;
  border-radius: 9px;
  background: rgb(var(--primary) / 0.12);
  color: rgb(var(--primary));
}

.transcriptions-rollout__copy {
  display: grid;
  flex: 1;
  gap: 0.1rem;
  min-width: 0;
}

.transcriptions-rollout__copy span,
.transcriptions-rollout__copy small {
  color: rgb(var(--muted));
  font-size: 0.72rem;
}

.transcriptions-rollout__copy small,
.transcriptions-rollout__status.is-error {
  color: rgb(var(--danger));
}

.transcriptions-rollout__status {
  padding: 0.25rem 0.52rem;
  border-radius: 999px;
  background: rgb(var(--surface));
  color: rgb(var(--muted));
  font-size: 0.68rem;
  font-weight: 800;
}

.transcriptions-rollout__status.is-active {
  color: rgb(var(--success));
}

.transcriptions-workspace__filter {
  width: 13rem;
  flex: 0 0 13rem;
}

.transcriptions-workspace__config-button {
  flex: 0 0 auto;
  gap: 0.42rem;
  cursor: pointer;
}

.transcriptions-workspace__config-button:hover {
  border-color: rgb(var(--primary) / 0.45);
  background: rgb(var(--primary) / 0.1);
  color: rgb(var(--primary));
}

.transcriptions-state {
  display: grid;
  min-height: 14rem;
  place-items: center;
  align-content: center;
  gap: 0.5rem;
  padding: 1.5rem;
  border: 1px dashed rgb(var(--border));
  border-radius: 14px;
  color: rgb(var(--muted));
  text-align: center;
}

.transcriptions-state p {
  margin: 0;
}

.transcriptions-state a {
  color: rgb(var(--primary));
  font-weight: 750;
  text-decoration: none;
}

.transcriptions-state--error,
.transcriptions-workspace__error {
  color: rgb(var(--danger));
}

.transcriptions-state__spin {
  animation: transcription-spin 900ms linear infinite;
}

.transcriptions-list {
  display: grid;
  gap: 0.55rem;
}

.transcriptions-workspace__error {
  padding: 0.6rem 0.75rem;
  border: 1px solid rgb(var(--danger) / 0.28);
  border-radius: 10px;
  background: rgb(var(--danger) / 0.08);
  font-size: 0.74rem;
}

.transcriptions-pagination,
.transcriptions-pagination > div {
  display: flex;
  align-items: center;
  gap: 0.35rem;
}

.transcriptions-pagination {
  justify-content: space-between;
  padding: 0.25rem;
  color: rgb(var(--muted));
  font-size: 0.7rem;
}

.transcriptions-pagination :deep(.app-panel-button) {
  min-height: 1.9rem;
  padding: 0 0.65rem;
  border-radius: 8px;
  font-size: 0.68rem;
}

@keyframes transcription-spin {
  to {
    transform: rotate(360deg);
  }
}

@media (max-width: 640px) {
  .transcriptions-workspace {
    padding: 0.65rem;
  }

  .transcriptions-workspace__header {
    align-items: flex-start;
  }

  .transcriptions-workspace__toolbar {
    align-items: stretch;
    flex-direction: column;
    overflow-x: visible;
  }

  .transcriptions-rollout {
    min-width: 0;
    align-items: flex-start;
    flex-wrap: wrap;
  }

  .transcriptions-rollout__copy {
    min-width: calc(100% - 2.75rem);
  }

  .transcriptions-workspace__filter {
    width: 100%;
  }

  .transcriptions-workspace__config-button {
    align-self: flex-end;
  }
}
</style>
