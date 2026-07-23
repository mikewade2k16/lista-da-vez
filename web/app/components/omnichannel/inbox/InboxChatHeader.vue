<script setup lang="ts">
import {
  UAvatar,
  UButton,
  UDashboardSidebarCollapse,
  UDashboardSidebarToggle,
  UDropdownMenu
} from "#components";
import { computed } from "vue";
import { resolveAvatarSource } from "~/composables/omnichannel/useAvatarProxy";

const {
  activeConversation,
  activeConversationLabel,
  currentUserId,
  userRole,
  showOutboundOperatorLabel,
  canManageConversation,
  updatingHandoff,
  getInitials,
  getChannelLabel,
  getStatusLabel,
  onOpenWhatsAppSession,
  onToggleShowOutboundOperatorLabel,
  onTakeConversation,
  onReleaseConversation,
  onCloseConversation
} = defineProps([
  "activeConversation",
  "activeConversationLabel",
  "currentUserId",
  "userRole",
  "showOutboundOperatorLabel",
  "canManageConversation",
  "updatingHandoff",
  "getInitials",
  "getChannelLabel",
  "getStatusLabel",
  "onOpenWhatsAppSession",
  "onToggleShowOutboundOperatorLabel",
  "onTakeConversation",
  "onReleaseConversation",
  "onCloseConversation"
]);

const adminMenuItems = computed(() => [
  [
    {
      label: "Configurar canal WhatsApp",
      icon: "i-lucide-message-circle-cog",
      onSelect: () => onOpenWhatsAppSession()
    },
    {
      label: showOutboundOperatorLabel ? "Ocultar nome do operador" : "Exibir nome do operador",
      icon: showOutboundOperatorLabel ? "i-lucide-eye-off" : "i-lucide-eye",
      onSelect: () => onToggleShowOutboundOperatorLabel(!showOutboundOperatorLabel)
    }
  ]
]);

const activeConversationIsGroup = computed(() =>
  Boolean(activeConversation?.externalId?.toLowerCase().endsWith("@g.us"))
);
</script>

<template>
  <div class="chat-page__panel-header chat-page__chat-header">
    <div class="chat-page__chat-headline">
      <UDashboardSidebarToggle side="left" color="neutral" variant="ghost" class="lg:hidden" />
      <UDashboardSidebarCollapse side="left" color="neutral" variant="ghost" class="hidden lg:inline-flex" />

      <template v-if="activeConversation">
        <UAvatar
          :src="resolveAvatarSource(activeConversation.contactAvatarUrl) || undefined"
          :alt="activeConversationLabel || 'Contato'"
          :text="getInitials(activeConversationLabel || 'Contato')"
          class="chat-page__contact-avatar"
        />
        <div class="chat-page__contact-meta">
          <p class="chat-page__contact-name">{{ activeConversationLabel }}</p>
          <div class="chat-page__contact-context">
			<span v-if="activeConversationIsGroup" class="chat-page__contact-kind">Grupo</span>
            <span>{{ getChannelLabel(activeConversation.channel) }}</span>
            <span v-if="activeConversation.instanceId">
              {{ activeConversation.instanceDisplayName || activeConversation.instanceName }}
            </span>
            <span v-if="activeConversation.status !== 'OPEN'" class="chat-page__contact-status">
              {{ getStatusLabel(activeConversation.status) }}
            </span>
          </div>
        </div>
      </template>

      <p v-else class="chat-page__empty-label">Selecione uma conversa na lista.</p>
    </div>

    <div class="chat-page__chat-actions">
      <UDashboardSidebarToggle side="right" color="neutral" variant="ghost" class="lg:hidden" />
      <UDashboardSidebarCollapse side="right" color="neutral" variant="ghost" class="hidden lg:inline-flex" />
      <UButton
        v-if="activeConversation && canManageConversation && !activeConversation.assignedToId && activeConversation.status !== 'CLOSED'"
        size="sm"
        color="primary"
        variant="solid"
        icon="i-lucide-hand"
        :loading="updatingHandoff"
        :disabled="updatingHandoff"
        @click="onTakeConversation()"
      >
        Assumir
      </UButton>
      <UButton
        v-else-if="activeConversation && canManageConversation && activeConversation.assignedToId === currentUserId"
        size="sm"
        color="warning"
        variant="soft"
        icon="i-lucide-hand-off"
        :loading="updatingHandoff"
        :disabled="updatingHandoff"
        @click="onReleaseConversation()"
      >
        Liberar
      </UButton>
      <span
        v-else-if="activeConversation?.assignedToId"
        class="chat-page__assignment-label"
      >
        Em atendimento
      </span>
      <UDropdownMenu
        v-if="userRole === 'ADMIN'"
        :items="adminMenuItems"
        :content="{ side: 'bottom', align: 'end', sideOffset: 8 }"
      >
        <UButton
          size="sm"
          color="neutral"
          variant="ghost"
          icon="i-lucide-ellipsis"
          aria-label="Mais opções do atendimento"
          title="Mais opções"
        />
      </UDropdownMenu>
      <UButton
        size="sm"
        color="neutral"
        variant="ghost"
        icon="i-lucide-circle-x"
        :disabled="!activeConversation || !canManageConversation"
        @click="onCloseConversation()"
      >
        Encerrar
      </UButton>
    </div>
  </div>
</template>

<style scoped>
.chat-page__panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}

.chat-page__chat-headline {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  min-width: 0;
}

.chat-page__contact-avatar {
  flex-shrink: 0;
}

.chat-page__contact-meta {
  min-width: 0;
}

.chat-page__contact-name {
  margin: 0;
  font-size: 0.95rem;
  font-weight: 600;
}

.chat-page__contact-context,
.chat-page__chat-actions {
  display: flex;
  align-items: center;
  gap: 0.35rem;
}

.chat-page__contact-context {
  margin-top: 0.12rem;
  color: rgb(var(--muted));
  font-size: 0.7rem;
}

.chat-page__contact-context > span + span::before {
  margin-right: 0.35rem;
  color: rgb(var(--muted) / 0.6);
  content: "·";
}

.chat-page__contact-status {
  color: rgb(var(--text));
}

.chat-page__contact-kind {
	color: rgb(var(--primary));
	font-weight: 700;
}

.chat-page__assignment-label {
  color: rgb(var(--muted));
  font-size: 0.75rem;
}

@media (max-width: 1100px) {
  .chat-page__chat-actions :deep(.ui-button-label) {
    display: none;
  }
}
</style>

