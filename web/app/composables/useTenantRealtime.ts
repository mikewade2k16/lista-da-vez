export function useTenantRealtime() {
  function start() {
    return
  }

  function subscribeEntity(_entity: string, _callback: () => void) {
    return () => {}
  }

  return {
    start,
    subscribeEntity,
  }
}
