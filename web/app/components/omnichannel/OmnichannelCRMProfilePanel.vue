<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { UBadge, UButton, UInput, USelect, UTextarea } from '#components'
import type {
  CRMContact,
  CRMContactPatch,
  CRMContactProfile,
} from '~/composables/omnichannel/useOmnichannelCRM'

const props = defineProps<{
  profile: CRMContactProfile | null
  loading: boolean
  saving?: boolean
  actionError?: string
  mergeCandidates?: CRMContact[]
}>()

const emit = defineEmits<{
  (event: 'close'): void
  (event: 'openConversation' | 'createNote', value: string): void
  (event: 'saveContact', patch: CRMContactPatch): void
  (event: 'mergeContact', payload: { targetId: string; reason: string }): void
}>()

const draft = reactive({ name: '', primaryEmail: '', relationshipStatus: '', tags: '' })
const baseline = ref('')
const noteDraft = ref('')
const mergeTargetId = ref('')
const mergeReason = ref('')

const statusOptions = [
  { label: 'Novo lead', value: 'new_lead' },
  { label: 'Lead conhecido', value: 'known_lead' },
  { label: 'Cliente', value: 'customer' },
  { label: 'Inativo', value: 'inactive' },
]

const mergeOptions = computed(() =>
  (props.mergeCandidates ?? [])
    .filter((entry) => entry.id !== props.profile?.contact.id && !entry.mergedIntoContactId)
    .map((entry) => ({
      label: `${entry.name || entry.phone || 'Contato'} · ${entry.phone || 'sem telefone'}`,
      value: entry.id,
    })),
)

const dirty = computed(() => JSON.stringify(draft) !== baseline.value)
const canSave = computed(() => dirty.value && draft.name.trim().length > 0 && !props.saving)
const canAddNote = computed(() => noteDraft.value.trim().length > 0 && !props.saving)
const canMerge = computed(
  () => Boolean(mergeTargetId.value && mergeReason.value.trim()) && !props.saving,
)

function statusLabel(value: string) {
  return statusOptions.find((option) => option.value === value)?.label ?? value
}

function syncDraft(contact: CRMContact | undefined) {
  draft.name = contact?.name ?? ''
  draft.primaryEmail = contact?.primaryEmail ?? ''
  draft.relationshipStatus = contact?.relationshipStatus ?? 'new_lead'
  draft.tags = contact?.tags?.join(', ') ?? ''
  baseline.value = JSON.stringify(draft)
  noteDraft.value = ''
  mergeTargetId.value = ''
  mergeReason.value = ''
}

watch(
  () => props.profile?.contact,
  (contact) => syncDraft(contact),
  { immediate: true, deep: true },
)

function save() {
  if (!canSave.value || !props.profile) return
  emit('saveContact', {
    name: draft.name.trim(),
    primaryEmail: draft.primaryEmail.trim() || null,
    relationshipStatus: draft.relationshipStatus,
    tags: draft.tags
      .split(',')
      .map((tag) => tag.trim().toLowerCase())
      .filter(Boolean),
    expectedUpdatedAt: props.profile.contact.updatedAt ?? null,
  })
}

function addNote() {
  const content = noteDraft.value.trim()
  if (!content || !canAddNote.value) return
  emit('createNote', content)
  noteDraft.value = ''
}

function merge() {
  if (!canMerge.value) return
  emit('mergeContact', { targetId: mergeTargetId.value, reason: mergeReason.value.trim() })
}
</script>

