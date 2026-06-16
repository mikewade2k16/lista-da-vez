<script setup lang="ts">
import { computed } from 'vue'

import BioSectionCard from '~/components/bio/BioSectionCard.vue'
import type { BioData, BioMeta } from '~/domain/bio/types'

// Secao meta: lang, title, favicon, gtmId. Escreve no bloco meta do draft,
// garantindo que o objeto exista antes de gravar.

const props = defineProps<{ draft: BioData }>()
const emit = defineEmits<{ (e: 'update:draft', value: BioData): void }>()

const meta = computed<BioMeta>(() => props.draft.meta || {})

function setField(key: keyof BioMeta, value: string) {
  emit('update:draft', {
    ...props.draft,
    meta: { ...(props.draft.meta || {}), [key]: value },
  })
}
</script>

<template>
  <BioSectionCard
    title="Meta"
    description="Informacoes basicas da pagina: idioma, titulo do navegador, favicon e ID do Google Tag Manager."
  >
    <div class="bio-section-grid">
      <div class="bio-field">
        <label class="bio-field__label">Idioma (lang)</label>
        <UInput
          :model-value="meta.lang || ''"
          placeholder="pt-BR"
          @update:model-value="setField('lang', String($event ?? ''))"
        />
      </div>
      <div class="bio-field">
        <label class="bio-field__label">Titulo</label>
        <UInput
          :model-value="meta.title || ''"
          placeholder="Titulo exibido na aba do navegador"
          @update:model-value="setField('title', String($event ?? ''))"
        />
      </div>
      <div class="bio-field">
        <label class="bio-field__label">Favicon (URL)</label>
        <UInput
          :model-value="meta.favicon || ''"
          placeholder="/uploads/bio/.../favicon.png"
          @update:model-value="setField('favicon', String($event ?? ''))"
        />
      </div>
      <div class="bio-field">
        <label class="bio-field__label">GTM ID</label>
        <UInput
          :model-value="meta.gtmId || ''"
          placeholder="GTM-XXXXXXX"
          @update:model-value="setField('gtmId', String($event ?? ''))"
        />
      </div>
    </div>
  </BioSectionCard>
</template>

<style scoped>
.bio-section-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 0.6rem 0.85rem;
}

.bio-field {
  display: grid;
  gap: 0.3rem;
}

.bio-field__label {
  font-size: 0.72rem;
  font-weight: 700;
  color: var(--text-muted);
}
</style>
