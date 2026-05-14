<script setup lang="ts">
type GradientType = 'linear' | 'radial' | 'conic'

interface ParsedColor {
  hex: string
  alpha: number
  hasAlpha: boolean
}

interface ParsedGradient {
  type: GradientType
  angle: number
  start: ParsedColor
  end: ParsedColor
}

const props = withDefaults(
  defineProps<{
    modelValue?: string
    allowGradient?: boolean
  }>(),
  {
    modelValue: '',
    allowGradient: false
  }
)

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const gradientTypeItems = [
  { label: 'Linear', value: 'linear' },
  { label: 'Radial', value: 'radial' },
  { label: 'Conic', value: 'conic' }
]

const rawValue = ref('')
const internalUpdateInFlight = ref(false)

const gradientEnabled = ref(false)
const gradientType = ref<GradientType>('linear')
const gradientAngle = ref(180)

const solidHex = ref('#0A84FF')
const solidHexInput = ref('#0A84FF')
const solidAlphaEnabled = ref(false)
const solidAlpha = ref(100)

const gradientStartHex = ref('#0A84FF')
const gradientStartHexInput = ref('#0A84FF')
const gradientStartAlphaEnabled = ref(false)
const gradientStartAlpha = ref(100)

const gradientEndHex = ref('#3B82F6')
const gradientEndHexInput = ref('#3B82F6')
const gradientEndAlphaEnabled = ref(false)
const gradientEndAlpha = ref(100)

const screenPickerSupported = computed(() => import.meta.client && typeof (window as any).EyeDropper === 'function')

const previewBackground = computed(() => {
  if (gradientEnabled.value) {
    return buildGradientValue()
  }

  return buildCssColor(solidHex.value, solidAlphaEnabled.value, solidAlpha.value)
})

watch(
  () => props.modelValue,
  (value) => {
    const nextValue = toStringValue(value)
    if (nextValue === rawValue.value) {
      return
    }

    if (internalUpdateInFlight.value) {
      rawValue.value = nextValue
      return
    }

    syncFromModel(nextValue)
  },
  { immediate: true }
)

function toStringValue(value: unknown) {
  if (typeof value === 'string') return value
  if (typeof value === 'number') return String(value)
  return ''
}

function clampPercent(value: number) {
  return Math.max(0, Math.min(100, Math.round(value)))
}

function clampAngle(value: number) {
  if (!Number.isFinite(value)) {
    return 180
  }

  const normalized = value % 360
  return normalized < 0 ? normalized + 360 : normalized
}

function normalizeHex(hex: string) {
  const cleaned = hex.trim().replace('#', '').replace(/[^0-9a-fA-F]/g, '')
  if (![3, 4, 6, 8].includes(cleaned.length)) {
    return null
  }

  let expanded = cleaned
  if (cleaned.length === 3 || cleaned.length === 4) {
    expanded = cleaned.split('').map(char => `${char}${char}`).join('')
  }

  const noAlpha = expanded.slice(0, 6)
  return `#${noAlpha.toUpperCase()}`
}

function parseHexColor(value: string): ParsedColor | null {
  const cleaned = value.trim().replace('#', '').replace(/[^0-9a-fA-F]/g, '')
  if (![3, 4, 6, 8].includes(cleaned.length)) {
    return null
  }

  let expanded = cleaned
  if (cleaned.length === 3 || cleaned.length === 4) {
    expanded = cleaned.split('').map(char => `${char}${char}`).join('')
  }

  const rgbPart = expanded.slice(0, 6)
  const alphaPart = expanded.length === 8 ? expanded.slice(6, 8) : null
  const alpha = alphaPart ? clampPercent((Number.parseInt(alphaPart, 16) / 255) * 100) : 100

  return {
    hex: `#${rgbPart.toUpperCase()}`,
    alpha,
    hasAlpha: Boolean(alphaPart)
  }
}

function parseRgbChannel(token: string) {
  const trimmed = token.trim()
  if (!trimmed) {
    return null
  }

  if (trimmed.endsWith('%')) {
    const parsedPercent = Number.parseFloat(trimmed.slice(0, -1))
    if (Number.isNaN(parsedPercent)) {
      return null
    }

    return Math.round(Math.max(0, Math.min(100, parsedPercent)) * 2.55)
  }

  const parsed = Number.parseFloat(trimmed)
  if (Number.isNaN(parsed)) {
    return null
  }

  return Math.round(Math.max(0, Math.min(255, parsed)))
}

