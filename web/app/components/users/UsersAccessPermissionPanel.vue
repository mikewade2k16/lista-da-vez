<script setup>
import AppSelectField from '~/components/ui/AppSelectField.vue'
import { useUsersAccessContext } from '~/composables/useUsersAccessManager'

const ctx = useUsersAccessContext()
</script>

<template>
  <article class="settings-card">
    <header class="settings-card__header">
      <div>
        <h3 class="settings-card__title">Acesso ao painel</h3>
        <p class="settings-card__text">
          Cada override abaixo sobrescreve somente esse usuario em cima do papel selecionado.
        </p>
      </div>
    </header>

    <p v-if="ctx.detailLoading" class="users-access-manager__detail-loading">
      Carregando matriz efetiva do usuario...
    </p>

    <div v-else-if="ctx.detailAccessError" class="users-access-manager__detail-error-card">
      <div>
        <strong>Permissoes indisponiveis neste ambiente.</strong>
        <p>{{ ctx.detailAccessError }}</p>
      </div>

      <button
        class="users-access-manager__detail-retry-btn"
        type="button"
        @click="ctx.refreshDetail(ctx.selectedDetailUser.id)"
      >
        Tentar novamente
      </button>
    </div>

    <div v-else class="users-access-manager__permission-grid">
      <div
        v-for="workspaceRow in ctx.detailWorkspaceRows"
        :key="workspaceRow.id"
        class="users-access-manager__permission-row"
      >
        <div class="users-access-manager__permission-copy">
          <strong>{{ workspaceRow.label }}</strong>
          <p>{{ workspaceRow.description }}</p>

          <div class="users-access-manager__permission-meta">
            <span :class="ctx.getAccessStateTone(workspaceRow.baseState)">
              Perfil: {{ ctx.getAccessStateLabel(workspaceRow.baseState) }}
            </span>
            <span :class="ctx.getAccessStateTone(workspaceRow.effectiveState)">
              Efetivo: {{ ctx.getAccessStateLabel(workspaceRow.effectiveState) }}
            </span>
          </div>
        </div>

        <AppSelectField
          class="users-access-manager__permission-select"
          label="Override"
          :model-value="workspaceRow.overrideState"
          :options="ctx.getWorkspaceAccessOptions(workspaceRow, { includeInherit: true })"
          :disabled="ctx.detailSaving || ctx.detailRoleLocked || !ctx.detailAccessReady"
          @update:model-value="ctx.detailWorkspaceStates[workspaceRow.id] = $event"
        />
      </div>
    </div>
  </article>

  <article v-if="!ctx.detailAccessError" class="settings-card">
    <header class="settings-card__header">
      <div>
        <h3 class="settings-card__title">Permissoes sensiveis</h3>
        <p class="settings-card__text">
          Use apenas quando o usuario precisar sair do padrao do tipo.
        </p>
      </div>
    </header>

    <div
      class="users-access-manager__permission-grid users-access-manager__permission-grid--advanced"
    >
      <div
        v-for="permissionRow in ctx.detailAdvancedRows"
        :key="permissionRow.key"
        class="users-access-manager__permission-row"
      >
        <div class="users-access-manager__permission-copy">
          <strong>{{ permissionRow.label }}</strong>
          <p>{{ permissionRow.description }}</p>

          <div class="users-access-manager__permission-meta">
            <span :class="ctx.getAccessStateTone(permissionRow.baseEnabled ? 'allow' : 'none')">
              Perfil: {{ permissionRow.baseEnabled ? 'Permitido' : 'Nao permitido' }}
            </span>
            <span
              :class="ctx.getAccessStateTone(permissionRow.effectiveEnabled ? 'allow' : 'none')"
            >
              Efetivo: {{ permissionRow.effectiveEnabled ? 'Permitido' : 'Nao permitido' }}
            </span>
          </div>
        </div>

        <AppSelectField
          class="users-access-manager__permission-select"
          label="Override"
          :model-value="permissionRow.overrideState"
          :options="ctx.permissionOverrideOptions"
          :disabled="ctx.detailSaving || ctx.detailRoleLocked || !ctx.detailAccessReady"
          @update:model-value="ctx.detailAdvancedStates[permissionRow.key] = $event"
        />
      </div>
    </div>
  </article>

  <footer class="users-access-manager__detail-footer">
    <p class="users-access-manager__detail-footer-note">
      {{
        ctx.detailAccessError
          ? 'Os dados do usuario ainda podem ser salvos. A parte de permissoes volta a funcionar quando a API de access estiver ativa.'
          : 'O acesso efetivo acima ja considera o papel escolhido no modal e os overrides desta edicao.'
      }}
    </p>

    <button
      class="users-access-manager__submit-btn"
      type="button"
      :disabled="ctx.detailSaving || ctx.detailLoading || ctx.detailRoleLocked"
      @click="ctx.saveDetails"
    >
      {{
        ctx.detailSaving
          ? 'Salvando...'
          : ctx.detailAccessError
            ? 'Salvar dados do usuario'
            : 'Salvar alteracoes'
      }}
    </button>
  </footer>
</template>
