<script setup lang="ts">
import '~/components/erp/erp-workspace.css'

import ErpBancoTab from '~/components/erp/ErpBancoTab.vue'
import ErpCrmWorkspace from '~/components/erp/ErpCrmWorkspace.vue'
import ErpProductsTab from '~/components/erp/ErpProductsTab.vue'
import ErpRecordsTab from '~/components/erp/ErpRecordsTab.vue'
import ErpSyncTab from '~/components/erp/ErpSyncTab.vue'
import ErpWorkspaceHeader from '~/components/erp/ErpWorkspaceHeader.vue'
import SettingsTabs from '~/components/settings/SettingsTabs.vue'
import { useErpWorkspace } from '~/composables/useErpWorkspace'

const {
  activeBancoSection,
  activeBancoTab,
  activeRecordsBootstrapLabel,
  activeRecordsColumns,
  activeRecordsGeneralSearchPlaceholder,
  activeRecordsLastImportedFile,
  activeRecordsLastRun,
  activeRecordsSpecificSearch,
  activeRecordsTotal,
  activeTab,
  bancoTabs,
  canSync,
  castRow,
  columns,
  crmRef,
  currentProductCount,
  erpScopeHint,
  erpScopeLabel,
  erpStore,
  exportCurrentDataAsCSV,
  exportingCsv,
  exportProductsAsCSV,
  formatCurrency,
  formatDateTime,
  formatNumber,
  formatPrice,
  formatSourceFileName,
  handleBackfillSync,
  handleBootstrap,
  handlePageChange,
  handlePageSizeChange,
  handleProductsSortBy,
  handleProductsSortDir,
  handleRecordsBootstrap,
  handleRecordsPageChange,
  handleRecordsPageSizeChange,
  handleRecordsSortBy,
  handleRecordsSortDir,
  handleSyncNow,
  identifierPrefixValue,
  isERPSystemAdmin,
  lastImportedFile,
  lastRun,
  orderStats,
  overview,
  pageSizeOptions,
  productRowKey,
  productsDateFrom,
  productsDateTo,
  productsSortBy,
  productsSortDir,
  rawItemRows,
  recordsDateFrom,
  recordsDateTo,
  recordsRowKey,
  recordsSearchValue,
  recordsSortBy,
  recordsSortDir,
  recordsSpecificSearchValue,
  reloadWorkspace,
  searchValue,
  selectedSyncRun,
  selectedSyncRunId,
  syncRuns,
  tabs,
} = useErpWorkspace()
</script>

<template>
  <section class="admin-panel erp-panel" data-testid="erp-panel">
    <ErpWorkspaceHeader :scope-label="erpScopeLabel" :scope-hint="erpScopeHint" />

    <SettingsTabs :tabs="tabs" :active-tab="activeTab" @update:active-tab="activeTab = $event" />

    <ErpProductsTab
      v-if="activeTab === 'produtos'"
      :can-sync="canSync"
      :cast-row="castRow"
      :columns="columns"
      :current-product-count="currentProductCount"
      :exporting="exportingCsv"
      :format-date-time="formatDateTime"
      :format-number="formatNumber"
      :format-price="formatPrice"
      :format-source-file-name="formatSourceFileName"
      :identifier-prefix-value="identifierPrefixValue"
      :last-imported-file="lastImportedFile"
      :last-run="lastRun"
      :loading="erpStore.loadingProducts || erpStore.loadingStatus"
      :page="erpStore.page"
      :page-size="erpStore.pageSize"
      :page-size-options="pageSizeOptions"
      :product-row-key="productRowKey"
      :products-date-from="productsDateFrom"
      :products-date-to="productsDateTo"
      :products-sort-by="productsSortBy"
      :products-sort-dir="productsSortDir"
      :raw-item-rows="rawItemRows"
      :rows="erpStore.products"
      :search-value="searchValue"
      :show-admin-cards="isERPSystemAdmin"
      :syncing="erpStore.syncing"
      :total="erpStore.totalProducts"
      @update:search-value="searchValue = $event"
      @update:identifier-prefix-value="identifierPrefixValue = $event"
      @update:date-from="productsDateFrom = $event"
      @update:date-to="productsDateTo = $event"
      @update:sort-by="handleProductsSortBy"
      @update:sort-dir="handleProductsSortDir"
      @update:page="handlePageChange"
      @update:page-size="handlePageSizeChange"
      @refresh="reloadWorkspace"
      @bootstrap="handleBootstrap"
      @export="exportProductsAsCSV"
    />

    <ErpCrmWorkspace v-else-if="activeTab === 'crm'" ref="crmRef" />

    <ErpBancoTab
      v-else-if="activeTab === 'banco' && isERPSystemAdmin"
      :active-banco-section="activeBancoSection"
      :active-banco-tab="activeBancoTab"
      :banco-tabs="bancoTabs"
      :current-product-count="currentProductCount"
      :raw-item-rows="rawItemRows"
      @update:active-banco-tab="activeBancoTab = $event"
    />

    <ErpSyncTab
      v-else-if="activeTab === 'sincronizacao' && isERPSystemAdmin"
      :can-sync="canSync"
      :last-imported-file="lastImportedFile"
      :last-run="lastRun"
      :loading-overview="erpStore.loadingOverview"
      :overview="overview"
      :selected-sync-run="selectedSyncRun"
      :selected-sync-run-id="selectedSyncRunId"
      :sync-runs="syncRuns"
      :syncing="erpStore.syncing"
      @refresh="reloadWorkspace"
      @sync="handleSyncNow"
      @backfill="handleBackfillSync"
      @select="selectedSyncRunId = $event"
    />

    <ErpRecordsTab
      v-else
      :active-records-bootstrap-label="activeRecordsBootstrapLabel"
      :active-records-columns="activeRecordsColumns"
      :active-records-general-search-placeholder="activeRecordsGeneralSearchPlaceholder"
      :active-records-last-imported-file="activeRecordsLastImportedFile"
      :active-records-last-run="activeRecordsLastRun"
      :active-records-specific-search="activeRecordsSpecificSearch"
      :active-records-total="activeRecordsTotal"
      :active-tab="activeTab"
      :can-sync="canSync"
      :exporting="exportingCsv"
      :format-currency="formatCurrency"
      :format-date-time="formatDateTime"
      :format-number="formatNumber"
      :loading="erpStore.loadingRecords || erpStore.loadingStatus"
      :order-stats="orderStats"
      :page="erpStore.recordsPage"
      :page-size="erpStore.recordsPageSize"
      :page-size-options="pageSizeOptions"
      :records-date-from="recordsDateFrom"
      :records-date-to="recordsDateTo"
      :records-row-key="recordsRowKey"
      :records-search-value="recordsSearchValue"
      :records-sort-by="recordsSortBy"
      :records-sort-dir="recordsSortDir"
      :records-specific-search-value="recordsSpecificSearchValue"
      :rows="erpStore.records"
      :show-admin-cards="isERPSystemAdmin"
      :syncing="erpStore.syncing"
      :total="erpStore.totalRecords"
      @update:records-search-value="recordsSearchValue = $event"
      @update:records-specific-search-value="recordsSpecificSearchValue = $event"
      @update:date-from="recordsDateFrom = $event"
      @update:date-to="recordsDateTo = $event"
      @update:sort-by="handleRecordsSortBy"
      @update:sort-dir="handleRecordsSortDir"
      @update:page="handleRecordsPageChange"
      @update:page-size="handleRecordsPageSizeChange"
      @refresh="reloadWorkspace"
      @bootstrap="handleRecordsBootstrap"
      @export="exportCurrentDataAsCSV"
    />
  </section>
</template>
