import { ref } from 'vue'

import { slugify } from '~/domain/cardapio/types'
import type { Product } from '~/domain/cardapio/types'
import type { VariationDraft } from '~/components/cardapio/product/CardapioProductVariations.vue'
import type { AddonDraft } from '~/components/cardapio/product/CardapioProductAddons.vue'

// Rascunho editavel de produto. Concentra a conversao Product <-> form e a
// montagem do payload PATCH (variations/addons replace-all), mantendo o modal
// como orquestrador fino de UI.
export interface ProductForm {
  name: string
  slug: string
  slugTouched: boolean
  categoryId: string
  shortDesc: string
  description: string
  body: string
  priceCents: number
  imageUrl: string
  gallery: string[]
  weight: string
  cookTime: string
  diet: string
  allergens: string
  tags: string
  isAvailable: boolean
  isFeatured: boolean
  sortOrder: number
  variations: VariationDraft[]
  addons: AddonDraft[]
}

function listToText(values: string[]): string {
  return (Array.isArray(values) ? values : []).join(', ')
}

function textToList(value: string): string[] {
  return String(value || '')
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)
}

export function emptyProductForm(): ProductForm {
  return {
    name: '',
    slug: '',
    slugTouched: false,
    categoryId: '',
    shortDesc: '',
    description: '',
    body: '',
    priceCents: 0,
    imageUrl: '',
    gallery: [],
    weight: '',
    cookTime: '',
    diet: '',
    allergens: '',
    tags: '',
    isAvailable: true,
    isFeatured: false,
    sortOrder: 0,
    variations: [],
    addons: [],
  }
}

export function productToForm(product: Product): ProductForm {
  return {
    name: product.name,
    slug: product.slug,
    slugTouched: true,
    categoryId: product.categoryId ?? '',
    shortDesc: product.shortDesc,
    description: product.description,
    body: product.body,
    priceCents: product.priceCents,
    imageUrl: product.imageUrl,
    gallery: [...(product.gallery ?? [])],
    weight: product.weight,
    cookTime: product.cookTime,
    diet: listToText(product.diet),
    allergens: listToText(product.allergens),
    tags: listToText(product.tags),
    isAvailable: product.isAvailable,
    isFeatured: product.isFeatured,
    sortOrder: product.sortOrder,
    variations: (product.variations ?? []).map((variation) => ({
      id: variation.id,
      name: variation.name,
      priceDeltaCents: variation.priceDeltaCents,
    })),
    addons: (product.addons ?? []).map((addon) => ({
      id: addon.id,
      name: addon.name,
      priceCents: addon.priceCents,
    })),
  }
}

export function formToPayload(form: ProductForm): Record<string, unknown> {
  return {
    name: form.name.trim(),
    slug: form.slug.trim() || slugify(form.name),
    categoryId: form.categoryId || null,
    shortDesc: form.shortDesc.trim(),
    description: form.description,
    body: form.body,
    priceCents: form.priceCents,
    imageUrl: form.imageUrl.trim(),
    gallery: form.gallery,
    weight: form.weight.trim(),
    cookTime: form.cookTime.trim(),
    diet: textToList(form.diet),
    allergens: textToList(form.allergens),
    tags: textToList(form.tags),
    isAvailable: form.isAvailable,
    isFeatured: form.isFeatured,
    sortOrder: Math.trunc(Number(form.sortOrder) || 0),
    variations: form.variations
      .filter((variation) => variation.name.trim())
      .map((variation, index) => ({
        id: variation.id || undefined,
        name: variation.name.trim(),
        priceDeltaCents: Math.trunc(Number(variation.priceDeltaCents) || 0),
        sortOrder: index,
      })),
    addons: form.addons
      .filter((addon) => addon.name.trim())
      .map((addon, index) => ({
        id: addon.id || undefined,
        name: addon.name.trim(),
        priceCents: Math.max(0, Math.trunc(Number(addon.priceCents) || 0)),
        sortOrder: index,
      })),
  }
}

export function useCardapioProductForm() {
  const form = ref<ProductForm>(emptyProductForm())

  function loadEmpty() {
    form.value = emptyProductForm()
  }

  function loadProduct(product: Product) {
    form.value = productToForm(product)
  }

  return { form, loadEmpty, loadProduct }
}
