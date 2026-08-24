import { afterEach, describe, expect, it, vi } from 'vitest'

import { useMetaAdsConnectionContext } from './useMetaAdsConnectionContext'

describe('useMetaAdsConnectionContext account isolation', () => {
  afterEach(() => {
    vi.useRealTimers()
  })

  it('clears the manual token and closes OAuth synchronously on A -> B', async () => {
    vi.useFakeTimers()
    const session = useMetaAdsConnectionContext('account-a')
    const accountA = session.capture()
    const popupA = { closed: false, close: vi.fn() }
    const poll = vi.fn()

    session.token.value = 'token-secreto-da-account-a'
    session.setError(accountA, 'erro da account A')
    expect(session.setPopup(accountA, popupA)).toBe(true)
    expect(session.setPending(accountA, true)).toBe(true)
    expect(session.schedulePoll(accountA, poll, 100)).toBe(true)

    expect(session.bindAccount('account-b')).toBe(true)

    expect(session.token.value).toBe('')
    expect(session.oauthError.value).toBe('')
    expect(session.oauthPending.value).toBe(false)
    expect(popupA.close).toHaveBeenCalledOnce()
    expect(session.isCurrent(accountA, 'account-b')).toBe(false)

    await vi.advanceTimersByTimeAsync(100)
    expect(poll).not.toHaveBeenCalled()
  })

  it('does not let a late operation from A clear or stop the state of B', () => {
    const session = useMetaAdsConnectionContext('account-a')
    const accountA = session.capture()
    session.bindAccount('account-b')

    const accountB = session.capture()
    const popupB = { closed: false, close: vi.fn() }
    session.token.value = 'token-da-account-b'
    expect(session.setPopup(accountB, popupB)).toBe(true)
    expect(session.setPending(accountB, true)).toBe(true)

    expect(session.setError(accountA, 'resposta tardia da account A')).toBe(false)
    expect(session.stopIfCurrent(accountA, true)).toBe(false)

    expect(session.token.value).toBe('token-da-account-b')
    expect(session.oauthError.value).toBe('')
    expect(session.oauthPending.value).toBe(true)
    expect(popupB.close).not.toHaveBeenCalled()
  })
})