<template>
  <aside class="crm-profile-panel" aria-label="Perfil 360 do contato">
    <header class="crm-profile-panel__header">
      <div>
        <p class="crm-profile-panel__eyebrow">CRM · Perfil 360°</p>
        <h2>{{ profile?.contact.name || profile?.contact.phone || 'Contato' }}</h2>
      </div>
      <UButton
        icon="i-lucide-x"
        color="neutral"
        variant="ghost"
        aria-label="Fechar perfil"
        @click="emit('close')"
      />
    </header>

    <div v-if="loading" class="crm-profile-panel__empty">Carregando histórico...</div>
    <template v-else-if="profile">
      <UAlert
        v-if="actionError"
        color="error"
        variant="soft"
        :title="actionError"
        class="crm-profile-panel__alert"
      />

      <section class="crm-profile-panel__section">
        <div class="crm-profile-panel__section-heading">
          <h3>Dados do contato</h3>
          <span v-if="dirty" class="crm-profile-panel__dirty">Alterações não salvas</span>
        </div>
        <UFormField label="Nome">
          <UInput v-model="draft.name" autocomplete="name" />
        </UFormField>
        <UFormField label="E-mail">
          <UInput
            v-model="draft.primaryEmail"
            type="email"
            autocomplete="email"
            placeholder="cliente@exemplo.com"
          />
        </UFormField>
        <UFormField label="Classificação">
          <USelect v-model="draft.relationshipStatus" :items="statusOptions" />
        </UFormField>
        <UFormField label="Tags (separadas por vírgula)">
          <UInput v-model="draft.tags" placeholder="vip, orçamento" />
        </UFormField>
        <div class="crm-profile-panel__actions">
          <UButton size="sm" color="primary" :disabled="!canSave" :loading="saving" @click="save">
            Salvar alterações
          </UButton>
          <span v-if="profile.contact.classificationSource" class="crm-profile-panel__muted">
            Fonte: {{ profile.contact.classificationSource }}
          </span>
        </div>
      </section>

      <div class="crm-profile-panel__status">
        <UBadge color="primary" variant="soft">
          {{ statusLabel(profile.contact.relationshipStatus) }}
        </UBadge>
        <span>
          {{
            profile.contact.lastChannel || profile.contact.firstChannel || 'Origem não informada'
          }}
        </span>
      </div>
      <dl class="crm-profile-panel__facts">
        <div>
          <dt>Telefone</dt>
          <dd>{{ profile.contact.phone || '—' }}</dd>
        </div>
        <div>
          <dt>Origem</dt>
          <dd>{{ profile.contact.source || '—' }}</dd>
        </div>
        <div>
          <dt>Responsável</dt>
          <dd>{{ profile.contact.ownerUserId || 'Não atribuído' }}</dd>
        </div>
      </dl>

      <section class="crm-profile-panel__section">
        <h3>Identidades</h3>
        <p v-if="!profile.identities.length" class="crm-profile-panel__muted">
          Nenhuma identidade.
        </p>
        <ul v-else>
          <li v-for="identity in profile.identities" :key="identity.id">
            {{ identity.channel }} · {{ identity.provider }}
            <small>{{ identity.externalId }}</small>
          </li>
        </ul>
      </section>

      <section class="crm-profile-panel__section">
        <h3>Touchpoints</h3>
        <p class="crm-profile-panel__muted">{{ profile.touchpoints.length }} eventos registrados</p>
      </section>

      <section class="crm-profile-panel__section">
        <h3>Notas internas</h3>
        <ul v-if="profile.notes.length">
          <li v-for="note in profile.notes.slice(0, 6)" :key="note.id">{{ note.content }}</li>
        </ul>
        <p v-else class="crm-profile-panel__muted">Nenhuma nota ainda.</p>
        <UTextarea v-model="noteDraft" :rows="3" placeholder="Adicionar uma nota interna" />
        <UButton
          class="crm-profile-panel__note-button"
          size="sm"
          color="neutral"
          variant="outline"
          :disabled="!canAddNote"
          :loading="saving"
          @click="addNote"
        >
          Adicionar nota
        </UButton>
      </section>

      <section v-if="mergeOptions.length" class="crm-profile-panel__section">
        <h3>Mesclar contato</h3>
        <p class="crm-profile-panel__muted">
          A operação move vínculos para o contato escolhido e fica auditada.
        </p>
        <USelect
          v-model="mergeTargetId"
          :items="mergeOptions"
          placeholder="Escolher contato destino"
        />
        <UTextarea v-model="mergeReason" :rows="2" placeholder="Motivo obrigatório da mesclagem" />
        <UButton
          class="crm-profile-panel__note-button"
          size="sm"
          color="warning"
          variant="soft"
          :disabled="!canMerge"
          :loading="saving"
          @click="merge"
        >
          Mesclar com o destino
        </UButton>
      </section>

      <UButton
        color="primary"
        variant="soft"
        block
        @click="emit('openConversation', profile.contact.id)"
      >
        Abrir conversa
      </UButton>
    </template>
    <div v-else class="crm-profile-panel__empty">Não foi possível carregar o perfil.</div>
  </aside>
</template>

<style scoped>
.crm-profile-panel {
  position: absolute;
  z-index: 30;
  top: 0;
  right: 0;
  width: min(24rem, 100%);
  height: 100%;
  padding: 1rem;
  overflow-y: auto;
  background: rgb(12 18 35 / 98%);
  border-left: 1px solid rgb(148 163 184 / 20%);
  box-shadow: -1rem 0 3rem rgb(0 0 0 / 20%);
}
.crm-profile-panel__header,
.crm-profile-panel__status,
.crm-profile-panel__section-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}
.crm-profile-panel__eyebrow,
.crm-profile-panel__muted {
  color: rgb(148 163 184);
  font-size: 0.75rem;
}
.crm-profile-panel h2 {
  margin: 0.2rem 0 0;
  font-size: 1.1rem;
}
.crm-profile-panel h3 {
  margin: 1rem 0 0.35rem;
  font-size: 0.8rem;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: rgb(148 163 184);
}
.crm-profile-panel__section {
  display: grid;
  gap: 0.55rem;
  margin: 1rem 0;
}
.crm-profile-panel__facts {
  display: grid;
  gap: 0.55rem;
  margin: 1rem 0;
}
.crm-profile-panel__facts div {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
}
.crm-profile-panel dt {
  color: rgb(148 163 184);
  font-size: 0.75rem;
}
.crm-profile-panel dd {
  margin: 0;
  max-width: 13rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.8rem;
}
.crm-profile-panel ul {
  display: grid;
  gap: 0.4rem;
  padding: 0;
  margin: 0;
  list-style: none;
  font-size: 0.8rem;
}
.crm-profile-panel li {
  padding: 0.5rem;
  border-radius: 0.5rem;
  background: rgb(30 41 59 / 70%);
}
.crm-profile-panel li small {
  display: block;
  margin-top: 0.2rem;
  color: rgb(148 163 184);
  overflow: hidden;
  text-overflow: ellipsis;
}
.crm-profile-panel__actions {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  flex-wrap: wrap;
}
.crm-profile-panel__dirty {
  color: rgb(251 191 36);
  font-size: 0.7rem;
}
.crm-profile-panel__alert {
  margin: 0.75rem 0;
}
.crm-profile-panel__note-button {
  justify-self: start;
}
.crm-profile-panel__empty {
  display: grid;
  place-items: center;
  min-height: 12rem;
  color: rgb(148 163 184);
}
</style>
