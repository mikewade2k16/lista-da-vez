<script setup lang="ts">
import { computed } from "vue"

const props = defineProps<{
  rules: Array<Record<string, any>>
  pending?: boolean
}>()

const emit = defineEmits<{
  "edit": [rule: Record<string, any>]
  "delete": [ruleId: string]
  "toggle": [ruleId: string, isActive: boolean]
  "apply-now": [ruleId: string]
}>()

const triggerTypeLabels: Record<string, string> = {
  long_open_service: "Atendimento longo",
  long_queue_wait: "Fila longa",
  long_pause: "Pausa longa",
  idle_store: "Loja parada",
  outside_business_hours: "Fora do horário"
}

const displayKindLabels: Record<string, string> = {
  card_badge: "Badge",
  banner: "Banner",
  toast: "Toast",
  corner_popup: "Popup",
  center_modal: "Modal",
  fullscreen: "Fullscreen"
}

const getTriggerLabel = (type: string) => triggerTypeLabels[type] || type
const getDisplayLabel = (kind: string) => displayKindLabels[kind] || kind
</script>

<template>
  <div class="alert-rule-list">
    <div v-if="rules.length === 0" class="empty-state">
      <div class="empty-state__icon">🎯</div>
      <p class="empty-state__title">Nenhuma regra de alerta configurada</p>
      <p class="empty-state__text">Comece criando uma nova regra para personalizar os alertas operacionais</p>
      <p class="empty-state__hint">Escolha um gatilho (atendimento longo, fila, pausa), tipo de display (banner, popup, modal, tela cheia) e configure templates com cores dinâmicas</p>
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
              @click="emit('toggle', rule.id, !rule.isActive)"
              :disabled="pending"
              :title="rule.isActive ? 'Desativar' : 'Ativar'"
            >
              {{ rule.isActive ? "✓ Ativa" : "✕ Inativa" }}
            </button>
          </td>
          <td class="updated-at">
            {{ new Date(rule.updatedAt).toLocaleDateString() }}
          </td>
          <td class="actions-cell">
            <button
              class="btn-icon edit"
              @click="emit('edit', rule)"
              :disabled="pending"
              title="Editar"
            >
              ✎
            </button>
            <button
              class="btn-icon apply"
              @click="emit('apply-now', rule.id)"
              :disabled="pending"
              title="Aplicar agora"
            >
              ⚡
            </button>
            <button
              class="btn-icon delete"
              @click="emit('delete', rule.id)"
              :disabled="pending"
              title="Deletar"
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
  color: #6b7280;
  background: linear-gradient(135deg, #f9fafb 0%, #f3f4f6 100%);
  border-radius: 12px;
  border: 1px solid #e5e7eb;
}

.empty-state__icon {
  font-size: 3rem;
  margin-bottom: 1rem;
}

.empty-state__title {
  margin: 0 0 0.5rem 0;
  font-size: 1.1rem;
  font-weight: 600;
  color: #374151;
}

.empty-state__text {
  margin: 0 0 1rem 0;
  font-size: 0.95rem;
  color: #6b7280;
}

.empty-state__hint {
  margin: 0;
  font-size: 0.85rem;
  color: #9ca3af;
  font-style: italic;
}

.rules-table {
  width: 100%;
  border-collapse: collapse;
  background: rgba(15, 23, 42, 0.4);
  border-radius: 8px;
  overflow: hidden;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.3);
  border: 1px solid rgba(148, 163, 184, 0.2);
}

.rules-table thead {
  background: rgba(30, 41, 59, 0.6);
  border-bottom: 1px solid rgba(148, 163, 184, 0.2);
}

.rules-table th {
  padding: 1rem;
  text-align: left;
  font-weight: 600;
  color: #cbd5e1;
  font-size: 0.9rem;
}

.rules-table td {
  padding: 1rem;
  border-bottom: 1px solid rgba(148, 163, 184, 0.15);
  color: #e2e8f0;
}

.rules-table tbody tr:last-child td {
  border-bottom: none;
}

.rules-table tbody tr.inactive {
  opacity: 0.5;
  background: rgba(100, 116, 139, 0.1);
}

.rules-table tbody tr:hover {
  background: rgba(59, 130, 246, 0.05);
}

.name-cell {
  min-width: 200px;
}

.rule-name {
  font-weight: 500;
  color: #e2e8f0;
}

.rule-desc {
  font-size: 0.85rem;
  color: #94a3b8;
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
  background: #dcfce7;
  color: #166534;
}

.toggle-btn.inactive {
  background: #fee2e2;
  color: #991b1b;
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
  color: #94a3b8;
}

.actions-cell {
  display: flex;
  gap: 0.5rem;
}

.btn-icon {
  background: none;
  border: 1px solid rgba(148, 163, 184, 0.28);
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
  border-color: rgba(148, 163, 184, 0.55);
  background: rgba(30, 41, 59, 0.75);
}

.btn-icon:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-icon.edit {
  border-color: #3b82f6;
  color: #3b82f6;
}

.btn-icon.edit:hover:not(:disabled) {
  background: rgba(59, 130, 246, 0.16);
}

.btn-icon.apply {
  border-color: #f59e0b;
  color: #f59e0b;
}

.btn-icon.apply:hover:not(:disabled) {
  background: rgba(245, 158, 11, 0.16);
}

.btn-icon.delete {
  border-color: #ef4444;
  color: #ef4444;
}

.btn-icon.delete:hover:not(:disabled) {
  background: rgba(239, 68, 68, 0.16);
}
</style>
