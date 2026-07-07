import { ref } from 'vue'

import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import { useAudioMeter } from '~/composables/useAudioMeter'
import { useAuthStore } from '~/stores/auth'

// Gravador de voz do chat do Calendario (SPEC-F7, contrato C8). Captura audio via
// MediaRecorder, para em 2min (limite), e envia o blob multipart para
// POST /v1/calendar/chat/transcribe (n8n Whisper) devolvendo o texto. O texto vai
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

export function useVoiceRecorder() {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
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
  let chunks: Blob[] = []
  let stream: MediaStream | null = null
  let stopTimer = 0
  // Promise pendente do stop(): resolvida no onstop com o blob capturado.
  let resolveStop: ((blob: Blob | null) => void) | null = null

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
    chunks = []
  }

  // Pede permissao e comeca a gravar. Retorna false (com errorMessage acionavel)
  // quando o navegador nao suporta ou o usuario nega o microfone.
  async function start(): Promise<boolean> {
    if (!supported) {
      errorMessage.value =
        'Seu navegador nao suporta gravacao de audio. Use o Chrome, Edge ou Firefox atualizado.'
      return false
    }
    if (state.value !== 'idle') {
      return false
    }
    errorMessage.value = ''
    try {
      stream = await navigator.mediaDevices.getUserMedia({ audio: true })
    } catch {
      errorMessage.value =
        'Permissao de microfone negada. Libere o acesso ao microfone nas configuracoes do navegador e tente de novo.'
      return false
    }

    const mimeType = pickMimeType()
    try {
      recorder = mimeType ? new MediaRecorder(stream, { mimeType }) : new MediaRecorder(stream)
    } catch {
      errorMessage.value = 'Nao foi possivel iniciar a gravacao neste navegador.'
      cleanup()
      return false
    }

    chunks = []
    recorder.ondataavailable = (event: BlobEvent) => {
      if (event.data && event.data.size > 0) {
        chunks.push(event.data)
      }
    }
    recorder.onstop = () => {
      const type = recorder?.mimeType || 'audio/webm'
      const blob = chunks.length ? new Blob(chunks, { type }) : null
      const resolve = resolveStop
      resolveStop = null
      cleanup()
      state.value = 'idle'
      resolve?.(blob)
    }

    recorder.start()
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
      if (!recorder || state.value !== 'recording') {
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

  // Descarta a gravacao em andamento sem transcrever.
  function cancel(): void {
    if (recorder && state.value === 'recording') {
      resolveStop = null
      try {
        recorder.stop()
      } catch {
        // ignora: o cleanup abaixo derruba tudo
      }
    }
    cleanup()
    state.value = 'idle'
  }

  function transcribeError(error: unknown): string {
    const status = httpStatus(error)
    const code = errorCode(error)
    if (status === 503 || code === 'transcribe_not_configured') {
      return 'A transcricao de voz ainda nao esta configurada. Defina o env CALENDAR_TRANSCRIBE_WEBHOOK_URL e importe o workflow no n8n.'
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
  async function transcribe(blob: Blob): Promise<string> {
    state.value = 'transcribing'
    errorMessage.value = ''
    try {
      const form = new FormData()
      const ext = blob.type.includes('mp4') ? 'mp4' : 'webm'
      // Campo "file" (contrato C8); o api-client NAO serializa FormData e deixa o
      // browser montar o boundary do multipart.
      form.append('file', blob, `voz.${ext}`)
      const response = (await apiRequest('/v1/calendar/chat/transcribe', {
        method: 'POST',
        body: form,
      })) as TranscribeResponse
      state.value = 'idle'
      return String(response?.text || '').trim()
    } catch (error) {
      state.value = 'idle'
      errorMessage.value = transcribeError(error)
      return ''
    }
  }

  // Fluxo do botao mic: para a gravacao atual e transcreve o resultado.
  async function stopAndTranscribe(): Promise<string> {
    const blob = await stop()
    if (!blob) {
      errorMessage.value =
        'Nada foi gravado. Toque no microfone, fale e toque de novo para transcrever.'
      return ''
    }
    return transcribe(blob)
  }

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
