import { ref } from 'vue'

import { useAudioMeter } from '~/composables/useAudioMeter'

// Ditado AO VIVO (SPEC-F7, modo alternativo ao Whisper): usa a Web Speech API NATIVA
// do navegador (webkitSpeechRecognition) — as palavras aparecem enquanto a pessoa fala,
// como no Claude. Roda 100% no cliente (ZERO carga no servidor / whisper); porem o audio
// passa pelos servidores do Google e so funciona bem no Chrome/Edge. E' opt-in: o padrao
// do chat continua sendo o Whisper self-hosted (privado). Ver useVoiceRecorder p/ o Whisper.

export type LiveDictationState = 'idle' | 'listening'

// Tipagem minima da SpeechRecognition (nao esta no lib.dom padrao do TS) — so o que usamos.
interface SpeechAlternative {
  transcript: string
}
interface SpeechResult {
  isFinal: boolean
  0: SpeechAlternative
}
interface SpeechResultList {
  length: number
  [index: number]: SpeechResult
}
interface SpeechRecognitionEventLike {
  resultIndex: number
  results: SpeechResultList
}
interface SpeechRecognitionLike {
  lang: string
  continuous: boolean
  interimResults: boolean
  onresult: ((event: SpeechRecognitionEventLike) => void) | null
  onerror: ((event: { error?: string }) => void) | null
  onend: (() => void) | null
  start: () => void
  stop: () => void
  abort: () => void
}
type SpeechRecognitionCtor = new () => SpeechRecognitionLike

function getRecognitionCtor(): SpeechRecognitionCtor | null {
  if (typeof window === 'undefined') return null
  const w = window as unknown as {
    SpeechRecognition?: SpeechRecognitionCtor
    webkitSpeechRecognition?: SpeechRecognitionCtor
  }
  return w.SpeechRecognition || w.webkitSpeechRecognition || null
}

export function useLiveDictation() {
  const supported = getRecognitionCtor() !== null
  const state = ref<LiveDictationState>('idle')
  const transcript = ref('')
  const errorMessage = ref('')

  // A Web Speech API NAO expoe o audio: abrimos um stream paralelo do mic so para o
  // waveform (medidor compartilhado). Cosmetico — se falhar, o ditado segue sem barras.
  const meter = useAudioMeter(28)
  const levels = meter.levels
  let meterStream: MediaStream | null = null

  function startMeter(): void {
    if (typeof navigator === 'undefined' || !navigator.mediaDevices?.getUserMedia) return
    navigator.mediaDevices
      .getUserMedia({ audio: true })
      .then((s) => {
        if (state.value !== 'listening') {
          s.getTracks().forEach((track) => track.stop())
          return
        }
        meterStream = s
        meter.start(s)
      })
      .catch(() => {
        // sem waveform (ex.: permissao negada so p/ este stream); o ditado segue
      })
  }

  function stopMeter(): void {
    meter.stop()
    if (meterStream) {
      meterStream.getTracks().forEach((track) => track.stop())
      meterStream = null
    }
  }

  let recognition: SpeechRecognitionLike | null = null
  let finalText = ''
  // A Web Speech API ENCERRA a sessao sozinha (silencio ou o limite interno do Chrome, ~60s):
  // sem isto o ditado "cortava" no meio de uma fala longa. wantListening = a INTENCAO do usuario
  // (so o stop/cancel dele desliga); enquanto true, o onend RE-INICIA a sessao mantendo o texto
  // ja transcrito — ditado continuo, sem limite de tempo, ate a pessoa parar.
  let wantListening = false

  // beginSession cria e inicia UMA sessao de reconhecimento (reutilizada nos reinicios). O
  // finalText acumula ENTRE sessoes (variavel de fora), entao o texto nunca se perde no restart.
  function beginSession(): boolean {
    const Ctor = getRecognitionCtor()
    if (!Ctor) return false
    recognition = new Ctor()
    recognition.lang = 'pt-BR'
    recognition.continuous = true
    recognition.interimResults = true
    recognition.onresult = (event) => {
      let interim = ''
      for (let i = event.resultIndex; i < event.results.length; i += 1) {
        const result = event.results[i]
        if (result.isFinal) {
          // separa segmentos de sessoes diferentes com espaco quando o browser nao traz.
          if (finalText && !/\s$/.test(finalText) && !/^\s/.test(result[0].transcript)) {
            finalText += ' '
          }
          finalText += result[0].transcript
        } else {
          interim += result[0].transcript
        }
      }
      transcript.value = `${finalText}${interim}`.trim()
    }
    recognition.onerror = (event) => {
      const code = event?.error || ''
      // Erros FATAIS: desligam o ditado (nao adianta reiniciar) e avisam.
      if (code === 'not-allowed' || code === 'service-not-allowed' || code === 'audio-capture') {
        wantListening = false
        errorMessage.value =
          'Permissao de microfone negada. Libere o microfone nas configuracoes do navegador.'
      }
      // no-speech / network / aborted: transitorios — o onend reinicia (silencio nao encerra o ditado).
    }
    recognition.onend = () => {
      // A sessao terminou (limite do browser ou silencio). Se o usuario AINDA quer ditar,
      // reinicia mantendo o texto; senao, encerra de verdade.
      if (wantListening && beginSession()) return
      stopMeter()
      state.value = 'idle'
    }
    try {
      recognition.start()
    } catch {
      return false
    }
    return true
  }

  // Comeca a ouvir. Retorna false (com errorMessage) quando o navegador nao suporta.
  function start(): boolean {
    const Ctor = getRecognitionCtor()
    if (!Ctor) {
      errorMessage.value =
        'Ditado ao vivo indisponivel neste navegador (ex.: Firefox). Use Chrome, Edge ou Safari, ou troque para o Whisper (self-hosted, funciona em todos).'
      return false
    }
    if (state.value !== 'idle') return false
    errorMessage.value = ''
    finalText = ''
    transcript.value = ''
    wantListening = true
    if (!beginSession()) {
      wantListening = false
      errorMessage.value = 'Nao foi possivel iniciar o ditado ao vivo.'
      recognition = null
      return false
    }
    state.value = 'listening'
    startMeter()
    return true
  }

  // Para de ouvir mantendo o texto ja transcrito.
  function stop(): void {
    wantListening = false // desliga a intencao ANTES do stop: o onend nao reinicia.
    if (recognition) {
      try {
        recognition.stop()
      } catch {
        // ignora: onend zera o estado
      }
    }
    stopMeter()
    state.value = 'idle'
  }

  // Cancela e descarta (ex.: fechar o chat).
  function cancel(): void {
    wantListening = false
    stopMeter()
    if (recognition) {
      try {
        recognition.abort()
      } catch {
        // ignora
      }
    }
    recognition = null
    state.value = 'idle'
    transcript.value = ''
  }

  return { supported, state, transcript, errorMessage, levels, start, stop, cancel }
}
