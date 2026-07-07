import { ref } from 'vue'

// Medidor de amplitude do microfone (waveform REAL, tipo WhatsApp). Extraido para ser
// compartilhado entre o Whisper (useVoiceRecorder) e o ditado ao vivo (useLiveDictation):
// recebe um MediaStream, mede a amplitude via Web Audio (AnalyserNode) e mantem uma janela
// das ultimas amostras em `levels` (0 = silencio, ~1 = falando alto). E' cosmetico: se o
// Web Audio falhar, as barras ficam paradas e a captura segue normal.

export function useAudioMeter(barCount = 28) {
  const levels = ref<number[]>(new Array(barCount).fill(0))
  let audioCtx: AudioContext | null = null
  let analyser: AnalyserNode | null = null
  // Tipado com ArrayBuffer (nao ArrayBufferLike) para casar com getByteTimeDomainData no
  // TS 5.7+, onde Uint8Array virou generico sobre o buffer.
  let buf: Uint8Array<ArrayBuffer> | null = null
  let rafId = 0
  let lastPush = 0

  // Loop de amostragem: le a amplitude e empurra uma barra ~a cada 55ms. Isolado para
  // poder ser retomado no resume() sem recriar o AnalyserNode.
  function loop(t: number): void {
    if (!analyser || !buf) return
    analyser.getByteTimeDomainData(buf)
    let sum = 0
    for (let i = 0; i < buf.length; i += 1) {
      const v = (buf[i] - 128) / 128
      sum += v * v
    }
    const rms = Math.sqrt(sum / buf.length)
    const level = Math.min(1, rms * 2.6)
    // Empurra uma amostra ~a cada 55ms (janela de ~1.5s nas barras).
    if (t - lastPush > 55) {
      lastPush = t
      const next = levels.value.slice(1)
      next.push(level)
      levels.value = next
    }
    rafId = window.requestAnimationFrame(loop)
  }

  function start(stream: MediaStream): void {
    try {
      const Ctx =
        window.AudioContext ||
        (window as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext
      if (!Ctx) return
      audioCtx = new Ctx()
      const source = audioCtx.createMediaStreamSource(stream)
      analyser = audioCtx.createAnalyser()
      analyser.fftSize = 256
      source.connect(analyser)
      buf = new Uint8Array(analyser.frequencyBinCount)
      rafId = window.requestAnimationFrame(loop)
    } catch {
      // Medidor cosmetico: falha nao impede a captura.
    }
  }

  // Congela a waveform (mantem o AnalyserNode/contexto vivos) — usado no pause da gravacao:
  // as barras param no ultimo estado em vez de sumirem, igual ao WhatsApp.
  function pause(): void {
    if (rafId) {
      window.cancelAnimationFrame(rafId)
      rafId = 0
    }
  }

  // Retoma a amostragem apos um pause() (o contexto/analyser seguem vivos).
  function resume(): void {
    if (!analyser || rafId) return
    rafId = window.requestAnimationFrame(loop)
  }

  function stop(): void {
    if (rafId) {
      window.cancelAnimationFrame(rafId)
      rafId = 0
    }
    analyser = null
    buf = null
    if (audioCtx) {
      void audioCtx.close().catch(() => {})
      audioCtx = null
    }
    levels.value = new Array(barCount).fill(0)
  }

  return { levels, start, stop, pause, resume }
}
