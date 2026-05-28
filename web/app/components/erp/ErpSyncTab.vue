<script setup lang="ts">
import ErpSyncOverview from '~/components/erp/ErpSyncOverview.vue'
import ErpSyncRunDetail from '~/components/erp/ErpSyncRunDetail.vue'
import ErpSyncRunsTable from '~/components/erp/ErpSyncRunsTable.vue'
import ErpSyncStatus from '~/components/erp/ErpSyncStatus.vue'
import type { ErpImportedFile, ErpRecord, ErpRun } from '~/domain/utils/erp-display'

defineProps<{
  canSync: boolean
  lastImportedFile: ErpImportedFile | null
  lastRun: ErpRun | null
  loadingOverview: boolean
  overview: ErpRecord | null
  selectedSyncRun: ErpRun | null
  selectedSyncRunId: string
  syncRuns: ErpRun[]
  syncing: boolean
}>()

const emit = defineEmits<{
  (e: 'backfill' | 'refresh' | 'sync'): void
  (e: 'select', value: string): void
}>()
</script>

<template>
  <section class="erp-sync-tab">
    <ErpSyncStatus
      :last-run="lastRun"
      :last-imported-file="lastImportedFile"
      :syncing="syncing"
      :can-sync="canSync"
      @refresh="emit('refresh')"
      @sync="emit('sync')"
      @backfill="emit('backfill')"
    />

    <ErpSyncOverview :overview="overview" :loading="loadingOverview" />

    <div class="erp-sync-tab__grid">
      <ErpSyncRunsTable
        :runs="syncRuns"
        :selected-run-id="selectedSyncRunId"
        @select="emit('select', $event)"
      />
      <ErpSyncRunDetail :run="selectedSyncRun" :last-imported-file="lastImportedFile" />
    </div>
  </section>
</template>
