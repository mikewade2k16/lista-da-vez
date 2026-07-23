<script setup lang="ts">
import {
  isHttpsMediaUrl,
  socialPostMediaTypeLabel,
  socialPostStatusLabel,
  type SocialPublishingPost,
} from '~/domain/social-publishing/model'

const props = withDefaults(
  defineProps<{
    posts: SocialPublishingPost[]
    mode?: 'queue' | 'content'
    canManage: boolean
    busyPostIds: string[]
  }>(),
  {
    mode: 'content',
  },
)

const emit = defineEmits<{
  edit: [post: SocialPublishingPost]
  cancel: [post: SocialPublishingPost]
  retry: [post: SocialPublishingPost]
}>()

const dateFormatter = new Intl.DateTimeFormat('pt-BR', {
  dateStyle: 'medium',
  timeStyle: 'short',
})

function formatDate(value: string | null): string {
  if (!value) return 'Horário não definido'
  const timestamp = Date.parse(value)
  return Number.isFinite(timestamp) ? dateFormatter.format(new Date(timestamp)) : 'Data inválida'
}

function canEdit(post: SocialPublishingPost): boolean {
  return (
    props.canManage &&
    post.lastErrorCode !== 'publish_outcome_unknown' &&
    !['publishing', 'published'].includes(post.status)
  )
}

function isBusy(post: SocialPublishingPost): boolean {
  return props.busyPostIds.includes(post.id)
}

function primaryDate(post: SocialPublishingPost): string | null {
  return post.publishedAt || post.scheduledFor || post.updatedAt || post.createdAt
}
</script>

<template>
  <section class="sp-posts" :aria-label="mode === 'queue' ? 'Fila de postagens' : 'Conteúdos'">
    <div v-if="posts.length" class="sp-posts__grid">
      <article v-for="post in posts" :key="post.id" class="sp-post omni-glass">
        <div class="sp-post__media">
          <img
            v-if="isHttpsMediaUrl(post.mediaUrl)"
            :src="post.mediaUrl"
            :alt="post.altText || 'Prévia da publicação'"
            loading="lazy"
          />
          <div v-else class="sp-post__media-empty">
            <UIcon name="i-lucide-image-off" aria-hidden="true" />
            <span>Imagem indisponível</span>
          </div>
          <span class="sp-post__status" :class="`sp-post__status--${post.status}`">
            {{ socialPostStatusLabel(post.status) }}
          </span>
        </div>

        <div class="sp-post__body">
          <div class="sp-post__meta">
            <span>{{ socialPostMediaTypeLabel(post.mediaType) }}</span>
            <time v-if="primaryDate(post)" :datetime="primaryDate(post) || undefined">
              {{ formatDate(primaryDate(post)) }}
            </time>
          </div>

          <p class="sp-post__caption">
            {{ post.caption || 'Sem legenda' }}
          </p>

          <p v-if="post.status === 'failed'" class="sp-post__error" role="status">
            <UIcon name="i-lucide-circle-alert" aria-hidden="true" />
            {{
              post.lastErrorCode === 'publish_outcome_unknown'
                ? 'O resultado do envio é incerto. Confira o perfil no Instagram antes de qualquer nova ação.'
                : post.lastErrorMessage || 'O Instagram recusou esta tentativa.'
            }}
          </p>

          <div class="sp-post__actions">
            <UButton
              v-if="canEdit(post)"
              type="button"
              color="neutral"
              variant="soft"
              size="sm"
              icon="i-lucide-pencil"
              label="Editar"
              :disabled="isBusy(post)"
              @click="emit('edit', post)"
            />
            <UButton
              v-if="canManage && post.status === 'scheduled'"
              type="button"
              color="neutral"
              variant="ghost"
              size="sm"
              icon="i-lucide-calendar-x"
              label="Cancelar"
              :loading="isBusy(post)"
              @click="emit('cancel', post)"
            />
            <UButton
              v-if="
                canManage &&
                post.status === 'failed' &&
                post.lastErrorCode !== 'publish_outcome_unknown'
              "
              type="button"
              color="primary"
              variant="soft"
              size="sm"
              icon="i-lucide-refresh-cw"
              label="Tentar novamente"
              :loading="isBusy(post)"
              @click="emit('retry', post)"
            />
            <a
              v-if="post.status === 'published' && post.permalink"
              class="sp-post__link"
              :href="post.permalink"
              target="_blank"
              rel="noopener noreferrer"
            >
              Ver no Instagram
              <UIcon name="i-lucide-external-link" aria-hidden="true" />
            </a>
          </div>
        </div>
      </article>
    </div>

    <div v-else class="sp-posts__empty omni-glass">
      <div class="sp-posts__empty-icon" aria-hidden="true">
        <UIcon :name="mode === 'queue' ? 'i-lucide-calendar-clock' : 'i-lucide-layout-grid'" />
      </div>
      <h2>{{ mode === 'queue' ? 'Sua fila está livre' : 'Nenhum conteúdo por aqui' }}</h2>
      <p>
        {{
          mode === 'queue'
            ? 'Quando uma publicação for agendada, ela aparecerá aqui em ordem de envio.'
            : 'Crie um rascunho para começar a preparar o conteúdo deste cliente.'
        }}
      </p>
    </div>
  </section>
