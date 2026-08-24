import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useCoreAccountStore } from '../../layers/core/stores/account'
import type { AssistantChatSurface } from '~/domain/calendar/calendar-chat-api'
import { useCalendarStore } from '~/stores/calendar'
import type { CalendarChatConfig, CalendarChatPosition } from '~/utils/calendar'

// Layout da janela compartilhada do assistente. Na surface Calendar preserva o contrato
// historico e persiste em calendar.config.chat SOMENTE depois de a configuracao da account
// estar hidratada. Meta/global usam memoria isolada por account+surface: nunca fazem o PUT
// full-replace do Calendar a partir de defaults.

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
const DEFAULT_CHAT_LAYOUT: CalendarChatConfig = { position: 'center', width: 0, height: 0 }

interface AreaRect {
  left: number
  top: number
  width: number
  height: number
}

export function shouldPersistAssistantWindowLayout(
  surface: AssistantChatSurface,
  calendarConfigLoaded: boolean,
): boolean {
  return surface === 'calendar' && calendarConfigLoaded
}

export function useCalendarChatWindow(
  anchorSelector: () => string = () => '.calendar-page',
  surface: () => AssistantChatSurface = () => 'calendar',
) {
  const store = useCalendarStore()
  const accountStore = useCoreAccountStore()

  // Retangulo da area interna do calendario (.calendar-page), base do posicionamento.
  const areaRect = ref<AreaRect>({ left: 0, top: 0, width: 0, height: 0 })
  const resizing = ref(false)

  // Layouts nao-Calendar sao efemeros e escopados. Nao usam localStorage nem o banco do
  // Calendar; ao trocar account nunca reaproveitam medidas da account anterior.
  const transientLayouts = useState<Record<string, CalendarChatConfig>>(
    'assistant-chat:window-layouts',
    () => ({}),
  )
  const localChat = ref<CalendarChatConfig>({ ...DEFAULT_CHAT_LAYOUT })
  let savePending = false
  let saveTimer = 0
  let contextGeneration = 0

  function activeAccountId(): string {
    return String(accountStore.activeAccountId || '').trim()
  }

  function transientLayoutKey(): string {
    return `${activeAccountId() || 'no-account'}:${surface()}`
  }

  function layoutForCurrentContext(): CalendarChatConfig {
    if (shouldPersistAssistantWindowLayout(surface(), store.isConfigLoaded)) {
      return { ...store.config.chat }
    }
    if (surface() === 'calendar') return { ...DEFAULT_CHAT_LAYOUT }
    return { ...(transientLayouts.value[transientLayoutKey()] ?? DEFAULT_CHAT_LAYOUT) }
  }

  function clearSaveTimer(): void {
    if (typeof window !== 'undefined' && saveTimer) window.clearTimeout(saveTimer)
    saveTimer = 0
  }

  function measureArea(): void {
    if (typeof document === 'undefined') return
    const el = document.querySelector(anchorSelector()) as HTMLElement | null
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

  // Trocar surface/account cancela qualquer debounce e troca imediatamente para o layout
  // daquele contexto. Calendar sem GET concluido usa somente o default visual, sem PUT.
  watch(
    [surface, () => accountStore.activeAccountId, () => store.isConfigLoaded],
    () => {
      contextGeneration += 1
      clearSaveTimer()
      savePending = false
      localChat.value = layoutForCurrentContext()
      measureArea()
    },
    { immediate: true },
  )

  // Re-hidrata do banco apenas na surface Calendar e somente para a account carregada,
  // EXCETO enquanto o proprio save deste layout esta pendente.
  watch(
    () => store.config.chat,
    (value) => {
      if (surface() === 'calendar' && store.isConfigLoaded && !savePending && value) {
        localChat.value = { ...value }
      }
    },
    { deep: true },
  )

  function applyChat(next: CalendarChatConfig): void {
    localChat.value = next
    if (surface() !== 'calendar') {
      transientLayouts.value = {
        ...transientLayouts.value,
        [transientLayoutKey()]: { ...next },
      }
      return
    }
    // Nunca salva o objeto default antes de fetchConfig concluir para a account ativa.
    if (!shouldPersistAssistantWindowLayout(surface(), store.isConfigLoaded)) return

    savePending = true
    clearSaveTimer()
    const generation = contextGeneration
    const accountId = activeAccountId()
    saveTimer = window.setTimeout(() => {
      saveTimer = 0
      if (
        generation !== contextGeneration ||
        surface() !== 'calendar' ||
        activeAccountId() !== accountId ||
        !shouldPersistAssistantWindowLayout(surface(), store.isConfigLoaded)
      ) {
        savePending = false
        return
      }
      // saveConfig faz full-replace do config; mantem as demais secoes e troca so chat.
      void store.saveConfig({ ...store.config, chat: { ...localChat.value } }).finally(() => {
        if (generation === contextGeneration) savePending = false
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
      clearSaveTimer()
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
