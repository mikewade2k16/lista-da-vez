<script setup lang="ts">
import AppSelectField from '~/components/ui/AppSelectField.vue'
import { useCustomerIntelligenceAccess } from '~/composables/customer-intelligence/useCustomerIntelligenceAccess'

withDefaults(defineProps<{ showClientSelector?: boolean }>(), {
  showClientSelector: true,
})

const route = useRoute()
const access = useCustomerIntelligenceAccess()

const tabs = computed(() =>
  [
    {
      label: 'Clientes',
      path: '/inteligencia-clientes',
      visible: access.canViewSubjects.value || access.canViewIntelligenceProfile.value,
    },
    {
      label: 'Segmentos',
      path: '/inteligencia-clientes/segmentos',
      visible: access.canViewSegments.value,
    },
    {
      label: 'Fontes',
      path: '/inteligencia-clientes/fontes',
      visible: access.canViewSources.value,
    },
    {
      label: 'Prompts',
      path: '/inteligencia-clientes/prompts',
      visible: access.canViewPrompts.value,
    },
    {
      label: 'Atendimentos',
      path: '/inteligencia-clientes/atendimentos',
      visible: access.canViewRuns.value,
    },
    {
      label: 'Auditoria',
      path: '/inteligencia-clientes/auditoria',
      visible: access.canViewAudit.value,
    },
    {
      label: 'Portfolio',
      path: '/inteligencia-clientes/portfolio',
      visible: access.canViewPortfolio.value,
    },
    {
      label: 'Configuracoes',
      path: '/inteligencia-clientes/configuracoes',
      visible:
        access.canManageCustomerDataCapabilities.value ||
        access.canViewIntelligenceProfile.value ||
        access.canManageAgents.value,
    },
  ].filter((tab) => tab.visible),
)

function isActive(path: string): boolean {
  if (path === '/inteligencia-clientes') {
    return route.path === path || /^\/inteligencia-clientes\/[^/]+$/.test(route.path)
  }
  return route.path === path
}
</script>

<template>
  <div class="ci-nav-wrap">
    <nav class="ci-nav" aria-label="Inteligencia de clientes">
      <NuxtLink
        v-for="tab in tabs"
        :key="tab.path"
        :to="tab.path"
        class="ci-nav__item"
        :class="{ 'is-active': isActive(tab.path) }"
      >
        {{ tab.label }}
      </NuxtLink>
    </nav>

    <AppSelectField
      v-if="showClientSelector && access.isAgencyAccount.value"
      :model-value="access.selectedClientAccountId.value"
      :options="access.clientOptions.value"
      label="Cliente em contexto"
      placeholder="Selecione um cliente"
      searchable
      class="ci-nav__client"
      @update:model-value="access.selectClient"
    />
  </div>
</template>

<style scoped>
.ci-nav-wrap {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 1rem;
  flex-wrap: wrap;
}

.ci-nav {
  display: flex;
  gap: 0.35rem;
  overflow-x: auto;
  padding-bottom: 0.2rem;
}

.ci-nav__item {
  flex: 0 0 auto;
  padding: 0.55rem 0.8rem;
  border-radius: 999px;
  color: rgb(var(--muted));
  font-size: 0.78rem;
  font-weight: 700;
  text-decoration: none;
}

.ci-nav__item:hover,
.ci-nav__item.is-active {
  background: rgb(var(--primary) / 0.12);
  color: rgb(var(--primary));
}

.ci-nav__client {
  width: min(22rem, 100%);
}
</style>
