<script setup>
import { ArrowDown, ArrowUp, Plus, X } from 'lucide-vue-next'
import { computed, nextTick, ref, watch } from 'vue'

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
const addPosition = ref('')
const topInputRef = ref(null)
const bottomInputRef = ref(null)

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
  addPosition.value = ''
}

function openAdd(position) {
  if (props.disabled) {
    return
  }

  addError.value = ''
  addPosition.value = position
  nextTick(() => {
    const input = position === 'top' ? topInputRef.value : bottomInputRef.value
    input?.focus()
  })
}

function closeAdd() {
  addError.value = ''
  newLabel.value = ''
  addPosition.value = ''
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
  <article class="settings-option-manager">
    <header class="settings-option-manager__header">
      <div class="settings-option-manager__heading">
        <div class="settings-option-manager__title-row">
          <h3>{{ title }}</h3>
          <span>{{ itemCountLabel }}</span>
        </div>
        <p>{{ description }}</p>
      </div>

      <button
        class="option-add-trigger option-add-trigger--top"
        type="button"
        :disabled="disabled"
        aria-label="Adicionar nova opcao no inicio da lista"
        title="Adicionar nova opcao"
        data-tooltip="Adicionar nova opcao"
        @click="openAdd('top')"
      >
        <Plus :size="18" :stroke-width="2.4" aria-hidden="true" />
      </button>
    </header>

    <form v-if="addPosition === 'top'" class="option-add-inline" @submit.prevent="submitAdd">
      <input
        ref="topInputRef"
        v-model="newLabel"
        class="option-add-inline__input"
        type="text"
        :placeholder="addPlaceholder"
        :disabled="disabled"
        :data-testid="testid ? `${testid}-add-input` : undefined"
        aria-label="Nome da nova opcao"
        @input="addError = ''"
        @keydown.esc.prevent="closeAdd"
      />
      <button
        class="option-add__button option-add-inline__submit"
        type="submit"
        :disabled="disabled || !newLabel.trim()"
        :data-testid="testid ? `${testid}-add-btn` : undefined"
      >
        Adicionar
      </button>
      <button
        class="option-add-inline__cancel"
        type="button"
        aria-label="Cancelar nova opcao"
        @click="closeAdd"
      >
        <X :size="17" :stroke-width="2.2" aria-hidden="true" />
      </button>
      <span v-if="addError" class="option-add-inline__error">{{ addError }}</span>
    </form>

    <div class="option-list">
      <span v-if="!items.length" class="settings-option-manager__empty">
        Sem opcoes cadastradas.
      </span>

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
          :aria-label="`Editar ${item.label}`"
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

    <form
      v-if="addPosition === 'bottom'"
      class="option-add-inline option-add-inline--bottom"
      @submit.prevent="submitAdd"
    >
      <input
        ref="bottomInputRef"
        v-model="newLabel"
        class="option-add-inline__input"
        type="text"
        :placeholder="addPlaceholder"
        :disabled="disabled"
        :data-testid="testid ? `${testid}-add-input` : undefined"
        aria-label="Nome da nova opcao"
        @input="addError = ''"
        @keydown.esc.prevent="closeAdd"
      />
      <button
        class="option-add__button option-add-inline__submit"
        type="submit"
        :disabled="disabled || !newLabel.trim()"
        :data-testid="testid ? `${testid}-add-btn` : undefined"
      >
        Adicionar
      </button>
      <button
        class="option-add-inline__cancel"
        type="button"
        aria-label="Cancelar nova opcao"
        @click="closeAdd"
      >
        <X :size="17" :stroke-width="2.2" aria-hidden="true" />
      </button>
      <span v-if="addError" class="option-add-inline__error">{{ addError }}</span>
    </form>

    <div class="settings-option-manager__bottom-action">
      <button
        class="option-add-trigger option-add-trigger--bottom"
        type="button"
        :disabled="disabled"
        aria-label="Adicionar nova opcao ao final da lista"
        title="Adicionar nova opcao"
        data-tooltip="Adicionar nova opcao"
        @click="openAdd('bottom')"
      >
        <Plus :size="18" :stroke-width="2.4" aria-hidden="true" />
      </button>
    </div>
  </article>
</template>

<style scoped src="./settings-option-manager.css"></style>
