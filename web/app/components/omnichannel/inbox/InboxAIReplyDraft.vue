<script setup lang="ts">
import { UButton, UIcon } from '#components'
import type { OmnichannelAIReplyDraft } from '~/composables/omnichannel/useOmnichannelAIReplyDraft'

defineProps<{
  draft: OmnichannelAIReplyDraft | null
  loading: boolean
  dismissing: boolean
  errorMessage: string
  disabled: boolean
  composerHasContent: boolean
}>()

defineEmits<{
  (event: 'use' | 'dismiss'): void
}>()
</script>

<template>
  <section v-if="draft" class="ai-reply-draft" aria-label="Sugestao de resposta da IA">
    <header>
      <span><UIcon name="i-lucide-sparkles" /> Sugestao da IA</span>
      <small>Nada sera enviado sem sua confirmacao.</small>
    </header>
    <p>{{ draft.content }}</p>
    <div class="ai-reply-draft__actions">
      <UButton
        size="xs"
        color="primary"
        icon="i-lucide-pen-line"
        :disabled="disabled || composerHasContent"
        @click="$emit('use')"
      >
        Usar e revisar
      </UButton>
      <UButton
        size="xs"
        color="neutral"
        variant="ghost"
        icon="i-lucide-x"
        :loading="dismissing"
        :disabled="disabled || dismissing"
        @click="$emit('dismiss')"
      >
        Descartar
      </UButton>
      <small v-if="composerHasContent">Limpe o campo atual para usar a sugestao.</small>
    </div>
  </section>
  <div v-else-if="loading" class="ai-reply-draft ai-reply-draft--loading">
    <UIcon name="i-lucide-loader-circle" class="animate-spin" /> Buscando sugestao da IA...
  </div>
  <p v-if="errorMessage" class="ai-reply-draft__error" role="alert">{{ errorMessage }}</p>
</template>

<style scoped>
.ai-reply-draft {
  margin: 0 0 8px;
  padding: 10px 12px;
  border: 1px solid color-mix(in srgb, rgb(var(--primary)) 35%, transparent);
  border-radius: 10px;
  background: color-mix(in srgb, rgb(var(--primary)) 8%, rgb(var(--surface)));
  color: rgb(var(--foreground));
}

.ai-reply-draft header,
.ai-reply-draft__actions,
.ai-reply-draft header span {
  display: flex;
  align-items: center;
  gap: 8px;
}

.ai-reply-draft header {
  justify-content: space-between;
  color: rgb(var(--muted));
  font-size: 0.75rem;
}

.ai-reply-draft header span {
  color: rgb(var(--primary));
  font-weight: 700;
}

.ai-reply-draft p {
  margin: 8px 0;
  white-space: pre-wrap;
  font-size: 0.875rem;
  line-height: 1.45;
}

.ai-reply-draft__actions {
  flex-wrap: wrap;
}

.ai-reply-draft__actions small,
.ai-reply-draft--loading {
  color: rgb(var(--muted));
  font-size: 0.75rem;
}

.ai-reply-draft--loading {
  display: flex;
  align-items: center;
  gap: 8px;
}

.ai-reply-draft__error {
  margin: 0 0 8px;
  color: rgb(var(--error));
  font-size: 0.75rem;
}
</style>
