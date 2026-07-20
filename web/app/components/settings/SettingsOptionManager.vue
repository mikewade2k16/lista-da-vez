<script setup>
import { ArrowDown, ArrowUp } from 'lucide-vue-next'
import { computed, ref, watch } from 'vue'

const props = defineProps({
  title: {
    type: String,
    required: true,
  },
  description: {
    type: String,
    default: '',
  },
  items: {
    type: Array,
    default: () => [],
  },
  addPlaceholder: {
    type: String,
    default: 'Adicionar nova opcao',
  },
  testid: {
    type: String,
    default: '',
  },
  disabled: {
    type: Boolean,
    default: false,
  },
})

const emit = defineEmits(['add', 'update', 'remove', 'reorder'])
const drafts = ref({})
const updateErrors = ref({})
const newLabel = ref('')
const addError = ref('')

const itemCountLabel = computed(() => {
  const count = props.items?.length || 0
  return `${count} ${count === 1 ? 'opcao' : 'opcoes'}`
})

watch(
  () => props.items,
  (items) => {
    drafts.value = Object.fromEntries((items || []).map((item) => [item.id, item.label]))
  },
  { immediate: true, deep: true },
)

function normalize(value) {
  return String(value || '')
    .trim()
    .toLowerCase()
}

function isDuplicate(label, excludeId = null) {
  const normalized = normalize(label)
  if (!normalized) {
    return false
  }

  return (props.items || []).some(
    (item) => item.id !== excludeId && normalize(item.label) === normalized,
  )
}

function submitAdd() {
  if (props.disabled) {
    return
  }

  const trimmed = newLabel.value.trim()
  if (!trimmed) {
    return
  }

  if (isDuplicate(trimmed)) {
    addError.value = 'Ja existe um registro com esse nome.'
    return
  }

  addError.value = ''
  emit('add', trimmed)
  newLabel.value = ''
}

function submitUpdate(id) {
  if (props.disabled) {
    return
  }

  const label = drafts.value[id]
  if (isDuplicate(label, id)) {
    updateErrors.value = { ...updateErrors.value, [id]: 'Ja existe um registro com esse nome.' }
    return
  }

  updateErrors.value = { ...updateErrors.value, [id]: '' }
  emit('update', id, label)
}

function moveItem(itemId, direction) {
  if (props.disabled) {
    return
  }

  const currentIds = (props.items || []).map((item) => item.id)
  const currentIndex = currentIds.findIndex((id) => id === itemId)
  const nextIndex = currentIndex + direction

  if (currentIndex < 0 || nextIndex < 0 || nextIndex >= currentIds.length) {
    return
  }

  const nextIds = [...currentIds]
  ;[nextIds[currentIndex], nextIds[nextIndex]] = [nextIds[nextIndex], nextIds[currentIndex]]
  emit('reorder', nextIds)
}
</script>

