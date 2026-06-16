<script setup lang="ts">
import { computed } from 'vue'

import BioCollapsibleItem from '~/components/bio/BioCollapsibleItem.vue'

// Editor reutilizavel de uma lista ordenada de itens (links[] e headerMenu[]).
// A ORDEM DO ARRAY e a ordem de exibicao — reordenar e mover o item no array
// com os botoes subir/descer. Cada item tem ao menos `label`; os demais campos
// (href, action, icon) sao editados livremente. O componente nao conhece o
// shape exato: opera sobre Record<string, unknown> e expoe os campos via prop
// `fields`, mantendo-o generico para link e item de menu. Cada item vive num
// bloco colapsavel (header = label do item + resumo do link); recolhido por
// padrao quando ha varios itens.

interface FieldDef {
  key: string
  label: string
  placeholder?: string
}

type ListItem = Record<string, unknown>

const props = defineProps<{
  modelValue?: ListItem[]
  fields: FieldDef[]
  itemLabel?: string
  addLabel?: string
  emptyHint?: string
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: ListItem[]): void
}>()

const items = computed<ListItem[]>(() => (Array.isArray(props.modelValue) ? props.modelValue : []))

// Com varios itens recolhe por padrao; com um unico item, abre para edicao direta.
const collapseByDefault = computed(() => items.value.length > 1)

function emitItems(next: ListItem[]) {
  emit('update:modelValue', next)
}

function fieldValue(item: ListItem, key: string): string {
  const value = item[key]
  return value === undefined || value === null ? '' : String(value)
}

function itemTitle(item: ListItem, index: number): string {
  const label = fieldValue(item, 'label').trim()
  return label || `${props.itemLabel || 'Item'} ${index + 1}`
}

function itemSummary(item: ListItem): string {
  const href = fieldValue(item, 'href').trim()
  const action = fieldValue(item, 'action').trim()
  return href || action || 'sem link'
}

function updateField(index: number, key: string, value: string) {
  const next = items.value.map((item, position) =>
    position === index ? { ...item, [key]: value } : item,
  )
  emitItems(next)
}

function addItem() {
  const blank: ListItem = {}
  props.fields.forEach((field) => {
    blank[field.key] = ''
  })
  emitItems([...items.value, blank])
}

function removeItem(index: number) {
  emitItems(items.value.filter((_, position) => position !== index))
}

function move(index: number, direction: -1 | 1) {
  const target = index + direction
  if (target < 0 || target >= items.value.length) {
    return
  }
  const next = [...items.value]
  const [moved] = next.splice(index, 1)
  next.splice(target, 0, moved)
  emitItems(next)
}
</script>

<template>
  <div class="bio-link-list">
    <p v-if="!items.length" class="bio-link-list__empty">
      {{ emptyHint || 'Nenhum item ainda. Adicione o primeiro abaixo.' }}
    </p>

    <ul v-else class="bio-link-list__items">
      <li v-for="(item, index) in items" :key="index" class="bio-link-list__item">
        <BioCollapsibleItem
          :title="itemTitle(item, index)"
          :summary="itemSummary(item)"
          :default-open="!collapseByDefault"
        >
          <template #actions>
            <button
              type="button"
              class="bio-link-list__order-btn"
              :disabled="index === 0"
              :aria-label="`Mover para cima o item ${index + 1}`"
              @click="move(index, -1)"
            >
              <UIcon name="i-lucide-chevron-up" />
            </button>
            <button
              type="button"
              class="bio-link-list__order-btn"
              :disabled="index === items.length - 1"
              :aria-label="`Mover para baixo o item ${index + 1}`"
              @click="move(index, 1)"
            >
              <UIcon name="i-lucide-chevron-down" />
            </button>
            <button
              type="button"
              class="bio-link-list__remove"
              :aria-label="`Remover o item ${index + 1}`"
              @click="removeItem(index)"
            >
              <UIcon name="i-lucide-trash-2" />
            </button>
          </template>

          <div class="bio-link-list__fields">
            <div v-for="field in fields" :key="field.key" class="bio-link-list__field">
              <label class="bio-link-list__label">{{ field.label }}</label>
              <UInput
                :model-value="fieldValue(item, field.key)"
                :placeholder="field.placeholder || ''"
                @update:model-value="updateField(index, field.key, String($event ?? ''))"
              />
            </div>
          </div>
        </BioCollapsibleItem>
      </li>
    </ul>

    <UButton
      icon="i-lucide-plus"
      color="neutral"
      variant="soft"
      :label="addLabel || `Adicionar ${itemLabel || 'item'}`"
      @click="addItem"
    />
  </div>
</template>

<style scoped>
.bio-link-list {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
}

.bio-link-list__empty {
  margin: 0;
  padding: 0.85rem 1rem;
  border: 1px dashed var(--line-soft);
  border-radius: var(--radius-soft);
  color: var(--text-muted);
  font-size: 0.85rem;
  background: rgb(var(--surface-2) / 0.4);
}

.bio-link-list__items {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 0.5rem;
  margin: 0;
  padding: 0;
  list-style: none;
}

.bio-link-list__item {
  min-width: 0;
}

.bio-link-list__order-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 1.5rem;
  height: 1.5rem;
  border: 1px solid var(--line-soft);
  border-radius: 0.4rem;
  background: rgb(var(--surface-2) / 0.6);
  color: var(--text-muted);
  cursor: pointer;
}

.bio-link-list__order-btn:hover:not(:disabled) {
  color: rgb(var(--primary));
  border-color: rgb(var(--primary) / 0.4);
}

.bio-link-list__order-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.bio-link-list__fields {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  gap: 0.6rem;
  min-width: 0;
}

.bio-link-list__field {
  display: grid;
  gap: 0.3rem;
}

.bio-link-list__label {
  font-size: 0.72rem;
  font-weight: 700;
  color: var(--text-muted);
}

.bio-link-list__remove {
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

.bio-link-list__remove:hover {
  color: rgb(var(--danger));
  border-color: rgb(var(--danger) / 0.4);
}
</style>
