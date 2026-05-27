<script setup>
import { computed } from 'vue'
import { AlertTriangle } from 'lucide-vue-next'

import { useAuthStore } from '~/stores/auth'
import { useMultiStoreStore } from '~/stores/multistore'

const props = defineProps({
  storeId: {
    type: String,
    default: '',
  },
})

const auth = useAuthStore()
const multiStore = useMultiStoreStore()

const resolvedStoreId = computed(() => {
  const explicit = String(props.storeId || '').trim()
  if (explicit) return explicit
  return String(auth.activeStoreId || '').trim()
})

const archivedStore = computed(() => {
  const id = resolvedStoreId.value
  if (!id) return null
  const allStores = multiStore.managedStores?.length
    ? multiStore.managedStores
    : auth.storeContext || []
  const found = allStores.find((store) => String(store?.id || '').trim() === id)
  if (!found) return null
  if (found.isActive === false) return found
  return null
})
</script>

<template>
  <aside v-if="archivedStore" class="archived-store-banner" role="status">
    <AlertTriangle :size="16" :stroke-width="2.2" class="archived-store-banner__icon" />
    <div class="archived-store-banner__content">
      <strong>{{ archivedStore.name }} foi arquivada.</strong>
      <span>
        Encerre os atendimentos em curso. Novos atendimentos, pausas e atribuicoes estao bloqueados
        ate a loja ser restaurada.
      </span>
    </div>
  </aside>
</template>

<style scoped>
.archived-store-banner {
  display: flex;
  gap: 0.6rem;
  align-items: flex-start;
  padding: 0.7rem 0.9rem;
  border-radius: 0.7rem;
  border: 1px solid rgb(234 179 8 / 0.45);
  background: rgb(234 179 8 / 0.12);
  color: rgb(234 179 8);
  font-size: 0.78rem;
  line-height: 1.35;
}

.archived-store-banner__icon {
  flex-shrink: 0;
  margin-top: 0.1rem;
}

.archived-store-banner__content {
  display: grid;
  gap: 0.18rem;
  color: var(--text-main);
}

.archived-store-banner__content strong {
  font-size: 0.82rem;
  font-weight: 700;
  color: rgb(234 179 8);
}

.archived-store-banner__content span {
  color: var(--text-muted);
}
</style>
