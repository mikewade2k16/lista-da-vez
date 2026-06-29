<script setup lang="ts">
import type {
  AccountMembershipItem,
  AdminUserItem,
  AvailablePermission,
  RoleSummary,
} from '~/types/admin-users'
import AdminRoleMatrixEditor from '~/components/admin/users/AdminRoleMatrixEditor.vue'

// Painel de papeis (core.roles) por escopo de um usuario. Para cada conta ativa
// (cliente OU conta-agencia/organizacao) renderiza um bloco colapsavel com os
// papeis disponiveis como checkboxes (marcado = atribuido). Salvar grava em lote
// via setUserRoles. Um atalho revela inline o editor de matriz
// (AdminRoleMatrixEditor) para gerenciar os papeis customizados daquele escopo.
// Inclui a conta-agencia para permitir que platform_admin/agency_owner gerenciem
// papeis de usuarios "sem cliente" (so vinculados a uma organizacao) — o backend
// escopa papeis por account_id e ja autoriza esse acesso. Fonte de verdade =
// sempre a resposta do backend (re-aplica o retorno apos salvar).

const props = defineProps<{ user: AdminUserItem }>()
const emit = defineEmits<{ updated: [] }>()

const r = useAccountRolesManager()
const m = useAdminUsersManager()

// Estado por conta-cliente (chaveado por accountId). Mantemos os papeis disponiveis,
// os ids selecionados (rascunho), o snapshot do que esta salvo (para detectar
// pendencia), o catalogo de permissoes da conta e flags de UI.
interface AccountBlock {
  accountId: string
  accountName: string
  isAgency: boolean
  roles: RoleSummary[]
  selectedIds: Set<string>
  savedIds: Set<string>
  available: AvailablePermission[]
  expanded: boolean
  showMatrix: boolean
  loading: boolean
  loadedMatrix: boolean
}

const blocks = ref<AccountBlock[]>([])
const loading = ref(true)

function setsEqual(a: Set<string>, b: Set<string>): boolean {
  if (a.size !== b.size) return false
  for (const value of a) if (!b.has(value)) return false
  return true
}

function isDirty(block: AccountBlock): boolean {
  return !setsEqual(block.selectedIds, block.savedIds)
}

// Chave de saving do composable para o "salvar papeis" daquela conta.
function isSavingRoles(accountId: string): boolean {
  return Boolean(r.savingMap.value[`${accountId}:member:${props.user.id}:roles`])
}

// Carrega os papeis disponiveis e os atribuidos de uma conta, montando o bloco.
async function buildBlock(membership: AccountMembershipItem): Promise<AccountBlock> {
  const accountId = membership.accountId
  const [roles, assigned] = await Promise.all([
    r.listRoles(accountId),
    r.getUserRoles(accountId, props.user.id),
  ])
  const assignedIds = new Set(assigned.map((role) => role.id))
  return {
    accountId,
    accountName: membership.accountName || membership.accountSlug || accountId,
    isAgency: membership.isAgency,
    roles,
    selectedIds: new Set(assignedIds),
    savedIds: new Set(assignedIds),
    available: [],
    expanded: false,
    showMatrix: false,
    loading: false,
    loadedMatrix: false,
  }
}

async function load() {
  loading.value = true
  const memberships = await m.fetchMemberships(props.user.id)
  // Inclui cliente E conta-agencia (organizacao): papeis sao escopados por
  // account_id no backend, entao a conta-agencia tambem recebe papeis. Clientes
  // primeiro, agencia depois, para a leitura ficar previsivel.
  const scopedMemberships = memberships
    .filter((item) => item.isActive)
    .sort((a, b) => Number(a.isAgency) - Number(b.isAgency))
  blocks.value = await Promise.all(scopedMemberships.map(buildBlock))
  loading.value = false
}

function toggleExpanded(block: AccountBlock) {
  block.expanded = !block.expanded
}

function isAssigned(block: AccountBlock, roleId: string): boolean {
  return block.selectedIds.has(roleId)
}

function toggleRole(block: AccountBlock, roleId: string, value: boolean) {
  const next = new Set(block.selectedIds)
  if (value) next.add(roleId)
  else next.delete(roleId)
  block.selectedIds = next
}

async function saveRoles(block: AccountBlock) {
  if (isSavingRoles(block.accountId) || !isDirty(block)) return
  const result = await r.setUserRoles(block.accountId, props.user.id, [...block.selectedIds])
  if (!result) return
  // Re-hidrata do retorno autoritativo do backend.
  const savedIds = new Set(result.map((role) => role.id))
  block.selectedIds = new Set(savedIds)
  block.savedIds = new Set(savedIds)
  emit('updated')
}

