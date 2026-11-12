import { onBeforeUnmount, onMounted, ref } from 'vue'

// Media query reativa (SSR-safe: false ate montar no cliente). Usada para trocar
// arvores de componente inteiras (ex.: tabela x cards), onde CSS nao basta.
export function useMediaMatch(query: string) {
  const matches = ref(false)
  let media: MediaQueryList | null = null
  const update = () => {
    matches.value = Boolean(media?.matches)
  }
  onMounted(() => {
    media = window.matchMedia(query)
    update()
    media.addEventListener('change', update)
  })
  onBeforeUnmount(() => media?.removeEventListener('change', update))
  return matches
}