function parseAlphaChannel(token: string | undefined) {
  if (!token) {
    return { alpha: 100, hasAlpha: false }
  }

  const trimmed = token.trim()
  if (!trimmed) {
    return { alpha: 100, hasAlpha: false }
  }

  if (trimmed.endsWith('%')) {
    const parsedPercent = Number.parseFloat(trimmed.slice(0, -1))
    if (Number.isNaN(parsedPercent)) {
      return { alpha: 100, hasAlpha: false }
    }

    return {
      alpha: clampPercent(parsedPercent),
      hasAlpha: true
    }
  }

  const parsed = Number.parseFloat(trimmed)
  if (Number.isNaN(parsed)) {
    return { alpha: 100, hasAlpha: false }
  }

  if (parsed <= 1) {
    return {
      alpha: clampPercent(parsed * 100),
      hasAlpha: true
    }
  }

  return {
    alpha: clampPercent(parsed),
    hasAlpha: true
  }
}

function channelsToHex(r: number, g: number, b: number) {
  return `#${[r, g, b].map(channel => channel.toString(16).padStart(2, '0')).join('').toUpperCase()}`
}

function hexToRgbChannels(hex: string) {
  const normalized = normalizeHex(hex)
  if (!normalized) {
    return null
  }

  const parsed = normalized.replace('#', '')
  const r = Number.parseInt(parsed.slice(0, 2), 16)
  const g = Number.parseInt(parsed.slice(2, 4), 16)
  const b = Number.parseInt(parsed.slice(4, 6), 16)

  return [r, g, b] as const
}

function resolveRgbVarColor(value: string): ParsedColor | null {
  if (!import.meta.client) {
    return null
  }

  const varRgbMatch = value.trim().match(/^rgb\(\s*var\((--[^)]+)\)\s*(?:\/\s*([^)]+))?\)$/i)
  if (!varRgbMatch) {
    return null
  }

  const varName = (varRgbMatch[1] ?? '').trim()
  if (!varName) {
    return null
  }

  const alphaToken = (varRgbMatch[2] ?? '').trim() || undefined
  const resolvedVarValue = getComputedStyle(document.documentElement).getPropertyValue(varName).trim()
  if (!resolvedVarValue) {
    return null
  }

  const channelTokens = resolvedVarValue.split(/\s+/).map(token => token.trim()).filter(Boolean)
  if (channelTokens.length < 3) {
    return null
  }

  const r = parseRgbChannel(channelTokens[0] ?? '')
  const g = parseRgbChannel(channelTokens[1] ?? '')
  const b = parseRgbChannel(channelTokens[2] ?? '')
  if (r === null || g === null || b === null) {
    return null
  }

  const parsedAlpha = parseAlphaChannel(alphaToken)
  return {
    hex: channelsToHex(r, g, b),
    alpha: parsedAlpha.alpha,
    hasAlpha: parsedAlpha.hasAlpha
  }
}

