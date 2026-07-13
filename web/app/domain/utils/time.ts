type FormatDurationOptions = {
  roundUpPartialSecond?: boolean
}

export function formatDuration(durationMs, options: FormatDurationOptions = {}) {
  const normalizedDurationMs = Math.max(0, Number(durationMs || 0) || 0)
  const totalSeconds =
    options?.roundUpPartialSecond && normalizedDurationMs > 0
      ? Math.ceil(normalizedDurationMs / 1000)
      : Math.floor(normalizedDurationMs / 1000)
  const hours = String(Math.floor(totalSeconds / 3600)).padStart(2, '0')
  const minutes = String(Math.floor((totalSeconds % 3600) / 60)).padStart(2, '0')
  const seconds = String(totalSeconds % 60).padStart(2, '0')

  return `${hours}:${minutes}:${seconds}`
}

// Regra de tempo "humana" do modulo Operacao (usada no aviso de auto-encerramento):
// abaixo de 1 min mostra segundos; de 1 min ate <1 h mostra min+seg; de 1 h em diante
// mostra h+min; de 1 dia em diante dia+h — sempre a unidade maior + a proxima.
// Ex.: 109s -> "1min 49s"; 3665s -> "1h 1min".
export function formatOperationCountdown(durationMs) {
  const totalSeconds = Math.max(0, Math.ceil(Number(durationMs || 0) / 1000))

  if (totalSeconds < 60) {
    return `${totalSeconds}s`
  }
  if (totalSeconds < 3600) {
    const minutes = Math.floor(totalSeconds / 60)
    const seconds = totalSeconds % 60
    return seconds > 0 ? `${minutes}min ${seconds}s` : `${minutes}min`
  }
  if (totalSeconds < 86400) {
    const hours = Math.floor(totalSeconds / 3600)
    const minutes = Math.floor((totalSeconds % 3600) / 60)
    return minutes > 0 ? `${hours}h ${minutes}min` : `${hours}h`
  }
  const days = Math.floor(totalSeconds / 86400)
  const hours = Math.floor((totalSeconds % 86400) / 3600)
  return hours > 0 ? `${days}d ${hours}h` : `${days}d`
}

export function formatClock(dateValue) {
  const date = new Date(dateValue)
  const hours = String(date.getHours()).padStart(2, '0')
  const minutes = String(date.getMinutes()).padStart(2, '0')

  return `${hours}:${minutes}`
}
