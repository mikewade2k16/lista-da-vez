<script setup lang="ts">
import ConfigAiRoleModelSelect from '~/components/omnichannel/config/ConfigAiRoleModelSelect.vue'
import { AI_PROVIDER_LABEL } from '~/utils/calendar'
import type {
  OmniAICredential,
  OmniMediaConfig,
  OmniMediaSectionConfig,
} from '~/domain/omnichannel/config-types'

type MediaRole = 'audio' | 'image' | 'video' | 'document'

const props = defineProps<{
  modelValue: OmniMediaConfig
  credentials: OmniAICredential[]
  disabled?: boolean
}>()
const emit = defineEmits<{ 'update:modelValue': [value: OmniMediaConfig] }>()

const roles: Array<{
  key: MediaRole
  title: string
  description: string
  capability: MediaRole
}> = [
  {
    key: 'audio',
    title: 'Áudio',
    description: 'Transcrição da mensagem de voz.',
    capability: 'audio',
  },
  {
    key: 'image',
    title: 'Imagem',
    description: 'Descrição visual e leitura de texto.',
    capability: 'image',
  },
  {
    key: 'video',
    title: 'Vídeo',
    description: 'Resumo de cenas, fala e texto visível.',
    capability: 'video',
  },
  {
    key: 'document',
    title: 'Documento',
    description: 'Extração e resumo de arquivos permitidos.',
    capability: 'document',
  },
]

function section(role: MediaRole): OmniMediaSectionConfig {
  return props.modelValue?.[role] || {}
}

function updateSection(role: MediaRole, patch: Partial<OmniMediaSectionConfig>): void {
  emit('update:modelValue', {
    ...props.modelValue,
    includeInReply: props.modelValue?.includeInReply ?? true,
    [role]: { ...section(role), ...patch },
  })
}

function selectCredential(role: MediaRole, credentialId: string): void {
  const credential = props.credentials.find((item) => item.id === credentialId)
  updateSection(role, {
    credentialId,
    provider: credential?.provider || '',
    model: '',
  })
}

function credentialLabel(item: OmniAICredential): string {
  return `${item.name} · ${AI_PROVIDER_LABEL[item.provider]} ····${item.last4}`
}

function credentialsFor(role: MediaRole): OmniAICredential[] {
  if (role === 'video' || role === 'document') {
    return props.credentials.filter((item) => item.provider === 'gemini')
  }
  return props.credentials.filter(
    (item) => item.provider === 'openai' || item.provider === 'gemini',
  )
}

function updateIncludeInReply(event: Event): void {
  emit('update:modelValue', {
    ...props.modelValue,
    includeInReply: (event.target as HTMLInputElement).checked,
  })
}
</script>

<template>
  <details class="calendar-config__collapse">
    <summary class="calendar-config__collapse-head">
      Modelos de áudio, imagem, vídeo e documentos
    </summary>
    <div class="calendar-config__collapse-body media-settings">
      <p class="calendar-config__hint media-settings__intro">
        Cada função pode usar uma chave e um modelo diferentes. O cérebro n8n interpreta a mídia
        primeiro e entrega o resultado ao modelo responsável pela resposta.
      </p>

      <section v-for="role in roles" :key="role.key" class="media-settings__role">
        <div class="media-settings__role-head">
          <div>
            <strong>{{ role.title }}</strong>
            <p class="calendar-config__hint">{{ role.description }}</p>
          </div>
          <label class="media-settings__toggle">
            <input
              type="checkbox"
              :checked="section(role.key).enabled ?? true"
              :disabled="disabled"
              @change="
                updateSection(role.key, { enabled: ($event.target as HTMLInputElement).checked })
              "
            />
            Ativo
          </label>
        </div>

        <div class="calendar-config__grid2">
          <label class="calendar-config__field">
            <span class="calendar-config__field-label">Chave de API</span>
            <select
              class="calendar-config__input"
              :value="section(role.key).credentialId || ''"
              :disabled="disabled || !(section(role.key).enabled ?? true)"
              @change="selectCredential(role.key, ($event.target as HTMLSelectElement).value)"
            >
              <option value="" disabled>Selecione uma credencial</option>
              <option
                v-for="credential in credentialsFor(role.key)"
                :key="credential.id"
                :value="credential.id"
              >
                {{ credentialLabel(credential) }}
              </option>
            </select>
          </label>

          <ConfigAiRoleModelSelect
            :model-value="section(role.key).model || ''"
            :credential-id="section(role.key).credentialId || ''"
            :capability="role.capability"
            :disabled="disabled || !(section(role.key).enabled ?? true)"
            @update:model-value="updateSection(role.key, { model: $event })"
          />
        </div>
      </section>

      <label class="media-settings__toggle media-settings__toggle--context">
        <input
          type="checkbox"
          :checked="modelValue?.includeInReply ?? true"
          :disabled="disabled"
          @change="updateIncludeInReply"
        />
        Entregar as interpretações ao modelo que responde
      </label>
    </div>
  </details>
</template>

<style scoped>
.media-settings,
.media-settings__role {
  display: grid;
  gap: 0.8rem;
}

.media-settings__intro,
.media-settings__role p {
  margin: 0;
}

.media-settings__role {
  padding: 0.8rem;
  border: 1px solid rgb(var(--border) / 0.7);
  border-radius: var(--radius-md);
}

.media-settings__role-head,
.media-settings__toggle {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.media-settings__role-head {
  justify-content: space-between;
}

.media-settings__toggle {
  font-size: 0.78rem;
}

.media-settings__toggle--context {
  font-weight: 600;
}
</style>
