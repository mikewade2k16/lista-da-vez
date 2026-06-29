<script setup lang="ts">
import type { AvailablePermission, RoleCreateInput, RoleSummary } from '~/types/admin-users'
import AdminRoleCreateForm from '~/components/admin/users/AdminRoleCreateForm.vue'
import AdminRolePermissionMatrix from '~/components/admin/users/AdminRolePermissionMatrix.vue'

// Editor de papeis customizados (core.roles) de UMA account. Lista os papeis,
// permite selecionar um para editar label/descricao + a matriz de permissoes
// (checkbox por permissao agrupada por modulo), criar novos e deletar os
// nao-bloqueados. Emite `changed` sempre que a lista de papeis muda, para o
// painel-pai re-buscar. A fonte de verdade e sempre a resposta do backend (re-le
// apos salvar). O form de criacao (AdminRoleCreateForm) e a grade de permissoes
// (AdminRolePermissionMatrix) foram fatiados para apresentacao; este componente
// concentra o ESTADO (selecao, rascunho de edicao, persistencia).

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

// Form de criacao de novo papel (o rascunho vive no AdminRoleCreateForm; aqui so
// controlamos a abertura). Fechar desmonta o form, que reseta no proximo open.
const showCreate = ref(false)

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

// Recebe o payload validado do AdminRoleCreateForm e persiste. Fechar o form o
// desmonta (reseta o rascunho), espelhando o reset manual de campos do fluxo antigo.
async function createRole(input: RoleCreateInput) {
  const created = await r.createRole(props.accountId, input)
  if (!created) return
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

    <!-- Form de criacao de papel (apresentacao isolada) -->
    <AdminRoleCreateForm v-if="showCreate" :creating="isCreating" @submit="createRole" />

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

            <AdminRolePermissionMatrix
              :groups="groupedPermissions"
              :checked-keys="editPermissions"
              @toggle="togglePermission"
            />

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

.role-matrix-editor__detail {
  display: grid;
  gap: 0.75rem;
  padding: 0.8rem;
  border-radius: var(--radius-sm);
  border: 1px solid rgb(var(--border) / 0.8);
  background: rgb(var(--surface) / 0.7);
}

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

@media (max-width: 720px) {
  .role-matrix-editor__head {
    flex-direction: column;
  }
}
</style>
