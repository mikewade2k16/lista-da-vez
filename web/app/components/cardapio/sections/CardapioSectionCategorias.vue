<script setup lang="ts">
import { computed, ref } from 'vue'

import { useCardapioStore } from '~/stores/cardapio'
import { useUiStore } from '~/stores/ui'
import { getApiErrorMessage } from '~/utils/api-client'
import { slugify } from '~/domain/cardapio/types'
import type { Category } from '~/domain/cardapio/types'
import { resolveMediaUrl } from '~/utils/media'
import { useSortableList } from '~/composables/useSortableList'

const store = useCardapioStore()
const ui = useUiStore()
const config = useRuntimeConfig()
const mediaUrl = (url: string) => resolveMediaUrl(url, String(config.public.apiBase || ''))

const newName = ref('')
const creating = ref(false)
const busyId = ref('')
const editingId = ref('')
const editName = ref('')
// description = subtitulo da categoria; imageUrl = capa; bannerUrl = banner.
const editDescription = ref('')
const editImageUrl = ref('')
const editBannerUrl = ref('')
const uploading = ref(false)
const uploadingBanner = ref(false)
const reordering = ref(false)

const ordered = computed(() => [...store.categories].sort((a, b) => a.sortOrder - b.sortOrder))

async function onCreate() {
  const name = newName.value.trim()
  if (!name || creating.value || !store.restaurantId) {
    return
  }
  creating.value = true
  try {
    await store.createCategory(store.restaurantId, {
      name,
      slug: slugify(name),
      imageUrl: '',
      sortOrder: store.categories.length,
      isActive: true,
    })
    newName.value = ''
    ui.success('Categoria criada.')
  } catch (caught) {
    ui.error(getApiErrorMessage(caught, 'Nao foi possivel criar a categoria.'))
  } finally {
    creating.value = false
  }
}

function startEdit(category: Category) {
  editingId.value = category.id
  editName.value = category.name
  editDescription.value = category.description
  editImageUrl.value = category.imageUrl
  editBannerUrl.value = category.bannerUrl
}

// O PATCH de categoria e full-replace no back (CategoryInput nao e parcial), entao
// todo patch precisa mandar o objeto COMPLETO + os campos alterados — senao zera
// description/imageUrl/bannerUrl/sortOrder/isActive e falha a validacao de nome/slug.
function categoryBody(category: Category, overrides: Record<string, unknown> = {}) {
  return {
    name: category.name,
    slug: category.slug,
    description: category.description,
    imageUrl: category.imageUrl,
    bannerUrl: category.bannerUrl,
    sortOrder: category.sortOrder,
    isActive: category.isActive,
    ...overrides,
  }
}

async function onImageUpload(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file || !store.restaurantId) {
    input.value = ''
    return
  }
  uploading.value = true
  try {
    const url = await store.uploadMedia(store.restaurantId, file)
    if (url) {
      editImageUrl.value = url
    }
  } catch (caught) {
    ui.error(getApiErrorMessage(caught, 'Nao foi possivel enviar a imagem.'))
  } finally {
    uploading.value = false
    input.value = ''
  }
}

// Espelha onImageUpload, gravando o banner (bannerUrl) com estado proprio.
async function onBannerUpload(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file || !store.restaurantId) {
    input.value = ''
    return
  }
  uploadingBanner.value = true
  try {
    const url = await store.uploadMedia(store.restaurantId, file)
    if (url) {
      editBannerUrl.value = url
    }
  } catch (caught) {
    ui.error(getApiErrorMessage(caught, 'Nao foi possivel enviar o banner.'))
  } finally {
    uploadingBanner.value = false
    input.value = ''
  }
}

async function saveEdit(category: Category) {
  const name = editName.value.trim()
  if (!name) {
    return
  }
  busyId.value = category.id
  try {
    await store.patchCategory(
      category.id,
      categoryBody(category, {
        name,
        slug: slugify(name),
        description: editDescription.value.trim(),
        imageUrl: editImageUrl.value.trim(),
        bannerUrl: editBannerUrl.value.trim(),
      }),
    )
    editingId.value = ''
    ui.success('Categoria atualizada.')
  } catch (caught) {
    ui.error(getApiErrorMessage(caught, 'Nao foi possivel atualizar a categoria.'))
  } finally {
    busyId.value = ''
  }
}

