import { computed, ref, watch } from 'vue'
import { defineStore } from 'pinia'

import { useAuthStore } from '~/stores/auth'

export interface SiteTenantOption {
  id: string
  label: string
}

export interface SiteProduct {
  id: string
  tenantId: string
  tenantName: string
  name: string
  code: string
  categories: string[]
  campaigns: string[]
  imageUrl: string
  available: boolean
  visible: boolean
  price: number
  factor: number
  type: string
  description: string
  source: 'webhook' | 'manual'
  webhookStatus: 'healthy' | 'pending' | 'error'
  webhookEndpoint: string
  lastWebhookSync: string
  updatedAt: string
  createdAt: string
  deletedAt: string
  isDeleted: boolean
}

export interface SiteLead {
  id: string
  tenantId: string
  tenantName: string
  name: string
  email: string
  phone: string
  source: string
  page: string
  coupon: string
  consentLabel: string
  createdAt: string
  trackingData: string
  payloadJson: string
}

type SiteProductPatch = Partial<
  Pick<
    SiteProduct,
    | 'tenantId'
    | 'tenantName'
    | 'name'
    | 'code'
    | 'categories'
    | 'campaigns'
    | 'imageUrl'
    | 'available'
    | 'visible'
    | 'price'
    | 'factor'
    | 'type'
    | 'description'
    | 'source'
    | 'webhookStatus'
    | 'webhookEndpoint'
    | 'lastWebhookSync'
    | 'isDeleted'
    | 'deletedAt'
  >
>

let productSequence = 0
let leadSequence = 0

function normalizeText(value: unknown, max = 255) {
  return String(value ?? '')
    .replace(/\s+/g, ' ')
    .trim()
    .slice(0, max)
}

function normalizeSlugCode(value: unknown) {
  return normalizeText(value, 80)
    .toLowerCase()
    .replace(/[^a-z0-9-_/]+/g, '-')
    .replace(/-+/g, '-')
    .replace(/^-|-$/g, '')
}

function normalizeList(value: unknown) {
  if (Array.isArray(value)) {
    return value
      .map((item) => normalizeText(item, 80))
      .filter(Boolean)
      .slice(0, 12)
  }

  return String(value ?? '')
    .split(/[\n,;|]+/)
    .map((item) => normalizeText(item, 80))
    .filter(Boolean)
    .slice(0, 12)
}

function normalizeNumber(value: unknown, fallback = 0) {
  const parsed = Number(String(value ?? '').replace(',', '.'))
  if (!Number.isFinite(parsed)) {
    return fallback
  }

  return Math.max(0, Number(parsed.toFixed(2)))
}

function formatIsoDate(value: Date | string) {
  const date = value instanceof Date ? value : new Date(String(value || ''))
  if (Number.isNaN(date.getTime())) {
    return new Date().toISOString()
  }

  return date.toISOString()
}

function formatWebhookEndpoint(tenantId: string) {
  const suffix = normalizeSlugCode(tenantId).slice(-8) || 'tenant'
  return `https://hooks.fila.local/site/products/${suffix}`
}

