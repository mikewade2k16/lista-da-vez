<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch, nextTick } from 'vue'
import CalendarChatComposer from '~/components/calendar/CalendarChatComposer.vue'
import { useVoiceRecorder } from '~/composables/useVoiceRecorder'
import { useLiveDictation } from '~/composables/useLiveDictation'
import { useAuthStore } from '~/stores/auth'
import { useFinanceAccountContext } from '../../composables/finance-account-context'
const props = defineProps<{
  value: string
  disabled: boolean
  sending?: boolean
  sendDisabled?: boolean
}>()
const emit = defineEmits<{ text: [value: string]; busy: [value: boolean]; send: []; attach: [] }>()
const auth = useAuthStore(),
  account = useFinanceAccountContext()
const voice = useVoiceRecorder(
    () => 'finance',
    () => account.activeAccountId,
  ),
  live = useLiveDictation()
const mode = useState<'whisper' | 'live'>('assistant-chat:voice-mode', () => 'whisper')
const scope = computed(
  () =>
    `${account.activeAccountId}:${auth.principal?.userId || auth.principal?.userID || auth.user?.id || ''}`,
)
const active = computed(() => voice.state.value !== 'idle' || live.state.value === 'listening')
const error = computed(() =>
  mode.value === 'live' ? live.errorMessage.value : voice.errorMessage.value,
)
const input = ref<HTMLTextAreaElement | null>(null)
const liveElapsed = ref(0)
let timer: ReturnType<typeof setInterval> | undefined
const elapsed = computed(() => (mode.value === 'live' ? liveElapsed.value : voice.elapsedMs.value))
const timeLabel = computed(
  () =>
    `${Math.floor(elapsed.value / 60000)}:${String(Math.floor(elapsed.value / 1000) % 60).padStart(2, '0')}`,
)
watch(
  () => live.state.value,
  (value) => {
    if (timer) clearInterval(timer)
    if (value === 'listening') {
      const start = Date.now()
      liveElapsed.value = 0
      timer = setInterval(() => (liveElapsed.value = Date.now() - start), 250)
    }
  },
)
function resize() {
  void nextTick(() => {
    if (input.value) {
      input.value.style.height = 'auto'
      input.value.style.height = `${Math.min(input.value.scrollHeight, 132)}px`
    }
  })
}
watch(() => props.value, resize)
function changeMode(value: 'whisper' | 'live') {
  cancel()
  mode.value = value
}
function enter(event: KeyboardEvent) {
  if (event.shiftKey || event.isComposing) return
  event.preventDefault()
  if (!active.value && !props.disabled && !props.sendDisabled) emit('send')
}
let base = '',
  origin = ''
function cancel() {
  voice.cancel()
  live.cancel()
}
async function toggle() {
  if (mode.value === 'live') {
    if (live.state.value === 'listening') live.stop()
    else {
      base = props.value
      origin = scope.value
      live.start()
    }
  } else if (voice.state.value === 'recording') {
    const ticket = scope.value
    const text = await voice.stopAndTranscribe()
    if (text && ticket === scope.value) emit('text', `${props.value} ${text}`.trim())
  } else if (voice.state.value === 'idle') await voice.start()
}
watch(
  () => live.transcript.value,
  (text) => {
    if (live.state.value === 'listening' && origin === scope.value)
      emit('text', `${base} ${text}`.trim())
  },
)
watch(active, (value) => emit('busy', value), { immediate: true })
watch(scope, cancel, { flush: 'sync' })
watch(
  () => props.disabled,
  (value) => {
    if (value) cancel()
  },
)
onBeforeUnmount(() => {
  cancel()
  if (timer) clearInterval(timer)
  emit('busy', false)
})
</script>
<template>
  <CalendarChatComposer
    :model-value="value"
    :voice-mode="mode"
    :is-capturing="voice.state.value === 'recording' || live.state.value === 'listening'"
    :is-transcribing="voice.state.value === 'transcribing'"
    :is-paused="voice.paused.value"
    :rec-time-label="timeLabel"
    :capture-levels="mode === 'live' ? live.levels.value : voice.levels.value"
    :voice-available="mode === 'live' ? live.supported : voice.supported"
    :voice-reason="
      mode === 'live'
        ? 'Ditado ao vivo indisponível neste navegador.'
        : 'Microfone indisponível neste navegador.'
    "
    :voice-error="error"
    :disabled="disabled"
    :sending="sending || sendDisabled || active"
    :maxlength="2000"
    attachment
    @update:model-value="emit('text', $event)"
    @input="resize"
    @input-element="input = $event"
    @enter="enter"
    @mode="changeMode"
    @mic="toggle"
    @pause="voice.paused.value ? voice.resume() : voice.pause()"
    @cancel="cancel"
    @send="emit('send')"
    @attach="emit('attach')"
  />
</template>
