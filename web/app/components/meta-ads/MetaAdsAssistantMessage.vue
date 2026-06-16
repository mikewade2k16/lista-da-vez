<script setup lang="ts">
import { computed } from 'vue'
import type { MetaAdsAssistantMessage } from '~/types/meta-ads'

const props = defineProps<{ message: MetaAdsAssistantMessage }>()

interface Segment {
  type: 'text' | 'image'
  value: string
}

// Detecta URLs de imagem (por extensao) ou do CDN da Meta (fbcdn/scontent) e
// quebra o conteudo em segmentos texto/imagem para renderizar miniaturas inline.
const IMAGE_URL =
  /(https?:\/\/[^\s<>()]+\.(?:png|jpe?g|gif|webp)(?:\?[^\s<>()]*)?|https?:\/\/[^\s<>()]*(?:fbcdn\.net|scontent)[^\s<>()]*)/gi

const segments = computed<Segment[]>(() => {
  const content = props.message.content || ''
  const out: Segment[] = []
  let last = 0
  for (const match of content.matchAll(IMAGE_URL)) {
    const index = match.index ?? 0
    if (index > last) out.push({ type: 'text', value: content.slice(last, index) })
    out.push({ type: 'image', value: match[0] })
    last = index + match[0].length
  }
  if (last < content.length) out.push({ type: 'text', value: content.slice(last) })
  if (out.length === 0) out.push({ type: 'text', value: content })
  return out
})

const timeLabel = computed<string>(() => {
  const date = new Date(props.message.createdAt)
  if (Number.isNaN(date.getTime())) return ''
  return date.toLocaleString('pt-BR', {
    day: '2-digit',
    month: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
})
</script>

<template>
  <div class="ma-msg" :class="message.role === 'user' ? 'ma-msg--user' : 'ma-msg--assistant'">
    <div class="ma-msg__content">
      <template v-for="(seg, i) in segments" :key="i">
        <a
          v-if="seg.type === 'image'"
          :href="seg.value"
          target="_blank"
          rel="noopener noreferrer"
          class="ma-msg__image-link"
        >
          <img :src="seg.value" class="ma-msg__image" alt="Criativo do anuncio" loading="lazy" />
        </a>
        <span v-else class="ma-msg__text">{{ seg.value }}</span>
      </template>
    </div>

    <ul v-if="message.actions.length" class="ma-msg__actions">
      <li
        v-for="(action, index) in message.actions"
        :key="index"
        class="ma-msg__chip"
        :class="action.status === 'ok' ? 'ma-msg__chip--ok' : 'ma-msg__chip--error'"
      >
        <span class="ma-msg__chip-tool">{{ action.tool }}</span>
        <span class="ma-msg__chip-summary">{{ action.summary }}</span>
      </li>
    </ul>

    <time v-if="timeLabel" class="ma-msg__time" :datetime="message.createdAt">{{ timeLabel }}</time>
  </div>
</template>

<style scoped>
.ma-msg {
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
  max-width: 78%;
  padding: 0.65rem 0.9rem;
  border-radius: var(--radius-soft);
  border: 1px solid var(--line-soft);
  font-size: 0.88rem;
  line-height: 1.55;
}

.ma-msg--user {
  align-self: flex-end;
  background: rgb(var(--primary) / 0.12);
  border-color: rgb(var(--primary) / 0.25);
}

.ma-msg--assistant {
  align-self: flex-start;
  background: rgb(var(--surface-2) / 0.55);
}

.ma-msg__content {
  color: var(--text-main);
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.ma-msg__text {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.ma-msg__image-link {
  display: inline-block;
  width: fit-content;
}

.ma-msg__image {
  max-width: 100%;
  max-height: 260px;
  border-radius: var(--radius-soft);
  border: 1px solid var(--line-soft);
  display: block;
}

.ma-msg__time {
  font-size: 0.7rem;
  color: var(--text-muted);
  align-self: flex-end;
}

.ma-msg__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
  list-style: none;
  margin: 0;
  padding: 0;
}

.ma-msg__chip {
  display: inline-flex;
  align-items: baseline;
  gap: 0.4rem;
  font-size: 0.72rem;
  padding: 0.2rem 0.55rem;
  border-radius: 999px;
  border: 1px solid transparent;
  max-width: 100%;
}

.ma-msg__chip--ok {
  color: rgb(var(--success));
  background: rgb(var(--success) / 0.12);
  border-color: rgb(var(--success) / 0.35);
}

.ma-msg__chip--error {
  color: rgb(var(--danger));
  background: rgb(var(--danger) / 0.12);
  border-color: rgb(var(--danger) / 0.35);
}

.ma-msg__chip-tool {
  font-weight: 700;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  white-space: nowrap;
}

.ma-msg__chip-summary {
  color: var(--text-muted);
  overflow-wrap: anywhere;
}
</style>
