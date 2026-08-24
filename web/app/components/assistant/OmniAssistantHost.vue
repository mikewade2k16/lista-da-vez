<script setup lang="ts">
import { computed } from 'vue'

import CalendarChatPanel from '~/components/calendar/CalendarChatPanel.vue'
import { useCalendarChat } from '~/composables/useCalendarChat'
import type { AssistantChatSurface } from '~/domain/calendar/calendar-chat-api'

// Host unico do assistente no shell autenticado. A rota fornece apenas a surface
// default para conversas novas; o controller preserva a surface de uma conversa
// ja aberta quando o usuario navega entre modulos.
const route = useRoute()
const chat = useCalendarChat()

const surface = computed<AssistantChatSurface>(() => {
  const path = String(route.path || '')
  if (path === '/calendario' || path.startsWith('/calendario/')) return 'calendar'
  if (path === '/meta-ads' || path.startsWith('/meta-ads/')) return 'meta_ads'
  return 'global'
})

function openConfig(): void {
  chat.closePanel()
  void navigateTo({ path: '/calendario', query: { config: 'ia' } })
}
</script>

<template>
  <CalendarChatPanel :surface="surface" @open-config="openConfig" />
</template>
