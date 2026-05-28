<script setup lang="ts">
import { ref } from 'vue'
import { ROADMAP_RULE_CATEGORY_LABEL, type RuleCategory } from '~/components/roadmap/roadmap-data'

const CATEGORIES: RuleCategory[] = [
  'frontend',
  'backend',
  'banco',
  'linguagens',
  'deploy',
  'padroes-gerais',
]

const emit = defineEmits<{
  (
    e: 'submit',
    payload: {
      sourceId: string
      category: RuleCategory
      title: string
      body: string
      why: string
      appliesWhen: string
    },
  ): void
  (e: 'cancel'): void
}>()

const sourceId = ref('')
const category = ref<RuleCategory>('frontend')
const title = ref('')
const body = ref('')
const why = ref('')
const appliesWhen = ref('')

function slugify(value: string): string {
  return value
    .toLowerCase()
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/(^-|-$)/g, '')
}

function onTitleBlur() {
  if (!sourceId.value.trim() && title.value.trim()) {
    const prefix =
      category.value === 'frontend'
        ? 'fe'
        : category.value === 'backend'
          ? 'be'
          : category.value === 'banco'
            ? 'banco'
            : category.value === 'linguagens'
              ? 'lang'
              : category.value === 'deploy'
                ? 'deploy'
                : 'geral'
    sourceId.value = `${prefix}-${slugify(title.value).slice(0, 32)}`
  }
}

function submit() {
  if (!sourceId.value.trim() || !title.value.trim() || !body.value.trim()) {
    return
  }
  emit('submit', {
    sourceId: sourceId.value.trim(),
    category: category.value,
    title: title.value.trim(),
    body: body.value.trim(),
    why: why.value.trim(),
    appliesWhen: appliesWhen.value.trim(),
  })
  resetForm()
}

function resetForm() {
  sourceId.value = ''
  title.value = ''
  body.value = ''
  why.value = ''
  appliesWhen.value = ''
}
</script>

<template>
  <form class="roadmap-rule-form" @submit.prevent="submit">
    <header class="roadmap-rule-form__head">
      <h4 class="roadmap-rule-form__title">Nova regra</h4>
    </header>

    <div class="roadmap-rule-form__row roadmap-rule-form__row--two">
      <label class="roadmap-rule-form__field">
        <span class="roadmap-rule-form__field-label">Categoria</span>
        <select v-model="category" class="roadmap-rule-form__select">
          <option v-for="c in CATEGORIES" :key="c" :value="c">
            {{ ROADMAP_RULE_CATEGORY_LABEL[c] }}
          </option>
        </select>
      </label>
      <label class="roadmap-rule-form__field">
        <span class="roadmap-rule-form__field-label">Source ID</span>
        <input
          v-model="sourceId"
          type="text"
          class="roadmap-rule-form__input"
          placeholder="fe-meu-padrao"
          required
        />
      </label>
    </div>

    <label class="roadmap-rule-form__field">
      <span class="roadmap-rule-form__field-label">Titulo</span>
      <input
        v-model="title"
        type="text"
        class="roadmap-rule-form__input"
        required
        @blur="onTitleBlur"
      />
    </label>

    <label class="roadmap-rule-form__field">
      <span class="roadmap-rule-form__field-label">Regra</span>
      <textarea v-model="body" class="roadmap-rule-form__textarea" rows="3" required></textarea>
    </label>

    <label class="roadmap-rule-form__field">
      <span class="roadmap-rule-form__field-label">Por que</span>
      <textarea v-model="why" class="roadmap-rule-form__textarea" rows="2"></textarea>
    </label>

    <label class="roadmap-rule-form__field">
      <span class="roadmap-rule-form__field-label">Aplica quando</span>
      <textarea v-model="appliesWhen" class="roadmap-rule-form__textarea" rows="2"></textarea>
    </label>

    <div class="roadmap-rule-form__actions">
      <button
        type="button"
        class="roadmap-rule-form__btn roadmap-rule-form__btn--ghost"
        @click="emit('cancel')"
      >
        Cancelar
      </button>
      <button type="submit" class="roadmap-rule-form__btn roadmap-rule-form__btn--primary">
        Criar regra
      </button>
    </div>
  </form>
</template>

<style scoped>
.roadmap-rule-form {
  display: grid;
  gap: 0.65rem;
  padding: 1rem 1.1rem;
  border: 1px dashed rgb(var(--primary) / 0.5);
  border-radius: 12px;
  background: rgb(var(--primary) / 0.04);
}

.roadmap-rule-form__head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
}

.roadmap-rule-form__title {
  margin: 0;
  font-size: 0.95rem;
  font-weight: 700;
  color: var(--text-main);
}

.roadmap-rule-form__row {
  display: grid;
  gap: 0.65rem;
}

.roadmap-rule-form__row--two {
  grid-template-columns: 1fr 1fr;
}

.roadmap-rule-form__field {
  display: grid;
  gap: 0.3rem;
}

.roadmap-rule-form__field-label {
  font-size: 0.7rem;
  font-weight: 800;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--text-muted);
}

.roadmap-rule-form__input,
.roadmap-rule-form__select,
.roadmap-rule-form__textarea {
  width: 100%;
  padding: 0.45rem 0.65rem;
  border: 1px solid var(--admin-header-border);
  border-radius: 9px;
  background: var(--admin-header-panel-bg);
  color: var(--text-main);
  font-size: 0.85rem;
  font-family: inherit;
}

.roadmap-rule-form__textarea {
  resize: vertical;
  line-height: 1.4;
}

.roadmap-rule-form__actions {
  display: inline-flex;
  justify-content: flex-end;
  gap: 0.45rem;
}

.roadmap-rule-form__btn {
  padding: 0.45rem 0.95rem;
  border: 1px solid var(--admin-header-border);
  border-radius: 10px;
  background: transparent;
  color: var(--text-main);
  font-size: 0.8rem;
  font-weight: 700;
  cursor: pointer;
  transition:
    border-color 0.16s ease,
    background 0.16s ease;
}

.roadmap-rule-form__btn--ghost:hover {
  background: var(--admin-header-hover-bg);
}

.roadmap-rule-form__btn--primary {
  background: rgb(var(--primary) / 0.18);
  border-color: rgb(var(--primary) / 0.4);
  color: rgb(var(--primary));
}

.roadmap-rule-form__btn--primary:hover {
  background: rgb(var(--primary) / 0.28);
}
</style>
