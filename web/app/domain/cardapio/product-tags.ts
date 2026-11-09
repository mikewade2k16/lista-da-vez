export function normalizeProductTags(tags: readonly string[]): string[] {
  const seen = new Set<string>()
  return tags
    .map((tag) => tag.trim())
    .filter((tag) => {
      const key = tag.toLowerCase()
      if (!key || seen.has(key)) return false
      seen.add(key)
      return true
    })
}

export function productTagOptions(products: { tags?: string[] }[]) {
  return normalizeProductTags(products.flatMap((product) => product.tags ?? []))
    .sort((a, b) => a.localeCompare(b, 'pt-BR'))
    .map((tag) => ({ label: tag, value: tag }))
}
