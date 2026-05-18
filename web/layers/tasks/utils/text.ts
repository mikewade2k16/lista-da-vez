// Utilitarios de texto compartilhados entre composables e stores do modulo tasks.
//
// `normalizeText` aplica trim + colapso de whitespaces (`\s+` -> ` `) + clamp por tamanho. Usar
// no flush/autosave e em strings que serao persistidas — o backend sempre faz TrimSpace de
// qualquer forma, mas normalizar antes evita gerar requests com payloads divergentes.
//
// `clampText` apenas faz `slice(0, max)` sem trim/colapso. Usar em `@update:model-value` de
// inputs controlados (`<UInput :model-value :update:model-value="...">`) — sem ele, o input
// "salta" quando o usuario digita espaco no final porque o `model-value` re-renderiza com o
// valor trimado a cada keystroke.
//
// Os dois helpers tambem ficam em `useTasksPageContext.ts` para compatibilidade com sub-componentes
// que ja consomem via inject, mas a implementacao canonica e' aqui — testaveis isoladamente.

export function normalizeText(value: unknown, max = 240): string {
  return String(value ?? '').replace(/\s+/g, ' ').trim().slice(0, max)
}

export function clampText(value: unknown, max = 240): string {
  return String(value ?? '').slice(0, max)
}
