<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'

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
//   painel -> iframe: { channel, type:'init', layout: SiteLayout|null }
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
  target.postMessage({ channel: STUDIO_CHANNEL, ...message }, studioOrigin.value)
}

// Carrega o rascunho do back e devolve 'init' ao Studio (layout ou null se vazio).
async function sendInitLayout() {
  const id = store.restaurantId
  if (!id) {
    postToStudio({ type: 'init', layout: null })
    return
  }
  try {
    const result = await store.loadLayout(id)
    layout.value = result.layout
    version.value = result.version
    dirty.value = false
    const hasPages = Object.keys(result.layout.pages ?? {}).length > 0
    postToStudio({ type: 'init', layout: hasPages ? result.layout : null })
  } catch (caught) {
    postToStudio({ type: 'init', layout: null })
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
  if (!studioOrigin.value || event.origin !== studioOrigin.value) {
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

<style scoped>
.cardapio-site {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  min-height: 0;
  flex: 1;
}

.cardapio-site__bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  flex-wrap: wrap;
}

.cardapio-site__info {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  flex-wrap: wrap;
  min-width: 0;
}

.cardapio-site__note {
  font-size: 0.86rem;
  color: var(--text-muted);
  margin: 0;
}

.cardapio-site__status {
  padding: 0.18rem 0.6rem;
  border-radius: 999px;
  font-size: 0.74rem;
  font-weight: 600;
  white-space: nowrap;
}

.cardapio-site__status--dirty {
  background: rgb(var(--primary) / 0.16);
  color: rgb(var(--primary));
}

.cardapio-site__status--saved {
  background: rgb(var(--success) / 0.16);
  color: rgb(var(--success));
}

.cardapio-site__actions {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  flex-shrink: 0;
}

.cardapio-site__btn {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  padding: 0.5rem 0.95rem;
  border-radius: var(--radius-sm);
  border: 1px solid var(--line-soft);
  background: rgb(var(--surface-2) / 0.8);
  color: var(--text-main);
  font-weight: 600;
  font-size: 0.87rem;
  cursor: pointer;
  white-space: nowrap;
}

.cardapio-site__btn:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.cardapio-site__btn--primary {
  border: none;
  color: rgb(var(--surface));
  background: linear-gradient(135deg, rgb(var(--primary)), rgb(var(--primary-600)));
}

.cardapio-site__empty {
  color: var(--text-muted);
  font-size: 0.9rem;
  padding: 2rem 0;
  text-align: center;
}

.cardapio-site__frame-wrap {
  position: relative;
  flex: 1;
  min-height: 0;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-md);
  overflow: hidden;
  background: rgb(var(--surface-2) / 0.4);
}

.cardapio-site__frame {
  display: block;
  width: 100%;
  height: 100%;
  min-height: 70vh;
  border: none;
}

.cardapio-site__divider {
  display: inline-block;
  width: 1px;
  height: 1.4rem;
  background: var(--line-soft);
  margin: 0 0.2rem;
  align-self: center;
}

.cardapio-site__btn--icon {
  padding: 0.5rem 0.7rem;
}

/* Tela cheia (W6): o frame-wrap ocupa toda a viewport quando ativo. */

.cardapio-site__frame-wrap--fullscreen {
  position: fixed;
  inset: 0;
  z-index: 999;
  border-radius: 0;
  border: none;
  background: rgb(var(--surface-2));
}

/* Botao de tela cheia posicionado sobre o canto superior direito do frame. */
.cardapio-site__fullscreen-btn {
  position: absolute;
  top: 0.6rem;
  right: 0.6rem;
  z-index: 10;
  padding: 0.3rem 0.75rem;
  border-radius: var(--radius-sm);
  border: 1px solid var(--line-soft);
  background: rgb(var(--surface-2) / 0.9);
  color: var(--text-main);
  font-size: 0.8rem;
  font-weight: 600;
  cursor: pointer;
  backdrop-filter: blur(4px);
  opacity: 0;
  transition: opacity 0.15s;
}

.cardapio-site__frame-wrap:hover .cardapio-site__fullscreen-btn {
  opacity: 1;
}

/* Botao de sair, sobreposto ao canto da tela em fullscreen. */
.cardapio-site__exit-fullscreen-btn {
  position: absolute;
  top: 0.75rem;
  right: 0.75rem;
  z-index: 1000;
  padding: 0.35rem 0.85rem;
  border-radius: var(--radius-sm);
  border: 1px solid var(--line-soft);
  background: rgb(var(--surface-2) / 0.95);
  color: var(--text-main);
  font-size: 0.82rem;
  font-weight: 600;
  cursor: pointer;
  backdrop-filter: blur(4px);
  box-shadow: 0 2px 8px rgb(0 0 0 / 0.18);
}

.cardapio-site__spinner {
  width: 0.85rem;
  height: 0.85rem;
  border-radius: 999px;
  border: 2px solid rgb(var(--primary) / 0.35);
  border-top-color: rgb(var(--primary));
  animation: cardapio-site-spin 0.7s linear infinite;
}

@keyframes cardapio-site-spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
