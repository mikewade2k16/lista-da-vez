<script setup lang="ts">
import {
  isHttpsMediaUrl,
  type SocialPublishingOverview,
  type SocialPublishingPost,
} from '~/domain/social-publishing/model'

const props = defineProps<{
  overview: SocialPublishingOverview | null
  posts: SocialPublishingPost[]
  syncing: boolean
  canSync: boolean
}>()

const emit = defineEmits<{
  sync: []
}>()

const numberFormatter = new Intl.NumberFormat('pt-BR')
const dateFormatter = new Intl.DateTimeFormat('pt-BR', {
  dateStyle: 'medium',
  timeStyle: 'short',
})

const publishedPosts = computed(() =>
  props.posts
    .filter((post) => post.status === 'published')
    .sort(
      (left, right) =>
        Date.parse(right.publishedAt || right.updatedAt || '') -
        Date.parse(left.publishedAt || left.updatedAt || ''),
    ),
)

const primaryMetrics = computed(() => [
  {
    label: 'Visualizações',
    value: props.overview?.views ?? 0,
    icon: 'i-lucide-play',
  },
  {
    label: 'Alcance',
    value: props.overview?.reach ?? 0,
    icon: 'i-lucide-users',
  },
  {
    label: 'Interações',
    value: props.overview?.totalInteractions ?? 0,
    icon: 'i-lucide-mouse-pointer-click',
  },
])

const engagementMetrics = computed(() => [
  { label: 'Curtidas', value: props.overview?.likes ?? 0 },
  { label: 'Comentários', value: props.overview?.comments ?? 0 },
  { label: 'Salvos', value: props.overview?.saved ?? 0 },
  { label: 'Compartilhamentos', value: props.overview?.shares ?? 0 },
])

function formatDate(value: string | null): string {
  if (!value) return 'Aguardando sincronização'
  const timestamp = Date.parse(value)
  return Number.isFinite(timestamp) ? dateFormatter.format(new Date(timestamp)) : 'Data inválida'
}
</script>

<template>
  <section class="sp-analytics" aria-labelledby="sp-analytics-title">
    <div class="sp-analytics__head">
      <div>
        <h2 id="sp-analytics-title">Desempenho do Instagram</h2>
        <p v-if="overview?.capturedAt">
          Dados capturados em
          <time :datetime="overview.capturedAt">{{ formatDate(overview.capturedAt) }}</time>
        </p>
        <p v-else>Os dados aparecerão depois da primeira sincronização.</p>
      </div>
      <UButton
        v-if="canSync"
        type="button"
        color="neutral"
        variant="soft"
        icon="i-lucide-refresh-cw"
        label="Sincronizar analytics"
        :loading="syncing"
        @click="emit('sync')"
      />
    </div>

    <div class="sp-analytics__primary">
      <article v-for="metric in primaryMetrics" :key="metric.label" class="omni-glass">
        <div aria-hidden="true"><UIcon :name="metric.icon" /></div>
        <p>{{ metric.label }}</p>
        <strong>{{ numberFormatter.format(metric.value) }}</strong>
      </article>
    </div>

    <div class="sp-analytics__engagement omni-glass">
      <div v-for="metric in engagementMetrics" :key="metric.label">
        <span>{{ metric.label }}</span>
        <strong>{{ numberFormatter.format(metric.value) }}</strong>
      </div>
    </div>

    <div class="sp-analytics__section-head">
      <div>
        <h2>Resultado por publicação</h2>
        <p>As métricas abaixo pertencem somente a posts já publicados.</p>
      </div>
      <span>{{ publishedPosts.length }} publicadas</span>
    </div>

    <div v-if="publishedPosts.length" class="sp-analytics__list">
      <article v-for="post in publishedPosts" :key="post.id" class="sp-analytics-post omni-glass">
        <div class="sp-analytics-post__identity">
          <img
            v-if="isHttpsMediaUrl(post.mediaUrl)"
            :src="post.mediaUrl"
            :alt="post.altText || 'Miniatura da publicação'"
            loading="lazy"
          />
          <div v-else class="sp-analytics-post__placeholder" aria-hidden="true">
            <UIcon name="i-lucide-image" />
          </div>
          <div>
            <p>{{ post.caption || 'Sem legenda' }}</p>
            <time v-if="post.publishedAt" :datetime="post.publishedAt">
              {{ formatDate(post.publishedAt) }}
            </time>
          </div>
        </div>

        <dl v-if="post.analytics" class="sp-analytics-post__metrics">
          <div>
            <dt>Visualizações</dt>
            <dd>{{ numberFormatter.format(post.analytics.views) }}</dd>
          </div>
          <div>
            <dt>Alcance</dt>
            <dd>{{ numberFormatter.format(post.analytics.reach) }}</dd>
          </div>
          <div>
            <dt>Interações</dt>
            <dd>{{ numberFormatter.format(post.analytics.totalInteractions) }}</dd>
          </div>
          <div>
            <dt>Curtidas</dt>
            <dd>{{ numberFormatter.format(post.analytics.likes) }}</dd>
          </div>
          <div>
            <dt>Comentários</dt>
            <dd>{{ numberFormatter.format(post.analytics.comments) }}</dd>
          </div>
          <div>
            <dt>Salvos</dt>
            <dd>{{ numberFormatter.format(post.analytics.saved) }}</dd>
          </div>
          <div>
            <dt>Compartilhamentos</dt>
            <dd>{{ numberFormatter.format(post.analytics.shares) }}</dd>
          </div>
        </dl>
        <p v-else class="sp-analytics-post__waiting">
          <UIcon name="i-lucide-clock-3" aria-hidden="true" />
          Analytics ainda não sincronizados para esta publicação.
        </p>
      </article>
    </div>

    <div v-else class="sp-analytics__empty omni-glass">
      <UIcon name="i-lucide-chart-no-axes-column" aria-hidden="true" />
      <h2>Ainda não há publicações com analytics</h2>
      <p>Depois da primeira postagem, o histórico de desempenho aparecerá aqui.</p>
    </div>
  </section>
