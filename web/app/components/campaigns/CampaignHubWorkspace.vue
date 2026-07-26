<script setup>
import { computed, ref, watch } from 'vue'

import AdminPageHeader from '../../../layers/core/components/admin/AdminPageHeader.vue'
import CommunicationsWorkspace from '~/components/operation/CommunicationsWorkspace.vue'
import { getAllowedWorkspaces } from '~/domain/utils/permissions'
import { useAuthStore } from '~/stores/auth'
import CampaignWorkspace from './CampaignWorkspace.vue'

defineProps({
  state: { type: Object, required: true },
  integratedScope: { type: Boolean, default: false },
  integratedHistory: { type: Array, default: () => [] },
  integratedPending: { type: Boolean, default: false },
  integratedError: { type: String, default: '' },
  stores: { type: Array, default: () => [] },
})

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const activeSection = ref('comunicados')

const canSeeCampaigns = computed(() =>
  getAllowedWorkspaces(auth.role, auth.effectivePermissionKeys, auth.permissionsResolved).includes(
    'campanhas',
  ),
)
const sections = computed(() => [
  {
    id: 'comunicados',
    label: 'Comunicados',
    icon: 'i-lucide-megaphone',
  },
  ...(canSeeCampaigns.value
    ? [
        { id: 'campanhas', label: 'Campanhas comerciais', icon: 'i-lucide-badge-percent' },
        { id: 'corridinhas', label: 'Corridinhas e premiações', icon: 'i-lucide-trophy' },
      ]
    : []),
])

function selectSection(sectionId) {
  if (!sections.value.some((section) => section.id === sectionId)) return
  activeSection.value = sectionId
  void router.replace({ query: { ...route.query, secao: sectionId } })
}

watch(
  () => [route.query.secao, canSeeCampaigns.value],
  ([section]) => {
    const requested = String(section || 'comunicados')
    activeSection.value = sections.value.some((item) => item.id === requested)
      ? requested
      : 'comunicados'
  },
  { immediate: true },
)
</script>

<template>
  <section class="campaign-hub">
    <header class="campaign-hub__header">
      <AdminPageHeader
        eyebrow="Fila de atendimento"
        title="Campanhas"
        description="Comunicados da operação, campanhas comerciais e corridinhas com premiações em um só lugar."
      />
    </header>

    <nav class="campaign-hub__tabs" aria-label="Áreas de campanhas">
      <button
        v-for="section in sections"
        :key="section.id"
        type="button"
        :class="['campaign-hub__tab', activeSection === section.id && 'is-active']"
        :aria-current="activeSection === section.id ? 'page' : undefined"
        @click="selectSection(section.id)"
      >
        <UIcon :name="section.icon" aria-hidden="true" />
        <span>{{ section.label }}</span>
      </button>
    </nav>

    <CommunicationsWorkspace v-if="activeSection === 'comunicados'" embedded />
    <CampaignWorkspace
      v-else
      :state="state"
      :campaign-type="activeSection === 'corridinhas' ? 'interna' : 'comercial'"
      :integrated-scope="integratedScope"
      :integrated-history="integratedHistory"
      :integrated-pending="integratedPending"
      :integrated-error="integratedError"
      :stores="stores"
    />
  </section>
</template>

<style scoped>
.campaign-hub {
  display: flex;
  min-height: 0;
  flex: 1;
  flex-direction: column;
  gap: 0.65rem;
  padding: 0.7rem 1rem 1rem;
  overflow-y: auto;
}

.campaign-hub__header {
  flex: 0 0 auto;
}

.campaign-hub__tabs {
  display: flex;
  flex: 0 0 auto;
  gap: 0.35rem;
  padding: 0.3rem;
  border: 1px solid var(--line-soft);
  border-radius: 13px;
  background: rgb(var(--surface-2) / 0.32);
  overflow-x: auto;
}

.campaign-hub__tab {
  display: inline-flex;
  min-height: 2.2rem;
  align-items: center;
  gap: 0.45rem;
  padding: 0 0.78rem;
  border: 1px solid transparent;
  border-radius: 10px;
  background: transparent;
  color: rgb(var(--muted));
  font-size: 0.72rem;
  font-weight: 800;
  white-space: nowrap;
  cursor: pointer;
  transition:
    border-color 0.15s ease,
    background 0.15s ease,
    color 0.15s ease,
    transform 0.15s ease;
}

.campaign-hub__tab:hover {
  border-color: var(--line-soft);
  background: rgb(var(--surface-2) / 0.65);
  color: var(--text-main);
}

.campaign-hub__tab:active {
  transform: translateY(1px);
}

.campaign-hub__tab.is-active {
  border-color: rgb(var(--primary) / 0.55);
  background: rgb(var(--primary) / 0.13);
  color: rgb(var(--primary));
}

@media (max-width: 640px) {
  .campaign-hub {
    padding: 0.65rem;
  }
}
</style>
