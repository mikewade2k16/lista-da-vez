<script setup lang="ts">
// OmniEntityDrawer — casca/template CANONICA de modal do painel Omni.
//
// E o "core" de todo modal: header com fechar + expandir/encolher (toggle de tela
// cheia) + popover de modo (lado/centro/tela cheia), resize no modo lado, corpo
// rolavel e rodape. Todo modal novo usa este componente; so o conteudo muda (slot
// default), acoes especificas vao no slot #header-extra e o rodape no slot #footer.
// Centraliza os ajustes do modal num lugar so (ver docs/frontend/MODAL_TEMPLATE.md).
//
// Modos (v-model:mode): 'side' (drawer a direita, redimensionavel e nao-bloqueante)
// | 'center' (modal central com overlay) | 'fullscreen' (tela cheia). A largura do
// modo side e redimensionavel (v-model:width opcional) e publicada na custom
// property --omni-drawer-side-width para o CSS do painel (e para a pagina que quer
// se ajustar ao modal aberto).
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'

type DrawerMode = 'side' | 'center' | 'fullscreen'

const SIDE_MIN_WIDTH = 560
const SIDE_MAX_CAP = 1120
const SIDE_DEFAULT_WIDTH = 720

const props = withDefaults(
  defineProps<{
    // Controla a abertura (v-model). Aberto = true.
    modelValue: boolean
    // Titulo principal exibido no header (opcional — tasks nao usa, poe no corpo).
    title?: string
    // Linha secundaria opcional sob o titulo.
    subtitle?: string
    // Modo de exibicao (v-model:mode). Default 'side' (drawer a direita).
    mode?: DrawerMode
    // Largura do modo 'side' em px (v-model:width). Sem binding, usa estado interno.
    width?: number
  }>(),
  {
    title: '',
    subtitle: '',
    mode: 'side',
    width: undefined,
  },
)

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
  (e: 'update:mode', value: DrawerMode): void
  (e: 'update:width', value: number): void
}>()

// Opcoes do popover de modo. Icones lucide via Nuxt UI.
const MODE_OPTIONS: Array<{ value: DrawerMode; icon: string; label: string }> = [
  { value: 'side', icon: 'i-lucide-panel-right', label: 'Modo lado a lado' },
  { value: 'center', icon: 'i-lucide-square', label: 'Modo centralizado' },
  { value: 'fullscreen', icon: 'i-lucide-expand', label: 'Pagina inteira' },
]

// Largura interna do modo side. Espelha a prop quando ha binding; serve de fonte
// quando o pai nao controla a largura.
const internalWidth = ref(typeof props.width === 'number' ? props.width : SIDE_DEFAULT_WIDTH)
watch(
  () => props.width,
  (value) => {
    if (typeof value === 'number') internalWidth.value = value
  },
)

const resizing = ref(false)
// Ultimo modo nao-fullscreen, para o botao expandir/encolher voltar ao modo certo.
const lastNonFullscreen = ref<DrawerMode>(props.mode === 'fullscreen' ? 'side' : props.mode)
watch(
  () => props.mode,
  (value) => {
    if (value !== 'fullscreen') lastNonFullscreen.value = value
  },
)

const isFullscreen = computed(() => props.mode === 'fullscreen')
// Modos 'center'/'fullscreen' ganham overlay bloqueante; 'side' nao bloqueia.
const hasOverlay = computed(() => props.mode !== 'side')

// Publica a largura do modo side numa custom property global enquanto aberto. O
// painel do USlideover e teleportado, entao o CSS le a var com fallback. A pagina
// hospedeira tambem pode ler --omni-drawer-side-width para se ajustar ao modal.
function syncSideWidthVar() {
  if (!import.meta.client) return
  const root = document.documentElement
  if (props.modelValue && props.mode === 'side') {
    root.style.setProperty('--omni-drawer-side-width', `${internalWidth.value}px`)
  } else {
    root.style.removeProperty('--omni-drawer-side-width')
  }
}

watch(
  () => [props.modelValue, props.mode, internalWidth.value],
  () => syncSideWidthVar(),
  { immediate: true },
)

function setMode(next: DrawerMode) {
  if (next === props.mode) return
  emit('update:mode', next)
}

// Botao expandir/encolher: alterna entre tela cheia e o ultimo modo nao-fullscreen.
function toggleFullscreen() {
  if (isFullscreen.value) {
    setMode(lastNonFullscreen.value || 'side')
  } else {
    lastNonFullscreen.value = props.mode
    setMode('fullscreen')
  }
}

function setWidth(next: number) {
  const clamped = Math.max(
    SIDE_MIN_WIDTH,
    Math.min(
      import.meta.client ? Math.min(window.innerWidth - 80, SIDE_MAX_CAP) : SIDE_MAX_CAP,
      next,
    ),
  )
  internalWidth.value = clamped
  emit('update:width', clamped)
}

