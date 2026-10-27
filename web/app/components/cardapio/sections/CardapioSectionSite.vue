<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'

import { useCardapioStore } from '~/stores/cardapio'
import { useUiStore } from '~/stores/ui'
import { getApiErrorMessage } from '~/utils/api-client'
import type { SiteLayout } from '~/domain/cardapio/types'

// Aba Site (Site Builder, desenho B4): embute o Studio do TAVOLA num iframe e
// SALVA/PUBLICA o layout pela API do painel — o painel detem o JWT; o iframe
// NUNCA recebe token. O Studio so edita o documento de layout e devolve por
// postMessage; a persistencia (rascunho/publicacao) e do painel.
//
// Protocolo postMessage (canal 'omni-studio'):
//   painel -> iframe: { channel, type:'init', layout: SiteLayout|null, restaurantTheme }
//                     { channel, type:'restaurant-theme', restaurantTheme }
//                     { channel, type:'upload-result', requestId, url }
//   iframe -> painel: { channel, type:'ready' } e { channel, type:'change', layout }
//                     { channel, type:'upload-request', requestId, file }
// Seguranca: o painel SO aceita mensagens cujo event.origin === origem do Studio
// (derivada de studioUrl). Em 'ready' carrega o rascunho (GET) e responde 'init'
// para essa origem. Em 'change' guarda o layout e marca rascunho nao salvo.
// Em 'upload-request' sobe o arquivo pela API de midia (o iframe nao tem token).

const STUDIO_CHANNEL = 'omni-studio'

interface StudioInboundMessage {
  channel: string
  type: string
  layout?: SiteLayout | null
  requestId?: string
  file?: File
}

const store = useCardapioStore()
const ui = useUiStore()
const runtimeConfig = useRuntimeConfig()

const studioUrl = computed(() => String(runtimeConfig.public.studioUrl || '').trim())

// Origem aceita nas mensagens do iframe. Derivada da studioUrl; vazia se a URL
// for invalida (nesse caso nenhuma mensagem e aceita — fail-safe).
const studioOrigin = computed(() => {
  if (!studioUrl.value) {
    return ''
  }
  try {
    return new URL(studioUrl.value).origin
  } catch {
    return ''
  }
})

const slug = computed(() => store.restaurant?.slug ?? '')

const iframeSrc = computed(() => {
  if (!studioUrl.value || !slug.value) {
    return ''
  }
  return `${studioUrl.value}/studio?slug=${encodeURIComponent(slug.value)}&embed=1`
})

const iframeRef = ref<HTMLIFrameElement | null>(null)

// Layout corrente (rascunho editado no Studio) + versao conhecida do back para o
// controle de concorrencia (If-Match no PUT). dirty = ha edicao nao salva.
const layout = ref<SiteLayout | null>(null)
const version = ref(0)
const dirty = ref(false)

const studioReady = ref(false)
const saving = ref(false)
const publishing = ref(false)

// Tela cheia (W6): abre o frame-wrap em modo fullscreen.
const fullscreen = ref(false)

function enterFullscreen(): void {
  fullscreen.value = true
}

function exitFullscreen(): void {
  fullscreen.value = false
}

function onFullscreenKey(event: KeyboardEvent): void {
  if (fullscreen.value && event.key === 'Escape') {
    exitFullscreen()
  }
}

// Historico de undo/redo reportado pelo Studio via postMessage.
const canUndo = ref(false)
const canRedo = ref(false)

const canSave = computed(
  () =>
    !!store.restaurantId && studioReady.value && dirty.value && !saving.value && !publishing.value,
)
const canPublish = computed(
  () =>
    !!store.restaurantId &&
    studioReady.value &&
    !!layout.value &&
    !saving.value &&
    !publishing.value,
)

// Extrai o HTTP status de um erro do $fetch (ofetch): pode vir como statusCode,
// status, response.status ou data.statusCode dependendo da camada.
function errorStatus(error: unknown): number {
  const source = error as {
    statusCode?: number
    status?: number
    response?: { status?: number }
    data?: { statusCode?: number }
  } | null
  return Number(
    source?.statusCode ??
      source?.status ??
      source?.response?.status ??
      source?.data?.statusCode ??
      0,
  )
}

function postToStudio(message: Record<string, unknown>) {
  const target = iframeRef.value?.contentWindow
  if (!target || !studioOrigin.value) {
    return
  }
  target.postMessage(
    JSON.parse(JSON.stringify({ channel: STUDIO_CHANNEL, ...message })),
    studioOrigin.value,
  )
}

function restaurantTheme() {
  return store.restaurant?.theme ?? null
}

