import { onBeforeUnmount, onMounted, watch } from 'vue'
import type { Ref } from 'vue'

interface DropdownDismissOptions {
  // Raiz do dropdown; quando informada, clicar FORA dela fecha. Sem raiz, o
  // clique-fora nao e tratado (ex.: drawer com backdrop proprio).
  rootRef?: Ref<HTMLElement | null>
  // Fecha ao trocar de rota (default true).
  closeOnRouteChange?: boolean
}

/**
 * Regra de dropdown do AGENT_RULES: todo menu/popover feito a mao fecha no
 * clique-fora, no Esc e ao navegar. Centraliza esse comportamento (listeners no
 * document removidos no unmount) para nao reescrever em cada componente.
 *
 * @param isOpen Getter do estado aberto.
 * @param close Acao que fecha o dropdown.
 * @param options Raiz para clique-fora e toggle de fechar-na-rota.
 */
export function useDropdownDismiss(
  isOpen: () => boolean,
  close: () => void,
  options: DropdownDismissOptions = {},
) {
  const { rootRef, closeOnRouteChange = true } = options

  function handlePointerDown(event: PointerEvent) {
    if (!isOpen() || !rootRef?.value) return
    const target = event.target as Node | null
    if (target && !rootRef.value.contains(target)) close()
  }

  function handleKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape' && isOpen()) close()
  }

  onMounted(() => {
    document.addEventListener('pointerdown', handlePointerDown)
    document.addEventListener('keydown', handleKeydown)
  })

  onBeforeUnmount(() => {
    document.removeEventListener('pointerdown', handlePointerDown)
    document.removeEventListener('keydown', handleKeydown)
  })

  if (closeOnRouteChange) {
    const route = useRoute()
    watch(
      () => route.fullPath,
      () => {
        if (isOpen()) close()
      },
    )
  }
}
