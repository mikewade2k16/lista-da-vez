<script setup lang="ts">
import { onMounted, watch } from 'vue'

import { useAttendanceAudioRecordingStore } from '~/stores/attendanceAudioRecording'

interface ActiveService {
  serviceId?: unknown
  name?: unknown
  stoppedAt?: unknown
}

const props = defineProps<{
  enabled: boolean
  activeServices: ActiveService[]
}>()

const recordingStore = useAttendanceAudioRecordingStore()

function ensureRecordingMatchesActiveService() {
  void recordingStore.reconcileActiveServices(props.activeServices)
}

onMounted(() => {
  if (props.enabled) void recordingStore.initialize()
})

watch(
  () => props.enabled,
  (enabled) => {
    if (enabled) {
      void recordingStore.initialize()
    } else {
      void recordingStore.stopAll('feature-disabled')
    }
  },
)

watch(
  () =>
    props.activeServices
      .map(
        (service) =>
          `${String(service?.serviceId || '').trim()}:${Number(service?.stoppedAt || 0)}`,
      )
      .sort()
      .join('|'),
  ensureRecordingMatchesActiveService,
)
</script>

<template>
  <span v-if="false" aria-hidden="true"></span>
</template>
