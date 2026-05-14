<script setup lang="ts">
import { inject } from 'vue'
import { TASKS_PAGE_CONTEXT_KEY } from '../composables/useTasksPageContext'
import OmniDataTable from './omni/table/OmniDataTable.vue'

const ctx = inject(TASKS_PAGE_CONTEXT_KEY)!
const {
  tableSelectedRows,
  tableRows,
  tableColumns,
  viewerUserType,
  pageBootstrapping,
  tableFocusCell,
  onTableCellUpdate,
  onTableRowAction,
  createTableTask,
} = ctx
</script>

<template>
  <div class="tasks-page__table-wrap">
    <OmniDataTable v-model="tableSelectedRows" :rows="tableRows" :columns="tableColumns" row-key="id"
      :viewer-user-type="viewerUserType" :loading="pageBootstrapping" :focus-cell="tableFocusCell"
      empty-text="Nenhuma task encontrada para os filtros atuais." @update:cell="onTableCellUpdate"
      @row-action="onTableRowAction" />
    <button class="tasks-page__table-add-row" type="button" @click="createTableTask">
      <UIcon name="i-lucide-plus" class="h-4 w-4" />
      <span>Nova linha</span>
    </button>
  </div>
</template>
