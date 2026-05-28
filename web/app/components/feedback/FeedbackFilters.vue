<script setup>
import AppSelectField from '~/components/ui/AppSelectField.vue'
import { useFeedbackWorkspaceContext } from '~/composables/useFeedbackWorkspace'

const ctx = useFeedbackWorkspaceContext()
</script>

<template>
  <div class="admin-feedback__toolbar">
    <input
      v-model="ctx.searchValue"
      class="admin-feedback__search"
      type="search"
      placeholder="Buscar por assunto, usuario, loja ou conteudo..."
    />

    <div class="admin-feedback__toolbar-filters">
      <AppSelectField v-model="ctx.selectedKindFilter" :options="ctx.kindOptions" compact />
      <AppSelectField v-model="ctx.selectedStatusFilter" :options="ctx.statusOptions" compact />
    </div>
  </div>
</template>
