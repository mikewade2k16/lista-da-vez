import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useCalendarStore } from '~/stores/calendar'
import type { CalendarChatConfig, CalendarChatPosition } from '~/utils/calendar'

// Layout + persistencia da janela de chat do Calendario (SPEC-F2, contrato CHATUI).
// Extraido do CalendarChatPanel.vue para manter o componente < 450 linhas. Calcula a
// caixa (px) da janela a partir da area interna do calendario (.calendar-page) e de
// config.chat (center = largura da area; left = ~painel esquerdo; right = ~modal
// direito), redimensiona por arrasto (molde do OmniEntityDrawer) e persiste em
// config.chat via store.saveConfig (debounced). O banco e' a fonte unica; o rascunho
// local so vence enquanto ha save pendente (principio 1).

// Margens/limites da janela (px).
const AREA_MARGIN = 12
const MIN_W = 320
const MIN_H = 320
const LEFT_W = 360 // ~ .calendar-leftcol
const RIGHT_W = 560 // ~ modal direito
// Centralizado = ~tamanho do calendario atras (proporcional a area), nem enxuto demais
// nem tela cheia: fracao da largura/altura da area do calendario.
const CENTER_W_RATIO = 0.62
const CENTER_H_RATIO = 0.84
const CENTER_W_MIN = 520
// Piso do topo: garante que a janela NUNCA cubra a barra de menu do painel, mesmo se a
// medicao da area do calendario falhar (header do dashboard ~3rem + folga).
const HEADER_SAFE = 64

interface AreaRect {
  left: number
  top: number
  width: number
  height: number
}

export function useCalendarChatWindow() {
  const store = useCalendarStore()

  // Retangulo da area interna do calendario (.calendar-page), base do posicionamento.
  const areaRect = ref<AreaRect>({ left: 0, top: 0, width: 0, height: 0 })
  const resizing = ref(false)

  // Rascunho local do layout: dirige o estilo na hora e agenda o PUT debounced.
  const localChat = ref<CalendarChatConfig>({ ...store.config.chat })
  let savePending = false
  let saveTimer = 0

  function measureArea(): void {
    if (typeof document === 'undefined') return
    const el = document.querySelector('.calendar-page') as HTMLElement | null
    const rect = el?.getBoundingClientRect()
    if (rect && rect.width > 0) {
      // top NUNCA acima do header (evita cobrir o menu quando o calendario encosta no topo).
      const top = Math.max(rect.top, HEADER_SAFE)
      areaRect.value = {
        left: rect.left,
        top,
        width: rect.width,
        height: rect.height - (top - rect.top),
      }
    } else if (typeof window !== 'undefined') {
      // Fallback seguro: abaixo do header, nao a tela inteira a partir do topo.
      areaRect.value = {
        left: 0,
        top: HEADER_SAFE,
        width: window.innerWidth,
        height: window.innerHeight - HEADER_SAFE,
      }
    }
  }

  // Re-hidrata de store.config.chat quando muda por fora, EXCETO com save pendente.
  watch(
    () => store.config.chat,
    (value) => {
      if (!savePending && value) localChat.value = { ...value }
    },
    { deep: true },
  )

  function applyChat(next: CalendarChatConfig): void {
    localChat.value = next
    savePending = true
    if (saveTimer) window.clearTimeout(saveTimer)
    saveTimer = window.setTimeout(() => {
      saveTimer = 0
      // saveConfig faz full-replace do config; mantem as demais secoes e troca so chat.
      void store.saveConfig({ ...store.config, chat: { ...localChat.value } }).finally(() => {
        savePending = false
      })
    }, 600)
  }

  function setPosition(position: CalendarChatPosition): void {
    // Trocar de posicao volta pro tamanho canonico dela (width/height 0 = default).
    applyChat({ position, width: 0, height: 0 })
  }

  // Caixa final (px). Posicoes: fullscreen = area toda; center = janela ENXUTA
  // centralizada; left/right ancoram nas bordas com largura de painel/modal. width/
  // height > 0 (arrasto) sempre vencem o default da posicao.
  const panelBox = computed(() => {
    const area = areaRect.value
    const maxW = Math.max(MIN_W, area.width - AREA_MARGIN * 2)
    const maxH = Math.max(MIN_H, area.height - AREA_MARGIN * 2)
    const cfg = localChat.value
    const full = cfg.position === 'fullscreen'

    let width: number
    if (full) width = maxW
    else if (cfg.width > 0) width = cfg.width
    else if (cfg.position === 'left') width = LEFT_W
    else if (cfg.position === 'right') width = RIGHT_W
    else width = Math.max(CENTER_W_MIN, Math.round(area.width * CENTER_W_RATIO))
    width = Math.min(maxW, Math.max(MIN_W, width))

    let height: number
    if (full) height = maxH
    else if (cfg.height > 0) height = cfg.height
    else if (cfg.position === 'center') height = Math.round(area.height * CENTER_H_RATIO)
    else height = maxH
    height = Math.min(maxH, Math.max(MIN_H, height))

    const top = full ? area.top + AREA_MARGIN : area.top + (area.height - height) / 2
    let left: number
    if (cfg.position === 'left') left = area.left + AREA_MARGIN
    else if (cfg.position === 'right') left = area.left + area.width - AREA_MARGIN - width
    else left = area.left + (area.width - width) / 2

    return { left, top: Math.max(top, area.top), width, height }
  })

  const panelStyle = computed(() => {
    const box = panelBox.value
    return {
      left: `${Math.round(box.left)}px`,
      top: `${Math.round(box.top)}px`,
      width: `${Math.round(box.width)}px`,
      height: `${Math.round(box.height)}px`,
    }
  })

  // No modo right a largura cresce pela borda ESQUERDA (handle bottom-left); nos demais
  // cresce pela direita (bottom-right).
  const resizeFromLeft = computed(() => localChat.value.position === 'right')

  // Redimensiona por arrasto. signX = sinal do delta X que aumenta a largura.
  function startResize(event: MouseEvent, signX: number): void {
    if (typeof window === 'undefined') return
    event.preventDefault()
    resizing.value = true
    const startX = event.clientX
    const startY = event.clientY
    const box = panelBox.value
    const startW = box.width
    const startH = box.height
    const onMove = (moveEvent: MouseEvent): void => {
      const area = areaRect.value
      const maxW = Math.max(MIN_W, area.width - AREA_MARGIN * 2)
      const maxH = Math.max(MIN_H, area.height - AREA_MARGIN * 2)
      const width = Math.min(maxW, Math.max(MIN_W, startW + signX * (moveEvent.clientX - startX)))
      const height = Math.min(maxH, Math.max(MIN_H, startH + (moveEvent.clientY - startY)))
      applyChat({ ...localChat.value, width, height })
    }
    const onUp = (): void => {
      resizing.value = false
      window.removeEventListener('mousemove', onMove)
      window.removeEventListener('mouseup', onUp)
    }
    window.addEventListener('mousemove', onMove)
    window.addEventListener('mouseup', onUp)
  }

  onMounted(() => {
    measureArea()
    window.addEventListener('resize', measureArea)
  })

  onBeforeUnmount(() => {
    if (typeof window !== 'undefined') {
      window.removeEventListener('resize', measureArea)
      if (saveTimer) window.clearTimeout(saveTimer)
    }
  })

  return {
    localChat,
    resizing,
    panelStyle,
    resizeFromLeft,
    measureArea,
    setPosition,
    startResize,
  }
}
