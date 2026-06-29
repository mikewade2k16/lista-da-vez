<script setup lang="ts">
import type { AvailablePermission, RoleSummary } from '~/types/admin-users'

// Editor de papeis customizados (core.roles) de UMA account. Lista os papeis,
// permite selecionar um para editar label/descricao + a matriz de permissoes
// (checkbox por permissao agrupada por modulo), criar novos e deletar os
// nao-bloqueados. Emite `changed` sempre que a lista de papeis muda, para o
// painel-pai re-buscar. Apenas presentational + composable; nao toca em outros
// arquivos. A fonte de verdade e sempre a resposta do backend (re-le apos salvar).

const props = defineProps<{
  accountId: string
  // Catalogo de permissoes elegiveis da account (vem do `.available` do override).
  // E o universo de checkboxes da matriz, agrupado por moduleId.
  availablePermissions: AvailablePermission[]
}>()

const emit = defineEmits<{ changed: [] }>()

const r = useAccountRolesManager()

const roles = ref<RoleSummary[]>([])
const loadingRoles = ref(false)
// Papel selecionado para edicao da matriz.
const selectedRoleId = ref('')
const loadingDetail = ref(false)

// Rascunho de edicao do papel selecionado. Re-hidrata da resposta do backend
// (getRole) assim que ela chega; so se preserva o que o usuario ja mexeu por
// enquanto a edicao esta aberta. permissions = Set das keys marcadas.
const editLabel = ref('')
const editDescription = ref('')
const editPermissions = ref<Set<string>>(new Set())

// Form de criacao de novo papel.
const showCreate = ref(false)
const newCode = ref('')
const newLabel = ref('')
const newDescription = ref('')

// Permissoes agrupadas por moduleId, ordenadas, para a matriz. Cada grupo lista
// suas permissoes; o checkbox reflete se a key esta em editPermissions.
const groupedPermissions = computed(() => {
  const groups = new Map<string, AvailablePermission[]>()
  for (const perm of props.availablePermissions || []) {
    const moduleId = String(perm.moduleId || 'outros')
    if (!groups.has(moduleId)) groups.set(moduleId, [])
    groups.get(moduleId)!.push(perm)
  }
  return [...groups.entries()]
    .map(([moduleId, items]) => ({
      moduleId,
      items: [...items].sort((a, b) => a.label.localeCompare(b.label, 'pt-BR')),
    }))
    .sort((a, b) => a.moduleId.localeCompare(b.moduleId, 'pt-BR'))
})

const selectedRole = computed(
  () => roles.value.find((role) => role.id === selectedRoleId.value) || null,
)

// Chaves de saving granular do composable (espelham as do useAccountRolesManager).
const isSavingDetail = computed(() =>
  Boolean(
    props.accountId && selectedRoleId.value
      ? r.savingMap.value[`${props.accountId}:role:${selectedRoleId.value}`]
      : false,
  ),
)
const isCreating = computed(() =>
  Boolean(props.accountId ? r.savingMap.value[`${props.accountId}:role:create`] : false),
)

// Feedback de formulario: o que falta para criar (nunca botao morto e silencioso).
const createMissing = computed(() => {
  const missing: string[] = []
  if (!newCode.value.trim()) missing.push('code (slug)')
  if (!newLabel.value.trim()) missing.push('nome')
  return missing
})
const canCreate = computed(() => createMissing.value.length === 0 && !isCreating.value)

async function loadRoles() {
  if (!props.accountId) return
  loadingRoles.value = true
  roles.value = await r.listRoles(props.accountId)
  loadingRoles.value = false
}

async function selectRole(roleId: string) {
  const id = String(roleId || '').trim()
  if (!id) return
  // Toggle: clicar no papel ja aberto fecha a edicao.
  if (selectedRoleId.value === id) {
    selectedRoleId.value = ''
    return
  }
  selectedRoleId.value = id
  loadingDetail.value = true
  const detail = await r.getRole(props.accountId, id)
  loadingDetail.value = false
  if (!detail) {
    selectedRoleId.value = ''
    return
  }
  // Re-hidrata o rascunho a partir da resposta autoritativa do backend.
  editLabel.value = detail.role.label
  editDescription.value = detail.role.description || ''
  editPermissions.value = new Set(detail.permissions)
}

function isPermissionChecked(key: string): boolean {
  return editPermissions.value.has(key)
}

function togglePermission(key: string, value: boolean) {
  const next = new Set(editPermissions.value)
  if (value) next.add(key)
  else next.delete(key)
  editPermissions.value = next
}

