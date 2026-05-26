<script setup lang="ts">
import { ref, watch } from 'vue'
import type { RoadmapRule } from '~/components/roadmap/roadmap-data'

interface EditableRule extends RoadmapRule {
  sourceId?: string
  isGlobal?: boolean
}

const props = defineProps<{ rule: EditableRule; editable?: boolean }>()

const emit = defineEmits<{
  (e: 'update', payload: { title: string; body: string; why: string; appliesWhen: string }): void
  (e: 'delete'): void
}>()

const editing = ref(false)
const draftTitle = ref(props.rule.title)
const draftBody = ref(props.rule.body)
const draftWhy = ref(props.rule.why || '')
const draftAppliesWhen = ref(props.rule.appliesWhen || '')

watch(
  () => [props.rule.title, props.rule.body, props.rule.why, props.rule.appliesWhen],
  () => {
    draftTitle.value = props.rule.title
    draftBody.value = props.rule.body
    draftWhy.value = props.rule.why || ''
    draftAppliesWhen.value = props.rule.appliesWhen || ''
  },
)

function startEdit() {
  draftTitle.value = props.rule.title
  draftBody.value = props.rule.body
  draftWhy.value = props.rule.why || ''
  draftAppliesWhen.value = props.rule.appliesWhen || ''
  editing.value = true
}

function cancelEdit() {
  editing.value = false
}

function save() {
  emit('update', {
    title: draftTitle.value.trim(),
    body: draftBody.value.trim(),
    why: draftWhy.value.trim(),
    appliesWhen: draftAppliesWhen.value.trim(),
  })
  editing.value = false
}
</script>

<template>
  <article class="roadmap-rule-card">
    <header class="roadmap-rule-card__head">
      <div class="roadmap-rule-card__title-wrap">
        <h4 class="roadmap-rule-card__title">{{ rule.title }}</h4>
        <code class="roadmap-rule-card__id">{{ rule.sourceId || rule.id }}</code>
        <span
          v-if="rule.isGlobal === false"
          class="roadmap-rule-card__override"
          title="Override por account"
        >
          override
        </span>
      </div>
      <div v-if="editable && !editing" class="roadmap-rule-card__head-actions">
        <button type="button" class="roadmap-rule-card__edit-btn" @click="startEdit">Editar</button>
        <button
          v-if="rule.isGlobal === false"
          type="button"
          class="roadmap-rule-card__delete-btn"
          title="Apagar override (volta ao seed global)"
          @click="emit('delete')"
        >
          Apagar
        </button>
      </div>
    </header>

    <template v-if="!editing">
      <p class="roadmap-rule-card__body">{{ rule.body }}</p>

      <dl v-if="rule.why || rule.appliesWhen" class="roadmap-rule-card__meta">
        <div v-if="rule.why" class="roadmap-rule-card__meta-row">
          <dt class="roadmap-rule-card__meta-label">Por que</dt>
          <dd class="roadmap-rule-card__meta-value">{{ rule.why }}</dd>
        </div>
        <div v-if="rule.appliesWhen" class="roadmap-rule-card__meta-row">
          <dt class="roadmap-rule-card__meta-label">Aplica quando</dt>
          <dd class="roadmap-rule-card__meta-value">{{ rule.appliesWhen }}</dd>
        </div>
      </dl>
    </template>

    <form v-else class="roadmap-rule-card__form" @submit.prevent="save">
      <label class="roadmap-rule-card__field">
        <span class="roadmap-rule-card__field-label">Titulo</span>
        <input v-model="draftTitle" type="text" class="roadmap-rule-card__input" />
      </label>
      <label class="roadmap-rule-card__field">
        <span class="roadmap-rule-card__field-label">Regra</span>
        <textarea v-model="draftBody" class="roadmap-rule-card__textarea" rows="4"></textarea>
      </label>
      <label class="roadmap-rule-card__field">
        <span class="roadmap-rule-card__field-label">Por que</span>
        <textarea v-model="draftWhy" class="roadmap-rule-card__textarea" rows="2"></textarea>
      </label>
      <label class="roadmap-rule-card__field">
        <span class="roadmap-rule-card__field-label">Aplica quando</span>
        <textarea
          v-model="draftAppliesWhen"
          class="roadmap-rule-card__textarea"
          rows="2"
        ></textarea>
      </label>
      <div class="roadmap-rule-card__actions">
        <button
          type="button"
          class="roadmap-rule-card__btn roadmap-rule-card__btn--ghost"
          @click="cancelEdit"
        >
          Cancelar
        </button>
        <button type="submit" class="roadmap-rule-card__btn roadmap-rule-card__btn--primary">
          Salvar
        </button>
      </div>
    </form>
  </article>
