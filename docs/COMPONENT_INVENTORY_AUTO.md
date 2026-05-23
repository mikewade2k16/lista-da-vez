# COMPONENT_INVENTORY_AUTO

> Arquivo gerado automaticamente por `npm run inventory`. Nao editar manualmente.
> Gerado em: 2026-05-21T13:42:44.917Z

## Escopo

- `web/app/components/`
- `web/app/features/`
- `web/layers/*/components/`

## Resumo

| Secao | Componentes | Linhas | Scoped | TipTap | Pinia | Com composables |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| web/app/components | 109 | 34809 | 50 | 1 | 31 | 15 |
| web/app/features | 0 | 0 | 0 | 0 | 0 | 0 |
| web/layers/core/components | 13 | 1830 | 5 | 0 | 2 | 6 |
| web/layers/queue/components | 0 | 0 | 0 | 0 | 0 | 0 |
| web/layers/tasks/components | 12 | 6212 | 1 | 0 | 0 | 5 |
| TOTAL | 134 | 42851 | 56 | 1 | 33 | 26 |

## Regras de deteccao

- `style scoped`: busca por bloco `<style scoped>`.
- `TipTap`: busca por imports ou referencias `@tiptap/*`, `tiptap`, `EditorContent` ou `useEditor`.
- `Pinia`: busca por import de `pinia`, `storeToRefs()` ou chamada `use*Store()`.
- `Composables externos`: lista imports cujo caminho contem `composables`.

## web/app/components

Componentes encontrados: 109