function parseCssColor(value: string): ParsedColor | null {
  const normalizedValue = value.trim()
  if (!normalizedValue) {
    return null
  }

  if (normalizedValue.includes('var(')) {
    const resolvedVarColor = resolveRgbVarColor(normalizedValue)
    if (resolvedVarColor) {
      return resolvedVarColor
    }

    return null
  }

  const parsedHex = parseHexColor(normalizedValue)
  if (parsedHex) {
    return parsedHex
  }

  const rgbMatch = normalizedValue.match(/^rgba?\((.*)\)$/i)
  if (!rgbMatch) {
    return null
  }

  const body = (rgbMatch[1] ?? '').trim()
  if (!body) {
    return null
  }

  let alphaToken: string | undefined
  let rgbTokens: string[] = []

  if (body.includes('/')) {
    const [rawRgb = '', rawAlpha = ''] = body.split('/').map(part => part.trim())
    alphaToken = rawAlpha
    rgbTokens = rawRgb.includes(',')
      ? rawRgb.split(',').map(token => token.trim()).filter(Boolean)
      : rawRgb.split(/\s+/).map(token => token.trim()).filter(Boolean)
  } else if (body.includes(',')) {
    const parts = body.split(',').map(token => token.trim()).filter(Boolean)
    rgbTokens = parts.slice(0, 3)
    alphaToken = parts[3]
  } else {
    const parts = body.split(/\s+/).map(token => token.trim()).filter(Boolean)
    rgbTokens = parts.slice(0, 3)
    alphaToken = parts[3]
  }

  if (rgbTokens.length < 3) {
    return null
  }

  const r = parseRgbChannel(rgbTokens[0] ?? '')
  const g = parseRgbChannel(rgbTokens[1] ?? '')
  const b = parseRgbChannel(rgbTokens[2] ?? '')
  if (r === null || g === null || b === null) {
    return null
  }

  const parsedAlpha = parseAlphaChannel(alphaToken)

  return {
    hex: channelsToHex(r, g, b),
    alpha: parsedAlpha.alpha,
    hasAlpha: parsedAlpha.hasAlpha
  }
}

function splitByTopLevelComma(value: string) {
  const result: string[] = []
  let current = ''
  let depth = 0

  for (const char of value) {
    if (char === '(') {
      depth += 1
      current += char
      continue
    }

    if (char === ')') {
      depth = Math.max(0, depth - 1)
      current += char
      continue
    }

    if (char === ',' && depth === 0) {
      if (current.trim()) {
        result.push(current.trim())
      }

      current = ''
      continue
    }

    current += char
  }

  if (current.trim()) {
    result.push(current.trim())
  }

  return result
}

function extractGradientColorToken(token: string) {
  const trimmed = token.trim()
  if (!trimmed) {
    return ''
  }

  if (trimmed.includes(')')) {
    const closeIndex = trimmed.lastIndexOf(')')
    return trimmed.slice(0, closeIndex + 1).trim()
  }

  return trimmed.split(/\s+/)[0] ?? ''
}

function parseAngleToken(value: string) {
  const token = value.trim().toLowerCase()
  if (!token) {
    return 180
  }

  const directionMap: Record<string, number> = {
    'to top': 0,
    'to right': 90,
    'to bottom': 180,
    'to left': 270,
    'to top right': 45,
    'to right top': 45,
    'to bottom right': 135,
    'to right bottom': 135,
    'to bottom left': 225,
    'to left bottom': 225,
    'to top left': 315,
    'to left top': 315
  }

  if (directionMap[token] !== undefined) {
    return directionMap[token]
  }

  if (token.endsWith('deg')) {
    return clampAngle(Number.parseFloat(token.slice(0, -3)))
  }

  if (token.endsWith('turn')) {
    return clampAngle(Number.parseFloat(token.slice(0, -4)) * 360)
  }

  if (token.endsWith('rad')) {
    return clampAngle((Number.parseFloat(token.slice(0, -3)) * 180) / Math.PI)
  }

  return 180
}

function parseGradient(value: string): ParsedGradient | null {
  const gradientMatch = value.trim().match(/^(linear|radial|conic)-gradient\((.*)\)$/i)
  if (!gradientMatch) {
    return null
  }

  const type = (gradientMatch[1] || 'linear').toLowerCase() as GradientType
  const args = splitByTopLevelComma(gradientMatch[2] || '')
  if (args.length < 2) {
    return null
  }

  let angle = 180
  let colorArgs = [...args]
  const firstArg = (args[0] || '').trim()

  if (type === 'linear' || type === 'conic') {
    const hasAngle = /\b(deg|turn|rad)\b/i.test(firstArg) || firstArg.startsWith('to ')
    if (hasAngle) {
      angle = parseAngleToken(firstArg)
      colorArgs = args.slice(1)
    }
  } else if (type === 'radial') {
    const firstColor = parseCssColor(extractGradientColorToken(firstArg))
    if (!firstColor) {
      colorArgs = args.slice(1)
    }
  }

  if (colorArgs.length < 2) {
    return null
  }

  const start = parseCssColor(extractGradientColorToken(colorArgs[0] || '')) ?? {
    hex: '#0A84FF',
    alpha: 100,
    hasAlpha: false
  }
  const end = parseCssColor(extractGradientColorToken(colorArgs[1] || '')) ?? {
    hex: '#3B82F6',
    alpha: 100,
    hasAlpha: false
  }

  return {
    type,
    angle,
    start,
    end
  }
}

