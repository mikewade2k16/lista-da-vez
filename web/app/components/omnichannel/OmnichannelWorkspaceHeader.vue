<script setup lang="ts">
import { computed } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import { useOmnichannelGlobalAI } from '~/composables/omnichannel/useOmnichannelGlobalAI'

const props = withDefaults(
  defineProps<{
    mode: 'inbox' | 'automation'
    visible?: boolean
    title: string
    description: string
    color: 'error' | 'warning' | 'success'
    connected?: boolean
    configured?: boolean
    canConnect?: boolean
    clientValue?: string
    clientOptions?: Array<{ value: string; label: string; meta?: string }>
    clientLoading?: boolean
    clientDisabled?: boolean
  }>(),
  {
    visible: true,
    connected: false,
    configured: false,
    canConnect: false,
    clientOptions: () => [],
  },
)
const emit = defineEmits<{
  (event: 'connect' | 'configure'): void
  (event: 'update:client', value: string): void
}>()

const { loading, saving, error, enabled, ready, canManageAutomation, canConfigure, toggle } =
  useOmnichannelGlobalAI()

const workspaceLink = computed(() =>
  props.mode === 'inbox'
    ? { label: 'Automação IA', to: '/omnichannel/automacao', icon: 'i-lucide-bot' }
    : { label: 'Ver Omnichannel', to: '/omnichannel', icon: 'i-lucide-messages-square' },
)
const configLabel = computed(() =>
  props.mode === 'inbox' ? 'Configurar' : 'Configurar atendimento',
)
const displayTitle = computed(() =>
  props.connected ? 'WhatsApp conectado' : 'WhatsApp desconectado',
)
const displayDescription = computed(() =>
  props.connected ? 'Canal pronto para atendimento.' : 'Gere o QR Code e pareie o WhatsApp.',
)
const displayColor = computed(() => (props.connected ? 'success' : 'warning'))
</script>

<template>
  <div v-if="visible" class="omnichannel-workspace-header">
    <div
      class="omnichannel-workspace-header__connection"
      :class="`omnichannel-workspace-header__connection--${displayColor}`"
      role="status"
      aria-live="polite"
    >
      <span class="omnichannel-workspace-header__pilot">Piloto Evolution</span>
      <span aria-hidden="true" class="omnichannel-workspace-header__separator">·</span>
      <span aria-hidden="true" class="omnichannel-workspace-header__dot"></span>
      <strong>{{ displayTitle }}</strong>
      <span aria-hidden="true" class="omnichannel-workspace-header__separator">·</span>
      <span class="omnichannel-workspace-header__description">{{ displayDescription }}</span>
      <UButton
        v-if="canConnect && !connected"
        class="omnichannel-workspace-header__connect"
        size="xs"
        color="primary"
        variant="soft"
        @click="emit('connect')"
      >
        Conectar
      </UButton>
    </div>

    <div
      v-if="canConfigure || clientOptions.length"
      class="omnichannel-workspace-header__actions"
      :title="error || 'Controles gerais do atendimento deste cliente'"
    >
      <AppSelectField
        v-if="clientOptions.length"
        class="omnichannel-workspace-header__client-select"
        :model-value="clientValue"
        :options="clientOptions"
        placeholder="Selecionar cliente"
        empty-label="Nenhum cliente acessível."
        search-placeholder="Buscar cliente"
        searchable
        compact
        :disabled="clientDisabled || clientLoading"
        @update:model-value="emit('update:client', String($event))"
      />

      <div v-if="canManageAutomation" class="omnichannel-workspace-header__global-ai">
        <span>IA geral</span>
        <USwitch
          :model-value="enabled"
          :loading="loading"
          :disabled="!ready || loading || saving"
          aria-label="Ativar ou parar a IA geral deste cliente"
          @update:model-value="toggle"
        />
        <span class="omnichannel-workspace-header__state">
          {{ loading ? '…' : enabled ? 'Ativa' : 'Parada' }}
        </span>
      </div>

      <NuxtLink
        v-if="canManageAutomation"
        :to="workspaceLink.to"
        class="omnichannel-workspace-header__link"
      >
        <UIcon :name="workspaceLink.icon" aria-hidden="true" />
        {{ workspaceLink.label }}
      </NuxtLink>

      <AppPanelButton variant="ghost" @click="emit('configure')">
        <UIcon name="i-lucide-settings-2" aria-hidden="true" />
        {{ configLabel }}
      </AppPanelButton>
    </div>
  </div>
</template>

<style scoped>
.omnichannel-workspace-header {
  position: relative;
  z-index: 30;
  display: flex;
  flex: 0 0 auto;
  min-width: 0;
  min-height: 2.5rem;
  align-items: center;
  gap: 0.55rem;
  margin: 0.35rem 1rem 0;
  white-space: nowrap;
}