| Arquivo | Linhas | style scoped | TipTap | Pinia | Composables externos |
| --- | ---: | --- | --- | --- | --- |
| web/app/components/alerts/AlertRuleEditor.vue | 751 | yes | no | no | - |
| web/app/components/alerts/AlertRuleList.vue | 306 | yes | no | no | - |
| web/app/components/alerts/AlertsWorkspace.vue | 865 | yes | no | yes | - |
| web/app/components/banco/BancoSettingsSchema.vue | 857 | yes | no | no | - |
| web/app/components/banco/BancoWorkspace.vue | 54 | yes | no | no | - |
| web/app/components/bi/BiDatasetTable.vue | 313 | yes | no | no | - |
| web/app/components/bi/BiIntelligencePanel.vue | 285 | yes | no | no | - |
| web/app/components/bi/BiWorkspace.vue | 684 | yes | no | yes | - |
| web/app/components/campaigns/CampaignWorkspace.vue | 904 | no | no | yes | - |
| web/app/components/consultant/ConsultantIntegratedWorkspace.vue | 623 | yes | no | no | - |
| web/app/components/consultant/ConsultantMetrics.vue | 122 | no | no | no | - |
| web/app/components/consultant/ConsultantSelector.vue | 30 | no | no | no | - |
| web/app/components/consultant/ConsultantSimulator.vue | 66 | no | no | no | - |
| web/app/components/consultant/ConsultantWorkspace.vue | 119 | no | no | yes | - |
| web/app/components/crm/CrmWorkspace.vue | 604 | yes | no | yes | - |
| web/app/components/dashboard/DashboardHeader.vue | 957 | yes | no | yes | useDashboardNav |
| web/app/components/dashboard/DashboardSidebarNav.vue | 537 | yes | no | no | useDashboardNav |
| web/app/components/dashboard/DashboardUnifiedHeader.vue | 875 | yes | no | yes | - |
| web/app/components/dashboard/DashboardWorkspaceNav.vue | 210 | yes | no | yes | - |
| web/app/components/data/DataWorkspace.vue | 167 | no | no | no | - |
| web/app/components/data/InsightHourlyTable.vue | 38 | no | no | no | - |
| web/app/components/data/InsightTagList.vue | 28 | no | no | no | - |
| web/app/components/demo/DemoWorkspacePage.vue | 362 | yes | no | no | - |
| web/app/components/erp/ErpBancoTab.vue | 62 | no | no | no | - |
| web/app/components/erp/ErpCrmWorkspace.vue | 835 | yes | no | yes | - |
| web/app/components/erp/ErpDataTable.vue | 719 | yes | no | no | - |
| web/app/components/erp/ErpProductsTab.vue | 174 | no | no | no | - |
| web/app/components/erp/ErpProductsTable.vue | 542 | yes | no | no | - |
| web/app/components/erp/ErpRecordsTab.vue | 173 | no | no | no | - |
| web/app/components/erp/ErpSyncOverview.vue | 430 | yes | no | no | - |
| web/app/components/erp/ErpSyncRunDetail.vue | 142 | yes | no | no | - |
| web/app/components/erp/ErpSyncRunsTable.vue | 131 | yes | no | no | - |
| web/app/components/erp/ErpSyncStatus.vue | 204 | yes | no | no | - |
| web/app/components/erp/ErpSyncTab.vue | 50 | no | no | no | - |
| web/app/components/erp/ErpWorkspace.vue | 201 | no | no | no | useErpWorkspace |
| web/app/components/erp/ErpWorkspaceHeader.vue | 31 | no | no | no | - |
| web/app/components/feedback/FeedbackDetailPanel.vue | 163 | no | no | no | useFeedbackWorkspace |
| web/app/components/feedback/FeedbackFilters.vue | 23 | no | no | no | useFeedbackWorkspace |
| web/app/components/feedback/FeedbackFormModal.vue | 617 | yes | no | yes | - |
| web/app/components/feedback/FeedbackList.vue | 58 | no | no | no | useFeedbackWorkspace |
| web/app/components/feedback/FeedbackNotificationsDropdown.vue | 508 | yes | no | yes | - |
| web/app/components/feedback/FeedbackWorkspace.vue | 30 | no | no | no | useFeedbackWorkspace |
| web/app/components/feedback/UserFeedbackWorkspace.vue | 1015 | yes | no | yes | - |
| web/app/components/intelligence/IntelligenceDiagnosisCard.vue | 44 | no | no | no | - |
| web/app/components/intelligence/IntelligenceWorkspace.vue | 171 | no | no | no | - |
| web/app/components/layout/AdminAuthShell.vue | 48 | no | no | no | - |
| web/app/components/multistore/MultiStoreUserAccessCard.vue | 625 | no | no | yes | - |
| web/app/components/multistore/MultiStoreWorkspace.vue | 692 | no | no | yes | - |
| web/app/components/omni/OmniEditor.vue | 1013 | yes | yes | no | - |
| web/app/components/operation/AlertDisplayCenterModal.vue | 250 | yes | no | yes | - |
| web/app/components/operation/AlertDisplayCornerPopup.vue | 214 | yes | no | yes | - |
| web/app/components/operation/AlertDisplayFullscreen.vue | 200 | yes | no | yes | - |
| web/app/components/operation/AlertDisplayHost.vue | 122 | yes | no | yes | - |
| web/app/components/operation/finish/FinishStepClient.vue | 301 | no | no | no | - |
| web/app/components/operation/finish/FinishStepNotes.vue | 432 | no | no | no | - |
| web/app/components/operation/finish/FinishStepOutcome.vue | 81 | no | no | no | - |
| web/app/components/operation/finish/FinishStepProduct.vue | 365 | no | no | no | - |
| web/app/components/operation/OperationActiveServiceCard.vue | 356 | no | no | yes | - |
| web/app/components/operation/OperationAlertBanner.vue | 288 | yes | no | yes | - |
| web/app/components/operation/OperationCampaignBrief.vue | 80 | no | no | no | - |
| web/app/components/operation/OperationConsultantStrip.vue | 274 | no | no | yes | - |
| web/app/components/operation/OperationFinishModal.vue | 399 | yes | no | yes | - |
| web/app/components/operation/OperationOverviewBoard.vue | 554 | yes | no | yes | - |
| web/app/components/operation/OperationPauseReasonDialog.vue | 144 | yes | no | no | - |
| web/app/components/operation/OperationProductPicker.vue | 987 | no | no | yes | - |
| web/app/components/operation/OperationQueueColumns.vue | 716 | no | no | yes | - |
| web/app/components/operation/OperationScopeBar.vue | 246 | yes | no | no | - |
| web/app/components/operation/OperationWorkspace.vue | 329 | no | no | yes | - |
| web/app/components/ranking/RankingTable.vue | 141 | yes | no | no | - |
| web/app/components/ranking/RankingWorkspace.vue | 84 | no | no | no | - |
| web/app/components/reports/ReportsFilterToolbar.vue | 440 | no | no | no | - |
| web/app/components/reports/ReportsQualityTable.vue | 85 | no | no | no | - |
| web/app/components/reports/ReportsRecentServicesTable.vue | 54 | no | no | no | - |
| web/app/components/reports/ReportsResultsTable.vue | 79 | no | no | no | - |
| web/app/components/reports/ReportsWorkspace.vue | 745 | no | no | yes | - |
| web/app/components/roadmap/RoadmapDatabaseDiagram.vue | 751 | yes | no | no | - |
| web/app/components/roadmap/RoadmapDatabaseSchema.vue | 791 | yes | no | no | - |
| web/app/components/roadmap/RoadmapTimeline.vue | 885 | yes | no | no | - |
| web/app/components/roadmap/RoadmapWorkspace.vue | 60 | yes | no | no | - |
| web/app/components/settings/sections/SettingsAlertsSection.vue | 69 | no | no | no | - |
| web/app/components/settings/sections/SettingsModalSection.vue | 308 | no | no | no | - |
| web/app/components/settings/sections/SettingsOperationSection.vue | 124 | no | no | no | - |
| web/app/components/settings/sections/SettingsOptionTabSection.vue | 58 | no | no | no | - |
| web/app/components/settings/sections/SettingsReasonInputSection.vue | 86 | no | no | no | - |
| web/app/components/settings/sections/SettingsWorkspaceHeader.vue | 43 | yes | no | no | - |
| web/app/components/settings/SettingsConsultantManager.vue | 256 | no | no | no | - |
| web/app/components/settings/SettingsOperationTemplateManager.vue | 52 | no | no | no | - |
| web/app/components/settings/SettingsOptionManager.vue | 284 | yes | no | no | - |
| web/app/components/settings/SettingsProductManager.vue | 155 | no | no | no | - |
| web/app/components/settings/SettingsTabs.vue | 31 | no | no | no | - |
| web/app/components/settings/SettingsWorkspace.vue | 75 | no | no | no | useSettingsWorkspace |
| web/app/components/tenants/TenantsWorkspace.vue | 853 | yes | no | yes | - |
| web/app/components/ui/AppDetailDialog.vue | 288 | yes | no | no | - |
| web/app/components/ui/AppDialogHost.vue | 104 | no | no | yes | - |
| web/app/components/ui/AppEntityGrid.vue | 710 | yes | no | no | - |
| web/app/components/ui/AppPanelButton.vue | 89 | yes | no | no | - |
| web/app/components/ui/AppSelectField.vue | 354 | yes | no | no | - |
| web/app/components/ui/AppToastStack.vue | 430 | yes | no | yes | - |
| web/app/components/ui/AppToggleSwitch.vue | 122 | yes | no | no | - |
| web/app/components/users/UsersAccessCreateModal.vue | 155 | no | no | no | useUsersAccessManager |
| web/app/components/users/UsersAccessDetailDrawer.vue | 31 | no | no | no | useUsersAccessManager |
| web/app/components/users/UsersAccessDetailForm.vue | 141 | no | no | no | useUsersAccessManager |
| web/app/components/users/UsersAccessDetailSummary.vue | 75 | no | no | no | useUsersAccessManager |
| web/app/components/users/UsersAccessManager.vue | 23 | no | no | no | useUsersAccessManager |
| web/app/components/users/UsersAccessPermissionPanel.vue | 141 | no | no | no | useUsersAccessManager |
| web/app/components/users/UsersAccessRoleBadge.vue | 25 | no | no | no | - |
| web/app/components/users/UsersAccessTable.vue | 204 | no | no | no | useUsersAccessManager |
| web/app/components/users/UsersRoleMatrixManager.vue | 581 | yes | no | yes | - |
| web/app/components/users/UsersWorkspace.vue | 31 | no | no | no | - |