</template>

<style scoped>
.roadmap-rule-card {
  display: grid;
  gap: 0.65rem;
  padding: 1rem 1.1rem;
  border: 1px solid var(--admin-header-border);
  border-radius: 12px;
  background: var(--admin-header-panel-bg);
  color: var(--text-main);
}

.roadmap-rule-card__head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 0.6rem;
}

.roadmap-rule-card__title-wrap {
  display: inline-flex;
  align-items: baseline;
  flex-wrap: wrap;
  gap: 0.45rem;
  min-width: 0;
}

.roadmap-rule-card__title {
  margin: 0;
  font-size: 0.95rem;
  font-weight: 700;
  line-height: 1.25;
}

.roadmap-rule-card__id {
  font-family: ui-monospace, SFMono-Regular, monospace;
  font-size: 0.68rem;
  color: var(--text-muted);
  padding: 0.05rem 0.4rem;
  border-radius: 5px;
  background: rgb(var(--muted) / 0.35);
}

.roadmap-rule-card__override {
  padding: 0.05rem 0.4rem;
  border-radius: 5px;
  background: rgb(var(--info) / 0.18);
  color: rgb(var(--info));
  font-size: 0.62rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.roadmap-rule-card__head-actions {
  display: inline-flex;
  gap: 0.35rem;
  align-items: center;
}

.roadmap-rule-card__edit-btn,
.roadmap-rule-card__delete-btn {
  padding: 0.2rem 0.65rem;
  border: 1px solid var(--admin-header-border);
  border-radius: 8px;
  background: transparent;
  color: var(--text-muted);
  font-size: 0.72rem;
  font-weight: 700;
  cursor: pointer;
  transition:
    border-color 0.16s ease,
    color 0.16s ease,
    background 0.16s ease;
}

.roadmap-rule-card__edit-btn:hover {
  border-color: rgb(var(--ring) / 0.4);
  background: var(--admin-header-hover-bg);
  color: var(--text-main);
}

.roadmap-rule-card__delete-btn:hover {
  border-color: rgb(var(--danger) / 0.5);
  background: rgb(var(--danger) / 0.1);
  color: rgb(var(--danger));
}

.roadmap-rule-card__body {
  margin: 0;
  font-size: 0.85rem;
  line-height: 1.5;
  color: var(--text-main);
}

.roadmap-rule-card__meta {
  display: grid;
  gap: 0.45rem;
  margin: 0;
  padding: 0.6rem 0.75rem;
  border-radius: 9px;
  background: rgb(var(--muted) / 0.2);
}

.roadmap-rule-card__meta-row {
  display: grid;
  gap: 0.12rem;
  margin: 0;
}

.roadmap-rule-card__meta-label {
  font-size: 0.65rem;
  font-weight: 800;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--text-muted);
}

.roadmap-rule-card__meta-value {
  margin: 0;
  font-size: 0.82rem;
  line-height: 1.4;
  color: var(--text-main);
}

.roadmap-rule-card__form {
  display: grid;
  gap: 0.55rem;
}

.roadmap-rule-card__field {
  display: grid;
  gap: 0.3rem;
}

.roadmap-rule-card__field-label {
  font-size: 0.7rem;
  font-weight: 800;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--text-muted);
}

.roadmap-rule-card__input,
.roadmap-rule-card__textarea {
  width: 100%;
  padding: 0.45rem 0.65rem;
  border: 1px solid var(--admin-header-border);
  border-radius: 9px;
  background: var(--admin-header-panel-bg);
  color: var(--text-main);
  font-size: 0.85rem;
  font-family: inherit;
}

.roadmap-rule-card__textarea {
  resize: vertical;
  line-height: 1.4;
}

.roadmap-rule-card__actions {
  display: inline-flex;
  justify-content: flex-end;
  gap: 0.45rem;
}

.roadmap-rule-card__btn {
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

.roadmap-rule-card__btn--ghost:hover {
  background: var(--admin-header-hover-bg);
}

.roadmap-rule-card__btn--primary {
  background: rgb(var(--primary) / 0.18);
  border-color: rgb(var(--primary) / 0.4);
  color: rgb(var(--primary));
}

.roadmap-rule-card__btn--primary:hover {
  background: rgb(var(--primary) / 0.28);
}
</style>
