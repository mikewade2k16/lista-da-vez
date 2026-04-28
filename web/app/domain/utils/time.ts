export function formatDuration(durationMs, options = {}) {
  const normalizedDurationMs = Math.max(0, Number(durationMs || 0) || 0);
  const totalSeconds = options?.roundUpPartialSecond && normalizedDurationMs > 0
    ? Math.ceil(normalizedDurationMs / 1000)
    : Math.floor(normalizedDurationMs / 1000);
  const hours = String(Math.floor(totalSeconds / 3600)).padStart(2, "0");
  const minutes = String(Math.floor((totalSeconds % 3600) / 60)).padStart(2, "0");
  const seconds = String(totalSeconds % 60).padStart(2, "0");

  return `${hours}:${minutes}:${seconds}`;
}

export function formatClock(dateValue) {
  const date = new Date(dateValue);
  const hours = String(date.getHours()).padStart(2, "0");
  const minutes = String(date.getMinutes()).padStart(2, "0");

  return `${hours}:${minutes}`;
}
