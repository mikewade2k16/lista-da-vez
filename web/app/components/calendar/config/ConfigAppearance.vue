<script setup lang="ts">
import { computed } from 'vue'
import {
  EVENT_TYPE_META,
  clientColorFor,
  tripletToHex,
  type CalendarClient,
  type CalendarEventType,
  type CalendarWhiteLabel,
  type WeekStart,
} from '~/utils/calendar'

// Secao Aparencia (SPEC-F3): inicio da semana + cor por cliente (com "sem cor") +
// cor por tipo + white-label (logo/titulo/cor primaria). Cores sao DADO: guardadas
// como `#rrggbb` (ou `none`) e aplicadas no calendario via triplet no ponto de uso.
const props = defineProps<{
  weekStartsOn: WeekStart
  clientColors: Record<string, string>
  typeColors: Record<string, string>
  whiteLabel: CalendarWhiteLabel
  clients: CalendarClient[]
}>()

const emit = defineEmits<{
  'update:weekStartsOn': [value: WeekStart]
  'update:clientColors': [value: Record<string, string>]
  'update:typeColors': [value: Record<string, string>]
  'update:whiteLabel': [value: CalendarWhiteLabel]
}>()

const eventTypes = Object.keys(EVENT_TYPE_META) as CalendarEventType[]

// Cor efetiva mostrada no seletor: override da config, ou a cor-semente do cliente.
function clientHex(client: CalendarClient, index: number): string {
  const custom = props.clientColors[client.id]
  if (custom && custom !== 'none') return custom
  return tripletToHex(clientColorFor(client.id, index))
}

function isClientNone(client: CalendarClient): boolean {
  return props.clientColors[client.id] === 'none'
}

// Remove uma chave do mapa sem `delete` dinamico (lint no-dynamic-delete).
function omitKey(map: Record<string, string>, key: string): Record<string, string> {
  const out: Record<string, string> = {}
  for (const [k, v] of Object.entries(map)) if (k !== key) out[k] = v
  return out
}

function setClientColor(id: string, value: string): void {
  emit('update:clientColors', { ...props.clientColors, [id]: value })
}

// "Sem cor" = grava `none`; desmarcar remove a chave (volta a paleta-semente).
function toggleClientNone(id: string): void {
  if (props.clientColors[id] === 'none')
    emit('update:clientColors', omitKey(props.clientColors, id))
  else emit('update:clientColors', { ...props.clientColors, [id]: 'none' })
}

function typeHex(type: CalendarEventType): string {
  return props.typeColors[type] || '#6366f1'
}

function hasTypeColor(type: CalendarEventType): boolean {
  return Boolean(props.typeColors[type])
}

function setTypeColor(type: CalendarEventType, value: string): void {
  emit('update:typeColors', { ...props.typeColors, [type]: value })
}

// Desmarcar remove o override (o chip volta a herdar a cor do cliente).
function toggleTypeColor(type: CalendarEventType): void {
  if (props.typeColors[type]) emit('update:typeColors', omitKey(props.typeColors, type))
  else emit('update:typeColors', { ...props.typeColors, [type]: '#6366f1' })
}

const wl = computed(() => props.whiteLabel)

function setWhiteLabel(patch: Partial<CalendarWhiteLabel>): void {
  emit('update:whiteLabel', { ...wl.value, ...patch })
}
</script>

<template>
  <section class="calendar-config__section">
    <h3 class="calendar-config__section-title">Aparência</h3>

    <div class="calendar-config__block">
      <span class="calendar-config__label">Início da semana</span>
      <div class="calendar-config__seg" role="radiogroup" aria-label="Início da semana">
        <button
          type="button"
          class="calendar-config__seg-btn"
          :class="{ 'is-active': weekStartsOn === 'sunday' }"
          @click="emit('update:weekStartsOn', 'sunday')"
        >
          Domingo
        </button>
        <button
          type="button"
          class="calendar-config__seg-btn"
          :class="{ 'is-active': weekStartsOn === 'monday' }"
          @click="emit('update:weekStartsOn', 'monday')"
        >
          Segunda
        </button>
      </div>
    </div>

    <div class="calendar-config__block">
      <span class="calendar-config__label">Cor por cliente</span>
      <p class="calendar-config__hint">Vazio = paleta automática. "Sem cor" usa cinza neutro.</p>
      <div v-if="clients.length" class="calendar-config__color-list">
        <div v-for="(client, index) in clients" :key="client.id" class="calendar-config__color-row">
          <input
            type="color"
            class="calendar-config__swatch"
            :value="clientHex(client, index)"
            :disabled="isClientNone(client)"
            :aria-label="`Cor de ${client.name}`"
            @input="setClientColor(client.id, ($event.target as HTMLInputElement).value)"
          />
          <span class="calendar-config__color-name">{{ client.name }}</span>
          <label class="calendar-config__none">
            <input
              type="checkbox"
              :checked="isClientNone(client)"
              @change="toggleClientNone(client.id)"
            />
            <span>Sem cor</span>
          </label>
        </div>
      </div>
      <p v-else class="calendar-config__empty">Nenhum cliente ativo.</p>
    </div>

    <div class="calendar-config__block">
      <span class="calendar-config__label">Cor por tipo de item</span>
      <p class="calendar-config__hint">Marque para sobrepor a cor do cliente naquele tipo.</p>
      <div class="calendar-config__color-list">
        <div v-for="type in eventTypes" :key="type" class="calendar-config__color-row">
          <input
            type="color"
            class="calendar-config__swatch"
            :value="typeHex(type)"
            :disabled="!hasTypeColor(type)"
            :aria-label="`Cor de ${EVENT_TYPE_META[type].label}`"
            @input="setTypeColor(type, ($event.target as HTMLInputElement).value)"
          />
          <span class="calendar-config__color-name">{{ EVENT_TYPE_META[type].label }}</span>
          <label class="calendar-config__none">
            <input type="checkbox" :checked="hasTypeColor(type)" @change="toggleTypeColor(type)" />
            <span>Usar</span>
          </label>
        </div>
      </div>
    </div>

    <div class="calendar-config__block">
      <span class="calendar-config__label">White-label</span>
      <div class="calendar-config__grid2">
        <label class="calendar-config__field">
          <span class="calendar-config__field-label">Título</span>
          <input
            class="calendar-config__input"
            :value="wl.title"
            placeholder="Nome exibido no calendário"
            @input="setWhiteLabel({ title: ($event.target as HTMLInputElement).value })"
          />
        </label>
        <label class="calendar-config__field">
          <span class="calendar-config__field-label">URL do logo</span>
          <input
            class="calendar-config__input"
            :value="wl.logoUrl"
            placeholder="https://..."
            @input="setWhiteLabel({ logoUrl: ($event.target as HTMLInputElement).value })"
          />
        </label>
        <label class="calendar-config__field">
          <span class="calendar-config__field-label">Cor primária</span>
          <input
            class="calendar-config__input"
            :value="wl.primaryColor"
            placeholder="#rrggbb"
            @input="setWhiteLabel({ primaryColor: ($event.target as HTMLInputElement).value })"
          />
        </label>
      </div>
    </div>
  </section>
</template>
