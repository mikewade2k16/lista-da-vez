<script setup>
import AppDetailDialog from '~/components/ui/AppDetailDialog.vue'
import { useUsersAccessContext } from '~/composables/useUsersAccessManager'
import UsersAccessDetailForm from '~/components/users/UsersAccessDetailForm.vue'
import UsersAccessDetailSummary from '~/components/users/UsersAccessDetailSummary.vue'
import UsersAccessPermissionPanel from '~/components/users/UsersAccessPermissionPanel.vue'

const ctx = useUsersAccessContext()
</script>

<template>
  <AppDetailDialog
    :model-value="Boolean(ctx.selectedDetailUser)"
    :title="ctx.selectedDetailUser?.displayName || 'Editar acesso'"
    :subtitle="
      ctx.selectedDetailUser
        ? `${ctx.getRoleLabel(ctx.detailDraft.role)} - ${ctx.selectedDetailUser.email}`
        : ''
    "
    :sections="[]"
    width="min(72rem, calc(100vw - 2rem))"
    @update:model-value="!$event && ctx.closeDetails()"
  >
    <div v-if="ctx.selectedDetailUser" class="users-access-manager__detail-layout">
      <UsersAccessDetailSummary />
      <UsersAccessDetailForm />
      <UsersAccessPermissionPanel />
    </div>
  </AppDetailDialog>
</template>
