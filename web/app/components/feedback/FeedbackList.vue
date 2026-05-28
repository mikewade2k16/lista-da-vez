<script setup>
import { useFeedbackWorkspaceContext } from '~/composables/useFeedbackWorkspace'

const ctx = useFeedbackWorkspaceContext()
</script>

<template>
  <aside class="admin-feedback__list" aria-label="Chamados de feedback">
    <button
      v-for="feedback in ctx.filteredFeedbacks"
      :key="feedback.id"
      class="admin-feedback__ticket"
      :class="{
        'is-active': ctx.selectedFeedback?.id === feedback.id,
        'has-unread': ctx.getUnreadCount(feedback) > 0,
      }"
      type="button"
      @click="ctx.selectFeedback(feedback.id)"
    >
      <span class="admin-feedback__ticket-line">
        <strong :title="feedback.subject || 'Chamado sem assunto'">
          {{ feedback.subject || 'Chamado sem assunto' }}
        </strong>
        <span class="admin-feedback__ticket-badges">
          <small
            class="admin-feedback__kind-tag"
            :class="`admin-feedback__kind-tag--${feedback.kind}`"
          >
            {{ ctx.kindLabel(feedback.kind) }}
          </small>
          <small
            class="admin-feedback__status-tag"
            :class="`admin-feedback__status-tag--${feedback.status}`"
          >
            {{ ctx.statusLabel(feedback.status) }}
          </small>
        </span>
      </span>
      <span class="admin-feedback__ticket-meta-row">
        <span class="admin-feedback__ticket-meta">
          {{ feedback.user_name || 'Usuario' }} - {{ ctx.getStoreLabel(feedback.store_id) }}
        </span>
        <small class="admin-feedback__ticket-time">
          {{ ctx.formatDate(feedback.updated_at || feedback.created_at) }}
        </small>
      </span>
      <small class="admin-feedback__ticket-preview" :title="ctx.getFeedbackPreview(feedback)">
        {{ ctx.getFeedbackPreview(feedback) }}
      </small>
    </button>

    <div v-if="!ctx.filteredFeedbacks.length" class="admin-feedback__empty-list">
      <strong>Nenhum feedback encontrado</strong>
      <span>Ajuste os filtros ou aguarde novos chamados.</span>
    </div>
  </aside>
</template>
