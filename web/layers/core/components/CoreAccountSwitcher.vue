<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useCoreAccountStore } from '../stores/account'
import { useCoreLoadingStore } from '../stores/loading'
import type { AccountSummary } from '../stores/account'

const props = withDefaults(
  defineProps<{
    canEnterPlatformView?: boolean
  }>(),
  { canEnterPlatformView: false },
)

const accountStore = useCoreAccountStore()
const loading = useCoreLoadingStore()
const open = ref(false)
const rootRef = ref<HTMLElement | null>(null)

const activeAccount = computed(() => accountStore.activeAccount)
const accounts = computed(() => accountStore.accounts)
const platformView = computed(() => accountStore.platformView)

// Label do trigger: no modo dev mostra "Plataforma (dev)"; senao o nome da conta.
const triggerLabel = computed(() =>
  platformView.value ? 'Plataforma (dev)' : (activeAccount.value?.name ?? 'Selecionar account'),
)

// Secao ORGANIZACOES: contas-agencia (isAgency === true). Ordenadas por nome.
const organizationAccounts = computed<AccountSummary[]>(() =>
  accounts.value
    .filter((a) => a.isAgency === true)
    .slice()
    .sort((a, b) => a.name.localeCompare(b.name)),
)

// Secao CLIENTES: contas nao-agencia (isAgency === false), AGRUPADAS por
// organizationName. Cada grupo tem um sub-cabecalho (nome da org) e os clientes
// ordenados por nome. Contas sem organizationName caem em '' (label generico).
interface ClientGroup {
  org: string
  label: string
  accounts: AccountSummary[]
}

const clientGroups = computed<ClientGroup[]>(() => {
  const buckets = new Map<string, AccountSummary[]>()
  for (const account of accounts.value) {
    if (account.isAgency === true) continue
    const key = account.organizationName ?? ''
    const list = buckets.get(key) ?? []
    list.push(account)
    buckets.set(key, list)
  }
  return Array.from(buckets.entries())
    .map(([org, list]) => ({
      org,
      label: org || 'Sem organizacao',
      accounts: list.slice().sort((a, b) => a.name.localeCompare(b.name)),
    }))
    .sort((a, b) => a.label.localeCompare(b.label))
})

const hasClients = computed(() => clientGroups.value.length > 0)

function isActive(id: string): boolean {
  return id === accountStore.activeAccountId
}

async function select(id: string) {
  open.value = false
  if (id !== accountStore.activeAccountId || accountStore.platformView) {
    // Fase 9B: garante overlay durante troca de account, mesmo se o fetch
    // for rapido demais para o threshold de 200ms do api-client.
    loading.push('Trocando de account...')
    try {
      await accountStore.switchAccount(id)
    } finally {
      loading.pop('Trocando de account...')
    }
  }
}

// Entra no contexto super-admin/dev (revela itens em desenvolvimento/hidden).
async function selectPlatform() {
  if (!props.canEnterPlatformView) return
  open.value = false
  if (!accountStore.platformView) {
    loading.push('Entrando no modo plataforma...')
    try {
      await accountStore.enterPlatformView()
    } finally {
      loading.pop('Entrando no modo plataforma...')
    }
  }
}

function toggle() {
  open.value = !open.value
}

// Dropdown fecha ao clicar FORA dele e ao apertar Esc (regra de UX de dropdown —
// ver AGENT_RULES). Clicar numa opcao ja fecha via select()/selectPlatform().
function handlePointerDown(event: PointerEvent) {
  if (!open.value) return
  if (rootRef.value && !rootRef.value.contains(event.target as Node)) {
    open.value = false
  }
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape' && open.value) {
    open.value = false
  }
}

onMounted(() => {
  document.addEventListener('pointerdown', handlePointerDown)
  document.addEventListener('keydown', handleKeydown)
})

onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', handlePointerDown)
  document.removeEventListener('keydown', handleKeydown)
})
</script>

