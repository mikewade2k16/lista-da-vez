<script setup lang="ts">
import { computed, ref, watch } from 'vue'

import type { ShortLinkItem } from '~/types/tools'

// Renomear o slug de um link curto: o link atual para de funcionar, entao exige
// confirmacao explicita (a API tambem exige confirmSlugChange). Para pausar o
// link sem trocar o endereco, o caminho e o toggle Ativo.
const props = defineProps<{
  item: ShortLinkItem | null
  saving: boolean
  rename: (id: string, slug: string) => Promise<{ item: ShortLinkItem | null; error: string }>
}>()
const open = defineModel<boolean>('open', { required: true })
const emit = defineEmits<{ renamed: [item: ShortLinkItem] }>()

const slug = ref('')
const confirmed = ref(false)
const error = ref('')

watch(open, (value) => {
  if (!value) return
  slug.value = props.item?.slug ?? ''
  confirmed.value = false
  error.value = ''
})

const canSubmit = computed(
  () =>
    Boolean(props.item) &&
    confirmed.value &&
    slug.value.trim() !== '' &&
    slug.value.trim() !== props.item?.slug &&
    !props.saving,
)

function dateLabel(value: string) {
  const date = new Date(value)
  return value && Number.isFinite(date.getTime()) ? date.toLocaleString('pt-BR') : ''
}

async function submit() {
  if (!canSubmit.value || !props.item) return
  error.value = ''
  const result = await props.rename(props.item.id, slug.value.trim())
  if (!result.item) {
    error.value = result.error
    return
  }
  emit('renamed', result.item)
  open.value = false
}
</script>

<template>
  <UModal
    v-model:open="open"
    title="Renomear slug"
    description="O link atual deixa de funcionar assim que o slug muda."
    :ui="{ content: 'max-w-xl' }"
  >
    <template #body>
      <div v-if="item" class="tools-rename">
        <div>
          <p class="tools-rename__label">Link atual</p>
          <p class="tools-rename__url">{{ item.shortUrl }}</p>
          <p class="tools-rename__hint">
            {{ item.hits }} clique(s)
            <template v-if="item.lastHitAt">, ultimo em {{ dateLabel(item.lastHitAt) }}</template>
            .
          </p>
        </div>
        <UFormField
          label="Novo slug"
          help="Se ja existir, um sufixo e adicionado. O link final aparece depois de salvar."
        >
          <UInput v-model="slug" placeholder="novo-slug" />
        </UFormField>
        <UAlert
          color="warning"
          variant="soft"
          icon="i-lucide-triangle-alert"
          description="Quem tiver o link atual (posts, mensagens, materiais impressos) vai cair em pagina nao encontrada. Para pausar sem trocar o endereco, use o Ativo."
        />
        <UCheckbox v-model="confirmed" label="Entendo que o link atual vai parar de funcionar" />
        <UAlert
          v-if="error"
          color="error"
          variant="soft"
          icon="i-lucide-alert-triangle"
          :description="error"
        />
      </div>
    </template>
    <template #footer>
      <div class="tools-rename__footer">
        <UButton label="Cancelar" color="neutral" variant="ghost" @click="open = false" />
        <UButton
          label="Trocar slug"
          icon="i-lucide-pencil-line"
          color="warning"
          :loading="saving"
          :disabled="!canSubmit"
          @click="submit"
        />
      </div>
    </template>
  </UModal>
</template>

<style scoped>
.tools-rename {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.tools-rename__label,
.tools-rename__hint {
  margin: 0;
  color: var(--text-muted);
  font-size: 0.76rem;
}

.tools-rename__url {
  margin: 0.1rem 0;
  overflow-wrap: anywhere;
  color: var(--text-main);
  font-weight: 600;
}

.tools-rename__footer {
  display: flex;
  justify-content: flex-end;
  gap: 0.5rem;
  width: 100%;
}
</style>
