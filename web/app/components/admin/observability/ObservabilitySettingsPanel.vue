<script setup lang="ts">
import ObservabilitySettingsAlerts from '~/components/admin/observability/ObservabilitySettingsAlerts.vue'
import ObservabilitySettingsEmail from '~/components/admin/observability/ObservabilitySettingsEmail.vue'
import ObservabilitySettingsWeekly from '~/components/admin/observability/ObservabilitySettingsWeekly.vue'
import ObservabilityState from '~/components/admin/observability/ObservabilityState.vue'
import { useObservabilitySettings } from '~/composables/useObservabilitySettings'

const { settings, loading, error, load, save, previewEmail, weeklyPreview, weeklySend } =
  useObservabilitySettings()
</script>

<template>
  <div class="obs-stack">
    <p class="obs-hint obs-settings-channel">
      <strong>Canal atual: e-mail. WhatsApp: planejado.</strong>
      <template v-if="settings">
        <span :class="settings.channel.smtpConfigured ? 'obs-text--good' : 'obs-text--bad'">
          {{ settings.channel.smtpConfigured ? 'SMTP configurado' : 'SMTP não configurado' }}
        </span>
        <span>
          Alertas vão para {{ settings.channel.alertRecipientCount }} destinatário(s). As mudanças
          valem em até {{ settings.channel.cacheSeconds }} s.
        </span>
      </template>
    </p>

    <ObservabilityState
      :loading="loading"
      :error="settings ? '' : error"
      :empty="!settings"
      @retry="load"
    >
      <template v-if="settings">
        <ObservabilitySettingsAlerts
          :settings="settings"
          :save="(key, value) => save(key, value)"
        />
        <ObservabilitySettingsEmail
          :settings="settings"
          :save="(key, value) => save(key, value)"
          :preview="previewEmail"
        />
        <ObservabilitySettingsWeekly
          :settings="settings"
          :save="(key, value) => save(key, value)"
          :preview="weeklyPreview"
          :send="weeklySend"
          :reload="load"
        />
      </template>
    </ObservabilityState>
  </div>
</template>

<style>
.obs-settings-channel {
  display: flex;
  flex-wrap: wrap;
  gap: 0.3rem 0.9rem;
}

.obs-fields {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr));
  gap: 0.8rem;
  align-items: start;
}

.obs-field {
  display: grid;
  gap: 0.35rem;
  min-width: 0;
}

.obs-field__label {
  font-size: 0.78rem;
  font-weight: 600;
}

.obs-select {
  padding: 0.35rem 0.5rem;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm, 6px);
  background: rgb(var(--surface));
  color: inherit;
  font: inherit;
  font-size: 0.82rem;
}

.obs-preview {
  display: grid;
  gap: 0.5rem;
  padding: 0.8rem 1rem;
  border: 1px dashed rgb(var(--border));
  border-radius: var(--radius-md);
  background: rgb(var(--surface-2));
}

.obs-preview__subject {
  font-size: 0.85rem;
}

.obs-preview__text {
  margin: 0;
  max-height: 24rem;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-word;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.76rem;
}
</style>
