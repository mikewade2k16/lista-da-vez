<script setup lang="ts">
// Seletor de escopo do contexto do chat do Calendario (SPEC-F11, contrato D3/D4).
// Um SELECT que escolhe o que a IA enxerga: "Todos os clientes" (scopeMode='all') ou um
// cliente especifico (scopeMode='client' + scopeClientId). A visibilidade e o ACESSO vem
// 100% do back (GET /chat/scope -> resolveChatAccess): se canSelect=false (usuario-cliente
// com 1 cliente) o componente NAO renderiza nada e a IA fica travada no lockedClientId; se
// canSelect=true (agencia/multi-cliente) mostra "Todos os clientes" + cada cliente visivel.
// A escolha viaja no ask() e fica salva na conversa. Puramente apresentacional: recebe o
// escopo + a selecao atual e EMITE 'change'; quem persiste o estado e o useCalendarChat.
// SELECT nativo de proposito: fecha no clique-fora/Esc sozinho (regra de dropdown) e e
// acessivel/mobile sem markup custom.
import { computed } from 'vue'
import type { CalendarChatScope, CalendarChatScopeMode } from '~/domain/calendar/calendar-chat-api'

const props = defineProps<{
  scope: CalendarChatScope
  mode: CalendarChatScopeMode
  clientId: string
  disabled?: boolean
}>()

const emit = defineEmits<{
  (e: 'change', mode: CalendarChatScopeMode, clientId: string): void
}>()

// Valor unico do <select>: '' = "Todos os clientes" (mode 'all'); qualquer outro = id do
// cliente (mode 'client'). Espelha o par mode/clientId do estado (fonte = composable).
const ALL_VALUE = ''

const selected = computed<string>(() => (props.mode === 'client' ? props.clientId : ALL_VALUE))

function onChange(event: Event): void {
  if (props.disabled) return
  const value = (event.target as HTMLSelectElement).value
  if (value === ALL_VALUE) emit('change', 'all', '')
  else emit('change', 'client', value)
}
</script>

<template>
  <div v-if="scope.canSelect" class="calendar-chat-scope">
    <UIcon name="i-lucide-users" class="calendar-chat-scope__icon" aria-hidden="true" />
    <select
      class="calendar-chat-scope__select"
      :value="selected"
      :disabled="disabled"
      aria-label="Escopo do contexto do assistente"
      title="Contexto que a IA usa: todos os clientes ou um cliente especifico"
      @change="onChange"
    >
      <option value="">Todos os clientes</option>
      <option v-for="client in scope.clients" :key="client.id" :value="client.id">
        {{ client.name || 'Cliente sem nome' }}
      </option>
    </select>
  </div>
</template>