async function toggleActive(category: Category) {
  busyId.value = category.id
  try {
    await store.patchCategory(category.id, categoryBody(category, { isActive: !category.isActive }))
  } catch (caught) {
    ui.error(getApiErrorMessage(caught, 'Nao foi possivel alterar o status.'))
  } finally {
    busyId.value = ''
  }
}

async function remove(category: Category) {
  const { confirmed } = (await ui.confirm({
    title: 'Remover categoria',
    message: `Remover a categoria "${category.name}"? Os produtos ficam sem categoria.`,
    confirmLabel: 'Remover',
  })) as { confirmed: boolean }
  if (!confirmed) {
    return
  }
  busyId.value = category.id
  try {
    await store.deleteCategory(category.id)
    ui.success('Categoria removida.')
  } catch (caught) {
    ui.error(getApiErrorMessage(caught, 'Nao foi possivel remover a categoria.'))
  } finally {
    busyId.value = ''
  }
}

// Reordena via drag-n-drop: reordena a lista local, recalcula sortOrder contiguo
// (0..n-1) e faz PATCH SO nos itens cujo sortOrder mudou (full-replace, objeto
// completo + novo sortOrder). Em erro, re-hidrata as categorias do back.
async function onReorder(from: number, to: number) {
  if (reordering.value || from === to) {
    return
  }
  const next = [...ordered.value]
  const moved = next[from]
  if (!moved) {
    return
  }
  next.splice(from, 1)
  next.splice(to, 0, moved)

  const changed = next.filter((category, position) => category.sortOrder !== position)
  if (!changed.length) {
    return
  }

  reordering.value = true
  try {
    for (const category of changed) {
      const position = next.indexOf(category)
      await store.patchCategory(category.id, categoryBody(category, { sortOrder: position }))
    }
  } catch (caught) {
    ui.error(getApiErrorMessage(caught, 'Nao foi possivel reordenar.'))
    if (store.restaurantId) {
      await store.reloadCategories(store.restaurantId)
    }
  } finally {
    reordering.value = false
  }
}

const { itemHandlers, draggingIndex, overIndex } = useSortableList({ onReorder })
</script>