## web/app/features

Diretorio inexistente no momento.

## web/layers/core/components

Componentes encontrados: 13

| Arquivo | Linhas | style scoped | TipTap | Pinia | Composables externos |
| --- | ---: | --- | --- | --- | --- |
| web/layers/core/components/admin/AdminPageHeader.vue | 54 | no | no | no | useAdminPageHeaderVisibility |
| web/layers/core/components/CoreAccountSwitcher.vue | 135 | yes | no | yes | - |
| web/layers/core/components/CoreEmptyState.vue | 97 | yes | no | no | - |
| web/layers/core/components/CoreErrorState.vue | 101 | yes | no | no | - |
| web/layers/core/components/CoreLoadingOverlay.vue | 93 | yes | no | yes | - |
| web/layers/core/components/CorePermissionGate.vue | 25 | no | no | no | usePermission |
| web/layers/core/components/CoreSkeleton.vue | 213 | yes | no | no | - |
| web/layers/core/components/theme/studio/ThemeStudioDetailedGrid.vue | 120 | no | no | no | useOmniTheme, useThemeStudio |
| web/layers/core/components/theme/studio/ThemeStudioHeaderControls.vue | 121 | no | no | no | useOmniTheme |
| web/layers/core/components/theme/studio/ThemeStudioSimplePanel.vue | 476 | no | no | no | useThemeStudio |
| web/layers/core/components/theme/ThemeColorInput.vue | 89 | no | no | no | - |
| web/layers/core/components/theme/ThemeColorPicker.vue | 228 | no | no | no | useThemeColorPicker |
| web/layers/core/components/theme/ThemeGradientColorStop.vue | 78 | no | no | no | - |

