import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { ComputedRef, Ref } from 'vue'

/**
 * Overflow responsivo do menu do header. Mede quantos itens top-level cabem em
 * `navRef` (usando a largura real de cada item, lida numa faixa de medicao
 * oculta) e devolve a fatia visivel + o excedente para um popover "Mais".
 *
 * Generico no tipo do item (`T`) para preservar o shape original (NavItem) nas
 * fatias retornadas. O componente deve: (1) registrar cada item da faixa de
 * medicao via `setItemEl`, (2) renderizar `visibleHeaderItems` na barra e
 * `overflowHeaderItems` no "Mais".
 *
 * @param navRef Container da navegacao (largura disponivel).
 * @param headerItems Itens top-level do header (ja filtrados/ordenados).
 * @returns Fatias visivel/overflow, flag de overflow e registradores de medicao.
 */
export function useHeaderNavOverflow<T extends { id: string }>(
  navRef: Ref<HTMLElement | null>,
  headerItems: ComputedRef<T[]>,
) {
  const itemEls = new Map<string, HTMLElement>()
  // Quantos itens cabem (do inicio da lista). Default = todos ate a primeira
  // medicao, para nao piscar o botao "Mais" no primeiro paint.
  const visibleCount = ref(Number.MAX_SAFE_INTEGER)
  let resizeObserver: ResizeObserver | null = null

  function setItemEl(id: string, el: unknown) {
    if (el instanceof HTMLElement) itemEls.set(id, el)
    else itemEls.delete(id)
  }

  const visibleHeaderItems = computed(() => {
    if (visibleCount.value >= headerItems.value.length) return headerItems.value
    return headerItems.value.slice(0, Math.max(0, visibleCount.value))
  })

  const overflowHeaderItems = computed(() => {
    if (visibleCount.value >= headerItems.value.length) return []
    return headerItems.value.slice(Math.max(0, visibleCount.value))
  })

  const hasOverflow = computed(() => overflowHeaderItems.value.length > 0)

  // Mede a largura disponivel e a de cada item para achar o cutoff, reservando
  // espaco para o botao "Mais" sempre que sobrar item.
  function measureOverflow() {
    if (!import.meta.client) return
    const nav = navRef.value
    if (!nav) return
    const available = nav.clientWidth
    if (available <= 0) return
    const gap = 4 // espacamento entre itens (.dashboard-header__nav gap ~0.24rem)
    const moreReserve = 64 // largura aproximada reservada para o botao "Mais"
    let used = 0
    let count = 0
    const widths = headerItems.value.map((item) => {
      const el = itemEls.get(item.id)
      return el ? el.offsetWidth : 0
    })
    for (let i = 0; i < widths.length; i += 1) {
      const next = used + widths[i] + (count > 0 ? gap : 0)
      const remaining = widths.length - (i + 1)
      const reserve = remaining > 0 ? moreReserve + gap : 0
      if (next + reserve > available) break
      used = next
      count += 1
    }
    visibleCount.value = count >= widths.length ? Number.MAX_SAFE_INTEGER : count
  }

  function scheduleMeasure() {
    void nextTick(() => measureOverflow())
  }

  watch(headerItems, () => scheduleMeasure())

  onMounted(() => {
    if (navRef.value && typeof ResizeObserver !== 'undefined') {
      resizeObserver = new ResizeObserver(() => scheduleMeasure())
      resizeObserver.observe(navRef.value)
    }
    scheduleMeasure()
  })

  onBeforeUnmount(() => {
    if (resizeObserver) {
      resizeObserver.disconnect()
      resizeObserver = null
    }
  })

  return { visibleHeaderItems, overflowHeaderItems, hasOverflow, setItemEl, scheduleMeasure }
}
