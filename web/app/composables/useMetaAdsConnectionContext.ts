import { ref } from 'vue'

export interface MetaAdsConnectionSnapshot {
  accountId: string
  generation: number
}

export interface MetaAdsOAuthPopupHandle {
  readonly closed?: boolean
  close: () => void
}

function normalizeAccountId(value: unknown): string {
  return String(value || '').trim()
}

// Estado efemero da conexao Meta. Ele nao pertence ao store porque token digitado,
// popup e polling existem somente enquanto o card esta montado; ainda assim precisam
// ficar presos a account em que a operacao comecou.
export function useMetaAdsConnectionContext(initialAccountId: unknown) {
  const token = ref('')
  const oauthPending = ref(false)
  const oauthError = ref('')

  let accountId = normalizeAccountId(initialAccountId)
  let generation = 0
  let popup: MetaAdsOAuthPopupHandle | null = null
  let pollTimer: ReturnType<typeof setTimeout> | null = null
  let pollAttempts = 0

  function capture(): MetaAdsConnectionSnapshot {
    return { accountId, generation }
  }

  function isCurrent(
    snapshot: MetaAdsConnectionSnapshot,
    liveAccountId: unknown = accountId,
  ): boolean {
    return (
      snapshot.accountId !== '' &&
      snapshot.accountId === accountId &&
      snapshot.accountId === normalizeAccountId(liveAccountId) &&
      snapshot.generation === generation
    )
  }

  function clearPollTimer(): void {
    if (pollTimer) clearTimeout(pollTimer)
    pollTimer = null
  }

  function stop(closePopup = false): void {
    clearPollTimer()
    oauthPending.value = false
    pollAttempts = 0
    if (closePopup) popup?.close()
    popup = null
  }

  function stopIfCurrent(snapshot: MetaAdsConnectionSnapshot, closePopup = false): boolean {
    if (!isCurrent(snapshot)) return false
    stop(closePopup)
    return true
  }

  function bindAccount(nextAccountId: unknown): boolean {
    const normalized = normalizeAccountId(nextAccountId)
    if (normalized === accountId) return false

    // Invalida primeiro para que callbacks sincronizados por popup.close() tambem
    // sejam considerados antigos.
    generation += 1
    accountId = normalized
    token.value = ''
    oauthError.value = ''
    stop(true)
    return true
  }

  function setPopup(
    snapshot: MetaAdsConnectionSnapshot,
    nextPopup: MetaAdsOAuthPopupHandle,
  ): boolean {
    if (!isCurrent(snapshot)) {
      nextPopup.close()
      return false
    }
    if (popup && popup !== nextPopup) popup.close()
    popup = nextPopup
    return true
  }

  function getPopup(): MetaAdsOAuthPopupHandle | null {
    return popup
  }

  function setPending(snapshot: MetaAdsConnectionSnapshot, value: boolean): boolean {
    if (!isCurrent(snapshot)) return false
    oauthPending.value = value
    return true
  }

  function setError(snapshot: MetaAdsConnectionSnapshot, value: string): boolean {
    if (!isCurrent(snapshot)) return false
    oauthError.value = value
    return true
  }

  function nextPollAttempt(snapshot: MetaAdsConnectionSnapshot): number | null {
    if (!isCurrent(snapshot)) return null
    pollAttempts += 1
    return pollAttempts
  }

  function schedulePoll(
    snapshot: MetaAdsConnectionSnapshot,
    callback: () => void,
    delayMs = 1500,
  ): boolean {
    if (!isCurrent(snapshot)) return false
    clearPollTimer()
    pollTimer = setTimeout(() => {
      pollTimer = null
      if (isCurrent(snapshot)) callback()
    }, delayMs)
    return true
  }

  function dispose(): void {
    generation += 1
    stop(true)
    token.value = ''
    oauthError.value = ''
  }

  return {
    token,
    oauthPending,
    oauthError,
    capture,
    isCurrent,
    bindAccount,
    setPopup,
    getPopup,
    setPending,
    setError,
    nextPollAttempt,
    schedulePoll,
    stopIfCurrent,
    dispose,
  }
}
