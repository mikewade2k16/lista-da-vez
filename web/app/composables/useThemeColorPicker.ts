import { computed, ref, watch, type Ref } from 'vue'
import {
  buildCssColor,
  buildGradientValue,
  clampAngle,
  clampPercent,
  normalizeHex,
  parseCssColor,
  parseGradient,
  type GradientType,
} from '~/domain/utils/color'

type ColorTarget = 'solid' | 'start' | 'end'

interface EyeDropperResult {
  sRGBHex?: string
}

interface EyeDropperInstance {
  open: () => Promise<EyeDropperResult>
}

interface EyeDropperConstructor {
  new (): EyeDropperInstance
}

interface UseThemeColorPickerOptions {
  value: Ref<string | number | undefined>
  allowGradient: Ref<boolean>
  apply: (value: string) => void
}

const gradientTypeItems = [
  { label: 'Linear', value: 'linear' },
  { label: 'Radial', value: 'radial' },
  { label: 'Conic', value: 'conic' },
]

export function useThemeColorPicker(options: UseThemeColorPickerOptions) {
  const currentValue = ref('')
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

  const screenPickerSupported = computed(() => Boolean(getEyeDropperCtor()))

  const previewBackground = computed(() => {
    if (gradientEnabled.value) {
      return buildCurrentGradient()
    }

    return buildCssColor(solidHex.value, solidAlphaEnabled.value, solidAlpha.value)
  })

  watch(options.value, (value) => syncFromValue(toStringValue(value)), { immediate: true })

  function toStringValue(value: unknown) {
    if (typeof value === 'string') return value
    if (typeof value === 'number') return String(value)
    return ''
  }

  function getEyeDropperCtor() {
    if (!import.meta.client) {
      return null
    }

    const browserWindow = window as Window & { EyeDropper?: EyeDropperConstructor }
    return typeof browserWindow.EyeDropper === 'function' ? browserWindow.EyeDropper : null
  }

  function resolveRootCssVariable(name: string) {
    if (!import.meta.client) {
      return ''
    }

    return getComputedStyle(document.documentElement).getPropertyValue(name).trim()
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

  function buildCurrentGradient() {
    return buildGradientValue({
      type: gradientType.value,
      angle: gradientAngle.value,
      start: {
        hex: gradientStartHex.value,
        alphaEnabled: gradientStartAlphaEnabled.value,
        alpha: gradientStartAlpha.value,
      },
      end: {
        hex: gradientEndHex.value,
        alphaEnabled: gradientEndAlphaEnabled.value,
        alpha: gradientEndAlpha.value,
      },
    })
  }

  function applyValue(nextValue: string) {
    currentValue.value = nextValue
    options.apply(nextValue)
  }

  function applySolidControls() {
    applyValue(buildCssColor(solidHex.value, solidAlphaEnabled.value, solidAlpha.value))
  }

  function applyGradientControls() {
    applyValue(buildCurrentGradient())
  }

  function syncFromValue(value: string) {
    currentValue.value = value

    if (options.allowGradient.value) {
      const parsedGradient = parseGradient(value, resolveRootCssVariable)
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

    const parsedColor = parseCssColor(value, resolveRootCssVariable)
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

  function onFinalValueInput(value: string | number | undefined) {
    const nextValue = toStringValue(value)
    applyValue(nextValue)
    syncFromValue(nextValue)
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

  function onGradientColorPickerChange(
    target: Exclude<ColorTarget, 'solid'>,
    value: string | undefined,
  ) {
    const normalized = normalizeHex(value || '')
    if (!normalized) {
      return
    }

    if (target === 'start') {
      gradientStartHex.value = normalized
      gradientStartHexInput.value = normalized
    } else {
      gradientEndHex.value = normalized
      gradientEndHexInput.value = normalized
    }

    applyGradientControls()
  }

  function onGradientHexInput(
    target: Exclude<ColorTarget, 'solid'>,
    value: string | number | undefined,
  ) {
    const nextHex = toStringValue(value)
    const normalized = normalizeHex(nextHex)

    if (target === 'start') {
      gradientStartHexInput.value = nextHex
      if (normalized) {
        gradientStartHex.value = normalized
        applyGradientControls()
      }
      return
    }

    gradientEndHexInput.value = nextHex
    if (normalized) {
      gradientEndHex.value = normalized
      applyGradientControls()
    }
  }

  function onGradientAlphaToggle(
    target: Exclude<ColorTarget, 'solid'>,
    value: boolean | 'indeterminate',
  ) {
    if (target === 'start') {
      gradientStartAlphaEnabled.value = value === true
    } else {
      gradientEndAlphaEnabled.value = value === true
    }

    applyGradientControls()
  }

  function onGradientAlphaSlider(
    target: Exclude<ColorTarget, 'solid'>,
    value: number | number[] | undefined,
  ) {
    if (target === 'start') {
      gradientStartAlpha.value = clampPercent(toNumberValue(value, gradientStartAlpha.value))
    } else {
      gradientEndAlpha.value = clampPercent(toNumberValue(value, gradientEndAlpha.value))
    }

    applyGradientControls()
  }

  function onGradientAlphaInput(
    target: Exclude<ColorTarget, 'solid'>,
    value: string | number | undefined,
  ) {
    if (target === 'start') {
      gradientStartAlpha.value = clampPercent(toNumberValue(value, gradientStartAlpha.value))
    } else {
      gradientEndAlpha.value = clampPercent(toNumberValue(value, gradientEndAlpha.value))
    }

    applyGradientControls()
  }

  async function pickScreenColor(target: ColorTarget = 'solid') {
    const EyeDropperCtor = getEyeDropperCtor()
    if (!EyeDropperCtor) {
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

  return {
    currentValue,
    gradientEnabled,
    gradientType,
    gradientTypeItems,
    gradientAngle,
    solidHex,
    solidHexInput,
    solidAlphaEnabled,
    solidAlpha,
    gradientStartHex,
    gradientStartHexInput,
    gradientStartAlphaEnabled,
    gradientStartAlpha,
    gradientEndHex,
    gradientEndHexInput,
    gradientEndAlphaEnabled,
    gradientEndAlpha,
    screenPickerSupported,
    previewBackground,
    onFinalValueInput,
    onSolidPickerChange,
    onSolidHexInput,
    onSolidAlphaToggle,
    onSolidAlphaSlider,
    onSolidAlphaInput,
    onGradientToggle,
    onGradientTypeChange,
    onGradientAngleSlider,
    onGradientAngleInput,
    onGradientColorPickerChange,
    onGradientHexInput,
    onGradientAlphaToggle,
    onGradientAlphaSlider,
    onGradientAlphaInput,
    pickScreenColor,
  }
}