<template>
  <article class="settings-card">
    <header class="settings-card__header">
      <h3 class="settings-card__title">{{ title }}</h3>
      <p class="settings-card__text">{{ description }}</p>
    </header>

    <details class="settings-collapse">
      <summary class="settings-collapse__summary">
        <div class="settings-collapse__title-wrap">
          <strong class="settings-collapse__title">Cadastrados</strong>
          <span class="settings-collapse__text">
            A ordem abaixo define como o select aparece no sistema.
          </span>
        </div>
        <span class="settings-collapse__meta">{{ itemCountLabel }}</span>
        <span class="material-icons-round settings-collapse__icon" aria-hidden="true">
          expand_more
        </span>
      </summary>

      <div class="settings-collapse__body">
        <div class="option-list">
          <span v-if="!items.length" class="insight-empty">Sem opcoes cadastradas.</span>

          <form
            v-for="(item, index) in items"
            :key="item.id"
            class="option-row option-row--sortable"
            @submit.prevent="submitUpdate(item.id)"
          >
            <div class="option-row__order">
              <span class="option-row__index">{{ index + 1 }}</span>
              <div class="option-row__order-actions">
                <button
                  class="option-row__move"
                  type="button"
                  :disabled="disabled || index === 0"
                  :aria-label="`Mover ${item.label} para cima`"
                  @click="moveItem(item.id, -1)"
                >
                  <ArrowUp :size="14" :stroke-width="2.2" />
                </button>
                <button
                  class="option-row__move"
                  type="button"
                  :disabled="disabled || index === items.length - 1"
                  :aria-label="`Mover ${item.label} para baixo`"
                  @click="moveItem(item.id, 1)"
                >
                  <ArrowDown :size="14" :stroke-width="2.2" />
                </button>
              </div>
            </div>

            <input
              v-model="drafts[item.id]"
              class="option-row__input"
              type="text"
              :disabled="disabled"
              @input="updateErrors[item.id] = ''"
            />
            <button class="option-row__save" type="submit" :disabled="disabled">Salvar</button>
            <button
              class="option-row__remove"
              type="button"
              :disabled="disabled"
              @click="$emit('remove', item.id)"
            >
              Excluir
            </button>
            <span v-if="updateErrors[item.id]" class="option-row__error">
              {{ updateErrors[item.id] }}
            </span>
          </form>
        </div>
      </div>
    </details>

    <details class="settings-collapse">
      <summary class="settings-collapse__summary">
        <div class="settings-collapse__title-wrap">
          <strong class="settings-collapse__title">Adicionar</strong>
          <span class="settings-collapse__text">Cadastre uma nova opcao para a lista.</span>
        </div>
        <span class="settings-collapse__meta">{{ itemCountLabel }}</span>
        <span class="material-icons-round settings-collapse__icon" aria-hidden="true">
          expand_more
        </span>
      </summary>

      <div class="settings-collapse__body">
        <form class="option-add" @submit.prevent="submitAdd">
          <input
            v-model="newLabel"
            class="option-add__input"
            type="text"
            :placeholder="addPlaceholder"
            :disabled="disabled"
            :data-testid="testid ? `${testid}-add-input` : undefined"
            @input="addError = ''"
          />
          <button
            class="option-add__button"
            type="submit"
            :disabled="disabled"
            :data-testid="testid ? `${testid}-add-btn` : undefined"
          >
            Adicionar
          </button>
        </form>
        <span v-if="addError" class="option-add__error">{{ addError }}</span>
      </div>
    </details>
  </article>
</template>

<style scoped>
.option-row--sortable {
  grid-template-columns: auto minmax(0, 1fr) auto auto;
  align-items: center;
}

.option-row__order {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.option-row__index {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 28px;
  height: 28px;
  padding: 0 8px;
  border: 1px solid rgb(var(--ring) / 0.18);
  border-radius: 999px;
  background: rgb(var(--primary) / 0.08);
  color: rgb(var(--primary));
  font-size: 0.72rem;
  font-weight: 700;
}

.option-row__order-actions {
  display: inline-flex;
  gap: 6px;
}

.option-row__move {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  padding: 0;
  border: 1px solid rgb(var(--border) / 0.72);
  border-radius: 10px;
  background: rgb(var(--surface-2) / 0.7);
  color: rgb(var(--text) / 0.9);
  cursor: pointer;
  transition:
    border-color 0.18s ease,
    background 0.18s ease,
    color 0.18s ease;
}

.option-row__move:hover:not(:disabled) {
  border-color: rgb(var(--ring) / 0.32);
  background: rgb(var(--primary) / 0.12);
  color: rgb(var(--primary));
}

.option-row__move:disabled {
  opacity: 0.42;
  cursor: not-allowed;
}

.option-row__error {
  grid-column: 2 / -1;
}

@media (max-width: 720px) {
  .option-row--sortable {
    grid-template-columns: 1fr;
  }

  .option-row__order {
    justify-content: space-between;
  }
}
</style>
