<script setup>
import '~/components/feedback/feedback-workspace.css'

import FeedbackDetailPanel from '~/components/feedback/FeedbackDetailPanel.vue'
import FeedbackFilters from '~/components/feedback/FeedbackFilters.vue'
import FeedbackList from '~/components/feedback/FeedbackList.vue'
import { provideFeedbackWorkspace, useFeedbackWorkspace } from '~/composables/useFeedbackWorkspace'

const ctx = useFeedbackWorkspace()
provideFeedbackWorkspace(ctx)
</script>

<template>
  <section class="admin-panel admin-feedback" data-testid="feedback-panel">
    <header class="admin-panel__header admin-feedback__header">
      <h2 class="admin-panel__title">Feedback</h2>
      <p class="admin-panel__subtitle">
        Acompanhe a conversa dos chamados, responda no mesmo fio e ajuste o status sem sair da tela.
      </p>
    </header>

    <FeedbackFilters />

    <div class="admin-feedback__layout">
      <FeedbackList />
      <FeedbackDetailPanel />
    </div>
  </section>
</template>
