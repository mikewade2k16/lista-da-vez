<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'

import { useCardapioStore } from '~/stores/cardapio'
import { useTenantsStore } from '~/stores/tenants'
import { useAuthStore } from '~/stores/auth'
import { useUiStore } from '~/stores/ui'
import { getApiErrorMessage } from '~/utils/api-client'

// Seletor de CLIENTE (account) do editor, so para platform_admin. Permite mover
// o estabelecimento inteiro para outra conta a partir da pagina de edicao (a
// lista tambem faz isso via celula inline; as duas coexistem). O shape da API
// nao muda: PATCH do restaurante com { accountId: alvo } via patchRestaurantScoped.

const props = defineProps<{ restaurantId: string; accountId?: string }>()

const store = useCardapioStore()
const tenantsStore = useTenantsStore()
const auth = useAuthStore()
const ui = useUiStore()

// Sentinela "agencia": estabelecimento na propria account ativa do admin
// (accountId vazio na rota e no backend). Espelha o CardapioCreateModal.
const AGENCY_SENTINEL = 'agency'

const isAdmin = computed(() => String(auth.role || '').trim() === 'platform_admin')

// Conta atual e a que a rota carrega (?account=). Vazio = agencia (account
// ativa). E a fonte autoritativa do escopo do editor; o GET do restaurante nao
// devolve o accountId, entao nao ha de onde re-hidratar alem da rota.
const currentAccountId = computed(() => String(props.accountId || '').trim())

// Valor do select. So vira valido quando os tenants estao carregados (fonte
// autoritativa). Antes disso fica vazio e o controle mostra "Carregando...".
const selected = ref('')
const moving = ref(false)

const ready = computed(() => isAdmin.value && tenantsStore.ready)

function syncFromScope() {
  selected.value = currentAccountId.value || AGENCY_SENTINEL
}

watch(currentAccountId, syncFromScope, { immediate: true })
watch(
  () => tenantsStore.ready,
  (loaded) => {
    if (loaded) {
      syncFromScope()
    }
  },
)

function tenantName(accountId: string): string {
  if (!accountId) {
    return 'Agencia (account ativa)'
  }
  const match = (tenantsStore.tenants || []).find((tenant) => tenant.id === accountId)
  return match?.name || 'cliente selecionado'
}

async function onChange(event: Event) {
  const raw = String((event.target as HTMLSelectElement).value || '').trim()
  const target = raw === AGENCY_SENTINEL ? '' : raw
  const current = currentAccountId.value

  if (moving.value || target === current) {
    syncFromScope()
    return
  }

  const { confirmed } = (await ui.confirm({
    title: 'Trocar cliente',
    message: `Trocar o cliente move TODO o estabelecimento (cardapio, pedidos, dominios) para "${tenantName(target)}". Deseja continuar?`,
    confirmLabel: 'Mover',
  })) as { confirmed: boolean }

  if (!confirmed) {
    syncFromScope()
    return
  }

  moving.value = true
  try {
    await store.patchRestaurantScoped(props.restaurantId, current, { accountId: target })
    ui.success('Estabelecimento movido para o novo cliente.')
    // Re-navega sob a conta NOVA: o editor escopa o GET por ?account= (sem isso o
    // restaurante cairia na account ativa e daria 404 apos o move).
    const query = target ? `?account=${encodeURIComponent(target)}` : ''
    await navigateTo(`/cardapio/${props.restaurantId}${query}`)
  } catch (caught) {
    ui.error(getApiErrorMessage(caught, 'Nao foi possivel mover o estabelecimento.'))
    // Re-hidrata do escopo real (mantem o valor antigo, sem mentir o estado).
    syncFromScope()
  } finally {
    moving.value = false
  }
}

onMounted(() => {
  if (isAdmin.value) {
    void tenantsStore.ensureLoaded()
  }
})
</script>

<template>
  <label v-if="isAdmin" class="cardapio-client-select">
    <span class="cardapio-client-select__label">Cliente</span>
    <select
      class="cardapio-client-select__input"
      :value="selected"
      :disabled="!ready || moving"
      @change="onChange"
    >
      <option v-if="!ready" value="">Carregando...</option>
      <template v-else>
        <option :value="AGENCY_SENTINEL">Agencia (account ativa)</option>
        <option v-for="tenant in tenantsStore.tenants" :key="tenant.id" :value="tenant.id">
          {{ tenant.name }}
        </option>
      </template>
    </select>
    <span v-if="moving" class="cardapio-client-select__spinner" aria-hidden="true"></span>
  </label>
</template>

<style scoped>
.cardapio-client-select {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
}

.cardapio-client-select__label {
  font-size: 0.8rem;
  font-weight: 600;
  color: var(--text-muted);
}

.cardapio-client-select__input {
  padding: 0.4rem 0.6rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2) / 0.6);
  color: var(--text-main);
  font-size: 0.85rem;
  font-family: inherit;
  max-width: 220px;
}

.cardapio-client-select__input:focus {
  outline: none;
  border-color: rgb(var(--ring));
  box-shadow: 0 0 0 3px rgb(var(--ring) / 0.18);
}

.cardapio-client-select__input:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.cardapio-client-select__spinner {
  width: 0.85rem;
  height: 0.85rem;
  border-radius: 999px;
  border: 2px solid rgb(var(--primary) / 0.35);
  border-top-color: rgb(var(--primary));
  animation: cardapio-client-select-spin 0.7s linear infinite;
}

@keyframes cardapio-client-select-spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
