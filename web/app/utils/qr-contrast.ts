// Regra de legibilidade do QR, espelho de back/internal/modules/tools/qr_contrast.go.
// O back e a fonte autoritativa (recusa com 400 low_contrast); aqui a tela avisa
// antes de salvar. Mudou o limite? Mude nos dois lados.
export const MIN_QR_CONTRAST = 4.5

const HEX = /^#[0-9a-fA-F]{6}$/

function channel(value: number) {
  const c = value / 255
  return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4
}

function luminance(hex: string) {
  const v = Number.parseInt(hex.slice(1), 16)
  return (
    0.2126 * channel((v >> 16) & 0xff) +
    0.7152 * channel((v >> 8) & 0xff) +
    0.0722 * channel(v & 0xff)
  )
}

// reason vazio = aprovado. Tipo plano (sem uniao discriminada): o projeto roda
// sem strict e a uniao nao estreitaria.
export interface QrContrastCheck {
  ok: boolean
  ratio: number
  reason: '' | 'invalid' | 'inverted' | 'low'
}

export function checkQrContrast(fill: string, back: string): QrContrastCheck {
  if (!HEX.test(fill) || !HEX.test(back)) return { ok: false, ratio: 0, reason: 'invalid' }
  const lf = luminance(fill)
  const lb = luminance(back)
  const ratio = (Math.max(lf, lb) + 0.05) / (Math.min(lf, lb) + 0.05)
  if (lf >= lb) return { ok: false, ratio, reason: 'inverted' }
  if (ratio < MIN_QR_CONTRAST) return { ok: false, ratio, reason: 'low' }
  return { ok: true, ratio, reason: '' }
}

export function qrContrastMessage(check: QrContrastCheck) {
  if (check.ok) return ''
  if (check.reason === 'invalid') return 'Cor invalida. Use o formato #rrggbb.'
  if (check.reason === 'inverted') {
    return 'A cor do QR precisa ser mais escura que o fundo. QR claro em fundo escuro falha em varias cameras.'
  }
  return `Pouco contraste (${check.ratio.toFixed(1)}:1, minimo ${MIN_QR_CONTRAST}:1). A camera pode nao ler este QR.`
}
