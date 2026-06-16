<script setup lang="ts">
import { computed } from 'vue'

import BioCollapsibleItem from '~/components/bio/BioCollapsibleItem.vue'
import BioMediaField from '~/components/bio/BioMediaField.vue'
import BioSectionCard from '~/components/bio/BioSectionCard.vue'
import type {
  BioData,
  BioLightbox,
  BioMediaKind,
  BioPlacesBounds,
  BioStore,
  BioStoreLocator,
} from '~/domain/bio/types'

// Secao storeLocator + lightbox: lista de lojas (cada uma com nome + lat/lng
// obrigatorios) em GRID de cards lado a lado, os bounds do mapa, a flag de abrir
// por query e o numero de WhatsApp do lightbox.

const props = defineProps<{
  draft: BioData
  bioId: string
  uploadMedia: (kind: BioMediaKind, file: File) => Promise<string | null>
  isUploading: (kind: BioMediaKind) => boolean
}>()

const emit = defineEmits<{ (e: 'update:draft', value: BioData): void }>()

const locator = computed<BioStoreLocator>(() => props.draft.storeLocator || {})
const stores = computed<BioStore[]>(() => props.draft.storeLocator?.stores || [])

// Com varias lojas recolhe por padrao; com uma unica, abre para edicao direta.
const collapseStores = computed(() => stores.value.length > 1)

function storeTitle(store: BioStore, index: number): string {
  const name = String(store.name || '').trim()
  return name || `Loja ${index + 1}`
}
const bounds = computed<BioPlacesBounds>(() => props.draft.storeLocator?.placesBounds || {})
const lightbox = computed<BioLightbox>(() => props.draft.lightbox || {})

function patchLocator(patch: Partial<BioStoreLocator>) {
  emit('update:draft', {
    ...props.draft,
    storeLocator: { ...(props.draft.storeLocator || {}), ...patch },
  })
}

function setOpenOnQuery(value: boolean) {
  patchLocator({ openOnQuery: value })
}

function setBound(key: keyof BioPlacesBounds, value: string) {
  patchLocator({
    placesBounds: { ...(props.draft.storeLocator?.placesBounds || {}), [key]: toNumber(value) },
  })
}

function setLightbox(value: string) {
  emit('update:draft', {
    ...props.draft,
    lightbox: { ...(props.draft.lightbox || {}), whatsappNumber: value },
  })
}

function setStores(next: BioStore[]) {
  patchLocator({ stores: next })
}

function updateStore<K extends keyof BioStore>(index: number, key: K, value: BioStore[K]) {
  const next = stores.value.map((store, position) =>
    position === index ? { ...store, [key]: value } : store,
  )
  setStores(next)
}

function addStore() {
  setStores([...stores.value, { name: '', lat: 0, lng: 0 }])
}

function removeStore(index: number) {
  setStores(stores.value.filter((_, position) => position !== index))
}

function toNumber(value: string): number {
  const parsed = Number(String(value || '').trim())
  return Number.isFinite(parsed) ? parsed : 0
}
</script>

