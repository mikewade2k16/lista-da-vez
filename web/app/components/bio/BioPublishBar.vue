<script setup lang="ts">
import { computed } from 'vue'

import type { BioStatus } from '~/domain/bio/types'

// Faixa de status/publicacao no topo do editor. Auto-save: NAO ha botao Salvar
// (o rascunho persiste sozinho) nem botao Previa (a previa ao vivo ja e a
// previa). Mantem Republicar / Despublicar (ir ao ar e decisao manual). Inclui
// um switch "Editando <-> Publicado" que controla a fonte do preview e um botao
// "Desfazer" (undo), alem do indicador de auto-save.

type SaveState = 'idle' | 'saving' | 'saved' | 'error'

const props = defineProps<{
  status: BioStatus
  slug: string
  dirty: boolean
  unpublishedChanges?: boolean
  saving: boolean
  publishing: boolean
  saveState: SaveState
  canUndo: boolean
}>()

const emit = defineEmits<{
  (e: 'publish' | 'unpublish' | 'undo'): void
}>()

const isPublished = computed(() => props.status === 'published')

// Indicador de auto-save: "Salvando..." durante o PATCH, "Salvo" apos sucesso,
// "Erro ao salvar" em falha. Sem alteracoes pendentes => sem ruido.
const saveLabel = computed(() => {
  if (props.saving || props.saveState === 'saving') {
    return 'Salvando...'
  }
  if (props.saveState === 'error') {
    return 'Erro ao salvar'
  }
  if (props.saveState === 'saved') {
    return 'Salvo'
  }
  return ''
})

// Link da bio publica: usa NUXT_PUBLIC_BIO_FRONT_URL se configurada (wiring de
// deploy). Sem ela, o link nao aparece. URL sem /bio (alinhado ao subagente D).
const bioFrontUrl = computed(() => {
  const config = useRuntimeConfig()
  const base = String((config.public as Record<string, unknown>).bioFrontUrl || '').trim()
  if (!base || !props.slug) {
    return ''
  }
  return `${base.replace(/\/$/, '')}/${props.slug}`
})
</script>

<template>
  <div class="bio-publish-bar">
    <div class="bio-publish-bar__status">
      <span
        class="bio-publish-bar__badge"
        :class="isPublished ? 'bio-publish-bar__badge--published' : 'bio-publish-bar__badge--draft'"
      >
        {{ isPublished ? 'Publicada' : 'Rascunho' }}
      </span>
      <span v-if="dirty" class="bio-publish-bar__dirty">{{ saveLabel || 'Editando...' }}</span>
      <span v-else-if="isPublished && unpublishedChanges" class="bio-publish-bar__dirty">
        Alteracoes salvas — clique em Republicar para publicar
      </span>
      <span v-else-if="saveLabel" class="bio-publish-bar__saved">{{ saveLabel }}</span>
      <a
        v-if="bioFrontUrl"
        class="bio-publish-bar__link"
        :href="bioFrontUrl"
        target="_blank"
        rel="noopener"
        title="Ver online (nova aba)"
        aria-label="Ver online"
      >
        <UIcon name="i-lucide-external-link" />
      </a>
    </div>

    <div class="bio-publish-bar__actions">
      <UButton
        icon="i-lucide-undo-2"
        color="neutral"
        variant="ghost"
        size="sm"
        title="Desfazer (Ctrl+Z)"
        aria-label="Desfazer"
        :disabled="!canUndo"
        @click="emit('undo')"
      />
      <UButton
        icon="i-lucide-rocket"
        color="primary"
        size="sm"
        :label="isPublished ? 'Republicar' : 'Publicar'"
        :loading="publishing"
        :disabled="publishing"
        @click="emit('publish')"
      />
      <UButton
        v-if="isPublished"
        icon="i-lucide-eye-off"
        color="warning"
        variant="soft"
        size="sm"
        title="Despublicar"
        aria-label="Despublicar"
        :loading="publishing"
        :disabled="publishing"
        @click="emit('unpublish')"
      />
    </div>
  </div>
</template>

<style scoped>
.bio-publish-bar {
  display: inline-flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.6rem;
}

.bio-publish-bar__status {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.bio-publish-bar__badge {
  display: inline-flex;
  align-items: center;
  padding: 0.2rem 0.65rem;
  border-radius: 999px;
  font-size: 0.74rem;
  font-weight: 700;
}

.bio-publish-bar__badge--published {
  background: rgb(var(--success) / 0.16);
  color: rgb(var(--success));
}

.bio-publish-bar__badge--draft {
  background: rgb(var(--muted) / 0.16);
  color: var(--text-muted);
}

.bio-publish-bar__dirty {
  font-size: 0.78rem;
  font-weight: 600;
  color: var(--accent-warning);
}

.bio-publish-bar__saved {
  font-size: 0.78rem;
  color: rgb(var(--success));
}

.bio-publish-bar__link {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  font-size: 0.82rem;
  font-weight: 600;
  color: rgb(var(--primary));
  text-decoration: none;
}

.bio-publish-bar__link:hover {
  text-decoration: underline;
}

.bio-publish-bar__actions {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  flex-wrap: wrap;
}
</style>
