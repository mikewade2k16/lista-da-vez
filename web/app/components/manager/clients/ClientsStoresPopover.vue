<script setup lang="ts">
import OmniMoneyInput from '../../../../layers/tasks/components/omni/inputs/OmniMoneyInput.vue'
import OmniMinimalPopover from '~/components/omni/overlay/OmniMinimalPopover.vue'
import type { ClientStoreCharge } from '~/types/clients'

const props = withDefaults(
  defineProps<{
    stores: ClientStoreCharge[]
    disabled?: boolean
    busy?: boolean
  }>(),
  {
    disabled: false,
    busy: false,
  },
)

const emit = defineEmits<{
  save: [stores: ClientStoreCharge[]]
}>()

const open = ref(false)
const draftStores = ref<ClientStoreCharge[]>([])
const newStoreName = ref('')
const newStoreAmount = ref(0)

const storesCountLabel = computed(() => String(draftStores.value.length))

watch(
  () => props.stores,
  (stores) => {
    draftStores.value = stores.map((store) => ({
      id: String(store.id || `store-${Math.random().toString(36).slice(2, 8)}`),
      name: String(store.name || ''),
      amount: Number.isFinite(store.amount) ? store.amount : 0,
    }))
  },
  { immediate: true, deep: true },
)

function normalizeAmount(value: unknown) {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return Math.max(0, Number(value.toFixed(2)))
  }

  const raw = String(value ?? '').trim()
  if (!raw) {
    return 0
  }

  let normalized = raw
    .replace(/\s+/g, '')
    .replace(/^R\$/i, '')
    .replace(/[^\d,.-]/g, '')

  const hasComma = normalized.includes(',')
  const hasDot = normalized.includes('.')

  if (hasComma && hasDot) {
    if (normalized.lastIndexOf(',') > normalized.lastIndexOf('.')) {
      normalized = normalized.replace(/\./g, '').replace(',', '.')
    } else {
      normalized = normalized.replace(/,/g, '')
    }
  } else if (hasComma) {
    normalized = normalized.replace(/\./g, '').replace(',', '.')
  }

  const parsed = Number(normalized)
  if (!Number.isFinite(parsed)) {
    return 0
  }

  return Math.max(0, Number(parsed.toFixed(2)))
}

function updateStoreName(index: number, value: string | number | undefined) {
  const target = draftStores.value[index]
  if (!target) {
    return
  }

  target.name = String(value ?? '').slice(0, 120)
}

function updateStoreAmount(index: number, value: number | string | undefined) {
  const target = draftStores.value[index]
  if (!target) {
    return
  }

  target.amount = normalizeAmount(value)
}

function addStore() {
  if (props.busy) {
    return
  }

  const nextName = newStoreName.value.trim()
  if (!nextName) {
    return
  }

  draftStores.value.push({
    id: `store-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 6)}`,
    name: nextName.slice(0, 120),
    amount: normalizeAmount(newStoreAmount.value),
  })

  newStoreName.value = ''
  newStoreAmount.value = 0
}

function removeStore(index: number) {
  if (props.busy) {
    return
  }

  draftStores.value.splice(index, 1)
}

function save() {
  if (props.busy) {
    return
  }

  const unique = new Set<string>()
  const normalized = draftStores.value
    .map((store) => ({
      id: String(store.id || `store-${Date.now().toString(36)}`),
      name: String(store.name || '').trim(),
      amount: normalizeAmount(store.amount),
    }))
    .filter((store) => {
      if (!store.name) {
        return false
      }

      const key = store.name.toLowerCase()
      if (unique.has(key)) {
        return false
      }

      unique.add(key)
      return true
    })

  emit('save', normalized)
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
    title="Lojas do cliente"
    width-class="w-[320px] max-w-[90vw]"
    :disabled="props.disabled"
    :close-on-submit-shortcut="false"
    @update:open="open = $event"
    @submit-shortcut="onSubmitShortcut"
    @cancel-shortcut="onCancelShortcut"
  >
    <template #trigger>
      <UButton
        icon="i-lucide-store"
        color="neutral"
        variant="ghost"
        size="sm"
        :disabled="props.disabled"
        class="clients-stores-popover__trigger relative"
        :aria-label="`Configurar lojas (${storesCountLabel})`"
      >
        <template #trailing>
          <span class="clients-stores-popover__trigger-count text-xs text-[rgb(var(--muted))]">
            {{ storesCountLabel }}
          </span>
        </template>
      </UButton>
    </template>

    <div class="clients-stores-popover__body space-y-3">
      <div class="clients-stores-popover__add-row grid gap-2 grid-cols-[1fr_140px_auto]">
        <UInput
          :model-value="newStoreName"
          class="clients-stores-popover__new-name"
          placeholder="Nome da loja"
          @update:model-value="newStoreName = String($event ?? '')"
          @keydown.enter.prevent="addStore"
        />
        <OmniMoneyInput
          :model-value="newStoreAmount"
          class="clients-stores-popover__new-amount"
          @update:model-value="newStoreAmount = $event"
        />
        <UButton
          icon="i-lucide-plus"
          color="primary"
          variant="soft"
          :disabled="props.busy"
          @click="addStore"
        />
      </div>

      <div class="clients-stores-popover__list max-h-64 space-y-2 overflow-y-auto pr-1">
        <div
          v-for="(store, index) in draftStores"
          :key="store.id"
          class="clients-stores-popover__item grid gap-2 rounded-[var(--radius-sm)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] p-2 grid-cols-[1fr_140px_auto]"
        >
          <UInput
            :model-value="store.name"
            class="clients-stores-popover__item-name"
            placeholder="Nome"
            @update:model-value="updateStoreName(index, $event)"
          />
          <OmniMoneyInput
            :model-value="store.amount"
            class="clients-stores-popover__item-amount"
            @update:model-value="updateStoreAmount(index, $event)"
          />
          <UButton
            icon="i-lucide-trash-2"
            color="error"
            variant="ghost"
            size="sm"
            :disabled="props.busy"
            data-omni-enter-ignore="true"
            @click="removeStore(index)"
          />
        </div>

        <p
          v-if="draftStores.length === 0"
          class="clients-stores-popover__empty text-xs text-[rgb(var(--muted))]"
        >
          Nenhuma loja cadastrada.
        </p>
      </div>
    </div>

    <template #footer>
      <UButton label="Cancelar" color="neutral" variant="ghost" size="sm" @click="cancel" />
      <UButton
        label="Salvar lojas"
        color="primary"
        size="sm"
        :loading="props.busy"
        :disabled="props.busy"
        @click="save"
      />
    </template>
  </OmniMinimalPopover>
</template>
