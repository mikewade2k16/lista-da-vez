<script setup lang="ts">
import { computed, reactive, watch } from 'vue'

import AppPanelButton from '~/components/ui/AppPanelButton.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import OmniEntityDrawer from '~/components/ui/OmniEntityDrawer.vue'
import type { QueueCommunication, QueueCommunicationInput } from '~/domain/operation/communications'

interface AccountOption {
  value: string
  label: string
}

interface StoreOption {
  id: string
  name: string
  accountId: string
}

const props = defineProps<{
  open: boolean
  item: QueueCommunication | null
  defaultAccountId: string
  accountOptions: AccountOption[]
  stores: StoreOption[]
  saving: boolean
  errorMessage: string
}>()

const emit = defineEmits<{
  (event: 'update:open', value: boolean): void
  (event: 'save', payload: { accountId: string; input: QueueCommunicationInput }): void
}>()

const draft = reactive({
  accountId: '',
  title: '',
  excerpt: '',
  body: '',
  startsAt: '',
  endsAt: '',
  isPublished: true,
  displayOrder: 0,
  targetsAllStores: true,
  storeIds: [] as string[],
})

function toLocalDateTime(value: string | null | undefined): string {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60_000)
  return local.toISOString().slice(0, 16)
}

function toISO(value: string): string | null {
  if (!value) return null
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? null : date.toISOString()
}

function resetDraft(): void {
  const item = props.item
  draft.accountId =
    item?.accountId || props.defaultAccountId || props.accountOptions[0]?.value || ''
  draft.title = item?.title || ''
  draft.excerpt = item?.excerpt || ''
  draft.body = item?.body || ''
  draft.startsAt = toLocalDateTime(item?.startsAt)
  draft.endsAt = toLocalDateTime(item?.endsAt)
  draft.isPublished = item?.isPublished ?? true
  draft.displayOrder = Number(item?.displayOrder || 0)
  draft.targetsAllStores = item?.targetsAllStores ?? true
  draft.storeIds = [...(item?.storeIds || [])]
}

const availableStores = computed(() =>
  props.stores.filter((store) => store.accountId === draft.accountId),
)
const canSave = computed(
  () =>
    !props.saving &&
    Boolean(draft.accountId && draft.title.trim() && draft.body.trim()) &&
    (draft.targetsAllStores || draft.storeIds.length > 0) &&
    (!draft.startsAt ||
      !draft.endsAt ||
      new Date(draft.endsAt).getTime() > new Date(draft.startsAt).getTime()),
)

function toggleStore(storeId: string): void {
  draft.storeIds = draft.storeIds.includes(storeId)
    ? draft.storeIds.filter((candidate) => candidate !== storeId)
    : [...draft.storeIds, storeId]
}

function submit(): void {
  if (!canSave.value) return
  emit('save', {
    accountId: draft.accountId,
    input: {
      title: draft.title.trim(),
      excerpt: draft.excerpt.trim(),
      body: draft.body.trim(),
      startsAt: toISO(draft.startsAt),
      endsAt: toISO(draft.endsAt),
      isPublished: draft.isPublished,
      displayOrder: Number(draft.displayOrder || 0),
      targetsAllStores: draft.targetsAllStores,
      storeIds: draft.targetsAllStores ? [] : [...draft.storeIds],
    },
  })
}

watch(
  () => [props.open, props.item?.id, props.defaultAccountId],
  ([open]) => {
    if (open) resetDraft()
  },
  { immediate: true },
)

watch(
  () => draft.accountId,
  () => {
    if (!props.item) draft.storeIds = []
  },
)
</script>

