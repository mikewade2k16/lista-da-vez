<script setup lang="ts">
import type { AccountMembershipItem, AdminUserItem, OrgRole } from '~/types/admin-users'

// Painel "Vinculos". Gerencia os vinculos do usuario com clientes (account
// nao-agencia) e com a organizacao (agencia). Resolve o bug do usuario sem nenhum
// cliente: o picker de adicionar fica sempre disponivel. Lista e mutacoes batem no
// core via useAdminUsersManager; clientes/orgs disponiveis vem dos managers proprios.
const props = defineProps<{ user: AdminUserItem }>()
const emit = defineEmits<{ updated: [] }>()

const m = useAdminUsersManager()
const clientsManager = useClientsManager()
const orgsManager = useAdminOrganizationsManager()

// Papeis coarse aceitos pelo backend ao vincular um cliente (tenant-scoped).
const CLIENT_ROLE_OPTIONS = [
  { value: 'owner', label: 'Owner — acesso total' },
  { value: 'director', label: 'Director — acesso amplo' },
  { value: 'marketing', label: 'Marketing — campanhas/site' },
]
const CLIENT_ROLE_LABELS: Record<string, string> = {
  owner: 'Owner',
  director: 'Director',
  marketing: 'Marketing',
  '': 'Sem papel',
}
// Cargo na agencia (organization). agency_owner = ve todos os clientes da org.
const ORG_ROLE_OPTIONS: { value: OrgRole; label: string }[] = [
  { value: 'agency_member', label: 'Membro — acesso limitado' },
  { value: 'agency_owner', label: 'Dono — acesso a toda a organizacao' },
]

const memberships = ref<AccountMembershipItem[]>([])
const loading = ref(false)

// Estado dos pickers de adicionar.
const newClientId = ref('')
const newClientRole = ref('owner')
const newOrgId = ref('')
const newOrgRole = ref<OrgRole>('agency_member')
const orgWideConfirmed = ref(false)

const clientMemberships = computed(() => memberships.value.filter((mb) => !mb.isAgency))
const agencyMemberships = computed(() => memberships.value.filter((mb) => mb.isAgency))

// Clientes vinculaveis: contas reais (nao-agencia) que o usuario ainda nao tem.
const availableClients = computed(() => {
  const linked = new Set(clientMemberships.value.map((mb) => mb.accountId))
  return clientsManager.clients.value.filter((c) => !c.isAgency && !linked.has(c.id))
})
// Organizacoes vinculaveis: as que o usuario ainda nao participa.
const availableOrgs = computed(() => {
  const linked = new Set(agencyMemberships.value.map((mb) => mb.accountId))
  return orgsManager.organizations.value.filter((o) => !linked.has(o.id))
})

const canAddClient = computed(() => Boolean(newClientId.value))
const canLinkOrg = computed(() => Boolean(newOrgId.value) && orgWideConfirmed.value)

function roleLabel(role: string) {
  return CLIENT_ROLE_LABELS[role] || role || 'Sem papel'
}

function saving(suffix: string) {
  return Boolean(m.savingMap.value[`${props.user.id}:${suffix}`])
}

// As opcoes de cliente/agencia NAO mudam por usuario — carregamos a lista de
// clientes/organizacoes UMA vez (na primeira montagem) em vez de re-baixar
// /v1/admin/accounts + /v1/admin/organizations inteiros a cada troca de usuario.
// So as memberships (que sao por-usuario) recarregam no watch.
const optionsLoaded = ref(false)
async function ensureOptions() {
  if (optionsLoaded.value) return
  optionsLoaded.value = true
  await Promise.all([clientsManager.fetchClients(), orgsManager.fetchOrganizations()])
}

async function load() {
  loading.value = true
  memberships.value = await m.fetchMemberships(props.user.id)
  loading.value = false
  await ensureOptions()
}

watch(() => props.user.id, load, { immediate: true })

async function addClient() {
  if (!canAddClient.value) return
  const next = await m.addMembership(props.user.id, newClientId.value, newClientRole.value)
  if (next) {
    memberships.value = next
    newClientId.value = ''
    newClientRole.value = 'owner'
    emit('updated')
  }
}

async function removeClient(accountId: string) {
  const next = await m.removeMembership(props.user.id, accountId)
  if (next) {
    memberships.value = next
    emit('updated')
  }
}

