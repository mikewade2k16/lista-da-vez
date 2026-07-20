<script setup lang="ts">
import { UButton, UIcon } from "#components";
import { computed } from "vue";
import InboxAudioMessagePlayer from "./InboxAudioMessagePlayer.vue";

const {
  replyTarget,
  hasAttachment,
  attachmentType,
  attachmentName,
  attachmentMimeType,
  attachmentSizeBytes,
  attachmentDurationSeconds,
  attachmentPreviewUrl,
  isRecording,
  recordingElapsedLabel,
  recordingWaveLevels,
  recordingError,
  sendError,
  sendingMessage,
  isComposerReadOnly,
  onClearReply,
  onClearAttachment,
  cancelRecording,
  sendRecordedAudio,
  getReplyTargetAuthorLabel,
  shouldShowReplyTypeMeta,
  resolveMessageType,
  getReplyTypeIcon,
  getMediaTypeLabel,
  getReplyTargetText,
  formatFileSize
} = defineProps([
  "replyTarget",
  "hasAttachment",
  "attachmentType",
  "attachmentName",
  "attachmentMimeType",
  "attachmentSizeBytes",
  "attachmentDurationSeconds",
  "attachmentPreviewUrl",
  "isRecording",
  "recordingElapsedLabel",
  "recordingWaveLevels",
  "recordingError",
  "sendError",
  "sendingMessage",
  "isComposerReadOnly",
  "onClearReply",
  "onClearAttachment",
  "cancelRecording",
  "sendRecordedAudio",
  "getReplyTargetAuthorLabel",
  "shouldShowReplyTypeMeta",
  "resolveMessageType",
  "getReplyTypeIcon",
  "getMediaTypeLabel",
  "getReplyTargetText",
  "formatFileSize"
]);

const hasVisibleState = computed(() =>
  Boolean(
    replyTarget ||
      hasAttachment ||
      isRecording ||
      recordingError ||
      sendError ||
      sendingMessage ||
      isComposerReadOnly
  )
);
</script>

<template>
  <div v-if="hasVisibleState" class="chat-page__panel-footer-state">
    <div v-if="replyTarget" class="chat-reply">
      <div class="chat-reply__content">
        <p class="chat-reply__label">Respondendo a</p>
        <p class="chat-reply__author">{{ getReplyTargetAuthorLabel(replyTarget) }}</p>
        <p v-if="shouldShowReplyTypeMeta(resolveMessageType(replyTarget))" class="chat-reply__meta">
          <UIcon :name="getReplyTypeIcon(resolveMessageType(replyTarget))" />
          <span>{{ getMediaTypeLabel(resolveMessageType(replyTarget)) }}</span>
        </p>
        <p class="chat-reply__text">{{ getReplyTargetText(replyTarget) }}</p>
      </div>
      <UButton
        size="xs"
        color="neutral"
        variant="ghost"
        icon="i-lucide-x"
        class="chat-reply__dismiss"
        @click="onClearReply()"
      />
    </div>

    <div v-if="hasAttachment" class="chat-attachment">
      <div class="chat-attachment__preview">
        <img
          v-if="attachmentPreviewUrl && attachmentType === 'IMAGE'"
          :src="attachmentPreviewUrl"
          alt="Imagem selecionada"
          class="chat-attachment__image"
        >
        <video
          v-else-if="attachmentPreviewUrl && attachmentType === 'VIDEO'"
          :src="attachmentPreviewUrl"
          class="chat-attachment__video"
          controls
          preload="metadata"
        ></video>
        <InboxAudioMessagePlayer
          v-else-if="attachmentPreviewUrl && attachmentType === 'AUDIO'"
          :src="attachmentPreviewUrl"
          :duration-seconds="attachmentDurationSeconds ?? null"
          :direction="'OUTBOUND'"
          :compact="true"
          class="chat-attachment__audio-player"
        />
        <div v-else class="chat-attachment__icon">
          <UIcon name="i-lucide-paperclip" />
        </div>
      </div>

      <div class="chat-attachment__meta">
        <p class="chat-attachment__name">{{ attachmentName || "Arquivo" }}</p>
        <p class="chat-attachment__info">
          {{ getMediaTypeLabel(attachmentType) }} - {{ attachmentMimeType || "arquivo" }} -
          {{ formatFileSize(attachmentSizeBytes) }}
        </p>
      </div>

      <UButton size="xs" color="neutral" variant="ghost" icon="i-lucide-x" @click="onClearAttachment()" />
    </div>

    <div v-if="isRecording" class="chat-composer__recording-shell">
      <button
        type="button"
        class="chat-composer__record-action chat-composer__record-action--cancel"
        @click="cancelRecording"
      >
        <UIcon name="i-lucide-trash-2" />
      </button>
      <span class="chat-composer__record-dot"></span>
      <span class="chat-composer__record-time">{{ recordingElapsedLabel }}</span>

      <div class="chat-composer__record-wave">
        <span
          v-for="(level, index) in recordingWaveLevels"
          :key="`record-level-${index}`"
          class="chat-composer__record-wave-bar"
          :style="{ height: `${Math.round(8 + level * 16)}px` }"
        />
      </div>

      <button
        type="button"
        class="chat-composer__record-action chat-composer__record-action--send"
        @click="sendRecordedAudio"
      >
        <UIcon name="i-lucide-send-horizontal" />
      </button>
    </div>

    <div v-else-if="recordingError" class="chat-composer__recording-error">
      {{ recordingError }}
    </div>

    <div v-if="sendError" class="chat-composer__send-error">
      {{ sendError }}
    </div>

    <div v-if="sendingMessage" class="chat-composer__sending-hint">
      Enviando mensagem...
    </div>

    <div v-if="isComposerReadOnly" class="chat-composer__readonly-hint">
      Seu perfil esta em modo somente leitura nesta conversa.
    </div>
  </div>