<template>
  <div class="cardapio-cats">
    <form class="cardapio-cats__create" @submit.prevent="onCreate">
      <input
        v-model="newName"
        type="text"
        class="cardapio-cats__input"
        placeholder="Nome da nova categoria"
      />
      <button type="submit" class="cardapio-cats__add" :disabled="creating || !newName.trim()">
        {{ creating ? 'Criando...' : 'Adicionar' }}
      </button>
    </form>

    <p v-if="!ordered.length" class="cardapio-cats__empty">
      Nenhuma categoria ainda. Crie a primeira para organizar os produtos.
    </p>

    <template v-else>
      <p class="cardapio-cats__hint">
        Arraste os cards pela alca para reordenar. O numero #1, #2... e a ordem de exibicao no site
        (esquerda para a direita, de cima para baixo).
        <span v-if="reordering" class="cardapio-cats__saving">Salvando ordem...</span>
      </p>

      <ul class="cardapio-cats__grid" :aria-busy="reordering">
        <li
          v-for="(category, index) in ordered"
          :key="category.id"
          class="cardapio-cats__item"
          :class="{
            'cardapio-cats__item--dragging': draggingIndex === index,
            'cardapio-cats__item--over': overIndex === index && draggingIndex !== index,
          }"
          v-bind="itemHandlers(index)"
        >
          <header class="cardapio-cats__head">
            <span
              class="cardapio-cats__handle"
              role="button"
              tabindex="0"
              aria-label="Arrastar para reordenar"
              title="Arrastar para reordenar"
            >
              &#x2630;
            </span>
            <span class="cardapio-cats__order" title="Ordem de exibicao no site">
              #{{ index + 1 }}
            </span>
            <span class="cardapio-cats__order-label">ordem no site</span>
            <span
              class="cardapio-cats__pill"
              :class="category.isActive ? 'is-on' : 'is-off'"
              role="button"
              tabindex="0"
              :aria-pressed="category.isActive"
              @click="toggleActive(category)"
              @keydown.enter.prevent="toggleActive(category)"
            >
              {{ category.isActive ? 'Ativa' : 'Inativa' }}
            </span>
          </header>

          <div class="cardapio-cats__body">
            <div v-if="editingId === category.id" class="cardapio-cats__edit">
              <input
                v-model="editName"
                type="text"
                class="cardapio-cats__input"
                aria-label="Nome da categoria"
                @keydown.enter="saveEdit(category)"
              />
              <textarea
                v-model="editDescription"
                rows="2"
                class="cardapio-cats__input cardapio-cats__textarea"
                placeholder="Subtitulo (texto curto sob o nome no site)"
                aria-label="Subtitulo da categoria"
              ></textarea>

              <span class="cardapio-cats__media-label">Capa</span>
              <div class="cardapio-cats__media">
                <img
                  v-if="editImageUrl"
                  :src="mediaUrl(editImageUrl)"
                  alt="Capa da categoria"
                  class="cardapio-cats__thumb"
                />
                <input
                  v-model="editImageUrl"
                  type="text"
                  class="cardapio-cats__input"
                  placeholder="URL da capa (opcional)"
                  aria-label="URL da capa"
                />
                <label class="cardapio-cats__upload">
                  <input type="file" accept="image/*" hidden @change="onImageUpload" />
                  {{ uploading ? 'Enviando...' : 'Enviar' }}
                </label>
              </div>

              <span class="cardapio-cats__media-label">Banner</span>
              <span class="cardapio-cats__media-note">
                Ainda nao exibido no site (render do banner por categoria e follow-up). Voce ja pode
                subir a imagem.
              </span>
              <div class="cardapio-cats__media">
                <img
                  v-if="editBannerUrl"
                  :src="mediaUrl(editBannerUrl)"
                  alt="Banner da categoria"
                  class="cardapio-cats__banner"
                />
                <input
                  v-model="editBannerUrl"
                  type="text"
                  class="cardapio-cats__input"
                  placeholder="URL do banner (opcional)"
                  aria-label="URL do banner"
                />
                <label class="cardapio-cats__upload">
                  <input type="file" accept="image/*" hidden @change="onBannerUpload" />
                  {{ uploadingBanner ? 'Enviando...' : 'Enviar' }}
                </label>
              </div>
            </div>
            <div v-else class="cardapio-cats__info">
              <img
                v-if="category.imageUrl"
                :src="mediaUrl(category.imageUrl)"
                alt=""
                class="cardapio-cats__thumb"
              />
              <span class="cardapio-cats__name">{{ category.name }}</span>
              <span class="cardapio-cats__slug">{{ category.slug }}</span>
            </div>
          </div>

          <div class="cardapio-cats__actions">
            <button
              v-if="editingId === category.id"
              type="button"
              class="cardapio-cats__btn"
              :disabled="busyId === category.id"
              @click="saveEdit(category)"
            >
              Salvar
            </button>
            <button v-else type="button" class="cardapio-cats__btn" @click="startEdit(category)">
              Editar
            </button>
            <button
              type="button"
              class="cardapio-cats__btn cardapio-cats__btn--danger"
              :disabled="busyId === category.id"
              @click="remove(category)"
            >
              Remover
            </button>
          </div>
        </li>
      </ul>
    </template>
  </div>
</template>

<style scoped>
.cardapio-cats {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.cardapio-cats__create {
  display: flex;
  gap: 0.6rem;
}

.cardapio-cats__input {
  flex: 1;
  min-width: 0;
  padding: 0.55rem 0.7rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2) / 0.6);
  color: var(--text-main);
  font-size: 0.9rem;
}

.cardapio-cats__input:focus {
  outline: none;
  border-color: rgb(var(--ring));
  box-shadow: 0 0 0 3px rgb(var(--ring) / 0.18);
}

.cardapio-cats__add {
  border: none;
  color: rgb(var(--surface));
  background: linear-gradient(135deg, rgb(var(--primary)), rgb(var(--primary-600)));
  padding: 0.55rem 1.1rem;
  border-radius: var(--radius-sm);
  font-weight: 600;
  font-size: 0.88rem;
  cursor: pointer;
  white-space: nowrap;
}

