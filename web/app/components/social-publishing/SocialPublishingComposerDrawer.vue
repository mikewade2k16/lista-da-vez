<script setup lang="ts">
import {
  createSocialPublishingIdempotencyKey,
  isHttpsMediaUrl,
  type SocialPublishingPost,
  type SocialPublishingPostInput,
} from '~/domain/social-publishing/model'

type DrawerMode = 'side' | 'center' | 'fullscreen'

const props = defineProps<{
  modelValue: boolean
  post: SocialPublishingPost | null
  busy: boolean
  error: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  save: [input: SocialPublishingPostInput]
  schedule: [input: SocialPublishingPostInput]
}>()

const drawerMode = ref<DrawerMode>('side')
const drawerWidth = ref(720)
const caption = ref('')
const mediaUrl = ref('')
const altText = ref('')
const scheduledLocal = ref('')
const timezone = ref('UTC')
const idempotencyKey = ref('')
const validationError = ref('')

const open = computed({
  get: () => props.modelValue,
  set: (value: boolean) => emit('update:modelValue', value),
})

const title = computed(() => (props.post ? 'Editar publicação' : 'Nova publicação'))
const draftButtonLabel = computed(() =>
  props.post?.status === 'scheduled' ? 'Salvar como rascunho' : 'Salvar rascunho',
)
const counterLabel = computed(() => `${caption.value.length}/2.200`)
const minSchedule = computed(() => toLocalInput(new Date(Date.now() + 60_000).toISOString()))

function resolveTimezone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'
  } catch {
    return 'UTC'
  }
}

function toLocalInput(value: string | null): string {
  if (!value) return ''
  const date = new Date(value)
  if (!Number.isFinite(date.getTime())) return ''
  const pad = (part: number) => String(part).padStart(2, '0')
  return [
    date.getFullYear(),
    '-',
    pad(date.getMonth() + 1),
    '-',
    pad(date.getDate()),
    'T',
    pad(date.getHours()),
    ':',
    pad(date.getMinutes()),
  ].join('')
}

function resetForm(): void {
  const post = props.post
  caption.value = post?.caption || ''
  mediaUrl.value = post?.mediaUrl || ''
  altText.value = post?.altText || ''
  scheduledLocal.value = toLocalInput(post?.scheduledFor || null)
  timezone.value = post?.timezone || resolveTimezone()
  idempotencyKey.value = createSocialPublishingIdempotencyKey()
  validationError.value = ''
}

function buildInput(status: 'draft' | 'scheduled'): SocialPublishingPostInput | null {
  const normalizedCaption = caption.value.trim()
  const normalizedUrl = mediaUrl.value.trim()
  if (!normalizedCaption) {
    validationError.value = 'Escreva a legenda da publicação.'
    return null
  }
  if (!isHttpsMediaUrl(normalizedUrl)) {
    validationError.value = 'Informe uma URL pública que comece com HTTPS.'
    return null
  }

  let scheduledFor: string | null = null
  if (status === 'scheduled') {
    if (!scheduledLocal.value) {
      validationError.value = 'Escolha a data e a hora da publicação.'
      return null
    }
    const date = new Date(scheduledLocal.value)
    if (!Number.isFinite(date.getTime()) || date.getTime() <= Date.now()) {
      validationError.value = 'O horário da publicação precisa estar no futuro.'
      return null
    }
    scheduledFor = date.toISOString()
  }

  validationError.value = ''
  return {
    idempotencyKey: idempotencyKey.value,
    mediaType: 'image',
    caption: normalizedCaption,
    mediaUrl: normalizedUrl,
    altText: altText.value.trim(),
    status,
    scheduledFor,
    timezone: timezone.value || 'UTC',
    version: props.post?.version,
  }
}

function submitDraft(): void {
  const input = buildInput('draft')
  if (input) emit('save', input)
}

function submitSchedule(): void {
  const input = buildInput('scheduled')
  if (input) emit('schedule', input)
}

