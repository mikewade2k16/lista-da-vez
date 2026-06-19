<script setup lang="ts">
import { computed, ref } from 'vue'

import { useCardapioStore } from '~/stores/cardapio'
import { useUiStore } from '~/stores/ui'
import { getApiErrorMessage } from '~/utils/api-client'
import { slugify } from '~/domain/cardapio/types'
import type { Category } from '~/domain/cardapio/types'

const store = useCardapioStore()
const ui = useUiStore()

const newName = ref('')
const creating = ref(false)
const busyId = ref('')
const editingId = ref('')
const editName = ref('')

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
}

// O PATCH de categoria e full-replace no back (CategoryInput nao e parcial), entao
// todo patch precisa mandar o objeto COMPLETO + os campos alterados — senao zera
// description/sortOrder/isActive e falha a validacao de nome/slug.
function categoryBody(category: Category, overrides: Record<string, unknown> = {}) {
  return {
    name: category.name,
    slug: category.slug,
    description: category.description,
    sortOrder: category.sortOrder,
    isActive: category.isActive,
    ...overrides,
  }
}

async function saveEdit(category: Category) {
  const name = editName.value.trim()
  if (!name) {
    return
  }
  busyId.value = category.id
  try {
    await store.patchCategory(category.id, categoryBody(category, { name, slug: slugify(name) }))
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

async function move(index: number, direction: -1 | 1) {
  const list = ordered.value
  const target = index + direction
  if (target < 0 || target >= list.length) {
    return
  }
  const current = list[index]
  const swap = list[target]
  if (!current || !swap) {
    return
  }
  busyId.value = current.id
  try {
    await store.patchCategory(current.id, categoryBody(current, { sortOrder: swap.sortOrder }))
    await store.patchCategory(swap.id, categoryBody(swap, { sortOrder: current.sortOrder }))
  } catch (caught) {
    ui.error(getApiErrorMessage(caught, 'Nao foi possivel reordenar.'))
  } finally {
    busyId.value = ''
  }
}
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

    <ul v-else class="cardapio-cats__list">
      <li v-for="(category, index) in ordered" :key="category.id" class="cardapio-cats__item">
        <div class="cardapio-cats__order">
          <button
            type="button"
            class="cardapio-cats__arrow"
            :disabled="index === 0 || busyId === category.id"
            aria-label="Subir"
            @click="move(index, -1)"
          >
            &uarr;
          </button>
          <button
            type="button"
            class="cardapio-cats__arrow"
            :disabled="index === ordered.length - 1 || busyId === category.id"
            aria-label="Descer"
            @click="move(index, 1)"
          >
            &darr;
          </button>
        </div>

        <div class="cardapio-cats__body">
          <input
            v-if="editingId === category.id"
            v-model="editName"
            type="text"
            class="cardapio-cats__input"
            @keydown.enter="saveEdit(category)"
          />
          <template v-else>
            <span class="cardapio-cats__name">{{ category.name }}</span>
            <span class="cardapio-cats__slug">{{ category.slug }}</span>
          </template>
        </div>

        <div class="cardapio-cats__actions">
          <span
            class="cardapio-cats__pill"
            :class="category.isActive ? 'is-on' : 'is-off'"
            @click="toggleActive(category)"
          >
            {{ category.isActive ? 'Ativa' : 'Inativa' }}
          </span>
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

.cardapio-cats__list {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  list-style: none;
}

.cardapio-cats__item {
  display: flex;
  align-items: center;
  gap: 0.85rem;
  padding: 0.65rem 0.85rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface) / 0.6);
}

.cardapio-cats__order {
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
}

.cardapio-cats__arrow {
  width: 1.5rem;
  height: 1.1rem;
  border: 1px solid var(--line-soft);
  border-radius: 0.4rem;
  background: rgb(var(--surface-2) / 0.8);
  color: var(--text-main);
  cursor: pointer;
  font-size: 0.7rem;
  line-height: 1;
}

.cardapio-cats__arrow:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.cardapio-cats__body {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: baseline;
  gap: 0.6rem;
}

.cardapio-cats__name {
  font-weight: 600;
  color: var(--text-main);
}

.cardapio-cats__slug {
  font-size: 0.8rem;
  color: var(--text-muted);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
}

.cardapio-cats__actions {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.cardapio-cats__pill {
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
