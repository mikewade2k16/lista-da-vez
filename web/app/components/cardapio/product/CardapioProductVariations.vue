<script setup lang="ts">
import CardapioMoneyInput from '~/components/cardapio/CardapioMoneyInput.vue'

// Variacoes editaveis do produto. O PATCH do produto faz replace-all, entao a
// lista local e enviada inteira. priceDeltaCents pode ser negativo (desconto).
export interface VariationDraft {
  id: string
  name: string
  priceDeltaCents: number
}

const props = defineProps<{ modelValue: VariationDraft[] }>()
const emit = defineEmits<{ (e: 'update:modelValue', value: VariationDraft[]): void }>()

function add() {
  emit('update:modelValue', [...props.modelValue, { id: '', name: '', priceDeltaCents: 0 }])
}

function remove(index: number) {
  emit(
    'update:modelValue',
    props.modelValue.filter((_, i) => i !== index),
  )
}

function updateName(index: number, value: string) {
  const next = props.modelValue.map((item, i) => (i === index ? { ...item, name: value } : item))
  emit('update:modelValue', next)
}

function updateDelta(index: number, value: number) {
  const next = props.modelValue.map((item, i) =>
    i === index ? { ...item, priceDeltaCents: value } : item,
  )
  emit('update:modelValue', next)
}
</script>

<template>
  <div class="cardapio-var">
    <div class="cardapio-var__head">
      <span class="cardapio-var__title">Variacoes</span>
      <button type="button" class="cardapio-var__add" @click="add">Adicionar</button>
    </div>
    <p v-if="!modelValue.length" class="cardapio-var__empty">
      Sem variacoes. Use para tamanhos ou opcoes que alteram o preco.
    </p>
    <div v-for="(variation, index) in modelValue" :key="index" class="cardapio-var__row">
      <input
        :value="variation.name"
        type="text"
        class="cardapio-var__input"
        placeholder="Ex.: Grande"
        @input="updateName(index, ($event.target as HTMLInputElement).value)"
      />
      <CardapioMoneyInput
        :model-value="variation.priceDeltaCents"
        placeholder="Acrescimo"
        @update:model-value="updateDelta(index, $event)"
      />
      <button type="button" class="cardapio-var__remove" @click="remove(index)">Remover</button>
    </div>
  </div>
</template>

<style scoped>
.cardapio-var {
  display: flex;
  flex-direction: column;
  gap: 0.55rem;
}

.cardapio-var__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.cardapio-var__title {
  font-size: 0.85rem;
  font-weight: 700;
  color: var(--text-main);
}

.cardapio-var__add {
  border: 1px solid var(--line-soft);
  background: rgb(var(--surface-2) / 0.8);
  color: var(--text-main);
  padding: 0.35rem 0.7rem;
  border-radius: var(--radius-sm);
  font-size: 0.8rem;
  font-weight: 600;
  cursor: pointer;
}

.cardapio-var__empty {
  font-size: 0.82rem;
  color: var(--text-muted);
}

.cardapio-var__row {
  display: grid;
  grid-template-columns: minmax(0, 1.4fr) minmax(0, 1fr) auto;
  gap: 0.5rem;
  align-items: center;
}

.cardapio-var__input {
  width: 100%;
  padding: 0.5rem 0.65rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2) / 0.6);
  color: var(--text-main);
  font-size: 0.88rem;
}

.cardapio-var__input:focus {
  outline: none;
  border-color: rgb(var(--ring));
  box-shadow: 0 0 0 3px rgb(var(--ring) / 0.18);
}

.cardapio-var__remove {
  border: 1px solid rgb(var(--danger) / 0.4);
  background: transparent;
  color: rgb(var(--danger));
  padding: 0.4rem 0.6rem;
  border-radius: var(--radius-sm);
  font-size: 0.8rem;
  font-weight: 600;
  cursor: pointer;
  white-space: nowrap;
}
</style>