// Carrega o catalogo de permissoes (`.available`) uma vez por conta, sob demanda,
// ao revelar o editor de matriz. E por-account, nao por-usuario.
async function toggleMatrix(block: AccountBlock) {
  block.showMatrix = !block.showMatrix
  if (block.showMatrix && !block.loadedMatrix) {
    block.loading = true
    const overrides = await m.getOverrides(props.user.id, block.accountId)
    block.available = overrides?.available ?? []
    block.loadedMatrix = true
    block.loading = false
  }
}

// Quando o editor de matriz altera papeis, a lista daquele cliente pode ter mudado
// (papel criado/removido/renomeado). Re-busca os papeis preservando a selecao.
async function onMatrixChanged(block: AccountBlock) {
  const roles = await r.listRoles(block.accountId)
  block.roles = roles
  // Remove da selecao/snapshot ids que nao existem mais (papel deletado).
  const validIds = new Set(roles.map((role) => role.id))
  block.selectedIds = new Set([...block.selectedIds].filter((id) => validIds.has(id)))
  block.savedIds = new Set([...block.savedIds].filter((id) => validIds.has(id)))
}

onMounted(load)
</script>

<template>
  <section class="user-roles-panel">
    <p v-if="loading" class="user-roles-panel__loading">Carregando vinculos...</p>

    <p v-else-if="!blocks.length" class="user-roles-panel__empty">
      Este usuario nao tem nenhum escopo (cliente ou organizacao). Adicione um vinculo na aba
      Vinculos para gerenciar papeis.
    </p>

    <p v-if="r.errorMessage.value" class="user-roles-panel__error">
      {{ r.errorMessage.value }}
    </p>

    <article v-for="block in blocks" :key="block.accountId" class="user-roles-panel__block">
      <button
        class="user-roles-panel__block-toggle"
        type="button"
        :aria-expanded="block.expanded ? 'true' : 'false'"
        @click="toggleExpanded(block)"
      >
        <span class="user-roles-panel__block-name">
          {{ block.accountName }}
          <span v-if="block.isAgency" class="user-roles-panel__org-badge">Organizacao</span>
        </span>
        <span class="user-roles-panel__block-meta">
          <span class="user-roles-panel__count">
            {{ block.selectedIds.size }} de {{ block.roles.length }} papeis
          </span>
          <span v-if="isDirty(block)" class="user-roles-panel__pending">pendente</span>
          <span class="user-roles-panel__chevron">{{ block.expanded ? '−' : '+' }}</span>
        </span>
      </button>

      <div v-if="block.expanded" class="user-roles-panel__body">
        <p v-if="!block.roles.length" class="user-roles-panel__empty">
          Este escopo ainda nao tem papeis. Use "Gerenciar papeis deste escopo" para criar.
        </p>

        <ul v-else class="user-roles-panel__roles">
          <li v-for="role in block.roles" :key="role.id" class="user-roles-panel__role">
            <label class="user-roles-panel__role-label">
              <input
                type="checkbox"
                :checked="isAssigned(block, role.id)"
                @change="toggleRole(block, role.id, ($event.target as HTMLInputElement).checked)"
              />
              <span class="user-roles-panel__role-copy">
                <span class="user-roles-panel__role-name">
                  {{ role.label }}
                  <span
                    v-if="role.isLocked"
                    class="user-roles-panel__lock"
                    title="Papel de sistema"
                  >
                    cadeado
                  </span>
                </span>
                <span v-if="role.description" class="user-roles-panel__role-desc">
                  {{ role.description }}
                </span>
              </span>
            </label>
          </li>
        </ul>

        <footer class="user-roles-panel__actions">
          <button class="user-roles-panel__matrix-btn" type="button" @click="toggleMatrix(block)">
            {{ block.showMatrix ? 'Ocultar gestao de papeis' : 'Gerenciar papeis deste escopo' }}
          </button>

          <div class="user-roles-panel__save-wrap">
            <span v-if="!isDirty(block)" class="user-roles-panel__save-hint">
              Sem alteracoes pendentes
            </span>
            <button
              class="user-roles-panel__save-btn"
              type="button"
              :disabled="!isDirty(block) || isSavingRoles(block.accountId)"
              @click="saveRoles(block)"
            >
              {{ isSavingRoles(block.accountId) ? 'Salvando...' : 'Salvar papeis' }}
            </button>
          </div>
        </footer>

        <div v-if="block.showMatrix" class="user-roles-panel__matrix">
          <p v-if="block.loading" class="user-roles-panel__loading">Carregando catalogo...</p>
          <AdminRoleMatrixEditor
            v-else
            :account-id="block.accountId"
            :available-permissions="block.available"
            @changed="onMatrixChanged(block)"
          />
        </div>
      </div>
    </article>
  </section>