async function linkOrg() {
  if (!canLinkOrg.value) return
  const ok = await m.linkOrganization(props.user.id, newOrgId.value, newOrgRole.value, true)
  if (ok) {
    newOrgId.value = ''
    newOrgRole.value = 'agency_member'
    orgWideConfirmed.value = false
    // O vinculo de org muda as memberships do usuario (entra a conta-agencia).
    memberships.value = await m.fetchMemberships(props.user.id)
    emit('updated')
  }
}

async function unlinkOrg(accountId: string) {
  const ok = await m.unlinkOrganization(props.user.id, accountId)
  if (ok) {
    memberships.value = await m.fetchMemberships(props.user.id)
    emit('updated')
  }
}
</script>

<template>
  <section class="admin-user-memberships">
    <UAlert
      v-if="m.errorMessage.value"
      class="admin-user-memberships__error"
      color="error"
      variant="soft"
      icon="i-lucide-alert-triangle"
      :description="m.errorMessage.value"
    />

    <p v-if="loading" class="admin-user-memberships__loading">Carregando vinculos...</p>

    <!-- Clientes (accounts nao-agencia) -->
    <div class="admin-user-memberships__block">
      <header class="admin-user-memberships__block-head">
        <h4 class="admin-user-memberships__block-title">Clientes</h4>
        <p class="admin-user-memberships__block-sub">
          Contas (clientes) que este usuario acessa. O papel define o nivel dentro do cliente.
        </p>
      </header>

      <p v-if="!loading && clientMemberships.length === 0" class="admin-user-memberships__empty">
        Nenhum cliente vinculado. Use o seletor abaixo para vincular o primeiro.
      </p>

      <ul class="admin-user-memberships__list">
        <li v-for="mb in clientMemberships" :key="mb.accountId" class="admin-user-memberships__row">
          <div class="admin-user-memberships__row-main">
            <div class="admin-user-memberships__row-top">
              <span class="admin-user-memberships__row-name">{{ mb.accountName }}</span>
              <UBadge color="neutral" variant="soft" size="xs">{{ roleLabel(mb.role) }}</UBadge>
              <UBadge v-if="!mb.isActive" color="warning" variant="soft" size="xs">inativo</UBadge>
            </div>
            <span class="admin-user-memberships__row-slug">{{ mb.accountSlug }}</span>
          </div>
          <UButton
            icon="i-lucide-x"
            color="error"
            variant="ghost"
            size="xs"
            title="Remover cliente"
            aria-label="Remover cliente"
            :loading="saving(`membership:${mb.accountId}`)"
            @click="removeClient(mb.accountId)"
          />
        </li>
      </ul>

      <div class="admin-user-memberships__add">
        <select v-model="newClientId" class="admin-user-memberships__select">
          <option value="">Selecione um cliente...</option>
          <option v-for="c in availableClients" :key="c.id" :value="c.id">{{ c.name }}</option>
        </select>
        <select v-model="newClientRole" class="admin-user-memberships__select">
          <option v-for="opt in CLIENT_ROLE_OPTIONS" :key="opt.value" :value="opt.value">
            {{ opt.label }}
          </option>
        </select>
        <UButton
          label="Vincular"
          color="primary"
          size="sm"
          :disabled="!canAddClient"
          :loading="saving(`membership:${newClientId}`)"
          @click="addClient"
        />
      </div>
      <p v-if="!canAddClient" class="admin-user-memberships__hint">
        Selecione um cliente para habilitar o vinculo.
      </p>
    </div>

    <!-- Organizacao (agencia) -->
    <div class="admin-user-memberships__block">
      <header class="admin-user-memberships__block-head">
        <h4 class="admin-user-memberships__block-title">Organizacao (agencia)</h4>
        <p class="admin-user-memberships__block-sub">
          Vincular a uma organizacao torna o usuario membro da agencia: ele passa a ver TODOS os
          clientes daquela organizacao.
        </p>
      </header>

      <p v-if="!loading && agencyMemberships.length === 0" class="admin-user-memberships__empty">
        Nao e membro de nenhuma organizacao.
      </p>

      <ul class="admin-user-memberships__list">
        <li v-for="mb in agencyMemberships" :key="mb.accountId" class="admin-user-memberships__row">
          <div class="admin-user-memberships__row-main">
            <div class="admin-user-memberships__row-top">
              <span class="admin-user-memberships__row-name">{{ mb.accountName }}</span>
              <UBadge color="primary" variant="soft" size="xs">{{ roleLabel(mb.role) }}</UBadge>
              <UBadge v-if="!mb.isActive" color="warning" variant="soft" size="xs">inativo</UBadge>
            </div>
            <span class="admin-user-memberships__row-slug">{{ mb.accountSlug }}</span>
          </div>
          <UButton
            icon="i-lucide-x"
            color="error"
            variant="ghost"
            size="xs"
            title="Desvincular organizacao"
            aria-label="Desvincular organizacao"
            :loading="saving(`organization:${mb.accountId}`)"
            @click="unlinkOrg(mb.accountId)"
          />
        </li>
      </ul>

      <div class="admin-user-memberships__add">
        <select v-model="newOrgId" class="admin-user-memberships__select">
          <option value="">Selecione uma organizacao...</option>
          <option v-for="o in availableOrgs" :key="o.id" :value="o.id">{{ o.name }}</option>
        </select>
        <select v-model="newOrgRole" class="admin-user-memberships__select">
          <option v-for="opt in ORG_ROLE_OPTIONS" :key="opt.value" :value="opt.value">
            {{ opt.label }}
          </option>
        </select>
        <UButton
          label="Vincular"
          color="primary"
          size="sm"
          :disabled="!canLinkOrg"
          :loading="saving(`organization:${newOrgId}`)"
          @click="linkOrg"
        />
      </div>

      <label class="admin-user-memberships__confirm">
        <input v-model="orgWideConfirmed" type="checkbox" />
        <span>
          Entendo que isso torna o usuario membro da agencia e ele passa a ver TODOS os clientes da
          organizacao.
        </span>
      </label>
      <p v-if="!canLinkOrg" class="admin-user-memberships__hint">
        {{
          !newOrgId
            ? 'Selecione uma organizacao para habilitar o vinculo.'
            : 'Marque a confirmacao de acesso amplo para habilitar o vinculo.'
        }}
      </p>
    </div>
  </section>
