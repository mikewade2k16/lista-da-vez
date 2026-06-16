<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'

import CardapioCreateModal from '~/components/cardapio/CardapioCreateModal.vue'
import { useCardapioStore } from '~/stores/cardapio'
import { useTenantsStore } from '~/stores/tenants'
import { useAuthStore } from '~/stores/auth'
import { useUiStore } from '~/stores/ui'

const store = useCardapioStore()
const tenantsStore = useTenantsStore()
const auth = useAuthStore()
const ui = useUiStore()

const isAdmin = computed(() => String(auth.role || '').trim() === 'platform_admin')

const search = ref('')
const accountFilter = ref('')
const createOpen = ref(false)
const creating = ref(false)

let searchTimer: ReturnType<typeof setTimeout> | null = null

const tenantOptions = computed(() =>
  (tenantsStore.tenants || []).map((tenant) => ({ id: tenant.id, name: tenant.name })),
)

function dateLabel(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return '—'
  }
  return date.toLocaleDateString('pt-BR', { day: '2-digit', month: '2-digit', year: 'numeric' })
}

async function refresh() {
  await store.loadRestaurants({
    accountId: isAdmin.value ? accountFilter.value : '',
    q: search.value.trim(),
  })
}

function onSearchInput() {
  if (searchTimer) {
    clearTimeout(searchTimer)
  }
  searchTimer = setTimeout(() => {
    void refresh()
  }, 300)
}

watch(accountFilter, () => {
  void refresh()
})

function openCreate() {
  createOpen.value = true
}

async function onCreate(payload: { name: string; slug: string; accountId: string }) {
  creating.value = true
  const result = await store.createRestaurant({
    name: payload.name,
    slug: payload.slug,
    accountId: isAdmin.value ? payload.accountId : undefined,
  })
  creating.value = false

  if (!result.ok) {
    ui.error(result.message)
    return
  }

  createOpen.value = false
  ui.success('Cardapio criado.')
  await navigateTo(`/cardapio/${result.restaurant.id}`)
}

function openEditor(id: string) {
  void navigateTo(`/cardapio/${id}`)
}

onMounted(async () => {
  await auth.ensureSession()
  if (isAdmin.value) {
    await tenantsStore.ensureLoaded()
  }
  await refresh()
})
</script>

<template>
  <section class="cardapio-list">
    <header class="cardapio-list__header">
      <div class="cardapio-list__heading">
        <h1 class="cardapio-list__title">Cardapio Online</h1>
        <p class="cardapio-list__subtitle">
          Gerencie os restaurantes, o catalogo, os dominios e os pedidos de cada cardapio.
        </p>
      </div>
      <button type="button" class="cardapio-list__create" @click="openCreate">Novo cardapio</button>
    </header>

    <div class="cardapio-list__toolbar">
      <label class="cardapio-list__search">
        <span class="cardapio-list__search-icon" aria-hidden="true">⌕</span>
        <input
          v-model="search"
          type="search"
          class="cardapio-list__search-input"
          placeholder="Buscar por nome ou slug"
          @input="onSearchInput"
        />
      </label>

      <label v-if="isAdmin" class="cardapio-list__filter">
        <span class="cardapio-list__filter-label">Cliente</span>
        <select v-model="accountFilter" class="cardapio-list__filter-select">
          <option value="">Todos os clientes</option>
          <option v-for="tenant in tenantOptions" :key="tenant.id" :value="tenant.id">
            {{ tenant.name }}
          </option>
        </select>
      </label>
    </div>

    <p v-if="store.listError" class="cardapio-list__error">{{ store.listError }}</p>

    <div v-if="store.listPending" class="cardapio-list__state">Carregando cardapios...</div>

    <div v-else-if="store.restaurants.length === 0" class="cardapio-list__empty">
      <strong class="cardapio-list__empty-title">Nenhum cardapio ainda</strong>
      <p class="cardapio-list__empty-text">
        Crie o primeiro restaurante para configurar catalogo, dominios e comecar a receber pedidos.
      </p>
      <button type="button" class="cardapio-list__create" @click="openCreate">
        Criar primeiro cardapio
      </button>
    </div>

    <div v-else class="cardapio-list__table-wrap">
      <table class="cardapio-list__table">
        <thead>
          <tr>
            <th class="cardapio-list__th">Nome</th>
            <th class="cardapio-list__th">Slug</th>
            <th v-if="isAdmin" class="cardapio-list__th">Cliente</th>
            <th class="cardapio-list__th">Dominio</th>
            <th class="cardapio-list__th">Status</th>
            <th class="cardapio-list__th">Atualizado</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="restaurant in store.restaurants"
            :key="restaurant.id"
            class="cardapio-list__row"
            tabindex="0"
            @click="openEditor(restaurant.id)"
            @keydown.enter="openEditor(restaurant.id)"
          >
            <td class="cardapio-list__td cardapio-list__td--name">{{ restaurant.name }}</td>
            <td class="cardapio-list__td cardapio-list__td--mono">{{ restaurant.slug }}</td>
            <td v-if="isAdmin" class="cardapio-list__td">{{ restaurant.accountName || '—' }}</td>
            <td class="cardapio-list__td cardapio-list__td--mono">
              {{ restaurant.primaryDomain || '—' }}
            </td>
            <td class="cardapio-list__td">
              <span
                class="cardapio-list__badge"
                :class="
                  restaurant.isActive
                    ? 'cardapio-list__badge--active'
                    : 'cardapio-list__badge--inactive'
                "
              >
                {{ restaurant.isActive ? 'Ativo' : 'Inativo' }}
              </span>
            </td>
            <td class="cardapio-list__td">{{ dateLabel(restaurant.updatedAt) }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <CardapioCreateModal
      :open="createOpen"
      :saving="creating"
      :is-admin="isAdmin"
      :tenants="tenantOptions"
      @close="createOpen = false"
      @submit="onCreate"
    />
  </section>
</template>

<style scoped>
.cardapio-list {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
  padding: 1.5rem;
  flex: 1;
  min-height: 0;
  overflow-y: auto;
}

.cardapio-list__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  flex-wrap: wrap;
}

