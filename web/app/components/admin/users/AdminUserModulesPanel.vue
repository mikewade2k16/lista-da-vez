<script setup lang="ts">
import type {
  AccountMembershipItem,
  AdminUserItem,
  AvailablePermission,
  PermissionEffect,
  UserPermissionOverride,
} from '~/types/admin-users'

// Painel "Modulos". Espelha a UX de overrides do UsersAccessPermissionPanel legado,
// mas batendo no core (useAdminUsersManager.getOverrides/setOverrides). Cada
// permissao tem tri-estado: Herdar (sem override) / Permitir (allow) / Negar (deny).
// O estado inicial vem dos overrides ativos do usuario; Herdar = nao mandar entrada.
// Escopo = cliente OU conta-agencia (organizacao): assim platform_admin/agency_owner
// ajustam modulos tambem de usuarios "sem cliente". A conta-agencia tem todos os
// modulos habilitados (migration 0158), entao o catalogo `available` vem completo.
const props = defineProps<{ user: AdminUserItem }>()
const emit = defineEmits<{ updated: [] }>()

const m = useAdminUsersManager()

type TriState = 'inherit' | PermissionEffect

const memberships = ref<AccountMembershipItem[]>([])
const loadingMemberships = ref(false)
const loadingOverrides = ref(false)

const selectedAccountId = ref('')
const available = ref<AvailablePermission[]>([])
// Estado tri por permissionKey. 'inherit' = sem override explicito.
const states = reactive<Record<string, TriState>>({})

// Escopos editaveis: clientes E conta-agencia (organizacao). Overrides de modulo
// sao por account_id; a conta-agencia (is_agency) tambem recebe overrides, o que
// permite ajustar modulos de usuarios "sem cliente". Clientes primeiro, agencia
// depois.
const scopeMemberships = computed(() =>
  [...memberships.value].sort((a, b) => Number(a.isAgency) - Number(b.isAgency)),
)
const hasScopes = computed(() => scopeMemberships.value.length > 0)

function scopeLabel(mb: AccountMembershipItem): string {
  return mb.isAgency ? `${mb.accountName} (Organizacao)` : mb.accountName
}

// Agrupa as permissoes disponiveis por moduleId, preservando a ordem de chegada.
const groups = computed(() => {
  const map = new Map<string, AvailablePermission[]>()
  for (const perm of available.value) {
    const list = map.get(perm.moduleId) ?? []
    list.push(perm)
    map.set(perm.moduleId, list)
  }
  return [...map.entries()].map(([moduleId, permissions]) => ({ moduleId, permissions }))
})

const savingOverrides = computed(() =>
  Boolean(m.savingMap.value[`${props.user.id}:overrides:${selectedAccountId.value}`]),
)

async function loadMemberships() {
  loadingMemberships.value = true
  memberships.value = await m.fetchMemberships(props.user.id)
  loadingMemberships.value = false
  // Default: primeiro escopo (cliente ou organizacao). Sem escopos, nada a ajustar.
  selectedAccountId.value = scopeMemberships.value[0]?.accountId ?? ''
}

async function loadOverrides() {
  if (!selectedAccountId.value) {
    available.value = []
    return
  }
  loadingOverrides.value = true
  const data = await m.getOverrides(props.user.id, selectedAccountId.value)
  loadingOverrides.value = false
  if (!data) {
    available.value = []
    return
  }
  available.value = data.available
  // Inicializa o tri-estado a partir dos overrides ativos; o resto herda.
  for (const key of Object.keys(states)) delete states[key]
  for (const perm of data.available) states[perm.key] = 'inherit'
  for (const ov of data.overrides) states[ov.permissionKey] = ov.effect
}

watch(() => props.user.id, loadMemberships, { immediate: true })
watch(selectedAccountId, loadOverrides)

function setState(key: string, value: TriState) {
  states[key] = value
}

// Quantidade de overrides explicitos (allow/deny), para o resumo no botao.
const overrideCount = computed(() => Object.values(states).filter((s) => s !== 'inherit').length)

async function save() {
  if (!selectedAccountId.value) return
  const payload: UserPermissionOverride[] = []
  for (const [permissionKey, state] of Object.entries(states)) {
    if (state === 'inherit') continue
    payload.push({ permissionKey, effect: state })
  }
  const result = await m.setOverrides(props.user.id, selectedAccountId.value, payload)
  if (result) {
    available.value = result.available
    for (const key of Object.keys(states)) delete states[key]
    for (const perm of result.available) states[perm.key] = 'inherit'
    for (const ov of result.overrides) states[ov.permissionKey] = ov.effect
    emit('updated')
  }
}
</script>

