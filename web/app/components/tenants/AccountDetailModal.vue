<script setup lang="ts">
import { reactive, watch } from 'vue'
import type { AccountFieldKey, AccountItem } from '~/types/accounts'
import {
  ACCOUNT_FIELD_GROUPS,
  accountFieldsByGroup,
  type AccountFieldDef,
  type AccountFieldEdit,
} from './account-fields'

const props = defineProps<{ account: AccountItem | null; open: boolean; canEdit?: boolean }>()
const emit = defineEmits<{
  (e: 'update:open', value: boolean): void
  (
    e: 'update-field',
    payload: { field: AccountFieldKey; value: unknown; immediate?: boolean },
  ): void
}>()

// `form` carrega os valores editaveis. Sincroniza ao trocar de account (por id),
// nao a cada patch — evita sobrescrever o que o usuario esta digitando.
const form = reactive<Record<string, unknown>>({})

watch(
  () => props.account?.id,
  () => {
    const account = props.account
    if (!account) return
    for (const field of accountFieldKeysWithEdit()) {
      form[field] = (account as unknown as Record<string, unknown>)[field]
    }
  },
  { immediate: true },
)

function accountFieldKeysWithEdit(): AccountFieldKey[] {
  return ACCOUNT_FIELD_GROUPS.flatMap((group) =>
    accountFieldsByGroup(group.id)
      .filter((f) => f.edit)
      .map((f) => f.edit!.field),
  )
}

function emitField(edit: AccountFieldEdit | undefined, value: unknown) {
  if (!edit || !props.canEdit) return
  form[edit.field] = value
  emit('update-field', { field: edit.field, value, immediate: edit.immediate })
}

function onText(edit: AccountFieldEdit | undefined, value: unknown) {
  emitField(edit, String(value ?? ''))
}

function onNumber(edit: AccountFieldEdit | undefined, value: unknown) {
  if (!edit) return
  const raw = String(value ?? '').trim()
  if (raw === '') {
    emitField(edit, edit.field === 'paymentDueDay' ? null : 0)
    return
  }
  emitField(edit, Number(raw) || 0)
}

function switchOn(edit: AccountFieldEdit | undefined): boolean {
  if (!edit) return false
  return edit.field === 'status' ? form[edit.field] === 'active' : Boolean(form[edit.field])
}

function onSwitch(edit: AccountFieldEdit | undefined, checked: boolean) {
  if (!edit) return
  emitField(edit, edit.field === 'status' ? (checked ? 'active' : 'inactive') : checked)
}

function fieldValue(field: AccountFieldDef): string {
  return props.account ? field.display(props.account) : '-'
}

function editValue(field: AccountFieldDef): string {
  const key = field.edit?.field
  return key ? String(form[key] ?? '') : ''
}
</script>

<template>
  <UModal :open="open" @update:open="emit('update:open', $event)">
    <template #content>
      <UCard v-if="account" class="account-detail">
        <template #header>
          <div class="account-detail__header">
            <h3 class="account-detail__title">{{ account.name || 'Sem nome' }}</h3>
            <UBadge
              :color="account.status === 'active' ? 'success' : 'neutral'"
              variant="soft"
              size="sm"
            >
              {{ account.status === 'active' ? 'Ativo' : 'Inativo' }}
            </UBadge>
          </div>
        </template>

        <div class="account-detail__groups">
          <section
            v-for="group in ACCOUNT_FIELD_GROUPS"
            :key="group.id"
            class="account-detail__group"
          >
            <h4 class="account-detail__group-title">{{ group.label }}</h4>
            <div class="account-detail__fields">
              <div
                v-for="field in accountFieldsByGroup(group.id)"
                :key="field.key"
                class="account-detail__field"
              >
                <label class="account-detail__field-label">{{ field.label }}</label>

                <template v-if="field.edit && canEdit">
                  <UInput
                    v-if="field.edit?.type === 'text'"
                    :model-value="editValue(field)"
                    @update:model-value="onText(field.edit, $event)"
                  />
                  <UInput
                    v-else-if="field.edit?.type === 'number' || field.edit?.type === 'money'"
                    type="number"
                    :model-value="editValue(field)"
                    @update:model-value="onNumber(field.edit, $event)"
                  />
                  <label v-else-if="field.edit?.type === 'switch'" class="account-detail__switch">
                    <input
                      type="checkbox"
                      :checked="switchOn(field.edit)"
                      @change="onSwitch(field.edit, ($event.target as HTMLInputElement).checked)"
                    />
                    <span>{{ switchOn(field.edit) ? 'Sim' : 'Nao' }}</span>
                  </label>
                  <select
                    v-else-if="field.edit?.type === 'select'"
                    class="account-detail__select"
                    :value="editValue(field)"
                    @change="emitField(field.edit, ($event.target as HTMLSelectElement).value)"
                  >
                    <option
                      v-for="opt in field.edit?.options ?? []"
                      :key="opt.value"
                      :value="opt.value"
                    >
                      {{ opt.label }}
                    </option>
                  </select>
                </template>

                <p v-else class="account-detail__field-value">{{ fieldValue(field) }}</p>
              </div>
            </div>
          </section>
        </div>

        <template #footer>
          <div class="account-detail__footer">
            <span class="account-detail__id">ID: {{ account.id }}</span>
            <UButton
              label="Fechar"
              color="neutral"
              variant="ghost"
              @click="emit('update:open', false)"
            />
          </div>
        </template>
      </UCard>
    </template>
  </UModal>
</template>

<style scoped>
.account-detail__header {
  display: flex;
  align-items: center;
  gap: 0.6rem;
}

.account-detail__title {
  margin: 0;
  font-size: 1rem;
  font-weight: 700;
  color: var(--text-main);
}

.account-detail__groups {
  display: grid;
  gap: 1.1rem;
  max-height: 60vh;
  overflow-y: auto;
}

.account-detail__group {
  display: grid;
  gap: 0.6rem;
}

.account-detail__group-title {
  margin: 0;
  font-size: 0.78rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-muted);
}

.account-detail__fields {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr));
  gap: 0.7rem 0.9rem;
}

.account-detail__field {
  display: grid;
  gap: 0.3rem;
  min-width: 0;
}

.account-detail__field-label {
  color: var(--text-muted);
  font-size: 0.72rem;
  font-weight: 700;
}

.account-detail__field-value {
  margin: 0;
  color: var(--text-main);
  font-size: 0.85rem;
  word-break: break-word;
}

.account-detail__switch {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  color: var(--text-main);
  font-size: 0.85rem;
}

.account-detail__select {
  min-height: 2.3rem;
  padding: 0 0.6rem;
  border-radius: 0.6rem;
  border: 1px solid rgb(var(--border) / 0.4);
  background: rgb(var(--surface) / 0.7);
  color: var(--text-main);
}

.account-detail__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.account-detail__id {
  color: var(--text-muted);
  font-size: 0.72rem;
}
</style>
