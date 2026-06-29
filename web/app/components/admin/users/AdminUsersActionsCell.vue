<script setup lang="ts">
import OmniMinimalPopover from '~/components/omni/overlay/OmniMinimalPopover.vue'
import { useAdminUsersManager } from '~/composables/useAdminUsersManager'
import type { AccountMembershipItem, AdminUserItem } from '~/types/admin-users'

// Celula de acoes da linha de usuario (popover de memberships + popover de detalhes
// + botoes editar/senha/excluir), fatiada do AdminUsersWorkspace para o host ficar
// fino. Apenas apresentacao + estado LOCAL dos dois popovers desta linha; as acoes
// sobem como eventos. O fetch de memberships usa o manager COMPARTILHADO (inject),
// igual ao host antigo. Comportamento/eventos IDENTICOS ao slot inline anterior.

const props = defineProps<{
  user: AdminUserItem
  canView: boolean
  canManage: boolean
  canDelete: boolean
  // True enquanto este usuario esta sendo excluido (spinner no botao de lixeira).
  deleting: boolean
}>()
const emit = defineEmits<{ edit: []; password: []; delete: [] }>()

const { fetchMemberships } = useAdminUsersManager()

// Estado controlado dos popovers desta linha (OmniMinimalPopover e controlled).
const membershipsOpen = ref(false)
const infoOpen = ref(false)

// Memberships carregadas sob demanda ao abrir o popover (mesma logica do host antigo:
// busca quando o popover dispara @opened). `loaded` evita renderizar a lista antes do fetch.
const memberships = ref<AccountMembershipItem[]>([])
const membershipsLoaded = ref(false)

async function onMembershipsOpened() {
  membershipsLoaded.value = false
  memberships.value = await fetchMemberships(props.user.id)
  membershipsLoaded.value = true
}
</script>

<template>
  <div class="flex items-center justify-end gap-1">
    <OmniMinimalPopover
      :open="membershipsOpen"
      title="Clientes (memberships)"
      width-class="w-[300px] max-w-[90vw]"
      @update:open="membershipsOpen = $event"
      @opened="onMembershipsOpened"
    >
      <template #trigger>
        <UButton
          icon="i-lucide-building-2"
          color="neutral"
          variant="ghost"
          size="sm"
          title="Contas que este usuario participa"
          aria-label="Memberships"
        />
      </template>
      <div v-if="membershipsLoaded" class="space-y-2 text-xs">
        <p v-if="memberships.length === 0" class="text-[rgb(var(--muted))]">
          Este usuario nao e membro de nenhuma conta.
        </p>
        <ul v-else class="space-y-1">
          <li
            v-for="m in memberships"
            :key="m.accountId"
            class="flex items-center justify-between gap-2 rounded-[var(--radius-sm)] border border-[rgb(var(--border))] bg-[rgb(var(--surface-2))] px-2 py-1"
          >
            <span class="font-medium">{{ m.accountName }}</span>
            <span class="text-[rgb(var(--muted))]">{{ m.accountSlug }}</span>
            <UBadge :color="m.isActive ? 'success' : 'neutral'" variant="soft" size="xs">
              {{ m.isActive ? 'ativo' : 'inativo' }}
            </UBadge>
          </li>
        </ul>
      </div>
    </OmniMinimalPopover>

    <OmniMinimalPopover
      :open="infoOpen"
      title="Detalhes"
      width-class="w-[280px] max-w-[90vw]"
      @update:open="infoOpen = $event"
    >
      <template #trigger>
        <UButton
          icon="i-lucide-info"
          color="neutral"
          variant="ghost"
          size="sm"
          title="Detalhes do usuario"
          aria-label="Info"
        />
      </template>
      <div class="space-y-1 text-xs">
        <p>
          <strong>ID:</strong>
          {{ user.id }}
        </p>
        <p>
          <strong>Email:</strong>
          {{ user.email }}
        </p>
        <p>
          <strong>Nome:</strong>
          {{ user.displayName }}
        </p>
        <p>
          <strong>Nick:</strong>
          {{ user.nick || '-' }}
        </p>
        <p>
          <strong>Platform admin:</strong>
          {{ user.isPlatformAdmin ? 'sim' : 'nao' }}
        </p>
        <p>
          <strong>Trocar senha:</strong>
          {{ user.mustChangePassword ? 'sim' : 'nao' }}
        </p>
        <p>
          <strong>Qtd clientes:</strong>
          {{ user.accountCount }}
        </p>
        <p>
          <strong>Cliente:</strong>
          {{ user.accountNames || '-' }}
        </p>
        <p>
          <strong>Membro de agencia:</strong>
          {{ user.isAgencyMember ? 'sim' : 'nao' }}
        </p>
      </div>
    </OmniMinimalPopover>

    <UButton
      v-if="canView"
      icon="i-lucide-pencil"
      color="neutral"
      variant="ghost"
      size="sm"
      title="Editar usuario (dados, vinculos, papeis, modulos)"
      aria-label="Editar"
      @click="emit('edit')"
    />

    <UButton
      v-if="canManage"
      icon="i-lucide-key-round"
      color="neutral"
      variant="ghost"
      size="sm"
      title="Definir/Resetar senha"
      aria-label="Definir senha"
      @click="emit('password')"
    />

    <UButton
      v-if="canDelete"
      icon="i-lucide-trash-2"
      color="error"
      variant="ghost"
      size="sm"
      title="Desativar usuario"
      aria-label="Excluir"
      :loading="deleting"
      @click="emit('delete')"
    />
  </div>
</template>
