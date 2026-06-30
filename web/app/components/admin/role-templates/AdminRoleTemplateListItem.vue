<script setup lang="ts">
import type { AvailablePermission } from '~/types/admin-users'
import type { RoleTemplateSummary } from '~/types/admin-role-templates'
import AdminRoleTemplateMatrix from '~/components/admin/role-templates/AdminRoleTemplateMatrix.vue'

// Uma linha da lista de papeis-padrao: cabecalho (nome + id + badges) e, quando
// expandida, o editor. Templates de sistema (isSystem || isLocked) sao READ-ONLY:
// mostram cadeado, campos desabilitados e a matriz em modo somente-leitura; nao ha
// botao salvar/remover. Custom: edita label/descricao (PATCH) + matriz (PUT) e pode
// remover (DELETE). O rascunho re-hidrata do template autoritativo a cada abertura;
// o host re-le do backend apos cada escrita e re-passa o template por prop.

const props = defineProps<{
  template: RoleTemplateSummary
  available: AvailablePermission[]
  expanded: boolean
  savingMeta: boolean
  savingPerms: boolean
  deleting: boolean
}>()

const emit = defineEmits<{
  toggle: []
  'save-meta': [{ label: string; description: string }]
  'save-perms': [string[]]
  remove: []
}>()

// Read-only = template de sistema OU bloqueado (defesa em profundidade no front;
// o backend tambem bloqueia a escrita).
const readonly = computed(() => props.template.isSystem || props.template.isLocked)

// Rascunho de edicao — re-hidrata do template autoritativo sempre que a linha abre
// ou o template muda (apos o host re-ler do backend). So fica "solto" enquanto a
// edicao esta aberta e o usuario mexeu.
const editLabel = ref('')
const editDescription = ref('')
const editPermissions = ref<Set<string>>(new Set())

function hydrate() {
  editLabel.value = props.template.label
  editDescription.value = props.template.description || ''
  editPermissions.value = new Set(props.template.permissionKeys)
}

watch(
  () => [props.expanded, props.template] as const,
  ([open]) => {
    if (open) hydrate()
  },
  { immediate: true, deep: true },
)

function togglePermission(key: string, value: boolean) {
  const next = new Set(editPermissions.value)
  if (value) next.add(key)
  else next.delete(key)
  editPermissions.value = next
}

function onSaveMeta() {
  if (readonly.value || props.savingMeta) return
  if (!editLabel.value.trim()) return
  emit('save-meta', {
    label: editLabel.value.trim(),
    description: editDescription.value.trim(),
  })
}

function onSavePerms() {
  if (readonly.value || props.savingPerms) return
  emit('save-perms', [...editPermissions.value])
}
</script>

<template>
  <li class="role-template-item">
    <div class="role-template-item__head">
      <button
        class="role-template-item__toggle"
        type="button"
        :aria-expanded="expanded ? 'true' : 'false'"
        @click="emit('toggle')"
      >
        <span class="role-template-item__name">
          {{ template.label }}
          <span
            v-if="readonly"
            class="role-template-item__lock"
            title="Papel-padrao de sistema (somente leitura)"
          >
            <UIcon name="i-lucide-lock" class="role-template-item__lock-icon" />
            sistema
          </span>
        </span>
        <span class="role-template-item__id">{{ template.id }}</span>
      </button>
      <button
        v-if="!readonly"
        class="role-template-item__delete-btn"
        type="button"
        title="Remover papel-padrao"
        :disabled="deleting"
        @click="emit('remove')"
      >
        {{ deleting ? 'Removendo...' : 'Remover' }}
      </button>
    </div>

    <div v-if="expanded" class="role-template-item__detail">
      <div class="role-template-item__fields">
        <label class="role-template-item__field">
          <span class="role-template-item__label">
            Nome
            <em v-if="!readonly">*</em>
          </span>
          <input
            v-model="editLabel"
            class="role-template-item__input"
            type="text"
            :disabled="readonly"
          />
        </label>
        <label class="role-template-item__field role-template-item__field--wide">
          <span class="role-template-item__label">Descricao</span>
          <input
            v-model="editDescription"
            class="role-template-item__input"
            type="text"
            :disabled="readonly"
          />
        </label>
      </div>

      <footer v-if="!readonly" class="role-template-item__meta-actions">
        <p v-if="!editLabel.trim()" class="role-template-item__missing">
          Informe o nome para salvar.
        </p>
        <button
          class="role-template-item__save-btn"
          type="button"
          :disabled="!editLabel.trim() || savingMeta"
          @click="onSaveMeta"
        >
          {{ savingMeta ? 'Salvando...' : 'Salvar dados' }}
        </button>
      </footer>

      <AdminRoleTemplateMatrix
        :available="available"
        :checked-keys="editPermissions"
        :readonly="readonly"
        @toggle="togglePermission"
      />

      <footer v-if="!readonly" class="role-template-item__perms-actions">
        <button
          class="role-template-item__save-btn"
          type="button"
          :disabled="savingPerms"
          @click="onSavePerms"
        >
          {{ savingPerms ? 'Salvando...' : 'Salvar permissoes' }}
        </button>
      </footer>
    </div>
  </li>
