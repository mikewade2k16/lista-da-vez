// Captura de poster (thumbnail) de um video no cliente, via <video> + <canvas>.
// Usado no upload de anexos do Calendario: depois de subir o video, gera um JPEG
// do primeiro frame para servir de capa. Puro DOM/canvas, sem estado nem fetch;
// isolado aqui para nao inflar useCalendarMedia.ts (limite de 450 linhas).

// Largura maxima do poster (px). Altura acompanha a proporcao do video.
const POSTER_MAX_WIDTH = 640
// Qualidade do JPEG de saida.
const POSTER_JPEG_QUALITY = 0.82
// Teto de tempo para toda a captura; estourou = desiste (null), o upload segue sem poster.
const CAPTURE_TIMEOUT_MS = 8000

// Deriva o nome do poster a partir do nome do video (poster-<base>.jpg).
function posterName(videoName: string): string {
  const base = videoName.replace(/\.[^./\\]+$/, '') || 'video'
  return `poster-${base}.jpg`
}

// capturePosterFromVideo desenha um frame inicial do video num canvas e devolve
// um File JPEG. Faz seek para min(0.5s, 10% da duracao) para evitar frame preto.
// Falha/timeout resolve null (o chamador trata como "sem poster"). SEMPRE
// revoga o objectURL, mesmo em erro.
export function capturePosterFromVideo(file: File): Promise<File | null> {
  return new Promise((resolve) => {
    const objectUrl = URL.createObjectURL(file)
    const video = document.createElement('video')
    video.muted = true
    video.playsInline = true
    video.preload = 'metadata'

    let settled = false
    const finish = (result: File | null): void => {
      if (settled) return
      settled = true
      window.clearTimeout(timer)
      video.removeAttribute('src')
      video.load()
      URL.revokeObjectURL(objectUrl)
      resolve(result)
    }

    const timer = window.setTimeout(() => finish(null), CAPTURE_TIMEOUT_MS)

    video.onloadedmetadata = () => {
      const duration = Number.isFinite(video.duration) ? video.duration : 0
      const target = duration > 0 ? Math.min(0.5, duration * 0.1) : 0
      // Alguns navegadores so disparam onseeked se o tempo mudar de fato.
      if (target > 0) video.currentTime = target
      else drawFrame()
    }

    video.onseeked = () => drawFrame()

    video.onerror = () => finish(null)

    function drawFrame(): void {
      const width = video.videoWidth
      const height = video.videoHeight
      if (!width || !height) {
        finish(null)
        return
      }
      const scale = Math.min(1, POSTER_MAX_WIDTH / width)
      const canvas = document.createElement('canvas')
      canvas.width = Math.max(1, Math.round(width * scale))
      canvas.height = Math.max(1, Math.round(height * scale))
      const ctx = canvas.getContext('2d')
      if (!ctx) {
        finish(null)
        return
      }
      ctx.drawImage(video, 0, 0, canvas.width, canvas.height)
      canvas.toBlob(
        (blob) => {
          if (!blob) {
            finish(null)
            return
          }
          finish(new File([blob], posterName(file.name), { type: 'image/jpeg' }))
        },
        'image/jpeg',
        POSTER_JPEG_QUALITY,
      )
    }

    video.src = objectUrl
  })
}
