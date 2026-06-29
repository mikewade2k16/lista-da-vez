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

// adminEmail e OPCIONAL (cliente de controle interno pode nascer sem usuario).
// So nome e obrigatorio; o slug deriva do nome quando vazio.
const canSubmit = computed(() => Boolean(form.name.trim() && slugPreview.value))

// Feedback do que falta quando o submit esta travado — nunca deixar o botao morto
// sem explicar o porque.
const missingHint = computed(() => {
  if (!form.name.trim()) return 'Informe o nome do cliente para continuar.'
  if (!slugPreview.value) return 'Nao foi possivel derivar o slug; informe um slug valido.'
  return ''
})

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
            <label class="account-create__label">
              Nome
              <span class="account-create__required" aria-hidden="true">*</span>
            </label>
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
            <label class="account-create__label">
              E-mail do admin inicial
              <span class="account-create__optional">(opcional)</span>
            </label>
            <UInput
              :model-value="form.adminEmail"
              placeholder="admin@cliente.com (deixe vazio se for so controle interno)"
              @update:model-value="form.adminEmail = String($event ?? '')"
            />
            <p class="account-create__hint">
              Deixe vazio para um cliente so de controle interno (sem usuario/acesso). Se informar,
              o e-mail deve ser de um usuario ja existente em core.users — o backend vincula esse
              usuario como owner do novo cliente.
            </p>
          </div>
        </div>

        <template #footer>
          <div class="account-create__footer">
            <p v-if="missingHint" class="account-create__missing">{{ missingHint }}</p>
            <div class="account-create__footer-actions">
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

.account-create__required {
  color: rgb(var(--danger));
  margin-left: 0.15rem;
}

.account-create__optional {
  color: var(--text-muted);
  font-weight: 500;
  font-size: 0.7rem;
  margin-left: 0.25rem;
}

.account-create__hint {
  margin: 0;
  color: var(--text-muted);
  font-size: 0.72rem;
  line-height: 1.4;
}

.account-create__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.account-create__missing {
  margin: 0;
  color: rgb(var(--danger));
  font-size: 0.74rem;
  line-height: 1.3;
}

.account-create__footer-actions {
  display: flex;
  justify-content: end;
  gap: 0.5rem;
  margin-left: auto;
}
</style>