</template>

<style scoped>
.sp-posts__grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 1rem;
}

.sp-post {
  min-width: 0;
  overflow: hidden;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface));
  box-shadow: var(--shadow-xs);
}

.sp-post__media {
  position: relative;
  aspect-ratio: 4 / 3;
  overflow: hidden;
  background: rgb(var(--surface-2));
}

.sp-post__media img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.sp-post__media-empty {
  display: grid;
  width: 100%;
  height: 100%;
  place-content: center;
  justify-items: center;
  gap: 0.45rem;
  color: rgb(var(--muted));
  font-size: 0.8rem;
}

.sp-post__media-empty :deep(svg) {
  width: 1.6rem;
  height: 1.6rem;
}

.sp-post__status {
  position: absolute;
  top: 0.65rem;
  left: 0.65rem;
  padding: 0.28rem 0.55rem;
  border: 1px solid rgb(var(--border) / 0.75);
  border-radius: 999px;
  color: rgb(var(--text));
  background: rgb(var(--surface) / 0.9);
  font-size: 0.7rem;
  font-weight: 750;
  backdrop-filter: blur(10px);
}

.sp-post__status--published {
  color: rgb(var(--success));
}

.sp-post__status--failed {
  color: rgb(var(--danger));
}

.sp-post__status--scheduled,
.sp-post__status--publishing {
  color: rgb(var(--primary));
}

.sp-post__body {
  display: grid;
  gap: 0.8rem;
  padding: 0.9rem;
}

.sp-post__meta {
  display: flex;
  justify-content: space-between;
  gap: 0.65rem;
  color: rgb(var(--muted));
  font-size: 0.72rem;
}

.sp-post__meta time {
  text-align: right;
}

.sp-post__caption {
  display: -webkit-box;
  min-height: 2.8rem;
  margin: 0;
  overflow: hidden;
  color: rgb(var(--text));
  font-size: 0.9rem;
  line-height: 1.45;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.sp-post__error {
  display: flex;
  align-items: flex-start;
  gap: 0.4rem;
  margin: 0;
  padding: 0.55rem;
  border-radius: var(--radius-xs);
  color: rgb(var(--danger));
  background: rgb(var(--danger) / 0.09);
  font-size: 0.76rem;
  line-height: 1.35;
}

.sp-post__error :deep(svg) {
  margin-top: 0.1rem;
  flex: 0 0 auto;
}

.sp-post__actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.4rem;
}

.sp-post__link {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  margin-left: auto;
  color: rgb(var(--primary));
  font-size: 0.78rem;
  font-weight: 650;
  text-decoration: none;
}

.sp-post__link:hover {
  text-decoration: underline;
}

.sp-post__link:focus-visible {
  outline: 2px solid rgb(var(--ring));
  outline-offset: 3px;
}

.sp-posts__empty {
  display: grid;
  min-height: 16rem;
  place-content: center;
  justify-items: center;
  padding: 2rem;
  border: 1px dashed rgb(var(--border));
  border-radius: var(--radius-card);
  background: rgb(var(--surface) / 0.76);
  text-align: center;
}

.sp-posts__empty-icon {
  display: grid;
  width: 3rem;
  height: 3rem;
  place-items: center;
  border-radius: 999px;
  color: rgb(var(--primary));
  background: rgb(var(--primary) / 0.12);
}

.sp-posts__empty h2 {
  margin: 0.8rem 0 0;
  color: rgb(var(--text));
  font-size: 1rem;
}

.sp-posts__empty p {
  max-width: 30rem;
  margin: 0.35rem 0 0;
  color: rgb(var(--muted));
  font-size: 0.86rem;
  line-height: 1.5;
}

@media (max-width: 1050px) {
  .sp-posts__grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 640px) {
  .sp-posts__grid {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
