<script setup lang="ts">
import { InputsOmniSelectMenuInput as OmniSelectMenuInput } from '#components'
// Pagina /finance — port fiel de web-reference/app/pages/admin/finance.vue.
// O layout (UDashboardGroup + sidebar redimensionavel + painel) e os estilos sao
// reproduzidos identicos; a logica vive nos composables (config + sheet). O
// slideover de configuracao esta em FinanceConfigPanel. FONTE: API Go real
// (/v1/finance/*) — mock BFF removido em AC-12.
import { useCoreAccountStore } from '../../core/stores/account'
import FinanceAccountScope from '../components/fiscal/FinanceAccountScope.vue'
import AdminPageHeader from '../../core/components/admin/AdminPageHeader.vue'
import { useFinanceConfigEditor, FINANCE_CONFIG_KEY } from '../composables/useFinanceConfigEditor'
import { useFinanceSheetEditor } from '../composables/useFinanceSheetEditor'
import { STATUS_OPTIONS, formatMoney, formatSignedMoney } from '../utils/finance-helpers'

definePageMeta({
  layout: 'dashboard',
  workspaceId: 'finance',
  pageLabel: 'Finance',
})

const route = useRoute()
const coreAccount = useCoreAccountStore()
const config = useFinanceConfigEditor()
const editor = useFinanceSheetEditor({
  configDraft: config.configDraft,
  clientRecurringEntries: config.clientRecurringEntries,
})

provide(FINANCE_CONFIG_KEY, config)

const {
  draft,
  selectedSheetId,
  detailLoading,
  creating,
  deletingId,
  errorMessage,
  filteredSheets,
  queuePersist,
  addLine,
  onCreateSheet,
  onDeleteSheet,
  selectSheet,
  entradaDisplayItems,
  onRecurringGroupEffectiveToggle,
  onRecurringGroupEffectiveDateChange,
  onRecurringStoreEffectiveToggle,
  onRecurringStoreEffectiveDateChange,
  entriesExpected,
  entriesEffective,
  exitsExpected,
  exitsEffective,
  balanceExpected,
  balanceEffective,
  canManageSheets,
  isClosed,
  onStatusChange,
  saveLabel,
  retrySave,
} = editor

const { openConfigPanel, loading: configLoading } = config
const assistantContext = useState<{ accountId: string; sheetId: string; month: string }>(
  'assistant:finance-context',
  () => ({ accountId: '', sheetId: '', month: '' }),
)
watch(
  [() => coreAccount.activeAccountId, selectedSheetId, () => draft.period],
  ([accountId, sheetId, month]) => {
    assistantContext.value = { accountId, sheetId: sheetId || '', month }
  },
  { immediate: true, flush: 'post' },
)
watch(
  () => route.query.config,
  (value) => {
    if (value === 'ia') openConfigPanel()
  },
  { immediate: true },
)

watch(
  () => coreAccount.activeAccountId,
  () => {
    void config.loadConfig()
  },
)

onMounted(async () => {
  await Promise.all([editor.fetchSheets(), config.loadConfig()])
})
async function applyCurrentConfig() {
  if (
    !window.confirm(
      'Aplicar os valores e recorrências atuais nesta planilha? Isso substitui os valores das linhas vinculadas.',
    )
  )
    return
  if (!(await retrySave())) return
  editor.syncAllFixedRows(true)
}
</script>

