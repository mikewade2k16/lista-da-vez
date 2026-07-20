<script setup lang="ts">
// Janela de chat do Calendario (SPEC-F2/F10, CHATUI/D3): abre CENTRALIZADA (Teleport),
// MINIMIZAR/FECHAR/resize (useCalendarChatWindow), menu "Conversas" + escopo (wave 4).
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import CalendarChatConversations from '~/components/calendar/CalendarChatConversations.vue'
import CalendarChatMessage from '~/components/calendar/CalendarChatMessage.vue'
import CalendarChatScope from '~/components/calendar/CalendarChatScope.vue'
import { useCalendarChat } from '~/composables/useCalendarChat'
import { useCalendarChatWindow } from '~/composables/useCalendarChatWindow'
import { useCalendarShortcuts } from '~/composables/useCalendarShortcuts'
import { useVoiceRecorder } from '~/composables/useVoiceRecorder'
import { useLiveDictation } from '~/composables/useLiveDictation'
import { useCalendarStore } from '~/stores/calendar'
import type { CalendarChatPosition } from '~/utils/calendar'

const chat = useCalendarChat()
const voice = useVoiceRecorder()
const live = useLiveDictation()
const store = useCalendarStore()

// WAVE 5 (E7): proposta de criacao da IA (evento/task) — o cartao de confirmacao usa isto.
// WAVE 5: repete a ultima pergunta do usuario (botao do bloco "IA fora do ar").
function retryLast(): void {
  if (chat.sending.value || chat.checkingAvailability.value) return
  const lastUser = [...chat.messages.value].reverse().find((m) => m.role === 'user')
  const text = lastUser?.text?.trim()
  if (text) void chat.ask(text)
}
const { localChat, resizing, panelStyle, resizeFromLeft, measureArea, setPosition, startResize } =
  useCalendarChatWindow()

// Modo de voz (persistido por usuario): 'whisper' = self-hosted privado (grava -> para ->
// transcreve); 'live' = ditado ao vivo pelo navegador (Web Speech API, palavras aparecem
// enquanto fala, mas o audio passa pelo Google). Padrao whisper (escolha do dono: privado).
type VoiceMode = 'whisper' | 'live'
const voiceMode = useState<VoiceMode>('calendar-chat:voice-mode', () => 'whisper')
// Prefixo do input capturado quando o ditado ao vivo comeca (o texto vivo e' anexado a ele).
let liveBase = ''

const streamRef = ref<HTMLElement | null>(null)
const inputRef = ref<HTMLTextAreaElement | null>(null)

const POSITION_OPTIONS: { value: CalendarChatPosition; icon: string; label: string }[] = [
  { value: 'left', icon: 'i-lucide-panel-left', label: 'Ancorar a esquerda' },
  { value: 'center', icon: 'i-lucide-rectangle-horizontal', label: 'Centralizar (janela enxuta)' },
  { value: 'right', icon: 'i-lucide-panel-right', label: 'Ancorar a direita' },
  { value: 'fullscreen', icon: 'i-lucide-maximize', label: 'Tela cheia (área do calendário)' },
]

// Guardrail da voz (UX): so libera o microfone quando a transcricao TEM como funcionar.
// Hoje so o Whisper (OpenAI) aceita o audio do navegador (webm); o Gemini NAO transcreve
// esse formato. Sem provedor viavel, NAO deixa gravar — mostra o motivo em vez de deixar a
// pessoa falar minutos para so depois falhar.
const voiceReason = computed(() => {
  if (voiceMode.value === 'live') {
    return live.supported
      ? ''
      : 'Ditado ao vivo indisponível neste navegador. Use Chrome/Edge ou troque para o Whisper.'
  }
  if (!store.config.ai.enabled) return 'A IA do calendário está desligada (aba IA).'
  // So Whisper (self-hosted 'local' ou 'openai') aceita o audio webm do navegador.
  if (!['local', 'openai'].includes(store.config.ai.transcribeProvider)) {
    return 'A transcrição atual não aceita o áudio do navegador. Selecione Whisper (self-hosted) na aba IA para usar a voz.'
  }
  return ''
})
const voiceAvailable = computed(() => voiceReason.value === '')

