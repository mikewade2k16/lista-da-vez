<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'

import { useCardapioStore } from '~/stores/cardapio'
import { useUiStore } from '~/stores/ui'
import { getApiErrorMessage } from '~/utils/api-client'
import { resolveMediaUrl } from '~/utils/media'
import type { ProductPhotoReview } from '~/domain/cardapio/types'

// Lembrete SO do admin, compacto na barra superior do editor: icone + contador;
// o clique abre a lista. Fotos de produto cujo original passou de 15MB (o site ja
// usa a versao convertida; o original fica guardado para ser tratado depois).
// Fonte: cardapio.product_photos (GET .../product-photos/large).
const store = useCardapioStore()
const ui = useUiStore()
const config = useRuntimeConfig()
const apiBase = computed(() => String(config.public.apiBase || ''))

const photos = ref<ProductPhotoReview[]>([])
const busyId = ref('')

async function load() {
  const id = store.restaurantId
  if (!id) return
  try {
    photos.value = await store.loadLargeProductPhotos(id)
  } catch {
    // Lembrete opcional: falha de leitura nao atrapalha a aba.
    photos.value = []
  }
}

async function markReviewed(photo: ProductPhotoReview) {
  if (busyId.value || !store.restaurantId) return
  busyId.value = photo.id
  try {
    await store.markProductPhotoReviewed(store.restaurantId, photo.id)
    await load()
    ui.success('Foto marcada como tratada.')
  } catch (caught) {
    ui.error(getApiErrorMessage(caught, 'Nao foi possivel marcar a foto.'))
  } finally {
    busyId.value = ''
  }
}

function megabytes(bytes: number) {
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

watch(() => store.restaurantId, load)
onMounted(load)
</script>

<template>
  <UPopover v-if="photos.length" :content="{ align: 'end' }">
    <button
      type="button"
      class="cardapio-large-photos__trigger"
      :title="`${photos.length} foto(s) com original acima de 15 MB para tratar`"
      :aria-label="`${photos.length} foto(s) com original acima de 15 MB para tratar`"
    >
      <UIcon name="i-lucide-image-up" />
      <span>{{ photos.length }}</span>
    </button>
    <template #content>
      <div class="cardapio-large-photos">
        <p class="cardapio-large-photos__head">
          <strong>{{ photos.length }}</strong>
          foto(s) com original acima de 15 MB. O site ja usa a versao otimizada; o original fica
          guardado para tratar.
          <UBadge color="warning" variant="soft" size="sm">Admin</UBadge>
        </p>
        <ul class="cardapio-large-photos__list">
          <li v-for="photo in photos" :key="photo.id">
            <img
              :src="resolveMediaUrl(photo.thumbUrl || photo.url, apiBase)"
              alt=""
              loading="lazy"
              width="40"
              height="40"
            />
            <span class="cardapio-large-photos__name">
              {{ photo.productNames.join(', ') || 'Sem produto vinculado' }}
            </span>
            <span class="cardapio-large-photos__size">{{ megabytes(photo.originalBytes) }}</span>
            <UButton
              :to="resolveMediaUrl(photo.originalUrl, apiBase)"
              target="_blank"
              icon="i-lucide-download"
              label="Original"
              color="neutral"
              variant="soft"
              size="xs"
            />
            <UButton
              icon="i-lucide-check"
              label="Tratada"
              color="neutral"
              variant="ghost"
              size="xs"
              :loading="busyId === photo.id"
              @click="markReviewed(photo)"
            />
          </li>
        </ul>
      </div>
    </template>
  </UPopover>
</template>

<style scoped>
.cardapio-large-photos__trigger {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  padding: 0.3rem 0.55rem;
  border: 1px solid rgb(var(--warning) / 0.45);
  border-radius: 999px;
  background: rgb(var(--warning) / 0.1);
  color: rgb(var(--warning));
  font-size: 0.78rem;
  font-weight: 700;
  cursor: pointer;
}

.cardapio-large-photos {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
  width: min(30rem, 90vw);
  max-height: 60vh;
  overflow-y: auto;
  padding: 0.85rem;
  font-size: 0.85rem;
}

.cardapio-large-photos__head {
  margin: 0;
  color: var(--text-main);
}

.cardapio-large-photos__list {
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
  margin: 0;
  padding: 0;
  list-style: none;
}

.cardapio-large-photos__list li {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.cardapio-large-photos__list img {
  width: 40px;
  height: 40px;
  border-radius: var(--radius-sm);
  object-fit: cover;
}

.cardapio-large-photos__name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.cardapio-large-photos__size {
  color: var(--text-muted);
  font-variant-numeric: tabular-nums;
}
</style>
