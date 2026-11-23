import { loadImageElement, replaceFileExtension } from '~/utils/feedback-image'

// Foto de perfil: aceita qualquer imagem que o navegador leia (sem limite de MB na
// entrada) e envia um quadrado centralizado de ate 512px em WebP (JPEG se o
// navegador nao codificar WebP). O avatar aparece com ~136px, entao 512px segue
// nitido em tela retina e o arquivo fica com dezenas de KB, abaixo do limite do back.
export const AVATAR_SIZE = 512
const AVATAR_QUALITY = 0.86

export function avatarCrop(width: number, height: number, max = AVATAR_SIZE) {
  const side = Math.min(width, height)
  return {
    sx: Math.round((width - side) / 2),
    sy: Math.round((height - side) / 2),
    side,
    size: Math.max(1, Math.min(max, side)),
  }
}

function encode(canvas: HTMLCanvasElement, mimeType: string) {
  return new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, mimeType, AVATAR_QUALITY))
}

export async function prepareAvatarImage(file: File): Promise<File> {
  if (!String(file?.type || '').startsWith('image/')) {
    throw new Error('Escolha um arquivo de imagem.')
  }
  if (!import.meta.client) return file

  const image = await loadImageElement(file)
  const crop = avatarCrop(image.naturalWidth, image.naturalHeight)
  const canvas = document.createElement('canvas')
  canvas.width = crop.size
  canvas.height = crop.size
  const context = canvas.getContext('2d')
  if (!context) throw new Error('Nao foi possivel preparar a foto.')
  context.imageSmoothingQuality = 'high'
  context.drawImage(image, crop.sx, crop.sy, crop.side, crop.side, 0, 0, crop.size, crop.size)

  for (const [mimeType, extension] of [
    ['image/webp', '.webp'],
    ['image/jpeg', '.jpg'],
  ] as const) {
    const blob = await encode(canvas, mimeType)
    // Navegador sem encoder WebP devolve PNG: tenta o proximo formato.
    if (blob && blob.type === mimeType) {
      return new File([blob], replaceFileExtension(file.name, extension), {
        type: mimeType,
        lastModified: Date.now(),
      })
    }
  }
  throw new Error('Nao foi possivel preparar a foto.')
}
