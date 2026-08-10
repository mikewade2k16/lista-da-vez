<script setup lang="ts">
import { computed, ref } from 'vue'

import { usePerformanceFeedback } from '~/composables/usePerformanceFeedback'
import AppToolbarButton from '~/components/ui/AppToolbarButton.vue'
import { canEditPerformanceFeedback } from '~/domain/utils/permissions'
import { useAuthStore } from '~/stores/auth'
import type { PerformanceFeedbackSection } from '~/types/performance-feedback'
import PerformanceFeedbackSettingsModal from './PerformanceFeedbackSettingsModal.vue'

const props = withDefaults(
  defineProps<{
    storeId?: string
    disabled?: boolean
  }>(),
  {
    storeId: '',
    disabled: false,
  },
)

const feedback = usePerformanceFeedback()
const auth = useAuthStore()
const open = ref(false)
const settings = computed(() => feedback.context.value?.settings ?? null)
const unavailable = computed(() => props.disabled || !String(props.storeId || '').trim())
const canConfigure = computed(() =>
  canEditPerformanceFeedback(
    auth.role,
    auth.effectivePermissionKeys,
    auth.effectivePermissionsResolved,
  ),
)

async function openSettings(): Promise<void> {
  if (unavailable.value) return
  open.value = true
  await feedback.openSettingsForStore(props.storeId)
}

async function saveSettings(value: {
  cadence: 'monthly' | 'weekly'
  defaultSections: PerformanceFeedbackSection[]
  expectedVersion: number
}): Promise<void> {
  const saved = await feedback.saveSettings(value)
  if (saved) open.value = false
}
</script>

<template>
  <template v-if="canConfigure">
    <AppToolbarButton
      icon="i-lucide-settings-2"
      label="Configurar feedback"
      variant="soft"
      :disabled="unavailable"
      :loading="open && feedback.pending.value"
      @click="openSettings"
    />

    <PerformanceFeedbackSettingsModal
      v-model:open="open"
      :settings="settings"
      :loading="feedback.pending.value"
      :saving="feedback.saving.value"
      @save="saveSettings"
    />
  </template>
</template>
