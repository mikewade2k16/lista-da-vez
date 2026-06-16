<script setup lang="ts">
import { computed, ref } from 'vue'

import CardapioProductModal from '~/components/cardapio/product/CardapioProductModal.vue'
import { useCardapioStore } from '~/stores/cardapio'
import { useUiStore } from '~/stores/ui'
import { getApiErrorMessage } from '~/utils/api-client'
import { formatCurrency } from '~/domain/cardapio/types'
import type { ProductListItem } from '~/domain/cardapio/types'

const store = useCardapioStore()
const ui = useUiStore()

const modalOpen = ref(false)
const editingId = ref('')
const busyId = ref('')

interface Group {
  id: string
  name: string
  products: ProductListItem[]
}

const groups = computed<Group[]>(() => {
  const buckets = new Map<string, ProductListItem[]>()

  for (const product of store.products) {
    const key = product.categoryId ?? ''
    const list = buckets.get(key) ?? []
    list.push(product)
    buckets.set(key, list)
  }

  const ordered: Group[] = []
  for (const category of store.categories) {
    const products = buckets.get(category.id)
    if (products?.length) {
      ordered.push({ id: category.id, name: category.name, products })
    }
  }
  const uncategorized = buckets.get('')
  if (uncategorized?.length) {
    ordered.push({ id: '', name: 'Sem categoria', products: uncategorized })
  }
  return ordered
})

function openCreate() {
  editingId.value = ''
  modalOpen.value = true
}

function openEdit(product: ProductListItem) {
  editingId.value = product.id
  modalOpen.value = true
}

async function toggleAvailable(product: ProductListItem) {
  busyId.value = product.id
  try {
    await store.patchProduct(product.id, { isAvailable: !product.isAvailable })
  } catch (caught) {
    ui.error(getApiErrorMessage(caught, 'Nao foi possivel alterar a disponibilidade.'))
  } finally {
    busyId.value = ''
  }
}

async function remove(product: ProductListItem) {
  const { confirmed } = (await ui.confirm({
    title: 'Remover produto',
    message: `Remover o produto "${product.name}"?`,
    confirmLabel: 'Remover',
  })) as { confirmed: boolean }
  if (!confirmed) {
    return
  }
  busyId.value = product.id
  try {
    await store.deleteProduct(product.id)
    ui.success('Produto removido.')
  } catch (caught) {
    ui.error(getApiErrorMessage(caught, 'Nao foi possivel remover o produto.'))
  } finally {
    busyId.value = ''
  }
}
</script>

<template>
  <div class="cardapio-prod">
    <div class="cardapio-prod__head">
      <p class="cardapio-prod__count">{{ store.products.length }} produto(s)</p>
      <button type="button" class="cardapio-prod__add" @click="openCreate">Novo produto</button>
    </div>

    <p v-if="!store.products.length" class="cardapio-prod__empty">
      Nenhum produto ainda. Crie o primeiro para montar o cardapio.
    </p>

    <div v-for="group in groups" :key="group.id || 'none'" class="cardapio-prod__group">
      <h3 class="cardapio-prod__group-title">{{ group.name }}</h3>
      <ul class="cardapio-prod__list">
        <li v-for="product in group.products" :key="product.id" class="cardapio-prod__item">
          <img
            v-if="product.imageUrl"
            :src="product.imageUrl"
            alt=""
            class="cardapio-prod__thumb"
          />
          <div
            v-else
            class="cardapio-prod__thumb cardapio-prod__thumb--empty"
            aria-hidden="true"
          ></div>

          <div class="cardapio-prod__info">
            <div class="cardapio-prod__name-row">
              <span class="cardapio-prod__name">{{ product.name }}</span>
              <span v-if="product.isFeatured" class="cardapio-prod__tag">Destaque</span>
            </div>
            <span class="cardapio-prod__price">{{ formatCurrency(product.priceCents) }}</span>
          </div>

          <div class="cardapio-prod__actions">
            <span
              class="cardapio-prod__pill"
              :class="product.isAvailable ? 'is-on' : 'is-off'"
              @click="toggleAvailable(product)"
            >
              {{ product.isAvailable ? 'Disponivel' : 'Indisponivel' }}
            </span>
            <button type="button" class="cardapio-prod__btn" @click="openEdit(product)">
              Editar
            </button>
            <button
              type="button"
              class="cardapio-prod__btn cardapio-prod__btn--danger"
              :disabled="busyId === product.id"
              @click="remove(product)"
            >
              Remover
            </button>
          </div>
        </li>
      </ul>
    </div>

    <CardapioProductModal
      :open="modalOpen"
      :product-id="editingId"
      :categories="store.categories"
      @close="modalOpen = false"
      @saved="modalOpen = false"
    />
  </div>
</template>

<style scoped>
.cardapio-prod {
  display: flex;
  flex-direction: column;
  gap: 1.1rem;
}

.cardapio-prod__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.cardapio-prod__count {
  font-size: 0.86rem;
  color: var(--text-muted);
}

.cardapio-prod__add {
  border: none;
  color: rgb(var(--surface));
  background: linear-gradient(135deg, rgb(var(--primary)), rgb(var(--primary-600)));
  padding: 0.55rem 1.1rem;
  border-radius: var(--radius-sm);
  font-weight: 600;
  font-size: 0.88rem;
  cursor: pointer;
}

.cardapio-prod__empty {
  color: var(--text-muted);
  font-size: 0.9rem;
  padding: 1.5rem 0;
  text-align: center;
}

.cardapio-prod__group-title {
  font-size: 0.82rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-muted);
  margin-bottom: 0.5rem;
}

.cardapio-prod__list {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  list-style: none;
}

.cardapio-prod__item {
  display: flex;
  align-items: center;
  gap: 0.85rem;
  padding: 0.6rem 0.85rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface) / 0.6);
}

.cardapio-prod__thumb {
  width: 2.6rem;
  height: 2.6rem;
  border-radius: var(--radius-sm);
  object-fit: cover;
  flex-shrink: 0;
  border: 1px solid var(--line-soft);
}

.cardapio-prod__thumb--empty {
  background: rgb(var(--surface-2) / 0.8);
}

.cardapio-prod__info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
}

.cardapio-prod__name-row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.cardapio-prod__name {
  font-weight: 600;
  color: var(--text-main);
}

.cardapio-prod__tag {
  font-size: 0.7rem;
  font-weight: 700;
  padding: 0.1rem 0.45rem;
  border-radius: 999px;
  background: rgb(var(--primary) / 0.16);
  color: rgb(var(--primary));
}

.cardapio-prod__price {
  font-size: 0.84rem;
  color: var(--text-muted);
}

.cardapio-prod__actions {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.cardapio-prod__pill {
  padding: 0.18rem 0.55rem;
  border-radius: 999px;
  font-size: 0.74rem;
  font-weight: 600;
  cursor: pointer;
}

.cardapio-prod__pill.is-on {
  background: rgb(var(--success) / 0.16);
  color: rgb(var(--success));
}

.cardapio-prod__pill.is-off {
  background: rgb(var(--muted) / 0.16);
  color: var(--text-muted);
}

.cardapio-prod__btn {
  border: 1px solid var(--line-soft);
  background: rgb(var(--surface-2) / 0.8);
  color: var(--text-main);
  padding: 0.4rem 0.7rem;
  border-radius: var(--radius-sm);
  font-size: 0.82rem;
  font-weight: 600;
  cursor: pointer;
}

.cardapio-prod__btn:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.cardapio-prod__btn--danger {
  color: rgb(var(--danger));
  border-color: rgb(var(--danger) / 0.4);
}
</style>