function buildCssColor(hex: string, alphaEnabled: boolean, alpha: number) {
  const normalized = normalizeHex(hex)
  if (!normalized) {
    return '#000000'
  }

  if (!alphaEnabled) {
    return normalized
  }

  const channels = hexToRgbChannels(normalized)
  if (!channels) {
    return normalized
  }

  const [r, g, b] = channels
  const alphaFraction = Math.max(0, Math.min(1, alpha / 100))
  const alphaValue = Number(alphaFraction.toFixed(2))
  return `rgba(${r}, ${g}, ${b}, ${alphaValue})`
}

function buildGradientValue() {
  const startColor = buildCssColor(gradientStartHex.value, gradientStartAlphaEnabled.value, gradientStartAlpha.value)
  const endColor = buildCssColor(gradientEndHex.value, gradientEndAlphaEnabled.value, gradientEndAlpha.value)
  const angle = Math.round(clampAngle(gradientAngle.value))

  if (gradientType.value === 'radial') {
    return `radial-gradient(circle, ${startColor}, ${endColor})`
  }

  if (gradientType.value === 'conic') {
    return `conic-gradient(from ${angle}deg, ${startColor}, ${endColor})`
  }

  return `linear-gradient(${angle}deg, ${startColor}, ${endColor})`
}

function emitValue(nextValue: string) {
  if (nextValue === rawValue.value) {
    return
  }

  internalUpdateInFlight.value = true
  rawValue.value = nextValue
  emit('update:modelValue', nextValue)

  queueMicrotask(() => {
    internalUpdateInFlight.value = false
  })
}

function applySolidControls() {
  emitValue(buildCssColor(solidHex.value, solidAlphaEnabled.value, solidAlpha.value))
}

function applyGradientControls() {
  emitValue(buildGradientValue())
}

function syncFromModel(value: string) {
  rawValue.value = value

  if (props.allowGradient) {
    const parsedGradient = parseGradient(value)
    if (parsedGradient) {
      gradientEnabled.value = true
      gradientType.value = parsedGradient.type
      gradientAngle.value = clampAngle(parsedGradient.angle)

      gradientStartHex.value = parsedGradient.start.hex
      gradientStartHexInput.value = parsedGradient.start.hex
      gradientStartAlpha.value = parsedGradient.start.alpha
      gradientStartAlphaEnabled.value = parsedGradient.start.hasAlpha

      gradientEndHex.value = parsedGradient.end.hex
      gradientEndHexInput.value = parsedGradient.end.hex
      gradientEndAlpha.value = parsedGradient.end.alpha
      gradientEndAlphaEnabled.value = parsedGradient.end.hasAlpha
      return
    }
  }

  gradientEnabled.value = false

  const parsedColor = parseCssColor(value)
  if (!parsedColor) {
    solidHex.value = '#0A84FF'
    solidHexInput.value = '#0A84FF'
    solidAlpha.value = 100
    solidAlphaEnabled.value = false
    return
  }

  solidHex.value = parsedColor.hex
  solidHexInput.value = parsedColor.hex
  solidAlpha.value = parsedColor.alpha
  solidAlphaEnabled.value = parsedColor.hasAlpha
}

function onRawValueInput(value: string | number | undefined) {
  const nextValue = toStringValue(value)
  if (nextValue === rawValue.value) {
    return
  }
  emitValue(nextValue)
  syncFromModel(nextValue)
}

function onSolidPickerChange(value: string | undefined) {
  const normalized = normalizeHex(value || '')
  if (!normalized) {
    return
  }

  solidHex.value = normalized
  solidHexInput.value = normalized
  applySolidControls()
}

function onSolidHexInput(value: string | number | undefined) {
  const nextHex = toStringValue(value)
  solidHexInput.value = nextHex

  const normalized = normalizeHex(nextHex)
  if (!normalized) {
    return
  }

  solidHex.value = normalized
  applySolidControls()
}