</template>

<style scoped>
.admin-user-memberships {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}

.admin-user-memberships__error,
.admin-user-memberships__loading {
  margin: 0;
}

.admin-user-memberships__loading,
.admin-user-memberships__empty {
  font-size: 0.8rem;
  color: rgb(var(--muted));
}

.admin-user-memberships__block {
  display: flex;
  flex-direction: column;
  gap: 0.7rem;
  padding: 0.9rem;
  border-radius: var(--radius-md);
  border: 1px solid rgb(var(--border));
  background: rgb(var(--surface));
}

.admin-user-memberships__block-head {
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
}

.admin-user-memberships__block-title {
  margin: 0;
  font-size: 0.92rem;
  font-weight: 600;
  color: rgb(var(--text));
}

.admin-user-memberships__block-sub {
  margin: 0;
  font-size: 0.76rem;
  color: rgb(var(--muted));
}

.admin-user-memberships__list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.admin-user-memberships__row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0.55rem 0.7rem;
  border-radius: var(--radius-md);
  border: 1px solid rgb(var(--border));
  background: rgb(var(--surface-2));
}

.admin-user-memberships__row-main {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
}

.admin-user-memberships__row-top {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  min-width: 0;
}

.admin-user-memberships__row-name {
  font-weight: 600;
  font-size: 0.85rem;
  color: rgb(var(--text));
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.admin-user-memberships__row-slug {
  font-size: 0.72rem;
  color: rgb(var(--muted));
}

.admin-user-memberships__add {
  display: grid;
  grid-template-columns: minmax(0, 1.4fr) minmax(0, 1fr) auto;
  gap: 0.5rem;
  align-items: center;
}

.admin-user-memberships__select {
  width: 100%;
  padding: 0.45rem 0.55rem;
  font-size: 0.85rem;
  border-radius: var(--radius-md);
  border: 1px solid rgb(var(--border));
  background: rgb(var(--surface));
  color: rgb(var(--text));
}

.admin-user-memberships__confirm {
  display: flex;
  align-items: flex-start;
  gap: 0.5rem;
  font-size: 0.78rem;
  color: rgb(var(--text));
  cursor: pointer;
}

.admin-user-memberships__confirm input {
  margin-top: 0.15rem;
}

.admin-user-memberships__hint {
  margin: 0;
  font-size: 0.74rem;
  color: rgb(var(--muted));
}

@media (max-width: 560px) {
  .admin-user-memberships__add {
    grid-template-columns: 1fr;
  }
}
</style>
