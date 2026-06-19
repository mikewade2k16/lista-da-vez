<script setup lang="ts">
import { ref, watch } from 'vue'

import CardapioMoneyInput from '~/components/cardapio/CardapioMoneyInput.vue'
import CardapioProductVariations from '~/components/cardapio/product/CardapioProductVariations.vue'
import CardapioProductAddons from '~/components/cardapio/product/CardapioProductAddons.vue'
import CardapioProductGallery from '~/components/cardapio/product/CardapioProductGallery.vue'
import { useCardapioStore } from '~/stores/cardapio'
import { useUiStore } from '~/stores/ui'
import { getApiErrorMessage } from '~/utils/api-client'
import { slugify } from '~/domain/cardapio/types'
import type { Category } from '~/domain/cardapio/types'
import {
  emptyProductForm,
  formToPayload,
  productToForm,
} from '~/composables/useCardapioProductForm'
import { resolveMediaUrl } from '~/utils/media'

const config = useRuntimeConfig()
const mediaUrl = (url: string) => resolveMediaUrl(url, String(config.public.apiBase || ''))

const props = defineProps<{
  open: boolean
  productId: string
  categories: Category[]
}>()

const emit = defineEmits<{
  (e: 'close' | 'saved'): void
}>()

const store = useCardapioStore()
const ui = useUiStore()

const form = ref(emptyProductForm())
const loading = ref(false)
const saving = ref(false)
const uploadingGallery = ref(false)

const isEditing = () => Boolean(props.productId)

async function hydrate() {
  form.value = emptyProductForm()
  if (!props.productId) {
    return
  }
  loading.value = true
  try {
    const product = await store.loadProduct(props.productId)
    form.value = productToForm(product)
  } catch (caught) {
    ui.error(getApiErrorMessage(caught, 'Nao foi possivel carregar o produto.'))
    emit('close')
  } finally {
    loading.value = false
  }
}

watch(
  () => props.open,
  (open) => {
    if (open) {
      void hydrate()
    }
  },
)

function onNameInput(event: Event) {
  const value = (event.target as HTMLInputElement).value
  form.value.name = value
  if (!form.value.slugTouched) {
    form.value.slug = slugify(value)
  }
}

function onSlugInput(event: Event) {
  form.value.slugTouched = true
  form.value.slug = slugify((event.target as HTMLInputElement).value)
}

async function onGalleryUpload(file: File) {
  if (!store.restaurantId) {
    return
  }
  uploadingGallery.value = true
  try {
    const url = await store.uploadMedia(store.restaurantId, file)
    if (url) {
      form.value.gallery = [...form.value.gallery, url]
    }
  } catch (caught) {
    ui.error(getApiErrorMessage(caught, 'Nao foi possivel enviar a imagem.'))
  } finally {
    uploadingGallery.value = false
  }
}

async function onMainImageUpload(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file || !store.restaurantId) {
    input.value = ''
    return
  }
  try {
    const url = await store.uploadMedia(store.restaurantId, file)
    if (url) {
      form.value.imageUrl = url
    }
  } catch (caught) {
    ui.error(getApiErrorMessage(caught, 'Nao foi possivel enviar a imagem.'))
  } finally {
    input.value = ''
  }
}

