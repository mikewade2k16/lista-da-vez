<script setup lang="ts">
import { UButton } from "#components";

const {
  activeConversation,
  canManageConversation,
  showLinkPreviewToggle,
  linkPreviewEnabled,
  toggleLinkPreview,
  composerHasContent,
  isRecording,
  emitSendFromComposer,
  toggleRecording
} = defineProps([
  "activeConversation",
  "canManageConversation",
  "showLinkPreviewToggle",
  "linkPreviewEnabled",
  "toggleLinkPreview",
  "composerHasContent",
  "isRecording",
  "emitSendFromComposer",
  "toggleRecording"
]);
</script>

<template>
  <template v-if="showLinkPreviewToggle">
    <UButton
      size="xs"
      color="neutral"
      :variant="linkPreviewEnabled ? 'soft' : 'ghost'"
      icon="i-lucide-link"
      :disabled="!activeConversation || !canManageConversation"
      :title="linkPreviewEnabled ? 'Previa de link ativada' : 'Previa de link desativada'"
      @click="toggleLinkPreview"
    >
      Previa
    </UButton>
  </template>

  <UButton
    v-if="composerHasContent"
    size="sm"
    color="primary"
    variant="solid"
    icon="i-lucide-send-horizontal"
    :disabled="!activeConversation || !canManageConversation"
    aria-label="Enviar"
    @click="emitSendFromComposer"
  />
  <UButton
    v-else
    size="sm"
    :color="isRecording ? 'error' : 'neutral'"
    :variant="isRecording ? 'soft' : 'ghost'"
    :icon="isRecording ? 'i-lucide-stop-circle' : 'i-lucide-mic'"
    :disabled="!activeConversation || !canManageConversation"
    :aria-label="isRecording ? 'Parar gravacao' : 'Gravar audio'"
    @click="toggleRecording"
  />
</template>
