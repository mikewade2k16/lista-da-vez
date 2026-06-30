<script setup lang="ts">
import type { AvailablePermission } from '~/types/admin-users'
import type { RoleTemplateCreateInput } from '~/types/admin-role-templates'
import { slugify } from '~/domain/utils/slugify'
import AdminRoleTemplateMatrix from '~/components/admin/role-templates/AdminRoleTemplateMatrix.vue'

// Form de criacao de papel-padrao (template global). Mantem o rascunho LOCAL (id/
// label/descricao + matriz) e o feedback de "o que falta" (nunca botao morto e
// silencioso). Emite `submit` com o payload; o host cuida da chamada + reset (fecha
// o form, que desmonta e zera no proximo open). O estado de "criando" vem do host.
//
// Slug: sugerido a partir do label enquanto o usuario nao mexer no id manualmente
// (flag idTouched). Charset permitido do contrato: minusculas/digitos/'.'/'_'/'-'.

const props = defineProps<{
  creating: boolean
  available: AvailablePermission[]
}>()

const emit = defineEmits<{ submit: [RoleTemplateCreateInput] }>()

const id = ref('')
const label = ref('')
const description = ref('')
// permissions = Set das keys marcadas na matriz.
const permissions = ref<Set<string>>(new Set())
// Marca quando o usuario edita o id a mao — a partir dai paramos de sugerir do label.
const idTouched = ref(false)

// Mantem so o charset permitido do contrato (minusculas/digitos/'.'/'_'/'-').
function sanitizeId(value: string): string {
  return String(value || '')
    .toLowerCase()
    .replace(/[^a-z0-9._-]+/g, '-')
    .replace(/-+/g, '-')
    .replace(/^[-._]+|[-._]+$/g, '')
}

// Enquanto o id nao foi tocado, sugere do label (slugify canonico -> charset valido).
watch(label, (next) => {
  if (idTouched.value) return
  id.value = slugify(next)
})

function onIdInput(raw: string) {
  idTouched.value = true
  id.value = sanitizeId(raw)
}

function togglePermission(key: string, value: boolean) {
  const next = new Set(permissions.value)
  if (value) next.add(key)
  else next.delete(key)
  permissions.value = next
}

const missing = computed(() => {
  const items: string[] = []
  if (!id.value.trim()) items.push('id (slug)')
  if (!label.value.trim()) items.push('nome')
  return items
})
const canSubmit = computed(() => missing.value.length === 0 && !props.creating)

function onSubmit() {
  if (!canSubmit.value) return
  emit('submit', {
    id: id.value.trim(),
    label: label.value.trim(),
    description: description.value.trim(),
    permissionKeys: [...permissions.value],
  })
}
</script>

<template>
  <form class="role-template-form" @submit.prevent="onSubmit">
    <div class="role-template-form__grid">
      <label class="role-template-form__field">
        <span class="role-template-form__label">
          Id (slug)
          <em>*</em>
        </span>
        <input
          :value="id"
          class="role-template-form__input"
          type="text"
          placeholder="ex.: gerente-loja"
          autocomplete="off"
          @input="onIdInput(($event.target as HTMLInputElement).value)"
        />
      </label>
      <label class="role-template-form__field">
        <span class="role-template-form__label">
          Nome
          <em>*</em>
        </span>
        <input
          v-model="label"
          class="role-template-form__input"
          type="text"
          placeholder="ex.: Gerente da loja"
          autocomplete="off"
        />
      </label>
      <label class="role-template-form__field role-template-form__field--wide">
        <span class="role-template-form__label">Descricao (opcional)</span>
        <input
          v-model="description"
          class="role-template-form__input"
          type="text"
          placeholder="Para que serve este papel-padrao"
          autocomplete="off"
        />
      </label>
    </div>

    <div class="role-template-form__matrix">
      <h5 class="role-template-form__matrix-title">Permissoes do papel-padrao</h5>
      <AdminRoleTemplateMatrix
        :available="available"
        :checked-keys="permissions"
        @toggle="togglePermission"
      />
    </div>

    <div class="role-template-form__actions">
      <p v-if="missing.length" class="role-template-form__missing">
        Informe: {{ missing.join(', ') }}
      </p>
      <button class="role-template-form__save-btn" type="submit" :disabled="!canSubmit">
        {{ creating ? 'Criando...' : 'Criar papel-padrao' }}
      </button>
    </div>
  </form>
</template>

<style scoped>
.role-template-form {
  display: grid;
  gap: 0.85rem;
  padding: 0.85rem;
  border-radius: var(--radius-sm);
  border: 1px solid rgb(var(--border) / 0.8);
  background: rgb(var(--surface) / 0.7);
}

.role-template-form__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(13rem, 1fr));
  gap: 0.7rem;
}

.role-template-form__field {
  display: grid;
  gap: 0.3rem;
  min-width: 0;
}

.role-template-form__field--wide {
  grid-column: 1 / -1;
}

.role-template-form__label {
  font-size: 0.74rem;
  font-weight: 600;
  color: rgb(var(--muted));
}

.role-template-form__label em {
  color: rgb(var(--danger));
  font-style: normal;
}

.role-template-form__input {
  min-height: 2.2rem;
  padding: 0 0.7rem;
  border-radius: var(--radius-sm);
  border: 1px solid rgb(var(--border) / 0.9);
  background: rgb(var(--surface));
  color: rgb(var(--text));
  font-size: 0.82rem;
}

.role-template-form__matrix {
  display: grid;
  gap: 0.5rem;
}

.role-template-form__matrix-title {
  margin: 0;
  font-size: 0.8rem;
  font-weight: 600;
  color: rgb(var(--text));
}

.role-template-form__actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.7rem;
  flex-wrap: wrap;
}

.role-template-form__missing {
  margin: 0;
  margin-right: auto;
  font-size: 0.74rem;
  color: rgb(var(--danger));
}

.role-template-form__save-btn {
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

.role-template-form__save-btn:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}
</style>