.cardapio-cats__add:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.cardapio-cats__empty {
  color: var(--text-muted);
  font-size: 0.9rem;
  padding: 1.5rem 0;
  text-align: center;
}

.cardapio-cats__hint {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
  align-items: center;
  color: var(--text-muted);
  font-size: 0.82rem;
}

.cardapio-cats__saving {
  color: rgb(var(--primary));
  font-weight: 600;
}

.cardapio-cats__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(20rem, 1fr));
  gap: 0.6rem;
  list-style: none;
  padding: 0;
  margin: 0;
}

.cardapio-cats__item {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
  padding: 0.7rem 0.85rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface) / 0.6);
  transition:
    border-color 0.12s ease,
    box-shadow 0.12s ease,
    opacity 0.12s ease;
}

.cardapio-cats__item--dragging {
  opacity: 0.55;
  border-color: rgb(var(--primary) / 0.6);
}

.cardapio-cats__item--over {
  border-color: rgb(var(--primary));
  box-shadow: 0 0 0 2px rgb(var(--primary) / 0.35);
}

.cardapio-cats__head {
  display: flex;
  align-items: center;
  gap: 0.55rem;
}

.cardapio-cats__handle {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 1.6rem;
  height: 1.6rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2) / 0.8);
  color: var(--text-muted);
  cursor: grab;
  font-size: 0.9rem;
  line-height: 1;
  flex-shrink: 0;
}

.cardapio-cats__handle:active {
  cursor: grabbing;
}

.cardapio-cats__order {
  font-weight: 700;
  font-size: 0.95rem;
  color: rgb(var(--primary));
  font-variant-numeric: tabular-nums;
}

.cardapio-cats__order-label {
  font-size: 0.7rem;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.cardapio-cats__pill {
  margin-left: auto;
  padding: 0.18rem 0.55rem;
  border-radius: 999px;
  font-size: 0.74rem;
  font-weight: 600;
  cursor: pointer;
}

.cardapio-cats__pill.is-on {
  background: rgb(var(--success) / 0.16);
  color: rgb(var(--success));
}

.cardapio-cats__pill.is-off {
  background: rgb(var(--muted) / 0.16);
  color: var(--text-muted);
}

.cardapio-cats__body {
  flex: 1;
  min-width: 0;
}

.cardapio-cats__edit {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
}

.cardapio-cats__textarea {
  resize: vertical;
  min-height: 2.4rem;
  line-height: 1.35;
}

.cardapio-cats__media-label {
  font-size: 0.72rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--text-muted);
  margin-top: 0.2rem;
}

.cardapio-cats__media-note {
  font-size: 0.72rem;
  line-height: 1.3;
  color: rgb(var(--primary));
  opacity: 0.9;
}

.cardapio-cats__media {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.cardapio-cats__banner {
  width: 4rem;
  height: 2.2rem;
  border-radius: var(--radius-sm);
  object-fit: cover;
  border: 1px solid var(--line-soft);
  flex-shrink: 0;
}

.cardapio-cats__info {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  min-width: 0;
}

.cardapio-cats__thumb {
  width: 2.2rem;
  height: 2.2rem;
  border-radius: var(--radius-sm);
  object-fit: cover;
  border: 1px solid var(--line-soft);
  flex-shrink: 0;
}

.cardapio-cats__upload {
  flex-shrink: 0;
  padding: 0.4rem 0.7rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2) / 0.8);
  color: var(--text-main);
  font-size: 0.82rem;
  font-weight: 600;
  cursor: pointer;
}

.cardapio-cats__name {
  font-weight: 600;
  color: var(--text-main);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.cardapio-cats__slug {
  font-size: 0.8rem;
  color: var(--text-muted);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.cardapio-cats__actions {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.cardapio-cats__btn {
  border: 1px solid var(--line-soft);
  background: rgb(var(--surface-2) / 0.8);
  color: var(--text-main);
  padding: 0.4rem 0.7rem;
  border-radius: var(--radius-sm);
  font-size: 0.82rem;
  font-weight: 600;
  cursor: pointer;
}

.cardapio-cats__btn:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.cardapio-cats__btn--danger {
  color: rgb(var(--danger));
  border-color: rgb(var(--danger) / 0.4);
}
</style>
