<script setup lang="ts">
import OmniMinimalPopover from '~/components/omni/overlay/OmniMinimalPopover.vue'
import type { AccountItem } from '~/types/accounts'

const props = defineProps<{
  account: AccountItem
  busy?: boolean
}>()

const emit = defineEmits<{
  'toggle-enabled': [value: boolean]
  'rotate-key': []
}>()

const open = ref(false)
const enabled = ref(false)

watch(
  () => open.value,
  (isOpen) => {
    if (!isOpen) {
      return
    }

    enabled.value = Boolean(props.account.webhookEnabled)
  },
)

function copyKey() {
  if (!import.meta.client) {
    return
  }

  const value = String(props.account.webhookKey || '').trim()
  if (!value) {
    return
  }

  navigator.clipboard?.writeText(value).catch(() => {})
}

function save() {
  if (props.busy) {
    return
  }

  emit('toggle-enabled', enabled.value)
  open.value = false
}

function rotate() {
  if (props.busy) {
    return
  }

  emit('rotate-key')
}

function cancel() {
  open.value = false
}

function onSubmitShortcut(event: KeyboardEvent) {
  event.preventDefault()
  save()
}

function onCancelShortcut() {
  cancel()
}
</script>

<template>
  <OmniMinimalPopover
    :open="open"
    title="Webhook"
    width-class="w-[320px] max-w-[90vw]"
    :close-on-submit-shortcut="false"
    @update:open="open = $event"
    @submit-shortcut="onSubmitShortcut"
    @cancel-shortcut="onCancelShortcut"
  >
    <template #trigger>
      <UButton
        icon="i-lucide-key-round"
        color="neutral"
        variant="ghost"
        size="sm"
        aria-label="Webhook"
      />
    </template>

    <div class="clients-webhook-popover__body space-y-3">
      <div
        class="clients-webhook-popover__key-box rounded-[var(--radius-sm)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] p-2 text-xs"
      >
        <p class="mb-1 text-[rgb(var(--muted))]">Chave atual</p>
        <code class="block break-all text-[rgb(var(--text))]">
          {{ props.account.webhookKey || '-' }}
        </code>
      </div>

      <div
        class="clients-webhook-popover__switch-row flex items-center justify-between rounded-[var(--radius-sm)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] px-3 py-2"
      >
        <span class="text-sm text-[rgb(var(--text))]">Webhook habilitado</span>
        <USwitch v-model="enabled" class="clients-webhook-popover__switch" />
      </div>

      <div class="clients-webhook-popover__actions flex items-center justify-between gap-2">
        <UButton
          icon="i-lucide-refresh-cw"
          label="Gerar nova chave"
          color="neutral"
          variant="soft"
          size="sm"
          :loading="Boolean(props.busy)"
          :disabled="Boolean(props.busy)"
          data-omni-enter-ignore="true"
          @click="rotate"
        />
        <UButton
          icon="i-lucide-copy"
          label="Copiar"
          color="neutral"
          variant="ghost"
          size="sm"
          data-omni-enter-ignore="true"
          @click="copyKey"
        />
      </div>
    </div>

    <template #footer>
      <UButton label="Cancelar" color="neutral" variant="ghost" size="sm" @click="cancel" />
      <UButton
        label="Salvar"
        color="primary"
        size="sm"
        :loading="Boolean(props.busy)"
        :disabled="Boolean(props.busy)"
        @click="save"
      />
    </template>
  </OmniMinimalPopover>
</template>
