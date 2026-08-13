import { describe, expect, it } from 'vitest'
import { orderMediaItemsByIds, reorderMediaItems } from './utils'

describe('reorderMediaItems', () => {
  it('move um item posterior para a primeira posicao de capa', () => {
    expect(reorderMediaItems(['video', 'imagem', 'nova-capa'], 2, 0)).toEqual([
      'nova-capa',
      'video',
      'imagem',
    ])
  })

  it('move a capa atual para uma posicao posterior', () => {
    expect(reorderMediaItems(['capa', 'segunda', 'terceira'], 0, 2)).toEqual([
      'segunda',
      'terceira',
      'capa',
    ])
  })

  it('preserva a ordem quando os indices sao invalidos', () => {
    expect(reorderMediaItems(['capa', 'segunda'], -1, 1)).toEqual(['capa', 'segunda'])
  })
})

describe('orderMediaItemsByIds', () => {
  const items = [{ id: 'task-a' }, { id: 'calendar:b' }, { id: 'task-c' }]

  it('ordena fontes diferentes e preserva itens novos no final', () => {
    expect(orderMediaItemsByIds(items, ['calendar:b', 'task-a']).map((item) => item.id)).toEqual([
      'calendar:b',
      'task-a',
      'task-c',
    ])
  })

  it('ignora ids removidos e duplicados', () => {
    expect(
      orderMediaItemsByIds(items, ['removido', 'task-c', 'task-c']).map((item) => item.id),
    ).toEqual(['task-c', 'task-a', 'calendar:b'])
  })
})
