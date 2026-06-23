<script setup lang="ts">
import { computed, ref, watch } from 'vue'

import { useCardapioStore } from '~/stores/cardapio'
import { useUiStore } from '~/stores/ui'
import { getApiErrorMessage } from '~/utils/api-client'
import type { Review, ReviewInput } from '~/domain/cardapio/types'

const store = useCardapioStore()
const ui = useUiStore()

// Escopo ativo: 'establishment' ou o id de um produto.
const ESTABLISHMENT = 'establishment'
const selectedScope = ref<string>(ESTABLISHMENT)
const reviews = ref<Review[]>([])
const loading = ref(false)
const saving = ref(false)
const busyId = ref('')

const isEstablishment = computed(() => selectedScope.value === ESTABLISHMENT)

// Nome do produto por id, para resolver o badge "do produto" no escopo estabelecimento.
const productNameById = computed(() => {
  const map = new Map<string, string>()
  for (const product of store.products) {
    map.set(product.id, product.name)
  }
  return map
})

const scopeLabel = computed(() =>
  isEstablishment.value
    ? 'Estabelecimento'
    : (productNameById.value.get(selectedScope.value) ?? 'Produto'),
)

function productLabel(productId?: string | null): string {
  if (!productId) {
    return ''
  }
  return productNameById.value.get(productId) ?? 'produto'
}

const draft = ref({
  authorName: '',
  authorLevel: '',
  rating: 5,
  body: '',
  isHighlight: false,
  dateLabel: '',
})

function resetDraft() {
  draft.value = {
    authorName: '',
    authorLevel: '',
    rating: 5,
    body: '',
    isHighlight: false,
    dateLabel: '',
  }
}

async function loadReviews() {
  if (!isEstablishment.value && !selectedScope.value) {
    reviews.value = []
    return
  }
  if (isEstablishment.value && !store.restaurantId) {
    reviews.value = []
    return
  }
  loading.value = true
  try {
    reviews.value = isEstablishment.value
      ? await store.loadEstablishmentReviews(store.restaurantId)
      : await store.loadReviews(selectedScope.value)
  } catch (caught) {
    ui.error(getApiErrorMessage(caught, 'Nao foi possivel carregar as avaliacoes.'))
  } finally {
    loading.value = false
  }
}

watch(selectedScope, () => {
  resetDraft()
  void loadReviews()
})

// Monta o ReviewInput COMPLETO (full-replace no back). SEM productId; SEMPRE com
// showOnEstablishment. No escopo estabelecimento a review e propria do
// estabelecimento (showOnEstablishment irrelevante; mantemos true). No escopo
// produto a review nasce so do produto (false ate o usuario decidir mostra-la).
function buildInput(overrides: Partial<ReviewInput> = {}): ReviewInput {
  return {
    authorName: draft.value.authorName.trim(),
    authorLevel: draft.value.authorLevel.trim(),
    rating: Math.min(5, Math.max(1, Math.trunc(Number(draft.value.rating) || 5))),
    body: draft.value.body.trim(),
    isHighlight: draft.value.isHighlight,
    showOnEstablishment: isEstablishment.value,
    dateLabel: draft.value.dateLabel.trim(),
    sortOrder: reviews.value.length,
    ...overrides,
  }
}

async function onCreate() {
  if (saving.value) {
    return
  }
  if (isEstablishment.value && !store.restaurantId) {
    return
  }
  if (!isEstablishment.value && !selectedScope.value) {
    return
  }
  if (!draft.value.authorName.trim() || !draft.value.body.trim()) {
    ui.error('Informe o autor e o texto da avaliacao.')
    return
  }
  saving.value = true
  try {
    if (isEstablishment.value) {
      await store.createEstablishmentReview(store.restaurantId, buildInput())
    } else {
      await store.createReview(selectedScope.value, buildInput())
    }
    resetDraft()
    await loadReviews()
    ui.success('Avaliacao adicionada.')
  } catch (caught) {
    ui.error(getApiErrorMessage(caught, 'Nao foi possivel adicionar a avaliacao.'))
  } finally {
    saving.value = false
  }
}

// Remonta o ReviewInput COMPLETO da review (full-replace) com o destaque trocado.
async function toggleHighlight(review: Review) {
  busyId.value = review.id
  try {
    await store.patchReview(review.id, {
      authorName: review.authorName,
      authorLevel: review.authorLevel,
      rating: review.rating,
      body: review.body,
      isHighlight: !review.isHighlight,
      showOnEstablishment: review.showOnEstablishment,
      dateLabel: review.dateLabel,
      sortOrder: review.sortOrder,
    })
    await loadReviews()
  } catch (caught) {
    ui.error(getApiErrorMessage(caught, 'Nao foi possivel atualizar a avaliacao.'))
  } finally {
    busyId.value = ''
  }
}

