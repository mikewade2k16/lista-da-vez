<script setup lang="ts">
type PopoverAlign = 'start' | 'center' | 'end'
type PopoverSide = 'top' | 'right' | 'bottom' | 'left'

const props = withDefaults(
  defineProps<{
    open?: boolean
    title?: string
    description?: string
    align?: PopoverAlign
    side?: PopoverSide
    sideOffset?: number
    widthClass?: string
    disabled?: boolean
    showClose?: boolean
    panelClass?: string
    bodyClass?: string
    footerClass?: string
    autoFocus?: boolean
    focusSelector?: string
    submitOnEnter?: boolean
    closeOnSubmitShortcut?: boolean
    closeOnEscape?: boolean
  }>(),
  {
    open: false,
    title: '',
    description: '',
    align: 'end',
    side: 'bottom',
    sideOffset: 6,
    widthClass: 'w-[330px] max-w-[90vw]',
    disabled: false,
    showClose: true,
    panelClass: '',
    bodyClass: '',
    footerClass: '',
    autoFocus: true,
    focusSelector: '',
    submitOnEnter: true,
    closeOnSubmitShortcut: true,
    closeOnEscape: true,
  },
)

const emit = defineEmits<{
  'update:open': [value: boolean]
  'submit-shortcut': [event: KeyboardEvent]
  'cancel-shortcut': [event: KeyboardEvent]
  opened: []
  closed: []
}>()

const slots = useSlots()
const panelRef = ref<HTMLElement | null>(null)

const openModel = computed({
  get: () => Boolean(props.open),
  set: (value: boolean) => {
    if (props.disabled && value) {
      return
    }

    emit('update:open', value)
  },
})

const hasFooter = computed(() => Boolean(slots.footer))

function closeModal() {
  openModel.value = false
}

function toggleModal() {
  openModel.value = !openModel.value
}

function focusFirstFocusable() {
  if (!props.autoFocus) {
    return
  }

  const root = panelRef.value
  if (!root) {
    return
  }

  if (props.focusSelector) {
    const specific = root.querySelector<HTMLElement>(props.focusSelector)
    if (specific) {
      specific.focus()
      return
    }
  }

  const primary = root.querySelector<HTMLElement>(
    'input:not([type="hidden"]):not([disabled]), textarea:not([disabled]), select:not([disabled]), [contenteditable="true"]',
  )
  if (primary) {
    primary.focus()
    return
  }

  const fallback = root.querySelector<HTMLElement>(
    'button:not([disabled]), [tabindex]:not([tabindex="-1"]), a[href]',
  )
  fallback?.focus()
}

function onPanelKeydown(event: KeyboardEvent) {
  if (event.defaultPrevented) {
    return
  }

  if (event.key === 'Escape' && props.closeOnEscape) {
    emit('cancel-shortcut', event)
    if (!event.defaultPrevented) {
      closeModal()
    }
    return
  }

  if (event.key !== 'Enter' || !props.submitOnEnter) {
    return
  }

  if (event.shiftKey || event.altKey || event.ctrlKey || event.metaKey) {
    return
  }

  const target = event.target as HTMLElement | null
  if (!target) {
    return
  }

  if (target.closest('[data-omni-enter-ignore="true"]')) {
    return
  }

  const targetTag = target.tagName.toLowerCase()
  if (targetTag === 'textarea') {
    return
  }
  if (targetTag === 'button' || targetTag === 'a') {
    return
  }

  emit('submit-shortcut', event)
  if (props.closeOnSubmitShortcut && !event.defaultPrevented) {
    closeModal()
  }
}

watch(
  () => openModel.value,
  async (isOpen) => {
    if (isOpen) {
      emit('opened')
      await nextTick()
      focusFirstFocusable()
      return
    }

    emit('closed')
  },
)

onMounted(async () => {
  if (!openModel.value) {
    return
  }

  emit('opened')
  await nextTick()
  focusFirstFocusable()
})
</script>

<template>
  <UPopover
    v-model:open="openModel"
    :modal="false"
    :content="{ align: props.align, side: props.side, sideOffset: props.sideOffset }"
    :ui="{ content: ['omni-minimal-popover__content', props.widthClass].join(' ') }"
  >
    <slot name="trigger" :open="openModel" :close="closeModal" :toggle="toggleModal"></slot>

    <template #content>
      <div
        ref="panelRef"
        class="omni-minimal-popover__panel space-y-3 rounded-[var(--radius-md)] border border-[rgb(var(--border))] bg-[rgb(var(--surface))] p-3 shadow-[var(--shadow-sm)]"
        :class="props.panelClass"
        @click.stop
        @keydown="onPanelKeydown"
      >
        <div
          v-if="props.title || props.description || props.showClose || $slots.header"
          class="omni-minimal-popover__header flex items-start justify-between gap-2"
        >
          <div class="omni-minimal-popover__header-main min-w-0">
            <slot name="header">
              <p
                v-if="props.title"
                class="omni-minimal-popover__title text-sm font-semibold text-[rgb(var(--text))]"
              >
                {{ props.title }}
              </p>
              <p
                v-if="props.description"
                class="omni-minimal-popover__description text-xs text-[rgb(var(--muted))]"
              >
                {{ props.description }}
              </p>
            </slot>
          </div>

          <UButton
            v-if="props.showClose"
            icon="i-lucide-x"
            color="neutral"
            variant="ghost"
            size="xs"
            class="omni-minimal-popover__close"
            @click="closeModal"
          />
        </div>

        <div class="omni-minimal-popover__body" :class="props.bodyClass">
          <slot></slot>
        </div>

        <div
          v-if="hasFooter"
          class="omni-minimal-popover__footer flex items-center justify-end gap-2"
          :class="props.footerClass"
        >
          <slot name="footer" :close="closeModal"></slot>
        </div>
      </div>
    </template>
  </UPopover>
</template>