.cardapio-list__title {
  font-size: 1.35rem;
  font-weight: 700;
  color: var(--text-main);
}

.cardapio-list__subtitle {
  margin-top: 0.25rem;
  font-size: 0.92rem;
  color: var(--text-muted);
  max-width: 52ch;
}

.cardapio-list__create {
  border: none;
  color: rgb(var(--surface));
  background: linear-gradient(135deg, rgb(var(--primary)), rgb(var(--primary-600)));
  padding: 0.6rem 1.1rem;
  border-radius: var(--radius-sm);
  font-weight: 600;
  font-size: 0.9rem;
  cursor: pointer;
  white-space: nowrap;
}

.cardapio-list__toolbar {
  display: flex;
  gap: 1rem;
  flex-wrap: wrap;
  align-items: flex-end;
}

.cardapio-list__search {
  position: relative;
  flex: 1;
  min-width: 220px;
  display: flex;
  align-items: center;
}

.cardapio-list__search-icon {
  position: absolute;
  left: 0.75rem;
  color: var(--text-muted);
  pointer-events: none;
}

.cardapio-list__search-input {
  width: 100%;
  padding: 0.6rem 0.75rem 0.6rem 2rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2) / 0.6);
  color: var(--text-main);
  font-size: 0.92rem;
}

.cardapio-list__search-input:focus,
.cardapio-list__filter-select:focus {
  outline: none;
  border-color: rgb(var(--ring));
  box-shadow: 0 0 0 3px rgb(var(--ring) / 0.18);
}

.cardapio-list__filter {
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
}

.cardapio-list__filter-label {
  font-size: 0.78rem;
  font-weight: 600;
  color: var(--text-muted);
}

.cardapio-list__filter-select {
  padding: 0.6rem 0.75rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2) / 0.6);
  color: var(--text-main);
  font-size: 0.92rem;
  min-width: 200px;
}

.cardapio-list__error {
  color: rgb(var(--danger));
  background: rgb(var(--danger) / 0.14);
  padding: 0.6rem 0.85rem;
  border-radius: var(--radius-sm);
  font-size: 0.9rem;
}

.cardapio-list__state {
  color: var(--text-muted);
  font-size: 0.92rem;
  padding: 1rem 0;
}

.cardapio-list__empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.5rem;
  text-align: center;
  padding: 3rem 1.5rem;
  border: 1px dashed var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface) / 0.5);
}

.cardapio-list__empty-title {
  font-size: 1.05rem;
  color: var(--text-main);
}

.cardapio-list__empty-text {
  font-size: 0.9rem;
  color: var(--text-muted);
  max-width: 46ch;
}

.cardapio-list__table-wrap {
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  overflow: hidden;
}

.cardapio-list__table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.9rem;
}

.cardapio-list__th {
  text-align: left;
  padding: 0.7rem 0.9rem;
  font-size: 0.74rem;
  font-weight: 700;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  color: var(--text-muted);
  background: rgb(var(--surface-2) / 0.6);
  border-bottom: 1px solid var(--line-soft);
}

.cardapio-list__row {
  cursor: pointer;
  transition: background 0.12s ease;
}

.cardapio-list__row:hover,
.cardapio-list__row:focus-visible {
  background: rgb(var(--primary) / 0.08);
  outline: none;
}

.cardapio-list__td {
  padding: 0.75rem 0.9rem;
  color: var(--text-main);
  border-bottom: 1px solid var(--line-soft);
}

.cardapio-list__row:last-child .cardapio-list__td {
  border-bottom: none;
}

.cardapio-list__td--name {
  font-weight: 600;
}

.cardapio-list__td--mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.84rem;
  color: var(--text-muted);
}

.cardapio-list__badge {
  display: inline-flex;
  align-items: center;
  padding: 0.2rem 0.6rem;
  border-radius: 999px;
  font-size: 0.76rem;
  font-weight: 600;
}

.cardapio-list__badge--active {
  background: rgb(var(--success) / 0.16);
  color: rgb(var(--success));
}

.cardapio-list__badge--inactive {
  background: rgb(var(--muted) / 0.16);
  color: var(--text-muted);
}
</style>