<template>
  <section class="finances-page space-y-4">
    <AdminPageHeader
      eyebrow="Finance"
      title="Finance v2"
      description="Planilhas mensais com entradas e saidas."
    />

    <FinanceAccountScope
      :disabled="detailLoading || configLoading || creating || editor.activeSheetSaving.value"
    />

    <UAlert
      v-if="errorMessage"
      class="finances-page__alert finances-page__alert--error"
      color="error"
      variant="soft"
      icon="i-lucide-alert-triangle"
      title="Erro"
      :description="errorMessage"
    />

    <div class="finances-page__mobile-navigation">
      <label for="finance-mobile-sheet">Planilha</label>
      <select
        id="finance-mobile-sheet"
        :value="selectedSheetId || ''"
        @change="selectSheet(($event.target as HTMLSelectElement).value)"
      >
        <option value="" disabled>Selecione uma planilha</option>
        <option v-for="sheet in filteredSheets" :key="sheet.id" :value="sheet.id">
          {{ sheet.title }} — {{ sheet.period }}
        </option>
      </select>
      <UButton
        icon="i-lucide-plus"
        aria-label="Nova planilha"
        :loading="creating"
        :disabled="!canManageSheets"
        @click="onCreateSheet"
      />
    </div>

    <UDashboardGroup class="finances-page__group !static !inset-auto !h-auto !w-full">
      <UDashboardSidebar
        class="finances-page__sidebar"
        resizable
        collapsible
        :min-size="12"
        :default-size="12"
        :max-size="20"
        :collapsed-size="12"
      >
        <template #header="{ collapsed }">
          <div class="finances-page__sidebar-header flex items-center justify-between gap-2">
            <h2
              class="finances-page__sidebar-title font-semibold text-[rgb(var(--text))]"
              :class="collapsed ? 'text-[11px]' : 'text-sm'"
            >
              Planilhas
            </h2>
            <div class="finances-page__sidebar-actions flex items-center gap-1">
              <UButton
                class="finances-page__sidebar-action finances-page__sidebar-action--create"
                icon="i-lucide-plus"
                size="sm"
                color="neutral"
                variant="soft"
                :square="collapsed"
                :loading="creating"
                :disabled="!canManageSheets"
                @click="onCreateSheet"
              />
              <UDashboardSidebarCollapse
                class="finances-page__sidebar-action finances-page__sidebar-action--collapse"
                color="neutral"
                variant="ghost"
                size="sm"
              />
            </div>
          </div>
        </template>

        <div class="finances-page__sidebar-list space-y-2">
          <button
            v-for="sheet in filteredSheets"
            :key="sheet.id"
            type="button"
            class="finances-page__sheet-card w-full rounded-[var(--radius-sm)] border p-3 text-left"
            :class="
              sheet.id === selectedSheetId
                ? 'border-primary bg-[rgb(var(--surface))]'
                : 'border-[rgb(var(--border))] bg-transparent'
            "
            @click="selectSheet(sheet.id)"
          >
            <div
              class="finances-page__sheet-card-header mb-1 flex items-start justify-between gap-2"
            >
              <p
                class="finances-page__sheet-card-title line-clamp-1 text-sm font-semibold text-[rgb(var(--text))]"
              >
                {{ sheet.title || 'Sem titulo' }}
              </p>
              <UButton
                class="finances-page__sheet-card-delete"
                icon="i-lucide-trash-2"
                color="error"
                variant="ghost"
                size="xs"
                :loading="deletingId === sheet.id"
                :disabled="!canManageSheets || sheet.status === 'fechada'"
                @click.stop="onDeleteSheet(sheet.id)"
              />
            </div>
            <p class="finances-page__sheet-card-meta text-xs text-[rgb(var(--muted))]">
              {{ sheet.period }} | {{ sheet.clientName }}
            </p>
            <p class="finances-page__sheet-card-balance text-xs text-[rgb(var(--muted))]">
              Saldo: {{ formatMoney(sheet.summary.effectiveBalance) }}
            </p>
          </button>
        </div>
      </UDashboardSidebar>

      <UDashboardPanel class="finances-page__panel">
        <section
          class="rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] p-4"
        >
          <div v-if="!selectedSheetId" class="py-14 text-center text-sm text-[rgb(var(--muted))]">
            Selecione uma planilha ou crie uma nova.
          </div>

          <div v-else-if="detailLoading" class="py-14 text-center text-sm text-[rgb(var(--muted))]">
            Carregando planilha...
          </div>

          <template v-else>
            <div
              class="finances-page__editor-header mb-4 flex flex-wrap items-start justify-between gap-3"
            >
              <input
                v-model="draft.title"
                :disabled="!canManageSheets || isClosed"
                class="finances-page__title finances-page__title-input w-full max-w-[620px]"
                placeholder="Titulo da planilha"
                @input="queuePersist()"
              />
              <div class="finances-page__editor-actions flex items-center gap-2">
                <UInput
                  v-model="draft.period"
                  :disabled="!canManageSheets || isClosed"
                  class="finances-page__period-input"
                  type="month"
                  @update:model-value="queuePersist()"
                />
                <OmniSelectMenuInput
                  :model-value="draft.status"
                  :disabled="!canManageSheets"
                  class="finances-page__status-select"
                  :items="STATUS_OPTIONS"
                  item-display-mode="text"
                  :searchable="false"
                  :creatable="false"
                  :clear="false"
                  :badge-mode="true"
                  option-edit-mode="none"
                  color="neutral"
                  variant="none"
                  :highlight="false"
                  @update:model-value="onStatusChange"
                />
                <UButton
                  class="finances-page__editor-action finances-page__editor-action--config"
                  icon="i-lucide-settings-2"
                  color="neutral"
                  variant="soft"
                  :loading="configLoading"
                  @click="openConfigPanel"
                >
                  Configuracao
                </UButton>
                <UButton
                  class="finances-page__editor-action finances-page__editor-action--delete"
                  icon="i-lucide-trash-2"
                  color="error"
                  variant="soft"
                  :loading="deletingId === selectedSheetId"
                  :disabled="!canManageSheets || isClosed"
                  @click="selectedSheetId && onDeleteSheet(selectedSheetId)"
                />
              </div>
            </div>

            <p v-if="isClosed" class="text-sm text-[rgb(var(--muted))]">
              Planilha fechada. Altere o status para Aberta antes de editar.
            </p>
            <UButton
              v-if="canManageSheets && !isClosed"
              class="finances-page__apply-config mb-3"
              size="xs"
              variant="soft"
              color="neutral"
              @click="applyCurrentConfig"
            >
              Aplicar configurações atuais nesta planilha
            </UButton>
            <fieldset
              :disabled="!canManageSheets || isClosed"
              class="finances-page__tables grid gap-3 lg:grid-cols-2"
            >
              <UCard class="finances-page__table-card finances-page__table-card--entrada">
                <template #header>
                  <div class="finances-page__table-header flex items-center justify-between">
                    <h3 class="finances-page__table-title text-sm font-semibold">Entradas</h3>
                    <UButton
                      class="finances-page__table-add-line finances-page__table-add-line--entrada"
                      icon="i-lucide-plus"
                      size="xs"
                      label="Linha"
                      @click="addLine('entrada')"
                    />
                  </div>
                </template>
                <div class="space-y-2">
                  <template v-for="item in entradaDisplayItems" :key="item.key">
                    <FinanceRecurringGroupCard
                      v-if="item.kind === 'group'"
                      :title="item.group.title"
                      :category="item.group.category"
                      :base-amount="item.group.baseAmount"
                      :adjustment-amount="item.group.adjustmentAmount"
                      :total-amount="item.group.totalAmount"
                      :effective="item.group.effective"
                      :effective-date="item.group.effectiveDate"
                      :stores="
                        item.group.rows.map((row) => ({
                          key: row.key,
                          rowId: row.rowId,
                          name: row.name,
                          amount: row.amount,
                          effective: row.row.effective,
                          effectiveDate: row.row.effectiveDate,
                        }))
                      "
                      :format-money="formatMoney"
                      :format-signed-money="formatSignedMoney"
                      @group-effective-toggle="onRecurringGroupEffectiveToggle(item.group, $event)"
                      @group-effective-date-change="
                        onRecurringGroupEffectiveDateChange(item.group, $event)
                      "
                      @child-effective-toggle="
                        onRecurringStoreEffectiveToggle(item.group, $event.rowId, $event.next)
                      "
                      @child-effective-date-change="
                        onRecurringStoreEffectiveDateChange(item.group, $event.rowId, $event.value)
                      "
                    />

                    <FinanceEditableLine
                      v-else
                      kind="entrada"
                      :row="item.row"
                      :index="item.index"
                      :editor="editor"
                    />
                  </template>
                </div>
                <template #footer>
                  <div class="finances-page__table-footer text-xs text-[rgb(var(--muted))]">
                    Esperado: {{ formatMoney(entriesExpected) }} | Efetivado:
                    {{ formatMoney(entriesEffective) }}
                  </div>
                </template>
              </UCard>

              <UCard class="finances-page__table-card finances-page__table-card--saida">
                <template #header>
                  <div class="finances-page__table-header flex items-center justify-between">
                    <h3 class="finances-page__table-title text-sm font-semibold">Saidas</h3>
                    <UButton
                      class="finances-page__table-add-line finances-page__table-add-line--saida"
                      icon="i-lucide-plus"
                      size="xs"
                      label="Linha"
                      @click="addLine('saida')"
                    />
                  </div>
                </template>
                <div class="space-y-2">
                  <FinanceEditableLine
                    v-for="(row, index) in draft.saidas"
                    :key="row.id"
                    kind="saida"
                    :row="row"
                    :index="index"
                    :editor="editor"
                  />
                </div>
                <template #footer>
                  <div class="finances-page__table-footer text-xs text-[rgb(var(--muted))]">
                    Esperado: {{ formatMoney(exitsExpected) }} | Efetivado:
                    {{ formatMoney(exitsEffective) }}
                  </div>
                </template>
              </UCard>
            </fieldset>

            <div
              class="finances-page__balance mt-4 rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface))] p-4"
            >
              <p
                class="finances-page__balance-label text-xs uppercase tracking-wide text-[rgb(var(--muted))]"
              >
                Saldo
              </p>
              <p
                class="finances-page__balance-value text-2xl font-semibold"
                :class="
                  balanceEffective >= 0 ? 'text-[rgb(var(--success))]' : 'text-[rgb(var(--danger))]'
                "
              >
                {{ formatMoney(balanceEffective) }}
              </p>
              <p class="finances-page__balance-expected text-xs text-[rgb(var(--muted))]">
                Esperado: {{ formatMoney(balanceExpected) }}
              </p>
              <UBadge color="neutral" variant="soft" class="finances-page__balance-status mt-2">
                {{ saveLabel }}
              </UBadge>
            </div>
          </template>
        </section>
      </UDashboardPanel>
    </UDashboardGroup>

    <UButton
      v-if="saveLabel.includes('Falha')"
      color="error"
      variant="soft"
      @click="void retrySave()"
    >
      Tentar salvar novamente
    </UButton>
    <FinanceConfigPanel />
  </section>
</template>

<style scoped src="../assets/finance-page.css"></style>
