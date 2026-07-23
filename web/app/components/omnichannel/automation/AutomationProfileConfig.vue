<script setup lang="ts">
import { reactive, watch } from 'vue'
import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import type { AutomationProfile, AutomationProfileInput } from '~/domain/omnichannel/automation-api'
import type { OmniAgent, OmniInstance } from '~/domain/omnichannel/config-types'

const props = defineProps<{
  profile: AutomationProfile | null
  instances: OmniInstance[]
  agents: OmniAgent[]
  loading?: boolean
  saving?: boolean
}>()

const emit = defineEmits<{ save: [input: AutomationProfileInput] }>()

const draft = reactive({
  whatsappInstanceId: '',
  aiAgentId: '',
  enabled: false,
  autoCloseEnabled: false,
  minimumConfidence: 0.9,
  requireAllRequiredFields: true,
  blockOnHumanRequest: true,
  blockSensitiveTopics: true,
})

watch(
  () => props.profile,
  (profile) => {
    draft.whatsappInstanceId = profile?.whatsappInstance?.id || ''
    draft.aiAgentId = profile?.aiAgent?.id || ''
    draft.enabled = Boolean(profile?.enabled)
    draft.autoCloseEnabled = Boolean(profile?.closePolicy.autoCloseEnabled)
    draft.minimumConfidence = profile?.closePolicy.minimumConfidence ?? 0.9
    draft.requireAllRequiredFields = profile?.closePolicy.requireAllRequiredFields ?? true
    draft.blockOnHumanRequest = profile?.closePolicy.blockOnHumanRequest ?? true
    draft.blockSensitiveTopics = profile?.closePolicy.blockSensitiveTopics ?? true
  },
  { immediate: true },
)

function submit(): void {
  if (!draft.whatsappInstanceId || !draft.aiAgentId) return
  emit('save', {
    whatsappInstanceId: draft.whatsappInstanceId,
    aiAgentId: draft.aiAgentId,
    enabled: draft.enabled,
    closePolicy: {
      autoCloseEnabled: draft.autoCloseEnabled,
      minimumConfidence: Number(draft.minimumConfidence),
      requireAllRequiredFields: draft.requireAllRequiredFields,
      blockOnHumanRequest: draft.blockOnHumanRequest,
      blockSensitiveTopics: draft.blockSensitiveTopics,
    },
  })
}
</script>

<template>
  <section class="profile-config">
    <p v-if="loading" class="profile-config__empty">Carregando configuração…</p>
    <template v-else-if="profile">
      <div class="profile-config__summary">
        <span>Configuração de {{ profile.client.name }}</span>
        <strong :class="{ 'is-ready': profile.ready }">
          {{ profile.ready ? 'pronta para testar' : 'requer atenção' }}
        </strong>
      </div>

      <div class="profile-config__grid">
        <label class="profile-field">
          <span>Número do WhatsApp</span>
          <select v-model="draft.whatsappInstanceId">
            <option value="">Selecione um número</option>
            <option v-for="instance in instances" :key="instance.id" :value="instance.id">
              {{ instance.displayName || instance.instanceName }}
              {{ instance.phoneNumber ? `— ${instance.phoneNumber}` : '' }}
            </option>
          </select>
          <small v-if="!instances.length">Conecte um número na tab WhatsApp.</small>
        </label>

        <label class="profile-field">
          <span>Agente de IA</span>
          <select v-model="draft.aiAgentId">
            <option value="">Selecione um agente</option>
            <option v-for="agent in agents" :key="agent.id" :value="agent.id">
              {{ agent.name }}{{ agent.activeVersionId ? '' : ' — sem versão publicada' }}
            </option>
          </select>
          <small v-if="!agents.length">Configure o agente na tab IA.</small>
        </label>
      </div>

      <label
        class="profile-toggle profile-toggle--main"
        :class="draft.enabled ? 'profile-toggle--enabled' : 'profile-toggle--disabled'"
      >
        <input v-model="draft.enabled" type="checkbox" />
        <span>
          <strong>
            {{ draft.enabled ? 'Automação de IA ligada' : 'Automação de IA desligada' }}
          </strong>
          <small v-if="draft.enabled">
            A IA pode responder somente conversas individuais deste número.
          </small>
          <small v-else>
            Nenhuma resposta de IA será enviada. Trabalhos pendentes também serão cancelados ao
            salvar.
          </small>
        </span>
      </label>

      <details class="profile-config__collapse">
        <summary>
          <span>Regras de encerramento</span>
          <small>{{ draft.autoCloseEnabled ? 'automático' : 'somente manual' }}</small>
        </summary>
        <div class="profile-config__collapse-body">
          <label class="profile-toggle">
            <input v-model="draft.autoCloseEnabled" type="checkbox" />
            <span>Permitir encerramento automático pela IA</span>
          </label>

          <template v-if="draft.autoCloseEnabled">
            <label class="profile-field profile-field--confidence">
              <span>
                Confiança mínima para encerrar: {{ Math.round(draft.minimumConfidence * 100) }}%
              </span>
              <input
                v-model.number="draft.minimumConfidence"
                type="range"
                min="0"
                max="1"
                step="0.01"
              />
              <small>
                Esta regra vale somente para o encerramento automático. A confiança para responder
                fica na tab IA, em “Resposta automática e limites”.
              </small>
            </label>
            <label class="profile-toggle">
              <input v-model="draft.requireAllRequiredFields" type="checkbox" />
              <span>Exigir todos os campos obrigatórios</span>
            </label>
            <label class="profile-toggle">
              <input v-model="draft.blockOnHumanRequest" type="checkbox" />
              <span>Não encerrar quando o cliente pedir atendente</span>
            </label>
            <label class="profile-toggle">
              <input v-model="draft.blockSensitiveTopics" type="checkbox" />
              <span>Não encerrar assuntos sensíveis</span>
            </label>
          </template>

          <div class="profile-config__lease">
            <UIcon name="i-lucide-shield-check" />
            <span>A validação da geração permanece obrigatória.</span>
          </div>
        </div>
      </details>

      <details class="profile-config__collapse">
        <summary>
          <span>Contexto do cliente</span>
          <small>{{ profile.strategicContext?.filled ? 'sincronizado' : 'não preenchido' }}</small>
        </summary>
        <div class="profile-config__context">
          <template v-if="profile.strategicContext?.filled">
            <div>
              <span>Segmento</span>
              <strong>{{ profile.strategicContext.profile.segment || '—' }}</strong>
            </div>
            <div>
              <span>Posicionamento</span>
              <strong>{{ profile.strategicContext.profile.positioning || '—' }}</strong>
            </div>
            <div>
              <span>Objetivos</span>
              <strong>{{ profile.strategicContext.profile.objectives || '—' }}</strong>
            </div>
            <div>
              <span>Tom de voz</span>
              <strong>{{ profile.strategicContext.profile.brandVoice || '—' }}</strong>
            </div>
          </template>
          <p v-else>Preencha o perfil estratégico no Calendário para enriquecer a IA.</p>
        </div>
      </details>

      <ul v-if="profile.readinessIssues.length" class="profile-config__issues">
        <li v-for="issue in profile.readinessIssues" :key="issue">{{ issue }}</li>
      </ul>

      <footer class="profile-config__footer">
        <AppPanelButton
          variant="primary"
          :disabled="saving || !draft.whatsappInstanceId || !draft.aiAgentId"
          @click="submit"
        >
          {{ saving ? 'Salvando…' : 'Salvar atendimento' }}
        </AppPanelButton>
      </footer>
    </template>
    <p v-else class="profile-config__empty">Selecione um cliente.</p>
  </section>