// Modo de voz: opcoes do toggle no header.
const VOICE_MODES: { value: VoiceMode; label: string }[] = [
  { value: 'whisper', label: 'Whisper (privado)' },
  { value: 'live', label: 'Ao vivo' },
]

// Estado unificado (mic + indicador) por modo.
const isCapturing = computed(() =>
  voiceMode.value === 'live' ? live.state.value === 'listening' : voice.state.value === 'recording',
)
const isTranscribing = computed(
  () => voiceMode.value === 'whisper' && voice.state.value === 'transcribing',
)
// Mensagem de erro da voz vem do composable ativo no modo.
const voiceError = computed(() =>
  voiceMode.value === 'live' ? live.errorMessage.value : voice.errorMessage.value,
)

// Trocar de modo cancela qualquer captura em andamento (nao mistura os dois).
function setVoiceMode(mode: VoiceMode): void {
  if (mode === voiceMode.value) return
  if (voice.state.value === 'recording') voice.cancel()
  if (live.state.value === 'listening') live.stop()
  voiceMode.value = mode
}

// ----- Barra de gravacao (estilo WhatsApp) --------------------------------------------
// Pausado so existe no modo Whisper (o ditado ao vivo nao pausa de forma confiavel).
const isPaused = computed(() => voiceMode.value === 'whisper' && voice.paused.value)
// Waveform ativa: do medidor do modo em uso.
const captureLevels = computed(() =>
  voiceMode.value === 'live' ? live.levels.value : voice.levels.value,
)

// Timer do ditado ao vivo: o useVoiceRecorder ja cronometra o Whisper (com pausa); para o
// modo ao vivo cronometramos aqui (nao ha pausa), zerando quando a escuta termina.
const liveElapsedMs = ref(0)
let liveTimer = 0
let liveStart = 0
watch(isCapturing, (capturing) => {
  if (liveTimer) {
    window.clearInterval(liveTimer)
    liveTimer = 0
  }
  if (capturing && voiceMode.value === 'live') {
    liveStart = performance.now()
    liveElapsedMs.value = 0
    liveTimer = window.setInterval(() => {
      liveElapsedMs.value = performance.now() - liveStart
    }, 200)
  }
})

const recTimeLabel = computed(() => {
  const ms = voiceMode.value === 'live' ? liveElapsedMs.value : voice.elapsedMs.value
  const total = Math.floor(ms / 1000)
  const m = Math.floor(total / 60)
  const s = total % 60
  return `${m}:${String(s).padStart(2, '0')}`
})

// Lixeira: descarta a captura sem transcrever. No modo ao vivo, tambem reverte o texto
// ditado ao prefixo capturado no start (nao deixa "meio texto" no input).
function cancelCapture(): void {
  if (voiceMode.value === 'live') {
    live.stop()
    chat.draft.value = liveBase
  } else {
    voice.cancel()
  }
  void nextTick(() => inputRef.value?.focus())
}

// Confirmar: reusa o fluxo do mic (Whisper para+transcreve; ao vivo para e mantem o texto).
async function confirmCapture(): Promise<void> {
  await onMic()
}

// Pausa/retoma a gravacao do Whisper.
function togglePause(): void {
  if (voice.paused.value) voice.resume()
  else voice.pause()
}

// Ditado ao vivo: o texto transcrito e' anexado ao prefixo capturado no start.
// WAVE 15: marca que o rascunho veio de VOZ (vai como viaVoice no /ask — o prompt
// trata erros foneticos como provaveis).
watch(
  () => live.transcript.value,
  (text) => {
    if (live.state.value !== 'listening') return
    chat.draft.value = liveBase ? `${liveBase} ${text}`.trim() : text
    chat.draftFromVoice.value = true
    void nextTick(resizeInput)
  },
)

