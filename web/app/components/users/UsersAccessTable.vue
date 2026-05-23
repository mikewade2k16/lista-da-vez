<script setup>
import { Archive, Info, KeyRound, Mail, RotateCcw } from 'lucide-vue-next'

import AppEntityGrid from '~/components/ui/AppEntityGrid.vue'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import AppToggleSwitch from '~/components/ui/AppToggleSwitch.vue'
import { useUsersAccessContext } from '~/composables/useUsersAccessManager'

const ctx = useUsersAccessContext()
</script>

<template>
  <AppEntityGrid
    :columns="ctx.gridColumns"
    :rows="ctx.filteredUsers"
    :loading="ctx.usersStore.pending"
    :search-value="ctx.filters.search"
    :storage-key="'users-access-grid-columns-v1'"
    empty-title="Nenhum usuario encontrado"
    empty-text="Ajuste os filtros ou abra um novo cadastro para preencher a grade."
    testid="users-access-grid"
    @update:search-value="ctx.filters.search = $event"
  >
    <template #toolbar-filters>
      <AppSelectField
        class="users-access-manager__toolbar-select"
        :model-value="ctx.filters.status"
        :options="ctx.statusFilterOptions"
        :show-leading-icon="false"
        compact
        @update:model-value="ctx.filters.status = $event"
      />

      <AppSelectField
        class="users-access-manager__toolbar-select"
        :model-value="ctx.filters.role"
        :options="ctx.filterRoleOptions"
        :show-leading-icon="false"
        compact
        @update:model-value="ctx.filters.role = $event"
      />

      <AppSelectField
        v-if="ctx.auth.role === 'platform_admin'"
        class="users-access-manager__toolbar-select"
        :model-value="ctx.filters.tenant"
        :options="ctx.clientFilterOptions"
        :show-leading-icon="false"
        compact
        @update:model-value="ctx.filters.tenant = $event"
      />

      <AppSelectField
        class="users-access-manager__toolbar-select"
        :model-value="ctx.filters.store"
        :options="ctx.storeFilterOptions"
        :show-leading-icon="false"
        compact
        @update:model-value="ctx.filters.store = $event"
      />
    </template>

    <template #toolbar-actions>
      <span class="users-access-manager__counter">{{ ctx.filteredUsers.length }} registros</span>
      <button class="users-access-manager__ghost-btn" type="button" @click="ctx.clearFilters">
        Limpar
      </button>
    </template>

    <template #cell-name="{ row }">
      <div class="users-access-manager__identity-cell">
        <input
          v-if="!ctx.isInlineLocked(row)"
          v-model="ctx.getRowDraft(row).displayName"
          class="users-access-manager__inline-input"
          type="text"
          @blur="ctx.handleInlineBlur(row)"
          @keydown.enter.prevent="$event.target.blur()"
        />
        <div v-else class="users-access-manager__locked-copy">
          <strong>{{ row.displayName }}</strong>
          <small>Gerenciado pelo roster</small>
        </div>
        <small class="users-access-manager__subcopy">
          {{ row.jobTitle || ctx.getRoleLabel(row.role) }}
        </small>
      </div>
    </template>

    <template #cell-nick="{ row }">
      <span class="users-access-manager__nick-chip">{{ ctx.buildNickname(row.displayName) }}</span>
    </template>

    <template #cell-email="{ row }">
      <input
        v-if="!ctx.isInlineLocked(row)"
        v-model="ctx.getRowDraft(row).email"
        class="users-access-manager__inline-input"
        type="email"
        @blur="ctx.handleInlineBlur(row)"
        @keydown.enter.prevent="$event.target.blur()"
      />
      <span v-else class="users-access-manager__static-copy">{{ row.email }}</span>
    </template>

    <template #cell-status="{ row }">
      <AppToggleSwitch
        compact
        :model-value="ctx.getRowDraft(row).active"
        :disabled="ctx.rowBusy[row.id] || ctx.isInlineLocked(row)"
        @change="ctx.handleStatusChange(row, $event)"
      />
    </template>

    <template #cell-profile="{ row }">
      <AppSelectField
        class="users-access-manager__inline-select"
        :model-value="ctx.getRowDraft(row).role"
        :options="ctx.getRoleSelectOptions(row)"
        :show-leading-icon="false"
        compact
        :disabled="ctx.rowBusy[row.id] || ctx.isInlineLocked(row)"
        @update:model-value="ctx.handleRoleChange(row, $event)"
      />
    </template>

    <template #cell-store="{ row }">
      <AppSelectField
        class="users-access-manager__inline-select"
        :model-value="ctx.getRowDraft(row).storeId"
        :options="ctx.getStoreSelectOptions(row, ctx.getRowDraft(row))"
        :show-leading-icon="false"
        compact
        :disabled="ctx.rowBusy[row.id] || ctx.isInlineLocked(row)"
        @update:model-value="ctx.handleStoreChange(row, $event)"
      />
    </template>

    <template #cell-employeeCode="{ row }">
      <input
        v-if="!ctx.isInlineLocked(row)"
        v-model="ctx.getRowDraft(row).employeeCode"
        class="users-access-manager__inline-input users-access-manager__inline-input--compact"
        type="text"
        placeholder="-"
        @blur="ctx.handleInlineBlur(row)"
        @keydown.enter.prevent="$event.target.blur()"
      />
      <span v-else class="users-access-manager__static-copy">{{ row.employeeCode || '-' }}</span>
    </template>

    <template #cell-onboarding="{ row }">
      <span :class="ctx.getOnboardingTone(row)">{{ ctx.getOnboardingLabel(row) }}</span>
    </template>

    <template #cell-actions="{ row }">
      <div class="users-access-manager__actions">
        <button
          class="users-access-manager__icon-btn"
          type="button"
          title="Ver detalhes"
          @click="ctx.openDetails(row)"
        >
          <Info :size="15" :stroke-width="2.15" />
        </button>

        <button
          v-if="ctx.canShowInviteAction(row)"
          class="users-access-manager__icon-btn"
          type="button"
          :title="
            ctx.normalizeText(row.onboarding?.status) === 'pending'
              ? 'Copiar convite'
              : 'Gerar convite'
          "
          @click="ctx.handleInviteAction(row)"
        >
          <Mail :size="15" :stroke-width="2.15" />
        </button>

        <button
          v-if="ctx.canManagePasswords && row.onboarding?.hasPassword"
          class="users-access-manager__icon-btn"
          type="button"
          title="Resetar senha"
          @click="ctx.handleResetPassword(row)"
        >
          <KeyRound :size="15" :stroke-width="2.15" />
        </button>

        <button
          class="users-access-manager__icon-btn"
          type="button"
          :title="row.active ? 'Inativar' : 'Reativar'"
          @click="ctx.handleArchiveAction(row)"
        >
          <Archive v-if="row.active" :size="15" :stroke-width="2.15" />
          <RotateCcw v-else :size="15" :stroke-width="2.15" />
        </button>
      </div>
    </template>
  </AppEntityGrid>
</template>
