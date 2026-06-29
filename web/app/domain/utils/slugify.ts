/**
 * Regra canonica de slugify — identica ao stringsx.Slugify do back (Go).
 *
 * Passos (mesma ordem do Go):
 *  1. Trim + lowercase.
 *  2. NFD: decompoe caracteres acentuados em letra-base + marca de combinacao.
 *  3. Remove marcas de combinacao (Unicode categoria Mn: U+0300–U+036F e
 *     faixas de combining marks do bloco Unicode basico).
 *  4. Troca qualquer caractere fora de [a-z0-9] por hifen.
 *  5. Colapsa hifens repetidos.
 *  6. Remove hifens nas pontas.
 *
 * Exemplos (saida identica ao Go para os mesmos inputs):
 *   "Ação"               → "acao"
 *   "Pérola@RioMar!"     → "perola-riomar"
 *   "  Loja  da Esquina" → "loja-da-esquina"
 *   "já-ok-123"          → "ja-ok-123"
 *   "___"                → ""
 *
 * Mudanca deliberada vs. copias antigas:
 *   - BioCreateModal / AccountCreateModal: nao normalizavam acentos; agora normalizam.
 *   - runtime/shared.ts slugifyLabel: nao normalizava acentos; agora normaliza.
 *   - RoadmapRuleForm / RoadmapModuleForm: ja usavam NFD; agora usam esta funcao.
 *   - domain/cardapio/types.ts slugify: era a mais proxima mas com regex de
 *     acento diferente; substituida por esta.
 */
export function slugify(value: string): string {
  return (
    String(value || '')
      .trim()
      .toLowerCase()
      .normalize('NFD')
      // Remove todas as marcas de combinacao (combining diacritical marks U+0300–U+036F)
      .replace(/\p{M}/gu, '')
      .replace(/[^a-z0-9]+/g, '-')
      .replace(/-+/g, '-')
      .replace(/^-|-$/g, '')
  )
}