// Teleport pro `.app-surface` (DENTRO do shell, mesmo contexto de empilhamento do header,
// nao no body): no body o chat escapa do stacking context do app e cobre o header do painel.
// Renderiza so apos montar no cliente (evita descasamento de hidratacao com Teleport sempre presente).
const mounted = ref(false)

// Auto-grow do input: ajusta a altura ao conteudo (min 1 linha, max ~5 linhas).
function resizeInput(): void {
  const el = inputRef.value
  if (!el) {
    return
  }
  el.style.height = 'auto'
  el.style.height = `${Math.min(el.scrollHeight, 132)}px`
}

// Janela visivel = aberta E nao minimizada. Remede a area quando volta a aparecer (o
// layout pode ter mudado enquanto fechada/minimizada) e foca o input.
const windowVisible = computed(() => chat.panelOpen.value && !chat.minimized.value)

// Rola o stream do chat ate a ultima mensagem (ver o "digitando..." / a pergunta recem-enviada).
function scrollToBottom(): void {
  void nextTick(() => {
    const stream = streamRef.value
    if (stream) stream.scrollTop = stream.scrollHeight
  })
}

// Ancora a ULTIMA pergunta do usuario no TOPO do viewport (estilo ChatGPT/Claude): quando a
// resposta chega, a pessoa le a resposta de cima pra baixo em vez de cair no FIM dela e ter que
// rolar pra cima. Precisa da resposta ja no DOM (senao nao ha conteudo abaixo para rolar).
function pinLastUserMessageToTop(): void {
  const stream = streamRef.value
  if (!stream) return
  const users = stream.querySelectorAll<HTMLElement>('.calendar-chat__message--user')
  const el = users[users.length - 1]
  if (!el) {
    stream.scrollTop = stream.scrollHeight
    return
  }
  const top = el.getBoundingClientRect().top - stream.getBoundingClientRect().top + stream.scrollTop
  stream.scrollTop = Math.max(0, top - 8)
}

watch(windowVisible, (visible) => {
  if (!visible) {
    return
  }
  void nextTick(() => {
    measureArea()
    resizeInput()
    inputRef.value?.focus()
    // Abrir/reabrir a janela posiciona na 1a mensagem (leitura de cima pra baixo, tipo
    // WhatsApp) em vez de cair na ultima.
    if (streamRef.value) streamRef.value.scrollTop = 0
  })
})

// Conversa ABERTA/carregada (menu de conversas): posiciona na 1a mensagem. Vale mesmo quando
// a nova conversa tem o mesmo numero de mensagens que a anterior (o length nao muda). Limpa o
// sinal so no nextTick, para o watch de length abaixo (flush post) ainda ve-lo como pendente.
watch(
  () => chat.pendingTopScroll.value,
  (pending) => {
    if (!pending) return
    void nextTick(() => {
      if (streamRef.value) streamRef.value.scrollTop = 0
      chat.pendingTopScroll.value = false
    })
  },
)

// Turno novo: ao ENVIAR a pergunta rola pro fim (ver o "digitando..."); quando a RESPOSTA chega
// ancora a pergunta no TOPO (le a resposta de cima pra baixo, nao caindo no fim dela). Pulado
// logo apos abrir/carregar uma conversa (o watch acima ja levou ao topo da conversa).
watch(
  () => chat.messages.value.length,
  () => {
    if (chat.pendingTopScroll.value) return
    const msgs = chat.messages.value
    const last = msgs[msgs.length - 1]
    if (last?.role === 'assistant') {
      void nextTick(pinLastUserMessageToTop)
    } else {
      scrollToBottom()
    }
  },
  { flush: 'post' },
)

// O texto transcrito cai no draft: reajusta a altura do input.
watch(
  () => chat.draft.value,
  () => void nextTick(resizeInput),
)

onMounted(() => {
  // O composable ja mede a area e escuta o resize; aqui so libera o Teleport no cliente.
  mounted.value = true
})

onBeforeUnmount(() => {
  // Desmontar (ex.: sair da pagina do calendario) para qualquer captura e o timer do vivo.
  stopCapture()
  if (liveTimer) {
    window.clearInterval(liveTimer)
    liveTimer = 0
  }
})