</template>

<style scoped>
.role-template-item {
  border-radius: var(--radius-sm);
  border: 1px solid rgb(var(--border) / 0.8);
  background: rgb(var(--surface) / 0.7);
  overflow: hidden;
}

.role-template-item__head {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  padding: 0.5rem 0.7rem;
}

.role-template-item__toggle {
  display: flex;
  flex: 1;
  align-items: center;
  justify-content: space-between;
  gap: 0.6rem;
  min-width: 0;
  padding: 0;
  border: 0;
  background: transparent;
  color: inherit;
  text-align: left;
  cursor: pointer;
}

.role-template-item__name {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  font-size: 0.84rem;
  font-weight: 600;
  color: rgb(var(--text));
}

.role-template-item__lock {
  display: inline-flex;
  align-items: center;
  gap: 0.2rem;
  font-size: 0.66rem;
  padding: 0.05rem 0.4rem;
  border-radius: 999px;
  background: rgb(var(--muted) / 0.18);
  color: rgb(var(--muted));
}

.role-template-item__lock-icon {
  width: 0.7rem;
  height: 0.7rem;
}

.role-template-item__id {
  font-size: 0.72rem;
  color: rgb(var(--muted));
}

.role-template-item__delete-btn,
.role-template-item__save-btn {
  flex-shrink: 0;
  min-height: 2.1rem;
  padding: 0 0.85rem;
  border-radius: 999px;
  font-size: 0.76rem;
  font-weight: 700;
  cursor: pointer;
}

.role-template-item__delete-btn {
  border: 1px solid rgb(var(--danger) / 0.35);
  background: rgb(var(--danger) / 0.12);
  color: rgb(var(--danger));
}

.role-template-item__save-btn {
  border: 1px solid rgb(var(--primary) / 0.4);
  background: rgb(var(--primary) / 0.16);
  color: rgb(var(--primary-600));
}

.role-template-item__delete-btn:disabled,
.role-template-item__save-btn:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.role-template-item__detail {
  display: grid;
  gap: 0.75rem;
  padding: 0.8rem;
  border-top: 1px solid rgb(var(--border) / 0.7);
  background: rgb(var(--surface-2) / 0.5);
}

.role-template-item__fields {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(13rem, 1fr));
  gap: 0.7rem;
}

.role-template-item__field {
  display: grid;
  gap: 0.3rem;
  min-width: 0;
}

.role-template-item__field--wide {
  grid-column: 1 / -1;
}

.role-template-item__label {
  font-size: 0.74rem;
  font-weight: 600;
  color: rgb(var(--muted));
}

.role-template-item__label em {
  color: rgb(var(--danger));
  font-style: normal;
}

.role-template-item__input {
  min-height: 2.2rem;
  padding: 0 0.7rem;
  border-radius: var(--radius-sm);
  border: 1px solid rgb(var(--border) / 0.9);
  background: rgb(var(--surface));
  color: rgb(var(--text));
  font-size: 0.82rem;
}

.role-template-item__input:disabled {
  opacity: 0.6;
}

.role-template-item__meta-actions,
.role-template-item__perms-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.7rem;
  flex-wrap: wrap;
}

.role-template-item__missing {
  margin: 0;
  margin-right: auto;
  font-size: 0.74rem;
  color: rgb(var(--danger));
}
</style>
