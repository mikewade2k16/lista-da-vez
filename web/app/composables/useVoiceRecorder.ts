import { computed, onScopeDispose, ref, watch } from 'vue'

import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import { useAudioMeter } from '~/composables/useAudioMeter'
import { useAuthStore } from '~/stores/auth'
import { useCoreAccountStore } from '../../layers/core/stores/account'
import {
  assistantTranscribePath,
  type AssistantChatSurface,
} from '~/domain/calendar/calendar-chat-api'

// Gravador de voz do assistente compartilhado (SPEC-F7, contrato C8). Captura audio via
// MediaRecorder, para em 2min (limite), e envia o blob multipart para
// POST /v1/assistant/chat/transcribe devolvendo o texto. O texto vai
// para o INPUT do chat (o usuario revisa e envia — nao dispara direto). Nada e'
// gravado em disco no back: o multipart e' repassado ao n8n.

export type VoiceRecorderState = 'idle' | 'recording' | 'transcribing'

// Limite de 2 minutos: a gravacao para sozinha ao atingir o teto.
const MAX_DURATION_MS = 2 * 60 * 1000

// Ordem de preferencia dos codecs. webm/opus e' o padrao (Chrome/Firefox/Edge);
// Safari nao suporta webm e cai no mp4. O mime escolhido volta no header do
// multipart para o back/n8n identificar o formato.
const MIME_CANDIDATES = ['audio/webm;codecs=opus', 'audio/webm', 'audio/mp4']

interface TranscribeResponse {
  text?: string
}

interface VoiceContextSnapshot {
  generation: number
  key: string
  surface: AssistantChatSurface
}

function pickMimeType(): string {
  if (typeof MediaRecorder === 'undefined') {
    return ''
  }
  for (const mime of MIME_CANDIDATES) {
    if (MediaRecorder.isTypeSupported(mime)) {
      return mime
    }
  }
  return ''
}

function httpStatus(error: unknown): number {
  const e = error as { status?: number; statusCode?: number; response?: { status?: number } }
  return Number(e?.status ?? e?.statusCode ?? e?.response?.status ?? 0)
}

function errorCode(error: unknown): string {
  const e = error as { data?: { error?: { code?: string } } }
  return String(e?.data?.error?.code || '')
}