function onInput(): void {
  // Digitacao MANUAL (fora do fluxo de captura) desmarca o sinal de voz: o usuario
  // reescreveu/ajustou o texto, entao ele deixa de ser "transcricao crua".
  if (!isCapturing.value) chat.draftFromVoice.value = false
  resizeInput()
}

function onEnter(event: KeyboardEvent): void {
  // Enter envia; Shift+Enter quebra linha.
  if (event.shiftKey) {
    return
  }
  event.preventDefault()
  chat.send()
}

// Cancela qualquer captura em andamento (whisper OU ao vivo).
function stopCapture(): void {
  if (voice.state.value === 'recording') voice.cancel()
  if (live.state.value === 'listening') live.stop()
}

function closePanel(): void {
  // Fechar cancela a captura em andamento (nao deixa o mic ligado escondido).
  stopCapture()
  chat.closePanel()
}

function minimizePanel(): void {
  // Minimizar tambem para a captura (o mic some com a janela) mas mantem a conversa.
  stopCapture()
  chat.minimize()
}

// Botao mic. Modo whisper: idle -> grava; recording -> para e transcreve. Modo ao vivo:
// idle -> comeca a ouvir (texto aparece enquanto fala); listening -> para (texto fica).
async function onMic(): Promise<void> {
  if (isTranscribing.value) return
  // Guardrail: sem transcricao/ditado viavel, nao inicia (evita capturar para so falhar).
  if (!isCapturing.value && !voiceAvailable.value) return

  if (voiceMode.value === 'live') {
    if (live.state.value === 'listening') {
      live.stop()
      void nextTick(() => inputRef.value?.focus())
    } else {
      liveBase = chat.draft.value.trim()
      live.start()
    }
    return
  }

  // Modo Whisper (self-hosted).
  if (voice.state.value === 'recording') {
    const text = await voice.stopAndTranscribe()
    if (text) {
      chat.draft.value = chat.draft.value ? `${chat.draft.value} ${text}` : text
      // WAVE 15: rascunho veio de transcricao => viaVoice no /ask (prompt trata fonetica).
      chat.draftFromVoice.value = true
    }
    void nextTick(() => inputRef.value?.focus())
    return
  }
  await voice.start()
}

// Atalhos do ASSISTENTE (WAVE 11; mapa em config.shortcuts): gravar/parar (reusa o toggle do
// onMic com guarda de estado) e fechar a janela. Parar (Enter) e Fechar (Esc) sao `force` —
// valem mesmo com o foco num campo (o dono pediu Esc "mesmo sem input em foco").
useCalendarShortcuts([
  {
    action: 'chatRecordStart',
    when: () => chat.panelOpen.value && !isCapturing.value && !isTranscribing.value,
    handler: () => void onMic(),
  },
  {
    action: 'chatRecordStop',
    force: true,
    when: () => chat.panelOpen.value && isCapturing.value,
    handler: () => void onMic(),
  },
  {
    action: 'chatClose',
    force: true,
    when: () => chat.panelOpen.value,
    handler: () => chat.closePanel(),
  },
])
</script>