async function saveDetail() {
  if (!selectedRoleId.value || isSavingDetail.value) return
  if (!editLabel.value.trim()) return
  const updated = await r.updateRole(props.accountId, selectedRoleId.value, {
    label: editLabel.value.trim(),
    description: editDescription.value.trim(),
    permissions: [...editPermissions.value],
  })
  // Papel bloqueado pode ser rejeitado pelo backend; o erro aparece via
  // r.errorMessage e o detalhe permanece aberto para o usuario reavaliar.
  if (!updated) return
  await loadRoles()
  emit('changed')
}

async function createRole() {
  if (!canCreate.value) return
  const created = await r.createRole(props.accountId, {
    code: newCode.value.trim(),
    label: newLabel.value.trim(),
    description: newDescription.value.trim(),
  })
  if (!created) return
  newCode.value = ''
  newLabel.value = ''
  newDescription.value = ''
  showCreate.value = false
  await loadRoles()
  emit('changed')
  // Abre direto o papel recem-criado para o usuario montar a matriz dele.
  await selectRole(created.id)
}

async function removeRole(role: RoleSummary) {
  if (role.isLocked) return
  if (!import.meta.client) return
  const ok = window.confirm(`Remover o papel "${role.label}"? Esta acao nao pode ser desfeita.`)
  if (!ok) return
  const removed = await r.deleteRole(props.accountId, role.id)
  if (!removed) return
  if (selectedRoleId.value === role.id) selectedRoleId.value = ''
  await loadRoles()
  emit('changed')
}

onMounted(loadRoles)
</script>

<template>
  <section class="role-matrix-editor">
    <header class="role-matrix-editor__head">
      <div>
        <h4 class="role-matrix-editor__title">Papeis customizados deste cliente</h4>
        <p class="role-matrix-editor__hint">
          Selecione um papel para editar a matriz de permissoes ou crie um novo.
        </p>
      </div>
      <button class="role-matrix-editor__new-btn" type="button" @click="showCreate = !showCreate">
        {{ showCreate ? 'Cancelar' : 'Novo papel' }}
      </button>
    </header>

    <p v-if="r.errorMessage.value" class="role-matrix-editor__error">
      {{ r.errorMessage.value }}
    </p>

    <!-- Form de criacao de papel -->
    <form v-if="showCreate" class="role-matrix-editor__create" @submit.prevent="createRole">
      <div class="role-matrix-editor__create-grid">
        <label class="role-matrix-editor__field">
          <span class="role-matrix-editor__label">
            Code (slug)
            <em>*</em>
          </span>
          <input
            v-model="newCode"
            class="role-matrix-editor__input"
            type="text"
            placeholder="ex.: gerente-loja"
            autocomplete="off"
          />
        </label>
        <label class="role-matrix-editor__field">
          <span class="role-matrix-editor__label">
            Nome
            <em>*</em>
          </span>
          <input
            v-model="newLabel"
            class="role-matrix-editor__input"
            type="text"
            placeholder="ex.: Gerente da loja"
            autocomplete="off"
          />
        </label>
        <label class="role-matrix-editor__field role-matrix-editor__field--wide">
          <span class="role-matrix-editor__label">Descricao (opcional)</span>
          <input
            v-model="newDescription"
            class="role-matrix-editor__input"
            type="text"
            placeholder="Para que serve este papel"
            autocomplete="off"
          />
        </label>
      </div>
      <div class="role-matrix-editor__create-actions">
        <p v-if="createMissing.length" class="role-matrix-editor__missing">
          Informe: {{ createMissing.join(', ') }}
        </p>
        <button class="role-matrix-editor__save-btn" type="submit" :disabled="!canCreate">
          {{ isCreating ? 'Criando...' : 'Criar papel' }}
        </button>
      </div>
    </form>

    <!-- Lista de papeis -->
    <p v-if="loadingRoles" class="role-matrix-editor__loading">Carregando papeis...</p>
    <p v-else-if="!roles.length" class="role-matrix-editor__empty">
      Nenhum papel customizado neste cliente ainda. Crie o primeiro acima.
    </p>

    <ul v-else class="role-matrix-editor__list">
      <li v-for="role in roles" :key="role.id" class="role-matrix-editor__item">
        <div class="role-matrix-editor__item-head">
          <button
            class="role-matrix-editor__item-toggle"
            type="button"
            :aria-expanded="selectedRoleId === role.id ? 'true' : 'false'"
            @click="selectRole(role.id)"
          >
            <span class="role-matrix-editor__role-name">
              {{ role.label }}
              <span v-if="role.isLocked" class="role-matrix-editor__lock" title="Papel de sistema">
                cadeado
              </span>
              <span v-if="role.isDefault" class="role-matrix-editor__badge">padrao</span>
            </span>
            <span class="role-matrix-editor__role-code">{{ role.code }}</span>
          </button>
          <button
            v-if="!role.isLocked"
            class="role-matrix-editor__delete-btn"
            type="button"
            title="Remover papel"
            @click="removeRole(role)"
          >
            Remover
          </button>
        </div>

        <!-- Editor da matriz do papel selecionado -->
        <div v-if="selectedRoleId === role.id" class="role-matrix-editor__detail">
          <p v-if="loadingDetail" class="role-matrix-editor__loading">Carregando detalhe...</p>
          <template v-else>
            <div class="role-matrix-editor__detail-fields">
              <label class="role-matrix-editor__field">
                <span class="role-matrix-editor__label">
                  Nome
                  <em>*</em>
                </span>
                <input
                  v-model="editLabel"
                  class="role-matrix-editor__input"
                  type="text"
                  :disabled="role.isLocked"
                />
              </label>
              <label class="role-matrix-editor__field role-matrix-editor__field--wide">
                <span class="role-matrix-editor__label">Descricao</span>
                <input
                  v-model="editDescription"
                  class="role-matrix-editor__input"
                  type="text"
                  :disabled="role.isLocked"
                />
              </label>
            </div>

            <p v-if="role.isLocked" class="role-matrix-editor__locked-note">
              Papel de sistema: a edicao pode ser rejeitada pelo backend.
            </p>

            <div v-if="!groupedPermissions.length" class="role-matrix-editor__empty">
              Nenhuma permissao disponivel no catalogo deste cliente.
            </div>

            <div
              v-for="group in groupedPermissions"
              :key="group.moduleId"
              class="role-matrix-editor__group"
            >
              <h5 class="role-matrix-editor__group-title">{{ group.moduleId }}</h5>
              <div class="role-matrix-editor__perms">
                <label v-for="perm in group.items" :key="perm.key" class="role-matrix-editor__perm">
                  <input
                    type="checkbox"
                    :checked="isPermissionChecked(perm.key)"
                    @change="
                      togglePermission(perm.key, ($event.target as HTMLInputElement).checked)
                    "
                  />
                  <span class="role-matrix-editor__perm-copy">
                    <strong>{{ perm.label }}</strong>
                    <span class="role-matrix-editor__perm-key">{{ perm.key }}</span>
                  </span>
                </label>
              </div>
            </div>

            <footer class="role-matrix-editor__detail-actions">
              <p v-if="!editLabel.trim()" class="role-matrix-editor__missing">
                Informe o nome do papel para salvar.
              </p>
              <button
                class="role-matrix-editor__save-btn"
                type="button"
                :disabled="!editLabel.trim() || isSavingDetail"
                @click="saveDetail"
              >
                {{ isSavingDetail ? 'Salvando...' : 'Salvar papel' }}
              </button>
            </footer>
          </template>
        </div>
      </li>
    </ul>
  </section>
