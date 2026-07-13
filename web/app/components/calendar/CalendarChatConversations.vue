<script setup lang="ts">
// Menu "Conversas" do chat do Calendario (SPEC-F10). Botao no header que abre um
// dropdown com as conversas PERSISTIDAS (agencia ve todas com autor+data; cliente-side
// so as suas — a lista ja vem permission-scoped do back) + "Nova conversa". Puramente
// apresentacional: recebe a lista e EMITE eventos; quem busca/abre/apaga e o composable
// useCalendarChat (via painel). Fecha no clique-fora e no Esc (regra de dropdown do
// design system). Renderiza DENTRO da janela de chat (nao escapa do overflow).
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import type { CalendarChatConversation } from '~/domain/calendar/calendar-chat-api'

const props = defineProps<{
  conversations: CalendarChatConversation[]
  activeId: string
  loading: boolean
}>()

const emit = defineEmits<{
  (e: 'select', id: string): void
  (e: 'new'): void
  (e: 'delete', id: string): void
}>()

const open = ref(false)
const rootRef = ref<HTMLElement | null>(null)
// Id em confirmacao de exclusao (dois cliques): evita apagar por engano sem recorrer ao
// confirm() bloqueante do navegador.
const confirmingId = ref('')

// Busca de conversa (pedido do dono): filtra client-side por titulo E autor, sem acento/caixa.
// A lista ja vem inteira do back (permission-scoped), entao filtrar local e imediato.
const query = ref('')
const searchRef = ref<HTMLInputElement | null>(null)

function norm(v: string): string {
  return String(v || '')
    .trim()
    .toLowerCase()
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
}

const filtered = computed(() => {
  const needle = norm(query.value)
  if (!needle) return props.conversations
  return props.conversations.filter(
    (c) => norm(c.title).includes(needle) || norm(c.createdByName).includes(needle),
  )
})

function onPointerDown(event: PointerEvent): void {
  const target = event.target as Node | null
  if (rootRef.value && target && !rootRef.value.contains(target)) close()
}

function onKeydown(event: KeyboardEvent): void {
  if (event.key === 'Escape') close()
}

// Liga/desliga os listeners globais so enquanto o menu esta aberto.
watch(open, (isOpen) => {
  if (isOpen) {
    document.addEventListener('pointerdown', onPointerDown)
    document.addEventListener('keydown', onKeydown)
  } else {
    document.removeEventListener('pointerdown', onPointerDown)
    document.removeEventListener('keydown', onKeydown)
  }
})

onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', onPointerDown)
  document.removeEventListener('keydown', onKeydown)
})

function close(): void {
  open.value = false
  confirmingId.value = ''
  query.value = ''
}

function toggle(): void {
  open.value = !open.value
  if (!open.value) {
    confirmingId.value = ''
    query.value = ''
    return
  }
  // Abriu: foco direto no campo de busca (digitar e achar, sem clique extra).
  void nextTick(() => searchRef.value?.focus())
}

function onSelect(id: string): void {
  emit('select', id)
  close()
}

function onNew(): void {
  emit('new')
  close()
}

// 1o clique arma a confirmacao; 2o clique no mesmo item apaga de fato.
function onDelete(id: string): void {
  if (confirmingId.value === id) {
    emit('delete', id)
    confirmingId.value = ''
    return
  }
  confirmingId.value = id
}

function titleOf(conversation: CalendarChatConversation): string {
  return conversation.title.trim() || 'Conversa sem titulo'
}

function subtitleOf(conversation: CalendarChatConversation): string {
  const when = formatWhen(conversation.updatedAt)
  const author = conversation.createdByName.trim()
  if (author) return when ? `${author} · ${when}` : author
  return when
}

function formatWhen(iso: string): string {
  if (!iso) return ''
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return ''
  return date.toLocaleDateString('pt-BR', {
    day: '2-digit',
    month: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}
</script>

<template>
  <div ref="rootRef" class="calendar-chat-convos">
    <button
      type="button"
      class="calendar-chat__icon-btn"
      :class="{ 'is-active': open }"
      :aria-expanded="open"
      aria-haspopup="menu"
      aria-label="Conversas salvas"
      title="Conversas salvas"
      @click="toggle"
    >
      <UIcon name="i-lucide-messages-square" aria-hidden="true" />
    </button>

    <div v-if="open" class="calendar-chat-convos__menu" role="menu">
      <button type="button" class="calendar-chat-convos__new" @click="onNew">
        <UIcon name="i-lucide-plus" aria-hidden="true" />
        Nova conversa
      </button>

      <label class="calendar-chat-convos__search">
        <UIcon name="i-lucide-search" aria-hidden="true" />
        <input
          ref="searchRef"
          v-model="query"
          type="search"
          placeholder="Buscar conversa..."
          aria-label="Buscar conversa"
        />
      </label>

      <p v-if="loading" class="calendar-chat-convos__hint">Carregando conversas...</p>
      <p v-else-if="!conversations.length" class="calendar-chat-convos__hint">
        Nenhuma conversa salva ainda. Comece perguntando ao assistente.
      </p>
      <p v-else-if="!filtered.length" class="calendar-chat-convos__hint">
        Nenhuma conversa combina com "{{ query }}".
      </p>

      <ul v-else class="calendar-chat-convos__list">
        <li
          v-for="conversation in filtered"
          :key="conversation.id"
          class="calendar-chat-convos__item"
          :class="{ 'is-active': conversation.id === activeId }"
        >
          <button
            type="button"
            class="calendar-chat-convos__open"
            @click="onSelect(conversation.id)"
          >
            <span class="calendar-chat-convos__title">{{ titleOf(conversation) }}</span>
            <span v-if="subtitleOf(conversation)" class="calendar-chat-convos__sub">
              {{ subtitleOf(conversation) }}
            </span>
          </button>
          <button
            type="button"
            class="calendar-chat-convos__del"
            :class="{ 'is-confirming': confirmingId === conversation.id }"
            :aria-label="
              confirmingId === conversation.id ? 'Confirmar exclusao' : 'Apagar conversa'
            "
            :title="
              confirmingId === conversation.id ? 'Clique de novo para apagar' : 'Apagar conversa'
            "
            @click="onDelete(conversation.id)"
          >
            <UIcon
              :name="confirmingId === conversation.id ? 'i-lucide-check' : 'i-lucide-trash-2'"
              aria-hidden="true"
            />
          </button>
        </li>
      </ul>
    </div>
  </div>
</template>
