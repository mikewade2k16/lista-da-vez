<script setup>
import { Archive, KeyRound, Mail, RotateCcw } from 'lucide-vue-next'

import AppSelectField from '~/components/ui/AppSelectField.vue'
import { useUsersAccessContext } from '~/composables/useUsersAccessManager'

const ctx = useUsersAccessContext()
</script>

<template>
  <div class="users-access-manager__detail-grid">
    <article class="settings-card">
      <header class="settings-card__header">
        <div>
          <h3 class="settings-card__title">Dados do usuario</h3>
          <p class="settings-card__text">Esses campos atualizam a conta usada no login.</p>
        </div>
      </header>

      <div class="users-access-manager__detail-form-grid">
        <label class="settings-field">
          <span>Nome completo</span>
          <input
            v-model="ctx.detailDraft.displayName"
            type="text"
            :disabled="ctx.detailSaving || ctx.detailRoleLocked"
          />
        </label>

        <label class="settings-field">
          <span>Email</span>
          <input
            v-model="ctx.detailDraft.email"
            type="email"
            :disabled="ctx.detailSaving || ctx.detailRoleLocked"
          />
        </label>

        <label class="settings-field">
          <span>Matricula</span>
          <input
            v-model="ctx.detailDraft.employeeCode"
            type="text"
            :disabled="ctx.detailSaving || ctx.detailRoleLocked"
          />
        </label>

        <AppSelectField
          class="settings-field"
          label="Perfil"
          :model-value="ctx.detailDraft.role"
          :options="ctx.detailRoleOptions"
          :disabled="ctx.detailSaving || ctx.detailRoleLocked"
          @update:model-value="ctx.detailDraft.role = $event"
        />

        <AppSelectField
          v-if="ctx.auth.role === 'platform_admin'"
          class="settings-field"
          label="Cliente"
          :model-value="ctx.detailDraft.tenantId"
          :options="ctx.clientFilterOptions.filter((option) => option.value)"
          :disabled="
            ctx.detailSaving || ctx.detailRoleLocked || ctx.detailDraft.role === 'platform_admin'
          "
          @update:model-value="ctx.detailDraft.tenantId = $event"
        />

        <AppSelectField
          class="settings-field"
          label="Loja"
          :model-value="ctx.detailDraft.storeId"
          :options="ctx.detailStoreOptions"
          :disabled="
            ctx.detailSaving || ctx.detailRoleLocked || !ctx.isStoreScopedRole(ctx.detailDraft.role)
          "
          @update:model-value="ctx.detailDraft.storeId = $event"
        />
      </div>

      <label class="settings-toggle users-access-manager__detail-toggle">
        <input
          v-model="ctx.detailDraft.active"
          type="checkbox"
          :disabled="ctx.detailSaving || ctx.detailRoleLocked"
        />
        <span>Conta ativa</span>
      </label>
    </article>

    <article class="settings-card">
      <header class="settings-card__header">
        <div>
          <h3 class="settings-card__title">Acoes rapidas</h3>
          <p class="settings-card__text">
            Atalhos para convite, senha temporaria e status da conta.
          </p>
        </div>
      </header>

      <div class="users-access-manager__detail-action-list">
        <button
          v-if="ctx.canShowInviteAction(ctx.selectedDetailUser)"
          class="users-access-manager__detail-action-btn"
          type="button"
          @click="ctx.handleInviteAction(ctx.selectedDetailUser)"
        >
          <Mail :size="15" :stroke-width="2.15" />
          <span>
            {{
              ctx.normalizeText(ctx.selectedDetailUser.onboarding?.status) === 'pending'
                ? 'Copiar convite'
                : 'Gerar convite'
            }}
          </span>
        </button>

        <button
          v-if="ctx.canManagePasswords && ctx.selectedDetailUser.onboarding?.hasPassword"
          class="users-access-manager__detail-action-btn"
          type="button"
          @click="ctx.handleResetPassword(ctx.selectedDetailUser)"
        >
          <KeyRound :size="15" :stroke-width="2.15" />
          <span>Resetar senha</span>
        </button>

        <button
          class="users-access-manager__detail-action-btn"
          type="button"
          @click="ctx.handleArchiveAction(ctx.selectedDetailUser)"
        >
          <Archive v-if="ctx.selectedDetailUser.active" :size="15" :stroke-width="2.15" />
          <RotateCcw v-else :size="15" :stroke-width="2.15" />
          <span>{{ ctx.selectedDetailUser.active ? 'Inativar conta' : 'Reativar conta' }}</span>
        </button>
      </div>
    </article>
  </div>
</template>
