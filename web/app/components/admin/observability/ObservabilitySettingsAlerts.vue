<script setup lang="ts">
import ObservabilityRecipientsField from '~/components/admin/observability/ObservabilityRecipientsField.vue'
import ObservabilitySettingsBlock from '~/components/admin/observability/ObservabilitySettingsBlock.vue'
import { useBlockSaver } from '~/composables/useObservabilitySettings'
import type { ObsAlertSettings, ObsSettings } from '~/types/observability-settings'
import {
  alertsGeneralSummary,
  cloneJson,
  rulesPayload,
  rulesSummary,
  RULE_SEVERITY_OPTIONS,
  sameJson,
  thresholdHint,
} from '~/utils/observability/settings'

const props = defineProps<{
  settings: ObsSettings
  save: (key: 'alerts', value: object) => Promise<void>
}>()

// Rascunho local; "salvar" de cada bloco envia so os campos do bloco por cima do
// ultimo valor gravado, para um bloco nao gravar sem querer o que foi editado no outro.
const draft = ref<ObsAlertSettings>(cloneJson(props.settings.alerts))
watch(
  () => props.settings.alerts,
  (value) => {
    draft.value = cloneJson(value)
  },
)

const general = useBlockSaver()
const rules = useBlockSaver()

const generalDirty = computed(() => {
  const saved = props.settings.alerts
  return !sameJson(
    [draft.value.recipients, draft.value.renotifyMinutes, draft.value.quietHours],
    [saved.recipients, saved.renotifyMinutes, saved.quietHours],
  )
})
const rulesDirty = computed(() => !sameJson(draft.value.rules, props.settings.alerts.rules))

const catalog = computed(() => props.settings.rulesCatalog)

function saveGeneral() {
  return general.run(() =>
    props.save('alerts', {
      ...cloneJson(props.settings.alerts),
      recipients: draft.value.recipients,
      renotifyMinutes: Number(draft.value.renotifyMinutes),
      quietHours: draft.value.quietHours,
    }),
  )
}

function saveRules() {
  return rules.run(() =>
    props.save('alerts', {
      ...cloneJson(props.settings.alerts),
      rules: rulesPayload(draft.value.rules, catalog.value),
    }),
  )
}

function ruleError(rule: string, field: 'threshold' | 'severity'): string {
  return rules.fieldErrors.value[`rules.${rule}.${field}`] || ''
}

function restoreDefaults() {
  for (const item of catalog.value) {
    draft.value.rules[item.rule] = {
      enabled: true,
      severity: 'auto',
      threshold: item.hasThreshold ? item.defaultThreshold : null,
    }
  }
}
</script>

