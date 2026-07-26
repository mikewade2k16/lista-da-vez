<script setup lang="ts">
import OmniEntityDrawer from '~/components/ui/OmniEntityDrawer.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import type { OfflineInteractionCreateDescriptor } from '~/domain/customer-data/offline-interaction-types'

const props = defineProps<{
  open: boolean
  descriptor: OfflineInteractionCreateDescriptor
  saving: boolean
}>()
const emit = defineEmits<{
  'update:open': [value: boolean]
  save: [
    input: {
      interactionType: string
      title: string
      content: string
      occurredAt: string
      timezone: string
      purposeKey: string
      attachmentRefs: string[]
    },
  ]
}>()

const interactionType = ref('')
const title = ref('')
const content = ref('')
const occurredAt = ref('')
const timezone = ref('')
const purposeKey = ref('')

watch(
  () => props.open,
  (open) => {
    if (!open) return
    interactionType.value = props.descriptor.interactionTypes[0]?.value ?? ''
    title.value = ''
    content.value = ''
    occurredAt.value = ''
    timezone.value = props.descriptor.timezoneOptions[0]?.value ?? ''
    purposeKey.value = props.descriptor.purposeOptions[0]?.value ?? ''
  },
)

const valid = computed(
  () =>
    Boolean(interactionType.value) &&
    Boolean(title.value.trim()) &&
    Boolean(content.value.trim()) &&
    Boolean(occurredAt.value) &&
    Boolean(timezone.value) &&
    Boolean(purposeKey.value),
)

function submit(): void {
  if (!valid.value || props.saving) return
  emit('save', {
    interactionType: interactionType.value,
    title: title.value.trim(),
    content: content.value.trim(),
    occurredAt: new Date(occurredAt.value).toISOString(),
    timezone: timezone.value,
    purposeKey: purposeKey.value,
    attachmentRefs: [],
  })
}
</script>

<template>
  <OmniEntityDrawer
    :model-value="open"
    title="Registrar interacao offline"
    subtitle="Dado deterministico do Customer Data; independe da IA."
    @update:model-value="emit('update:open', $event)"
  >
    <form class="offline-form" @submit.prevent="submit">
      <AppSelectField
        v-model="interactionType"
        label="Tipo"
        :options="descriptor.interactionTypes"
      />
      <label>
        Titulo
        <input v-model="title" type="text" :maxlength="descriptor.maxTitleLength" />
      </label>
      <label>
        Conteudo
        <textarea v-model="content" :maxlength="descriptor.maxContentLength"></textarea>
      </label>
      <label>
        Quando ocorreu
        <input v-model="occurredAt" type="datetime-local" />
      </label>
      <AppSelectField
        v-model="timezone"
        label="Timezone"
        :options="descriptor.timezoneOptions"
        searchable
      />
      <AppSelectField
        v-model="purposeKey"
        label="Finalidade"
        :options="descriptor.purposeOptions"
      />
    </form>
    <template #footer>
      <button type="button" :disabled="!valid || saving" @click="submit">
        {{ saving ? 'Salvando...' : 'Registrar' }}
      </button>
    </template>
  </OmniEntityDrawer>
</template>

<style scoped>
.offline-form,
.offline-form label {
  display: grid;
  gap: 0.7rem;
}

.offline-form label {
  gap: 0.3rem;
  color: rgb(var(--muted));
  font-size: 0.72rem;
  font-weight: 700;
}

.offline-form input,
.offline-form textarea {
  min-height: 2.4rem;
  border: 1px solid rgb(var(--border));
  border-radius: 0.65rem;
  background: rgb(var(--surface));
  color: inherit;
}

.offline-form textarea {
  min-height: 8rem;
  resize: vertical;
}
</style>
