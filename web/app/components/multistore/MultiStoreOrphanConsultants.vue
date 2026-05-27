<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { UserMinus, ArrowRight } from 'lucide-vue-next'

import AppSelectField from '~/components/ui/AppSelectField.vue'
import { useAuthStore } from '~/stores/auth'
import { useUiStore } from '~/stores/ui'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'

const props = defineProps({
  managedStores: {
    type: Array,
    default: () => [],
  },
})

const runtimeConfig = useRuntimeConfig()
const auth = useAuthStore()
const ui = useUiStore()
const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

const orphans = ref([])
const loading = ref(false)
const targetStoreByConsultant = reactive({})
const busyByConsultant = reactive({})
const bulkTargetStoreId = ref('')
const bulkRunning = ref(false)
const bulkProgress = reactive({ done: 0, total: 0 })

const activeStoreOptions = computed(() =>
  (props.managedStores || [])
    .filter((store) => store.isActive !== false)
    .map((store) => ({
      value: String(store.id || '').trim(),
      label: String(store.name || '').trim(),
      description: String(store.code || '').trim(),
    })),
)

watch(
  () => auth.activeTenantId,
  () => {
    void refresh()
  },
)

onMounted(() => {
  void refresh()
})

async function refresh() {
  if (!auth.isAuthenticated || !auth.activeTenantId) {
    orphans.value = []
    return
  }

  loading.value = true
  try {
    const params = new URLSearchParams()
    params.set('tenantId', auth.activeTenantId)
    const response = await apiRequest(`/v1/consultants/orphans?${params.toString()}`)
    orphans.value = Array.isArray(response?.consultants) ? response.consultants : []
  } catch (error) {
    if (isOptionalOrphansEndpointUnavailable(error)) {
      orphans.value = []
      return
    }

    ui.error(getApiErrorMessage(error, 'Nao foi possivel carregar consultores sem loja.'))
    orphans.value = []
  } finally {
    loading.value = false
  }
}

async function reassignAll() {
  const targetStoreId = String(bulkTargetStoreId.value || '').trim()
  if (!targetStoreId) {
    ui.error('Selecione a loja de destino.')
    return
  }
  if (!orphans.value.length || bulkRunning.value) return

  const target = (props.managedStores || []).find((s) => s.id === targetStoreId)
  const { confirmed } = await ui.confirm({
    title: 'Realocar todos',
    message: `${orphans.value.length} consultor(es) serao realocados para ${target?.name || 'a loja selecionada'}. Continuar?`,
    confirmLabel: 'Realocar todos',
  })
  if (!confirmed) return

  bulkRunning.value = true
  bulkProgress.done = 0
  bulkProgress.total = orphans.value.length

  const snapshot = [...orphans.value]
  let succeeded = 0
  let failed = 0

  try {
    for (const consultant of snapshot) {
      busyByConsultant[consultant.id] = true
      try {
        await apiRequest(`/v1/consultants/${encodeURIComponent(consultant.id)}`, {
          method: 'PATCH',
          body: { storeId: targetStoreId },
        })
        succeeded++
      } catch {
        failed++
      } finally {
        busyByConsultant[consultant.id] = false
        bulkProgress.done++
      }
    }
  } finally {
    bulkRunning.value = false
  }

  bulkTargetStoreId.value = ''
  await refresh()

  if (succeeded && !failed) {
    ui.success(`${succeeded} consultor(es) realocado(s) para ${target?.name || 'a loja'}.`)
  } else if (succeeded) {
    ui.info(`${succeeded} realocado(s), ${failed} com erro.`)
  } else {
    ui.error('Nao foi possivel realocar os consultores.')
  }
}

