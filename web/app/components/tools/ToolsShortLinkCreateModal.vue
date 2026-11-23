<script setup lang="ts">
import { reactive, ref, watch } from 'vue'

import type { CreateShortLinkPayload, ShortLinkItem } from '~/types/tools'

interface ClientOption {
  label: string
  value: string
}

// Criar link curto: destino obrigatorio, slug opcional (vazio gera codigo) e,
// para admin, a conta dona.
const props = defineProps<{
  canChooseClient: boolean
  clientOptions: ClientOption[]
  activeClientLabel: string
  activeClientId: string
  saving: boolean
  create: (payload: CreateShortLinkPayload) => Promise<ShortLinkItem | null>
  errorText: () => string
}>()
const open = defineModel<boolean>('open', { required: true })
const emit = defineEmits<{ created: [item: ShortLinkItem] }>()

const form = reactive({ targetUrl: '', slug: '', clientId: '' })
const error = ref('')

watch(open, (value) => {
  if (!value) return
  Object.assign(form, {
    targetUrl: '',
    slug: '',
    clientId: props.canChooseClient ? props.activeClientId : '',
  })
  error.value = ''
})

async function submit() {
  error.value = ''
  if (!form.targetUrl.trim()) {
    error.value = 'Informe a URL de destino.'
    return
  }
  const created = await props.create({
    targetUrl: form.targetUrl,
    slug: form.slug,
    accountId: props.canChooseClient ? form.clientId : undefined,
  })
  if (!created) {
    error.value = props.errorText()
    return
  }
  emit('created', created)
  open.value = false
}
</script>

<template>
  <UModal v-model:open="open" title="Novo link curto" :ui="{ content: 'max-w-xl' }">
    <template #body>
      <form class="tools-create" @submit.prevent="submit">
        <UFormField label="URL de destino">
          <UInput v-model="form.targetUrl" placeholder="https://..." autofocus />
        </UFormField>
        <UFormField
          label="Slug (opcional)"
          help="Vazio gera um codigo aleatorio. Se ja existir, um sufixo e adicionado."
        >
          <UInput v-model="form.slug" placeholder="promo2026" />
        </UFormField>
        <UFormField label="Conta dona">
          <USelect
            v-if="canChooseClient"
            v-model="form.clientId"
            :items="clientOptions"
            placeholder="Selecionar conta"
          />
          <UInput v-else :model-value="activeClientLabel" disabled />
        </UFormField>
        <UAlert
          v-if="error"
          color="error"
          variant="soft"
          icon="i-lucide-alert-triangle"
          :description="error"
        />
        <button type="submit" hidden></button>
      </form>
    </template>
    <template #footer>
      <div class="tools-create__footer">
        <UButton label="Fechar" color="neutral" variant="ghost" @click="open = false" />
        <UButton
          label="Encurtar"
          icon="i-lucide-scissors"
          color="primary"
          :loading="saving"
          :disabled="saving"
          @click="submit"
        />
      </div>
    </template>
  </UModal>
</template>

<style scoped>
.tools-create {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.tools-create__footer {
  display: flex;
  justify-content: flex-end;
  gap: 0.5rem;
  width: 100%;
}
</style>
