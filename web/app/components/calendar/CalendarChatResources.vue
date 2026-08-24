<script setup lang="ts">
import { ref } from 'vue'
import type { AssistantResource } from '~/domain/calendar/calendar-chat-api'

const props = defineProps<{ resources: AssistantResource[] }>()
const emit = defineEmits<{ use: [resource: AssistantResource] }>()

const failedImages = ref<Set<string>>(new Set())

const metadataLabels: Record<string, string> = {
  username: 'Perfil',
  mediaType: 'Formato',
  timestamp: 'Publicado em',
  objective: 'Objetivo',
  dailyBudget: 'Orçamento diário',
  lifetimeBudget: 'Orçamento total',
  adAccountName: 'Conta',
  adAccountId: 'Conta interna',
  metaCampaignId: 'ID da campanha',
  syncedAt: 'Sincronizado em',
  currency: 'Moeda',
  clientAccountId: 'Cliente',
  metaAdAccountId: 'ID Meta',
}

const metadataOrder: Record<AssistantResource['kind'], string[]> = {
  instagram_post: ['username', 'mediaType', 'timestamp'],
  meta_campaign: [
    'objective',
    'dailyBudget',
    'lifetimeBudget',
    'adAccountName',
    'syncedAt',
    'metaCampaignId',
    'adAccountId',
  ],
  meta_ad_account: ['currency', 'metaAdAccountId', 'clientAccountId'],
}

function kindLabel(resource: AssistantResource): string {
  if (resource.kind === 'instagram_post') return 'Instagram'
  if (resource.kind === 'meta_campaign') return 'Campanha Meta'
  return 'Conta de anuncios'
}

function kindIcon(resource: AssistantResource): string {
  if (resource.kind === 'instagram_post') return 'i-lucide-instagram'
  if (resource.kind === 'meta_campaign') return 'i-lucide-megaphone'
  return 'i-lucide-badge-dollar-sign'
}

function actionLabel(resource: AssistantResource): string {
  if (resource.kind === 'instagram_post') return 'Usar este post'
  if (resource.kind === 'meta_campaign') return 'Usar esta campanha'
  return 'Usar esta conta'
}

function metadataRows(
  resource: AssistantResource,
): Array<{ key: string; label: string; value: string }> {
  return metadataOrder[resource.kind]
    .map((key) => [key, resource.metadata?.[key]] as const)
    .filter((entry): entry is readonly [string, string] => {
      return Boolean(metadataLabels[entry[0]] && String(entry[1] || '').trim())
    })
    .slice(0, 3)
    .map(([key, value]) => ({
      key,
      label: metadataLabels[key]!,
      value: metadataValue(resource, key, value),
    }))
}

function metadataValue(resource: AssistantResource, key: string, value: string): string {
  if (key === 'dailyBudget' || key === 'lifetimeBudget') {
    const amount = Number(value)
    if (Number.isFinite(amount)) {
      const currency = String(resource.metadata?.currency || '')
        .trim()
        .toUpperCase()
      if (/^[A-Z]{3}$/.test(currency)) {
        try {
          return new Intl.NumberFormat('pt-BR', {
            style: 'currency',
            currency,
          }).format(amount)
        } catch {
          // Codigo remoto desconhecido: mantemos o valor numerico legivel.
        }
      }
      return new Intl.NumberFormat('pt-BR', {
        minimumFractionDigits: 2,
        maximumFractionDigits: 2,
      }).format(amount)
    }
  }

  if (key === 'timestamp' || key === 'syncedAt') {
    const instant = new Date(value)
    if (!Number.isNaN(instant.getTime())) {
      return new Intl.DateTimeFormat('pt-BR', {
        dateStyle: 'short',
        timeStyle: 'short',
      }).format(instant)
    }
  }

  return value
}

function safeLink(url: string): string {
  return url.startsWith('https://') ? url : ''
}

function imageFailed(id: string): void {
  failedImages.value = new Set(failedImages.value).add(id)
}
</script>

<template>
  <section
    v-if="props.resources.length"
    class="calendar-chat__resources"
    aria-label="Recursos encontrados"
  >
    <header class="calendar-chat__resources-head">
      <span>
        <UIcon name="i-lucide-layout-grid" aria-hidden="true" />
        Recursos
      </span>
      <strong>{{ props.resources.length }}</strong>
    </header>

    <div class="calendar-chat__resources-list">
      <article
        v-for="resource in props.resources"
        :key="resource.id"
        class="calendar-chat__resource"
        :class="`calendar-chat__resource--${resource.kind}`"
      >
        <a
          v-if="
            safeLink(resource.imageUrl) &&
            !failedImages.has(resource.id) &&
            safeLink(resource.permalink)
          "
          :href="safeLink(resource.permalink)"
          class="calendar-chat__resource-media"
          target="_blank"
          rel="noopener noreferrer"
          aria-label="Abrir publicacao no Instagram"
        >
          <img
            :src="safeLink(resource.imageUrl)"
            :alt="resource.title"
            loading="lazy"
            @error="imageFailed(resource.id)"
          />
        </a>
        <div
          v-else-if="safeLink(resource.imageUrl) && !failedImages.has(resource.id)"
          class="calendar-chat__resource-media"
        >
          <img
            :src="safeLink(resource.imageUrl)"
            :alt="resource.title"
            loading="lazy"
            @error="imageFailed(resource.id)"
          />
        </div>

        <div class="calendar-chat__resource-body">
          <div class="calendar-chat__resource-kicker">
            <span>
              <UIcon :name="kindIcon(resource)" aria-hidden="true" />
              {{ kindLabel(resource) }}
            </span>
            <span v-if="resource.status" class="calendar-chat__resource-status">
              {{ resource.status }}
            </span>
          </div>
          <strong>{{ resource.title }}</strong>
          <p v-if="resource.subtitle">{{ resource.subtitle }}</p>
          <dl v-if="metadataRows(resource).length" class="calendar-chat__resource-meta">
            <div v-for="row in metadataRows(resource)" :key="row.key">
              <dt>{{ row.label }}</dt>
              <dd>{{ row.value }}</dd>
            </div>
          </dl>
          <div class="calendar-chat__resource-actions">
            <a
              v-if="safeLink(resource.permalink)"
              :href="safeLink(resource.permalink)"
              target="_blank"
              rel="noopener noreferrer"
            >
              Abrir origem
              <UIcon name="i-lucide-external-link" aria-hidden="true" />
            </a>
            <button type="button" @click="emit('use', resource)">
              <UIcon name="i-lucide-message-square-plus" aria-hidden="true" />
              {{ actionLabel(resource) }}
            </button>
          </div>
        </div>
      </article>
    </div>
  </section>
</template>