## web/layers/queue/components

Diretorio inexistente no momento.

## web/layers/tasks/components

Componentes encontrados: 12

| Arquivo | Linhas | style scoped | TipTap | Pinia | Composables externos |
| --- | ---: | --- | --- | --- | --- |
| web/layers/tasks/components/admin/AdminPageHeader.vue | 48 | no | no | no | - |
| web/layers/tasks/components/AppDatePicker.vue | 1314 | yes | no | no | - |
| web/layers/tasks/components/inputs/OmniSelectInput.vue | 813 | no | no | no | - |
| web/layers/tasks/components/inputs/OmniSelectMenuInput.vue | 715 | no | no | no | - |
| web/layers/tasks/components/omni/inputs/OmniMoneyInput.vue | 144 | no | no | no | - |
| web/layers/tasks/components/omni/inputs/OmniSwitchInput.vue | 55 | no | no | no | - |
| web/layers/tasks/components/omni/table/OmniDataTable.vue | 601 | no | no | no | - |
| web/layers/tasks/components/TasksBoardView.vue | 837 | no | no | no | useTasksPageContext |
| web/layers/tasks/components/TasksFilterBar.vue | 413 | no | no | no | useTasksPageContext |
| web/layers/tasks/components/TasksProjectSettings.vue | 362 | no | no | no | useTasksPageContext |
| web/layers/tasks/components/TasksTableView.vue | 40 | no | no | no | useTasksPageContext |
| web/layers/tasks/components/TasksTaskModal.vue | 870 | no | no | no | useTasksPageContext |
