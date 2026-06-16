<script setup lang="ts">
import { computed, reactive, watch } from 'vue'

interface TenantOption {
  id: string
  name: string
}

const props = defineProps<{
  open: boolean
  creating?: boolean
  isAdmin?: boolean
  tenants?: TenantOption[]
}>()

const emit = defineEmits<{
  (e: 'update:open', value: boolean): void
  // edit=true => criar e abrir o editor; edit=false => criar e voltar a lista.
  (e: 'submit', payload: { name: string; slug: string; accountId: string; edit: boolean }): void
}>()

const form = reactive({ name: '', slug: '', accountId: '' })

watch(
  () => props.open,
  (open) => {
    if (!open) return
    form.name = ''
    form.slug = ''
    // Default = sem cliente (agencia); o admin troca no select se quiser.
    form.accountId = props.isAdmin ? AGENCY_SENTINEL : ''
  },
)

// Slug global, regex ^[a-z0-9-]+$ (mesma normalizacao do backend).
function slugify(value: string): string {
  return value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
}

const slugPreview = computed(() => (form.slug.trim() ? slugify(form.slug) : slugify(form.name)))

// Cliente OPCIONAL: o sentinela '' = "Sem cliente (agencia)" (a bio fica na
// account ativa do admin). USelect (Reka UI) proibe value vazio, entao usamos
// o sentinela 'agency' e convertemos para '' no submit.
const AGENCY_SENTINEL = 'agency'

const tenantItems = computed(() => [
  { label: 'Sem cliente (agencia)', value: AGENCY_SENTINEL },
  ...(props.tenants || []).map((tenant) => ({ label: tenant.name, value: tenant.id })),
])

// Slug e nome opcionais alem do nome: so o nome e obrigatorio (slug deriva dele).
const canSubmit = computed(() => Boolean(form.name.trim() && slugPreview.value))

function submit(edit: boolean) {
  if (!canSubmit.value) return
  const accountId = form.accountId === AGENCY_SENTINEL ? '' : form.accountId.trim()
  emit('submit', {
    name: form.name.trim(),
    slug: form.slug.trim() ? slugPreview.value : '',
    accountId,
    edit,
  })
}
</script>

<template>
  <UModal :open="open" @update:open="emit('update:open', $event)">
    <template #content>
      <UCard>
        <template #header>
          <h3 class="text-base font-semibold">Nova bio</h3>
        </template>

        <div class="bio-create__form">
          <div v-if="isAdmin" class="bio-create__field">
            <label class="bio-create__label">Cliente</label>
            <USelect
              :model-value="form.accountId"
              :items="tenantItems"
              value-key="value"
              placeholder="Sem cliente (agencia)"
              @update:model-value="form.accountId = String($event ?? '')"
            />
            <p class="bio-create__hint">
              Opcional. Sem cliente, a bio fica na sua agencia (da para mover depois).
            </p>
          </div>

          <div class="bio-create__field">
            <label class="bio-create__label">Nome</label>
            <UInput
              :model-value="form.name"
              placeholder="Nome interno da bio"
              @update:model-value="form.name = String($event ?? '')"
            />
          </div>

          <div class="bio-create__field">
            <label class="bio-create__label">Slug</label>
            <UInput
              :model-value="form.slug"
              placeholder="auto-derivado do nome se vazio"
              @update:model-value="form.slug = String($event ?? '')"
            />
            <p class="bio-create__hint">
              Opcional. URL publica:
              <strong>/{{ slugPreview || '—' }}</strong>
            </p>
          </div>
        </div>

        <template #footer>
          <div class="bio-create__footer">
            <UButton
              label="Cancelar"
              color="neutral"
              variant="ghost"
              @click="emit('update:open', false)"
            />
            <UButton
              label="Criar"
              color="neutral"
              variant="soft"
              :loading="creating"
              :disabled="!canSubmit || creating"
              @click="submit(false)"
            />
            <UButton
              label="Criar e editar"
              color="primary"
              :loading="creating"
              :disabled="!canSubmit || creating"
              @click="submit(true)"
            />
          </div>
        </template>
      </UCard>
    </template>
  </UModal>
</template>

<style scoped>
.bio-create__form {
  display: grid;
  gap: 0.85rem;
}

.bio-create__field {
  display: grid;
  gap: 0.35rem;
}

.bio-create__label {
  color: var(--text-muted);
  font-size: 0.74rem;
  font-weight: 700;
}

.bio-create__hint {
  margin: 0;
  color: var(--text-muted);
  font-size: 0.72rem;
  line-height: 1.4;
}

.bio-create__footer {
  display: flex;
  justify-content: end;
  gap: 0.5rem;
}
</style>
