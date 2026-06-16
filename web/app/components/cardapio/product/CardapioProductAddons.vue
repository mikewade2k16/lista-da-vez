<script setup lang="ts">
import CardapioMoneyInput from '~/components/cardapio/CardapioMoneyInput.vue'

// Adicionais editaveis do produto (replace-all no PATCH). priceCents >= 0.
export interface AddonDraft {
  id: string
  name: string
  priceCents: number
}

const props = defineProps<{ modelValue: AddonDraft[] }>()
const emit = defineEmits<{ (e: 'update:modelValue', value: AddonDraft[]): void }>()

function add() {
  emit('update:modelValue', [...props.modelValue, { id: '', name: '', priceCents: 0 }])
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

function updatePrice(index: number, value: number) {
  const next = props.modelValue.map((item, i) =>
    i === index ? { ...item, priceCents: value } : item,
  )
  emit('update:modelValue', next)
}
</script>

<template>
  <div class="cardapio-addon">
    <div class="cardapio-addon__head">
      <span class="cardapio-addon__title">Adicionais</span>
      <button type="button" class="cardapio-addon__add" @click="add">Adicionar</button>
    </div>
    <p v-if="!modelValue.length" class="cardapio-addon__empty">
      Sem adicionais. Use para complementos opcionais que somam ao preco.
    </p>
    <div v-for="(addon, index) in modelValue" :key="index" class="cardapio-addon__row">
      <input
        :value="addon.name"
        type="text"
        class="cardapio-addon__input"
        placeholder="Ex.: Bacon extra"
        @input="updateName(index, ($event.target as HTMLInputElement).value)"
      />
      <CardapioMoneyInput
        :model-value="addon.priceCents"
        placeholder="Preco"
        @update:model-value="updatePrice(index, $event)"
      />
      <button type="button" class="cardapio-addon__remove" @click="remove(index)">Remover</button>
    </div>
  </div>
</template>

<style scoped>
.cardapio-addon {
  display: flex;
  flex-direction: column;
  gap: 0.55rem;
}

.cardapio-addon__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.cardapio-addon__title {
  font-size: 0.85rem;
  font-weight: 700;
  color: var(--text-main);
}

.cardapio-addon__add {
  border: 1px solid var(--line-soft);
  background: rgb(var(--surface-2) / 0.8);
  color: var(--text-main);
  padding: 0.35rem 0.7rem;
  border-radius: var(--radius-sm);
  font-size: 0.8rem;
  font-weight: 600;
  cursor: pointer;
}

.cardapio-addon__empty {
  font-size: 0.82rem;
  color: var(--text-muted);
}

.cardapio-addon__row {
  display: grid;
  grid-template-columns: minmax(0, 1.4fr) minmax(0, 1fr) auto;
  gap: 0.5rem;
  align-items: center;
}

.cardapio-addon__input {
  width: 100%;
  padding: 0.5rem 0.65rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2) / 0.6);
  color: var(--text-main);
  font-size: 0.88rem;
}

.cardapio-addon__input:focus {
  outline: none;
  border-color: rgb(var(--ring));
  box-shadow: 0 0 0 3px rgb(var(--ring) / 0.18);
}

.cardapio-addon__remove {
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
