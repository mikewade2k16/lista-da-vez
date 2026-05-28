<script setup>
import { ImagePlus, Send, X } from 'lucide-vue-next'

import AppSelectField from '~/components/ui/AppSelectField.vue'
import { useFeedbackWorkspaceContext } from '~/composables/useFeedbackWorkspace'
import { formatFeedbackImageSize } from '~/utils/feedback-image'

const ctx = useFeedbackWorkspaceContext()
</script>

<template>
  <article v-if="ctx.selectedFeedback" class="admin-feedback__conversation">
    <header class="admin-feedback__conversation-header">
      <div class="admin-feedback__conversation-copy">
        <div class="admin-feedback__conversation-meta">
          <span
            class="admin-feedback__kind-tag"
            :class="`admin-feedback__kind-tag--${ctx.selectedFeedback.kind}`"
          >
            {{ ctx.kindLabel(ctx.selectedFeedback.kind) }}
          </span>
          <p>
            {{ ctx.selectedFeedback.user_name || 'Usuario' }} -
            {{ ctx.getStoreLabel(ctx.selectedFeedback.store_id) }} -
            {{ ctx.formatDate(ctx.selectedFeedback.created_at) }}
          </p>
        </div>
        <h3 :title="ctx.selectedFeedback.subject">{{ ctx.selectedFeedback.subject }}</h3>
      </div>

      <AppSelectField
        v-model="ctx.editingStatus"
        class="admin-feedback__status-select"
        :options="ctx.detailStatusOptions"
        compact
        :disabled="!ctx.canEditFeedback || ctx.saving"
      />
    </header>

    <div :ref="ctx.setMessagesViewport" class="admin-feedback__messages">
      <article
        v-for="message in ctx.selectedMessages"
        :key="message.id"
        class="admin-feedback__message"
        :class="{ 'admin-feedback__message--own': message.author_user_id === ctx.user?.id }"
      >
        <header>
          <strong>{{ message.author_name || 'Usuario' }}</strong>
          <span>{{ ctx.formatDate(message.created_at) }}</span>
        </header>
        <p v-if="message.body">{{ message.body }}</p>
        <a
          v-if="message.image_url"
          class="admin-feedback__message-image-link"
          :href="message.image_url"
          target="_blank"
          rel="noopener noreferrer"
        >
          <img
            :src="message.image_url"
            alt="Imagem anexada ao feedback"
            class="admin-feedback__message-image"
          />
        </a>
      </article>

      <article v-if="!ctx.selectedMessages.length" class="admin-feedback__message">
        <header>
          <strong>{{ ctx.selectedFeedback.user_name || 'Usuario' }}</strong>
          <span>{{ ctx.formatDate(ctx.selectedFeedback.created_at) }}</span>
        </header>
        <p>{{ ctx.selectedFeedback.body }}</p>
      </article>
    </div>

    <div
      v-if="ctx.isSelectedFeedbackClosed"
      class="admin-feedback__readonly admin-feedback__readonly--closed"
    >
      Chamado encerrado. A conversa esta bloqueada para novas mensagens.
    </div>
    <div v-else-if="!ctx.canEditFeedback" class="admin-feedback__readonly">
      Seu acesso ao feedback esta em modo somente leitura.
    </div>

    <form class="admin-feedback__reply" @submit.prevent="ctx.sendReply">
      <div class="admin-feedback__reply-input-row">
        <textarea
          :ref="ctx.setReplyTextarea"
          v-model="ctx.replyMessage"
          :placeholder="
            ctx.isSelectedFeedbackClosed ? 'Chamado encerrado' : 'Responder este chamado'
          "
          rows="1"
          :disabled="!ctx.canEditFeedback || ctx.saving || ctx.isSelectedFeedbackClosed"
          @input="ctx.syncReplyTextareaHeight()"
          @keydown="ctx.handleReplyKeydown"
        ></textarea>

        <button
          type="submit"
          :disabled="
            ctx.saving ||
            !ctx.canEditFeedback ||
            ctx.isSelectedFeedbackClosed ||
            (!ctx.replyMessage.trim() && !ctx.replyImage)
          "
        >
          <Send :size="16" :stroke-width="2.2" />
          <span>{{ ctx.saving ? 'Enviando...' : 'Enviar' }}</span>
        </button>
      </div>

      <div class="admin-feedback__reply-tools">
        <label
          class="admin-feedback__upload-btn"
          :class="{
            'is-disabled': !ctx.canEditFeedback || ctx.saving || ctx.isSelectedFeedbackClosed,
          }"
        >
          <input
            type="file"
            accept="image/png,image/jpeg,image/webp"
            hidden
            :disabled="!ctx.canEditFeedback || ctx.saving || ctx.isSelectedFeedbackClosed"
            @change="ctx.handleReplyImageChange"
          />
          <ImagePlus :size="16" :stroke-width="2.1" />
          <span>{{ ctx.replyImage ? 'Trocar imagem' : 'Anexar imagem' }}</span>
        </label>
        <small class="admin-feedback__upload-hint">
          A imagem e compactada no envio e apagada 7 dias apos o fechamento.
        </small>
      </div>

      <div v-if="ctx.replyImagePreviewUrl" class="admin-feedback__reply-preview">
        <img
          :src="ctx.replyImagePreviewUrl"
          alt="Preview da imagem anexada"
          class="admin-feedback__reply-preview-image"
        />
        <div class="admin-feedback__reply-preview-copy">
          <strong>{{ ctx.replyImage?.name }}</strong>
          <span>{{ formatFeedbackImageSize(ctx.replyImage?.size || 0) }}</span>
        </div>
        <button
          type="button"
          class="admin-feedback__reply-preview-remove"
          :disabled="ctx.saving"
          @click="ctx.clearReplyImage"
        >
          <X :size="14" :stroke-width="2.2" />
        </button>
      </div>
    </form>
  </article>

  <article v-else class="admin-feedback__placeholder">
    <strong>Selecione um chamado</strong>
    <span>Quando voce abrir um feedback, a conversa aparece aqui.</span>
  </article>
</template>
