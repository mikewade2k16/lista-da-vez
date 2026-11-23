<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'

import type { QrCodeItem } from '~/types/tools'
import { buildQrDataUrl } from '~/utils/qr-image'

// QR em tela cheia (mesmo modal da foto do produto no cardapio): fundo
// desfocado, slug e acoes flutuando no topo, link e destino no rodape.
// Fecha com Esc, no X ou clicando fora. Teleportado para ficar acima do app.
const props = defineProps<{
  item: QrCodeItem
  canManage: boolean
  toggling: boolean
}>()
const emit = defineEmits<{
  close: []
  copy: []
  download: []
  edit: []
  toggle: [value: boolean]
}>()

const src = ref('')

watch(
  () => [props.item.qrUrl, props.item.fillColor, props.item.backColor],
  async () => {
    try {
      src.value = await buildQrDataUrl(
        props.item.qrUrl,
        props.item.fillColor,
        props.item.backColor,
        720,
      )
    } catch {
      src.value = ''
    }
  },
  { immediate: true },
)

function onKey(event: KeyboardEvent) {
  if (event.key === 'Escape') emit('close')
}

onMounted(() => window.addEventListener('keydown', onKey))
onBeforeUnmount(() => window.removeEventListener('keydown', onKey))
</script>

<template>
  <Teleport to="body">
    <div class="tools-qr-viewer" role="dialog" aria-modal="true" :aria-label="`QR ${item.slug}`">
      <header class="tools-qr-viewer__bar" @click.self="emit('close')">
        <p class="tools-qr-viewer__name">{{ item.slug }}</p>
        <div class="tools-qr-viewer__actions">
          <UButton
            icon="i-lucide-copy"
            label="Copiar link"
            color="neutral"
            variant="soft"
            @click="emit('copy')"
          />
          <UButton
            icon="i-lucide-download"
            label="Baixar"
            color="neutral"
            variant="soft"
            @click="emit('download')"
          />
          <UButton
            v-if="canManage"
            icon="i-lucide-pencil"
            label="Editar"
            color="neutral"
            variant="soft"
            @click="emit('edit')"
          />
          <UButton
            icon="i-lucide-x"
            color="neutral"
            variant="ghost"
            aria-label="Fechar"
            title="Fechar (Esc)"
            @click="emit('close')"
          />
        </div>
      </header>

      <div class="tools-qr-viewer__stage" @click.self="emit('close')">
        <img v-if="src" :src="src" :alt="`QR ${item.slug}`" :class="{ 'is-off': !item.isActive }" />
      </div>

      <footer class="tools-qr-viewer__info">
        <USwitch
          v-if="canManage"
          size="sm"
          :label="item.isActive ? 'Ativo' : 'Inativo'"
          :model-value="item.isActive"
          :loading="toggling"
          :disabled="toggling"
          @update:model-value="emit('toggle', Boolean($event))"
        />
        <span v-else :class="item.isActive ? '' : 'tools-qr-viewer__off'">
          {{ item.isActive ? 'Ativo' : 'Inativo' }}
        </span>
        <span class="tools-qr-viewer__text">{{ item.qrUrl }}</span>
        <span class="tools-qr-viewer__text tools-qr-viewer__muted">
          {{ item.targetUrl }} · {{ item.scanCount }} scan(s)
        </span>
        <span v-if="!item.isActive" class="tools-qr-viewer__off">
          Desativado: quem escanear vê "pagina nao encontrada".
        </span>
      </footer>
    </div>
  </Teleport>
</template>

<style scoped>
.tools-qr-viewer {
  position: fixed;
  inset: 0;
  z-index: 700;
  background: rgb(0 0 0 / 0.55);
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
}

.tools-qr-viewer__bar {
  position: absolute;
  top: 0.75rem;
  right: 0.75rem;
  left: 0.75rem;
  z-index: 2;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.tools-qr-viewer__name,
.tools-qr-viewer__actions,
.tools-qr-viewer__info {
  border: 1px solid rgb(255 255 255 / 0.12);
  background: rgb(var(--surface) / 0.78);
  box-shadow: var(--shadow-md);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
}

.tools-qr-viewer__name {
  min-width: 0;
  margin: 0;
  padding: 0.45rem 0.95rem;
  overflow: hidden;
  border-radius: 999px;
  color: var(--text-main);
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tools-qr-viewer__actions {
  display: flex;
  flex-shrink: 0;
  align-items: center;
  gap: 0.25rem;
  padding: 0.25rem;
  border-radius: 999px;
}

.tools-qr-viewer__actions :deep(button) {
  border-radius: 999px;
}

.tools-qr-viewer__stage {
  position: absolute;
  inset: 0;
}

/* QR centralizado e inteiro, com margem para a barra (topo) e o rodape. */
.tools-qr-viewer__stage img {
  position: absolute;
  inset: 4.25rem 1.5rem 7rem;
  margin: auto;
  max-width: calc(100% - 3rem);
  max-height: calc(100% - 11.25rem);
  object-fit: contain;
  border-radius: var(--radius-md);
  box-shadow: 0 20px 60px rgb(0 0 0 / 0.5);
}

.tools-qr-viewer__stage img.is-off {
  opacity: 0.45;
}

.tools-qr-viewer__info {
  position: absolute;
  right: 0.75rem;
  bottom: 0.75rem;
  left: 0.75rem;
  z-index: 2;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.2rem;
  margin: 0 auto;
  max-width: 40rem;
  padding: 0.55rem 0.95rem;
  border-radius: var(--radius-md);
  color: var(--text-main);
  font-size: 0.82rem;
  text-align: center;
}

.tools-qr-viewer__text {
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tools-qr-viewer__muted {
  color: var(--text-muted);
}

.tools-qr-viewer__off {
  color: rgb(var(--danger));
  font-weight: 600;
}

/* Mobile: botoes so com icone; QR quase de borda a borda. */
@media (max-width: 720px) {
  .tools-qr-viewer__bar {
    top: 0.5rem;
    right: 0.5rem;
    left: 0.5rem;
  }

  .tools-qr-viewer__actions :deep(.truncate),
  .tools-qr-viewer__actions :deep([data-slot='label']) {
    display: none;
  }

  .tools-qr-viewer__stage img {
    inset: 3.75rem 0.4rem 7.5rem;
    max-width: calc(100% - 0.8rem);
    max-height: calc(100% - 11.25rem);
  }

  .tools-qr-viewer__info {
    right: 0.5rem;
    bottom: 0.5rem;
    left: 0.5rem;
  }
}
</style>
