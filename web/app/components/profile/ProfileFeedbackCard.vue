<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import FeedbackFormModal from '~/components/feedback/FeedbackFormModal.vue'
import { useFeedbackStore } from '~/stores/feedback'
import { useCoreAccountStore } from '../../../layers/core/stores/account'

// Bloco "Seus chamados": resumo dos feedbacks do proprio usuario — total,
// nao-lidas e a ultima resposta. Dado 100% escopado ao usuario no backend
// (GET /v1/feedback/me filtra por UserID). Feedback e feature do modulo queue:
// se a conta nao tem queue, o card nem aparece (igual o sino do header).
const account = useCoreAccountStore()
const feedback = useFeedbackStore()

const hasQueueModule = computed(() => (account.enabledModules || []).includes('queue'))
const feedbackModalOpen = ref(false)

const myFeedbacks = computed(() => feedback.myFeedbacks || [])
const total = computed(() => myFeedbacks.value.length)
const unreadTotal = computed(() =>
  myFeedbacks.value.reduce(
    (sum, item) =>
      item.status !== 'closed' ? sum + Math.max(0, Number(item.unread_count || 0)) : sum,
    0,
  ),
)

const latestReply = computed(() => {
  const withReply = myFeedbacks.value.filter((item) => item.last_message_at)
  if (!withReply.length) {
    return null
  }
  const sorted = [...withReply].sort(
    (a, b) => Date.parse(String(b.last_message_at)) - Date.parse(String(a.last_message_at)),
  )
  const top = sorted[0]
  return {
    subject: String(top.subject || 'Chamado').trim(),
    body: String(top.last_message_body || '').trim(),
  }
})

const isLoading = computed(() => feedback.loading && !myFeedbacks.value.length)

onMounted(() => {
  if (hasQueueModule.value) {
    void feedback.fetchMyFeedbacks()
  }
})
</script>

<template>
  <article v-if="hasQueueModule" class="settings-card profile-feedback">
    <header class="settings-card__header">
      <h3 class="settings-card__title">Seus chamados</h3>
      <p class="settings-card__text">Chamados de suporte e respostas do time.</p>
    </header>

    <p v-if="isLoading" class="profile-feedback__loading">Carregando seus chamados...</p>

    <template v-else>
      <div class="profile-feedback__stats">
        <div class="profile-feedback__stat">
          <span class="profile-feedback__stat-value">{{ total }}</span>
          <span class="profile-feedback__stat-label">chamados</span>
        </div>
        <div
          class="profile-feedback__stat"
          :class="{ 'profile-feedback__stat--alert': unreadTotal > 0 }"
        >
          <span class="profile-feedback__stat-value">{{ unreadTotal }}</span>
          <span class="profile-feedback__stat-label">nao lidas</span>
        </div>
      </div>

      <div v-if="latestReply" class="profile-feedback__latest">
        <span class="profile-feedback__latest-label">Ultima resposta</span>
        <strong class="profile-feedback__latest-subject">{{ latestReply.subject }}</strong>
        <p v-if="latestReply.body" class="profile-feedback__latest-body">{{ latestReply.body }}</p>
      </div>
    </template>

    <div class="profile-feedback__actions">
      <AppPanelButton variant="secondary" @click="feedbackModalOpen = true">
        Abrir chamado
      </AppPanelButton>
      <NuxtLink class="profile-feedback__link" to="/meus-chamados">Ver todos</NuxtLink>
    </div>

    <FeedbackFormModal v-model="feedbackModalOpen" />
  </article>
</template>

<style scoped>
.profile-feedback__loading {
  margin: 0;
  font-size: 0.85rem;
  color: var(--text-muted);
}

.profile-feedback__stats {
  display: flex;
  gap: 1.5rem;
}

.profile-feedback__stat {
  display: grid;
  gap: 0.1rem;
}

.profile-feedback__stat-value {
  font-size: 1.5rem;
  font-weight: 700;
  color: var(--text-main);
  line-height: 1;
}

.profile-feedback__stat--alert .profile-feedback__stat-value {
  color: var(--accent-warning);
}

.profile-feedback__stat-label {
  font-size: 0.74rem;
  color: var(--text-muted);
}

.profile-feedback__latest {
  display: grid;
  gap: 0.2rem;
  margin-top: 1rem;
  padding: 0.7rem 0.85rem;
  border-radius: var(--radius-soft);
  background: var(--bg-muted);
}

.profile-feedback__latest-label {
  font-size: 0.72rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--text-muted);
}

.profile-feedback__latest-subject {
  font-size: 0.85rem;
  color: var(--text-main);
}

.profile-feedback__latest-body {
  margin: 0;
  font-size: 0.8rem;
  color: var(--text-muted);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.profile-feedback__actions {
  display: flex;
  align-items: center;
  gap: 1rem;
  margin-top: 1rem;
}

.profile-feedback__link {
  font-size: 0.82rem;
  color: var(--accent-info);
  text-decoration: none;
}

.profile-feedback__link:hover {
  text-decoration: underline;
}
</style>
