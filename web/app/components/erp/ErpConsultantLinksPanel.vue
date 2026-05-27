<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import type { ErpConsultantLinkEmployee } from '~/stores/erp'
import { useErpStore } from '~/stores/erp'
import { useUiStore } from '~/stores/ui'

const erpStore = useErpStore()
const ui = useUiStore()
const emit = defineEmits<{ updated: [] }>()

const draftByRow = ref<Record<string, string>>({})

const employees = computed(() => erpStore.consultantLinks?.employees || [])
const consultants = computed(() => erpStore.consultantLinks?.consultants || [])

function rowKey(row: ErpConsultantLinkEmployee) {
  return `${row.erpStoreCode || 'global'}:${row.erpEmployeeId}`
}

function statusLabel(status?: string | null) {
  switch (status) {
    case 'manual':
      return 'manual'
    case 'employee_code':
      return 'codigo'
    case 'name_exact':
      return 'nome'
    case 'ambiguous':
      return 'ambiguo'
    case 'unmatched':
      return 'sem vinculo'
    default:
      return 'pendente'
  }
}

function statusClass(status?: string | null) {
  switch (status) {
    case 'manual':
    case 'employee_code':
      return 'erp-link-panel__badge--ok'
    case 'name_exact':
      return 'erp-link-panel__badge--info'
    case 'ambiguous':
      return 'erp-link-panel__badge--warn'
    default:
      return 'erp-link-panel__badge--neutral'
  }
}

async function refreshLinks() {
  const result = await erpStore.fetchConsultantLinks()
  if (!result.ok && result.message) {
    ui.error(result.message)
  }
}

async function saveLink(row: ErpConsultantLinkEmployee) {
  const consultantId = String(draftByRow.value[rowKey(row)] || '').trim()
  if (!consultantId) {
    ui.error('Selecione um consultor da Lista de Vez.')
    return
  }

  const result = await erpStore.upsertConsultantLink({
    erpStoreCode: row.erpStoreCode || '',
    erpEmployeeId: row.erpEmployeeId,
    erpEmployeeName: row.erpEmployeeName,
    consultantId,
  })
  if (!result.ok && result.message) {
    ui.error(result.message)
    return
  }
  emit('updated')
  ui.success('Vinculo ERP atualizado.')
}

async function autoLink() {
  const result = await erpStore.autoLinkConsultants()
  if (!result.ok && result.message) {
    ui.error(result.message)
    return
  }
  emit('updated')
  ui.success('Vinculos automaticos aplicados.')
}

async function deleteLink(row: ErpConsultantLinkEmployee) {
  if (!row.linkId) return
  const result = await erpStore.deleteConsultantLink({ linkId: row.linkId })
  if (!result.ok && result.message) {
    ui.error(result.message)
    return
  }
  emit('updated')
  ui.success('Vinculo ERP removido.')
}

watch(
  employees,
  (rows) => {
    const nextDrafts: Record<string, string> = {}
    for (const row of rows) {
      const key = rowKey(row)
      nextDrafts[key] = draftByRow.value[key] || row.linkedConsultantId || ''
    }
    draftByRow.value = nextDrafts
  },
  { immediate: true },
)

onMounted(() => {
  void refreshLinks()
})
</script>