export function useVoiceRecorder(surface: () => AssistantChatSurface) {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const accountStore = useCoreAccountStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  const state = ref<VoiceRecorderState>('idle')
  const errorMessage = ref('')
  // Pausado (MediaRecorder.pause): a gravacao continua "recording" mas nao captura audio;
  // o timer e a waveform congelam (igual ao WhatsApp).
  const paused = ref(false)
  // Tempo decorrido de gravacao em ms (nao conta o tempo pausado): alimenta o timer mm:ss.
  const elapsedMs = ref(0)
  // Waveform REAL (tipo WhatsApp): amplitude do microfone via medidor compartilhado.
  const meter = useAudioMeter(28)
  const levels = meter.levels

  const supported =
    typeof navigator !== 'undefined' &&
    !!navigator.mediaDevices &&
    typeof navigator.mediaDevices.getUserMedia === 'function' &&
    typeof MediaRecorder !== 'undefined'

  let recorder: MediaRecorder | null = null
  let stream: MediaStream | null = null
  let stopTimer = 0
  let transcriptionController: AbortController | null = null
  // getUserMedia nao aceita AbortSignal e pode ficar aguardando a permissao do
  // navegador enquanto o estado visual ainda e `idle`. O snapshot pendente faz
  // single-flight desse intervalo e permite que cancel/troca de contexto invalide
  // a stream antes que ela vire uma gravacao.
  let pendingStart: VoiceContextSnapshot | null = null
  // Promise pendente do stop(): resolvida no onstop com o blob capturado.
  let resolveStop: ((blob: Blob | null) => void) | null = null

  // Toda captura/transcricao pertence a uma identidade + surface imutavel. O chat e
  // singleton no shell; sem esta fronteira uma resposta tardia da conta anterior
  // poderia repovoar o draft depois da troca de contexto.
  const contextKey = computed(() => {
    const accountId = String(accountStore.activeAccountId || '').trim()
    const userId = String(
      auth.principal?.userId || auth.principal?.userID || auth.user?.id || '',
    ).trim()
    return `${accountId}:${userId}:${surface()}`
  })
  let contextGeneration = 0
  let boundContextKey = '\u0000'

  function contextSnapshot(): VoiceContextSnapshot {
    return {
      generation: contextGeneration,
      key: contextKey.value,
      surface: surface(),
    }
  }

  function isCurrentContext(snapshot: VoiceContextSnapshot): boolean {
    return (
      snapshot.generation === contextGeneration &&
      snapshot.key === boundContextKey &&
      snapshot.key === contextKey.value &&
      snapshot.surface === surface()
    )
  }

  function isAbortError(error: unknown): boolean {
    return (
      (error instanceof DOMException && error.name === 'AbortError') ||
      (error as { name?: string } | null)?.name === 'AbortError'
    )
  }

  // Cronometro do timer: acumula o tempo GRAVADO (descontando pausas). elapsedBase guarda
  // os segmentos ja fechados; segStart marca o inicio do segmento em andamento. Usa
  // performance.now() (monotonico) e um interval so para atualizar o ref na tela.
  let elapsedBase = 0
  let segStart = 0
  let elapsedTimer = 0

  function startElapsed(): void {
    segStart = performance.now()
    if (elapsedTimer) window.clearInterval(elapsedTimer)
    elapsedTimer = window.setInterval(() => {
      elapsedMs.value = elapsedBase + (performance.now() - segStart)
    }, 200)
  }

  // Fecha o segmento atual (soma no acumulado) e para o interval — usado no pause.
  function freezeElapsed(): void {
    if (elapsedTimer) {
      window.clearInterval(elapsedTimer)
      elapsedTimer = 0
    }
    if (segStart) {
      elapsedBase += performance.now() - segStart
      segStart = 0
    }
    elapsedMs.value = elapsedBase
  }

  function resetElapsed(): void {
    if (elapsedTimer) {
      window.clearInterval(elapsedTimer)
      elapsedTimer = 0
    }
    elapsedBase = 0
    segStart = 0
    elapsedMs.value = 0
  }

  function cleanup(): void {
    if (stopTimer) {
      window.clearTimeout(stopTimer)
      stopTimer = 0
    }
    resetElapsed()
    paused.value = false
    meter.stop()
    if (stream) {
      stream.getTracks().forEach((track) => track.stop())
      stream = null
    }
    recorder = null
  }

  // Pede permissao e comeca a gravar. Retorna false (com errorMessage acionavel)
  // quando o navegador nao suporta ou o usuario nega o microfone.
  async function start(): Promise<boolean> {
    if (!supported) {
      errorMessage.value =
        'Seu navegador nao suporta gravacao de audio. Use o Chrome, Edge ou Firefox atualizado.'
      return false
    }
    if (state.value !== 'idle' || pendingStart) {
      return false
    }
    const snapshot = contextSnapshot()
    pendingStart = snapshot
    errorMessage.value = ''
    let acquiredStream: MediaStream
    try {
      acquiredStream = await navigator.mediaDevices.getUserMedia({ audio: true })
    } catch {
      if (pendingStart === snapshot) pendingStart = null
      if (isCurrentContext(snapshot)) {
        errorMessage.value =
          'Permissao de microfone negada. Libere o acesso ao microfone nas configuracoes do navegador e tente de novo.'
      }
      return false
    }
    if (pendingStart !== snapshot || !isCurrentContext(snapshot)) {
      acquiredStream.getTracks().forEach((track) => track.stop())
      return false
    }
    pendingStart = null
    stream = acquiredStream

    const mimeType = pickMimeType()
    let captureRecorder: MediaRecorder
    try {
      captureRecorder = mimeType
        ? new MediaRecorder(stream, { mimeType })
        : new MediaRecorder(stream)
    } catch {
      errorMessage.value = 'Nao foi possivel iniciar a gravacao neste navegador.'
      cleanup()
      return false
    }

    recorder = captureRecorder
    const captureChunks: Blob[] = []
    captureRecorder.ondataavailable = (event: BlobEvent) => {
      // `stop` e assincrono. Depois de cancel/troca de conta, eventos da
      // instancia anterior nao podem contaminar os chunks da captura nova.
      if (recorder !== captureRecorder) return
      if (event.data && event.data.size > 0) {
        captureChunks.push(event.data)
      }
    }
    captureRecorder.onstop = () => {
      // Um onstop tardio de uma captura cancelada deve ser um no-op: nao pode
      // resolver o stop nem limpar/parar o stream de uma gravacao posterior.
      if (recorder !== captureRecorder) return
      const type = captureRecorder.mimeType || 'audio/webm'
      const blob = captureChunks.length ? new Blob(captureChunks, { type }) : null
      const resolve = resolveStop
      resolveStop = null
      cleanup()
      state.value = 'idle'
      resolve?.(blob)
    }

    captureRecorder.start()
    meter.start(stream)
    paused.value = false
    resetElapsed()
    startElapsed()
    state.value = 'recording'
    // Limite de 2min: para sozinho.
    stopTimer = window.setTimeout(() => {
      void stopAndTranscribe()
    }, MAX_DURATION_MS)
    return true
  }

  // Pausa a captura: o audio para de ser gravado, o timer e a waveform congelam. O microfone
  // continua aberto (retomar e instantaneo). MediaRecorder.pause e suportado em
  // Chrome/Edge/Firefox/Safari 15+; se falhar, mantem gravando (nao quebra o fluxo).
  function pause(): void {
    if (!recorder || state.value !== 'recording' || paused.value) return
    try {
      recorder.pause()
    } catch {
      return
    }
    paused.value = true
    freezeElapsed()
    meter.pause()
  }

  // Retoma a captura apos um pause(): reabre o segmento do timer e a waveform.
  function resume(): void {
    if (!recorder || state.value !== 'recording' || !paused.value) return
    try {
      recorder.resume()
    } catch {
      return
    }
    paused.value = false
    startElapsed()
    meter.resume()
  }

  // Para a gravacao e resolve com o blob capturado (ou null se nada foi gravado).
  function stop(): Promise<Blob | null> {
    return new Promise((resolve) => {
      if (!recorder || (state.value !== 'recording' && state.value !== 'transcribing')) {
        resolve(null)
        return
      }
      // MediaRecorder.stop() conclui apenas no evento `stop`. Um segundo gesto
      // antes desse evento nao pode sobrescrever o resolver da primeira chamada.
      if (resolveStop) {
        resolve(null)
        return
      }
      resolveStop = resolve
      try {
        recorder.stop()
      } catch {
        resolveStop = null
        cleanup()
        state.value = 'idle'
        resolve(null)
      }
    })
  }

  // Descarta captura/transcricao. Tambem resolve um stop() pendente com null para
  // nao deixar a Promise suspensa quando a conta/surface muda entre stop e onstop.
  function cancel(invalidateOperation = true): void {
    if (invalidateOperation) contextGeneration += 1
    pendingStart = null
    transcriptionController?.abort()
    transcriptionController = null
    const pendingStop = resolveStop
    resolveStop = null
    if (recorder && state.value === 'recording') {
      try {
        recorder.stop()
      } catch {
        // ignora: o cleanup abaixo derruba tudo
      }
    }
    cleanup()
    state.value = 'idle'
    pendingStop?.(null)
  }

  function transcribeError(error: unknown): string {
    const status = httpStatus(error)
    const code = errorCode(error)
    if (status === 503 || code === 'transcribe_not_configured') {
      return 'A transcricao de voz ainda nao esta configurada para o assistente.'
    }
    if (status === 413 || code === 'media_too_large') {
      return 'Audio muito longo (limite 15 MB / 2 min). Grave um trecho menor.'
    }
    if (status === 400 || code === 'invalid_media') {
      return 'Formato de audio nao suportado. Tente gravar novamente.'
    }
    if (status === 502 || status === 504) {
      return 'O servico de transcricao nao respondeu agora. Tente novamente em instantes.'
    }
    return getApiErrorMessage(error, 'Nao foi possivel transcrever o audio agora. Tente novamente.')
  }

  // Envia o blob multipart para o back e devolve o texto (vazio em erro).
  async function transcribe(blob: Blob, snapshot: VoiceContextSnapshot): Promise<string> {
    if (!isCurrentContext(snapshot)) return ''
    transcriptionController?.abort()
    const controller = new AbortController()
    transcriptionController = controller
    state.value = 'transcribing'
    errorMessage.value = ''
    try {
      const form = new FormData()
      const ext = blob.type.includes('mp4') ? 'mp4' : 'webm'
      // Campo "file" (contrato C8); o api-client NAO serializa FormData e deixa o
      // browser montar o boundary do multipart.
      form.append('file', blob, `voz.${ext}`)
      const response = (await apiRequest(assistantTranscribePath(snapshot.surface), {
        method: 'POST',
        body: form,
        signal: controller.signal,
      })) as TranscribeResponse
      if (transcriptionController !== controller || !isCurrentContext(snapshot)) return ''
      return String(response?.text || '').trim()
    } catch (error) {
      if (
        isAbortError(error) ||
        transcriptionController !== controller ||
        !isCurrentContext(snapshot)
      ) {
        return ''
      }
      errorMessage.value = transcribeError(error)
      return ''
    } finally {
      if (transcriptionController === controller) {
        transcriptionController = null
        if (isCurrentContext(snapshot)) state.value = 'idle'
      }
    }
  }

  // Fluxo do botao mic: para a gravacao atual e transcreve o resultado.
  async function stopAndTranscribe(): Promise<string> {
    // Fecha a janela concorrente imediatamente. O evento `stop` do
    // MediaRecorder e assincrono; sem esta transicao, dois cliques (ou o timeout
    // junto do clique) ainda enxergariam `recording` e iniciariam dois fluxos.
    if (state.value !== 'recording') return ''
    const snapshot = contextSnapshot()
    state.value = 'transcribing'
    const blob = await stop()
    if (!isCurrentContext(snapshot)) return ''
    if (!blob) {
      errorMessage.value =
        'Nada foi gravado. Toque no microfone, fale e toque de novo para transcrever.'
      return ''
    }
    return transcribe(blob, snapshot)
  }

  watch(
    contextKey,
    (nextKey) => {
      if (nextKey === boundContextKey) return
      boundContextKey = nextKey
      contextGeneration += 1
      cancel(false)
    },
    { immediate: true, flush: 'sync' },
  )

  onScopeDispose(() => cancel())

  return {
    state,
    errorMessage,
    supported,
    levels,
    paused,
    elapsedMs,
    start,
    stop,
    cancel,
    pause,
    resume,
    stopAndTranscribe,
  }
}
