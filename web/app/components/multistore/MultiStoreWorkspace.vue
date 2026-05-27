<script setup>
import { computed, onMounted, ref, watch } from 'vue'

import MultiStoreGoalsSection from '~/components/multistore/MultiStoreGoalsSection.vue'
import MultiStoreLojasSection from '~/components/multistore/MultiStoreLojasSection.vue'
import MultiStoreOrphanConsultants from '~/components/multistore/MultiStoreOrphanConsultants.vue'
import SettingsTabs from '~/components/settings/SettingsTabs.vue'
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
const activeTab = ref('lojas')

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

const workspaceTabs = computed(() => {
  const tabs = []
  if (canEditStores.value || managedStores.value.length) {
    tabs.push({ id: 'lojas', label: 'Lojas', icon: 'storefront' })
  }
  if (canViewGoalTargets.value) {
    tabs.push({ id: 'goals', label: 'Metas mensais', icon: 'flag' })
  }
  return tabs
})

watch(
  () => workspaceTabs.value,
  (tabs) => {
    if (!tabs.length) {
      activeTab.value = ''
      return
    }
    if (!tabs.some((tab) => tab.id === activeTab.value)) {
      activeTab.value = tabs[0].id
    }
  },
  { immediate: true, deep: true },
)

onMounted(() => {
  void multiStore.ensureLoaded({ includeOverview: false })
})
</script>

<template>
  <section class="admin-panel" data-testid="multistore-panel">
    <header class="admin-panel__header">
      <h2 class="admin-panel__title">Multi-loja</h2>
      <p class="admin-panel__text">
        Cadastro de lojas e metas mensais. Performance comercial fica no CRM.
      </p>
    </header>

    <article v-if="!workspaceTabs.length" class="settings-card">
      <p class="settings-card__text">Seu perfil nao possui acesso a multi-loja.</p>
    </article>

    <SettingsTabs
      v-else
      :tabs="workspaceTabs"
      :active-tab="activeTab"
      @update:active-tab="activeTab = $event"
    />

    <template v-if="activeTab === 'lojas'">
      <MultiStoreOrphanConsultants v-if="canEditStores" :managed-stores="managedStores" />
      <MultiStoreLojasSection
        :managed-stores="managedStores"
        :operation-templates="operationTemplates"
        :can-edit="canEditStores"
      />
    </template>

    <MultiStoreGoalsSection
      v-else-if="activeTab === 'goals'"
      :stores="activeManagedStores"
      :active-store-id="state.activeStoreId"
      :can-edit-goals="canEditGoalTargets"
      :allow-all-store-scope="allowAllStoreScope"
    />
  </section>
</template>