</template>

<style scoped>
.profile-config {
  display: grid;
  gap: 0.8rem;
}

.profile-config__summary {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  color: var(--text-muted);
  font-size: 0.78rem;
}

.profile-config__summary strong {
  padding: 0.18rem 0.5rem;
  border-radius: 999px;
  background: rgb(var(--surface-2));
  font-size: 0.7rem;
}

.profile-config__summary strong.is-ready {
  background: rgb(var(--success) / 0.15);
  color: rgb(var(--success));
}

.profile-config__grid,
.profile-config__context {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.7rem;
}

.profile-field {
  display: grid;
  gap: 0.3rem;
}

.profile-field > span,
.profile-config__context span {
  color: var(--text-muted);
  font-size: 0.72rem;
  font-weight: 700;
}

.profile-field select {
  min-height: 38px;
  padding: 0 0.65rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: rgb(var(--surface));
  color: var(--text-main);
}

.profile-field small,
.profile-toggle small {
  color: var(--text-muted);
  font-size: 0.7rem;
}

.profile-toggle {
  display: flex;
  align-items: flex-start;
  gap: 0.55rem;
  color: var(--text-main);
  font-size: 0.8rem;
}

.profile-toggle--main {
  padding: 0.8rem;
  border: 1px solid rgb(var(--primary) / 0.32);
  border-radius: var(--radius-card);
  background: rgb(var(--primary) / 0.08);
}

.profile-toggle--enabled {
  border-color: rgb(var(--success) / 0.4);
  background: linear-gradient(135deg, rgb(var(--success) / 0.12), rgb(var(--surface) / 0.56));
}

.profile-toggle--disabled {
  border-color: rgb(var(--error) / 0.42);
  background: linear-gradient(135deg, rgb(var(--error) / 0.12), rgb(var(--surface) / 0.56));
}

.profile-toggle span {
  display: grid;
}

.profile-config__collapse {
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
}

.profile-config__collapse summary {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0.7rem 0.75rem;
  color: var(--text-main);
  font-size: 0.8rem;
  font-weight: 700;
  cursor: pointer;
}

.profile-config__collapse summary small {
  color: var(--text-muted);
  font-size: 0.7rem;
  font-weight: 500;
}

.profile-config__collapse-body,
.profile-config__context {
  gap: 0.7rem;
  padding: 0 0.75rem 0.75rem;
}

.profile-field--confidence {
  padding: 0.4rem 0;
}

.profile-config__lease {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  color: rgb(var(--success));
  font-size: 0.75rem;
}

.profile-config__context > div {
  display: grid;
  gap: 0.2rem;
  padding: 0.6rem;
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2) / 0.55);
}

.profile-config__context strong {
  color: var(--text-main);
  font-size: 0.78rem;
  font-weight: 500;
}

.profile-config__context p {
  grid-column: 1 / -1;
  margin: 0;
  color: var(--text-muted);
  font-size: 0.78rem;
}

.profile-config__issues {
  margin: 0;
  padding: 0.65rem 0.8rem 0.65rem 1.8rem;
  border-radius: var(--radius-sm);
  background: color-mix(in srgb, var(--accent-warning) 12%, transparent);
  color: var(--text-main);
  font-size: 0.76rem;
}

.profile-config__footer {
  display: flex;
  justify-content: flex-end;
}

@media (max-width: 720px) {
  .profile-config__grid,
  .profile-config__context {
    grid-template-columns: 1fr;
  }

  .profile-config__summary {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
