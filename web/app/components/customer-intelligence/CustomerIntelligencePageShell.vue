<script setup lang="ts">
import AdminPageHeader from '../../../layers/core/components/admin/AdminPageHeader.vue'
import { useCustomerIntelligenceAccess } from '~/composables/customer-intelligence/useCustomerIntelligenceAccess'

const props = withDefaults(
  defineProps<{
    title: string
    description: string
    eyebrow?: string
    requiresClient?: boolean
    showClientSelector?: boolean
  }>(),
  {
    eyebrow: 'Customer Intelligence',
    requiresClient: true,
    showClientSelector: true,
  },
)

const access = useCustomerIntelligenceAccess()
const hasAnyModule = computed(
  () => access.hasCustomerDataModule.value || access.hasCustomerIntelligenceModule.value,
)
const canRender = computed(
  () =>
    access.contextReady.value &&
    hasAnyModule.value &&
    (!props.requiresClient || access.clientScopeReady.value),
)
</script>

<template>
  <section class="ci-page">
    <AdminPageHeader :eyebrow="eyebrow" :title="title" :description="description" />
    <CustomerIntelligenceNav :show-client-selector="showClientSelector" />

    <CustomerIntelligenceStatus
      v-if="!access.contextReady.value"
      title="Validando acesso"
      loading
    />
    <CustomerIntelligenceStatus
      v-else-if="!hasAnyModule"
      title="Modulo nao habilitado"
      :error="{
        kind: 'capability_off',
        message: '',
        reasonCode: 'module_disabled',
        statusCode: 0,
      }"
    />
    <CustomerIntelligenceStatus
      v-else-if="requiresClient && !access.clientScopeReady.value"
      title="Selecione um cliente"
      empty
      empty-text="A account de agencia exige um cliente explicito. Nao existe opcao de todos os clientes."
    />
    <slot v-else-if="canRender"></slot>
  </section>
</template>

<style scoped>
.ci-page {
  display: grid;
  gap: 1rem;
  min-width: 0;
}
</style>