async function reassign(consultant) {
  const targetStoreId = String(targetStoreByConsultant[consultant.id] || '').trim()
  if (!targetStoreId) {
    ui.error('Selecione uma loja de destino.')
    return
  }

  busyByConsultant[consultant.id] = true
  try {
    await apiRequest(`/v1/consultants/${encodeURIComponent(consultant.id)}`, {
      method: 'PATCH',
      body: { storeId: targetStoreId },
    })

    const target = (props.managedStores || []).find((s) => s.id === targetStoreId)
    ui.success(`${consultant.name} realocado para ${target?.name || 'loja selecionada'}.`)
    targetStoreByConsultant[consultant.id] = ''
    await refresh()
  } catch (error) {
    ui.error(getApiErrorMessage(error, 'Nao foi possivel realocar o consultor.'))
  } finally {
    busyByConsultant[consultant.id] = false
  }
}

function getErrorStatus(error) {
  const candidates = [
    error?.statusCode,
    error?.status,
    error?.response?.status,
    error?.data?.statusCode,
  ]

  for (const candidate of candidates) {
    const status = Number(candidate)
    if (Number.isFinite(status) && status > 0) {
      return status
    }
  }

  return 0
}

function isOptionalOrphansEndpointUnavailable(error) {
  return [404, 405].includes(getErrorStatus(error))
}

defineExpose({ refresh })
</script>

<template>
  <section
    v-if="orphans.length || loading"
    class="orphan-consultants"
    data-testid="orphan-consultants-section"
  >
    <header class="orphan-consultants__head">
      <UserMinus :size="14" :stroke-width="2.1" class="orphan-consultants__icon" />
      <div class="orphan-consultants__head-text">
        <strong>Consultores sem loja</strong>
        <span>
          Resultado da exclusao de lojas. Atribua cada um a uma loja ativa para reativar o cadastro
          na operacao.
        </span>
      </div>
      <span v-if="orphans.length" class="orphan-consultants__counter">{{ orphans.length }}</span>
    </header>

    <div v-if="orphans.length > 1" class="orphan-consultants__bulk">
      <span class="orphan-consultants__bulk-label">Mover todos para:</span>
      <AppSelectField
        class="orphan-consultants__bulk-select"
        :model-value="bulkTargetStoreId"
        :options="activeStoreOptions"
        placeholder="Selecione a loja"
        :show-leading-icon="false"
        compact
        :disabled="bulkRunning"
        @update:model-value="bulkTargetStoreId = $event"
      />
      <button
        class="orphan-consultants__cta"
        type="button"
        :disabled="bulkRunning || !bulkTargetStoreId"
        @click="reassignAll"
      >
        <template v-if="bulkRunning">
          Realocando {{ bulkProgress.done }}/{{ bulkProgress.total }}...
        </template>
        <template v-else>
          Mover {{ orphans.length }} consultor(es)
          <ArrowRight :size="13" :stroke-width="2.2" />
        </template>
      </button>
    </div>

    <div v-if="!loading && orphans.length" class="orphan-consultants__list">
      <article v-for="consultant in orphans" :key="consultant.id" class="orphan-consultants__row">
        <span
          class="orphan-consultants__avatar"
          :style="{ background: consultant.color || 'rgb(var(--primary) / 0.4)' }"
        >
          {{ consultant.initials || '??' }}
        </span>
        <div class="orphan-consultants__identity">
          <strong>{{ consultant.name }}</strong>
          <small>{{ consultant.roleLabel || consultant.role || 'Consultor' }}</small>
        </div>
        <AppSelectField
          class="orphan-consultants__select"
          :model-value="targetStoreByConsultant[consultant.id] || ''"
          :options="activeStoreOptions"
          placeholder="Selecione a loja de destino"
          :show-leading-icon="false"
          compact
          :disabled="busyByConsultant[consultant.id]"
          @update:model-value="targetStoreByConsultant[consultant.id] = $event"
        />
        <button
          class="orphan-consultants__cta"
          type="button"
          :disabled="busyByConsultant[consultant.id] || !targetStoreByConsultant[consultant.id]"
          @click="reassign(consultant)"
        >
          {{ busyByConsultant[consultant.id] ? 'Realocando...' : 'Realocar' }}
          <ArrowRight v-if="!busyByConsultant[consultant.id]" :size="13" :stroke-width="2.2" />
        </button>
      </article>
    </div>

    <p v-else-if="loading" class="orphan-consultants__muted">Carregando consultores...</p>
  </section>