</template>

<style scoped>
.chat-page__panel-footer-state {
  display: grid;
  gap: 0.45rem;
  width: 100%;
}

.chat-reply {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
  border: 1px solid rgb(var(--border));
  border-left: 3px solid rgb(var(--warning));
  background: rgb(var(--surface));
  border-radius: 1rem;
  padding: 0.55rem 0.75rem;
  min-height: 3.75rem;
}

.chat-reply__content {
  min-width: 0;
  flex: 1;
  display: grid;
  gap: 0.14rem;
}

.chat-reply__label,
.chat-reply__text,
.chat-reply__author,
.chat-reply__meta {
  margin: 0;
}

.chat-reply__label {
  font-size: 0.7rem;
  color: rgb(var(--muted));
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.chat-reply__author {
  font-size: 0.84rem;
  color: rgb(var(--warning));
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.chat-reply__meta,
.chat-reply__text {
  font-size: 0.82rem;
  color: rgb(var(--muted));
}

.chat-reply__text {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  line-height: 1.35;
}

.chat-reply__dismiss {
  margin-top: 0.05rem;
  flex: 0 0 auto;
}

.chat-attachment {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.75rem;
  padding: 0.65rem 0.75rem;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  background: rgb(var(--surface));
}

.chat-attachment__image,
.chat-attachment__video {
  width: 4rem;
  height: 4rem;
  object-fit: cover;
  border-radius: var(--radius-xs);
  display: block;
}

.chat-attachment__audio-player {
  min-width: 16rem;
}

.chat-attachment__icon {
  width: 4rem;
  height: 4rem;
  display: grid;
  place-items: center;
  border-radius: var(--radius-xs);
  background: rgb(var(--surface-2));
  color: rgb(var(--muted));
}

.chat-attachment__meta {
  min-width: 0;
  display: grid;
  gap: 0.15rem;
}

.chat-attachment__name,
.chat-attachment__info {
  margin: 0;
}

.chat-attachment__name {
  font-size: 0.82rem;
  font-weight: 600;
}

.chat-attachment__info,
.chat-composer__recording-error,
.chat-composer__send-error,
.chat-composer__readonly-hint,
.chat-composer__sending-hint {
  font-size: 0.72rem;
  color: rgb(var(--muted));
}

.chat-composer__send-error,
.chat-composer__recording-error {
  color: rgb(var(--error));
}

.chat-composer__readonly-hint {
  color: rgb(var(--warning));
}

.chat-composer__recording-shell {
  display: grid;
  grid-template-columns: auto auto auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.5rem;
  padding: 0.6rem 0.75rem;
  border: 1px solid rgb(var(--border));
  border-radius: 999px;
  background: rgb(var(--surface));
}

.chat-composer__record-time {
  font-size: 0.82rem;
  font-weight: 600;
}

.chat-composer__record-action {
  width: 2.5rem;
  height: 2.5rem;
  display: grid;
  place-items: center;
  border: 0;
  border-radius: 999px;
  cursor: pointer;
}

.chat-composer__record-action--cancel {
  background: rgb(var(--surface-2));
  color: rgb(var(--text));
}

.chat-composer__record-action--send {
  background: rgb(var(--primary));
  color: rgb(var(--primary-foreground));
}

.chat-composer__record-wave {
  min-width: 0;
  height: 1.5rem;
  display: flex;
  align-items: center;
  gap: 0.1rem;
}

.chat-composer__record-wave-bar {
  width: 0.12rem;
  border-radius: 999px;
  background: rgb(var(--primary) / 0.75);
}

.chat-composer__record-dot {
  width: 0.46rem;
  height: 0.46rem;
  border-radius: 999px;
  background: rgb(var(--error));
}
</style>
