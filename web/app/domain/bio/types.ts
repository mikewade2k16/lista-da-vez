// Port do contrato BioData (PLANO_MODULO_BIO.md §4 / API-INTEGRATION.md §3).
// Fonte da verdade do shape e o `types/bio.ts` do front bio; em caso de duvida,
// o type daquele repo vence. Todos os blocos sao opcionais: o backend devolve o
// objeto JA RESOLVIDO (deepMerge defaults + data) no endpoint publico, mas o
// painel edita um draft parcial (dataDraft), por isso tudo aqui e opcional.

export type BioStatus = 'draft' | 'published'

export type BioAlignItems = 'start' | 'center' | 'end'

export interface BioMeta {
  lang?: string
  title?: string
  favicon?: string
  gtmId?: string
}

export interface BioLogo {
  href?: string
  srcMobile: string
  srcDesktop?: string
  widthMobile?: number
  widthDesktop?: number
  borderRadiusClass?: string
  marginBottom?: number
}

export interface BioBranding {
  nameProfile?: string
  nameProfileActive?: boolean
  nameMt?: number
  nameMtMob?: number
  logo?: BioLogo
  footerLogoActive?: boolean
  footerLogoSrc?: string
  footerLogoHref?: string
}

export interface BioLayout {
  alignItems?: BioAlignItems
  animDelayProjection?: number
  slideTopActive?: boolean
  headerMobileActive?: boolean
  bodyTemplateName?: string
}

export interface BioVideoOverlay {
  active?: boolean
  color?: string
  opacity?: number
}

export interface BioVideo {
  bgVideo?: string
  bgVideoPc?: string
  // Fundo por imagem (alternativa ao video). Sem bgVideo, o front usa
  // bgImage/bgImagePc. Pelo menos um (video ou imagem) para publicar.
  bgImage?: string
  bgImagePc?: string
  poster?: string
  overlay?: BioVideoOverlay
}

export type BioMenuAction = 'openStoreLocator'

export type BioIconType = 'sprite' | 'material'

export interface BioIcon {
  type?: BioIconType
  name?: string
}

export interface BioDropdownItem {
  label?: string
  href?: string
}

export interface BioMenuItem {
  label: string
  href?: string
  action?: BioMenuAction
  icon?: BioIcon
  dropdown?: BioDropdownItem[]
}

export interface BioLink {
  label: string
  href?: string
  action?: string
  icon?: BioIcon
}

export interface BioSlide {
  src: string
  title?: string
  desc?: string
  price?: string
  // Link do slide manual (numero WhatsApp ja em formato de URL/href).
  whatsapp?: string
  // Link final resolvido de um slide-produto (fonte B7): produto no site ou
  // wa.me, conforme o source.link. O front usa `href || whatsapp`.
  href?: string
}

export interface BioCarouselBreakpoint {
  max?: number
  perView?: number
}

export interface BioCarousel {
  loop?: boolean
  autoplay?: boolean
  autoplayMs?: number
  pauseOnHover?: boolean
  perView?: number
  spacing?: number
  nav?: boolean
  breakpoints?: BioCarouselBreakpoint[]
  limit?: number
}

// Origem do conteudo dos slides (B7). `manual` = slides a mao (como sempre);
// `site_products` = produtos resolvidos no backend a partir de site.products.
// Tudo opcional/retrocompativel: bio sem `source` mantem o comportamento manual.
export type BioSlideSourceType = 'manual' | 'site_products'

// Para onde o clique de um slide-produto leva. `product` = link do produto no
// site do cliente; `whatsapp` = numero da bio/lightbox; `none` = sem link.
export type BioSlideLinkMode = 'product' | 'whatsapp' | 'none'

export interface BioSlideSource {
  type?: BioSlideSourceType
  category?: string
  campaigns?: string[]
  tipo?: string
  limit?: number
  link?: BioSlideLinkMode
}

// Modo de exibicao dos slides. `carousel` = carrossel keen; `static` = imagens
// estaticas empilhadas (sem auto-rotacao).
export type BioSlideMode = 'carousel' | 'static'

// Botao opcional abaixo do carrossel (ex.: "Ver toda a colecao").
export interface BioSlideButton {
  text?: string
  href?: string
}

export interface BioSlideTop {
  active?: boolean
  engine?: 'keen'
  slides?: BioSlide[]
  carousel?: BioCarousel
  source?: BioSlideSource
  mode?: BioSlideMode
  button?: BioSlideButton
}

// Facets distintos de site.products da account (GET .../facets) — popula os
// selects de categoria/campanha/tipo no editor de fonte de produtos.
export interface BioSlideSourceFacets {
  categories: string[]
  campaigns: string[]
  tipos: string[]
}

// Fonte disponivel para a account (GET /v1/bio/sources). MVP: site_products.
export interface BioSlideSourceOption {
  type: BioSlideSourceType
  label: string
  available: boolean
}

export interface BioPlacesBounds {
  south?: number
  west?: number
  north?: number
  east?: number
}

export interface BioStore {
  name: string
  lat: number
  lng: number
  placeId?: string
  img?: string
  phone?: string
  address?: string
}

export interface BioStoreLocator {
  openOnQuery?: boolean
  placesBounds?: BioPlacesBounds
  stores?: BioStore[]
}

export interface BioLightbox {
  whatsappNumber?: string
}

// Conteudo profundo de uma bio (data_draft / data_published / defaults / preview).
export interface BioData {
  meta?: BioMeta
  branding?: BioBranding
  layout?: BioLayout
  video?: BioVideo
  headerMenu?: BioMenuItem[]
  links?: BioLink[]
  slideTop?: BioSlideTop
  storeLocator?: BioStoreLocator
  lightbox?: BioLightbox
}

// Projecao lean da listagem (GET /v1/bio/bios) — sem o jsonb pesado.
export interface BioSummary {
  id: string
  accountId: string
  accountName?: string
  slug: string
  name: string
  status: BioStatus
  updatedAt: string
  publishedAt?: string | null
}

// Detalhe de uma bio (GET /v1/bio/bios/{id}).
export interface BioDetail {
  id: string
  accountId: string
  slug: string
  name: string
  status: BioStatus
  dataDraft: BioData
  dataPublished?: BioData | null
  publishedAt?: string | null
  updatedAt: string
}

// Defaults globais (GET/PUT /v1/bio/defaults — so platform_admin).
export interface BioDefaults {
  data: BioData
  updatedAt?: string
}

export interface BioListFilters {
  accountId?: string
  status?: BioStatus | ''
  q?: string
}

export interface BioCreatePayload {
  accountId?: string
  slug: string
  name: string
}

export interface BioPatchPayload {
  name?: string
  slug?: string
  dataDraft?: BioData
}

// kind aceito pelo upload de midia (POST /v1/bio/bios/{id}/media).
export type BioMediaKind =
  | 'video'
  | 'background'
  | 'poster'
  | 'logo'
  | 'favicon'
  | 'slide'
  | 'store'

export interface BioMediaUploadResult {
  url: string
}