<template>
  <div class="obs-stack">
    <ObservabilitySettingsBlock
      title="Destinatários, reaviso e horário de silêncio"
      :summary="alertsGeneralSummary(draft, settings.channel.envRecipientCount)"
    >
      <ObservabilityRecipientsField
        v-model="draft.recipients"
        label="Destinatários dos alertas"
        :hint="`Vazio = usa OBS_ALERT_EMAILS do ambiente (${settings.channel.envRecipientCount} e-mail(s)).`"
        :error="general.fieldErrors.value.recipients"
      />

      <div class="obs-fields">
        <div class="obs-field">
          <label class="obs-field__label" for="obs-renotify">Reavisar a cada (minutos)</label>
          <UInput
            id="obs-renotify"
            v-model.number="draft.renotifyMinutes"
            type="number"
            size="sm"
            :min="15"
            :max="1440"
          />
          <p v-if="general.fieldErrors.value.renotifyMinutes" class="obs-hint obs-text--bad">
            {{ general.fieldErrors.value.renotifyMinutes }}
          </p>
          <p v-else class="obs-hint">Entre 15 e 1440. Enquanto o alerta continua aberto.</p>
        </div>
      </div>

      <div class="obs-field">
        <USwitch
          v-model="draft.quietHours.enabled"
          label="Horário de silêncio (segura os e-mails)"
        />
        <div v-if="draft.quietHours.enabled" class="obs-fields">
          <div class="obs-field">
            <label class="obs-field__label" for="obs-quiet-start">Início</label>
            <UInput id="obs-quiet-start" v-model="draft.quietHours.start" type="time" size="sm" />
            <p v-if="general.fieldErrors.value['quietHours.start']" class="obs-hint obs-text--bad">
              {{ general.fieldErrors.value['quietHours.start'] }}
            </p>
          </div>
          <div class="obs-field">
            <label class="obs-field__label" for="obs-quiet-end">Fim</label>
            <UInput id="obs-quiet-end" v-model="draft.quietHours.end" type="time" size="sm" />
            <p v-if="general.fieldErrors.value['quietHours.end']" class="obs-hint obs-text--bad">
              {{ general.fieldErrors.value['quietHours.end'] }}
            </p>
          </div>
        </div>
        <USwitch
          v-if="draft.quietHours.enabled"
          v-model="draft.quietHours.allowCritical"
          label="Críticos passam mesmo no silêncio"
        />
        <p v-if="draft.quietHours.enabled" class="obs-hint">
          Fuso {{ draft.quietHours.timezone }}. Ao fim do silêncio sai um e-mail com o acumulado.
          <span v-if="general.fieldErrors.value['quietHours.timezone']" class="obs-text--bad">
            {{ general.fieldErrors.value['quietHours.timezone'] }}
          </span>
        </p>
      </div>

      <div class="obs-toolbar">
        <UButton
          label="Salvar bloco"
          icon="i-lucide-save"
          size="sm"
          :disabled="!generalDirty"
          :loading="general.saving.value"
          @click="saveGeneral"
        />
        <span
          v-if="general.message.value"
          :class="['obs-hint', general.failed.value ? 'obs-text--bad' : 'obs-text--good']"
        >
          {{ general.message.value }}
        </span>
      </div>
    </ObservabilitySettingsBlock>

    <ObservabilitySettingsBlock title="Regras" :summary="rulesSummary(draft.rules, catalog)">
      <p class="obs-hint">
        Desligar uma regra resolve os alertas abertos dela. Severidade "automática" usa a do código;
        escolher uma fixa vale para todos os alertas da regra.
      </p>
      <div class="obs-tablewrap">
        <table class="obs-table">
          <thead>
            <tr>
              <th>Ligada</th>
              <th>Regra</th>
              <th>Área</th>
              <th>Severidade</th>
              <th>Limite</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in catalog" :key="item.rule">
              <td>
                <USwitch
                  v-if="draft.rules[item.rule]"
                  v-model="draft.rules[item.rule]!.enabled"
                  :aria-label="`Ligar ${item.label}`"
                />
              </td>
              <td>
                <strong>{{ item.label }}</strong>
                <div class="obs-muted obs-mono">{{ item.rule }}</div>
              </td>
              <td>{{ item.area }}</td>
              <td>
                <select
                  v-if="draft.rules[item.rule]"
                  v-model="draft.rules[item.rule]!.severity"
                  class="obs-select"
                  :aria-label="`Severidade de ${item.label}`"
                >
                  <option
                    v-for="option in RULE_SEVERITY_OPTIONS"
                    :key="option.value"
                    :value="option.value"
                  >
                    {{ option.label }}
                  </option>
                </select>
                <div v-if="ruleError(item.rule, 'severity')" class="obs-hint obs-text--bad">
                  {{ ruleError(item.rule, 'severity') }}
                </div>
              </td>
              <td>
                <template v-if="item.hasThreshold && draft.rules[item.rule]">
                  <div class="obs-threshold">
                    <UInput
                      v-model.number="draft.rules[item.rule]!.threshold"
                      type="number"
                      size="xs"
                      :min="item.min"
                      :max="item.max"
                      :aria-label="`Limite de ${item.label}`"
                    />
                    <span class="obs-muted">{{ item.unit }}</span>
                  </div>
                  <div
                    :class="['obs-hint', ruleError(item.rule, 'threshold') ? 'obs-text--bad' : '']"
                  >
                    {{ ruleError(item.rule, 'threshold') || thresholdHint(item) }}
                  </div>
                </template>
                <span v-else class="obs-muted">sem limite</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div class="obs-toolbar">
        <UButton
          label="Salvar bloco"
          icon="i-lucide-save"
          size="sm"
          :disabled="!rulesDirty"
          :loading="rules.saving.value"
          @click="saveRules"
        />
        <UButton
          label="Restaurar padrões"
          icon="i-lucide-rotate-ccw"
          size="sm"
          color="neutral"
          variant="soft"
          @click="restoreDefaults"
        />
        <span
          v-if="rules.message.value"
          :class="['obs-hint', rules.failed.value ? 'obs-text--bad' : 'obs-text--good']"
        >
          {{ rules.message.value }}
        </span>
      </div>
    </ObservabilitySettingsBlock>
  </div>
</template>

<style scoped>
.obs-threshold {
  display: flex;
  align-items: center;
  gap: 0.4rem;
}

.obs-select {
  padding: 0.25rem 0.4rem;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm, 6px);
  background: rgb(var(--surface));
  color: inherit;
  font: inherit;
  font-size: 0.8rem;
}
</style>