function onSolidAlphaToggle(value: boolean | 'indeterminate') {
  solidAlphaEnabled.value = value === true
  applySolidControls()
}

function toNumberValue(value: number | number[] | string | undefined, fallback = 0) {
  if (Array.isArray(value)) {
    const firstValue = value[0]
    return Number.isFinite(firstValue) ? Number(firstValue) : fallback
  }

  if (typeof value === 'number') {
    return Number.isFinite(value) ? value : fallback
  }

  const parsed = Number.parseFloat(toStringValue(value))
  return Number.isFinite(parsed) ? parsed : fallback
}

function onSolidAlphaSlider(value: number | number[] | undefined) {
  solidAlpha.value = clampPercent(toNumberValue(value, solidAlpha.value))
  applySolidControls()
}

function onSolidAlphaInput(value: string | number | undefined) {
  solidAlpha.value = clampPercent(toNumberValue(value, solidAlpha.value))
  applySolidControls()
}

function onGradientToggle(value: boolean | 'indeterminate') {
  gradientEnabled.value = value === true
  if (!gradientEnabled.value) {
    applySolidControls()
    return
  }

  applyGradientControls()
}

function onGradientTypeChange(value: string | number | undefined) {
  const nextType = toStringValue(value) as GradientType
  if (!['linear', 'radial', 'conic'].includes(nextType)) {
    return
  }

  gradientType.value = nextType
  applyGradientControls()
}

function onGradientAngleSlider(value: number | number[] | undefined) {
  gradientAngle.value = clampAngle(toNumberValue(value, gradientAngle.value))
  applyGradientControls()
}

function onGradientAngleInput(value: string | number | undefined) {
  gradientAngle.value = clampAngle(toNumberValue(value, gradientAngle.value))
  applyGradientControls()
}

function onGradientStartPickerChange(value: string | undefined) {
  const normalized = normalizeHex(value || '')
  if (!normalized) {
    return
  }

  gradientStartHex.value = normalized
  gradientStartHexInput.value = normalized
  applyGradientControls()
}

function onGradientStartHexInput(value: string | number | undefined) {
  const nextHex = toStringValue(value)
  gradientStartHexInput.value = nextHex

  const normalized = normalizeHex(nextHex)
  if (!normalized) {
    return
  }

  gradientStartHex.value = normalized
  applyGradientControls()
}

function onGradientStartAlphaToggle(value: boolean | 'indeterminate') {
  gradientStartAlphaEnabled.value = value === true
  applyGradientControls()
}

function onGradientStartAlphaSlider(value: number | number[] | undefined) {
  gradientStartAlpha.value = clampPercent(toNumberValue(value, gradientStartAlpha.value))
  applyGradientControls()
}

function onGradientStartAlphaInput(value: string | number | undefined) {
  gradientStartAlpha.value = clampPercent(toNumberValue(value, gradientStartAlpha.value))
  applyGradientControls()
}

function onGradientEndPickerChange(value: string | undefined) {
  const normalized = normalizeHex(value || '')
  if (!normalized) {
    return
  }

  gradientEndHex.value = normalized
  gradientEndHexInput.value = normalized
  applyGradientControls()
}

function onGradientEndHexInput(value: string | number | undefined) {
  const nextHex = toStringValue(value)
  gradientEndHexInput.value = nextHex

  const normalized = normalizeHex(nextHex)
  if (!normalized) {
    return
  }

  gradientEndHex.value = normalized
  applyGradientControls()
}

function onGradientEndAlphaToggle(value: boolean | 'indeterminate') {
  gradientEndAlphaEnabled.value = value === true
  applyGradientControls()
}

function onGradientEndAlphaSlider(value: number | number[] | undefined) {
  gradientEndAlpha.value = clampPercent(toNumberValue(value, gradientEndAlpha.value))
  applyGradientControls()
}

function onGradientEndAlphaInput(value: string | number | undefined) {
  gradientEndAlpha.value = clampPercent(toNumberValue(value, gradientEndAlpha.value))
  applyGradientControls()
}

