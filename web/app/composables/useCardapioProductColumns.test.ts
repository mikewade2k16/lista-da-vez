import { beforeEach, describe, expect, it, vi } from 'vitest'
import { emptyProductForm } from './useCardapioProductForm'
import { useCardapioProductColumns } from './useCardapioProductColumns'
import type { Product } from '~/domain/cardapio/types'

const store = vi.hoisted(() => ({
  loadProduct: vi.fn(),
  uploadProductPhoto: vi.fn(),
  patchProduct: vi.fn(),
}))
vi.mock('~/stores/cardapio', () => ({ useCardapioStore: () => store }))
vi.mock('vue', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue')>()),
  onBeforeUnmount: vi.fn(),
}))

const product: Product = {
  ...emptyProductForm(),
  id: 'product-1',
  restaurantId: 'restaurant-9',
  categoryId: 'category-1',
  name: 'Produto',
  slug: 'produto',
  diet: ['vegano'],
  allergens: ['soja'],
  tags: ['Novo'],
  pairing: { name: 'Suco', type: 'bebida', priceCents: 1200 },
  rating: null,
  reviewCount: 0,
  soldCount: 0,
  createdAt: '2026-11-03T12:00:00Z',
  updatedAt: '2026-11-03T12:00:00Z',
}

describe('upload da imagem na tabela do cardapio', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    store.loadProduct.mockResolvedValue(product)
    store.uploadProductPhoto.mockResolvedValue({
      url: '/uploads/cardapio/account-1/foto.webp',
      thumbUrl: '',
    })
    store.patchProduct.mockResolvedValue({
      ...product,
      imageUrl: '/uploads/cardapio/account-1/foto.webp',
    })
  })

  it.each([
    { tags: [' Novo ', 'Picante', 'novo', ''], expected: ['Novo', 'Picante'] },
    { tags: [], expected: [] },
  ])(
    'salva a selecao de tags $expected sem alterar os demais campos',
    async ({ tags, expected }) => {
      useCardapioProductColumns().onCellInput(product.id, 'tags', tags, true)
      await vi.waitFor(() =>
        expect(store.patchProduct).toHaveBeenCalledWith(
          product.id,
          expect.objectContaining({
            tags: expected,
            name: product.name,
            diet: product.diet,
            pairing: product.pairing,
          }),
        ),
      )
    },
  )

  it.each(['webp', 'png', 'jpeg', 'gif'])(
    'envia %s para o restaurante e grava no produto correto',
    async (format) => {
      const file = new File(['imagem'], `foto.${format}`, { type: `image/${format}` })
      await useCardapioProductColumns().applyImageUpload(product.id, file)
      expect(store.loadProduct).toHaveBeenCalledWith(product.id)
      expect(store.uploadProductPhoto).toHaveBeenCalledWith(product.restaurantId, file)
      expect(store.patchProduct).toHaveBeenCalledWith(
        product.id,
        expect.objectContaining({
          imageUrl: '/uploads/cardapio/account-1/foto.webp',
          categoryId: product.categoryId,
          diet: product.diet,
          allergens: product.allergens,
          tags: product.tags,
          pairing: product.pairing,
        }),
      )
    },
  )

  it('nao envia arquivo quando o produto nao existe ou esta fora do escopo', async () => {
    store.loadProduct.mockRejectedValue(new Error('Recurso nao encontrado'))
    await expect(
      useCardapioProductColumns().applyImageUpload(product.id, new File([], 'foto.png')),
    ).rejects.toThrow('Recurso nao encontrado')
    expect(store.uploadProductPhoto).not.toHaveBeenCalled()
    expect(store.patchProduct).not.toHaveBeenCalled()
  })

  it('nao altera a imagem gravada quando o upload falha', async () => {
    store.uploadProductPhoto.mockRejectedValue(new Error('Falha no envio'))
    await expect(
      useCardapioProductColumns().applyImageUpload(product.id, new File([], 'foto.png')),
    ).rejects.toThrow('Falha no envio')
    expect(store.patchProduct).not.toHaveBeenCalled()
  })

  it('nao apaga a imagem anterior se a resposta do upload vier sem URL', async () => {
    store.uploadProductPhoto.mockResolvedValue({ url: '', thumbUrl: '' })
    await expect(
      useCardapioProductColumns().applyImageUpload(product.id, new File([], 'foto.png')),
    ).rejects.toThrow('O servidor nao retornou')
    expect(store.patchProduct).not.toHaveBeenCalled()
  })
})
