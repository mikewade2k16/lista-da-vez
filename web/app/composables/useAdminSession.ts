export function useAdminSession() {
  const coreToken = useState<string>('reference-admin-core-token', () => 'mock-reference-session')

  function hydrate() {
    return
  }

  return {
    coreToken,
    hydrate,
  }
}
