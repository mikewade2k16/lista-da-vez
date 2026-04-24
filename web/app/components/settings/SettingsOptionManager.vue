<script setup>
import { ref, watch } from "vue";

const props = defineProps({
  title: {
    type: String,
    required: true
  },
  description: {
    type: String,
    default: ""
  },
  items: {
    type: Array,
    default: () => []
  },
  addPlaceholder: {
    type: String,
    default: "Adicionar nova opcao"
  },
  testid: {
    type: String,
    default: ""
  }
});

const emit = defineEmits(["add", "update", "remove"]);
const drafts = ref({});
const updateErrors = ref({});
const newLabel = ref("");
const addError = ref("");

watch(
  () => props.items,
  (items) => {
    drafts.value = Object.fromEntries((items || []).map((item) => [item.id, item.label]));
  },
  { immediate: true, deep: true }
);

function normalize(str) {
  return String(str || "").trim().toLowerCase();
}

function isDuplicate(label, excludeId = null) {
  const normalized = normalize(label);
  if (!normalized) return false;
  return (props.items || []).some(
    (item) => item.id !== excludeId && normalize(item.label) === normalized
  );
}

function submitAdd() {
  const trimmed = newLabel.value.trim();
  if (!trimmed) return;
  if (isDuplicate(trimmed)) {
    addError.value = "Já existe um registro com esse nome.";
    return;
  }
  addError.value = "";
  emit("add", trimmed);
  newLabel.value = "";
}

function submitUpdate(id) {
  const label = drafts.value[id];
  if (isDuplicate(label, id)) {
    updateErrors.value = { ...updateErrors.value, [id]: "Já existe um registro com esse nome." };
    return;
  }
  updateErrors.value = { ...updateErrors.value, [id]: "" };
  emit("update", id, label);
}
</script>

<template>
  <article class="settings-card">
    <header class="settings-card__header">
      <h3 class="settings-card__title">{{ title }}</h3>
      <p class="settings-card__text">{{ description }}</p>
    </header>

    <div class="option-list">
      <span v-if="!items.length" class="insight-empty">Sem opcoes cadastradas.</span>
      <form
        v-for="item in items"
        :key="item.id"
        class="option-row"
        @submit.prevent="submitUpdate(item.id)"
      >
        <input v-model="drafts[item.id]" class="option-row__input" type="text" @input="updateErrors[item.id] = ''">
        <button class="option-row__save" type="submit">Salvar</button>
        <button class="option-row__remove" type="button" @click="$emit('remove', item.id)">Excluir</button>
        <span v-if="updateErrors[item.id]" class="option-row__error">{{ updateErrors[item.id] }}</span>
      </form>
    </div>

    <form class="option-add" @submit.prevent="submitAdd">
      <input v-model="newLabel" class="option-add__input" type="text" :placeholder="addPlaceholder" :data-testid="testid ? `${testid}-add-input` : undefined" @input="addError = ''">
      <button class="option-add__button" type="submit" :data-testid="testid ? `${testid}-add-btn` : undefined">Adicionar</button>
    </form>
    <span v-if="addError" class="option-add__error">{{ addError }}</span>
  </article>
</template>