async function onSave() {
  if (saving.value) {
    return
  }
  if (!form.value.name.trim()) {
    ui.error('Informe o nome do produto.')
    return
  }
  saving.value = true
  const payload = formToPayload(form.value)
  try {
    if (isEditing()) {
      await store.patchProduct(props.productId, payload)
    } else if (store.restaurantId) {
      await store.createProduct(store.restaurantId, payload)
    }
    ui.success('Produto salvo.')
    emit('saved')
    emit('close')
  } catch (caught) {
    ui.error(getApiErrorMessage(caught, 'Nao foi possivel salvar o produto.'))
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div v-if="open" class="cardapio-pm" role="dialog" aria-modal="true">
    <div class="cardapio-pm__backdrop" @click="emit('close')"></div>
    <div class="cardapio-pm__panel">
      <header class="cardapio-pm__header">
        <h2 class="cardapio-pm__title">{{ isEditing() ? 'Editar produto' : 'Novo produto' }}</h2>
        <button type="button" class="cardapio-pm__close" aria-label="Fechar" @click="emit('close')">
          &times;
        </button>
      </header>

      <div v-if="loading" class="cardapio-pm__loading">Carregando produto...</div>

      <div v-else class="cardapio-pm__body">
        <div class="cardapio-pm__grid">
          <label class="cardapio-pm__field">
            <span class="cardapio-pm__label">Nome</span>
            <input :value="form.name" type="text" class="cardapio-pm__input" @input="onNameInput" />
          </label>
          <label class="cardapio-pm__field">
            <span class="cardapio-pm__label">Slug</span>
            <input :value="form.slug" type="text" class="cardapio-pm__input" @input="onSlugInput" />
          </label>
          <label class="cardapio-pm__field">
            <span class="cardapio-pm__label">Categoria</span>
            <select v-model="form.categoryId" class="cardapio-pm__input">
              <option value="">Sem categoria</option>
              <option v-for="category in categories" :key="category.id" :value="category.id">
                {{ category.name }}
              </option>
            </select>
          </label>
          <label class="cardapio-pm__field">
            <span class="cardapio-pm__label">Preco base</span>
            <CardapioMoneyInput v-model="form.priceCents" />
          </label>
          <label class="cardapio-pm__field cardapio-pm__field--full">
            <span class="cardapio-pm__label">Descricao curta</span>
            <input v-model="form.shortDesc" type="text" class="cardapio-pm__input" />
          </label>
          <label class="cardapio-pm__field cardapio-pm__field--full">
            <span class="cardapio-pm__label">Descricao</span>
            <textarea v-model="form.description" rows="2" class="cardapio-pm__input"></textarea>
          </label>
          <label class="cardapio-pm__field cardapio-pm__field--full">
            <span class="cardapio-pm__label">Texto longo (body)</span>
            <textarea v-model="form.body" rows="3" class="cardapio-pm__input"></textarea>
          </label>
          <div class="cardapio-pm__field cardapio-pm__field--full">
            <span class="cardapio-pm__label">Imagem principal</span>
            <div class="cardapio-pm__media">
              <img
                v-if="form.imageUrl"
                :src="mediaUrl(form.imageUrl)"
                alt="Imagem"
                class="cardapio-pm__thumb"
              />
              <input
                v-model="form.imageUrl"
                type="text"
                class="cardapio-pm__input"
                placeholder="URL"
              />
              <label class="cardapio-pm__upload">
                <input type="file" accept="image/*" hidden @change="onMainImageUpload" />
                Enviar
              </label>
            </div>
          </div>
          <label class="cardapio-pm__field">
            <span class="cardapio-pm__label">Peso</span>
            <input
              v-model="form.weight"
              type="text"
              class="cardapio-pm__input"
              placeholder="350g"
            />
          </label>
          <label class="cardapio-pm__field">
            <span class="cardapio-pm__label">Tempo de preparo</span>
            <input
              v-model="form.cookTime"
              type="text"
              class="cardapio-pm__input"
              placeholder="25min"
            />
          </label>
          <label class="cardapio-pm__field">
            <span class="cardapio-pm__label">Dieta (separe por virgula)</span>
            <input
              v-model="form.diet"
              type="text"
              class="cardapio-pm__input"
              placeholder="vegano, sem gluten"
            />
          </label>
          <label class="cardapio-pm__field">
            <span class="cardapio-pm__label">Alergenos (virgula)</span>
            <input
              v-model="form.allergens"
              type="text"
              class="cardapio-pm__input"
              placeholder="gluten, leite"
            />
          </label>
          <label class="cardapio-pm__field cardapio-pm__field--full">
            <span class="cardapio-pm__label">Tags (virgula)</span>
            <input
              v-model="form.tags"
              type="text"
              class="cardapio-pm__input"
              placeholder="novo, picante"
            />
          </label>
          <div class="cardapio-pm__toggles cardapio-pm__field--full">
            <label class="cardapio-pm__toggle">
              <input v-model="form.isAvailable" type="checkbox" />
              <span>Disponivel</span>
            </label>
            <label class="cardapio-pm__toggle">
              <input v-model="form.isFeatured" type="checkbox" />
              <span>Destaque</span>
            </label>
          </div>
        </div>

        <div class="cardapio-pm__section">
          <CardapioProductVariations v-model="form.variations" />
        </div>
        <div class="cardapio-pm__section">
          <CardapioProductAddons v-model="form.addons" />
        </div>
        <div class="cardapio-pm__section">
          <CardapioProductGallery
            v-model="form.gallery"
            :uploading="uploadingGallery"
            @upload="onGalleryUpload"
          />
        </div>
      </div>

      <footer class="cardapio-pm__footer">
        <button type="button" class="cardapio-pm__btn" :disabled="saving" @click="emit('close')">
          Cancelar
        </button>
        <button
          type="button"
          class="cardapio-pm__btn cardapio-pm__btn--primary"
          :disabled="saving || loading"
          @click="onSave"
        >
          <span v-if="saving" class="cardapio-pm__spinner" aria-hidden="true"></span>
          {{ saving ? 'Salvando...' : 'Salvar produto' }}
        </button>
      </footer>
    </div>
  </div>
</template>

<style scoped>
.cardapio-pm {
  position: fixed;
  inset: 0;
  z-index: 60;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 1.5rem;
}

.cardapio-pm__backdrop {
  position: absolute;
  inset: 0;
  background: rgb(var(--text) / 0.4);
  backdrop-filter: blur(2px);
}

.cardapio-pm__panel {
  position: relative;
  width: 100%;
  max-width: 720px;
  max-height: 90vh;
  display: flex;
  flex-direction: column;
  background: rgb(var(--surface));
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-md);
  overflow: hidden;
}

.cardapio-pm__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1.1rem 1.25rem;
  border-bottom: 1px solid var(--line-soft);
}

