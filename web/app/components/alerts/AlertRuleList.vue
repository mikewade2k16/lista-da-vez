<script setup lang="ts">
const props = defineProps<{
  rules: Array<Record<string, any>>
  pending?: boolean
}>()

const emit = defineEmits<{
  edit: [rule: Record<string, any>]
  delete: [ruleId: string]
  toggle: [ruleId: string, isActive: boolean]
  'apply-now': [ruleId: string]
}>()

const triggerTypeLabels: Record<string, string> = {
  long_open_service: 'Atendimento longo',
  long_queue_wait: 'Fila longa',
  long_pause: 'Pausa longa',
  idle_store: 'Loja parada',
  outside_business_hours: 'Fora do horário',
}

const displayKindLabels: Record<string, string> = {
  card_badge: 'Badge',
  banner: 'Banner',
  toast: 'Toast',
  corner_popup: 'Popup',
  center_modal: 'Modal',
  fullscreen: 'Fullscreen',
}

const getTriggerLabel = (type: string) => triggerTypeLabels[type] || type
const getDisplayLabel = (kind: string) => displayKindLabels[kind] || kind
</script>

<template>
  <div class="alert-rule-list">
    <div v-if="rules.length === 0" class="empty-state">
      <div class="empty-state__icon">🎯</div>
      <p class="empty-state__title">Nenhuma regra de alerta configurada</p>
      <p class="empty-state__text">
        Comece criando uma nova regra para personalizar os alertas operacionais
      </p>
      <p class="empty-state__hint">
        Escolha um gatilho (atendimento longo, fila, pausa), tipo de display (banner, popup, modal,
        tela cheia) e configure templates com cores dinâmicas
      </p>
    </div>

    <table v-else class="rules-table">
      <thead>
        <tr>
          <th>Nome</th>
          <th>Gatilho</th>
          <th>Limite</th>
          <th>Display</th>
          <th>Status</th>
          <th>Atualizado</th>
          <th>Ações</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="rule in rules" :key="rule.id" :class="{ inactive: !rule.isActive }">
          <td class="name-cell">
            <div class="rule-name">{{ rule.name }}</div>
            <div class="rule-desc">{{ rule.description }}</div>
          </td>
          <td>{{ getTriggerLabel(rule.triggerType) }}</td>
          <td class="threshold">{{ rule.thresholdMinutes }}m</td>
          <td>{{ getDisplayLabel(rule.displayKind) }}</td>
          <td>
            <button
              :class="['toggle-btn', rule.isActive ? 'active' : 'inactive']"
              :disabled="pending"
              :title="rule.isActive ? 'Desativar' : 'Ativar'"
              @click="emit('toggle', rule.id, !rule.isActive)"
            >
              {{ rule.isActive ? '✓ Ativa' : '✕ Inativa' }}
            </button>
          </td>
          <td class="updated-at">
            {{ new Date(rule.updatedAt).toLocaleDateString() }}
          </td>
          <td class="actions-cell">
            <button
              class="btn-icon edit"
              :disabled="pending"
              title="Editar"
              @click="emit('edit', rule)"
            >
              ✎
            </button>
            <button
              class="btn-icon apply"
              :disabled="pending"
              title="Aplicar agora"
              @click="emit('apply-now', rule.id)"
            >
              ⚡
            </button>
            <button
              class="btn-icon delete"
              :disabled="pending"
              title="Deletar"
              @click="emit('delete', rule.id)"
            >
              🗑
            </button>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<style scoped>
.alert-rule-list {
  width: 100%;
}

.empty-state {
  text-align: center;
  padding: 3rem 2rem;
  color: rgb(var(--muted));
  background: linear-gradient(135deg, rgb(var(--surface)) 0%, rgb(var(--surface-2)) 100%);
  border-radius: 12px;
  border: 1px solid rgb(var(--border) / 0.82);
}

.empty-state__icon {
  font-size: 3rem;
  margin-bottom: 1rem;
}

.empty-state__title {
  margin: 0 0 0.5rem 0;
  font-size: 1.1rem;
  font-weight: 600;
  color: rgb(var(--text));
}

.empty-state__text {
  margin: 0 0 1rem 0;
  font-size: 0.95rem;
  color: rgb(var(--muted));
}

.empty-state__hint {
  margin: 0;
  font-size: 0.85rem;
  color: rgb(var(--muted) / 0.72);
  font-style: italic;
}

.rules-table {
  width: 100%;
  border-collapse: collapse;
  background: rgb(var(--surface) / 0.78);
  border-radius: 8px;
  overflow: hidden;
  box-shadow: var(--shadow-sm);
  border: 1px solid rgb(var(--border) / 0.82);
}

.rules-table thead {
  background: rgb(var(--surface-2) / 0.86);
  border-bottom: 1px solid rgb(var(--border) / 0.82);
}

.rules-table th {
  padding: 1rem;
  text-align: left;
  font-weight: 600;
  color: rgb(var(--text));
  font-size: 0.9rem;
}

.rules-table td {
  padding: 1rem;
  border-bottom: 1px solid rgb(var(--border) / 0.72);
  color: rgb(var(--text));
}

.rules-table tbody tr:last-child td {
  border-bottom: none;
}

.rules-table tbody tr.inactive {
  opacity: 0.5;
  background: rgb(var(--muted) / 0.1);
}

.rules-table tbody tr:hover {
  background: rgb(var(--primary) / 0.05);
}

.name-cell {
  min-width: 200px;
}

.rule-name {
  font-weight: 500;
  color: rgb(var(--text));
}

.rule-desc {
  font-size: 0.85rem;
  color: rgb(var(--muted));
  margin-top: 0.25rem;
}

.threshold {
  text-align: center;
  font-weight: 500;
}

.toggle-btn {
  padding: 0.4rem 0.8rem;
  border-radius: 4px;
  border: none;
  font-weight: 500;
  cursor: pointer;
  font-size: 0.85rem;
  transition: all 0.2s;
}

.toggle-btn.active {
  background: rgb(var(--success) / 0.14);
  color: rgb(var(--success));
}

.toggle-btn.inactive {
  background: rgb(var(--danger) / 0.14);
  color: rgb(var(--danger));
}

.toggle-btn:hover:not(:disabled) {
  opacity: 0.8;
}

.toggle-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.updated-at {
  font-size: 0.9rem;
  color: rgb(var(--muted));
}

.actions-cell {
  display: flex;
  gap: 0.5rem;
}

.btn-icon {
  background: none;
  border: 1px solid rgb(var(--border) / 0.86);
  border-radius: 4px;
  width: 2rem;
  height: 2rem;
  cursor: pointer;
  font-size: 1rem;
  transition: all 0.2s;
  display: flex;
  align-items: center;
  justify-content: center;
}

.btn-icon:hover:not(:disabled) {
  border-color: rgb(var(--ring) / 0.38);
  background: rgb(var(--primary) / 0.1);
}

.btn-icon:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-icon.edit {
  border-color: rgb(var(--primary));
  color: rgb(var(--primary));
}

.btn-icon.edit:hover:not(:disabled) {
  background: rgb(var(--primary) / 0.16);
}

.btn-icon.apply {
  border-color: rgb(var(--primary-600));
  color: rgb(var(--primary-600));
}

.btn-icon.apply:hover:not(:disabled) {
  background: rgb(var(--primary) / 0.12);
}

.btn-icon.delete {
  border-color: rgb(var(--danger));
  color: rgb(var(--danger));
}

.btn-icon.delete:hover:not(:disabled) {
  background: rgb(var(--danger) / 0.16);
}
</style>