<template>
  <div ref="rootRef" class="core-account-switcher" :class="{ 'is-open': open }">
    <button
      class="core-account-switcher__trigger"
      type="button"
      :aria-expanded="open ? 'true' : 'false'"
      aria-haspopup="listbox"
      @click="toggle"
    >
      <span class="core-account-switcher__name">
        {{ triggerLabel }}
      </span>
      <span class="core-account-switcher__arrow" aria-hidden="true">▾</span>
    </button>

    <div
      v-if="open && accounts.length > 1"
      class="core-account-switcher__menu"
      role="listbox"
      aria-label="Selecionar account"
    >
      <!-- Secao 1: ADMIN DA PLATAFORMA — contexto super-admin/dev (revela itens
           em desenvolvimento/hidden nao liberados nem para a conta-agencia) -->
      <div
        v-if="canEnterPlatformView"
        class="core-account-switcher__section"
        role="group"
        aria-label="Admin da plataforma"
      >
        <p class="core-account-switcher__section-title">Admin da plataforma</p>
        <ul class="core-account-switcher__list">
          <li
            class="core-account-switcher__option core-account-switcher__option--platform"
            :class="{ 'is-active': platformView }"
            role="option"
            :aria-selected="platformView ? 'true' : 'false'"
            tabindex="0"
            title="Mostra módulos e telas ainda em desenvolvimento, não liberados nem para a agência"
            @click="selectPlatform()"
            @keydown.enter="selectPlatform()"
          >
            Plataforma (dev)
          </li>
        </ul>
      </div>

      <div
        v-if="canEnterPlatformView"
        class="core-account-switcher__divider"
        role="separator"
        aria-hidden="true"
      ></div>

      <!-- Secao 2: ORGANIZACOES (contas-agencia) -->
      <div
        v-if="organizationAccounts.length"
        class="core-account-switcher__section"
        role="group"
        aria-label="Organizacoes"
      >
        <p class="core-account-switcher__section-title">Organizacoes</p>
        <ul class="core-account-switcher__list">
          <li
            v-for="account in organizationAccounts"
            :key="account.id"
            class="core-account-switcher__option"
            :class="{ 'is-active': isActive(account.id) }"
            role="option"
            :aria-selected="isActive(account.id) ? 'true' : 'false'"
            tabindex="0"
            @click="select(account.id)"
            @keydown.enter="select(account.id)"
          >
            {{ account.name }}
          </li>
        </ul>
      </div>

      <div
        v-if="organizationAccounts.length && hasClients"
        class="core-account-switcher__divider"
        role="separator"
        aria-hidden="true"
      ></div>

      <!-- Secao 3: CLIENTES (contas nao-agencia, agrupadas por organizacao) -->
      <div
        v-if="hasClients"
        class="core-account-switcher__section"
        role="group"
        aria-label="Clientes"
      >
        <p class="core-account-switcher__section-title">Clientes</p>
        <template v-for="group in clientGroups" :key="group.org">
          <p class="core-account-switcher__group-title">{{ group.label }}</p>
          <ul class="core-account-switcher__list">
            <li
              v-for="account in group.accounts"
              :key="account.id"
              class="core-account-switcher__option"
              :class="{ 'is-active': isActive(account.id) }"
              role="option"
              :aria-selected="isActive(account.id) ? 'true' : 'false'"
              tabindex="0"
              @click="select(account.id)"
              @keydown.enter="select(account.id)"
            >
              {{ account.name }}
            </li>
          </ul>
        </template>
      </div>
    </div>
  </div>
</template>

<style scoped>
.core-account-switcher {
  position: relative;
}

.core-account-switcher__trigger {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0.4rem 0.7rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2) / 0.6);
  color: var(--text-main);
  font-size: 0.82rem;
  font-weight: 600;
  cursor: pointer;
  transition:
    border-color 0.15s ease,
    background 0.15s ease;
}

.core-account-switcher__trigger:hover {
  border-color: rgb(var(--primary) / 0.32);
  background: rgb(var(--primary) / 0.09);
}

.core-account-switcher__menu {
  position: absolute;
  top: calc(100% + 0.35rem);
  left: 0;
  min-width: 14rem;
  max-height: 70vh;
  overflow-y: auto;
  margin: 0;
  padding: 0.35rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface));
  box-shadow: var(--shadow-md);
  z-index: 100;
}

.core-account-switcher__section {
  display: grid;
  gap: 0.15rem;
  padding: 0.2rem 0.15rem;
}

.core-account-switcher__section-title {
  margin: 0;
  padding: 0.25rem 0.5rem;
  color: var(--text-muted);
  font-size: 0.64rem;
  font-weight: 800;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.core-account-switcher__placeholder {
  margin: 0;
  padding: 0.3rem 0.5rem;
  color: var(--text-muted);
  font-size: 0.76rem;
  font-style: italic;
  opacity: 0.7;
}

.core-account-switcher__group-title {
  margin: 0.2rem 0 0;
  padding: 0.2rem 0.5rem;
  color: var(--text-muted);
  font-size: 0.7rem;
  font-weight: 700;
}

.core-account-switcher__divider {
  height: 1px;
  margin: 0.25rem 0.15rem;
  background: var(--line-soft);
}

.core-account-switcher__list {
  display: grid;
  gap: 0.1rem;
  margin: 0;
  padding: 0;
  list-style: none;
}

.core-account-switcher__option {
  padding: 0.5rem 0.65rem;
  border-radius: var(--radius-sm);
  color: rgb(var(--text) / 0.82);
  font-size: 0.8rem;
  cursor: pointer;
  transition:
    background 0.13s ease,
    color 0.13s ease;
}

.core-account-switcher__option:hover,
.core-account-switcher__option:focus {
  background: rgb(var(--primary) / 0.12);
  color: var(--text-main);
  outline: none;
}

.core-account-switcher__option.is-active {
  color: rgb(var(--primary));
  font-weight: 700;
}
</style>
