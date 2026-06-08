<script setup>
import { computed, onMounted, ref } from 'vue'
import { ChevronDown, Flag, Store } from 'lucide-vue-next'

import MultiStoreGoalsSection from '~/components/multistore/MultiStoreGoalsSection.vue'
import MultiStoreLojasSection from '~/components/multistore/MultiStoreLojasSection.vue'
import MultiStoreOrphanConsultants from '~/components/multistore/MultiStoreOrphanConsultants.vue'
import {
  canAccessMultiStore,
  canManageGoalTargets,
  canManageStores,
} from '~/domain/utils/permissions'
import { useAuthStore } from '~/stores/auth'
import { useMultiStoreStore } from '~/stores/multistore'

const props = defineProps({
  state: {
    type: Object,
    required: true,
  },
})

const multiStore = useMultiStoreStore()
const auth = useAuthStore()
const selectedGoalsMonth = ref('')
const selectedGoalsStoreId = ref('')
const selectedGoalsPeriod = ref('month')

const canEditStores = computed(() =>
  canManageStores(auth.role, auth.permissionKeys, auth.permissionsResolved),
)
const canViewGoalTargets = computed(() =>
  canAccessMultiStore(auth.role, auth.permissionKeys, auth.permissionsResolved),
)
const canEditGoalTargets = computed(() =>
  canManageGoalTargets(auth.role, auth.permissionKeys, auth.permissionsResolved),
)

const operationTemplates = computed(() => props.state.operationTemplates || [])
const managedStores = computed(() => props.state.managedStores || props.state.stores || [])
const activeManagedStores = computed(() =>
  managedStores.value.filter((store) => store.isActive !== false),
)
const allowAllStoreScope = computed(
  () => new Set(activeManagedStores.value.map((store) => store.id)).size > 1,
)

onMounted(() => {
  void multiStore.ensureLoaded({ includeOverview: false })
})
</script>

<template>
  <section class="admin-panel multistore-workspace" data-testid="multistore-panel">
    <header class="admin-panel__header">
      <h2 class="admin-panel__title">Multi-loja</h2>
      <p class="admin-panel__text">
        Cadastro de lojas e metas mensais. Performance comercial fica no CRM.
      </p>
    </header>

    <article v-if="!canEditStores && !canViewGoalTargets" class="settings-card">
      <p class="settings-card__text">Seu perfil nao possui acesso a multi-loja.</p>
    </article>

    <MultiStoreGoalsSection
      v-if="canViewGoalTargets"
      :stores="activeManagedStores"
      :active-store-id="state.activeStoreId"
      :can-edit-goals="canEditGoalTargets"
      :allow-all-store-scope="allowAllStoreScope"
      :selected-month="selectedGoalsMonth"
      :selected-store-id="selectedGoalsStoreId"
      :selected-period="selectedGoalsPeriod"
      :show-tables="false"
      :manage-data-lifecycle="true"
      @update:selected-month="selectedGoalsMonth = $event"
      @update:selected-store-id="selectedGoalsStoreId = $event"
      @update:selected-period="selectedGoalsPeriod = $event"
    />

    <details v-if="canEditStores || managedStores.length" class="multistore-workspace__collapse">
      <summary class="multistore-workspace__collapse-summary">
        <div class="multistore-workspace__collapse-copy">
          <span class="multistore-workspace__collapse-icon-wrap">
            <Store :size="16" :stroke-width="2.2" />
          </span>
          <div>
            <strong class="multistore-workspace__collapse-title">Lojas</strong>
            <span class="multistore-workspace__collapse-text">
              {{ managedStores.length }} cadastrada(s) para consulta e manutencao.
            </span>
          </div>
        </div>
        <div class="multistore-workspace__collapse-meta">
          <span class="multistore-workspace__collapse-badge">{{ managedStores.length }}</span>
          <ChevronDown class="multistore-workspace__chevron" :size="18" :stroke-width="2.2" />
        </div>
      </summary>

      <div class="multistore-workspace__collapse-body">
        <MultiStoreOrphanConsultants v-if="canEditStores" :managed-stores="managedStores" />
        <MultiStoreLojasSection
          :managed-stores="managedStores"
          :operation-templates="operationTemplates"
          :can-edit="canEditStores"
        />
      </div>
    </details>

    <details v-if="canViewGoalTargets" class="multistore-workspace__collapse">
      <summary class="multistore-workspace__collapse-summary">
        <div class="multistore-workspace__collapse-copy">
          <span class="multistore-workspace__collapse-icon-wrap">
            <Flag :size="16" :stroke-width="2.2" />
          </span>
          <div>
            <strong class="multistore-workspace__collapse-title">Metas</strong>
            <span class="multistore-workspace__collapse-text">
              Metas por loja e consultor com edicao inline e leitura do CRM.
            </span>
          </div>
        </div>
        <div class="multistore-workspace__collapse-meta">
          <span class="multistore-workspace__collapse-badge">Detalhes</span>
          <ChevronDown class="multistore-workspace__chevron" :size="18" :stroke-width="2.2" />
        </div>
      </summary>

      <div class="multistore-workspace__collapse-body">
        <MultiStoreGoalsSection
          :stores="activeManagedStores"
          :active-store-id="state.activeStoreId"
          :can-edit-goals="canEditGoalTargets"
          :allow-all-store-scope="allowAllStoreScope"
          :selected-month="selectedGoalsMonth"
          :selected-store-id="selectedGoalsStoreId"
          :selected-period="selectedGoalsPeriod"
          :show-cards="false"
          :manage-data-lifecycle="false"
          @update:selected-month="selectedGoalsMonth = $event"
          @update:selected-store-id="selectedGoalsStoreId = $event"
          @update:selected-period="selectedGoalsPeriod = $event"
        />
      </div>
    </details>
  </section>
