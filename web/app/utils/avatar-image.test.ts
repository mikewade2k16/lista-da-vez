import { describe, expect, it } from 'vitest'

import { AVATAR_SIZE, avatarCrop } from './avatar-image'

describe('avatarCrop', () => {
  it('recorta o centro de uma foto paisagem e reduz para o tamanho do avatar', () => {
    expect(avatarCrop(4000, 3000)).toEqual({ sx: 500, sy: 0, side: 3000, size: AVATAR_SIZE })
  })

  it('recorta o centro de uma foto retrato', () => {
    expect(avatarCrop(1080, 1920)).toEqual({ sx: 0, sy: 420, side: 1080, size: AVATAR_SIZE })
  })

  it('nao amplia foto menor que o avatar', () => {
    expect(avatarCrop(300, 400)).toEqual({ sx: 0, sy: 50, side: 300, size: 300 })
  })
})
