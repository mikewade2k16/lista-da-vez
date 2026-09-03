export function isAssistantRoute(path: unknown): boolean {
  const normalizedPath = String(path || '')
  return (
    normalizedPath === '/calendario' ||
    normalizedPath.startsWith('/calendario/') ||
    normalizedPath === '/meta-ads' ||
    normalizedPath.startsWith('/meta-ads/')
  )
}