</template>

<style scoped>
.user-roles-panel {
  display: grid;
  gap: 0.75rem;
}

.user-roles-panel__loading,
.user-roles-panel__empty {
  margin: 0;
  font-size: 0.82rem;
  color: rgb(var(--muted));
}

.user-roles-panel__error {
  margin: 0;
  padding: 0.5rem 0.7rem;
  border-radius: var(--radius-sm);
  border: 1px solid rgb(var(--danger) / 0.3);
  background: rgb(var(--danger) / 0.1);
  color: rgb(var(--danger));
  font-size: 0.78rem;
}

.user-roles-panel__block {
  border-radius: var(--radius-md);
  border: 1px solid rgb(var(--border) / 0.85);
  background: rgb(var(--surface) / 0.7);
  overflow: hidden;
}

.user-roles-panel__block-toggle {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  width: 100%;
  padding: 0.7rem 0.85rem;
  border: 0;
  background: transparent;
  color: inherit;
  text-align: left;
  cursor: pointer;
}

.user-roles-panel__block-name {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  font-size: 0.88rem;
  font-weight: 700;
  color: rgb(var(--text));
}

.user-roles-panel__org-badge {
  font-size: 0.64rem;
  font-weight: 700;
  padding: 0.05rem 0.42rem;
  border-radius: 999px;
  background: rgb(var(--primary) / 0.14);
  color: rgb(var(--primary-600));
  text-transform: uppercase;
  letter-spacing: 0.02em;
}

.user-roles-panel__block-meta {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  flex-shrink: 0;
}

.user-roles-panel__count {
  font-size: 0.74rem;
  color: rgb(var(--muted));
}

.user-roles-panel__pending {
  font-size: 0.68rem;
  font-weight: 700;
  padding: 0.05rem 0.45rem;
  border-radius: 999px;
  background: rgb(var(--primary) / 0.16);
  color: rgb(var(--primary-600));
}

.user-roles-panel__chevron {
  font-size: 1rem;
  font-weight: 700;
  color: rgb(var(--muted));
}

.user-roles-panel__body {
  display: grid;
  gap: 0.75rem;
  padding: 0 0.85rem 0.85rem;
  border-top: 1px solid rgb(var(--border) / 0.7);
}

.user-roles-panel__roles {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(16rem, 1fr));
  gap: 0.5rem;
  margin: 0.75rem 0 0;
  padding: 0;
  list-style: none;
}

.user-roles-panel__role-label {
  display: flex;
  align-items: flex-start;
  gap: 0.55rem;
  padding: 0.55rem 0.65rem;
  border-radius: var(--radius-sm);
  border: 1px solid rgb(var(--border) / 0.75);
  background: rgb(var(--surface-2) / 0.6);
  cursor: pointer;
}

.user-roles-panel__role-label input {
  margin-top: 0.15rem;
}

.user-roles-panel__role-copy {
  display: grid;
  gap: 0.18rem;
  min-width: 0;
}

.user-roles-panel__role-name {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  font-size: 0.82rem;
  font-weight: 600;
  color: rgb(var(--text));
}

.user-roles-panel__lock {
  font-size: 0.64rem;
  padding: 0.05rem 0.38rem;
  border-radius: 999px;
  background: rgb(var(--muted) / 0.18);
  color: rgb(var(--muted));
}

.user-roles-panel__role-desc {
  font-size: 0.74rem;
  color: rgb(var(--muted));
  line-height: 1.4;
}

.user-roles-panel__actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.7rem;
  flex-wrap: wrap;
}

.user-roles-panel__save-wrap {
  display: inline-flex;
  align-items: center;
  gap: 0.6rem;
}

.user-roles-panel__save-hint {
  font-size: 0.72rem;
  color: rgb(var(--muted));
}

.user-roles-panel__matrix-btn,
.user-roles-panel__save-btn {
  min-height: 2.1rem;
  padding: 0 0.85rem;
  border-radius: 999px;
  font-size: 0.76rem;
  font-weight: 700;
  cursor: pointer;
}

.user-roles-panel__matrix-btn {
  border: 1px solid rgb(var(--border) / 0.9);
  background: rgb(var(--surface));
  color: rgb(var(--text));
}

.user-roles-panel__save-btn {
  border: 1px solid rgb(var(--primary) / 0.4);
  background: rgb(var(--primary) / 0.16);
  color: rgb(var(--primary-600));
}

.user-roles-panel__save-btn:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.user-roles-panel__matrix {
  margin-top: 0.25rem;
}

@media (max-width: 720px) {
  .user-roles-panel__actions {
    flex-direction: column;
    align-items: stretch;
  }

  .user-roles-panel__save-wrap {
    justify-content: space-between;
  }
}
</style>