</template>

<style scoped>
.sp-analytics {
  display: grid;
  gap: 1rem;
}

.sp-analytics__head,
.sp-analytics__section-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.sp-analytics h2,
.sp-analytics p {
  margin: 0;
}

.sp-analytics h2 {
  color: rgb(var(--text));
  font-size: 1rem;
}

.sp-analytics__head p,
.sp-analytics__section-head p {
  margin-top: 0.22rem;
  color: rgb(var(--muted));
  font-size: 0.78rem;
}

.sp-analytics__primary {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.8rem;
}

.sp-analytics__primary article {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 0.25rem 0.65rem;
  padding: 1rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface));
}

.sp-analytics__primary article > div {
  display: grid;
  width: 2.25rem;
  height: 2.25rem;
  grid-row: span 2;
  place-items: center;
  border-radius: var(--radius-soft);
  color: rgb(var(--primary));
  background: rgb(var(--primary) / 0.12);
}

.sp-analytics__primary p {
  color: rgb(var(--muted));
  font-size: 0.75rem;
}

.sp-analytics__primary strong {
  color: rgb(var(--text));
  font-size: 1.45rem;
  line-height: 1;
}

.sp-analytics__engagement {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface));
}

.sp-analytics__engagement div {
  display: grid;
  gap: 0.22rem;
  padding: 0.85rem 1rem;
  border-right: 1px solid var(--line-soft);
}

.sp-analytics__engagement div:last-child {
  border-right: 0;
}

.sp-analytics__engagement span,
.sp-analytics__section-head > span {
  color: rgb(var(--muted));
  font-size: 0.72rem;
}

.sp-analytics__engagement strong {
  color: rgb(var(--text));
  font-size: 1rem;
}

.sp-analytics__section-head {
  margin-top: 0.5rem;
}

.sp-analytics__section-head > span {
  padding: 0.3rem 0.55rem;
  border-radius: 999px;
  background: rgb(var(--surface-2));
}

.sp-analytics__list {
  display: grid;
  gap: 0.7rem;
}

.sp-analytics-post {
  display: grid;
  grid-template-columns: minmax(13rem, 0.8fr) minmax(0, 1.6fr);
  gap: 1rem;
  padding: 0.8rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface));
}

.sp-analytics-post__identity {
  display: flex;
  align-items: center;
  min-width: 0;
  gap: 0.7rem;
}

.sp-analytics-post__identity img,
.sp-analytics-post__placeholder {
  width: 3.6rem;
  height: 3.6rem;
  flex: 0 0 auto;
  border-radius: var(--radius-xs);
  background: rgb(var(--surface-2));
}

.sp-analytics-post__identity img {
  object-fit: cover;
}

.sp-analytics-post__placeholder {
  display: grid;
  place-items: center;
  color: rgb(var(--muted));
}

.sp-analytics-post__identity > div {
  min-width: 0;
}

.sp-analytics-post__identity p {
  overflow: hidden;
  color: rgb(var(--text));
  font-size: 0.8rem;
  font-weight: 650;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sp-analytics-post__identity time {
  color: rgb(var(--muted));
  font-size: 0.7rem;
}

.sp-analytics-post__metrics {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 0.5rem;
  margin: 0;
}

.sp-analytics-post__metrics div {
  min-width: 0;
}

.sp-analytics-post__metrics dt {
  overflow: hidden;
  color: rgb(var(--muted));
  font-size: 0.66rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sp-analytics-post__metrics dd {
  margin: 0.15rem 0 0;
  color: rgb(var(--text));
  font-size: 0.85rem;
  font-weight: 700;
}

.sp-analytics-post__waiting {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  color: rgb(var(--muted));
  font-size: 0.76rem;
}

.sp-analytics__empty {
  display: grid;
  min-height: 13rem;
  place-content: center;
  justify-items: center;
  padding: 2rem;
  border: 1px dashed rgb(var(--border));
  border-radius: var(--radius-card);
  color: rgb(var(--muted));
  background: rgb(var(--surface) / 0.75);
  text-align: center;
}

.sp-analytics__empty :deep(svg) {
  width: 2rem;
  height: 2rem;
  color: rgb(var(--primary));
}

.sp-analytics__empty h2 {
  margin-top: 0.7rem;
}

.sp-analytics__empty p {
  margin-top: 0.3rem;
  font-size: 0.8rem;
}

@media (max-width: 850px) {
  .sp-analytics-post {
    grid-template-columns: minmax(0, 1fr);
  }
}

@media (max-width: 680px) {
  .sp-analytics__primary,
  .sp-analytics__engagement {
    grid-template-columns: minmax(0, 1fr);
  }

  .sp-analytics__engagement div {
    border-right: 0;
    border-bottom: 1px solid var(--line-soft);
  }

  .sp-analytics__engagement div:last-child {
    border-bottom: 0;
  }

  .sp-analytics-post__metrics {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .sp-analytics__head {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>
