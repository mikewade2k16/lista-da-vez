<script setup lang="ts">
import { ref, watch } from 'vue'

import { buildQrDataUrl } from '~/utils/qr-image'

// Miniatura do QR na tabela/cards (mesmo formato da foto do produto no
// cardapio). Clique abre o QR grande; o icone sobre a miniatura baixa o PNG.
// Gerada no cliente a partir do qrUrl rastreado e das cores salvas.
const props = defineProps<{
  slug: string
  qrUrl: string
  fillColor: string
  backColor: string
  isActive: boolean
  // Lado exibido em px (72 na tabela/lista; maior nos cards).
  size?: number
}>()
const emit = defineEmits<{ view: []; download: [] }>()

const src = ref('')
const side = () => props.size ?? 72

watch(
  () => [props.qrUrl, props.fillColor, props.backColor],
  async () => {
    if (!import.meta.client || !props.qrUrl) {
      src.value = ''
      return
    }
    try {
      // 2x o tamanho exibido para ficar nitido em tela retina.
      src.value = await buildQrDataUrl(props.qrUrl, props.fillColor, props.backColor, side() * 2)
    } catch {
      src.value = ''
    }
  },
  { immediate: true },
)
</script>

<template>
  <div
    class="tools-qr-thumb"
    :class="{ 'is-off': !isActive }"
    :style="{ '--qr-side': `${side()}px` }"
  >
    <button
      type="button"
      class="tools-qr-thumb__main"
      :aria-label="`Ver QR ${slug} em tela cheia`"
      title="Ver QR em tela cheia"
      @click="emit('view')"
    >
      <img
        v-if="src"
        :src="src"
        :alt="`QR ${slug}`"
        :width="side()"
        :height="side()"
        draggable="false"
      />
      <UIcon v-else name="i-lucide-qr-code" class="tools-qr-thumb__icon" />
    </button>
    <span v-if="!isActive" class="tools-qr-thumb__badge">Inativo</span>
    <div v-if="src" class="tools-qr-thumb__tools">
      <button
        type="button"
        :aria-label="`Baixar QR ${slug}`"
        title="Baixar PNG"
        @click.stop="emit('download')"
      >
        <UIcon name="i-lucide-download" />
      </button>
    </div>
  </div>
</template>

<style scoped>
.tools-qr-thumb {
  position: relative;
  width: var(--qr-side, 72px);
  height: var(--qr-side, 72px);
  overflow: hidden;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2) / 0.6);
  transition: border-color 0.12s ease;
}

.tools-qr-thumb:hover {
  border-color: rgb(var(--primary));
}

.tools-qr-thumb__main {
  display: grid;
  place-items: center;
  width: 100%;
  height: 100%;
  padding: 0;
  border: none;
  background: transparent;
  color: var(--text-muted);
  cursor: zoom-in;
}

.tools-qr-thumb__main img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.tools-qr-thumb.is-off img {
  opacity: 0.35;
}

.tools-qr-thumb__icon {
  width: 1.5rem;
  height: 1.5rem;
}

.tools-qr-thumb__badge {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  padding: 0.1rem 0.35rem;
  border-radius: 999px;
  background: rgb(var(--surface) / 0.92);
  color: rgb(var(--danger));
  font-size: 0.62rem;
  font-weight: 700;
  pointer-events: none;
}

/* Icone sobre a miniatura: aparece no hover/foco (desktop) e sempre no toque. */
.tools-qr-thumb__tools {
  position: absolute;
  right: 0.2rem;
  bottom: 0.2rem;
  display: flex;
  gap: 0.2rem;
  opacity: 0;
  transition: opacity 0.12s ease;
}

.tools-qr-thumb:hover .tools-qr-thumb__tools,
.tools-qr-thumb:focus-within .tools-qr-thumb__tools {
  opacity: 1;
}

@media (hover: none) {
  .tools-qr-thumb__tools {
    opacity: 1;
  }
}

.tools-qr-thumb__tools button {
  display: grid;
  place-items: center;
  width: 1.4rem;
  height: 1.4rem;
  padding: 0;
  border: none;
  border-radius: 999px;
  background: rgb(var(--surface) / 0.88);
  color: rgb(var(--text));
  font-size: 0.75rem;
  cursor: pointer;
}
</style>