<template>
  <BioSectionCard
    title="Lojas e lightbox"
    description="Localizador de lojas no mapa e numero de WhatsApp do lightbox. Cada loja precisa de nome e coordenadas."
  >
    <div class="bio-subsection">
      <div class="bio-section-grid">
        <div class="bio-field bio-field--switch">
          <label class="bio-field__label">Abrir localizador por query</label>
          <USwitch
            :model-value="locator.openOnQuery ?? false"
            @update:model-value="setOpenOnQuery(Boolean($event))"
          />
        </div>
        <div class="bio-field">
          <label class="bio-field__label">WhatsApp do lightbox</label>
          <UInput
            :model-value="lightbox.whatsappNumber || ''"
            placeholder="5511999999999"
            @update:model-value="setLightbox(String($event ?? ''))"
          />
        </div>
      </div>
    </div>

    <div class="bio-subsection">
      <h3 class="bio-subsection__title">Limites do mapa (bounds)</h3>
      <div class="bio-section-grid bio-section-grid--tight">
        <div class="bio-field">
          <label class="bio-field__label">Sul (south)</label>
          <UInput
            type="number"
            :model-value="bounds.south ?? 0"
            @update:model-value="setBound('south', String($event ?? ''))"
          />
        </div>
        <div class="bio-field">
          <label class="bio-field__label">Oeste (west)</label>
          <UInput
            type="number"
            :model-value="bounds.west ?? 0"
            @update:model-value="setBound('west', String($event ?? ''))"
          />
        </div>
        <div class="bio-field">
          <label class="bio-field__label">Norte (north)</label>
          <UInput
            type="number"
            :model-value="bounds.north ?? 0"
            @update:model-value="setBound('north', String($event ?? ''))"
          />
        </div>
        <div class="bio-field">
          <label class="bio-field__label">Leste (east)</label>
          <UInput
            type="number"
            :model-value="bounds.east ?? 0"
            @update:model-value="setBound('east', String($event ?? ''))"
          />
        </div>
      </div>
    </div>

    <div class="bio-cards">
      <p v-if="!stores.length" class="bio-cards__empty">
        Nenhuma loja ainda. Adicione a primeira abaixo.
      </p>

      <div class="bio-cards__grid">
        <BioCollapsibleItem
          v-for="(store, index) in stores"
          :key="index"
          :title="storeTitle(store, index)"
          :default-open="!collapseStores"
        >
          <template #actions>
            <button
              type="button"
              class="bio-card__remove"
              aria-label="Remover loja"
              @click="removeStore(index)"
            >
              <UIcon name="i-lucide-trash-2" />
            </button>
          </template>

          <div class="bio-card__body">
            <BioMediaField
              label="Imagem da loja"
              kind="store"
              accept="image/*"
              :model-value="store.img || ''"
              :uploading="isUploading('store')"
              :on-upload="uploadMedia"
              @update:model-value="updateStore(index, 'img', String($event ?? ''))"
            />

            <div class="bio-field">
              <label class="bio-field__label">Nome (obrigatorio)</label>
              <UInput
                :model-value="store.name"
                @update:model-value="updateStore(index, 'name', String($event ?? ''))"
              />
            </div>

            <div class="bio-section-grid bio-section-grid--tight">
              <div class="bio-field">
                <label class="bio-field__label">Latitude</label>
                <UInput
                  type="number"
                  :model-value="store.lat"
                  @update:model-value="updateStore(index, 'lat', toNumber(String($event ?? '')))"
                />
              </div>
              <div class="bio-field">
                <label class="bio-field__label">Longitude</label>
                <UInput
                  type="number"
                  :model-value="store.lng"
                  @update:model-value="updateStore(index, 'lng', toNumber(String($event ?? '')))"
                />
              </div>
            </div>

            <div class="bio-field">
              <label class="bio-field__label">Place ID</label>
              <UInput
                :model-value="store.placeId || ''"
                @update:model-value="updateStore(index, 'placeId', String($event ?? ''))"
              />
            </div>
            <div class="bio-section-grid bio-section-grid--tight">
              <div class="bio-field">
                <label class="bio-field__label">Telefone</label>
                <UInput
                  :model-value="store.phone || ''"
                  @update:model-value="updateStore(index, 'phone', String($event ?? ''))"
                />
              </div>
              <div class="bio-field">
                <label class="bio-field__label">Endereco</label>
                <UInput
                  :model-value="store.address || ''"
                  @update:model-value="updateStore(index, 'address', String($event ?? ''))"
                />
              </div>
            </div>
          </div>
        </BioCollapsibleItem>
      </div>

      <UButton
        icon="i-lucide-plus"
        color="neutral"
        variant="soft"
        label="Adicionar loja"
        @click="addStore"
      />
    </div>
  </BioSectionCard>
</template>

<style scoped>
.bio-section-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: 0.6rem 0.85rem;
}

.bio-section-grid--tight {
  grid-template-columns: repeat(auto-fit, minmax(110px, 1fr));
}

.bio-field {
  display: grid;
  gap: 0.3rem;
}

.bio-field--switch {
  align-content: start;
}

.bio-field__label {
  font-size: 0.72rem;
  font-weight: 700;
  color: var(--text-muted);
}

.bio-subsection {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
  padding: 0.85rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-soft);
  background: rgb(var(--surface-2) / 0.35);
}

.bio-subsection__title {
  font-size: 0.85rem;
  font-weight: 700;
  color: var(--text-main);
}

.bio-cards {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.bio-cards__empty {
  margin: 0;
  padding: 0.85rem 1rem;
  border: 1px dashed var(--line-soft);
  border-radius: var(--radius-soft);
  color: var(--text-muted);
  font-size: 0.85rem;
  background: rgb(var(--surface-2) / 0.4);
}

.bio-cards__grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 0.5rem;
}

.bio-card__body {
  display: flex;
  flex-direction: column;
  gap: 0.55rem;
}

.bio-card__remove {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 1.5rem;
  height: 1.5rem;
  border: 1px solid var(--line-soft);
  border-radius: 0.4rem;
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
}

.bio-card__remove:hover {
  color: rgb(var(--danger));
  border-color: rgb(var(--danger) / 0.4);
}
</style>
