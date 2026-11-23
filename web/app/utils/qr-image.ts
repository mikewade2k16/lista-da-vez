import QRCode from 'qrcode'

import type { QrCodeItem } from '~/types/tools'

// Gera o PNG (data URL) de um QR no cliente, sempre a partir do qrUrl rastreado
// (/q/{slug}) que veio do banco. Correcao de erro H: aguenta impressao gasta.
export function buildQrDataUrl(content: string, fill: string, back: string, size: number) {
  return QRCode.toDataURL(content, {
    width: size,
    margin: 1,
    color: { dark: fill, light: back },
    errorCorrectionLevel: 'H',
  })
}

// Baixa o PNG do QR salvo (cores e tamanho gravados no banco).
export async function downloadQrImage(item: QrCodeItem) {
  if (!import.meta.client || !item.qrUrl) return false
  try {
    const dataUrl = await buildQrDataUrl(item.qrUrl, item.fillColor, item.backColor, item.size)
    const anchor = document.createElement('a')
    anchor.href = dataUrl
    anchor.download = `${item.slug || 'qrcode'}.png`
    anchor.rel = 'noopener'
    document.body.appendChild(anchor)
    anchor.click()
    anchor.remove()
    return true
  } catch {
    return false
  }
}

// Copia um texto para a area de transferencia; false quando o navegador nega.
export async function copyToClipboard(value: string) {
  const text = String(value ?? '').trim()
  if (!import.meta.client || !text || !navigator.clipboard?.writeText) return false
  try {
    await navigator.clipboard.writeText(text)
    return true
  } catch {
    return false
  }
}
