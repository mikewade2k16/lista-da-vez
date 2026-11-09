import { afterEach, describe, expect, it, vi } from 'vitest'
import { createStudioSnapshotRequest } from './studio-draft'
import type { SiteLayout } from './types'

const updated: SiteLayout = {
  pages: {
    home: {
      page: 'home',
      blocks: [
        {
          id: 'hero',
          type: 'hero.cinematografico',
          visible: true,
          props: { titleEm: 'Bari', titleTail: '' },
        },
      ],
    },
  },
}

describe('confirmacao do documento do Studio', () => {
  afterEach(() => vi.useRealTimers())

  it('aguarda resposta explicita, sem salvar uma copia antiga depois de 250ms', async () => {
    vi.useFakeTimers()
    const post = vi.fn()
    const client = createStudioSnapshotRequest(post)
    const received = vi.fn()
    const pending = client.request().then(received)
    await vi.advanceTimersByTimeAsync(250)
    expect(received).not.toHaveBeenCalled()
    client.accept('id-incorreto', updated)
    expect(received).not.toHaveBeenCalled()
    const id = post.mock.calls[0][0].requestId
    client.accept(id, updated)
    await pending
    expect(received.mock.calls[0][0].pages.home.blocks[0].props).toEqual({
      titleEm: 'Bari',
      titleTail: '',
    })
    expect(received.mock.calls[0][0]).not.toBe(updated)
  })

  it('falha sem resposta do Studio antigo em vez de anunciar publicacao', async () => {
    vi.useFakeTimers()
    const client = createStudioSnapshotRequest(vi.fn())
    const outcome = expect(client.request()).rejects.toThrow('Nada foi publicado')
    await vi.advanceTimersByTimeAsync(5000)
    await outcome
  })

  it('cancela ao sair do editor e ignora respostas atrasadas', async () => {
    const post = vi.fn()
    const client = createStudioSnapshotRequest(post)
    const outcome = expect(client.request()).rejects.toThrow('editor foi fechado')
    client.cancel()
    client.accept(post.mock.calls[0][0].requestId, updated)
    await outcome
  })
})