// So no escopo produto: liga/desliga a exibicao da review na vitrine do
// estabelecimento (a store remonta o objeto completo com a flag trocada).
async function toggleOnEstablishment(review: Review) {
  busyId.value = review.id
  try {
    await store.setReviewOnEstablishment(review, !review.showOnEstablishment)
    await loadReviews()
    ui.success(
      review.showOnEstablishment
        ? 'Avaliacao removida do estabelecimento.'
        : 'Avaliacao passou a aparecer no estabelecimento.',
    )
  } catch (caught) {
    ui.error(getApiErrorMessage(caught, 'Nao foi possivel atualizar a avaliacao.'))
  } finally {
    busyId.value = ''
  }
}

async function remove(review: Review) {
  const { confirmed } = (await ui.confirm({
    title: 'Remover avaliacao',
    message: `Remover a avaliacao de "${review.authorName}"?`,
    confirmLabel: 'Remover',
  })) as { confirmed: boolean }
  if (!confirmed) {
    return
  }
  busyId.value = review.id
  try {
    await store.deleteReview(review.id)
    await loadReviews()
    ui.success('Avaliacao removida.')
  } catch (caught) {
    ui.error(getApiErrorMessage(caught, 'Nao foi possivel remover a avaliacao.'))
  } finally {
    busyId.value = ''
  }
}

function stars(rating: number): string {
  const value = Math.min(5, Math.max(0, Math.round(rating)))
  return '★'.repeat(value) + '☆'.repeat(5 - value)
}
</script>

<template>
  <div class="cardapio-rev">
    <div class="cardapio-rev__scope">
      <label class="cardapio-rev__select-field">
        <span class="cardapio-rev__label">Escopo</span>
        <select v-model="selectedScope" class="cardapio-rev__input">
          <option :value="ESTABLISHMENT">Estabelecimento</option>
          <option v-for="product in store.products" :key="product.id" :value="product.id">
            {{ product.name }}
          </option>
        </select>
      </label>
      <span class="cardapio-rev__scope-badge">
        {{ scopeLabel }} · {{ reviews.length }}
        {{ reviews.length === 1 ? 'avaliacao' : 'avaliacoes' }}
      </span>
    </div>

    <p class="cardapio-rev__hint">
      <template v-if="isEstablishment">
        Avaliacoes do estabelecimento — inclui as proprias e as de produto marcadas para aparecer
        aqui.
      </template>
      <template v-else>
        Avaliacoes do produto. Use "Mostrar no estabelecimento" para exibir uma delas na vitrine
        geral.
      </template>
    </p>

    <form class="cardapio-rev__form" @submit.prevent="onCreate">
      <div class="cardapio-rev__form-grid">
        <input
          v-model="draft.authorName"
          type="text"
          class="cardapio-rev__input"
          placeholder="Nome do autor"
        />
        <input
          v-model="draft.authorLevel"
          type="text"
          class="cardapio-rev__input"
          placeholder="Nivel (ex.: Top reviewer)"
        />
        <select v-model.number="draft.rating" class="cardapio-rev__input">
          <option v-for="n in 5" :key="n" :value="n">{{ n }} estrela(s)</option>
        </select>
        <input
          v-model="draft.dateLabel"
          type="text"
          class="cardapio-rev__input"
          placeholder="Data (ex.: ha 2 dias)"
        />
      </div>
      <textarea
        v-model="draft.body"
        rows="2"
        class="cardapio-rev__input"
        placeholder="Texto da avaliacao"
      ></textarea>
      <div class="cardapio-rev__form-foot">
        <label class="cardapio-rev__toggle">
          <input v-model="draft.isHighlight" type="checkbox" />
          <span>Destacar</span>
        </label>
        <button type="submit" class="cardapio-rev__add" :disabled="saving">
          {{ saving ? 'Adicionando...' : 'Adicionar avaliacao' }}
        </button>
      </div>
    </form>

    <div v-if="loading" class="cardapio-rev__state">Carregando avaliacoes...</div>
    <p v-else-if="!reviews.length" class="cardapio-rev__hint">
      {{
        isEstablishment
          ? 'Nenhuma avaliacao no estabelecimento ainda.'
          : 'Nenhuma avaliacao para este produto.'
      }}
    </p>

    <ul v-else class="cardapio-rev__list">
      <li v-for="review in reviews" :key="review.id" class="cardapio-rev__item">
        <div class="cardapio-rev__top">
          <span class="cardapio-rev__author">{{ review.authorName }}</span>
          <span class="cardapio-rev__stars">{{ stars(review.rating) }}</span>
          <span
            v-if="isEstablishment && review.productId"
            class="cardapio-rev__origin"
            :title="`Avaliacao do produto ${productLabel(review.productId)}`"
          >
            do produto{{
              productLabel(review.productId) ? `: ${productLabel(review.productId)}` : ''
            }}
          </span>
          <span v-if="review.dateLabel" class="cardapio-rev__date">{{ review.dateLabel }}</span>
        </div>
        <p class="cardapio-rev__body">{{ review.body }}</p>
        <div class="cardapio-rev__actions">
          <span
            class="cardapio-rev__pill"
            :class="review.isHighlight ? 'is-on' : 'is-off'"
            @click="toggleHighlight(review)"
          >
            {{ review.isHighlight ? 'Destaque' : 'Comum' }}
          </span>
          <button
            v-if="!isEstablishment"
            type="button"
            class="cardapio-rev__btn"
            :class="{ 'cardapio-rev__btn--active': review.showOnEstablishment }"
            :disabled="busyId === review.id"
            @click="toggleOnEstablishment(review)"
          >
            {{ review.showOnEstablishment ? 'No estabelecimento' : 'Mostrar no estabelecimento' }}
          </button>
          <button
            type="button"
            class="cardapio-rev__btn cardapio-rev__btn--danger"
            :disabled="busyId === review.id"
            @click="remove(review)"
          >
            Remover
          </button>
        </div>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.cardapio-rev {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.cardapio-rev__scope {
  display: flex;
  align-items: flex-end;
  flex-wrap: wrap;
  gap: 0.75rem;
}

.cardapio-rev__select-field {
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
  max-width: 360px;
}

.cardapio-rev__scope-badge {
  display: inline-flex;
  align-items: center;
  padding: 0.3rem 0.65rem;
  border-radius: 999px;
  font-size: 0.78rem;
  font-weight: 600;
  background: rgb(var(--primary) / 0.14);
  color: rgb(var(--primary));
}

.cardapio-rev__label {
  font-size: 0.8rem;
  font-weight: 600;
  color: var(--text-main);
}

.cardapio-rev__input {
  width: 100%;
  padding: 0.55rem 0.7rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2) / 0.6);
  color: var(--text-main);
  font-size: 0.9rem;
  font-family: inherit;
}

