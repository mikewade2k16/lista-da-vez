import type { ErpRun } from '~/domain/utils/erp-display'

type ErpStatusProvider = {
  status?: {
    lastRun?: ErpRun | null
  } | null
}

type UiNotifier = {
  info: (message: string, title?: string) => void
  success: (message: string, title?: string) => void
}

type UseErpSyncNotificationsOptions = {
  erpStore: ErpStatusProvider
  ui: UiNotifier
}

export function useErpSyncNotifications(options: UseErpSyncNotificationsOptions) {
  const announcedAutomaticRunIds = new Set<string>()

  function notifyAutomaticSyncRun() {
    const run = options.erpStore.status?.lastRun
    if (!run?.id || run.triggeredBy !== 'cron' || announcedAutomaticRunIds.has(run.id)) return

    if (run.status === 'running') {
      announcedAutomaticRunIds.add(run.id)
      options.ui.info(
        'Sincronizacao automatica do ERP em andamento. A rotina agendada foi retomada e a tela sera atualizada em seguida.',
        'ERP',
      )
      return
    }

    const finishedAt = run.finishedAt ? Date.parse(run.finishedAt) : 0
    const finishedRecently = Number.isFinite(finishedAt) && Date.now() - finishedAt <= 5 * 60 * 1000
    if (run.status === 'succeeded' && finishedRecently) {
      announcedAutomaticRunIds.add(run.id)
      options.ui.success(
        `Sincronizacao automatica do ERP concluida: ${run.rowsImported || 0} linhas importadas em ${run.filesImported || 0} arquivos.`,
        'ERP',
      )
    }
  }

  return { notifyAutomaticSyncRun }
}
