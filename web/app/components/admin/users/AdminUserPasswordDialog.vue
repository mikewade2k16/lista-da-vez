<script setup lang="ts">
import { useAdminUsersManager } from '~/composables/useAdminUsersManager'

// Modal de definir/resetar senha de um usuario ja criado, fatiado do
// AdminUsersWorkspace para o host ficar fino. Comportamento IDENTICO: mesma
// validacao de minimo, mesmo setPassword no manager COMPARTILHADO, fecha so em
// sucesso. So platform_admin chega aqui (o host gateia a abertura).

const PASSWORD_MIN_LENGTH = 8

const props = defineProps<{ open: boolean; target: { id: string; email: string } | null }>()
const emit = defineEmits<{ 'update:open': [boolean] }>()

const { setPassword } = useAdminUsersManager()

const passwordValue = ref('')
const passwordSaving = ref(false)
const passwordError = computed(() => {
  const pw = passwordValue.value.trim()
  if (!pw) return ''
  return pw.length < PASSWORD_MIN_LENGTH ? `Minimo de ${PASSWORD_MIN_LENGTH} caracteres.` : ''
})

// Abrir o modal limpa o campo (espelha o openPassword antigo, que zerava o valor).
watch(
  () => props.open,
  (isOpen) => {
    if (isOpen) passwordValue.value = ''
  },
)

async function submitPassword() {
  const target = props.target
  const pw = passwordValue.value.trim()
  if (!target || pw.length < PASSWORD_MIN_LENGTH) return
  passwordSaving.value = true
  const ok = await setPassword(target.id, pw)
  passwordSaving.value = false
  if (ok) emit('update:open', false)
}
</script>

<template>
  <UModal :open="open" @update:open="emit('update:open', $event)">
    <template #content>
      <UCard>
        <template #header>
          <h3 class="text-base font-semibold">Definir senha</h3>
        </template>

        <div class="space-y-3">
          <p class="text-xs text-[rgb(var(--muted))]">
            Define uma nova senha para
            <strong>{{ target?.email || 'este usuario' }}</strong>
            . O usuario passa a logar com ela imediatamente.
          </p>
          <div>
            <label class="block text-xs text-[rgb(var(--muted))] mb-1">Nova senha</label>
            <UInput
              :model-value="passwordValue"
              type="password"
              placeholder="minimo 8 chars"
              @update:model-value="passwordValue = String($event ?? '')"
              @keyup.enter="submitPassword"
            />
            <p v-if="passwordError" class="text-xs text-[rgb(var(--danger))] mt-1">
              {{ passwordError }}
            </p>
          </div>
        </div>

        <template #footer>
          <div class="flex justify-end gap-2">
            <UButton
              label="Cancelar"
              color="neutral"
              variant="ghost"
              @click="emit('update:open', false)"
            />
            <UButton
              label="Salvar senha"
              color="primary"
              :loading="passwordSaving"
              :disabled="passwordSaving || Boolean(passwordError) || !passwordValue.trim()"
              @click="submitPassword"
            />
          </div>
        </template>
      </UCard>
    </template>
  </UModal>
</template>
