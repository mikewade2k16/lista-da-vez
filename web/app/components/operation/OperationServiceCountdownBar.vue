<script setup>
import { computed, ref, watch } from 'vue'
import { formatOperationCountdown } from '~/domain/utils/time'

// Barra de auto-encerramento (2h): quando o backend abre o countdown, ele grava um
// graceDeadline (epoch ms de SERVIDOR) no atendimento. Esta barra e' so DISPLAY:
// encolhe comparando graceDeadline contra `now` (= adjustedNow server-anchored,
// NUNCA Date.now). Quem realmente fecha e' o sweep do backend. O botao "Continuar"
// NAO vive aqui — ele SUBSTITUI o botao "Encerrar" no card enquanto o aviso aparece.
const props = defineProps({
  graceDeadline: {
    type: Number,
    default: 0,
  },
  now: {
    type: Number,
    default: 0,
  },
  snoozeCount: {
    type: Number,
    default: 0,
  },
})

// Janela AUTO-MEDIDA: o snapshot carrega so o graceDeadline (instante do vencimento),
// nao a largura da janela configurada. Guardamos o maior "restante" ja visto para
// este graceDeadline para desenhar a % de encolhimento sem precisar da config no
// snapshot. Como o WS refetcha assim que o grace abre, a 1a leitura ja e ~janela cheia.
const windowMs = ref(0)

const remainingMs = computed(() =>
  Math.max(0, Number(props.graceDeadline || 0) - Number(props.now || 0)),
)

watch(
  () => props.graceDeadline,
  () => {
    windowMs.value = 0
  },
)

watch(
  remainingMs,
  (value) => {
    if (value > windowMs.value) {
      windowMs.value = value
    }
  },
  { immediate: true },
)

// Regra de tempo do modulo Operacao: acima de 60s conta minutos, acima de 60min
// conta horas (min+seg, hora+min...). Ver formatOperationCountdown.
const remainingLabel = computed(() => formatOperationCountdown(remainingMs.value))

const barStyle = computed(() => {
  const total = windowMs.value || 1
  const ratio = Math.max(0, Math.min(1, remainingMs.value / total))
  return { width: `${ratio * 100}%` }
})
</script>

<template>
  <div class="service-card__autoclose" data-testid="operation-autoclose-countdown">
    <div class="service-card__autoclose-head">
      <span class="material-icons-round service-card__autoclose-icon" aria-hidden="true">
        timer
      </span>
      <span class="service-card__autoclose-text">
        Encerrando automaticamente em {{ remainingLabel }}
        <template v-if="snoozeCount > 0">· adiado {{ snoozeCount }}x</template>
      </span>
    </div>
    <div class="service-card__autoclose-track" aria-hidden="true">
      <span class="service-card__autoclose-fill" :style="barStyle"></span>
    </div>
  </div>
</template>
