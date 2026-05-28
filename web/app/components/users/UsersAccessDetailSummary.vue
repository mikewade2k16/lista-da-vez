<script setup>
import { useUsersAccessContext } from '~/composables/useUsersAccessManager'
import UsersAccessRoleBadge from '~/components/users/UsersAccessRoleBadge.vue'

const ctx = useUsersAccessContext()
</script>

<template>
  <article class="settings-card users-access-manager__detail-summary-card">
    <header class="settings-card__header">
      <div>
        <h3 class="settings-card__title">Resumo do acesso</h3>
        <p class="settings-card__text">
          Edite os dados do usuario e ajuste o que ele pode ver ou alterar no painel.
        </p>
      </div>

      <span :class="ctx.getOnboardingTone(ctx.selectedDetailUser)">
        {{ ctx.getOnboardingLabel(ctx.selectedDetailUser) }}
      </span>
    </header>

    <div class="users-access-manager__detail-summary-grid">
      <article class="users-access-manager__detail-summary-item">
        <span>Perfil base</span>
        <UsersAccessRoleBadge :role="ctx.detailDraft.role" />
      </article>

      <article class="users-access-manager__detail-summary-item">
        <span>Escopo</span>
        <strong>
          {{
            ctx.isStoreScopedRole(ctx.detailDraft.role)
              ? ctx.getStoreName(ctx.detailDraft.storeId)
              : 'ALL'
          }}
        </strong>
      </article>

      <article v-if="ctx.showTenantControls" class="users-access-manager__detail-summary-item">
        <span>Cliente</span>
        <strong>
          {{
            ctx.tenantLookup.get(ctx.normalizeText(ctx.detailDraft.tenantId))?.name ||
            ctx.detailDraft.tenantId ||
            'Plataforma'
          }}
        </strong>
      </article>

      <article v-else class="users-access-manager__detail-summary-item">
        <span>Modulos</span>
        <strong>{{ ctx.getUserWorkspaceSummaryText(ctx.selectedDetailUser) }}</strong>
      </article>

      <article class="users-access-manager__detail-summary-item">
        <span>Origem</span>
        <strong>
          {{ ctx.isConsultantManaged(ctx.selectedDetailUser) ? 'Consultores' : 'Usuarios' }}
        </strong>
      </article>
    </div>

    <p v-if="ctx.detailRoleLocked" class="users-access-manager__detail-warning">
      Esse consultor continua gerenciado pelo roster. Nesta tela ele fica somente para consulta e
      reset de senha, quando permitido.
    </p>

    <p
      v-else-if="
        ctx.isConsultantManaged(ctx.selectedDetailUser) && ctx.canOverrideConsultantManaged
      "
      class="users-access-manager__detail-info"
    >
      Como admin da plataforma, voce pode reposicionar este consultor de loja e ajustar o papel
      vinculado a ele por aqui.
    </p>
  </article>
</template>