.cardapio-pm__title {
  font-size: 1.05rem;
  font-weight: 700;
  color: var(--text-main);
}

.cardapio-pm__close {
  border: none;
  background: transparent;
  font-size: 1.5rem;
  line-height: 1;
  color: var(--text-muted);
  cursor: pointer;
}

.cardapio-pm__loading {
  padding: 2rem 1.25rem;
  color: var(--text-muted);
}

.cardapio-pm__body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 1.25rem;
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}

.cardapio-pm__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.85rem;
}

.cardapio-pm__field {
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
  min-width: 0;
}

.cardapio-pm__field--full {
  grid-column: 1 / -1;
}

.cardapio-pm__label {
  font-size: 0.8rem;
  font-weight: 600;
  color: var(--text-main);
}

.cardapio-pm__input {
  width: 100%;
  padding: 0.55rem 0.7rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2) / 0.6);
  color: var(--text-main);
  font-size: 0.9rem;
  font-family: inherit;
}

.cardapio-pm__input:focus {
  outline: none;
  border-color: rgb(var(--ring));
  box-shadow: 0 0 0 3px rgb(var(--ring) / 0.18);
}

.cardapio-pm__media {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.cardapio-pm__thumb {
  width: 2.4rem;
  height: 2.4rem;
  border-radius: var(--radius-sm);
  object-fit: cover;
  border: 1px solid var(--line-soft);
}

.cardapio-pm__upload {
  flex-shrink: 0;
  padding: 0.5rem 0.7rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2) / 0.8);
  color: var(--text-main);
  font-size: 0.82rem;
  font-weight: 600;
  cursor: pointer;
}

.cardapio-pm__toggles {
  display: flex;
  gap: 1.25rem;
}

.cardapio-pm__toggle {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  font-size: 0.88rem;
  color: var(--text-main);
}

.cardapio-pm__section {
  border-top: 1px solid var(--line-soft);
  padding-top: 1rem;
}

.cardapio-pm__footer {
  display: flex;
  justify-content: flex-end;
  gap: 0.6rem;
  padding: 1rem 1.25rem;
  border-top: 1px solid var(--line-soft);
}

.cardapio-pm__btn {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  padding: 0.6rem 1.1rem;
  border-radius: var(--radius-sm);
  border: 1px solid var(--line-soft);
  background: rgb(var(--surface-2) / 0.8);
  color: var(--text-main);
  font-weight: 600;
  font-size: 0.9rem;
  cursor: pointer;
}

.cardapio-pm__btn:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.cardapio-pm__btn--primary {
  border: none;
  color: rgb(var(--surface));
  background: linear-gradient(135deg, rgb(var(--primary)), rgb(var(--primary-600)));
}

.cardapio-pm__spinner {
  width: 0.85rem;
  height: 0.85rem;
  border-radius: 999px;
  border: 2px solid rgb(var(--surface) / 0.5);
  border-top-color: rgb(var(--surface));
  animation: cardapio-pm-spin 0.7s linear infinite;
}

@keyframes cardapio-pm-spin {
  to {
    transform: rotate(360deg);
  }
}

@media (max-width: 640px) {
  .cardapio-pm__grid {
    grid-template-columns: 1fr;
  }
}
</style>
