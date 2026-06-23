import { ref } from 'vue'
import type { Ref } from 'vue'

// Drag-n-drop de lista com HTML5 nativo (sem dependencia externa — o projeto
// nao usa vuedraggable/sortablejs e nao deve passar a usar). Espelha a mecanica
// ja adotada em OmniTableColumnsConfig.vue: dataTransfer no dragstart,
// preventDefault no dragover (sem isso o drop nao dispara) e reordenacao no
// consumidor via splice.
//
// O composable so cuida do estado do arraste e de chamar onReorder(from, to)
// quando solta; quem possui a lista faz o splice/persistencia dentro de
// onReorder. Tudo agnostico ao tipo dos itens — a lista e referida por indice.
//
// Como o consumidor aplica (bind por item):
//   const { itemHandlers, draggingIndex, overIndex } = useSortableList({
//     onReorder(from, to) {
//       const next = [...lista.value]
//       const [moved] = next.splice(from, 1)
//       next.splice(to, 0, moved)
//       lista.value = next // persistir aqui
//     },
//   })
//   <div v-for="(item, i) in lista" :key="item.id" v-bind="itemHandlers(i)">
// `itemHandlers(i)` ja traz draggable + handlers + data/aria de realce, entao
// `v-bind` no elemento de cada item basta.

export interface SortableListOptions {
  // Chamado ao soltar um item arrastado sobre outro: `from` = indice de origem,
  // `to` = indice de destino. O consumidor faz o splice e persiste.
  onReorder: (from: number, to: number) => void
}

export interface SortableItemHandlers {
  draggable: true
  // Realce do item em arraste / sob o cursor — usados para estilizar via
  // [data-dragging] / [data-drag-over] e lidos por leitores de tela.
  'data-dragging': boolean
  'data-drag-over': boolean
  'aria-grabbed': boolean
  onDragstart: (event: DragEvent) => void
  onDragover: (event: DragEvent) => void
  onDrop: (event: DragEvent) => void
  onDragend: () => void
}

export interface UseSortableListReturn {
  draggingIndex: Ref<number | null>
  overIndex: Ref<number | null>
  itemHandlers: (index: number) => SortableItemHandlers
}

export function useSortableList(options: SortableListOptions): UseSortableListReturn {
  const draggingIndex = ref<number | null>(null)
  const overIndex = ref<number | null>(null)

  function reset(): void {
    draggingIndex.value = null
    overIndex.value = null
  }

  function onDragstart(index: number, event: DragEvent): void {
    draggingIndex.value = index
    if (event.dataTransfer) {
      event.dataTransfer.effectAllowed = 'move'
      // Alguns navegadores so iniciam o drag se ha payload em dataTransfer.
      event.dataTransfer.setData('text/plain', String(index))
    }
  }

  function onDragover(index: number, event: DragEvent): void {
    if (draggingIndex.value === null) return
    // Sem preventDefault o elemento nao e um alvo valido e o drop nao dispara.
    event.preventDefault()
    if (event.dataTransfer) event.dataTransfer.dropEffect = 'move'
    overIndex.value = index
  }

  function onDrop(index: number, event: DragEvent): void {
    const from = draggingIndex.value
    if (from === null || from === index) {
      reset()
      return
    }
    event.preventDefault()
    options.onReorder(from, index)
    reset()
  }

  function itemHandlers(index: number): SortableItemHandlers {
    return {
      draggable: true,
      'data-dragging': draggingIndex.value === index,
      'data-drag-over': overIndex.value === index && draggingIndex.value !== index,
      'aria-grabbed': draggingIndex.value === index,
      onDragstart: (event: DragEvent) => onDragstart(index, event),
      onDragover: (event: DragEvent) => onDragover(index, event),
      onDrop: (event: DragEvent) => onDrop(index, event),
      onDragend: reset,
    }
  }

  return { draggingIndex, overIndex, itemHandlers }
}
