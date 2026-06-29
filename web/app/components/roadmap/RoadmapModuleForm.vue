<script setup lang="ts">
import { ref } from 'vue'
import {
  ROADMAP_MODULE_STATUS_LABEL,
  ROADMAP_PRIORITY_LABEL,
  type ModulePriority,
  type ModuleStatus,
} from '~/components/roadmap/roadmap-data'
import { slugify } from '~/domain/utils/slugify'

const STATUS_OPTIONS: ModuleStatus[] = ['pending', 'in_progress', 'beta', 'done']
const PRIORITY_OPTIONS: ModulePriority[] = ['P0', 'P1', 'P2', 'P3']
const CATEGORIES = [
  { id: 'atendimento', label: 'Atendimento' },
  { id: 'tools', label: 'Tools' },
  { id: 'operacao-comercial', label: 'Operacao comercial' },
  { id: 'indicadores', label: 'Indicadores' },
  { id: 'manage', label: 'Manage' },
]

const emit = defineEmits<{
  (
    e: 'submit',
    payload: {
      sourceId: string
      label: string
      route: string
      status: ModuleStatus
      priority: ModulePriority
      category: string
      description: string
    },
  ): void
  (e: 'cancel'): void
}>()

const sourceId = ref('')
const label = ref('')
const route = ref('')
const status = ref<ModuleStatus>('pending')
const priority = ref<ModulePriority>('P2')
const category = ref('atendimento')
const description = ref('')

function onLabelBlur() {
  if (!sourceId.value.trim() && label.value.trim()) {
    sourceId.value = slugify(label.value).slice(0, 40)
  }
  if (!route.value.trim() && label.value.trim()) {
    route.value = `/${slugify(label.value).slice(0, 40)}`
  }
}

function submit() {
  if (!sourceId.value.trim() || !label.value.trim() || !description.value.trim()) {
    return
  }
  emit('submit', {
    sourceId: sourceId.value.trim(),
    label: label.value.trim(),
    route: route.value.trim(),
    status: status.value,
    priority: priority.value,
    category: category.value,
    description: description.value.trim(),
  })
  resetForm()
}

function resetForm() {
  sourceId.value = ''
  label.value = ''
  route.value = ''
  status.value = 'pending'
  priority.value = 'P2'
  category.value = 'atendimento'
  description.value = ''
}
</script>

<template>
  <form class="roadmap-module-form" @submit.prevent="submit">
    <header class="roadmap-module-form__head">
      <h4 class="roadmap-module-form__title">Novo modulo</h4>
    </header>

    <label class="roadmap-module-form__field">
      <span class="roadmap-module-form__field-label">Nome</span>
      <input
        v-model="label"
        type="text"
        class="roadmap-module-form__input"
        placeholder="Ex: WhatsApp Cloud"
        required
        @blur="onLabelBlur"
      />
    </label>

    <div class="roadmap-module-form__row roadmap-module-form__row--two">
      <label class="roadmap-module-form__field">
        <span class="roadmap-module-form__field-label">Source ID</span>
        <input
          v-model="sourceId"
          type="text"
          class="roadmap-module-form__input"
          placeholder="whatsapp-cloud"
          required
        />
      </label>
      <label class="roadmap-module-form__field">
        <span class="roadmap-module-form__field-label">Rota</span>
        <input
          v-model="route"
          type="text"
          class="roadmap-module-form__input"
          placeholder="/whatsapp-cloud"
        />
      </label>
    </div>

    <div class="roadmap-module-form__row roadmap-module-form__row--three">
      <label class="roadmap-module-form__field">
        <span class="roadmap-module-form__field-label">Status</span>
        <select v-model="status" class="roadmap-module-form__select">
          <option v-for="s in STATUS_OPTIONS" :key="s" :value="s">
            {{ ROADMAP_MODULE_STATUS_LABEL[s] }}
          </option>
        </select>
      </label>
      <label class="roadmap-module-form__field">
        <span class="roadmap-module-form__field-label">Prioridade</span>
        <select v-model="priority" class="roadmap-module-form__select">
          <option v-for="p in PRIORITY_OPTIONS" :key="p" :value="p">
            {{ ROADMAP_PRIORITY_LABEL[p] }}
          </option>
        </select>
      </label>
      <label class="roadmap-module-form__field">
        <span class="roadmap-module-form__field-label">Categoria</span>
        <select v-model="category" class="roadmap-module-form__select">
          <option v-for="c in CATEGORIES" :key="c.id" :value="c.id">
            {{ c.label }}
          </option>
        </select>
      </label>
    </div>

    <label class="roadmap-module-form__field">
      <span class="roadmap-module-form__field-label">Descricao</span>
      <textarea
        v-model="description"
        class="roadmap-module-form__textarea"
        rows="3"
        required
      ></textarea>
    </label>

    <div class="roadmap-module-form__actions">
      <button
        type="button"
        class="roadmap-module-form__btn roadmap-module-form__btn--ghost"
        @click="emit('cancel')"
      >
        Cancelar
      </button>
      <button type="submit" class="roadmap-module-form__btn roadmap-module-form__btn--primary">
        Criar modulo
      </button>
    </div>
  </form>
</template>

<style scoped>
.roadmap-module-form {
  display: grid;
  gap: 0.65rem;
  padding: 1rem 1.1rem;
  border: 1px dashed rgb(var(--primary) / 0.5);
  border-radius: 12px;
  background: rgb(var(--primary) / 0.04);
}

.roadmap-module-form__head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
}

.roadmap-module-form__title {
  margin: 0;
  font-size: 0.95rem;
  font-weight: 700;
  color: var(--text-main);
}

.roadmap-module-form__row {
  display: grid;
  gap: 0.65rem;
}

.roadmap-module-form__row--two {
  grid-template-columns: 1fr 1fr;
}

.roadmap-module-form__row--three {
  grid-template-columns: 1fr 1fr 1fr;
}

.roadmap-module-form__field {
  display: grid;
  gap: 0.3rem;
}

.roadmap-module-form__field-label {
  font-size: 0.7rem;
  font-weight: 800;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--text-muted);
}

.roadmap-module-form__input,
.roadmap-module-form__select,
.roadmap-module-form__textarea {
  width: 100%;
  padding: 0.45rem 0.65rem;
  border: 1px solid var(--admin-header-border);
  border-radius: 9px;
  background: var(--admin-header-panel-bg);
  color: var(--text-main);
  font-size: 0.85rem;
  font-family: inherit;
}

.roadmap-module-form__textarea {
  resize: vertical;
  line-height: 1.4;
}

.roadmap-module-form__actions {
  display: inline-flex;
  justify-content: flex-end;
  gap: 0.45rem;
}

.roadmap-module-form__btn {
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

.roadmap-module-form__btn--ghost:hover {
  background: var(--admin-header-hover-bg);
}

.roadmap-module-form__btn--primary {
  background: rgb(var(--primary) / 0.18);
  border-color: rgb(var(--primary) / 0.4);
  color: rgb(var(--primary));
}

.roadmap-module-form__btn--primary:hover {
  background: rgb(var(--primary) / 0.28);
}
</style>