<template>
  <section class="erp-link-panel">
    <header class="erp-link-panel__header">
      <div>
        <h3 class="erp-link-panel__title">Vinculos ERP x Lista de Vez</h3>
        <p class="erp-link-panel__text">
          Funcionarios do FTP com sugestao automatica por codigo e nome, mais ajuste manual.
        </p>
      </div>
      <div class="erp-link-panel__header-actions">
        <button
          type="button"
          class="erp-link-panel__button"
          :disabled="erpStore.loadingConsultantLinks || erpStore.savingConsultantLink"
          @click="autoLink"
        >
          Vincular automaticamente
        </button>
        <button
          type="button"
          class="erp-link-panel__button erp-link-panel__button--ghost"
          :disabled="erpStore.loadingConsultantLinks || erpStore.savingConsultantLink"
          @click="refreshLinks"
        >
          Atualizar
        </button>
      </div>
    </header>

    <div v-if="erpStore.loadingConsultantLinks && !employees.length" class="erp-link-panel__empty">
      Carregando vinculos...
    </div>

    <div v-else class="erp-link-panel__table-wrap">
      <table class="erp-link-panel__table">
        <thead>
          <tr>
            <th>Funcionario ERP</th>
            <th>Loja</th>
            <th>Vinculo atual</th>
            <th>Consultor Lista</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in employees" :key="rowKey(row)">
            <td>
              <div class="erp-link-panel__identity">
                <strong>{{ row.erpEmployeeName }}</strong>
                <small>ERP {{ row.erpEmployeeId }}</small>
              </div>
            </td>
            <td>
              <div class="erp-link-panel__identity">
                <strong>{{ row.erpStoreLabel || '-' }}</strong>
                <small>{{ row.erpStoreRawCode || row.erpStoreCode || 'global' }}</small>
              </div>
            </td>
            <td>
              <span class="erp-link-panel__badge" :class="statusClass(row.linkStatus)">
                {{ statusLabel(row.linkStatus) }}
              </span>
              <small v-if="row.linkedConsultantName" class="erp-link-panel__current">
                {{ row.linkedConsultantName }}
              </small>
            </td>
            <td>
              <select v-model="draftByRow[rowKey(row)]" class="erp-link-panel__select">
                <option value="">Selecionar consultor</option>
                <option
                  v-for="consultant in consultants"
                  :key="consultant.consultantId"
                  :value="consultant.consultantId"
                >
                  {{ consultant.consultantName }} - {{ consultant.storeName }}
                </option>
              </select>
            </td>
            <td>
              <div class="erp-link-panel__actions">
                <button
                  type="button"
                  class="erp-link-panel__button"
                  :disabled="erpStore.savingConsultantLink"
                  @click="saveLink(row)"
                >
                  Salvar
                </button>
                <button
                  v-if="row.linkId"
                  type="button"
                  class="erp-link-panel__button erp-link-panel__button--ghost"
                  :disabled="erpStore.savingConsultantLink"
                  @click="deleteLink(row)"
                >
                  Remover
                </button>
              </div>
            </td>
          </tr>
          <tr v-if="!employees.length">
            <td colspan="5" class="erp-link-panel__empty">Nenhum funcionario ERP encontrado.</td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>

<style scoped>
.erp-link-panel {
  display: grid;
  gap: 0.85rem;
}

.erp-link-panel__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
}

.erp-link-panel__header-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 0.55rem;
}

.erp-link-panel__title {
  margin: 0;
  color: var(--text-main);
  font-size: 0.95rem;
}

.erp-link-panel__text,
.erp-link-panel__empty,
.erp-link-panel__current,
.erp-link-panel__identity small {
  color: var(--text-muted);
  font-size: 0.78rem;
}

.erp-link-panel__table-wrap {
  overflow-x: auto;
  border: 1px solid var(--line-soft);
  border-radius: 0.75rem;
}

.erp-link-panel__table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.82rem;
}

.erp-link-panel__table th,
.erp-link-panel__table td {
  padding: 0.62rem 0.75rem;
  text-align: left;
  border-bottom: 1px solid var(--line-soft);
  vertical-align: middle;
  white-space: nowrap;
}

.erp-link-panel__table th {
  color: var(--text-muted);
  font-size: 0.7rem;
  font-weight: 800;
  text-transform: uppercase;
}

.erp-link-panel__identity {
  display: grid;
  gap: 0.16rem;
}

.erp-link-panel__badge {
  display: inline-flex;
  align-items: center;
  min-height: 1.35rem;
  padding: 0.16rem 0.45rem;
  border-radius: 999px;
  font-size: 0.67rem;
  font-weight: 800;
  line-height: 1;
  text-transform: uppercase;
}

.erp-link-panel__badge--ok {
  background: rgb(var(--success) / 0.12);
  color: var(--erp-success-text);
}

.erp-link-panel__badge--info,
.erp-link-panel__badge--warn {
  background: rgb(var(--primary) / 0.12);
  color: rgb(var(--primary));
}

.erp-link-panel__badge--neutral {
  background: rgb(var(--surface-2) / 0.78);
  color: var(--text-muted);
}

.erp-link-panel__current {
  display: block;
  margin-top: 0.22rem;
}

.erp-link-panel__select {
  min-width: 240px;
  border: 1px solid var(--line-soft);
  border-radius: 0.55rem;
  background: var(--erp-control-bg);
  color: var(--text-main);
  padding: 0.48rem 0.6rem;
}

.erp-link-panel__actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.45rem;
}

.erp-link-panel__button {
  border: 1px solid rgb(var(--primary) / 0.35);
  border-radius: 0.55rem;
  background: rgb(var(--primary) / 0.16);
  color: rgb(var(--primary));
  cursor: pointer;
  font-size: 0.76rem;
  font-weight: 800;
  padding: 0.48rem 0.68rem;
}

.erp-link-panel__button--ghost {
  border-color: var(--line-soft);
  background: transparent;
  color: var(--text-muted);
}

.erp-link-panel__button:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}

@media (max-width: 860px) {
  .erp-link-panel__header {
    align-items: flex-start;
    flex-direction: column;
  }

  .erp-link-panel__header-actions {
    justify-content: flex-start;
  }
}
</style>