async function pickScreenColor(target: 'solid' | 'start' | 'end' = 'solid') {
  if (!import.meta.client) {
    return
  }

  const EyeDropperCtor = (window as any).EyeDropper
  if (typeof EyeDropperCtor !== 'function') {
    return
  }

  try {
    const picker = new EyeDropperCtor()
    const result = await picker.open()
    const normalized = normalizeHex(result?.sRGBHex || '')
    if (!normalized) {
      return
    }

    if (target === 'start') {
      gradientStartHex.value = normalized
      gradientStartHexInput.value = normalized
      applyGradientControls()
      return
    }

    if (target === 'end') {
      gradientEndHex.value = normalized
      gradientEndHexInput.value = normalized
      applyGradientControls()
      return
    }

    solidHex.value = normalized
    solidHexInput.value = normalized
    applySolidControls()
  } catch {
    // User cancelled the picker.
  }
}
</script>

<template>
  <div class="space-y-1">
    <div class="flex items-center gap-2">
      <UPopover :content="{ align: 'start', side: 'bottom' }">
        <UButton color="neutral" variant="outline" class="h-8 px-2">
          <template #leading>
            <span class="size-4 rounded border border-[rgb(var(--border))]" :style="{ background: previewBackground }" />
          </template>
          <span class="hidden text-xs sm:inline">Cor</span>
          <template #trailing>
            <UIcon name="i-lucide-pipette" class="size-4" />
          </template>
        </UButton>

        <template #content>
          <div
            class="space-y-3 p-3"
            :class="gradientEnabled ? 'w-[440px] max-w-[96vw]' : 'w-[320px] max-w-[92vw]'"
          >
            <div v-if="allowGradient" class="flex items-center justify-between gap-2">
              <div class="flex items-center gap-2">
                <USwitch :model-value="gradientEnabled" @update:model-value="onGradientToggle" />
                <span class="text-xs font-medium text-[rgb(var(--muted))]">Usar gradiente</span>
              </div>
              <span class="text-xs text-[rgb(var(--muted))]">
                {{ gradientEnabled ? 'Gradiente' : 'Cor solida' }}
              </span>
            </div>

            <div v-if="!gradientEnabled" class="space-y-2">
              <div class="flex items-center gap-2">
                <UColorPicker :model-value="solidHex" size="sm" @update:model-value="onSolidPickerChange" />
                <UInput
                  :model-value="solidHexInput"
                  placeholder="#0A84FF"
                  class="w-full"
                  @update:model-value="onSolidHexInput"
                />
                <UButton
                  v-if="screenPickerSupported"
                  icon="i-lucide-pipette"
                  color="neutral"
                  variant="outline"
                  size="sm"
                  @click="pickScreenColor('solid')"
                />
              </div>

              <div class="flex items-center gap-2 rounded-[var(--radius-sm)] border border-[rgb(var(--border))] px-2 py-1">
                <USwitch :model-value="solidAlphaEnabled" @update:model-value="onSolidAlphaToggle" />
                <span class="text-xs text-[rgb(var(--muted))]">Transparencia</span>
              </div>

              <div v-if="solidAlphaEnabled" class="flex items-center gap-2">
                <USlider
                  :model-value="solidAlpha"
                  :min="0"
                  :max="100"
                  :step="1"
                  class="w-full"
                  @update:model-value="onSolidAlphaSlider"
                />
                <UInput
                  :model-value="String(solidAlpha)"
                  type="number"
                  min="0"
                  max="100"
                  class="w-20"
                  @update:model-value="onSolidAlphaInput"
                />
              </div>
            </div>

            <div v-else class="space-y-3">
              <div class="grid gap-2 sm:grid-cols-[140px_1fr] sm:items-center">
                <USelect
                  :model-value="gradientType"
                  :items="gradientTypeItems"
                  class="w-full"
                  @update:model-value="onGradientTypeChange"
                />

                <div v-if="gradientType !== 'radial'" class="flex items-center gap-2">
                  <USlider
                    :model-value="gradientAngle"
                    :min="0"
                    :max="360"
                    :step="1"
                    class="w-full"
                    @update:model-value="onGradientAngleSlider"
                  />
                  <UInput
                    :model-value="String(Math.round(gradientAngle))"
                    type="number"
                    min="0"
                    max="360"
                    class="w-14"
                    @update:model-value="onGradientAngleInput"
                  />
                </div>
              </div>

              <div class="grid gap-2 md:grid-cols-2">
                <div class="space-y-2 rounded-[var(--radius-sm)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] p-1.5">
                  <p class="text-xs font-medium text-[rgb(var(--muted))]">Cor 1</p>
                  <div class="flex justify-start">
                    <UColorPicker :model-value="gradientStartHex" size="xs" @update:model-value="onGradientStartPickerChange" />
                  </div>
                  <div class="flex items-center gap-2">
                    <UInput
                      :model-value="gradientStartHexInput"
                      placeholder="#0A84FF"
                      class="w-full min-w-0"
                      @update:model-value="onGradientStartHexInput"
                    />
                    <UButton
                      v-if="screenPickerSupported"
                      icon="i-lucide-pipette"
                      color="neutral"
                      variant="outline"
                      size="sm"
                      @click="pickScreenColor('start')"
                    />
                  </div>

                  <div class="flex items-center gap-2 rounded-[var(--radius-sm)] border border-[rgb(var(--border))] px-2 py-1">
                    <USwitch :model-value="gradientStartAlphaEnabled" @update:model-value="onGradientStartAlphaToggle" />
                    <span class="text-xs text-[rgb(var(--muted))]">Transparencia</span>
                  </div>

                  <div v-if="gradientStartAlphaEnabled" class="flex items-center gap-2">
                    <USlider
                      :model-value="gradientStartAlpha"
                      :min="0"
                      :max="100"
                      :step="1"
                      class="w-full"
                      @update:model-value="onGradientStartAlphaSlider"
                    />
                    <UInput
                      :model-value="String(gradientStartAlpha)"
                      type="number"
                      min="0"
                      max="100"
                      class="w-10"
                      @update:model-value="onGradientStartAlphaInput"
                    />
                  </div>
                </div>

                <div class="space-y-2 rounded-[var(--radius-sm)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] p-1.5">
                  <p class="text-xs font-medium text-[rgb(var(--muted))]">Cor 2</p>
                  <div class="flex justify-start">
                    <UColorPicker :model-value="gradientEndHex" size="xs" @update:model-value="onGradientEndPickerChange" />
                  </div>
                  <div class="flex items-center gap-2">
                    <UInput
                      :model-value="gradientEndHexInput"
                      placeholder="#3B82F6"
                      class="w-full min-w-0"
                      @update:model-value="onGradientEndHexInput"
                    />
                    <UButton
                      v-if="screenPickerSupported"
                      icon="i-lucide-pipette"
                      color="neutral"
                      variant="outline"
                      size="sm"
                      @click="pickScreenColor('end')"
                    />
                  </div>

                  <div class="flex items-center gap-2 rounded-[var(--radius-sm)] border border-[rgb(var(--border))] px-2 py-1">
                    <USwitch :model-value="gradientEndAlphaEnabled" @update:model-value="onGradientEndAlphaToggle" />
                    <span class="text-xs text-[rgb(var(--muted))]">Transparencia</span>
                  </div>

                  <div v-if="gradientEndAlphaEnabled" class="flex items-center gap-2">
                    <USlider
                      :model-value="gradientEndAlpha"
                      :min="0"
                      :max="100"
                      :step="1"
                      class="w-full"
                      @update:model-value="onGradientEndAlphaSlider"
                    />
                    <UInput
                      :model-value="String(gradientEndAlpha)"
                      type="number"
                      min="0"
                      max="100"
                      class="w-10"
                      @update:model-value="onGradientEndAlphaInput"
                    />
                  </div>
                </div>
              </div>
            </div>

            <div class="space-y-1">
              <p class="text-[11px] text-[rgb(var(--muted))]">Valor CSS final</p>
              <UInput
                :model-value="rawValue"
                placeholder="Ex: #0A84FF, rgba(...), linear-gradient(...)"
                @update:model-value="onRawValueInput"
              />
            </div>
          </div>
        </template>
      </UPopover>

      <UInput
        :model-value="rawValue"
        placeholder="Valor CSS da cor"
        class="w-full"
        @update:model-value="onRawValueInput"
      />
    </div>
  </div>
</template>