watch(
  () => store.restaurant?.theme,
  () => {
    if (studioReady.value)
      postToStudio({ type: 'restaurant-theme', restaurantTheme: restaurantTheme() })
  },
  { deep: true },
)

// Carrega o rascunho do back e devolve 'init' ao Studio (layout ou null se vazio).
async function sendInitLayout() {
  const id = store.restaurantId
  if (!id) {
    postToStudio({ type: 'init', layout: null, restaurantTheme: restaurantTheme() })
    return
  }
  try {
    const result = await store.loadLayout(id)
    layout.value = result.layout
    version.value = result.version
    dirty.value = false
    const hasPages = Object.keys(result.layout.pages ?? {}).length > 0
    postToStudio({
      type: 'init',
      layout: hasPages ? result.layout : null,
      restaurantTheme: restaurantTheme(),
    })
  } catch (caught) {
    postToStudio({ type: 'init', layout: null, restaurantTheme: restaurantTheme() })
    ui.error(getApiErrorMessage(caught, 'Nao foi possivel carregar o rascunho do site.'))
  }
}

// Upload pedido pelo Studio: o iframe NAO tem token, entao o painel (que detem o
// JWT) sobe o arquivo pela MESMA API de midia do cardapio usada por logo/banner/
// produto (store.uploadMedia) e devolve a URL ao Studio. Em erro, responde com
// url vazia e mostra o toast — o Studio trata o vazio como falha.
async function handleUploadRequest(requestId: string, file: File | undefined) {
  const id = store.restaurantId
  if (!id || !file) {
    postToStudio({ type: 'upload-result', requestId, url: '' })
    if (!id) {
      ui.error('Carregando estabelecimento. Tente enviar a imagem novamente.')
    }
    return
  }
  try {
    const url = await store.uploadMedia(id, file)
    postToStudio({ type: 'upload-result', requestId, url })
    if (!url) {
      ui.error('Nao foi possivel enviar a imagem.')
    }
  } catch (caught) {
    postToStudio({ type: 'upload-result', requestId, url: '' })
    ui.error(getApiErrorMessage(caught, 'Nao foi possivel enviar a imagem.'))
  }
}

function onMessage(event: MessageEvent) {
  // Defesa: so a origem do Studio e aceita; mensagens de outras janelas/origens
  // (ads, extensoes, outros iframes) sao ignoradas silenciosamente.
  if (
    !studioOrigin.value ||
    event.origin !== studioOrigin.value ||
    event.source !== iframeRef.value?.contentWindow
  ) {
    return
  }
  const data = event.data as StudioInboundMessage | null
  if (!data || data.channel !== STUDIO_CHANNEL) {
    return
  }

  switch (data.type) {
    case 'ready':
      studioReady.value = true
      void sendInitLayout()
      break
    case 'loaded':
      // Layout inicial que o Studio exibe (carregado do back OU o default).
      // Guarda SEM marcar "nao salvo": ja e publicavel como esta.
      if (data.layout && typeof data.layout === 'object') {
        layout.value = data.layout
        dirty.value = false
      }
      break
    case 'change':
      if (data.layout && typeof data.layout === 'object') {
        layout.value = data.layout
        dirty.value = true
      }
      break
    case 'history': {
      const h = data as StudioInboundMessage & { canUndo?: boolean; canRedo?: boolean }
      canUndo.value = h.canUndo === true
      canRedo.value = h.canRedo === true
      break
    }
    case 'upload-request':
      void handleUploadRequest(String(data.requestId ?? ''), data.file)
      break
    default:
      break
  }
}

function onUndo(): void {
  postToStudio({ type: 'undo' })
}

function onRedo(): void {
  postToStudio({ type: 'redo' })
}

async function onSaveDraft() {
  const id = store.restaurantId
  if (!id || !layout.value || !canSave.value) {
    return
  }
  saving.value = true
  try {
    const result = await store.putDraftLayout(id, layout.value, version.value)
    layout.value = result.layout
    version.value = result.version
    dirty.value = false
    ui.success('Rascunho salvo.')
  } catch (caught) {
    // 412 = outro editor salvou no meio (conflito de versao).
    if (errorStatus(caught) === 412) {
      ui.error(
        'O site foi alterado em outra aba/sessao. Recarregue a aba Site para nao perder dados.',
      )
    } else {
      ui.error(getApiErrorMessage(caught, 'Nao foi possivel salvar o rascunho.'))
    }
  } finally {
    saving.value = false
  }
}