<template>
  <section class="admin-user-modules">
    <UAlert
      v-if="m.errorMessage.value"
      class="admin-user-modules__error"
      color="error"
      variant="soft"
      icon="i-lucide-alert-triangle"
      :description="m.errorMessage.value"
    />

    <p v-if="loadingMemberships" class="admin-user-modules__muted">Carregando clientes...</p>

    <UAlert
      v-else-if="!hasScopes"
      color="neutral"
      variant="soft"
      icon="i-lucide-info"
      description="Vincule um cliente ou uma organizacao ao usuario (aba Vinculos) antes de ajustar modulos."
    />

    <template v-else>
      <label class="admin-user-modules__account">
        <span class="admin-user-modules__label">Cliente / Organizacao</span>
        <select v-model="selectedAccountId" class="admin-user-modules__select">
          <option v-for="mb in scopeMemberships" :key="mb.accountId" :value="mb.accountId">
            {{ scopeLabel(mb) }}
          </option>
        </select>
      </label>

      <p class="admin-user-modules__legend">
        <strong>Herdar</strong>
        usa o que os papeis do usuario ja concedem.
        <strong>Permitir</strong>
        /
        <strong>Negar</strong>
        sobrescrevem so este usuario neste cliente.
      </p>

      <p v-if="loadingOverrides" class="admin-user-modules__muted">Carregando permissoes...</p>

      <p v-else-if="groups.length === 0" class="admin-user-modules__muted">
        Nenhum modulo habilitado neste cliente para ajustar.
      </p>

      <div v-else class="admin-user-modules__groups">
        <div v-for="group in groups" :key="group.moduleId" class="admin-user-modules__group">
          <h4 class="admin-user-modules__group-title">{{ group.moduleId }}</h4>
          <ul class="admin-user-modules__perms">
            <li v-for="perm in group.permissions" :key="perm.key" class="admin-user-modules__perm">
              <div class="admin-user-modules__perm-copy">
                <span class="admin-user-modules__perm-label">{{ perm.label }}</span>
                <span class="admin-user-modules__perm-key">{{ perm.key }}</span>
              </div>
              <div class="admin-user-modules__tri" role="group" :aria-label="perm.label">
                <button
                  type="button"
                  class="admin-user-modules__tri-btn"
                  :class="{ 'is-active': states[perm.key] === 'inherit' }"
                  @click="setState(perm.key, 'inherit')"
                >
                  Herdar
                </button>
                <button
                  type="button"
                  class="admin-user-modules__tri-btn admin-user-modules__tri-btn--allow"
                  :class="{ 'is-active': states[perm.key] === 'allow' }"
                  @click="setState(perm.key, 'allow')"
                >
                  Permitir
                </button>
                <button
                  type="button"
                  class="admin-user-modules__tri-btn admin-user-modules__tri-btn--deny"
                  :class="{ 'is-active': states[perm.key] === 'deny' }"
                  @click="setState(perm.key, 'deny')"
                >
                  Negar
                </button>
              </div>
            </li>
          </ul>
        </div>
      </div>

      <div class="admin-user-modules__foot">
        <span class="admin-user-modules__count">
          {{ overrideCount }} override(s) explicito(s); o restante herda dos papeis.
        </span>
        <UButton
          label="Salvar modulos"
          color="primary"
          :loading="savingOverrides"
          :disabled="loadingOverrides || groups.length === 0"
          @click="save"
        />
      </div>
    </template>
  </section>
</template>

<style scoped>
.admin-user-modules {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.admin-user-modules__error {
  margin: 0;
}

.admin-user-modules__muted {
  margin: 0;
  font-size: 0.8rem;
  color: rgb(var(--muted));
}

.admin-user-modules__account {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  max-width: 22rem;
}

.admin-user-modules__label {
  font-size: 0.78rem;
  color: rgb(var(--muted));
}

.admin-user-modules__select {
  width: 100%;
  padding: 0.45rem 0.55rem;
  font-size: 0.85rem;
  border-radius: var(--radius-md);
  border: 1px solid rgb(var(--border));
  background: rgb(var(--surface));
  color: rgb(var(--text));
}

.admin-user-modules__legend {
  margin: 0;
  font-size: 0.76rem;
  color: rgb(var(--muted));
}

.admin-user-modules__legend strong {
  color: rgb(var(--text));
}

.admin-user-modules__groups {
  display: flex;
  flex-direction: column;
  gap: 0.9rem;
}

.admin-user-modules__group {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  padding: 0.8rem;
  border-radius: var(--radius-md);
  border: 1px solid rgb(var(--border));
  background: rgb(var(--surface));
}

.admin-user-modules__group-title {
  margin: 0;
  font-size: 0.85rem;
  font-weight: 600;
  color: rgb(var(--text));
  text-transform: capitalize;
}

.admin-user-modules__perms {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
}

.admin-user-modules__perm {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0.5rem 0.6rem;
  border-radius: var(--radius-md);
  background: rgb(var(--surface-2));
}

.admin-user-modules__perm-copy {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 0.1rem;
}

.admin-user-modules__perm-label {
  font-size: 0.83rem;
  font-weight: 500;
  color: rgb(var(--text));
}

.admin-user-modules__perm-key {
  font-size: 0.7rem;
  color: rgb(var(--muted));
}

.admin-user-modules__tri {
  display: inline-flex;
  flex-shrink: 0;
  border-radius: var(--radius-md);
  border: 1px solid rgb(var(--border));
  overflow: hidden;
}

.admin-user-modules__tri-btn {
  padding: 0.3rem 0.6rem;
  font-size: 0.74rem;
  border: none;
  background: rgb(var(--surface));
  color: rgb(var(--muted));
  cursor: pointer;
}

.admin-user-modules__tri-btn + .admin-user-modules__tri-btn {
  border-left: 1px solid rgb(var(--border));
}

.admin-user-modules__tri-btn.is-active {
  background: rgb(var(--primary));
  color: rgb(var(--surface));
}

.admin-user-modules__tri-btn--allow.is-active {
  background: rgb(var(--success));
}

.admin-user-modules__tri-btn--deny.is-active {
  background: rgb(var(--danger));
}

.admin-user-modules__foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.admin-user-modules__count {
  font-size: 0.76rem;
  color: rgb(var(--muted));
}
</style>
