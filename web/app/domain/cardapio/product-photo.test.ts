import { describe, expect, it } from 'vitest'

import { PRODUCT_PHOTO_MAX_BYTES, productPhotoError } from './product-photo'

describe('productPhotoError', () => {
  it('aceita imagens dos formatos permitidos ate 20MB', () => {
    expect(productPhotoError({ type: 'image/jpeg', size: 1024 })).toBe('')
    expect(productPhotoError({ type: 'image/webp', size: PRODUCT_PHOTO_MAX_BYTES })).toBe('')
  })

  it('recusa formato que nao e imagem permitida', () => {
    expect(productPhotoError({ type: 'image/heic', size: 1024 })).toMatch(/JPG/)
    expect(productPhotoError({ type: 'application/pdf', size: 1024 })).toMatch(/JPG/)
  })

  it('recusa foto acima de 20MB', () => {
    expect(productPhotoError({ type: 'image/png', size: PRODUCT_PHOTO_MAX_BYTES + 1 })).toMatch(
      /20MB/,
    )
  })
})
