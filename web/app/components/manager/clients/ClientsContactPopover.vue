<script setup lang="ts">
import OmniMinimalPopover from '~/components/omni/overlay/OmniMinimalPopover.vue'
import type { AccountItem } from '~/types/accounts'

const props = defineProps<{
  account: AccountItem
  busy?: boolean
}>()

const emit = defineEmits<{
  save: [
    payload: { logo: string; contactPhone: string; contactSite: string; contactAddress: string },
  ]
}>()

const open = ref(false)
const logo = ref('')
const phone = ref('')
const site = ref('')
const address = ref('')

watch(
  () => open.value,
  (isOpen) => {
    if (!isOpen) {
      return
    }

    logo.value = String(props.account.logo || '')
    phone.value = String(props.account.contactPhone || '')
    site.value = String(props.account.contactSite || '')
    address.value = String(props.account.contactAddress || '')
  },
)

function normalize(value: unknown, max = 255) {
  return String(value ?? '')
    .replace(/\s+/g, ' ')
    .trim()
    .slice(0, max)
}

function save() {
  if (props.busy) {
    return
  }

  emit('save', {
    logo: normalize(logo.value, 255),
    contactPhone: normalize(phone.value, 60),
    contactSite: normalize(site.value, 255),
    contactAddress: normalize(address.value, 255),
  })

  open.value = false
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
    title="Contato e logo"
    width-class="w-[330px] max-w-[90vw]"
    :close-on-submit-shortcut="false"
    @update:open="open = $event"
    @submit-shortcut="onSubmitShortcut"
    @cancel-shortcut="onCancelShortcut"
  >
    <template #trigger>
      <UButton
        icon="i-lucide-badge"
        color="neutral"
        variant="ghost"
        size="sm"
        aria-label="Contato e logo"
      />
    </template>

    <div class="clients-contact-popover__form space-y-3">
      <div class="clients-contact-popover__field space-y-1">
        <p class="clients-contact-popover__label text-xs text-[rgb(var(--muted))]">Logo URL</p>
        <UInput
          :model-value="logo"
          class="clients-contact-popover__input clients-contact-popover__input--logo"
          placeholder="https://..."
          @update:model-value="logo = String($event ?? '')"
        />
      </div>

      <div class="clients-contact-popover__field space-y-1">
        <p class="clients-contact-popover__label text-xs text-[rgb(var(--muted))]">Telefone</p>
        <UInput
          :model-value="phone"
          class="clients-contact-popover__input clients-contact-popover__input--phone"
          placeholder="(00) 00000-0000"
          @update:model-value="phone = String($event ?? '')"
        />
      </div>

      <div class="clients-contact-popover__field space-y-1">
        <p class="clients-contact-popover__label text-xs text-[rgb(var(--muted))]">Site</p>
        <UInput
          :model-value="site"
          class="clients-contact-popover__input clients-contact-popover__input--site"
          placeholder="https://site.com"
          @update:model-value="site = String($event ?? '')"
        />
      </div>

      <div class="clients-contact-popover__field space-y-1">
        <p class="clients-contact-popover__label text-xs text-[rgb(var(--muted))]">Endereco</p>
        <UInput
          :model-value="address"
          class="clients-contact-popover__input clients-contact-popover__input--address"
          placeholder="Rua, numero, cidade"
          @update:model-value="address = String($event ?? '')"
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
