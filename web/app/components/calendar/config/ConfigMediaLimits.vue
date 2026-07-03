<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import { useCalendarMedia } from '~/composables/useCalendarMedia'
import { useAuthStore } from '~/stores/auth'
import { useUiStore } from '~/stores/ui'
import { formatBytes } from '~/utils/calendar'

// Secao Limites de midia (SPEC-F3): tetos de upload GLOBAIS da plataforma. GET
// para todo autenticado (o front mostra); edicao so platform_admin (regra
// isPlatformAdmin || has(...)) — o back tambem restringe o PUT.
const auth = useAuthStore()
const ui = useUiStore()
const { mediaLimits, fetchMediaLimits, saveMediaLimits } = useCalendarMedia()

// Media-limits e identity-global -> so platform_admin edita (nao ha permissao
// fina para isso; abrir a delegado so daria 403 no PUT).
const canEdit = computed(() => auth.role === 'platform_admin')

const MB = 1024 * 1024
const imageMb = ref(0)
const videoMb = ref(0)
const saving = ref(false)

watch(
  mediaLimits,
  (limits) => {
    imageMb.value = Math.round(limits.imageMaxBytes / MB)
    videoMb.value = Math.round(limits.videoMaxBytes / MB)
  },
  { immediate: true, deep: true },
)

onMounted(() => {
  void fetchMediaLimits()
})

const currentImage = computed(() => formatBytes(mediaLimits.value.imageMaxBytes))
const currentVideo = computed(() => formatBytes(mediaLimits.value.videoMaxBytes))

async function save(): Promise<void> {
  if (!canEdit.value) return
  saving.value = true
  const ok = await saveMediaLimits({
    imageMaxBytes: Math.max(1, Math.round(imageMb.value)) * MB,
    videoMaxBytes: Math.max(1, Math.round(videoMb.value)) * MB,
  })
  saving.value = false
  if (ok) ui.success('Limites de mídia atualizados.')
  else ui.error('Não foi possível salvar os limites.')
}
</script>

<template>
  <section class="calendar-config__section">
    <h3 class="calendar-config__section-title">Limites de mídia</h3>
    <p class="calendar-config__hint">
      Tetos de upload globais da plataforma. Atuais: imagem {{ currentImage }}, vídeo
      {{ currentVideo }}.
    </p>

    <p v-if="!canEdit" class="calendar-config__warn">
      <UIcon name="i-lucide-lock" aria-hidden="true" />
      Apenas administradores da plataforma podem alterar os limites.
    </p>

    <div class="calendar-config__grid2">
      <label class="calendar-config__field">
        <span class="calendar-config__field-label">Imagem (MB)</span>
        <input
          v-model.number="imageMb"
          class="calendar-config__input"
          type="number"
          min="1"
          :disabled="!canEdit"
        />
      </label>
      <label class="calendar-config__field">
        <span class="calendar-config__field-label">Vídeo (MB)</span>
        <input
          v-model.number="videoMb"
          class="calendar-config__input"
          type="number"
          min="1"
          :disabled="!canEdit"
        />
      </label>
    </div>

    <div v-if="canEdit" class="calendar-config__section-actions">
      <AppPanelButton variant="ghost" :disabled="saving" @click="save">
        Salvar limites de mídia
      </AppPanelButton>
    </div>
  </section>
</template>
