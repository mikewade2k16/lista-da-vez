import type { ComputedRef, Ref } from 'vue'

import { ERP_RECORDS_LABEL_BY_TAB } from '~/domain/utils/erp-display'

type SyncResult = {
  ok: boolean
  message?: string
  data?: {
    filesImported?: number | null
    rowsImported?: number | null
  } | null
}

type ErpSyncStore = {
  backfillStore: () => Promise<SyncResult>
  bootstrapDataType: (payload: { dataType: string }) => Promise<SyncResult>
  bootstrapItems: () => Promise<SyncResult>
  syncStore: () => Promise<SyncResult>
}

type UiNotifier = {
  error: (message: string) => void
  info: (message: string, title?: string) => void
  success: (message: string, title?: string) => void
}

type UseErpSyncActionsOptions = {
  activeRecordsDataType: ComputedRef<string>
  activeTab: Ref<string>
  erpStore: ErpSyncStore
  reloadWorkspace: () => Promise<void>
  ui: UiNotifier
}

export function useErpSyncActions(options: UseErpSyncActionsOptions) {
  async function handleBootstrap() {
    const result = await options.erpStore.bootstrapItems()
    if (!result.ok) {
      options.ui.error(result.message || 'Nao foi possivel iniciar o bootstrap do ERP.')
      return
    }
    options.ui.success(
      `Bootstrap ERP concluido: ${result.data?.rowsImported || 0} linhas importadas em ${result.data?.filesImported || 0} lotes.`,
    )
    await options.reloadWorkspace()
  }

  async function handleRecordsBootstrap() {
    const dataType = options.activeRecordsDataType.value
    const label = ERP_RECORDS_LABEL_BY_TAB[options.activeTab.value] || 'registros'
    if (!dataType) {
      options.ui.error('Selecione uma aba ERP valida antes de iniciar o bootstrap.')
      return
    }
    const result = await options.erpStore.bootstrapDataType({ dataType })
    if (!result.ok) {
      options.ui.error(result.message || 'Nao foi possivel iniciar o bootstrap do ERP.')
      return
    }
    options.ui.success(
      `Bootstrap ERP de ${label} concluido: ${result.data?.rowsImported || 0} linhas importadas em ${result.data?.filesImported || 0} lotes.`,
    )
    await options.reloadWorkspace()
  }

  async function handleSyncNow() {
    options.ui.info(
      'Sincronizacao manual do ERP iniciada. Vou atualizar a tela quando terminar.',
      'ERP',
    )
    const result = await options.erpStore.syncStore()
    if (!result.ok) {
      options.ui.error(result.message || 'Nao foi possivel iniciar a sincronizacao ERP.')
      return
    }
    options.ui.success(
      `Sincronizacao ERP concluida: ${result.data?.rowsImported || 0} linhas importadas em ${result.data?.filesImported || 0} arquivos.`,
    )
    await options.reloadWorkspace()
  }

  async function handleBackfillSync() {
    const confirmed =
      typeof window === 'undefined'
        ? true
        : window.confirm('Iniciar o backfill retroativo do ERP para o escopo completo do sistema?')
    if (!confirmed) return
    options.ui.info('Backfill ERP iniciado. Esse processo pode levar alguns minutos.', 'ERP')
    const result = await options.erpStore.backfillStore()
    if (!result.ok) {
      options.ui.error(result.message || 'Nao foi possivel iniciar o backfill ERP.')
      return
    }
    options.ui.success(
      `Backfill ERP concluido: ${result.data?.rowsImported || 0} linhas importadas em ${result.data?.filesImported || 0} arquivos.`,
    )
    await options.reloadWorkspace()
  }

  return {
    handleBackfillSync,
    handleBootstrap,
    handleRecordsBootstrap,
    handleSyncNow,
  }
}