.cardapio-rev__input:focus {
  outline: none;
  border-color: rgb(var(--ring));
  box-shadow: 0 0 0 3px rgb(var(--ring) / 0.18);
}

.cardapio-rev__hint {
  font-size: 0.88rem;
  color: var(--text-muted);
}

.cardapio-rev__form {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
  padding: 1rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface) / 0.6);
}

.cardapio-rev__form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.6rem;
}

.cardapio-rev__form-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.cardapio-rev__toggle {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  font-size: 0.86rem;
  color: var(--text-main);
}

.cardapio-rev__add {
  border: none;
  color: rgb(var(--surface));
  background: linear-gradient(135deg, rgb(var(--primary)), rgb(var(--primary-600)));
  padding: 0.5rem 1rem;
  border-radius: var(--radius-sm);
  font-weight: 600;
  font-size: 0.86rem;
  cursor: pointer;
}

.cardapio-rev__add:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.cardapio-rev__state {
  color: var(--text-muted);
  font-size: 0.9rem;
}

.cardapio-rev__list {
  display: flex;
  flex-direction: column;
  gap: 0.55rem;
  list-style: none;
}

.cardapio-rev__item {
  padding: 0.75rem 0.9rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface) / 0.6);
}

.cardapio-rev__top {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.6rem;
  margin-bottom: 0.35rem;
}

.cardapio-rev__author {
  font-weight: 600;
  color: var(--text-main);
}

.cardapio-rev__stars {
  color: rgb(var(--accent-warning, var(--primary)));
  letter-spacing: 0.1em;
}

.cardapio-rev__origin {
  padding: 0.12rem 0.5rem;
  border-radius: 999px;
  font-size: 0.72rem;
  font-weight: 600;
  background: rgb(var(--surface-2) / 0.9);
  color: var(--text-muted);
  border: 1px solid var(--line-soft);
}

.cardapio-rev__date {
  font-size: 0.8rem;
  color: var(--text-muted);
}

.cardapio-rev__body {
  font-size: 0.9rem;
  color: var(--text-main);
  margin-bottom: 0.5rem;
}

.cardapio-rev__actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.cardapio-rev__pill {
  padding: 0.18rem 0.55rem;
  border-radius: 999px;
  font-size: 0.74rem;
  font-weight: 600;
  cursor: pointer;
}

.cardapio-rev__pill.is-on {
  background: rgb(var(--primary) / 0.16);
  color: rgb(var(--primary));
}

.cardapio-rev__pill.is-off {
  background: rgb(var(--muted) / 0.16);
  color: var(--text-muted);
}

.cardapio-rev__btn {
  border: 1px solid var(--line-soft);
  background: rgb(var(--surface-2) / 0.8);
  color: var(--text-main);
  padding: 0.4rem 0.7rem;
  border-radius: var(--radius-sm);
  font-size: 0.82rem;
  font-weight: 600;
  cursor: pointer;
}

.cardapio-rev__btn:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.cardapio-rev__btn--active {
  border-color: rgb(var(--primary) / 0.4);
  background: rgb(var(--primary) / 0.16);
  color: rgb(var(--primary));
}

.cardapio-rev__btn--danger {
  color: rgb(var(--danger));
  border-color: rgb(var(--danger) / 0.4);
}

@media (max-width: 640px) {
  .cardapio-rev__form-grid {
    grid-template-columns: 1fr;
  }
}
</style>
