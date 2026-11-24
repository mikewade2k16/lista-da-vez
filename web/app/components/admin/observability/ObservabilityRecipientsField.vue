<script setup lang="ts">
import { parseRecipients } from '~/utils/observability/settings'

// Lista editavel de e-mails: digite ou cole (virgula, espaco ou quebra de linha) e
// confirme com Enter ou "Adicionar".
const props = defineProps<{ label: string; hint?: string; error?: string }>()
const model = defineModel<string[]>({ required: true })

const draft = ref('')
const draftError = ref('')

function add() {
  const parsed = parseRecipients(draft.value, model.value)
  if (parsed.valid.length) model.value = [...model.value, ...parsed.valid]
  draftError.value = parsed.invalid.length ? `E-mail inválido: ${parsed.invalid.join(', ')}` : ''
  draft.value = parsed.invalid.join(' ')
}

function remove(email: string) {
  model.value = model.value.filter((item) => item !== email)
}

const shownError = computed(() => draftError.value || props.error || '')
</script>

<template>
  <div class="obs-recipients">
    <label class="obs-field__label">{{ label }}</label>
    <div v-if="model.length" class="obs-recipients__list">
      <span v-for="email in model" :key="email" class="obs-chip obs-recipients__chip">
        {{ email }}
        <button
          type="button"
          class="obs-recipients__remove"
          :aria-label="`Remover ${email}`"
          @click="remove(email)"
        >
          <UIcon name="i-lucide-x" />
        </button>
      </span>
    </div>
    <div class="obs-toolbar">
      <UInput
        v-model="draft"
        type="email"
        size="sm"
        placeholder="nome@empresa.com"
        class="obs-recipients__input"
        @keydown.enter.prevent="add"
      />
      <UButton
        label="Adicionar"
        icon="i-lucide-plus"
        size="sm"
        color="neutral"
        variant="soft"
        :disabled="!draft.trim()"
        @click="add"
      />
    </div>
    <p v-if="shownError" class="obs-hint obs-text--bad">{{ shownError }}</p>
    <p v-else-if="hint" class="obs-hint">{{ hint }}</p>
  </div>
</template>

<style scoped>
.obs-recipients {
  display: grid;
  gap: 0.4rem;
}

.obs-recipients__list {
  display: flex;
  flex-wrap: wrap;
  gap: 0.3rem;
}

.obs-recipients__chip {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  margin: 0;
  font-size: 0.78rem;
}

.obs-recipients__remove {
  display: inline-flex;
  padding: 0;
  border: 0;
  background: none;
  color: rgb(var(--muted));
  cursor: pointer;
}

.obs-recipients__input {
  width: min(22rem, 100%);
}
</style>
