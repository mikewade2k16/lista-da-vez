<script setup lang="ts">
import type { RoleCreateInput } from '~/types/admin-users'

// Form de criacao de papel customizado, fatiado do AdminRoleMatrixEditor. Mantem o
// rascunho LOCAL (code/label/descricao) e o feedback de "o que falta" (nunca botao
// morto e silencioso). Emite `submit` com o payload; o pai cuida da chamada ao
// backend + reset (fecha o form, que desmonta e zera no proximo open). O estado de
// "criando" vem do pai (saving granular do manager). Comportamento IDENTICO ao inline.

const props = defineProps<{ creating: boolean }>()
const emit = defineEmits<{ submit: [RoleCreateInput] }>()

const newCode = ref('')
const newLabel = ref('')
const newDescription = ref('')

// Feedback de formulario: o que falta para criar.
const createMissing = computed(() => {
  const missing: string[] = []
  if (!newCode.value.trim()) missing.push('code (slug)')
  if (!newLabel.value.trim()) missing.push('nome')
  return missing
})
const canCreate = computed(() => createMissing.value.length === 0 && !props.creating)

function onSubmit() {
  if (!canCreate.value) return
  emit('submit', {
    code: newCode.value.trim(),
    label: newLabel.value.trim(),
    description: newDescription.value.trim(),
  })
}
</script>

<template>
  <form class="role-create-form" @submit.prevent="onSubmit">
    <div class="role-create-form__grid">
      <label class="role-create-form__field">
        <span class="role-create-form__label">
          Code (slug)
          <em>*</em>
        </span>
        <input
          v-model="newCode"
          class="role-create-form__input"
          type="text"
          placeholder="ex.: gerente-loja"
          autocomplete="off"
        />
      </label>
      <label class="role-create-form__field">
        <span class="role-create-form__label">
          Nome
          <em>*</em>
        </span>
        <input
          v-model="newLabel"
          class="role-create-form__input"
          type="text"
          placeholder="ex.: Gerente da loja"
          autocomplete="off"
        />
      </label>
      <label class="role-create-form__field role-create-form__field--wide">
        <span class="role-create-form__label">Descricao (opcional)</span>
        <input
          v-model="newDescription"
          class="role-create-form__input"
          type="text"
          placeholder="Para que serve este papel"
          autocomplete="off"
        />
      </label>
    </div>
    <div class="role-create-form__actions">
      <p v-if="createMissing.length" class="role-create-form__missing">
        Informe: {{ createMissing.join(', ') }}
      </p>
      <button class="role-create-form__save-btn" type="submit" :disabled="!canCreate">
        {{ creating ? 'Criando...' : 'Criar papel' }}
      </button>
    </div>
  </form>
</template>

<style scoped>
.role-create-form {
  display: grid;
  gap: 0.75rem;
  padding: 0.8rem;
  border-radius: var(--radius-sm);
  border: 1px solid rgb(var(--border) / 0.8);
  background: rgb(var(--surface) / 0.7);
}

.role-create-form__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(13rem, 1fr));
  gap: 0.7rem;
}

.role-create-form__field {
  display: grid;
  gap: 0.3rem;
  min-width: 0;
}

.role-create-form__field--wide {
  grid-column: 1 / -1;
}

.role-create-form__label {
  font-size: 0.74rem;
  font-weight: 600;
  color: rgb(var(--muted));
}

.role-create-form__label em {
  color: rgb(var(--danger));
  font-style: normal;
}

.role-create-form__input {
  min-height: 2.2rem;
  padding: 0 0.7rem;
  border-radius: var(--radius-sm);
  border: 1px solid rgb(var(--border) / 0.9);
  background: rgb(var(--surface));
  color: rgb(var(--text));
  font-size: 0.82rem;
}

.role-create-form__actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.7rem;
  flex-wrap: wrap;
}

.role-create-form__missing {
  margin: 0;
  margin-right: auto;
  font-size: 0.74rem;
  color: rgb(var(--danger));
}

.role-create-form__save-btn {
  flex-shrink: 0;
  min-height: 2.1rem;
  padding: 0 0.85rem;
  border-radius: 999px;
  font-size: 0.76rem;
  font-weight: 700;
  cursor: pointer;
  border: 1px solid rgb(var(--primary) / 0.4);
  background: rgb(var(--primary) / 0.16);
  color: rgb(var(--primary-600));
}

.role-create-form__save-btn:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}
</style>