// Resize do modo side: arrasta a borda esquerda do painel (à direita). Mesma
// mecanica comprovada do modal de tasks.
function startResize(event: MouseEvent) {
  if (props.mode !== 'side' || !import.meta.client) return
  event.preventDefault()
  resizing.value = true
  const startX = event.clientX
  const startWidth = internalWidth.value
  const onMove = (moveEvent: MouseEvent) => {
    setWidth(startWidth + (startX - moveEvent.clientX))
  }
  const onUp = () => {
    resizing.value = false
    window.removeEventListener('mousemove', onMove)
    window.removeEventListener('mouseup', onUp)
  }
  window.addEventListener('mousemove', onMove)
  window.addEventListener('mouseup', onUp)
}

function close() {
  emit('update:modelValue', false)
}

// USlideover emite update:open; espelha de volta no v-model do pai e fecha.
function onOpenChange(open: boolean) {
  if (!open) close()
}

// Esc fecha em qualquer modo. No modo 'side' o USlideover roda sem modal
// (:dismissible=false para nao fechar no clique-fora), entao o Esc nativo dele nao
// dispara — este listener garante o Esc tambem nesse modo, sem reativar o
// fechamento por clique-fora (principio: features coexistem, nao se substituem).
function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape' && props.modelValue) close()
}

onMounted(() => {
  if (import.meta.client) document.addEventListener('keydown', onKeydown)
})

onBeforeUnmount(() => {
  if (import.meta.client) {
    document.documentElement.style.removeProperty('--omni-drawer-side-width')
    document.removeEventListener('keydown', onKeydown)
  }
})
</script>

<template>
  <USlideover
    :open="modelValue"
    :overlay="hasOverlay"
    :modal="hasOverlay"
    :dismissible="hasOverlay"
    :ui="{ content: `omni-entity-drawer omni-entity-drawer--${mode}` }"
    @update:open="onOpenChange"
  >
    <template #header>
      <div class="omni-entity-drawer__header">
        <!-- Controles canonicos: fechar + expandir/encolher + popover de modo. -->
        <div class="omni-entity-drawer__nav">
          <UButton
            icon="i-lucide-chevrons-right"
            color="neutral"
            variant="ghost"
            size="xs"
            title="Fechar"
            aria-label="Fechar"
            @click="close"
          />
          <UButton
            :icon="isFullscreen ? 'i-lucide-shrink' : 'i-lucide-expand'"
            color="neutral"
            variant="ghost"
            size="xs"
            :title="isFullscreen ? 'Sair da tela cheia' : 'Tela cheia'"
            :aria-label="isFullscreen ? 'Sair da tela cheia' : 'Tela cheia'"
            @click="toggleFullscreen"
          />
          <UPopover :content="{ side: 'bottom', align: 'start' }">
            <UButton
              icon="i-lucide-panel-right"
              color="neutral"
              variant="ghost"
              size="xs"
              title="Modo do modal"
              aria-label="Modo do modal"
            />
            <template #content>
              <div class="omni-entity-drawer__mode-menu">
                <button
                  v-for="option in MODE_OPTIONS"
                  :key="option.value"
                  type="button"
                  class="omni-entity-drawer__mode-item"
                  @click="setMode(option.value)"
                >
                  <UIcon :name="option.icon" class="h-4 w-4" />
                  <span>{{ option.label }}</span>
                  <UIcon
                    v-if="mode === option.value"
                    name="i-lucide-check"
                    class="ml-auto h-4 w-4"
                  />
                </button>
              </div>
            </template>
          </UPopover>
        </div>

        <div v-if="title || subtitle" class="omni-entity-drawer__copy">
          <h3 v-if="title" class="omni-entity-drawer__title">{{ title }}</h3>
          <p v-if="subtitle" class="omni-entity-drawer__subtitle">{{ subtitle }}</p>
        </div>

        <!-- Acoes especificas do consumidor (ex.: presenca/compartilhar do tasks). -->
        <div class="omni-entity-drawer__extra">
          <slot name="header-extra"></slot>
        </div>
      </div>
    </template>

    <template #body>
      <!-- Container de rolagem do corpo: cresce e rola sozinho. -->
      <div class="omni-entity-drawer__body">
        <div class="omni-entity-drawer__content">
          <button
            v-if="mode === 'side'"
            class="omni-entity-drawer__resize-handle"
            type="button"
            aria-label="Redimensionar modal"
            @mousedown="startResize"
          ></button>
          <slot></slot>
        </div>
      </div>
    </template>

    <template v-if="$slots.footer" #footer>
      <div class="omni-entity-drawer__footer">
        <slot name="footer"></slot>
      </div>
    </template>
  </USlideover>
</template>

