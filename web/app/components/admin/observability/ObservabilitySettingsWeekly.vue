<script setup lang="ts">
import ObservabilityRecipientsField from '~/components/admin/observability/ObservabilityRecipientsField.vue'
import ObservabilitySettingsBlock from '~/components/admin/observability/ObservabilitySettingsBlock.vue'
import { useBlockSaver } from '~/composables/useObservabilitySettings'
import type { ObsSettings, ObsTextPreview, ObsWeeklySettings } from '~/types/observability-settings'
import { getApiErrorMessage } from '~/utils/api-client'
import { formatDate } from '~/utils/observability/format'
import { cloneJson, sameJson, WEEKDAY_OPTIONS, weeklySummary } from '~/utils/observability/settings'

const props = defineProps<{
  settings: ObsSettings
  save: (key: 'weeklyReport', value: object) => Promise<void>
  preview: () => Promise<ObsTextPreview>
  send: () => Promise<void>
  reload: () => Promise<void>
}>()

const draft = ref<ObsWeeklySettings>(cloneJson(props.settings.weeklyReport))
watch(
  () => props.settings.weeklyReport,
  (value) => {
    draft.value = cloneJson(value)
  },
)

const saver = useBlockSaver()
const dirty = computed(() => !sameJson(draft.value, props.settings.weeklyReport))

const previewing = ref(false)
const previewData = ref<ObsTextPreview | null>(null)
const sending = ref(false)
const actionMessage = ref('')
const actionFailed = ref(false)

function submit() {
  return saver.run(() => props.save('weeklyReport', draft.value))
}

async function showPreview() {
  previewing.value = true
  actionMessage.value = ''
  try {
    previewData.value = await props.preview()
  } catch (cause) {
    previewData.value = null
    actionFailed.value = true
    actionMessage.value = getApiErrorMessage(cause, 'Não foi possível montar o relatório.')
  } finally {
    previewing.value = false
  }
}

function isNotConfigured(cause: unknown): boolean {
  const err = cause as { statusCode?: number; status?: number }
  return (err?.statusCode ?? err?.status) === 409
}

async function sendNow() {
  sending.value = true
  actionMessage.value = ''
  try {
    await props.send()
    actionFailed.value = false
    actionMessage.value = 'Relatório enviado aos destinatários.'
    await props.reload()
  } catch (cause) {
    actionFailed.value = true
    actionMessage.value = isNotConfigured(cause)
      ? 'E-mail não configurado: defina o SMTP e ao menos um destinatário.'
      : getApiErrorMessage(cause, 'Não foi possível enviar o relatório.')
  } finally {
    sending.value = false
  }
}
</script>

<template>
  <ObservabilitySettingsBlock title="Relatório semanal" :summary="weeklySummary(draft)">
    <USwitch v-model="draft.enabled" label="Enviar o relatório semanal por e-mail" />

    <div class="obs-fields">
      <div class="obs-field">
        <label class="obs-field__label" for="obs-weekly-day">Dia da semana</label>
        <select id="obs-weekly-day" v-model.number="draft.weekday" class="obs-select">
          <option v-for="option in WEEKDAY_OPTIONS" :key="option.value" :value="option.value">
            {{ option.label }}
          </option>
        </select>
        <p v-if="saver.fieldErrors.value.weekday" class="obs-hint obs-text--bad">
          {{ saver.fieldErrors.value.weekday }}
        </p>
      </div>
      <div class="obs-field">
        <label class="obs-field__label" for="obs-weekly-hour">Hora (0 a 23)</label>
        <UInput
          id="obs-weekly-hour"
          v-model.number="draft.hour"
          type="number"
          size="sm"
          :min="0"
          :max="23"
        />
        <p v-if="saver.fieldErrors.value.hour" class="obs-hint obs-text--bad">
          {{ saver.fieldErrors.value.hour }}
        </p>
        <p v-else class="obs-hint">Fuso {{ draft.timezone }}.</p>
      </div>
    </div>

    <ObservabilityRecipientsField
      v-model="draft.recipients"
      label="Destinatários do relatório"
      hint="Vazio = os mesmos dos alertas."
      :error="saver.fieldErrors.value.recipients"
    />

    <p class="obs-hint">
      <template v-if="settings.weeklyState.lastSentAt">
        Último envio: {{ formatDate(settings.weeklyState.lastSentAt) }}.
      </template>
      <template v-else>Nenhum envio ainda.</template>
      <span v-if="settings.weeklyState.lastError" class="obs-text--bad">
        Última falha: {{ settings.weeklyState.lastError }}
      </span>
    </p>

    <div class="obs-toolbar">
      <UButton
        label="Salvar bloco"
        icon="i-lucide-save"
        size="sm"
        :disabled="!dirty"
        :loading="saver.saving.value"
        @click="submit"
      />
      <UButton
        label="Pré-visualizar"
        icon="i-lucide-eye"
        size="sm"
        color="neutral"
        variant="soft"
        :loading="previewing"
        @click="showPreview"
      />
      <UButton
        label="Enviar agora"
        icon="i-lucide-send"
        size="sm"
        color="neutral"
        variant="soft"
        :loading="sending"
        @click="sendNow"
      />
      <span
        v-if="saver.message.value"
        :class="['obs-hint', saver.failed.value ? 'obs-text--bad' : 'obs-text--good']"
      >
        {{ saver.message.value }}
      </span>
      <span
        v-if="actionMessage"
        :class="['obs-hint', actionFailed ? 'obs-text--bad' : 'obs-text--good']"
      >
        {{ actionMessage }}
      </span>
    </div>
    <p v-if="dirty" class="obs-hint">
      A prévia e o envio usam a configuração salva; salve antes para valer as mudanças.
    </p>

    <div v-if="previewData" class="obs-preview">
      <div class="obs-preview__subject">
        <span class="obs-muted">Assunto:</span>
        <strong>{{ previewData.subject }}</strong>
      </div>
      <pre class="obs-preview__text">{{ previewData.text }}</pre>
    </div>
  </ObservabilitySettingsBlock>
</template>
