export type TaskVideoUploadPhase = 'uploading' | 'processing' | 'linking'

export interface TaskVideoUploadProgress {
  key: string
  name: string
  percent: number
  phase: TaskVideoUploadPhase
  previewUrl: string
}

interface TaskVideoUploadRequest {
  endpoint: string
  token: string
  accountId: string
  file: File
  checklistItemId: string
  onProgress: (percent: number) => void
  onPhase: (phase: TaskVideoUploadPhase) => void
}

export class TaskVideoUploadError extends Error {
  readonly status: number
  readonly code: string

  constructor(message: string, status = 0, code = '') {
    super(message)
    this.name = 'TaskVideoUploadError'
    this.status = status
    this.code = code
  }
}

function responseError(xhr: XMLHttpRequest): TaskVideoUploadError {
  let code = ''
  let message = `O servidor recusou o upload (HTTP ${xhr.status}).`
  try {
    const body = JSON.parse(xhr.responseText) as {
      error?: { code?: string; message?: string }
    }
    code = String(body.error?.code || '')
    message = String(body.error?.message || message)
  } catch {
    // Resposta nao-JSON: preserva status e mensagem padrao.
  }
  return new TaskVideoUploadError(message, xhr.status, code)
}

export function uploadTaskVideoFile(request: TaskVideoUploadRequest): Promise<unknown> {
  return new Promise((resolve, reject) => {
    const form = new FormData()
    form.append('video', request.file)
    if (request.checklistItemId) form.append('checklistItemId', request.checklistItemId)

    const xhr = new XMLHttpRequest()
    xhr.open('POST', request.endpoint)
    xhr.timeout = 15 * 60 * 1000
    if (request.token) xhr.setRequestHeader('Authorization', `Bearer ${request.token}`)
    xhr.setRequestHeader('X-Account-Id', request.accountId)
    xhr.setRequestHeader('Idempotency-Key', crypto.randomUUID())

    xhr.upload.onprogress = (event) => {
      if (event.lengthComputable) {
        request.onProgress(Math.round((event.loaded / event.total) * 100))
      }
    }
    xhr.upload.onload = () => {
      request.onProgress(100)
      request.onPhase('processing')
    }
    xhr.onload = () => {
      if (xhr.status < 200 || xhr.status >= 300) {
        reject(responseError(xhr))
        return
      }
      try {
        const body = JSON.parse(xhr.responseText) as { video?: unknown }
        resolve(body.video)
      } catch {
        reject(new TaskVideoUploadError('A API devolveu uma resposta de upload invalida.'))
      }
    }
    xhr.onerror = () =>
      reject(new TaskVideoUploadError('A conexao com a API foi interrompida durante o envio.'))
    xhr.ontimeout = () =>
      reject(new TaskVideoUploadError('A API nao concluiu o upload dentro de 15 minutos.'))
    xhr.onabort = () => reject(new TaskVideoUploadError('O upload foi cancelado.'))

    request.onPhase('uploading')
    xhr.send(form)
  })
}
