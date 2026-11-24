<script setup lang="ts">
import ObservabilitySettingsBlock from '~/components/admin/observability/ObservabilitySettingsBlock.vue'
import { useBlockSaver } from '~/composables/useObservabilitySettings'
import type { ObsEmailSettings, ObsSettings, ObsTextPreview } from '~/types/observability-settings'
import { getApiErrorMessage } from '~/utils/api-client'
import {
  cloneJson,
  EMAIL_SECTION_OPTIONS,
  emailSummary,
  fieldErrorsFrom,
  sameJson,
} from '~/utils/observability/settings'

const props = defineProps<{
  settings: ObsSettings
  save: (key: 'email', value: object) => Promise<void>
  preview: (value: object) => Promise<ObsTextPreview>
}>()

const draft = ref<ObsEmailSettings>(cloneJson(props.settings.email))
watch(
  () => props.settings.email,
  (value) => {
    draft.value = cloneJson(value)
  },
)

const saver = useBlockSaver()
const dirty = computed(() => !sameJson(draft.value, props.settings.email))

const previewing = ref(false)
const previewData = ref<ObsTextPreview | null>(null)
const previewError = ref('')

function toggleSection(section: string, on: boolean) {
  const next = new Set(draft.value.sections)
  if (on) next.add(section)
  else next.delete(section)
  draft.value.sections = [...next]
}

function submit() {
  return saver.run(() => props.save('email', draft.value))
}

async function showPreview() {
  previewing.value = true
  previewError.value = ''
  try {
    previewData.value = await props.preview(draft.value)
  } catch (cause) {
    previewData.value = null
    const fields = fieldErrorsFrom(cause)
    previewError.value =
      Object.values(fields)[0] || getApiErrorMessage(cause, 'Não foi possível montar a prévia.')
  } finally {
    previewing.value = false
  }
}
</script>

<template>
  <ObservabilitySettingsBlock title="Layout do e-mail de alertas" :summary="emailSummary(draft)">
    <div class="obs-fields">
      <div class="obs-field">
        <label class="obs-field__label" for="obs-subject-prefix">Prefixo do assunto</label>
        <UInput id="obs-subject-prefix" v-model="draft.subjectPrefix" size="sm" maxlength="40" />
        <p v-if="saver.fieldErrors.value.subjectPrefix" class="obs-hint obs-text--bad">
          {{ saver.fieldErrors.value.subjectPrefix }}
        </p>
        <p v-else class="obs-hint">Ex.: [Omni] ou [Omni produção]. Até 40 caracteres.</p>
      </div>
    </div>

    <USwitch v-model="draft.group" label="Agrupar os alertas da rodada em um único e-mail" />
    <USwitch v-model="draft.includeResolved" label="Avisar também quando um alerta for resolvido" />

    <div class="obs-field">
      <span class="obs-field__label">Seções do e-mail</span>
      <UCheckbox
        v-for="option in EMAIL_SECTION_OPTIONS"
        :key="option.value"
        :model-value="draft.sections.includes(option.value)"
        :label="option.label"
        @update:model-value="toggleSection(option.value, $event === true)"
      />
      <p v-if="saver.fieldErrors.value.sections" class="obs-hint obs-text--bad">
        {{ saver.fieldErrors.value.sections }}
      </p>
    </div>

    <div class="obs-field">
      <label class="obs-field__label" for="obs-footer">Rodapé (opcional)</label>
      <UTextarea id="obs-footer" v-model="draft.footer" :rows="3" maxlength="500" />
      <p v-if="saver.fieldErrors.value.footer" class="obs-hint obs-text--bad">
        {{ saver.fieldErrors.value.footer }}
      </p>
    </div>

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
      <span
        v-if="saver.message.value"
        :class="['obs-hint', saver.failed.value ? 'obs-text--bad' : 'obs-text--good']"
      >
        {{ saver.message.value }}
      </span>
    </div>

    <p v-if="previewError" class="obs-hint obs-text--bad">{{ previewError }}</p>
    <div v-if="previewData" class="obs-preview">
      <div class="obs-preview__subject">
        <span class="obs-muted">Assunto:</span>
        <strong>{{ previewData.subject }}</strong>
      </div>
      <pre class="obs-preview__text">{{ previewData.text }}</pre>
      <p class="obs-hint">
        Prévia com alertas de exemplo; os e-mails reais usam os dados do momento.
      </p>
    </div>
  </ObservabilitySettingsBlock>
</template>
