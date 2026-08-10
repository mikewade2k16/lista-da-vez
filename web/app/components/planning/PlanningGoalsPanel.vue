<script setup lang="ts">
import MultiStoreGoalsSection from '~/components/multistore/MultiStoreGoalsSection.vue'

defineProps<{
  stores: unknown[]
  activeStoreId: string
  canView: boolean
  canEdit: boolean
  allowAllStoreScope: boolean
  selectedMonth: string
  selectedStoreId: string
  selectedPeriod: string
  planningAllocations: unknown[]
}>()

defineEmits<{
  'update:selected-month': [value: string]
  'update:selected-store-id': [value: string]
  'update:selected-period': [value: string]
}>()
</script>

<template>
  <header class="planning-workspace__section-head">
    <span>
      <strong>Metas comerciais</strong>
      <small>Indicadores e cadastro canônico do Multi-loja.</small>
    </span>
  </header>
  <MultiStoreGoalsSection
    v-if="canView"
    :stores="stores"
    :active-store-id="activeStoreId"
    :can-edit-goals="canEdit"
    :allow-all-store-scope="allowAllStoreScope"
    :selected-month="selectedMonth"
    :selected-store-id="selectedStoreId"
    :selected-period="selectedPeriod"
    :manage-data-lifecycle="true"
    :planning-allocations="planningAllocations"
    @update:selected-month="$emit('update:selected-month', $event)"
    @update:selected-store-id="$emit('update:selected-store-id', $event)"
    @update:selected-period="$emit('update:selected-period', $event)"
  />
  <p v-else class="planning-workspace__empty-access">
    Seu perfil não possui acesso às metas do Multi-loja.
  </p>
</template>