function buildSeedProducts(tenant: SiteTenantOption): SiteProduct[] {
  const now = new Date()

  return [
    {
      id: `site-product-${++productSequence}`,
      tenantId: tenant.id,
      tenantName: tenant.label,
      name: 'Sofa modular Viena',
      code: `site-${normalizeSlugCode(tenant.label)}-viena`,
      categories: ['Sala', 'Lancamento'],
      campaigns: ['Catalogo principal', 'Semana premium'],
      imageUrl: '',
      available: true,
      visible: true,
      price: 4899.9,
      factor: 1.2,
      type: 'Sofa',
      description: 'Configuracao em L com tecido hidrorepelente e modulos expansivos.',
      source: 'webhook',
      webhookStatus: 'healthy',
      webhookEndpoint: formatWebhookEndpoint(tenant.id),
      lastWebhookSync: formatIsoDate(now),
      updatedAt: formatIsoDate(now),
      createdAt: formatIsoDate(now),
      deletedAt: '',
      isDeleted: false,
    },
    {
      id: `site-product-${++productSequence}`,
      tenantId: tenant.id,
      tenantName: tenant.label,
      name: 'Mesa lateral Aura',
      code: `site-${normalizeSlugCode(tenant.label)}-aura`,
      categories: ['Decoracao'],
      campaigns: ['Outlet digital'],
      imageUrl: '',
      available: true,
      visible: false,
      price: 799.5,
      factor: 1,
      type: 'Mesa',
      description: 'Pequeno apoio para living com tampo mineral e base champanhe.',
      source: 'manual',
      webhookStatus: 'pending',
      webhookEndpoint: formatWebhookEndpoint(tenant.id),
      lastWebhookSync: formatIsoDate(new Date(now.getTime() - 1000 * 60 * 90)),
      updatedAt: formatIsoDate(now),
      createdAt: formatIsoDate(new Date(now.getTime() - 1000 * 60 * 60 * 24)),
      deletedAt: '',
      isDeleted: false,
    },
  ]
}

function buildSeedLeads(tenant: SiteTenantOption): SiteLead[] {
  const now = new Date()

  return [
    {
      id: `site-lead-${++leadSequence}`,
      tenantId: tenant.id,
      tenantName: tenant.label,
      name: 'Ana Martins',
      email: `ana+${normalizeSlugCode(tenant.label)}@example.com`,
      phone: '79999990001',
      source: 'Formulario hero',
      page: '/site/produtos',
      coupon: 'SITE10',
      consentLabel: 'Aceitou contato comercial',
      createdAt: formatIsoDate(new Date(now.getTime() - 1000 * 60 * 30)),
      trackingData: JSON.stringify({ utm_source: 'instagram', utm_campaign: 'catalogo-maio' }),
      payloadJson: JSON.stringify({
        interest: 'Sofa modular Viena',
        preferredStore: tenant.label,
      }),
    },
    {
      id: `site-lead-${++leadSequence}`,
      tenantId: tenant.id,
      tenantName: tenant.label,
      name: 'Carlos Lima',
      email: `carlos+${normalizeSlugCode(tenant.label)}@example.com`,
      phone: '79999990002',
      source: 'Popup de saida',
      page: '/landing/colecao-premium',
      coupon: '',
      consentLabel: 'Aceitou novidades e remarketing',
      createdAt: formatIsoDate(new Date(now.getTime() - 1000 * 60 * 60 * 8)),
      trackingData: JSON.stringify({ utm_source: 'google', utm_medium: 'cpc' }),
      payloadJson: JSON.stringify({
        interest: 'Mesa lateral Aura',
        notes: 'Pediu retorno por WhatsApp',
      }),
    },
  ]
}