<template>
  <div>
    <!-- Launcher flutuante (centro-baixo): aparece sempre que a janela nao esta visivel
         (fechada OU minimizada). Clicar abre/restaura a MESMA conversa (openPanel). -->
    <Teleport v-if="mounted" to=".app-surface">
      <div v-if="!windowVisible" class="calendar-chat-pill">
        <button
          type="button"
          class="calendar-chat-pill__open"
          aria-label="Abrir o Crow Assistant"
          title="Abrir o Crow Assistant"
          @click="chat.openPanel()"
        >
          <UIcon name="i-lucide-sparkles" aria-hidden="true" />
          <span class="calendar-chat-pill__label">Crow Assistant</span>
          <span
            v-if="chat.errorMessage.value"
            class="calendar-chat-pill__badge"
            aria-hidden="true"
          ></span>
        </button>
      </div>
    </Teleport>

    <!-- Janela: aberta e nao minimizada. Posicao/tamanho via style calculado. -->
    <Teleport v-if="mounted" to=".app-surface">
      <section
        v-if="windowVisible"
        class="calendar-chat"
        :class="{ 'calendar-chat--resizing': resizing }"
        :style="panelStyle"
        role="dialog"
        aria-label="Crow Assistant do calendario"
        @keydown.esc="closePanel"
      >
        <header class="calendar-chat__header">
          <strong class="calendar-chat__title">
            <UIcon name="i-lucide-sparkles" aria-hidden="true" />
            Crow Assistant
            <span
              v-if="chat.aiOffline.value"
              class="calendar-chat__aistatus"
              title="A IA está indisponível no momento"
            >
              <span class="calendar-chat__aistatus-dot" aria-hidden="true"></span>
              IA fora do ar
            </span>
          </strong>
          <div class="calendar-chat__head-actions">
            <CalendarChatConversations
              :conversations="chat.conversations.value"
              :active-id="chat.conversationId.value"
              :loading="chat.loadingConversations.value"
              @select="chat.openConversation"
              @new="chat.newConversation"
              @delete="chat.removeConversation"
            />
            <div class="calendar-chat__pos" role="group" aria-label="Posicao da janela">
              <button
                v-for="option in POSITION_OPTIONS"
                :key="option.value"
                type="button"
                class="calendar-chat__pos-btn"
                :class="{ 'is-active': localChat.position === option.value }"
                :aria-pressed="localChat.position === option.value"
                :aria-label="option.label"
                :title="option.label"
                @click="setPosition(option.value)"
              >
                <UIcon :name="option.icon" aria-hidden="true" />
              </button>
            </div>
            <button
              type="button"
              class="calendar-chat__icon-btn"
              :disabled="!chat.messages.value.length && !chat.sending.value"
              aria-label="Nova conversa"
              title="Nova conversa (limpa o historico)"
              @click="chat.newConversation()"
            >
              <UIcon name="i-lucide-rotate-ccw" aria-hidden="true" />
            </button>
            <button
              type="button"
              class="calendar-chat__icon-btn"
              aria-label="Minimizar"
              title="Minimizar"
              @click="minimizePanel"
            >
              <UIcon name="i-lucide-minus" aria-hidden="true" />
            </button>
            <button
              type="button"
              class="calendar-chat__icon-btn"
              aria-label="Fechar"
              title="Fechar"
              @click="closePanel"
            >
              <UIcon name="i-lucide-x" aria-hidden="true" />
            </button>
          </div>
        </header>

        <!-- Escopo do contexto (SPEC-F11): so aparece p/ agencia/multi-cliente
             (canSelect=true); usuario-cliente fica travado no lockedClientId. A escolha
             viaja no ask() e fica salva na conversa. -->
        <CalendarChatScope
          :scope="chat.chatScope.value"
          :mode="chat.scopeMode.value"
          :client-id="chat.scopeClientId.value"
          @change="chat.setScope"
        />

        <div ref="streamRef" class="calendar-chat__stream">
          <p v-if="chat.loadingConversation.value" class="calendar-chat__empty">
            Carregando conversa...
          </p>
          <p v-else-if="!chat.messages.value.length" class="calendar-chat__empty">
            Pergunte sobre o planejamento do mes, ideias por cliente ou datas. O assistente usa o
            cliente filtrado e o mes em foco como contexto.
          </p>

          <template v-else>
            <CalendarChatMessage
              v-for="message in chat.messages.value"
              :key="message.id"
              :message="message"
              :busy="chat.proposalBusyId.value === message.id"
              :clients="chat.chatScope.value.clients"
              :scope-mode="chat.scopeMode.value"
              :scope-client-id="chat.scopeClientId.value"
              @accept-selected="chat.confirmSelectedProposals"
              @reject-selected="chat.rejectSelectedProposals"
            />
          </template>

          <div
            v-if="chat.sending.value"
            class="calendar-chat__msg calendar-chat__msg--assistant calendar-chat__typing"
            aria-live="polite"
          >
            digitando...
          </div>
        </div>

        <!-- Estado "IA fora do ar" (WAVE 5): visual DISTINTO (nunca um balao normal) sempre que
             a IA nao responder — 503/cota/chave/timeout/kill switch. A pergunta fica salva. -->
        <div
          v-if="chat.aiOffline.value"
          class="calendar-chat__aioff"
          role="alert"
          aria-live="assertive"
        >
          <UIcon name="i-lucide-plug-zap" class="calendar-chat__aioff-icon" aria-hidden="true" />
          <div class="calendar-chat__aioff-body">
            <strong>IA fora do ar</strong>
            <span>{{ chat.aiOfflineReason.value || 'A IA não respondeu agora.' }}</span>
          </div>
          <button
            type="button"
            class="calendar-chat__aioff-retry"
            :disabled="chat.sending.value || chat.checkingAvailability.value"
            @click="retryLast"
          >
            <UIcon name="i-lucide-rotate-cw" aria-hidden="true" />
            Repetir
          </button>
        </div>

        <p v-if="chat.errorMessage.value" class="calendar-chat__error" role="alert">
          {{ chat.errorMessage.value }}
        </p>
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
        <div
          v-if="!isCapturing"
          class="calendar-chat__voicemode"
          role="group"
          aria-label="Modo de voz"
        >
          <button
            v-for="m in VOICE_MODES"
            :key="m.value"
            type="button"
            class="calendar-chat__voicemode-btn"
            :class="{ 'is-active': voiceMode === m.value }"
            :title="
              m.value === 'live'
                ? 'Ditado ao vivo pelo navegador (Chrome/Edge; audio passa pelo Google)'
                : 'Whisper self-hosted (privado; transcreve ao parar)'
            "
            @click="setVoiceMode(m.value)"
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
            @click="cancelCapture"
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
            @click="togglePause"
          >
            <UIcon :name="isPaused ? 'i-lucide-mic' : 'i-lucide-pause'" aria-hidden="true" />
          </button>

          <button
            type="button"
            class="calendar-chat__recbtn calendar-chat__recbtn--confirm"
            :disabled="isTranscribing"
            :aria-label="voiceMode === 'whisper' ? 'Parar e transcrever' : 'Concluir'"
            :title="voiceMode === 'whisper' ? 'Parar e transcrever' : 'Concluir'"
            @click="confirmCapture"
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
            :disabled="isTranscribing || !voiceAvailable"
            aria-label="Falar"
            :title="!voiceAvailable ? voiceReason : 'Falar'"
            @click="onMic"
          >
            <UIcon
              v-if="isTranscribing"
              name="i-lucide-loader-circle"
              class="calendar-chat__spin"
              aria-hidden="true"
            />
            <UIcon v-else name="i-lucide-mic" aria-hidden="true" />
          </button>
          <textarea
            ref="inputRef"
            v-model="chat.draft.value"
            class="calendar-chat__field"
            rows="1"
            :placeholder="
              isCapturing ? 'Fale que o texto aparece aqui...' : 'Pergunte ao assistente...'
            "
            @input="onInput"
            @keydown.enter="onEnter"
          ></textarea>
          <button
            v-if="!isCapturing"
            type="button"
            class="calendar-chat__send"
            :disabled="
              chat.sending.value || chat.checkingAvailability.value || !chat.draft.value.trim()
            "
            aria-label="Enviar"
            :title="chat.checkingAvailability.value ? 'Verificando IA...' : 'Enviar'"
            @click="chat.send()"
          >
            <UIcon name="i-lucide-send" aria-hidden="true" />
          </button>
        </div>

        <!-- Handle de resize (canto inferior): arrasto redimensiona a janela. -->
        <button
          type="button"
          class="calendar-chat__resize"
          :class="resizeFromLeft ? 'calendar-chat__resize--sw' : 'calendar-chat__resize--se'"
          aria-label="Redimensionar janela"
          title="Redimensionar"
          @mousedown="startResize($event, resizeFromLeft ? -1 : 1)"
        ></button>
      </section>
    </Teleport>
  </div>
</template>
