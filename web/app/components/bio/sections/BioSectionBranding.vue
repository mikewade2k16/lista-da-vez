<script setup lang="ts">
import { computed } from 'vue'

import BioMediaField from '~/components/bio/BioMediaField.vue'
import BioSectionCard from '~/components/bio/BioSectionCard.vue'
import type { BioBranding, BioData, BioLogo, BioMediaKind } from '~/domain/bio/types'

// Secao branding: nome do perfil + logo + logo do rodape. O logo.srcMobile e o
// unico campo obrigatorio do bloco (validado na publicacao pelo backend).

const props = defineProps<{
  draft: BioData
  bioId: string
  uploadMedia: (kind: BioMediaKind, file: File) => Promise<string | null>
  isUploading: (kind: BioMediaKind) => boolean
}>()

const emit = defineEmits<{ (e: 'update:draft', value: BioData): void }>()

const branding = computed<BioBranding>(() => props.draft.branding || {})
const logo = computed<BioLogo>(() => props.draft.branding?.logo || ({ srcMobile: '' } as BioLogo))

function setBranding<K extends keyof BioBranding>(key: K, value: BioBranding[K]) {
  emit('update:draft', {
    ...props.draft,
    branding: { ...(props.draft.branding || {}), [key]: value },
  })
}

function setLogo<K extends keyof BioLogo>(key: K, value: BioLogo[K]) {
  const baseLogo = props.draft.branding?.logo || ({ srcMobile: '' } as BioLogo)
  emit('update:draft', {
    ...props.draft,
    branding: {
      ...(props.draft.branding || {}),
      logo: { ...baseLogo, [key]: value },
    },
  })
}

function toNumber(value: string): number {
  const parsed = Number(String(value || '').trim())
  return Number.isFinite(parsed) ? parsed : 0
}
</script>

<template>
  <BioSectionCard
    title="Branding"
    description="Logo, nome do perfil e logo do rodape. O logo mobile e obrigatorio para publicar."
  >
    <div class="bio-section-grid">
      <div class="bio-field">
        <label class="bio-field__label">Nome do perfil</label>
        <UInput
          :model-value="branding.nameProfile || ''"
          placeholder="Nome exibido no topo"
          @update:model-value="setBranding('nameProfile', String($event ?? ''))"
        />
      </div>
      <div class="bio-field bio-field--switch">
        <label class="bio-field__label">Exibir nome do perfil</label>
        <USwitch
          :model-value="branding.nameProfileActive ?? false"
          @update:model-value="setBranding('nameProfileActive', Boolean($event))"
        />
      </div>
      <div class="bio-field">
        <label class="bio-field__label">Margem do nome (desktop)</label>
        <UInput
          type="number"
          :model-value="branding.nameMt ?? 0"
          @update:model-value="setBranding('nameMt', toNumber(String($event ?? '')))"
        />
      </div>
      <div class="bio-field">
        <label class="bio-field__label">Margem do nome (mobile)</label>
        <UInput
          type="number"
          :model-value="branding.nameMtMob ?? 0"
          @update:model-value="setBranding('nameMtMob', toNumber(String($event ?? '')))"
        />
      </div>
    </div>

    <div class="bio-subsection">
      <h3 class="bio-subsection__title">Logo</h3>
      <div class="bio-media-grid">
        <BioMediaField
          label="Logo mobile (obrigatorio)"
          kind="logo"
          accept="image/*"
          :model-value="logo.srcMobile"
          :uploading="isUploading('logo')"
          :on-upload="uploadMedia"
          :duplicate-targets="[{ label: 'Desktop', apply: (v) => setLogo('srcDesktop', v) }]"
          @update:model-value="setLogo('srcMobile', String($event ?? ''))"
        />
        <BioMediaField
          label="Logo desktop"
          kind="logo"
          accept="image/*"
          :model-value="logo.srcDesktop || ''"
          :uploading="isUploading('logo')"
          :on-upload="uploadMedia"
          :duplicate-targets="[{ label: 'Mobile', apply: (v) => setLogo('srcMobile', v) }]"
          @update:model-value="setLogo('srcDesktop', String($event ?? ''))"
        />
      </div>
      <div class="bio-section-grid">
        <div class="bio-field">
          <label class="bio-field__label">Link do logo (href)</label>
          <UInput
            :model-value="logo.href || ''"
            placeholder="https://..."
            @update:model-value="setLogo('href', String($event ?? ''))"
          />
        </div>
        <div class="bio-field">
          <label class="bio-field__label">Largura mobile (px)</label>
          <UInput
            type="number"
            :model-value="logo.widthMobile ?? 0"
            @update:model-value="setLogo('widthMobile', toNumber(String($event ?? '')))"
          />
        </div>
        <div class="bio-field">
          <label class="bio-field__label">Largura desktop (px)</label>
          <UInput
            type="number"
            :model-value="logo.widthDesktop ?? 0"
            @update:model-value="setLogo('widthDesktop', toNumber(String($event ?? '')))"
          />
        </div>
        <div class="bio-field">
          <label class="bio-field__label">Classe de borda</label>
          <UInput
            :model-value="logo.borderRadiusClass || ''"
            placeholder="rounded-full"
            @update:model-value="setLogo('borderRadiusClass', String($event ?? ''))"
          />
        </div>
        <div class="bio-field">
          <label class="bio-field__label">Margem inferior (px)</label>
          <UInput
            type="number"
            :model-value="logo.marginBottom ?? 0"
            @update:model-value="setLogo('marginBottom', toNumber(String($event ?? '')))"
          />
        </div>
      </div>
    </div>

    <div class="bio-subsection">
      <div class="bio-subsection__head">
        <h3 class="bio-subsection__title">Logo do rodape</h3>
        <div class="bio-field bio-field--switch">
          <label class="bio-field__label">Exibir no rodape</label>
          <USwitch
            :model-value="branding.footerLogoActive ?? false"
            @update:model-value="setBranding('footerLogoActive', Boolean($event))"
          />
        </div>
      </div>
      <div class="bio-footer-logo">
        <BioMediaField
          label="Imagem do rodape"
          kind="logo"
          accept="image/*"
          :model-value="branding.footerLogoSrc || ''"
          :uploading="isUploading('logo')"
          :on-upload="uploadMedia"
          @update:model-value="setBranding('footerLogoSrc', String($event ?? ''))"
        />
        <div class="bio-field bio-footer-logo__href">
          <label class="bio-field__label">Link do rodape (href)</label>
          <UInput
            :model-value="branding.footerLogoHref || ''"
            placeholder="https://..."
            @update:model-value="setBranding('footerLogoHref', String($event ?? ''))"
          />
        </div>
      </div>
    </div>
  </BioSectionCard>
</template>

<style scoped>
.bio-section-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  gap: 0.6rem 0.85rem;
}

.bio-media-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
  gap: 0.75rem;
}

.bio-field {
  display: grid;
  gap: 0.3rem;
}

.bio-field--switch {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.bio-field__label {
  font-size: 0.72rem;
  font-weight: 700;
  color: var(--text-muted);
}

.bio-subsection {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
  padding: 0.85rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-soft);
  background: rgb(var(--surface-2) / 0.35);
}

.bio-subsection__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.bio-subsection__title {
  font-size: 0.85rem;
  font-weight: 700;
  color: var(--text-main);
}

.bio-footer-logo {
  display: flex;
  align-items: flex-start;
  gap: 0.85rem;
  flex-wrap: wrap;
}

.bio-footer-logo__href {
  flex: 1;
  min-width: 180px;
}
</style>