watch(
  () => [props.modelValue, props.post?.id] as const,
  ([isOpen]) => {
    if (isOpen) resetForm()
  },
  { immediate: true },
)
</script>

<template>
  <OmniEntityDrawer
    v-model="open"
    v-model:mode="drawerMode"
    v-model:width="drawerWidth"
    :title="title"
    subtitle="Instagram · publicação de imagem"
  >
    <form class="sp-composer" @submit.prevent="submitSchedule">
      <section class="sp-composer__section" aria-labelledby="sp-format-title">
        <div class="sp-composer__section-head">
          <div>
            <h2 id="sp-format-title">Formato</h2>
            <p>O piloto publica uma imagem por vez.</p>
          </div>
          <span class="sp-composer__beta">Beta</span>
        </div>

        <div class="sp-formats">
          <div class="sp-format sp-format--active">
            <UIcon name="i-lucide-image" aria-hidden="true" />
            <strong>Imagem</strong>
            <span>Disponível</span>
          </div>
          <div class="sp-format sp-format--future">
            <UIcon name="i-lucide-clapperboard" aria-hidden="true" />
            <strong>Reels</strong>
            <span>Próximo formato</span>
          </div>
          <div class="sp-format sp-format--future">
            <UIcon name="i-lucide-gallery-horizontal" aria-hidden="true" />
            <strong>Carrossel</strong>
            <span>Próximo formato</span>
          </div>
        </div>
      </section>

      <section class="sp-composer__section" aria-labelledby="sp-content-title">
        <div class="sp-composer__section-head">
          <div>
            <h2 id="sp-content-title">Conteúdo</h2>
            <p>Use uma imagem pública e acessível pelo Instagram.</p>
          </div>
        </div>

        <label class="sp-field">
          <span>URL HTTPS da imagem</span>
          <input
            v-model="mediaUrl"
            type="url"
            inputmode="url"
            autocomplete="url"
            placeholder="https://..."
            maxlength="2048"
            :disabled="busy"
            required
          />
          <small>O arquivo precisa continuar disponível até a publicação.</small>
        </label>

        <label class="sp-field">
          <span>Legenda</span>
          <textarea
            v-model="caption"
            rows="7"
            maxlength="2200"
            :disabled="busy"
            required
            aria-describedby="sp-caption-counter"
          ></textarea>
          <small id="sp-caption-counter">{{ counterLabel }}</small>
        </label>

        <label class="sp-field">
          <span>
            Texto alternativo
            <em>opcional</em>
          </span>
          <textarea
            v-model="altText"
            rows="3"
            maxlength="1000"
            :disabled="busy"
            placeholder="Descreva a imagem para acessibilidade."
          ></textarea>
        </label>
      </section>

      <section class="sp-composer__section" aria-labelledby="sp-schedule-title">
        <div class="sp-composer__section-head">
          <div>
            <h2 id="sp-schedule-title">Agendamento</h2>
            <p>O horário será registrado com o fuso abaixo.</p>
          </div>
        </div>

        <div class="sp-composer__schedule">
          <label class="sp-field">
            <span>Data e hora</span>
            <input
              v-model="scheduledLocal"
              type="datetime-local"
              :min="minSchedule"
              :disabled="busy"
            />
          </label>
          <label class="sp-field">
            <span>Fuso horário</span>
            <input v-model="timezone" type="text" readonly aria-readonly="true" />
          </label>
        </div>
      </section>

      <p v-if="validationError || error" class="sp-composer__error" role="alert">
        <UIcon name="i-lucide-circle-alert" aria-hidden="true" />
        {{ validationError || error }}
      </p>
    </form>

    <template #footer>
      <div class="sp-composer__footer">
        <UButton
          type="button"
          color="neutral"
          variant="ghost"
          label="Cancelar"
          :disabled="busy"
          @click="open = false"
        />
        <div class="sp-composer__footer-actions">
          <UButton
            type="button"
            color="neutral"
            variant="soft"
            icon="i-lucide-save"
            :label="draftButtonLabel"
            :disabled="busy"
            @click="submitDraft"
          />
          <UButton
            type="button"
            color="primary"
            icon="i-lucide-calendar-check"
            label="Salvar e agendar"
            :loading="busy"
            @click="submitSchedule"
          />
        </div>
      </div>
    </template>
  </OmniEntityDrawer>