<style scoped>
/*
  CSS self-contained (BEM .omni-entity-drawer + --side/--center/--fullscreen).
  As classes caem no slot :ui="{ content }" do USlideover, que renderiza o painel
  fora do <style scoped> normal — por isso os seletores de posicao usam :deep para
  alcancar o elemento de conteudo. Valores de posicao usam !important para vencer o
  tema base do Nuxt UI (mesmo padrao do tasks-modal.css).
*/

/* Largura/posicao do painel por modo --------------------------------------- */
:deep(.omni-entity-drawer.omni-entity-drawer--side) {
  width: var(--omni-drawer-side-width, min(720px, calc(100vw - 1rem))) !important;
  max-width: min(var(--omni-drawer-side-width, 720px), calc(100vw - 1rem)) !important;
  border-radius: 0 !important;
  box-shadow: var(--shadow-md) !important;
}

:deep(.omni-entity-drawer.omni-entity-drawer--center) {
  right: auto !important;
  left: 50% !important;
  top: 50% !important;
  bottom: auto !important;
  width: min(880px, calc(100vw - 2rem)) !important;
  max-width: min(880px, calc(100vw - 2rem)) !important;
  height: min(840px, calc(100vh - 2rem)) !important;
  transform: translate(-50%, -50%) !important;
  border-radius: var(--radius-md) !important;
  box-shadow: var(--shadow-dropdown, 0 28px 70px rgb(15 23 42 / 0.22)) !important;
}

:deep(.omni-entity-drawer.omni-entity-drawer--fullscreen) {
  inset: 0 !important;
  width: 100vw !important;
  max-width: 100vw !important;
  height: 100vh !important;
  border-radius: 0 !important;
}

/* Header ------------------------------------------------------------------- */
.omni-entity-drawer__header {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  width: 100%;
  min-height: 2.25rem;
}

.omni-entity-drawer__nav {
  display: inline-flex;
  align-items: center;
  gap: 0.15rem;
  flex-shrink: 0;
}

.omni-entity-drawer__copy {
  display: grid;
  gap: 0.2rem;
  min-width: 0;
}

.omni-entity-drawer__title {
  margin: 0;
  color: rgb(var(--text));
  font-size: 1.05rem;
  font-weight: 700;
  line-height: 1.25;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.omni-entity-drawer__subtitle {
  margin: 0;
  color: rgb(var(--muted));
  font-size: 0.82rem;
  line-height: 1.35;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.omni-entity-drawer__extra {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  margin-left: auto;
  min-width: 0;
}

/* Popover de modo ---------------------------------------------------------- */
.omni-entity-drawer__mode-menu {
  width: 14rem;
  padding: 0.25rem;
  display: grid;
  gap: 0.15rem;
}

.omni-entity-drawer__mode-item {
  width: 100%;
  min-height: 2rem;
  display: flex;
  align-items: center;
  gap: 0.5rem;
  border-radius: var(--radius-sm);
  padding: 0.35rem 0.5rem;
  color: rgb(var(--text));
  font-size: 0.85rem;
  text-align: left;
  background: transparent;
}

.omni-entity-drawer__mode-item:hover {
  background: rgb(var(--surface-2));
}

/* Corpo: container de rolagem (principio "pagina nova precisa rolar") ------- */
.omni-entity-drawer__body {
  display: flex;
  flex-direction: column;
  min-height: 0;
  width: 100%;
}

.omni-entity-drawer__content {
  position: relative;
  display: flex;
  flex-direction: column;
  min-height: 0;
  max-width: 860px;
  margin-inline: auto;
  width: 100%;
  padding: 0 0 1.5rem;
}

/* Handle de resize: borda esquerda do painel (so no modo side). ------------ */
.omni-entity-drawer__resize-handle {
  position: absolute;
  left: -0.85rem;
  top: -3.5rem;
  bottom: -3rem;
  width: 0.85rem;
  padding: 0;
  border: 0;
  background: transparent;
  cursor: col-resize;
  z-index: 2;
}

.omni-entity-drawer__resize-handle::after {
  content: '';
  position: absolute;
  left: 0.35rem;
  top: 4rem;
  bottom: 2rem;
  width: 1px;
  background: rgb(var(--border));
  opacity: 0;
  transition: opacity 0.16s ease;
}

.omni-entity-drawer__resize-handle:hover::after {
  opacity: 1;
}

/* Rodape ------------------------------------------------------------------- */
.omni-entity-drawer__footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.5rem;
  width: 100%;
}

@media (max-width: 720px) {
  /* Em tela estreita, side e center viram tela cheia para nao espremer. */
  :deep(.omni-entity-drawer.omni-entity-drawer--side),
  :deep(.omni-entity-drawer.omni-entity-drawer--center) {
    width: 100vw !important;
    max-width: 100vw !important;
    height: 100vh !important;
    border-radius: 0 !important;
    inset: 0 !important;
    transform: none !important;
  }

  .omni-entity-drawer__resize-handle {
    display: none;
  }
}
</style>
