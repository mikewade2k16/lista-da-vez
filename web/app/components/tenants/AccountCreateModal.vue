<script setup lang="ts">
import { computed, reactive, watch } from 'vue'

const props = defineProps<{ open: boolean; creating?: boolean }>()
const emit = defineEmits<{
  (e: 'update:open', value: boolean): void
  (e: 'submit', payload: { name: string; slug: string; planCode: string; adminEmail: string }): void
}>()

const form = reactive({ name: '', slug: '', planCode: 'standard', adminEmail: '' })

watch(
  () => props.open,
  (open) => {
    if (!open) return
    form.name = ''
    form.slug = ''
    form.planCode = 'standard'
    form.adminEmail = ''
  },
)

function slugify(value: string): string {
  return value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
}

// Preview do slug que sera enviado: usa o que foi digitado; se vazio, deriva do
// nome. NAO pre-preenchemos o input (isso causava slug duplicado quando o campo
// ja vinha com valor e recebia digitacao por cima — concatenava).
const slugPreview = computed(() => (form.slug.trim() ? slugify(form.slug) : slugify(form.name)))

const canSubmit = computed(() =>
  Boolean(form.name.trim() && slugPreview.value && form.adminEmail.trim()),
)

function submit() {
  if (!canSubmit.value) return
  emit('submit', {
    name: form.name.trim(),
    slug: slugPreview.value,
    planCode: form.planCode.trim() || 'standard',
    adminEmail: form.adminEmail.trim().toLowerCase(),
  })
}
</script>

<template>
  <UModal :open="open" @update:open="emit('update:open', $event)">
    <template #content>
      <UCard>
        <template #header>
          <h3 class="text-base font-semibold">Nova conta</h3>
        </template>

        <div class="account-create__form">
          <div class="account-create__field">
            <label class="account-create__label">Nome</label>
            <UInput
              :model-value="form.name"
              placeholder="Nome da conta"
              @update:model-value="form.name = String($event ?? '')"
            />
          </div>
          <div class="account-create__field">
            <label class="account-create__label">Slug</label>
            <UInput
              :model-value="form.slug"
              placeholder="auto-derivado do nome se vazio"
              @update:model-value="form.slug = String($event ?? '')"
            />
            <p class="account-create__hint">
              Sera salvo como:
              <strong>{{ slugPreview || '—' }}</strong>
            </p>
          </div>
          <div class="account-create__field">
            <label class="account-create__label">Plano</label>
            <UInput
              :model-value="form.planCode"
              placeholder="standard"
              @update:model-value="form.planCode = String($event ?? '')"
            />
          </div>
          <div class="account-create__field">
            <label class="account-create__label">E-mail do admin inicial</label>
            <UInput
              :model-value="form.adminEmail"
              placeholder="admin@cliente.com (usuario ja existente)"
              @update:model-value="form.adminEmail = String($event ?? '')"
            />
            <p class="account-create__hint">
              O admin deve ja existir em core.users. O backend cria a conta, clona os cargos do
              template e vincula o admin como owner.
            </p>
          </div>
        </div>

        <template #footer>
          <div class="account-create__footer">
            <UButton
              label="Cancelar"
              color="neutral"
              variant="ghost"
              @click="emit('update:open', false)"
            />
            <UButton
              label="Criar conta"
              color="primary"
              :loading="creating"
              :disabled="!canSubmit || creating"
              @click="submit"
            />
          </div>
        </template>
      </UCard>
    </template>
  </UModal>
</template>

<style scoped>
.account-create__form {
  display: grid;
  gap: 0.85rem;
}

.account-create__field {
  display: grid;
  gap: 0.35rem;
}

.account-create__label {
  color: var(--text-muted);
  font-size: 0.74rem;
  font-weight: 700;
}

.account-create__hint {
  margin: 0;
  color: var(--text-muted);
  font-size: 0.72rem;
  line-height: 1.4;
}

.account-create__footer {
  display: flex;
  justify-content: end;
  gap: 0.5rem;
}
</style>
