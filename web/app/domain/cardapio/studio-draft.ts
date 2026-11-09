import type { SiteLayout } from './types'

// O painel aceita respostas somente depois de validar origin/source do iframe.
// Nunca salvar a copia local se o Studio nao confirmar o documento atual.
export function createStudioSnapshotRequest(post: (message: Record<string, unknown>) => void) {
  let sequence = 0
  let pending: {
    id: string
    resolve: (layout: SiteLayout) => void
    reject: (error: Error) => void
    timer: ReturnType<typeof setTimeout>
  } | null = null

  function cancel() {
    if (!pending) return
    clearTimeout(pending.timer)
    pending.reject(new Error('O editor foi fechado. Reabra a aba Site antes de salvar.'))
    pending = null
  }

  function request(): Promise<SiteLayout> {
    cancel()
    return new Promise((resolve, reject) => {
      const id = `snapshot-${++sequence}`
      const timer = setTimeout(() => {
        pending = null
        reject(
          new Error(
            'O Studio nao confirmou as alteracoes. Atualize o Studio/TAVOLA e recarregue a aba Site. Nada foi publicado.',
          ),
        )
      }, 5000)
      pending = { id, resolve, reject, timer }
      post({ type: 'snapshot-request', requestId: id })
    })
  }

  function accept(requestId: string | undefined, layout: SiteLayout | null | undefined) {
    if (!pending || requestId !== pending.id || !layout?.pages) return
    clearTimeout(pending.timer)
    const resolve = pending.resolve
    pending = null
    resolve(JSON.parse(JSON.stringify(layout)))
  }

  return { request, accept, cancel }
}
