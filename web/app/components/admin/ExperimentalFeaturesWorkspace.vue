<script setup lang="ts">
import { computed, onMounted } from 'vue'

import { useAuthStore } from '~/stores/auth'
import { usePlatformFeaturesStore } from '~/stores/platformFeatures'

const auth = useAuthStore()
const platformFeatures = usePlatformFeaturesStore()

const isPlatformAdmin = computed(() => auth.role === 'platform_admin')
const statusLabel = computed(() => {
  if (platformFeatures.loading) return 'Carregando'
  if (platformFeatures.errorMessage) return 'Erro'
  return platformFeatures.attendanceAudioRecordingEnabled ? 'Ativo' : 'Desativado'
})

async function setAttendanceAudioRecording(value: unknown) {
  await platformFeatures.save({
    ...platformFeatures.features,
    attendanceAudioRecording: value === true,
  })
}

onMounted(() => {
  if (!isPlatformAdmin.value) return
  void platformFeatures.load()
})
</script>

<template>
  <section class="experimental-features">
    <AdminPageHeader
      eyebrow="Plataforma"
      title="Recursos experimentais"
      description="Ative funcionalidades em validação antes de liberá-las como parte estável do produto."
    />

    <p v-if="!isPlatformAdmin" class="experimental-features__denied">
      Esta área é exclusiva para administradores da plataforma.
    </p>

    <template v-else>
      <UAlert
        v-if="platformFeatures.errorMessage"
        color="error"
        variant="soft"
        icon="i-lucide-alert-triangle"
        title="Não foi possível carregar a configuração"
        :description="platformFeatures.errorMessage"
      >
        <template #actions>
          <UButton
            color="error"
            variant="soft"
            label="Tentar novamente"
            :loading="platformFeatures.loading"
            @click="platformFeatures.load(true)"
          />
        </template>
      </UAlert>

      <details class="experimental-features__group">
        <summary class="experimental-features__summary">
          <span class="experimental-features__summary-copy">
            <strong>Atendimento presencial</strong>
            <span>Experimentos ligados ao fluxo operacional da fila</span>
          </span>
          <span
            class="experimental-features__status"
            :class="{
              'is-active': platformFeatures.attendanceAudioRecordingEnabled,
              'is-error': platformFeatures.errorMessage,
            }"
          >
            {{ statusLabel }}
          </span>
          <span class="material-icons-round experimental-features__chevron" aria-hidden="true">
            expand_more
          </span>
        </summary>

        <div class="experimental-features__body">
          <div class="experimental-features__feature">
            <div class="experimental-features__feature-copy">
              <strong>Gravação de áudio do atendimento</strong>
              <p>
                Libera a base para gravar atendimentos em blocos resilientes. Neste primeiro bloco,
                nenhuma captura de microfone é iniciada.
              </p>
              <span>Experimental · desligado por padrão</span>
            </div>

            <USwitch
              :model-value="platformFeatures.attendanceAudioRecordingEnabled"
              :loading="platformFeatures.saving"
              :disabled="
                !platformFeatures.loaded ||
                platformFeatures.loading ||
                platformFeatures.saving ||
                Boolean(platformFeatures.errorMessage)
              "
              aria-label="Ativar gravação experimental de áudio do atendimento"
              @update:model-value="setAttendanceAudioRecording"
            />
          </div>
        </div>
      </details>
    </template>
  </section>
</template>

<style scoped>
.experimental-features {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  display: grid;
  align-content: start;
  gap: 1rem;
  padding: 1rem 1.2rem 2rem;
}

.experimental-features__denied {
  margin: 0;
  color: rgb(var(--muted));
}

.experimental-features__group {
  border: 1px solid rgb(var(--border) / 0.9);
  border-radius: var(--radius-lg);
  background: rgb(var(--surface));
  box-shadow: var(--shadow-card);
  overflow: hidden;
}

.experimental-features__summary {
  display: flex;
  align-items: center;
  gap: 0.85rem;
  min-height: 4.5rem;
  padding: 1rem 1.1rem;
  cursor: pointer;
  list-style: none;
}

.experimental-features__summary::-webkit-details-marker {
  display: none;
}

.experimental-features__summary-copy,
.experimental-features__feature-copy {
  display: grid;
  gap: 0.25rem;
  min-width: 0;
}

.experimental-features__summary-copy {
  flex: 1;
}

.experimental-features__summary-copy strong,
.experimental-features__feature-copy strong {
  color: rgb(var(--text));
}

.experimental-features__summary-copy span,
.experimental-features__feature-copy p,
.experimental-features__feature-copy span {
  color: rgb(var(--muted));
  font-size: 0.84rem;
  line-height: 1.45;
}

.experimental-features__status {
  padding: 0.28rem 0.58rem;
  border-radius: 999px;
  background: rgb(var(--surface-2));
  color: rgb(var(--muted));
  font-size: 0.72rem;
  font-weight: 700;
}

.experimental-features__status.is-active {
  color: rgb(var(--success));
}

.experimental-features__status.is-error {
  color: rgb(var(--error));
}

.experimental-features__chevron {
  color: rgb(var(--muted));
  transition: transform 0.2s ease;
}

.experimental-features__group[open] .experimental-features__chevron {
  transform: rotate(180deg);
}

.experimental-features__body {
  padding: 0 1.1rem 1.1rem;
}

.experimental-features__feature {
  display: flex;
  align-items: center;
  gap: 1rem;
  padding: 1rem;
  border: 1px solid rgb(var(--border) / 0.75);
  border-radius: var(--radius-md);
  background: rgb(var(--surface-2));
}

.experimental-features__feature-copy {
  flex: 1;
}

.experimental-features__feature-copy p {
  margin: 0;
}

@media (max-width: 640px) {
  .experimental-features {
    padding-inline: 0.8rem;
  }

  .experimental-features__feature {
    align-items: flex-start;
  }
}
</style>
