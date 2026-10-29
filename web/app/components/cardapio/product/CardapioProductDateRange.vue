<script setup lang="ts">
import type { CardapioProductFilters } from '~/domain/cardapio/product-filters'
const model = defineModel<CardapioProductFilters & Record<string, unknown>>({ required: true })
function update(key: 'createdFrom' | 'createdTo', value: unknown) {
  model.value = { ...model.value, [key]: String(value ?? '') }
}
</script>

<template>
  <div class="cardapio-product-dates">
    <UFormField label="Cadastrados a partir de">
      <UInput
        type="date"
        :model-value="String(model.createdFrom ?? '')"
        :max="String(model.createdTo ?? '') || undefined"
        @update:model-value="update('createdFrom', $event)"
      />
    </UFormField>
    <UFormField label="Cadastrados até">
      <UInput
        type="date"
        :model-value="String(model.createdTo ?? '')"
        :min="String(model.createdFrom ?? '') || undefined"
        @update:model-value="update('createdTo', $event)"
      />
    </UFormField>
    <span>Inclui as datas inicial e final. Esta semana começa na segunda-feira.</span>
  </div>
</template>

<style scoped>
.cardapio-product-dates {
  display: flex;
  align-items: flex-end;
  flex-wrap: wrap;
  gap: 0.75rem;
  margin-top: 0.75rem;
  color: var(--text-muted);
  font-size: 0.8rem;
}
</style>
