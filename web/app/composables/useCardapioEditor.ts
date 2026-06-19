import { computed, ref, watch } from 'vue'

import { useCardapioStore } from '~/stores/cardapio'
import { useUiStore } from '~/stores/ui'
import { getApiErrorMessage } from '~/utils/api-client'
import { normalizePayment } from '~/domain/cardapio/types'
import type {
  Restaurant,
  RestaurantAddress,
  RestaurantHour,
  RestaurantPayment,
  RestaurantSettings,
} from '~/domain/cardapio/types'

// Estado/salvamento da secao "Dados" do editor de restaurante. As demais secoes
// (categorias, produtos, avaliacoes, pedidos, dominios, entrega, aparencia) sao
// CRUD direto na store — so a secao de dados tem um form extenso com edicao
// livre, entao o dirty-check vive aqui. O TEMA nao e mais salvo aqui: a secao
// "Aparencia" e a dona do theme (PATCH parcial {theme}); o saveDados nao envia
// theme para nao sobrescrever o que a Aparencia salvou.

function cloneAddress(address?: RestaurantAddress): RestaurantAddress {
  return {
    street: String(address?.street ?? ''),
    neighborhood: String(address?.neighborhood ?? ''),
    city: String(address?.city ?? ''),
    state: String(address?.state ?? ''),
    zip: String(address?.zip ?? ''),
    number: String(address?.number ?? ''),
    complement: String(address?.complement ?? ''),
    reference: String(address?.reference ?? ''),
  }
}

function clonePayment(payment?: RestaurantPayment): RestaurantPayment {
  return normalizePayment(payment)
}

function cloneSettings(settings?: RestaurantSettings): RestaurantSettings {
  return {
    deliveryFeeCents: Number(settings?.deliveryFeeCents ?? 0),
    deliveryEnabled: Boolean(settings?.deliveryEnabled),
    pickupEnabled: Boolean(settings?.pickupEnabled),
    dineInEnabled: Boolean(settings?.dineInEnabled),
    minOrderCents: Number(settings?.minOrderCents ?? 0),
    freeDeliveryAboveCents: Number(settings?.freeDeliveryAboveCents ?? 0),
    payment: clonePayment(settings?.payment),
  }
}

function cloneHours(hours?: RestaurantHour[]): RestaurantHour[] {
  return (Array.isArray(hours) ? hours : []).map((hour) => ({
    days: String(hour?.days ?? ''),
    hours: String(hour?.hours ?? ''),
  }))
}

export interface DadosForm {
  name: string
  slug: string
  tagline: string
  description: string
  segment: string
  logoUrl: string
  bannerUrl: string
  whatsapp: string
  phone: string
  email: string
  instagram: string
  facebook: string
  youtube: string
  googleAnalyticsId: string
  facebookPixelId: string
  customHeadHtml: string
  address: RestaurantAddress
  hours: RestaurantHour[]
  settings: RestaurantSettings
}

function buildForm(source: Restaurant | null): DadosForm {
  return {
    name: String(source?.name ?? ''),
    slug: String(source?.slug ?? ''),
    tagline: String(source?.tagline ?? ''),
    description: String(source?.description ?? ''),
    segment: String(source?.segment ?? ''),
    logoUrl: String(source?.logoUrl ?? ''),
    bannerUrl: String(source?.bannerUrl ?? ''),
    whatsapp: String(source?.whatsapp ?? ''),
    phone: String(source?.phone ?? ''),
    email: String(source?.email ?? ''),
    instagram: String(source?.instagram ?? ''),
    facebook: String(source?.facebook ?? ''),
    youtube: String(source?.youtube ?? ''),
    googleAnalyticsId: String(source?.googleAnalyticsId ?? ''),
    facebookPixelId: String(source?.facebookPixelId ?? ''),
    customHeadHtml: String(source?.customHeadHtml ?? ''),
    address: cloneAddress(source?.address),
    hours: cloneHours(source?.hours),
    settings: cloneSettings(source?.settings),
  }
}

export function useCardapioEditor() {
  const store = useCardapioStore()
  const ui = useUiStore()

  const form = ref<DadosForm>(buildForm(null))
  const baseline = ref<string>(JSON.stringify(form.value))
  const savingDados = ref(false)
  const uploading = ref(false)

  const dirty = computed(() => JSON.stringify(form.value) !== baseline.value)

  function syncFromStore() {
    form.value = buildForm(store.restaurant)
    baseline.value = JSON.stringify(form.value)
  }

  // Reconstroi o formulario quando o restaurante ativo muda (load ou patch).
  watch(
    () => store.restaurant,
    () => {
      syncFromStore()
    },
    { immediate: true },
  )

  async function saveDados() {
    if (savingDados.value || !store.restaurantId) {
      return { ok: false }
    }

    savingDados.value = true
    try {
      // slug NAO entra no PATCH: e imutavel no back (UpdateRestaurantInput nao
      // tem o campo) e o ReadJSON com DisallowUnknownFields rejeitaria o body.
      // theme tambem NAO entra: a secao Aparencia e a dona (evita sobrescrever).
      await store.patchRestaurant(store.restaurantId, {
        name: form.value.name.trim(),
        tagline: form.value.tagline.trim(),
        description: form.value.description,
        segment: form.value.segment.trim(),
        logoUrl: form.value.logoUrl.trim(),
        bannerUrl: form.value.bannerUrl.trim(),
        whatsapp: form.value.whatsapp.trim(),
        phone: form.value.phone.trim(),
        email: form.value.email.trim(),
        instagram: form.value.instagram.trim(),
        facebook: form.value.facebook.trim(),
        youtube: form.value.youtube.trim(),
        googleAnalyticsId: form.value.googleAnalyticsId.trim(),
        facebookPixelId: form.value.facebookPixelId.trim(),
        customHeadHtml: form.value.customHeadHtml,
        address: form.value.address,
        hours: form.value.hours.filter((hour) => hour.days.trim() || hour.hours.trim()),
        settings: form.value.settings,
      })
      syncFromStore()
      ui.success('Dados do cardapio salvos.')
      return { ok: true }
    } catch (caught) {
      ui.error(getApiErrorMessage(caught, 'Nao foi possivel salvar os dados.'))
      return { ok: false }
    } finally {
      savingDados.value = false
    }
  }

  async function uploadAndApply(file: File, apply: (url: string) => void) {
    if (uploading.value || !store.restaurantId) {
      return
    }
    uploading.value = true
    try {
      const url = await store.uploadMedia(store.restaurantId, file)
      if (url) {
        apply(url)
      }
    } catch (caught) {
      ui.error(getApiErrorMessage(caught, 'Nao foi possivel enviar a imagem.'))
    } finally {
      uploading.value = false
    }
  }

  function addHour() {
    form.value.hours.push({ days: '', hours: '' })
  }

  function removeHour(index: number) {
    form.value.hours.splice(index, 1)
  }

  return {
    form,
    dirty,
    savingDados,
    uploading,
    saveDados,
    syncFromStore,
    uploadAndApply,
    addHour,
    removeHour,
  }
}