.omnichannel-workspace-header__connection {
  display: flex;
  flex: 1 1 auto;
  align-items: center;
  gap: 0.42rem;
  min-width: 0;
  min-height: 2.5rem;
  margin: 0;
  padding: 0.38rem 0.5rem 0.38rem 0.7rem;
  overflow: hidden;
  border: 1px solid rgb(var(--border) / 0.58);
  border-radius: 0.8rem;
  background: linear-gradient(110deg, rgb(var(--surface) / 0.72), rgb(var(--surface-2) / 0.46));
  box-shadow:
    inset 0 1px 0 rgb(var(--text) / 0.05),
    0 8px 24px rgb(var(--surface) / 0.18);
  color: rgb(var(--text));
  font-size: 0.75rem;
  backdrop-filter: blur(18px) saturate(135%);
}

.omnichannel-workspace-header__connection--warning {
  border-color: rgb(var(--warning) / 0.3);
  background: linear-gradient(110deg, rgb(var(--warning) / 0.09), rgb(var(--surface) / 0.66));
}

.omnichannel-workspace-header__connection--error {
  border-color: rgb(var(--error) / 0.3);
  background: linear-gradient(110deg, rgb(var(--error) / 0.09), rgb(var(--surface) / 0.66));
}

.omnichannel-workspace-header__connection--success {
  border-color: rgb(var(--success) / 0.3);
  background: linear-gradient(110deg, rgb(var(--success) / 0.09), rgb(var(--surface) / 0.66));
}

.omnichannel-workspace-header__pilot {
  flex: 0 0 auto;
  color: rgb(var(--muted));
  font-size: 0.7rem;
  font-weight: 700;
}

.omnichannel-workspace-header__separator {
  color: rgb(var(--muted));
}

.omnichannel-workspace-header__dot {
  flex: 0 0 auto;
  width: 0.42rem;
  height: 0.42rem;
  border-radius: 999px;
  background: rgb(var(--warning));
  box-shadow: 0 0 0 3px rgb(var(--warning) / 0.1);
}

.omnichannel-workspace-header__connection--success .omnichannel-workspace-header__dot {
  background: rgb(var(--success));
  box-shadow: 0 0 0 3px rgb(var(--success) / 0.1);
}

.omnichannel-workspace-header__connection--error .omnichannel-workspace-header__dot {
  background: rgb(var(--error));
  box-shadow: 0 0 0 3px rgb(var(--error) / 0.1);
}

.omnichannel-workspace-header__description {
  min-width: 0;
  overflow: hidden;
  color: rgb(var(--muted));
  text-overflow: ellipsis;
}

.omnichannel-workspace-header__connect {
  flex: 0 0 auto;
  margin-left: auto;
}

.omnichannel-workspace-header__actions {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 0.5rem;
  min-width: 0;
}

.omnichannel-workspace-header__client-select {
  width: min(16rem, 24vw);
  min-width: 11rem;
}

.omnichannel-workspace-header__client-select :deep(.app-select-field__label) {
  display: none;
}

.omnichannel-workspace-header__global-ai {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  min-height: 32px;
  padding: 0 0.55rem;
  border: 1px solid rgb(var(--success) / 0.24);
  border-radius: var(--radius-soft);
  background: rgb(var(--success) / 0.08);
  color: rgb(var(--text));
  font-size: 0.76rem;
  font-weight: 600;
}

.omnichannel-workspace-header__global-ai:has(:disabled) {
  border-color: rgb(var(--border) / 0.5);
  background: rgb(var(--surface-2) / 0.55);
  color: rgb(var(--muted));
}

.omnichannel-workspace-header__state {
  min-width: 2.45rem;
  color: rgb(var(--success));
  text-align: center;
}

.omnichannel-workspace-header__global-ai:has(:disabled) .omnichannel-workspace-header__state {
  color: rgb(var(--muted));
}

.omnichannel-workspace-header__link {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  min-height: 32px;
  padding: 0 0.65rem;
  border-radius: var(--radius-soft);
  color: var(--text-muted);
  font-size: 0.78rem;
  font-weight: 600;
  text-decoration: none;
}

.omnichannel-workspace-header__link:hover {
  background: rgb(var(--surface-2));
  color: var(--text-main);
}

.omnichannel-workspace-header :deep(.app-panel-button) {
  min-height: 32px;
  gap: 0.4rem;
  padding-inline: 0.65rem;
  border: 0;
  border-radius: var(--radius-soft);
  font-weight: 600;
}

@media (max-width: 900px) {
  .omnichannel-workspace-header {
    align-items: stretch;
    flex-direction: column;
    white-space: normal;
  }

  .omnichannel-workspace-header__actions {
    justify-content: flex-end;
  }
}
</style>