<template>
  <OmniEntityDrawer
    :model-value="open"
    :title="item ? 'Editar comunicado' : 'Novo comunicado'"
    subtitle="Defina o conteúdo, a vigência e em quais lojas ele será exibido."
    @update:model-value="emit('update:open', $event)"
  >
    <form class="communication-editor" @submit.prevent="submit">
      <AppSelectField
        v-if="accountOptions.length > 1"
        v-model="draft.accountId"
        label="Conta"
        :options="accountOptions"
        :disabled="Boolean(item)"
      />

      <label class="communication-editor__field">
        <span>Título</span>
        <input
          v-model="draft.title"
          type="text"
          maxlength="160"
          placeholder="Ex.: Campanha Progressiva"
        />
      </label>

      <label class="communication-editor__field">
        <span>Resumo curto</span>
        <input
          v-model="draft.excerpt"
          type="text"
          maxlength="300"
          placeholder="Texto compacto exibido no card"
        />
      </label>

      <label class="communication-editor__field">
        <span>Comunicado completo</span>
        <textarea
          v-model="draft.body"
          rows="10"
          maxlength="20000"
          placeholder="Escreva aqui todo o conteúdo que será aberto no modal."
        ></textarea>
      </label>

      <div class="communication-editor__grid">
        <label class="communication-editor__field">
          <span>Início da exibição</span>
          <input v-model="draft.startsAt" type="datetime-local" />
        </label>
        <label class="communication-editor__field">
          <span>Fim da exibição</span>
          <input v-model="draft.endsAt" type="datetime-local" />
        </label>
      </div>

      <div class="communication-editor__grid">
        <label class="communication-editor__toggle">
          <input v-model="draft.isPublished" type="checkbox" />
          <span>Publicado</span>
        </label>
        <label class="communication-editor__field">
          <span>Ordem de exibição</span>
          <input v-model.number="draft.displayOrder" type="number" min="-10000" max="10000" />
        </label>
      </div>

      <section class="communication-editor__targets">
        <label class="communication-editor__toggle">
          <input v-model="draft.targetsAllStores" type="checkbox" />
          <span>Todas as lojas desta conta</span>
        </label>

        <div v-if="!draft.targetsAllStores" class="communication-editor__store-grid">
          <label
            v-for="store in availableStores"
            :key="store.id"
            class="communication-editor__store"
          >
            <input
              type="checkbox"
              :checked="draft.storeIds.includes(store.id)"
              @change="toggleStore(store.id)"
            />
            <span>{{ store.name }}</span>
          </label>
        </div>
      </section>

      <p v-if="errorMessage" class="communication-editor__error" role="alert">
        {{ errorMessage }}
      </p>
    </form>

    <template #footer>
      <div class="communication-editor__footer">
        <AppPanelButton variant="ghost" @click="emit('update:open', false)">
          Cancelar
        </AppPanelButton>
        <AppPanelButton :disabled="!canSave" @click="submit">
          {{ saving ? 'Salvando…' : item ? 'Salvar alterações' : 'Criar comunicado' }}
        </AppPanelButton>
      </div>
    </template>
  </OmniEntityDrawer>
</template>

<style scoped>
.communication-editor {
  display: grid;
  gap: 0.9rem;
}

.communication-editor__field {
  display: grid;
  gap: 0.38rem;
  color: rgb(var(--muted));
  font-size: 0.76rem;
  font-weight: 700;
}

.communication-editor__field input,
.communication-editor__field textarea {
  width: 100%;
  border: 1px solid var(--line-soft);
  border-radius: 11px;
  background: rgb(var(--surface-2) / 0.55);
  color: var(--text-main);
  padding: 0.7rem 0.78rem;
  font: inherit;
  font-weight: 500;
  outline: none;
}

.communication-editor__field textarea {
  resize: vertical;
  line-height: 1.55;
}

.communication-editor__field input:focus,
.communication-editor__field textarea:focus {
  border-color: rgb(var(--primary) / 0.65);
  box-shadow: 0 0 0 3px rgb(var(--primary) / 0.11);
}

.communication-editor__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.75rem;
}

.communication-editor__toggle,
.communication-editor__store {
  display: flex;
  align-items: center;
  gap: 0.55rem;
  color: var(--text-main);
  font-size: 0.78rem;
  font-weight: 700;
  cursor: pointer;
}

.communication-editor__targets {
  display: grid;
  gap: 0.7rem;
  padding: 0.85rem;
  border: 1px solid var(--line-soft);
  border-radius: 12px;
  background: rgb(var(--surface-2) / 0.28);
}

.communication-editor__store-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.45rem;
}

.communication-editor__store {
  min-height: 2.35rem;
  padding: 0.45rem 0.6rem;
  border: 1px solid var(--line-soft);
  border-radius: 9px;
  background: rgb(var(--surface-2) / 0.45);
}

.communication-editor__error {
  margin: 0;
  color: rgb(var(--danger));
  font-size: 0.76rem;
}

.communication-editor__footer {
  display: flex;
  justify-content: flex-end;
  gap: 0.5rem;
  width: 100%;
}

@media (max-width: 640px) {
  .communication-editor__grid,
  .communication-editor__store-grid {
    grid-template-columns: 1fr;
  }
}
</style>
