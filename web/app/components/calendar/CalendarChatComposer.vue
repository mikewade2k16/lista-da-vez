<script setup lang="ts">
import { ref, watch } from 'vue'
withDefaults(
  defineProps<{
    modelValue: string
    voiceMode: 'whisper' | 'live'
    isCapturing: boolean
    isTranscribing: boolean
    isPaused: boolean
    recTimeLabel: string
    captureLevels: number[]
    voiceAvailable: boolean
    voiceReason: string
    voiceError: string
    disabled?: boolean
    sending?: boolean
    maxlength?: number
    attachment?: boolean
  }>(),
  { disabled: false, sending: false, maxlength: 4000, attachment: false },
)
const emit = defineEmits<{
  'update:modelValue': [value: string]
  input: []
  enter: [event: KeyboardEvent]
  mode: [value: 'whisper' | 'live']
  mic: []
  pause: []
  cancel: []
  send: []
  attach: []
  'input-element': [value: HTMLTextAreaElement | null]
}>()
const input = ref<HTMLTextAreaElement | null>(null)
watch(input, (value) => emit('input-element', value), { flush: 'post' })
const VOICE_MODES = [
  { value: 'whisper' as const, label: 'Whisper compartilhado' },
  { value: 'live' as const, label: 'Ao vivo' },
]
function changed(event: Event) {
  emit('update:modelValue', (event.target as HTMLTextAreaElement).value)
  emit('input')
}
</script>
<template>
  <p v-if="voiceError" class="calendar-chat__error" role="alert">
    {{ voiceError }}
  </p>
  <!-- Voz indisponivel: aviso honesto (so quando NAO capturando). -->
  <p
    v-if="!isCapturing && !voiceAvailable"
    class="calendar-chat__recording calendar-chat__recording--muted"
  >
    <UIcon name="i-lucide-mic-off" aria-hidden="true" />
    {{ voiceReason }}
  </p>
  <!-- Transcrevendo (apos parar no Whisper). -->
  <p v-else-if="isTranscribing" class="calendar-chat__recording" aria-live="polite">
    <UIcon name="i-lucide-loader-circle" class="calendar-chat__spin" aria-hidden="true" />
    Transcrevendo o áudio...
  </p>

  <!-- Toggle do modo de voz: escondido enquanto grava. -->
  <div v-if="!isCapturing" class="calendar-chat__voicemode" role="group" aria-label="Modo de voz">
    <button
      v-for="m in VOICE_MODES"
      :key="m.value"
      type="button"
      class="calendar-chat__voicemode-btn"
      :class="{ 'is-active': voiceMode === m.value }"
      :title="
        m.value === 'live'
          ? 'Ditado ao vivo pelo navegador (Chrome/Edge; audio passa pelo Google)'
          : 'Serviço Whisper compartilhado; transcreve ao parar'
      "
      @click="emit('mode', m.value)"
    >
      {{ m.label }}
    </button>
  </div>

  <!-- Barra de gravacao (estilo WhatsApp): lixeira + timer + waveform + pausar +
             confirmar. Substitui o input no Whisper; no modo ao vivo aparece ACIMA do
             input (o texto continua surgindo no campo enquanto a pessoa fala). -->
  <div v-if="isCapturing" class="calendar-chat__recbar" aria-live="polite">
    <button
      type="button"
      class="calendar-chat__recbtn calendar-chat__recbtn--trash"
      aria-label="Descartar gravação"
      title="Descartar"
      @click="emit('cancel')"
    >
      <UIcon name="i-lucide-trash-2" aria-hidden="true" />
    </button>

    <span class="calendar-chat__rectime">
      <span
        class="calendar-chat__rec-dot"
        :class="{ 'is-paused': isPaused }"
        aria-hidden="true"
      ></span>
      {{ recTimeLabel }}
    </span>

    <span class="calendar-chat__wave calendar-chat__wave--bar" aria-hidden="true">
      <span
        v-for="(lvl, i) in captureLevels"
        :key="i"
        :style="{ height: `${Math.round((0.12 + lvl * 0.88) * 100)}%` }"
      ></span>
    </span>

    <button
      v-if="voiceMode === 'whisper'"
      type="button"
      class="calendar-chat__recbtn"
      :aria-label="isPaused ? 'Retomar' : 'Pausar'"
      :title="isPaused ? 'Retomar' : 'Pausar'"
      @click="emit('pause')"
    >
      <UIcon :name="isPaused ? 'i-lucide-mic' : 'i-lucide-pause'" aria-hidden="true" />
    </button>

    <button
      type="button"
      class="calendar-chat__recbtn calendar-chat__recbtn--confirm"
      :disabled="isTranscribing"
      :aria-label="voiceMode === 'whisper' ? 'Parar e transcrever' : 'Concluir'"
      :title="voiceMode === 'whisper' ? 'Parar e transcrever' : 'Concluir'"
      @click="emit('mic')"
    >
      <UIcon name="i-lucide-check" aria-hidden="true" />
    </button>
  </div>

  <!-- Input: escondido no Whisper enquanto grava; no modo ao vivo continua visivel
             (o texto ditado aparece aqui). Mic/enviar somem durante a captura. -->
  <div v-if="!isCapturing || voiceMode === 'live'" class="calendar-chat__input">
    <button
      v-if="!isCapturing"
      type="button"
      class="calendar-chat__mic"
      :class="{ 'calendar-chat__mic--busy': isTranscribing }"
      :disabled="disabled || isTranscribing || !voiceAvailable"
      aria-label="Falar"
      :title="!voiceAvailable ? voiceReason : 'Falar'"
      @click="emit('mic')"
    >
      <UIcon
        v-if="isTranscribing"
        name="i-lucide-loader-circle"
        class="calendar-chat__spin"
        aria-hidden="true"
      />
      <UIcon v-else name="i-lucide-mic" aria-hidden="true" />
    </button>
    <button
      v-if="attachment && !isCapturing"
      type="button"
      class="calendar-chat__mic"
      aria-label="Anexar comprovante"
      title="Anexar comprovante"
      :disabled="disabled || sending"
      @click="emit('attach')"
    >
      <UIcon name="i-lucide-paperclip" />
    </button>
    <textarea
      ref="input"
      :value="modelValue"
      class="calendar-chat__field"
      rows="1"
      :placeholder="isCapturing ? 'Fale que o texto aparece aqui...' : 'Pergunte ao assistente...'"
      :disabled="disabled"
      :maxlength="maxlength"
      @input="changed"
      @keydown.enter="emit('enter', $event)"
    ></textarea>
    <button
      v-if="!isCapturing"
      type="button"
      class="calendar-chat__send"
      :disabled="disabled || sending || !modelValue.trim()"
      aria-label="Enviar"
      :title="'Enviar'"
      @click="emit('send')"
    >
      <UIcon name="i-lucide-send" aria-hidden="true" />
    </button>
  </div>
</template>