export const useSiteStore = defineStore('site', () => {
  const auth = useAuthStore()

  const products = ref<SiteProduct[]>([])
  const leads = ref<SiteLead[]>([])
  const ready = ref(false)

  const isPlatformAdmin = computed(() => normalizeText(auth.role) === 'platform_admin')
  const tenantOptions = computed<SiteTenantOption[]>(() => {
    const fromContext = Array.isArray(auth.tenantContext)
      ? auth.tenantContext
          .map((tenant) => ({
            id: normalizeText(tenant?.id),
            label: normalizeText(tenant?.name || tenant?.slug || 'Cliente'),
          }))
          .filter((tenant) => tenant.id)
      : []

    if (fromContext.length > 0) {
      return fromContext
    }

    const fallbackTenantId = normalizeText(auth.activeTenantId)
    return fallbackTenantId ? [{ id: fallbackTenantId, label: 'Cliente ativo' }] : []
  })

  const currentTenantId = computed(() =>
    normalizeText(auth.activeTenantId || tenantOptions.value[0]?.id || ''),
  )

  const scopedProducts = computed(() => {
    if (isPlatformAdmin.value) {
      return products.value
    }

    return products.value.filter((product) => product.tenantId === currentTenantId.value)
  })

  const scopedLeads = computed(() => {
    if (isPlatformAdmin.value) {
      return leads.value
    }

    return leads.value.filter((lead) => lead.tenantId === currentTenantId.value)
  })

  function patchTenantLabels() {
    const labelMap = new Map(tenantOptions.value.map((tenant) => [tenant.id, tenant.label]))
    products.value = products.value.map((product) => ({
      ...product,
      tenantName: labelMap.get(product.tenantId) || product.tenantName,
    }))
    leads.value = leads.value.map((lead) => ({
      ...lead,
      tenantName: labelMap.get(lead.tenantId) || lead.tenantName,
    }))
  }

  function ensureSeedData() {
    const tenantList = tenantOptions.value.length
      ? tenantOptions.value
      : currentTenantId.value
        ? [{ id: currentTenantId.value, label: 'Cliente ativo' }]
        : []

    if (!tenantList.length) {
      return
    }

    for (const tenant of tenantList) {
      if (!products.value.some((product) => product.tenantId === tenant.id)) {
        products.value = [...products.value, ...buildSeedProducts(tenant)]
      }

      if (!leads.value.some((lead) => lead.tenantId === tenant.id)) {
        leads.value = [...leads.value, ...buildSeedLeads(tenant)]
      }
    }

    patchTenantLabels()
    ready.value = true
  }

  function canMutateTenant(tenantId: string) {
    const normalizedTenantId = normalizeText(tenantId)
    if (!normalizedTenantId) {
      return false
    }

    return isPlatformAdmin.value || normalizedTenantId === currentTenantId.value
  }

  function createProduct(payload: Partial<SiteProduct> = {}) {
    const tenantId = normalizeText(
      payload.tenantId || currentTenantId.value || tenantOptions.value[0]?.id,
    )
    const tenantName =
      tenantOptions.value.find((tenant) => tenant.id === tenantId)?.label ||
      normalizeText(payload.tenantName || 'Cliente')

    if (!canMutateTenant(tenantId)) {
      return { ok: false, message: 'Escopo do cliente invalido para criar produto.' }
    }

    const now = formatIsoDate(new Date())
    const product: SiteProduct = {
      id: `site-product-${++productSequence}`,
      tenantId,
      tenantName,
      name: normalizeText(payload.name || 'Novo produto', 100),
      code: normalizeSlugCode(payload.code || `site-${productSequence}`),
      categories: normalizeList(payload.categories),
      campaigns: normalizeList(payload.campaigns),
      imageUrl: normalizeText(payload.imageUrl, 400),
      available: Boolean(payload.available ?? true),
      visible: Boolean(payload.visible ?? true),
      price: normalizeNumber(payload.price, 0),
      factor: normalizeNumber(payload.factor, 1),
      type: normalizeText(payload.type || 'Produto', 60),
      description: normalizeText(payload.description, 4000),
      source: payload.source === 'manual' ? 'manual' : 'webhook',
      webhookStatus:
        payload.webhookStatus === 'error'
          ? 'error'
          : payload.webhookStatus === 'pending'
            ? 'pending'
            : 'healthy',
      webhookEndpoint: normalizeText(
        payload.webhookEndpoint || formatWebhookEndpoint(tenantId),
        255,
      ),
      lastWebhookSync: formatIsoDate(payload.lastWebhookSync || now),
      updatedAt: now,
      createdAt: now,
      deletedAt: '',
      isDeleted: false,
    }

    products.value = [product, ...products.value]
    return { ok: true, product }
  }

  function updateProduct(productId: string, patch: SiteProductPatch) {
    const normalizedId = normalizeText(productId)
    const index = products.value.findIndex((product) => product.id === normalizedId)
    if (index < 0) {
      return { ok: false, message: 'Produto nao encontrado.' }
    }

    const current = products.value[index]
    if (!canMutateTenant(current.tenantId)) {
      return { ok: false, message: 'Voce nao pode alterar produtos desse cliente.' }
    }

    const nextTenantId = normalizeText(patch.tenantId || current.tenantId)
    const nextTenantName =
      tenantOptions.value.find((tenant) => tenant.id === nextTenantId)?.label || current.tenantName
    const nextProduct: SiteProduct = {
      ...current,
      tenantId: nextTenantId,
      tenantName: nextTenantName,
      name: patch.name !== undefined ? normalizeText(patch.name, 100) : current.name,
      code: patch.code !== undefined ? normalizeSlugCode(patch.code) : current.code,
      categories:
        patch.categories !== undefined ? normalizeList(patch.categories) : current.categories,
      campaigns: patch.campaigns !== undefined ? normalizeList(patch.campaigns) : current.campaigns,
      imageUrl:
        patch.imageUrl !== undefined ? normalizeText(patch.imageUrl, 400) : current.imageUrl,
      available: patch.available !== undefined ? Boolean(patch.available) : current.available,
      visible: patch.visible !== undefined ? Boolean(patch.visible) : current.visible,
      price:
        patch.price !== undefined ? normalizeNumber(patch.price, current.price) : current.price,
      factor:
        patch.factor !== undefined ? normalizeNumber(patch.factor, current.factor) : current.factor,
      type: patch.type !== undefined ? normalizeText(patch.type, 60) : current.type,
      description:
        patch.description !== undefined
          ? normalizeText(patch.description, 4000)
          : current.description,
      source:
        patch.source === 'manual' || patch.source === 'webhook' ? patch.source : current.source,
      webhookStatus:
        patch.webhookStatus === 'error' ||
        patch.webhookStatus === 'pending' ||
        patch.webhookStatus === 'healthy'
          ? patch.webhookStatus
          : current.webhookStatus,
      webhookEndpoint:
        patch.webhookEndpoint !== undefined
          ? normalizeText(patch.webhookEndpoint, 255)
          : current.webhookEndpoint,
      lastWebhookSync:
        patch.lastWebhookSync !== undefined
          ? formatIsoDate(patch.lastWebhookSync)
          : current.lastWebhookSync,
      isDeleted: patch.isDeleted !== undefined ? Boolean(patch.isDeleted) : current.isDeleted,
      deletedAt:
        patch.deletedAt !== undefined
          ? normalizeText(patch.deletedAt, 80)
          : patch.isDeleted === true
            ? formatIsoDate(new Date())
            : patch.isDeleted === false
              ? ''
              : current.deletedAt,
      updatedAt: formatIsoDate(new Date()),
    }

    products.value.splice(index, 1, nextProduct)
    return { ok: true, product: nextProduct }
  }

  function archiveProduct(productId: string) {
    return updateProduct(productId, { isDeleted: true, deletedAt: formatIsoDate(new Date()) })
  }

  function restoreProduct(productId: string) {
    return updateProduct(productId, { isDeleted: false, deletedAt: '' })
  }

  function removeLead(leadId: string) {
    const normalizedId = normalizeText(leadId)
    const lead = leads.value.find((item) => item.id === normalizedId)
    if (!lead) {
      return { ok: false, message: 'Lead nao encontrado.' }
    }

    if (!canMutateTenant(lead.tenantId)) {
      return { ok: false, message: 'Voce nao pode excluir leads desse cliente.' }
    }

    leads.value = leads.value.filter((item) => item.id !== normalizedId)
    return { ok: true }
  }

  watch(
    () => [
      auth.activeTenantId,
      auth.role,
      tenantOptions.value.map((tenant) => tenant.id).join('|'),
    ],
    () => {
      ensureSeedData()
    },
    { immediate: true },
  )

  return {
    products,
    leads,
    ready,
    isPlatformAdmin,
    tenantOptions,
    currentTenantId,
    scopedProducts,
    scopedLeads,
    createProduct,
    updateProduct,
    archiveProduct,
    restoreProduct,
    removeLead,
    ensureSeedData,
  }
})