</template>

<style scoped>
.sp-composer {
  display: grid;
  gap: 1rem;
}
.sp-composer__section {
  display: grid;
  gap: 0.9rem;
  padding: 1rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-card);
  background: rgb(var(--surface) / 0.74);
}
.sp-composer__section-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}
.sp-composer__section h2,
.sp-composer__section p {
  margin: 0;
}
.sp-composer__section h2 {
  color: rgb(var(--text));
  font-size: 0.96rem;
  font-weight: 750;
}
.sp-composer__section p {
  margin-top: 0.18rem;
  color: rgb(var(--muted));
  font-size: 0.78rem;
}
.sp-composer__beta {
  padding: 0.2rem 0.45rem;
  border-radius: 999px;
  color: rgb(var(--primary));
  background: rgb(var(--primary) / 0.12);
  font-size: 0.68rem;
  font-weight: 750;
  text-transform: uppercase;
}
.sp-formats {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.55rem;
}
.sp-format {
  display: grid;
  justify-items: start;
  gap: 0.3rem;
  min-width: 0;
  padding: 0.75rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-soft);
  color: rgb(var(--muted));
  background: rgb(var(--surface-2) / 0.6);
}
.sp-format--active {
  border-color: rgb(var(--primary) / 0.5);
  color: rgb(var(--primary));
  background: rgb(var(--primary) / 0.08);
}
.sp-format strong {
  color: rgb(var(--text));
  font-size: 0.82rem;
}
.sp-format span {
  font-size: 0.68rem;
}
.sp-field {
  display: grid;
  gap: 0.4rem;
  min-width: 0;
  color: rgb(var(--text));
  font-size: 0.8rem;
  font-weight: 650;
}
.sp-field em,
.sp-field small {
  color: rgb(var(--muted));
  font-size: 0.72rem;
  font-style: normal;
  font-weight: 450;
}
.sp-field input,
.sp-field textarea {
  width: 100%;
  min-width: 0;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-xs);
  outline: none;
  color: rgb(var(--text));
  background: rgb(var(--surface));
  font: inherit;
  font-weight: 450;
}
.sp-field input {
  min-height: 2.7rem;
  padding: 0 0.75rem;
}
.sp-field textarea {
  padding: 0.7rem 0.75rem;
  line-height: 1.5;
  resize: vertical;
}
.sp-field input:focus,
.sp-field textarea:focus {
  border-color: rgb(var(--ring));
  box-shadow: 0 0 0 3px rgb(var(--ring) / 0.14);
}
.sp-field input:disabled,
.sp-field textarea:disabled {
  cursor: not-allowed;
  opacity: 0.65;
}
.sp-composer__schedule {
  display: grid;
  grid-template-columns: minmax(0, 1.35fr) minmax(0, 1fr);
  gap: 0.65rem;
}
.sp-composer__error {
  display: flex;
  align-items: flex-start;
  gap: 0.45rem;
  margin: 0;
  padding: 0.7rem 0.8rem;
  border-radius: var(--radius-xs);
  color: rgb(var(--danger));
  background: rgb(var(--danger) / 0.1);
  font-size: 0.8rem;
}
.sp-composer__footer,
.sp-composer__footer-actions {
  display: flex;
  align-items: center;
  gap: 0.55rem;
}
.sp-composer__footer {
  width: 100%;
  justify-content: space-between;
}
@media (max-width: 620px) {
  .sp-formats,
  .sp-composer__schedule {
    grid-template-columns: minmax(0, 1fr);
  }
  .sp-composer__footer {
    align-items: stretch;
    flex-direction: column-reverse;
  }
  .sp-composer__footer-actions {
    align-items: stretch;
    flex-direction: column-reverse;
  }
}
</style>