</template>

<style scoped>
.role-matrix-editor {
  display: grid;
  gap: 0.85rem;
  padding: 0.9rem;
  border-radius: var(--radius-md);
  border: 1px solid rgb(var(--border) / 0.8);
  background: rgb(var(--surface-2) / 0.6);
}

.role-matrix-editor__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
}

.role-matrix-editor__title {
  margin: 0;
  font-size: 0.9rem;
  color: rgb(var(--text));
}

.role-matrix-editor__hint {
  margin: 0.2rem 0 0;
  font-size: 0.78rem;
  color: rgb(var(--muted));
}

.role-matrix-editor__new-btn,
.role-matrix-editor__save-btn,
.role-matrix-editor__delete-btn {
  flex-shrink: 0;
  min-height: 2.1rem;
  padding: 0 0.85rem;
  border-radius: 999px;
  font-size: 0.76rem;
  font-weight: 700;
  cursor: pointer;
}

.role-matrix-editor__new-btn {
  border: 1px solid rgb(var(--border) / 0.9);
  background: rgb(var(--surface));
  color: rgb(var(--text));
}

.role-matrix-editor__save-btn {
  border: 1px solid rgb(var(--primary) / 0.4);
  background: rgb(var(--primary) / 0.16);
  color: rgb(var(--primary-600));
}

.role-matrix-editor__save-btn:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.role-matrix-editor__delete-btn {
  border: 1px solid rgb(var(--danger) / 0.35);
  background: rgb(var(--danger) / 0.12);
  color: rgb(var(--danger));
}