</template>

<style scoped>
.orphan-consultants {
  display: grid;
  gap: 0.5rem;
  padding: 0.7rem 0.85rem;
  border-radius: 0.7rem;
  border: 1px dashed rgb(234 179 8 / 0.45);
  background: rgb(234 179 8 / 0.07);
}

.orphan-consultants__head {
  display: flex;
  align-items: center;
  gap: 0.55rem;
}

.orphan-consultants__icon {
  color: rgb(234 179 8);
  flex-shrink: 0;
}

.orphan-consultants__head-text {
  display: grid;
  gap: 0.1rem;
  flex: 1;
  min-width: 0;
}

.orphan-consultants__head-text strong {
  font-size: 0.82rem;
  font-weight: 700;
  color: rgb(234 179 8);
}

.orphan-consultants__head-text span {
  font-size: 0.7rem;
  color: var(--text-muted);
}

.orphan-consultants__counter {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 1.5rem;
  height: 1.6rem;
  padding: 0 0.45rem;
  border-radius: 999px;
  background: rgb(234 179 8 / 0.18);
  color: rgb(234 179 8);
  font-size: 0.7rem;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  flex-shrink: 0;
}

.orphan-consultants__bulk {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.45rem;
  padding: 0.5rem 0.6rem;
  border-radius: 0.55rem;
  background: rgb(234 179 8 / 0.08);
  border: 1px solid rgb(234 179 8 / 0.25);
}

.orphan-consultants__bulk-label {
  font-size: 0.72rem;
  font-weight: 700;
  color: rgb(234 179 8);
  letter-spacing: 0.02em;
  flex-shrink: 0;
}

.orphan-consultants__bulk-select {
  min-width: 11rem;
  flex: 0 1 14rem;
}

.orphan-consultants__list {
  display: grid;
  gap: 0.35rem;
}

.orphan-consultants__row {
  display: flex;
  align-items: center;
  gap: 0.55rem;
  padding: 0.45rem 0.6rem;
  border-radius: 0.55rem;
  background: rgb(var(--surface-2) / 0.55);
  border: 1px solid rgb(var(--ring) / 0.12);
}

.orphan-consultants__avatar {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 1.85rem;
  height: 1.85rem;
  border-radius: 50%;
  color: rgb(255 255 255);
  font-size: 0.68rem;
  font-weight: 800;
  letter-spacing: 0.02em;
  flex-shrink: 0;
}

.orphan-consultants__identity {
  display: grid;
  gap: 0;
  min-width: 0;
  flex: 1 1 12rem;
}

.orphan-consultants__identity strong {
  font-size: 0.8rem;
  font-weight: 700;
  color: var(--text-main);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.orphan-consultants__identity small {
  font-size: 0.66rem;
  color: var(--text-muted);
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.orphan-consultants__select {
  min-width: 10rem;
  flex: 0 1 14rem;
}

.orphan-consultants__cta {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  min-height: 1.95rem;
  padding: 0 0.7rem;
  border-radius: 0.5rem;
  border: 1px solid rgb(var(--primary) / 0.4);
  background: rgb(var(--primary) / 0.16);
  color: rgb(var(--primary));
  font-size: 0.72rem;
  font-weight: 700;
  cursor: pointer;
  flex-shrink: 0;
  white-space: nowrap;
}

.orphan-consultants__cta:hover:not(:disabled) {
  background: rgb(var(--primary) / 0.24);
}

.orphan-consultants__cta:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.orphan-consultants__muted {
  margin: 0;
  font-size: 0.74rem;
  color: var(--text-muted);
}

@media (max-width: 720px) {
  .orphan-consultants__row {
    flex-wrap: wrap;
  }

  .orphan-consultants__select {
    flex: 1 1 100%;
  }
}
</style>
