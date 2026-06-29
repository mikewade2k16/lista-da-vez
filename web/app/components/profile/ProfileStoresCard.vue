<script setup lang="ts">
import { computed } from 'vue'
import { useAuthStore } from '~/stores/auth'

// Bloco "Suas lojas": lojas que o usuario acessa (escopadas pelo Principal no
// back via ListAccessible), com a loja ativa destacada, opcao de trocar e as
// metas cadastradas da loja ativa. Dado ja vem no contexto da sessao
// (auth.storeContext, com goals normalizados). Nenhuma chamada nova.
const auth = useAuthStore()

const stores = computed(() => auth.storeContext || [])
const activeStoreId = computed(() => String(auth.activeStoreId || '').trim())
const activeStore = computed(
  () =>
    stores.value.find((store) => String(store?.id || '').trim() === activeStoreId.value) || null,
)

const brl = new Intl.NumberFormat('pt-BR', { style: 'currency', currency: 'BRL' })

function formatBRL(value: number): string {
  return value > 0 ? brl.format(value) : '—'
}

function formatPercent(value: number): string {
  return value > 0 ? `${value}%` : '—'
}

function formatNumber(value: number): string {
  return value > 0 ? String(value) : '—'
}

const goals = computed(() => {
  const store = activeStore.value
  if (!store) {
    return []
  }
  return [
    { id: 'monthly', label: 'Meta mensal', value: formatBRL(store.monthlyGoal) },
    { id: 'ticket', label: 'Ticket medio', value: formatBRL(store.avgTicketGoal) },
    { id: 'conversion', label: 'Conversao', value: formatPercent(store.conversionGoal) },
    { id: 'pa', label: 'P.A.', value: formatNumber(store.paGoal) },
  ]
})

function selectStore(storeId: string) {
  const id = String(storeId || '').trim()
  if (!id || id === activeStoreId.value) {
    return
  }
  void auth.setActiveStore(id)
}
</script>

<template>
  <article v-if="stores.length" class="settings-card profile-stores">
    <header class="settings-card__header">
      <h3 class="settings-card__title">Suas lojas</h3>
      <p class="settings-card__text">
        Lojas que voce acessa.
        <template v-if="auth.canUseAllStores">Voce tem escopo de todas as lojas.</template>
      </p>
    </header>

    <ul class="profile-stores__list">
      <li
        v-for="store in stores"
        :key="store.id"
        class="profile-stores__item"
        :class="{ 'profile-stores__item--active': store.id === activeStoreId }"
      >
        <button
          type="button"
          class="profile-stores__select"
          :disabled="store.id === activeStoreId"
          @click="selectStore(store.id)"
        >
          <span class="profile-stores__name">{{ store.name || store.code || 'Loja' }}</span>
          <span class="profile-stores__meta">
            <template v-if="store.city">{{ store.city }}</template>
            <template v-if="store.code">· {{ store.code }}</template>
          </span>
        </button>
        <span v-if="store.id === activeStoreId" class="profile-stores__active-tag">Ativa</span>
      </li>
    </ul>

    <div v-if="activeStore" class="profile-stores__goals">
      <span class="profile-stores__goals-label">Metas da loja ativa</span>
      <dl class="profile-stores__goals-grid">
        <div v-for="goal in goals" :key="goal.id" class="profile-stores__goal">
          <dt>{{ goal.label }}</dt>
          <dd>{{ goal.value }}</dd>
        </div>
      </dl>
    </div>
  </article>
</template>

<style scoped>
.profile-stores__list {
  display: grid;
  gap: 0.45rem;
  margin: 0;
  padding: 0;
  list-style: none;
}

.profile-stores__item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.6rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-soft);
  background: var(--bg-muted);
}

.profile-stores__item--active {
  border-color: var(--accent-info);
}

.profile-stores__select {
  flex: 1;
  display: grid;
  gap: 0.15rem;
  padding: 0.6rem 0.8rem;
  border: 0;
  background: transparent;
  text-align: left;
  cursor: pointer;
  color: var(--text-main);
}

.profile-stores__select:disabled {
  cursor: default;
}

.profile-stores__name {
  font-size: 0.88rem;
  font-weight: 600;
}

.profile-stores__meta {
  font-size: 0.76rem;
  color: var(--text-muted);
}

.profile-stores__active-tag {
  padding: 0.15rem 0.5rem;
  margin-right: 0.7rem;
  border-radius: 999px;
  background: color-mix(in srgb, var(--accent-info) 16%, transparent);
  color: var(--accent-info);
  font-size: 0.72rem;
  font-weight: 600;
}

.profile-stores__goals {
  display: grid;
  gap: 0.5rem;
  margin-top: 1rem;
  padding-top: 0.85rem;
  border-top: 1px solid var(--line-soft);
}

.profile-stores__goals-label {
  font-size: 0.72rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--text-muted);
}

.profile-stores__goals-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
  gap: 0.7rem;
  margin: 0;
}

.profile-stores__goal {
  display: grid;
  gap: 0.15rem;
}

.profile-stores__goal dt {
  font-size: 0.74rem;
  color: var(--text-muted);
}

.profile-stores__goal dd {
  margin: 0;
  font-size: 0.92rem;
  font-weight: 600;
  color: var(--text-main);
}
</style>
