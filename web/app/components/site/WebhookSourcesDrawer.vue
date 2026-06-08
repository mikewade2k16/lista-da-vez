<script setup lang="ts">
import type { WebhookEntityType } from '~/types/webhook-sources'

const props = defineProps<{
  open: boolean
  defaultEntity?: WebhookEntityType
}>()
const emit = defineEmits<{
  'update:open': [value: boolean]
}>()

const {
  sources,
  loading,
  creating,
  errorMessage,
  lastSecret,
  lastSecretFor,
  fetchSources,
  createSource,
  rotateSecret,
  deleteSource,
  clearSecret,
} = useWebhookSourcesManager()

const createForm = reactive({
  slug: '',
  name: '',
  entityType: props.defaultEntity || ('leads' as WebhookEntityType),
})

watch(
  () => props.open,
  (isOpen) => {
    if (isOpen) {
      void fetchSources()
      clearSecret()
      createForm.entityType = props.defaultEntity || 'leads'
    }
  },
)

const filteredSources = computed(() =>
  sources.value.filter((s) => (props.defaultEntity ? s.entityType === props.defaultEntity : true)),
)

async function onCreate() {
  if (!createForm.slug.trim() || !createForm.name.trim()) return
  const result = await createSource({ ...createForm })
  if (!result) return
  createForm.slug = ''
  createForm.name = ''
}

async function onRotate(id: string) {
  if (
    import.meta.client &&
    !window.confirm('Gerar nova chave para esta fonte? A anterior para de funcionar.')
  )
    return
  await rotateSecret(id)
}

async function onDelete(id: string) {
  if (
    import.meta.client &&
    !window.confirm('Desativar esta fonte? Webhooks recebidos com a chave dela vao falhar.')
  )
    return
  await deleteSource(id)
}

function copySecret() {
  if (!import.meta.client || !lastSecret.value) return
  navigator.clipboard?.writeText(lastSecret.value).catch(() => {})
}

function webhookUrl(slug: string, entityType: WebhookEntityType) {
  const runtimeConfig = useRuntimeConfig()
  const base = String(runtimeConfig.public.apiBase || '').replace(/\/$/, '')
  return `${base}/v1/webhooks/${entityType}/${slug}`
}

function close() {
  emit('update:open', false)
}
</script>

<template>
  <USlideover :open="props.open" side="right" @update:open="emit('update:open', $event)">
    <template #content>
      <div class="webhook-sources-drawer flex h-full flex-col gap-3 p-4">
        <div class="webhook-sources-drawer__header flex items-center justify-between">
          <h3 class="text-base font-semibold">Webhook sources</h3>
          <UButton icon="i-lucide-x" color="neutral" variant="ghost" size="sm" @click="close" />
        </div>

        <p class="text-xs text-[rgb(var(--muted))]">
          Cada fonte de webhook ganha uma URL + chave secreta. O servico externo (Shopify, Typeform,
          formulario customizado, etc.) precisa enviar
          <code>X-Signature: sha256=&lt;hex(HMAC_SHA256(body, secret))&gt;</code>
          para autenticar o POST.
        </p>

        <UAlert
          v-if="errorMessage"
          color="error"
          variant="soft"
          icon="i-lucide-alert-triangle"
          title="Erro"
          :description="errorMessage"
        />

        <UAlert
          v-if="lastSecret"
          color="warning"
          variant="soft"
          icon="i-lucide-key"
          title="Secret revelado uma unica vez"
          :description="`Copie agora. Apos fechar este painel, essa chave nao sera mais mostrada — use rotate para gerar uma nova.`"
        >
          <template #actions>
            <div class="flex items-center gap-2">
              <code
                class="block break-all rounded-[var(--radius-sm)] bg-[rgb(var(--surface-2))] px-2 py-1 text-xs"
              >
                {{ lastSecret }}
              </code>
              <UButton
                icon="i-lucide-copy"
                color="primary"
                variant="soft"
                size="xs"
                label="Copiar"
                @click="copySecret"
              />
            </div>
          </template>
        </UAlert>

        <div
          class="webhook-sources-drawer__create rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] p-3 space-y-2"
        >
          <p class="text-xs font-semibold">Cadastrar nova fonte</p>
          <UInput
            :model-value="createForm.name"
            placeholder="Nome (ex.: Typeform Captura)"
            @update:model-value="createForm.name = String($event ?? '')"
          />
          <UInput
            :model-value="createForm.slug"
            placeholder="Slug (ex.: typeform-perola; minimo 3 chars)"
            @update:model-value="createForm.slug = String($event ?? '')"
          />
          <USelect
            v-if="!props.defaultEntity"
            :model-value="createForm.entityType"
            :items="[
              { label: 'Leads', value: 'leads' },
              { label: 'Products', value: 'products' },
              { label: 'Tracking', value: 'tracking' },
            ]"
            @update:model-value="createForm.entityType = $event as WebhookEntityType"
          />
          <UButton
            label="Criar fonte"
            icon="i-lucide-plus"
            color="primary"
            block
            :loading="creating"
            @click="onCreate"
          />
        </div>

        <div class="webhook-sources-drawer__list flex-1 min-h-0 overflow-y-auto space-y-2">
          <p v-if="loading" class="text-xs text-[rgb(var(--muted))]">Carregando...</p>
          <p v-else-if="filteredSources.length === 0" class="text-xs text-[rgb(var(--muted))]">
            Nenhuma fonte cadastrada.
          </p>
          <div
            v-for="source in filteredSources"
            :key="source.id"
            class="webhook-sources-drawer__item rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] p-3 space-y-1"
          >
            <div class="flex items-center justify-between gap-2">
              <p class="text-sm font-medium">{{ source.name }}</p>
              <UBadge :color="source.isActive ? 'success' : 'neutral'" variant="soft" size="xs">
                {{ source.entityType }}
              </UBadge>
            </div>
            <code class="block break-all text-[10px] text-[rgb(var(--muted))]">
              POST {{ webhookUrl(source.slug, source.entityType) }}
            </code>
            <div class="flex items-center justify-end gap-1 pt-1">
              <UButton
                icon="i-lucide-refresh-cw"
                color="primary"
                variant="ghost"
                size="xs"
                title="Gerar nova chave"
                @click="onRotate(source.id)"
              />
              <UButton
                icon="i-lucide-trash-2"
                color="error"
                variant="ghost"
                size="xs"
                title="Desativar fonte"
                @click="onDelete(source.id)"
              />
            </div>
            <p v-if="lastSecretFor === source.id" class="text-[10px] text-[rgb(var(--primary))]">
              Nova chave gerada — copie da box acima.
            </p>
          </div>
        </div>
      </div>
    </template>
  </USlideover>
</template>
