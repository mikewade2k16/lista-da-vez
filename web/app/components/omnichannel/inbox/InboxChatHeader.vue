<script setup lang="ts">
import {
  UAvatar,
  UBadge,
  UButton,
  UDashboardSidebarCollapse,
  UDashboardSidebarToggle
} from "#components";
import { resolveAvatarSource } from "~/composables/omnichannel/useAvatarProxy";

const {
  activeConversation,
  activeConversationLabel,
  userRole,
  showOutboundOperatorLabel,
  canManageConversation,
  getInitials,
  getChannelLabel,
  getStatusColor,
  getStatusLabel,
  onToggleShowOutboundOperatorLabel,
  onCloseConversation
} = defineProps([
  "activeConversation",
  "activeConversationLabel",
  "userRole",
  "showOutboundOperatorLabel",
  "canManageConversation",
  "getInitials",
  "getChannelLabel",
  "getStatusColor",
  "getStatusLabel",
  "onOpenWhatsAppSession",
  "onToggleShowOutboundOperatorLabel",
  "onCloseConversation"
]);
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
          <div class="chat-page__contact-tags">
            <UBadge color="neutral" variant="soft" size="sm">
              {{ getChannelLabel(activeConversation.channel) }}
            </UBadge>
            <UBadge
              v-if="activeConversation.instanceId"
              color="primary"
              variant="soft"
              size="sm"
            >
              {{ activeConversation.instanceDisplayName || activeConversation.instanceName }}
            </UBadge>
            <UBadge :color="getStatusColor(activeConversation.status)" variant="soft" size="sm">
              {{ getStatusLabel(activeConversation.status) }}
            </UBadge>
          </div>
        </div>
      </template>

      <p v-else class="chat-page__empty-label">Selecione uma conversa na lista.</p>
    </div>

    <div class="chat-page__chat-actions">
      <UDashboardSidebarToggle side="right" color="neutral" variant="ghost" class="lg:hidden" />
      <UDashboardSidebarCollapse side="right" color="neutral" variant="ghost" class="hidden lg:inline-flex" />
      <UButton v-if="userRole === 'ADMIN'" size="sm" color="primary" variant="outline" @click="onOpenWhatsAppSession()">
        Canal WhatsApp
      </UButton>
      <UButton
        v-if="userRole === 'ADMIN'"
        size="sm"
        color="neutral"
        :variant="showOutboundOperatorLabel ? 'soft' : 'ghost'"
        @click="onToggleShowOutboundOperatorLabel(!showOutboundOperatorLabel)"
      >
        {{ showOutboundOperatorLabel ? "Operador visivel" : "Operador oculto" }}
      </UButton>
      <UButton
        size="sm"
        color="neutral"
        variant="ghost"
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

.chat-page__contact-tags,
.chat-page__chat-actions {
  display: flex;
  align-items: center;
  gap: 0.35rem;
}
</style>

