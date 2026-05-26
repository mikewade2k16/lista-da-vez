<script setup>
import { Plus, X } from 'lucide-vue-next'

import AppSelectField from '~/components/ui/AppSelectField.vue'
import { useUsersAccessContext } from '~/composables/useUsersAccessManager'

const ctx = useUsersAccessContext()
</script>

<template>
  <div class="users-access-manager__create-shell">
    <div class="users-access-manager__launcher-row">
      <button
        class="users-access-manager__launcher"
        type="button"
        @click="ctx.createComposerOpen = !ctx.createComposerOpen"
      >
        <Plus class="users-access-manager__button-icon" :size="16" :stroke-width="2.15" />
        <span>{{ ctx.createComposerOpen ? 'Fechar cadastro' : 'Novo cadastro' }}</span>
      </button>

      <p class="users-access-manager__launcher-hint">
        Abra o cadastro rapido acima ou edite cada usuario no modal com visibilidade e override do
        painel.
      </p>
    </div>

    <transition name="users-access-manager-fade">
      <form
        v-if="ctx.createComposerOpen"
        class="users-access-manager__create-card"
        @submit.prevent="ctx.submitCreate"
      >
        <header class="users-access-manager__create-header">
          <div>
            <h3>Novo acesso</h3>
            <p>Abra o cadastro via convite ou defina senha inicial quando o perfil permitir.</p>
          </div>

          <button
            class="users-access-manager__close-btn"
            type="button"
            @click="ctx.createComposerOpen = false"
          >
            <X :size="16" :stroke-width="2.15" />
          </button>
        </header>

        <div class="users-access-manager__mode-switch">
          <button
            class="users-access-manager__mode-btn"
            :class="{ 'is-active': ctx.createMode === 'invite' }"
            type="button"
            @click="ctx.switchToInviteMode()"
          >
            Convite
          </button>

          <button
            v-if="ctx.canManagePasswords"
            class="users-access-manager__mode-btn"
            :class="{ 'is-active': ctx.createMode === 'password' }"
            type="button"
            @click="ctx.createMode = 'password'"
          >
            Senha inicial
          </button>
        </div>

        <div class="users-access-manager__create-grid">
          <input
            v-model="ctx.createDraft.displayName"
            class="users-access-manager__field"
            type="text"
            placeholder="Nome completo *"
          />
          <input
            v-model="ctx.createDraft.email"
            class="users-access-manager__field"
            type="email"
            placeholder="Email *"
          />
          <input
            v-model="ctx.createDraft.employeeCode"
            class="users-access-manager__field"
            type="text"
            placeholder="Matricula"
          />

          <input
            v-if="ctx.canManagePasswords && ctx.createMode === 'password'"
            v-model="ctx.createDraft.password"
            class="users-access-manager__field"
            type="password"
            placeholder="Senha inicial *"
          />

          <AppSelectField
            class="users-access-manager__select"
            :model-value="ctx.createDraft.role"
            :options="ctx.createRoleOptions"
            :show-leading-icon="false"
            placeholder="Perfil"
            @update:model-value="ctx.createDraft.role = $event"
          />

          <AppSelectField
            v-if="ctx.showTenantControls"
            class="users-access-manager__select"
            :model-value="ctx.createDraft.tenantId"
            :options="ctx.clientFilterOptions.filter((option) => option.value)"
            :show-leading-icon="false"
            placeholder="Cliente"
            @update:model-value="ctx.createDraft.tenantId = $event"
          />

          <AppSelectField
            class="users-access-manager__select"
            :model-value="ctx.createDraft.storeId"
            :options="
              ctx.isStoreScopedRole(ctx.createDraft.role)
                ? ctx.getScopedStoreOptions(ctx.createDraft.tenantId)
                : [{ value: ctx.allStoresValue, label: 'ALL' }]
            "
            :show-leading-icon="false"
            :disabled="!ctx.isStoreScopedRole(ctx.createDraft.role)"
            placeholder="Loja"
            @update:model-value="ctx.createDraft.storeId = $event"
          />
        </div>

        <div class="users-access-manager__create-actions">
          <label class="users-access-manager__checkbox">
            <input v-model="ctx.createDraft.active" type="checkbox" />
            <span>Criar conta ativa</span>
          </label>

          <button class="users-access-manager__submit-btn" type="submit">
            {{
              ctx.canManagePasswords && ctx.createMode === 'password'
                ? 'Criar acesso'
                : 'Enviar convite'
            }}
          </button>
        </div>

        <p class="users-access-manager__hint">
          Consultores seguem vinculados ao roster e continuam sendo gerenciados na area de
          consultores, nao por esta tela.
        </p>
      </form>
    </transition>
  </div>
</template>
