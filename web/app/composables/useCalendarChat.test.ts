import { describe, expect, it } from 'vitest'

import { createAssistantChatRuntime, createAssistantConversationLoadFence } from './useCalendarChat'

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((done) => {
    resolve = done
  })
  return { promise, resolve }
}

describe('assistant conversation load isolation', () => {
  it('descarta o GET tardio de A depois de nova conversa B', async () => {
    const fence = createAssistantConversationLoadFence()
    const requestA = deferred<string>()
    const ticketA = fence.begin()
    let visibleConversation = ''
    const loadA = requestA.promise.then((value) => {
      if (ticketA.isCurrent()) visibleConversation = value
    })

    fence.invalidate()
    const requestB = deferred<string>()
    const ticketB = fence.begin()
    const loadB = requestB.promise.then((value) => {
      if (ticketB.isCurrent()) visibleConversation = value
    })

    requestA.resolve('conversation-a')
    await loadA
    expect(ticketA.signal.aborted).toBe(true)
    expect(visibleConversation).toBe('')

    requestB.resolve('conversation-b')
    await loadB
    expect(ticketB.isCurrent()).toBe(true)
    expect(visibleConversation).toBe('conversation-b')
  })

  it('mantem controladores independentes entre duas instancias Nuxt/SSR', () => {
    const runtimeA = createAssistantChatRuntime()
    const runtimeB = createAssistantChatRuntime()
    const ticketA = runtimeA.conversationLoadFence.begin()
    const ticketB = runtimeB.conversationLoadFence.begin()

    runtimeA.conversationLoadFence.invalidate()

    expect(ticketA.signal.aborted).toBe(true)
    expect(ticketB.signal.aborted).toBe(false)
    expect(ticketB.isCurrent()).toBe(true)
  })
})