</template>

<style scoped>
.admin-panel.multistore-workspace {
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 1rem;
  min-height: 0;
  overflow-y: auto;
  scrollbar-gutter: stable;
}

.multistore-workspace__collapse {
  display: block;
  width: 100%;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: var(--bg-muted);
  box-shadow: var(--shadow-card);
  overflow: visible;
}

.multistore-workspace__collapse-summary {
  list-style: none;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 1rem 1.1rem;
  cursor: pointer;
}

.multistore-workspace__collapse-summary::-webkit-details-marker {
  display: none;
}

.multistore-workspace__collapse-copy {
  display: flex;
  align-items: center;
  gap: 0.85rem;
  min-width: 0;
}

.multistore-workspace__collapse-icon-wrap {
  width: 2.2rem;
  height: 2.2rem;
  border-radius: 0.75rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: rgb(var(--primary));
  background: rgb(var(--primary) / 0.12);
  border: 1px solid rgb(var(--primary) / 0.2);
  flex-shrink: 0;
}

.multistore-workspace__collapse-title {
  display: block;
  color: var(--text-main);
  font-size: 0.98rem;
  font-weight: 700;
}

.multistore-workspace__collapse-text {
  display: block;
  margin-top: 0.2rem;
  color: var(--text-muted);
  font-size: 0.78rem;
  line-height: 1.45;
}

.multistore-workspace__collapse-meta {
  display: inline-flex;
  align-items: center;
  gap: 0.75rem;
  flex-shrink: 0;
}

.multistore-workspace__collapse-badge {
  min-width: 2.5rem;
  padding: 0.35rem 0.65rem;
  border-radius: 999px;
  border: 1px solid rgb(var(--primary) / 0.24);
  background: rgb(var(--primary) / 0.1);
  color: rgb(var(--primary));
  font-size: 0.72rem;
  font-weight: 700;
  text-align: center;
}

.multistore-workspace__chevron {
  color: var(--text-muted);
  transition: transform 0.2s ease;
}

.multistore-workspace__collapse[open] .multistore-workspace__chevron {
  transform: rotate(180deg);
}

.multistore-workspace__collapse-body {
  padding: 0 1rem 1rem;
  border-top: 1px solid var(--line-soft);
}

@media (max-width: 820px) {
  .multistore-workspace__collapse-summary {
    align-items: flex-start;
    flex-direction: column;
  }

  .multistore-workspace__collapse-meta {
    width: 100%;
    justify-content: space-between;
  }
}
</style>
