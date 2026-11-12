<script setup lang="ts">
import { computed } from 'vue'

import CardapioProductImageCell from '~/components/cardapio/product/CardapioProductImageCell.vue'
import OmniMoneyInput from '../../../../layers/tasks/components/omni/inputs/OmniMoneyInput.vue'
import type { CardapioProductRow } from '~/composables/useCardapioProductColumns'
import { formatCurrency, type Category } from '~/domain/cardapio/types'

// Lista de produtos no mobile: um card por produto com foto (toque para
// enviar/trocar), nome e preco sempre visiveis E editaveis no proprio card
// (mesmo commit com debounce da tabela). Os demais campos aparecem conforme as
// colunas escolhidas no menu do mobile. Edicao completa fica no modal (lapis).
const props = defineProps<{
  rows: CardapioProductRow[]
  visibleKeys: string[]
  categories: Category[]
  uploadingIds: string[]
  busyId: string
  disabled: boolean
  wholeAmounts: boolean
}>()
const selected = defineModel<Array<string | number>>('selected', { required: true })
const emit = defineEmits<{
  upload: [id: string, file: File]
  invalid: [message: string]
  view: [id: string]
  removePhoto: [id: string]
  edit: [id: string]
  remove: [row: CardapioProductRow]
  cell: [id: string, key: string, value: unknown, immediate: boolean]
}>()

const show = computed(() => new Set(props.visibleKeys))
const categoryName = computed(
  () => new Map(props.categories.map((category) => [category.id, category.name])),
)
const selectedSet = computed(() => new Set(selected.value.map(String)))

function toggle(id: string, checked: boolean) {
  const next = selected.value.map(String).filter((value) => value !== id)
  selected.value = checked ? [...next, id] : next
}

function dateLabel(value: string) {
  const date = new Date(value)
  return Number.isFinite(date.getTime())
    ? date.toLocaleDateString('pt-BR', { day: '2-digit', month: '2-digit', year: '2-digit' })
    : ''
}
</script>

<template>
  <ul class="cardapio-cards">
    <li v-if="!rows.length" class="cardapio-cards__empty">
      Nenhum produto encontrado com os filtros atuais.
    </li>
    <li
      v-for="row in rows"
      :key="row.id"
      class="cardapio-card"
      :class="{ 'is-selected': selectedSet.has(row.id), 'is-off': !row.isAvailable }"
    >
      <UCheckbox
        class="cardapio-card__check"
        :model-value="selectedSet.has(row.id)"
        :aria-label="`Selecionar ${row.name}`"
        @update:model-value="toggle(row.id, Boolean($event))"
      />
      <CardapioProductImageCell
        :name="row.name"
        :src="row.imageSrc"
        :uploading="uploadingIds.includes(row.id)"
        :disabled="disabled"
        @upload="emit('upload', row.id, $event)"
        @invalid="emit('invalid', $event)"
        @view="emit('view', row.id)"
        @remove="emit('removePhoto', row.id)"
      />
      <div class="cardapio-card__body">
        <UInput
          :model-value="row.name"
          variant="ghost"
          size="sm"
          class="cardapio-card__name"
          :disabled="disabled"
          :aria-label="`Nome de ${row.name}`"
          @update:model-value="emit('cell', row.id, 'name', String($event ?? ''), false)"
        />
        <div class="cardapio-card__price">
          <OmniMoneyInput
            class="cardapio-card__money"
            :model-value="row.priceReais"
            :whole-amounts="wholeAmounts"
            :aria-label="`Preco de ${row.name}`"
            @update:model-value="emit('cell', row.id, 'priceReais', $event, false)"
          />
          <s v-if="show.has('compareAtPriceReais') && row.compareAtPriceCents">
            {{ formatCurrency(row.compareAtPriceCents) }}
          </s>
          <span v-if="!row.isAvailable && !show.has('isAvailable')" class="cardapio-card__off">
            Indisponivel
          </span>
        </div>
        <p v-if="show.has('categoryId')" class="cardapio-card__meta">
          {{ categoryName.get(row.categoryId || '') || 'Sem categoria' }}
        </p>
        <p v-if="show.has('tags') && row.tags?.length" class="cardapio-card__meta">
          {{ row.tags.join(' · ') }}
        </p>
        <p v-if="show.has('createdAt')" class="cardapio-card__meta">
          Cadastro {{ dateLabel(row.createdAt) }}
        </p>
        <div
          v-if="show.has('isAvailable') || show.has('isFeatured')"
          class="cardapio-card__toggles"
        >
          <USwitch
            v-if="show.has('isAvailable')"
            size="sm"
            label="Disponivel"
            :model-value="row.isAvailable"
            :disabled="disabled"
            @update:model-value="emit('cell', row.id, 'isAvailable', $event, true)"
          />
          <USwitch
            v-if="show.has('isFeatured')"
            size="sm"
            label="Destaque"
            :model-value="row.isFeatured"
            :disabled="disabled"
            @update:model-value="emit('cell', row.id, 'isFeatured', $event, true)"
          />
        </div>
      </div>
      <div class="cardapio-card__actions">
        <UButton
          icon="i-lucide-pencil"
          color="neutral"
          variant="ghost"
          size="sm"
          :aria-label="`Editar ${row.name}`"
          @click="emit('edit', row.id)"
        />
        <UButton
          icon="i-lucide-trash-2"
          color="error"
          variant="ghost"
          size="sm"
          :aria-label="`Remover ${row.name}`"
          :loading="busyId === row.id"
          @click="emit('remove', row)"
        />
      </div>
    </li>
  </ul>
</template>

<style scoped>
.cardapio-cards {
  display: flex;
  flex-direction: column;
  padding: 0;
  list-style: none;
  /* Quase nas bordas da tela: 2px de respiro (o editor mobile tem 0.5rem). */
  margin: 0 calc(-0.5rem + 2px);
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  overflow: hidden;
}

.cardapio-cards__empty {
  padding: 1.25rem;
  color: var(--text-muted);
  font-size: 0.88rem;
  text-align: center;
}

.cardapio-card {
  display: grid;
  grid-template-columns: auto 72px minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.6rem;
  padding: 0.55rem 0.5rem;
  border-bottom: 1px solid var(--line-soft);
}

.cardapio-card:last-child {
  border-bottom: none;
}

.cardapio-card.is-selected {
  background: rgb(var(--primary) / 0.08);
}

.cardapio-card__body {
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
  min-width: 0;
}

/* Nome e preco editaveis: parecem texto, com sublinhado discreto de campo. */
.cardapio-card__name :deep(input) {
  padding-inline: 0.15rem;
  border-bottom: 1px dashed var(--line-soft);
  border-radius: 0;
  color: var(--text-main);
  font-size: 0.92rem;
  font-weight: 600;
}

.cardapio-card.is-off .cardapio-card__name :deep(input) {
  color: var(--text-muted);
}

.cardapio-card__price {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.4rem;
  font-size: 0.86rem;
  font-variant-numeric: tabular-nums;
}

.cardapio-card__money {
  width: 7.5rem;
}

.cardapio-card__price s {
  color: var(--text-muted);
  font-size: 0.78rem;
}

.cardapio-card__off {
  color: rgb(var(--danger));
  font-size: 0.72rem;
  font-weight: 600;
}

.cardapio-card__meta {
  margin: 0;
  overflow: hidden;
  color: var(--text-muted);
  font-size: 0.76rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.cardapio-card__toggles {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem 0.9rem;
  margin-top: 0.2rem;
}

.cardapio-card__actions {
  display: flex;
  flex-direction: column;
  gap: 0.1rem;
}
</style>
