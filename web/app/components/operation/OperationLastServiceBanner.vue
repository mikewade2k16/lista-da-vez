<script setup lang="ts">
import { buildNickname } from '~/domain/utils/person-display'
import OperationRecentServicesModal from '~/components/operation/OperationRecentServicesModal.vue'
import { useRecentServices } from '~/composables/useRecentServices'

// Faixa "Ultimo atendimento" na coluna Em atendimento (so platform_admin; o
// host decide a exibicao). refreshKey muda quando um atendimento sai da coluna,
// para reler o mais recente do banco.
const props = defineProps<{ storeId: string; refreshKey: string }>()

const { items, loading, errorMessage, fetchPage } = useRecentServices(1)
const modalOpen = ref(false)
const latest = computed(() => items.value[0] || null)

const dateTimeFormatter = new Intl.DateTimeFormat('pt-BR', {
  dateStyle: 'short',
  timeStyle: 'short',
})

const summary = computed(() => {
  const item = latest.value
  if (!item) return ''
  const when = item.finishedAt > 0 ? dateTimeFormatter.format(new Date(item.finishedAt)) : ''
  const person = buildNickname(item.personName || '') || 'Consultor'
  return [person, item.storeName, when].filter(Boolean).join(' · ')
})

let refreshTimer: ReturnType<typeof setTimeout> | null = null
watch(
  () => [props.storeId, props.refreshKey] as const,
  () => {
    if (refreshTimer) clearTimeout(refreshTimer)
    // Pequeno atraso: o encerramento persiste o historico antes do snapshot chegar,
    // mas agrupa mudancas em rajada (varios encerramentos seguidos) num so fetch.
    refreshTimer = setTimeout(() => void fetchPage(props.storeId, 1), 400)
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  if (refreshTimer) clearTimeout(refreshTimer)
})
</script>

<template>
  <div class="last-service-banner">
    <button
      type="button"
      class="last-service-banner__button"
      data-testid="operation-last-service-banner"
      :disabled="!latest && !errorMessage"
      @click="modalOpen = true"
    >
      <span class="last-service-banner__label">Ultimo atendimento</span>
      <span class="last-service-banner__value">
        <template v-if="latest">{{ summary }}</template>
        <template v-else-if="errorMessage">Erro ao carregar — ver detalhes</template>
        <template v-else-if="loading">Carregando...</template>
        <template v-else>Nenhum atendimento encerrado</template>
      </span>
    </button>

    <OperationRecentServicesModal
      v-if="modalOpen"
      :open="modalOpen"
      :store-id="storeId"
      @close="modalOpen = false"
    />
  </div>
</template>

<style scoped>
.last-service-banner {
  padding: 10px 12px 0;
}

.last-service-banner__button {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  width: 100%;
  padding: 8px 12px;
  border-radius: 10px;
  border: 1px solid var(--line-soft);
  background: rgb(var(--surface-2) / 0.4);
  color: var(--text-main);
  font-size: 0.8rem;
  text-align: left;
  cursor: pointer;
}

.last-service-banner__button:hover:not(:disabled) {
  border-color: rgb(var(--primary));
}

.last-service-banner__button:disabled {
  cursor: default;
}

.last-service-banner__label {
  font-size: 0.68rem;
  font-weight: 700;
  text-transform: uppercase;
  color: rgb(var(--muted));
  white-space: nowrap;
}

.last-service-banner__value {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 600;
}
</style>