.role-matrix-editor__error {
  margin: 0;
  padding: 0.5rem 0.7rem;
  border-radius: var(--radius-sm);
  border: 1px solid rgb(var(--danger) / 0.3);
  background: rgb(var(--danger) / 0.1);
  color: rgb(var(--danger));
  font-size: 0.78rem;
}

.role-matrix-editor__create,
.role-matrix-editor__detail {
  display: grid;
  gap: 0.75rem;
  padding: 0.8rem;
  border-radius: var(--radius-sm);
  border: 1px solid rgb(var(--border) / 0.8);
  background: rgb(var(--surface) / 0.7);
}

.role-matrix-editor__create-grid,
.role-matrix-editor__detail-fields {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(13rem, 1fr));
  gap: 0.7rem;
}

.role-matrix-editor__field {
  display: grid;
  gap: 0.3rem;
  min-width: 0;
}

.role-matrix-editor__field--wide {
  grid-column: 1 / -1;
}

.role-matrix-editor__label {
  font-size: 0.74rem;
  font-weight: 600;
  color: rgb(var(--muted));
}

.role-matrix-editor__label em {
  color: rgb(var(--danger));
  font-style: normal;
}

.role-matrix-editor__input {
  min-height: 2.2rem;
  padding: 0 0.7rem;
  border-radius: var(--radius-sm);
  border: 1px solid rgb(var(--border) / 0.9);
  background: rgb(var(--surface));
  color: rgb(var(--text));
  font-size: 0.82rem;
}

.role-matrix-editor__input:disabled {
  opacity: 0.6;
}

.role-matrix-editor__create-actions,
.role-matrix-editor__detail-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.7rem;
  flex-wrap: wrap;
}

.role-matrix-editor__missing {
  margin: 0;
  margin-right: auto;
  font-size: 0.74rem;
  color: rgb(var(--danger));
}

.role-matrix-editor__loading,
.role-matrix-editor__empty {
  margin: 0;
  font-size: 0.8rem;
  color: rgb(var(--muted));
}

.role-matrix-editor__list {
  display: grid;
  gap: 0.6rem;
  margin: 0;
  padding: 0;
  list-style: none;
}

.role-matrix-editor__item {
  border-radius: var(--radius-sm);
  border: 1px solid rgb(var(--border) / 0.8);
  background: rgb(var(--surface) / 0.7);
  overflow: hidden;
}

.role-matrix-editor__item-head {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  padding: 0.5rem 0.7rem;
}

.role-matrix-editor__item-toggle {
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

.role-matrix-editor__role-name {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  font-size: 0.84rem;
  font-weight: 600;
  color: rgb(var(--text));
}

.role-matrix-editor__lock {
  font-size: 0.66rem;
  padding: 0.05rem 0.4rem;
  border-radius: 999px;
  background: rgb(var(--muted) / 0.18);
  color: rgb(var(--muted));
}

.role-matrix-editor__badge {
  font-size: 0.66rem;
  padding: 0.05rem 0.4rem;
  border-radius: 999px;
  background: rgb(var(--primary) / 0.16);
  color: rgb(var(--primary-600));
}

.role-matrix-editor__role-code {
  font-size: 0.72rem;
  color: rgb(var(--muted));
}

.role-matrix-editor__locked-note {
  margin: 0;
  font-size: 0.74rem;
  color: rgb(var(--muted));
}

.role-matrix-editor__group {
  display: grid;
  gap: 0.45rem;
}

.role-matrix-editor__group-title {
  margin: 0;
  font-size: 0.76rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: rgb(var(--muted));
}

.role-matrix-editor__perms {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(15rem, 1fr));
  gap: 0.45rem;
}

.role-matrix-editor__perm {
  display: flex;
  align-items: flex-start;
  gap: 0.5rem;
  padding: 0.45rem 0.55rem;
  border-radius: var(--radius-xs);
  border: 1px solid rgb(var(--border) / 0.7);
  background: rgb(var(--surface-2) / 0.6);
  cursor: pointer;
}

.role-matrix-editor__perm input {
  margin-top: 0.15rem;
}

.role-matrix-editor__perm-copy {
  display: grid;
  gap: 0.1rem;
  min-width: 0;
}

.role-matrix-editor__perm-copy strong {
  font-size: 0.78rem;
  font-weight: 600;
  color: rgb(var(--text));
}

.role-matrix-editor__perm-key {
  font-size: 0.68rem;
  color: rgb(var(--muted));
  word-break: break-word;
}

@media (max-width: 720px) {
  .role-matrix-editor__head {
    flex-direction: column;
  }
}
</style>