async function onPublish() {
  const id = store.restaurantId
  if (!id || !canPublish.value) {
    return
  }
  // Publica o rascunho salvo. Se ha edicao pendente, salva antes para publicar o
  // estado mais recente.
  publishing.value = true
  try {
    // Garante que ha um rascunho gravado (cria/atualiza a linha) ANTES de publicar
    // — o publish promove o rascunho; sem linha daria 404.
    if (layout.value) {
      const draft = await store.putDraftLayout(id, layout.value, version.value)
      layout.value = draft.layout
      version.value = draft.version
      dirty.value = false
    }
    const result = await store.publishLayout(id)
    layout.value = result.layout
    version.value = result.version
    ui.success('Site publicado.')
  } catch (caught) {
    if (errorStatus(caught) === 412) {
      ui.error('O site foi alterado em outra aba/sessao. Recarregue a aba Site antes de publicar.')
    } else {
      ui.error(getApiErrorMessage(caught, 'Nao foi possivel publicar o site.'))
    }
  } finally {
    publishing.value = false
  }
}

onMounted(() => {
  window.addEventListener('message', onMessage)
  window.addEventListener('keydown', onFullscreenKey)
})

onBeforeUnmount(() => {
  window.removeEventListener('message', onMessage)
  window.removeEventListener('keydown', onFullscreenKey)
})
</script>

<template>
  <div class="cardapio-site">
    <header class="cardapio-site__bar">
      <div class="cardapio-site__info">
        <p class="cardapio-site__note">
          Edite o site no Studio abaixo. As alteracoes ficam em rascunho ate voce salvar; publique
          para o site publico refletir.
        </p>
        <span
          v-if="dirty"
          class="cardapio-site__status cardapio-site__status--dirty"
          aria-live="polite"
        >
          Rascunho nao salvo
        </span>
        <span
          v-else-if="studioReady"
          class="cardapio-site__status cardapio-site__status--saved"
          aria-live="polite"
        >
          Tudo salvo
        </span>
      </div>

      <div class="cardapio-site__actions">
        <!-- Desfazer/Refazer (W4): postam undo/redo pro Studio via postMessage. -->
        <button
          type="button"
          class="cardapio-site__btn cardapio-site__btn--icon"
          :disabled="!studioReady || !canUndo"
          title="Desfazer"
          @click="onUndo"
        >
          Desfazer
        </button>
        <button
          type="button"
          class="cardapio-site__btn cardapio-site__btn--icon"
          :disabled="!studioReady || !canRedo"
          title="Refazer"
          @click="onRedo"
        >
          Refazer
        </button>
        <span class="cardapio-site__divider" aria-hidden="true"></span>
        <button type="button" class="cardapio-site__btn" :disabled="!canSave" @click="onSaveDraft">
          <span v-if="saving" class="cardapio-site__spinner" aria-hidden="true"></span>
          {{ saving ? 'Salvando...' : 'Salvar rascunho' }}
        </button>
        <button
          type="button"
          class="cardapio-site__btn cardapio-site__btn--primary"
          :disabled="!canPublish"
          @click="onPublish"
        >
          <span v-if="publishing" class="cardapio-site__spinner" aria-hidden="true"></span>
          {{ publishing ? 'Publicando...' : 'Publicar' }}
        </button>
      </div>
    </header>

    <p v-if="!studioUrl" class="cardapio-site__empty">
      O endereco do Studio nao esta configurado (NUXT_PUBLIC_STUDIO_URL). Fale com o suporte.
    </p>
    <p v-else-if="!slug" class="cardapio-site__empty">Carregando estabelecimento...</p>

    <div
      v-else
      class="cardapio-site__frame-wrap"
      :class="{ 'cardapio-site__frame-wrap--fullscreen': fullscreen }"
    >
      <!-- Botão Tela cheia (W6): exibe quando NAO estiver em fullscreen. -->
      <button
        v-if="!fullscreen"
        type="button"
        class="cardapio-site__fullscreen-btn"
        title="Tela cheia"
        @click="enterFullscreen"
      >
        Tela cheia
      </button>
      <!-- Botão Sair da tela cheia: exibe dentro do overlay. -->
      <button
        v-if="fullscreen"
        type="button"
        class="cardapio-site__exit-fullscreen-btn"
        title="Sair da tela cheia (Esc)"
        @click="exitFullscreen"
      >
        Sair
      </button>
      <iframe
        ref="iframeRef"
        :src="iframeSrc"
        class="cardapio-site__frame"
        title="Studio do site"
        loading="lazy"
      ></iframe>
    </div>
  </div>
</template>

<style scoped src="./cardapio-site.css"></style>
