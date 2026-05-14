import type { InjectionKey } from 'vue'
import { useCoreLoading } from '../../core/composables/useCoreLoading'
import { useAuthStore } from '~/stores/auth'
import { useSessionSimulationStore } from '../stores/session-simulation'
import { useCan } from './useCan'
import { useTasksWorkspace } from './useTasksWorkspace'
import { useTimeTracking } from './useTimeTracking'
import type { OmniFocusCell, OmniSelectOption, OmniTableCellUpdate, OmniTableColumn } from '../types/omni/collection'
import type { OrchestratorView, TaskBoardColumn, TaskItem, TaskPriority, TaskProjectItem } from '../types/tasks'

export const TASKS_PAGE_CONTEXT_KEY: InjectionKey<TasksPageContext> = Symbol('tasksPageContext')

export function useTasksPageContext() {
	const auth = useAuthStore()
  const sessionSimulation = useSessionSimulationStore()
  const tasksWorkspace = useTasksWorkspace()
  const pageLoading = useCoreLoading()
  const canManageBoards = useCan('tasks.boards.manage')
  const canClientView = useCan('tasks.client_view')
  const { startTracking, pauseTracking, stopTracking, isTracking, isRunning, getElapsedMs, formatElapsed } = useTimeTracking()

  const ORDER_STEP = 10
  const PRIORITY_OPTIONS: OmniSelectOption[] = [
    { label: 'Baixa', value: 'baixa' },
    { label: 'Media', value: 'media' },
    { label: 'Alta', value: 'alta' }
  ]
  const COLUMN_COLOR_OPTIONS: OmniSelectOption[] = [
    { label: 'Indigo', value: 'indigo' },
    { label: 'Slate', value: 'slate' },
    { label: 'Blue', value: 'blue' },
    { label: 'Amber', value: 'amber' },
    { label: 'Emerald', value: 'emerald' },
    { label: 'Violet', value: 'violet' },
    { label: 'Rose', value: 'rose' }
  ]
  const DEFAULT_FILTERS = { search: '', responsible: '', clientId: '', type: '', hideArchived: true }
  const BOARD_GROUP_OPTIONS: OmniSelectOption[] = [
    { label: 'Status', value: 'status' },
    { label: 'Responsavel', value: 'responsible' },
    { label: 'Cliente', value: 'clientId' },
    { label: 'Tipo', value: 'type' },
    { label: 'Prioridade', value: 'priority' }
  ]
  const FIELD_DEFS = [
    { key: 'title', label: 'Titulo' },
    { key: 'description', label: 'Descricao' },
    { key: 'status', label: 'Status' },
    { key: 'responsible', label: 'Responsavel' },
    { key: 'involved', label: 'Envolvidos' },
    { key: 'clientId', label: 'Cliente' },
    { key: 'type', label: 'Tipo' },
    { key: 'priority', label: 'Prioridade' },
    { key: 'dueDate', label: 'Entrega' },
    { key: 'createdAt', label: 'Criado em' },
    { key: 'archived', label: 'Arquivada' }
  ] as const

  const filterSwitchDefs = [
    { key: 'search', label: 'Busca' },
    { key: 'responsible', label: 'Responsavel' },
    { key: 'client', label: 'Cliente' },
    { key: 'type', label: 'Tipo' },
    { key: 'hideArchived', label: 'Ocultar arquivadas' }
  ] as const

  const cardFieldSwitchDefs = [
    { key: 'status', label: 'Status' },
    { key: 'responsible', label: 'Responsavel' },
    { key: 'involved', label: 'Envolvidos' },
    { key: 'client', label: 'Cliente' },
    { key: 'type', label: 'Tipo' },
    { key: 'dueDate', label: 'Entrega' },
    { key: 'priority', label: 'Prioridade' },
    { key: 'createdAt', label: 'Criado em' }
  ] as const

  const modalModeOptions = [
    { label: 'Modo lado a lado', value: 'side', icon: 'i-lucide-panel-right' },
    { label: 'Modo centralizado', value: 'center', icon: 'i-lucide-square' },
    { label: 'Pagina inteira', value: 'fullscreen', icon: 'i-lucide-expand' }
  ] as const

  const viewMode = ref<'board' | 'table'>('board')
  const pageBootstrapping = ref(true)
  const draggingTaskId = ref('')
  const draggingColumnId = ref('')
  const filters = reactive({ ...DEFAULT_FILTERS })
  const tableSelectedRows = ref<Array<string | number>>([])
  const tableFocusCell = ref<OmniFocusCell | null>(null)
  const activeInlineTaskId = ref('')
  const creatingCards = reactive<Record<string, {
    title: string
    status: string
    responsible: string
    involved: string[]
    clientId: number
    clientName: string
    type: string
    priority: TaskPriority
    dueDate: string
    firstEnterDone: boolean
  }>>({})
  const draftAddedFields = reactive<Record<string, string[]>>({})
  const draftMenuOpen = reactive<Record<string, boolean>>({})
  const draftFieldOpen = reactive<Record<string, Record<string, boolean>>>({})
  function setDraftFieldOpen(columnId: string, fieldKey: string, value: boolean) {
    if (!draftFieldOpen[columnId]) draftFieldOpen[columnId] = {}
    draftFieldOpen[columnId][fieldKey] = value
  }
  const dragKind = ref<'task' | 'column' | ''>('')
  const dropTarget = reactive({ columnId: '', index: -1 })

  const projectSettingsOpen = ref(false)
  const columnSettingsOpen = ref(false)
  const taskEditorOpen = ref(false)
  const taskEditorMode = ref<'side' | 'center' | 'fullscreen'>('side')
  const taskEditorWidth = ref(720)
  const taskEditorResizing = ref(false)
  const settingsSaving = ref(false)
  const taskSaving = ref(false)

  const projectSettingsDraft = reactive<{
    name: string
    description: string
    icon: string
    statuses: string[]
    columns: TaskBoardColumn[]
    responsibles: string[]
    types: string[]
    boardGroupBy: string
    boardVisibleFieldKeys: string[]
    tableVisibleFieldKeys: string[]
    modalVisibleFieldKeys: string[]
    showAggregation: boolean
    defaults: TaskProjectItem['defaults']
    filters: TaskProjectItem['filters']
    cardFields: TaskProjectItem['cardFields']
  }>({
    name: '', description: '', icon: '', statuses: [], columns: [], responsibles: [], types: [],
    boardGroupBy: 'status', boardVisibleFieldKeys: [], tableVisibleFieldKeys: [], modalVisibleFieldKeys: [], showAggregation: true,
    defaults: { responsibleFromCreator: true, clientFromSession: true, showCreatedAt: false },
    filters: { search: true, responsible: true, client: true, type: true, hideArchived: true },
    cardFields: { status: true, responsible: true, involved: true, client: true, type: true, dueDate: true, priority: true, createdAt: false }
  })

  const columnDraft = reactive({ id: '', label: '', color: 'indigo' })

  const taskDraft = reactive({
    id: '', title: '', description: '', contentHtml: '', status: '', responsible: '', involved: [] as string[], clientId: 0, clientName: '', type: '',
    priority: 'media' as TaskPriority, dueDate: '', archived: false, createdBy: '', createdAt: ''
  })

  function normalizeText(value: unknown, max = 240) { return String(value ?? '').replace(/\s+/g, ' ').trim().slice(0, max) }
  function normalizeKey(value: unknown) {
    return normalizeText(value, 120).normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase().replace(/[^a-z0-9]+/g, '_').replace(/^_+|_+$/g, '')
  }
  function toNumberId(value: unknown) { const n = Number.parseInt(String(value ?? '').trim(), 10); return Number.isFinite(n) && n > 0 ? n : 0 }
  function dateLabel(value: unknown) {
    const iso = normalizeText(value, 24)
    if (!iso) return '-'
    const d = new Date(iso)
    return Number.isNaN(d.getTime()) ? iso : d.toLocaleDateString('pt-BR')
  }
  function dateLabelLong(value: unknown) {
    const iso = normalizeText(value, 24)
    if (!iso) return ''
    const d = new Date(iso.length === 10 ? `${iso}T00:00:00` : iso)
    if (Number.isNaN(d.getTime())) return iso
    const dd = String(d.getDate()).padStart(2, '0')
    const mm = String(d.getMonth() + 1).padStart(2, '0')
    const dateStr = `${dd}/${mm}/${d.getFullYear()}`
    if (iso.length >= 16) {
      const hh = String(d.getHours()).padStart(2, '0')
      const min = String(d.getMinutes()).padStart(2, '0')
      return `${dateStr} ${hh}:${min}`
    }
    return dateStr
  }
  function priorityLabel(value: TaskPriority) { return value === 'alta' ? 'Alta' : value === 'baixa' ? 'Baixa' : 'Media' }
  function priorityColor(value: TaskPriority): 'error' | 'warning' | 'neutral' { return value === 'alta' ? 'error' : value === 'media' ? 'warning' : 'neutral' }
  function toPriority(value: unknown): TaskPriority {
    const key = normalizeKey(value)
    return key === 'alta' || key === 'baixa' || key === 'media' ? key : 'media'
  }
  function columnColorClass(color: string) { return `tasks-page__board-column--${normalizeKey(color) || 'indigo'}` }
  function clientLabel(clientId: number) { return sessionSimulation.clientOptions.find(c => c.value === clientId)?.label || `Cliente #${clientId}` }
  function taskSort(a: TaskItem, b: TaskItem) { const d = Number(a.order || 0) - Number(b.order || 0); return d !== 0 ? d : a.createdAt.localeCompare(b.createdAt) }
  function renumber(projectId: string, status: string) {
    tasksWorkspace.tasks.value
      .filter(t => t.projectId === projectId && normalizeKey(t.status) === normalizeKey(status))
      .sort(taskSort)
      .forEach((t, i) => { t.order = (i + 1) * ORDER_STEP; t.updatedAt = new Date().toISOString() })
  }
  async function moveTask(taskId: string, targetStatus: string, targetIndex?: number) {
    const project = activeProject.value
    if (!project) return
    await tasksWorkspace.moveTaskToStatus(taskId, targetStatus, targetIndex)
  }

  function patchForGroupColumn(column: { groupFieldKey?: string, value?: string, status: string }) {
    const fieldKey = column.groupFieldKey || 'status'
    const value = normalizeText(column.value ?? column.status, 140)
    if (fieldKey === 'responsible') return { responsible: value }
    if (fieldKey === 'type') return { type: value }
    if (fieldKey === 'clientId') {
      const clientId = toNumberId(value)
      return clientId ? { clientId, clientName: clientLabel(clientId) } : {}
    }
    if (fieldKey === 'priority') return { priority: toPriority(value) }
    return { status: column.status }
  }

  async function moveTaskToGroupColumn(taskId: string, column: { groupFieldKey?: string, value?: string, status: string }, targetIndex?: number) {
    if ((column.groupFieldKey || 'status') === 'status') {
      await moveTask(taskId, column.status, targetIndex)
      return
    }
    const patch = patchForGroupColumn(column)
    await tasksWorkspace.updateTask(taskId, patch)
  }

  const viewerUserType = computed<'admin' | 'client'>(() => canClientView.value && !canManageBoards.value ? 'client' : 'admin')
  const activeProject = computed(() => tasksWorkspace.projects.value.find(p => p.id === tasksWorkspace.activeProjectId.value) ?? null)
  const projectOptions = computed(() => tasksWorkspace.projects.value.map(p => ({ label: p.name, value: p.id })))
  const clientOptions = computed(() => sessionSimulation.clientOptions.map(c => ({ label: c.label, value: c.value })))
  const currentUserName = computed(() => normalizeText(auth.user?.name || auth.user?.fullName || auth.user?.email, 120) || (viewerUserType.value === 'client' ? sessionSimulation.activeClientLabel : 'Usuario'))
  const taskEditorCssVars = computed(() => ({ '--tasks-editor-width': `${taskEditorWidth.value}px` }))

  watch(taskEditorWidth, (width) => {
    if (import.meta.client) document.documentElement.style.setProperty('--tasks-editor-width', `${width}px`)
  }, { immediate: true })

  function uniqueValues(list: string[]) {
    const seen = new Set<string>()
    return list.filter((v) => { const k = normalizeKey(v); if (!k || seen.has(k)) return false; seen.add(k); return true })
  }

  function defaultView(type: 'board' | 'table'): OrchestratorView {
    return {
      id: type === 'board' ? 'view-board' : 'view-table',
      name: type === 'board' ? 'Board' : 'Tabela',
      type,
      groupByFieldKey: 'status',
      visibleFieldKeys: type === 'board'
        ? ['responsible', 'involved', 'clientId', 'type', 'priority', 'dueDate']
        : ['title', 'status', 'responsible', 'involved', 'clientId', 'type', 'priority', 'dueDate', 'archived'],
      modalVisibleFieldKeys: ['description', 'status', 'responsible', 'involved', 'clientId', 'type', 'priority', 'dueDate', 'archived'],
      hiddenColumnIds: [],
      showAggregation: true,
      sortBy: 'order',
      sortDirection: 'asc'
    }
  }

  function projectView(project: TaskProjectItem | null, type: 'board' | 'table') {
    return project?.views.find(view => view.type === type) || defaultView(type)
  }

  function updateProjectView(type: 'board' | 'table', patch: Partial<OrchestratorView>) {
    const project = activeProject.value
    if (!project) return
    const currentView = projectView(project, type)
    const views = [
      ...project.views.filter(view => view.id !== currentView.id),
      { ...currentView, ...patch }
    ].sort((a, b) => a.type === b.type ? 0 : a.type === 'board' ? -1 : 1)
    tasksWorkspace.saveProjectSettings(project.id, { views, activeViewId: currentView.id })
    hydrateProjectDraft(activeProject.value)
  }

  function fieldLabel(key: string) {
    return FIELD_DEFS.find(field => field.key === key)?.label || key
  }

  function fieldSwitchValue(list: string[], key: string) { return list.includes(key) }

  function setFieldSwitch(list: string[], key: string, value: boolean) {
    const exists = list.includes(key)
    if (value && !exists) list.push(key)
    if (!value && exists) list.splice(list.indexOf(key), 1)
  }

  const boardSchemaColumns = computed(() => {
    const project = activeProject.value
    if (!project) return []
    const columns = Array.isArray(project.columns) && project.columns.length > 0
      ? project.columns
      : project.statuses.map((status, index) => ({ id: `column-${normalizeKey(status) || index}`, label: status, color: 'indigo', order: (index + 1) * ORDER_STEP }))
    return [...columns].sort((a, b) => Number(a.order || 0) - Number(b.order || 0))
  })
  const boardView = computed(() => projectView(activeProject.value, 'board'))
  const tableView = computed(() => projectView(activeProject.value, 'table'))
  const boardGroupBy = computed(() => normalizeText(boardView.value.groupByFieldKey, 80) || 'status')
  const statuses = computed(() => uniqueValues(boardSchemaColumns.value.map(column => normalizeText(column.label, 120)).filter(Boolean)))
  const statusOptions = computed<OmniSelectOption[]>(() => statuses.value.map(v => ({ label: v, value: v })))
  const responsibleOptions = computed<OmniSelectOption[]>(() => {
    const project = activeProject.value
    if (!project) return []
    const values = uniqueValues([...project.responsibles, ...tasksWorkspace.tasks.value.filter(t => t.projectId === project.id).map(t => t.responsible)])
    return values.map(v => ({ label: v, value: v }))
  })
  const involvedOptions = computed<OmniSelectOption[]>(() => {
    const project = activeProject.value
    if (!project) return []
    const values = uniqueValues([
      currentUserName.value,
      ...project.responsibles,
      ...tasksWorkspace.tasks.value
        .filter(t => t.projectId === project.id)
        .flatMap(t => [t.responsible, ...(Array.isArray(t.involved) ? t.involved : [])])
    ])
    return values.map(v => ({ label: v, value: v }))
  })
  const typeOptions = computed<OmniSelectOption[]>(() => {
    const project = activeProject.value
    if (!project) return []
    const values = uniqueValues([...project.types, ...tasksWorkspace.tasks.value.filter(t => t.projectId === project.id).map(t => t.type)])
    return values.map(v => ({ label: v, value: v }))
  })
  function initialsFor(value: unknown) {
    const s = String(value ?? '').trim()
    if (!s) return '?'
    const parts = s.split(/\s+/).filter(Boolean).slice(0, 2)
    const initials = parts.map(p => p[0]?.toUpperCase() || '').join('')
    return initials || s[0]!.toUpperCase()
  }
  const responsibleOptionsAvatar = computed<OmniSelectOption[]>(() => responsibleOptions.value.map((o: OmniSelectOption) => ({ ...o, avatar: { text: initialsFor(o.label) } })))
  const involvedOptionsAvatar = computed<OmniSelectOption[]>(() => involvedOptions.value.map((o: OmniSelectOption) => ({ ...o, avatar: { text: initialsFor(o.label) } })))
  const clientOptionsAvatar = computed<OmniSelectOption[]>(() => clientOptions.value.map((o: OmniSelectOption) => ({ ...o, avatar: { text: initialsFor(o.label) } })))
  const peopleMentionLabels = computed(() => involvedOptions.value.map(option => String(option.label || option.value)))
  const clientMentionLabels = computed(() => clientOptions.value.map(option => String(option.label || option.value)))
  const taskMentionLabels = computed(() => projectTasks.value.map(task => task.title))

  const projectModel = computed({
    get: () => activeProject.value?.id ?? '',
    set: (value: string | number | null) => { const id = normalizeText(value, 120); if (id) tasksWorkspace.setActiveProject(id) }
  })

  const projectTasks = computed(() => {
    const project = activeProject.value
    if (!project) return []
    return tasksWorkspace.tasks.value.filter((t) => t.projectId === project.id)
  })

  const filteredTasks = computed(() => {
    const project = activeProject.value
    if (!project) return []
    const search = normalizeText(filters.search, 180).toLowerCase()
    const fResponsible = normalizeText(filters.responsible, 120)
    const fType = normalizeText(filters.type, 120)
    const fClient = toNumberId(filters.clientId)
    return projectTasks.value
      .filter((t) => {
        if (project.filters.hideArchived && filters.hideArchived && t.archived) return false
        if (project.filters.search && search) {
          const hay = [t.title, t.description, t.responsible, t.clientName, t.type, t.status].join(' ').toLowerCase()
          if (!hay.includes(search)) return false
        }
        if (project.filters.responsible && fResponsible && normalizeKey(t.responsible) !== normalizeKey(fResponsible)) return false
        if (project.filters.type && fType && normalizeKey(t.type) !== normalizeKey(fType)) return false
        if (viewerUserType.value === 'admin' && project.filters.client && fClient > 0 && t.clientId !== fClient) return false
        return true
      })
      .sort(taskSort)
  })

  function valueForGroup(task: TaskItem, fieldKey: string) {
    if (fieldKey === 'clientId') return String(task.clientId || '')
    if (fieldKey === 'priority') return task.priority
    return normalizeText((task as Record<string, unknown>)[fieldKey], 140)
  }

  function labelForGroup(fieldKey: string, value: string) {
    if (!value) return `Sem ${fieldLabel(fieldKey).toLowerCase()}`
    if (fieldKey === 'clientId') return clientLabel(toNumberId(value))
    if (fieldKey === 'priority') return priorityLabel(toPriority(value))
    return value
  }

  function groupOptionsFor(fieldKey: string) {
    if (fieldKey === 'status') {
      return boardSchemaColumns.value.map(column => ({
        id: column.id, label: column.label, value: column.label, color: column.color,
        order: column.order, editable: true
      }))
    }
    if (fieldKey === 'responsible') {
      return responsibleOptions.value.map((option, index) => ({ id: `responsible-${normalizeKey(option.value) || index}`, label: option.label, value: String(option.value), color: 'blue', order: (index + 1) * ORDER_STEP, editable: false }))
    }
    if (fieldKey === 'clientId') {
      return clientOptions.value.map((option, index) => ({ id: `client-${option.value}`, label: option.label, value: String(option.value), color: 'emerald', order: (index + 1) * ORDER_STEP, editable: false }))
    }
    if (fieldKey === 'type') {
      return typeOptions.value.map((option, index) => ({ id: `type-${normalizeKey(option.value) || index}`, label: option.label, value: String(option.value), color: 'violet', order: (index + 1) * ORDER_STEP, editable: false }))
    }
    if (fieldKey === 'priority') {
      return PRIORITY_OPTIONS.map((option, index) => ({ id: `priority-${option.value}`, label: option.label, value: String(option.value), color: option.value === 'alta' ? 'rose' : option.value === 'media' ? 'amber' : 'slate', order: (index + 1) * ORDER_STEP, editable: false }))
    }
    return []
  }

  const boardColumns = computed(() => {
    const fieldKey = boardGroupBy.value
    const hidden = new Set(boardView.value.hiddenColumnIds || [])
    const configured = groupOptionsFor(fieldKey)
    const fromTasks = filteredTasks.value.map(task => valueForGroup(task, fieldKey))
    const allValues = uniqueValues([...configured.map(group => group.value), ...fromTasks])
    const configMap = new Map(configured.map(group => [normalizeKey(group.value), group] as const))
    const groups = allValues.length > 0 ? allValues : ['']
    return groups
      .map((value, index) => {
        const config = configMap.get(normalizeKey(value))
        const id = config?.id || `${fieldKey}-${normalizeKey(value) || 'empty'}`
        return {
          id,
          label: config?.label || labelForGroup(fieldKey, value),
          status: config?.label || labelForGroup(fieldKey, value),
          value,
          color: config?.color || COLUMN_COLOR_OPTIONS[index % COLUMN_COLOR_OPTIONS.length]?.value as string || 'indigo',
          order: config?.order || (index + 1) * ORDER_STEP,
          editable: Boolean(config?.editable),
          groupFieldKey: fieldKey,
          tasks: filteredTasks.value.filter(t => normalizeKey(valueForGroup(t, fieldKey)) === normalizeKey(value)).sort(taskSort)
        }
      })
      .filter(column => !hidden.has(column.id))
      .sort((a, b) => Number(a.order || 0) - Number(b.order || 0))
  })
  const tableRows = computed(() => filteredTasks.value.map(t => ({ ...t, clientId: t.clientId })))
  const projectCount = computed(() => tasksWorkspace.projects.value.length)

  const tableColumns = computed<OmniTableColumn[]>(() => {
    const columns: OmniTableColumn[] = [
      { key: 'title', label: 'Titulo', type: 'text', editable: true, minWidth: 220, focusOnCreate: true },
      { key: 'description', label: 'Descricao', type: 'text', editable: true, minWidth: 260 },
      { key: 'status', label: 'Status', type: 'select', editable: true, minWidth: 180, options: statusOptions.value },
      { key: 'responsible', label: 'Responsavel', type: 'select', editable: true, minWidth: 170, options: responsibleOptions.value, creatable: true },
      { key: 'involved', label: 'Envolvidos', type: 'text', editable: true, minWidth: 220, formatter: v => Array.isArray(v) ? v.join(', ') : normalizeText(v, 300) },
      { key: 'clientId', label: 'Cliente', type: 'select', editable: true, minWidth: 170, adminOnly: true, options: clientOptions.value },
      { key: 'type', label: 'Tipo', type: 'select', editable: true, minWidth: 150, options: typeOptions.value, creatable: true },
      { key: 'priority', label: 'Prioridade', type: 'select', editable: true, minWidth: 130, options: PRIORITY_OPTIONS },
      { key: 'dueDate', label: 'Entrega', type: 'text', editable: true, minWidth: 130, formatter: v => dateLabel(v) },
      { key: 'archived', label: 'Arquivada', type: 'switch', editable: true, minWidth: 120, align: 'center', switchOnValue: true, switchOffValue: false }
    ]
    const visible = new Set(tableView.value.visibleFieldKeys.length > 0 ? tableView.value.visibleFieldKeys : defaultView('table').visibleFieldKeys)
    return [
      ...columns.filter(column => visible.has(column.key)),
      {
        key: 'actions', label: 'Acoes', minWidth: 120, align: 'right', actions: [
          { id: 'edit', icon: 'i-lucide-pencil', label: 'Editar', color: 'neutral', variant: 'ghost' },
          { id: 'archive', icon: 'i-lucide-archive', label: 'Arquivar', color: 'warning', variant: 'ghost' },
          { id: 'delete', icon: 'i-lucide-trash-2', label: 'Excluir', color: 'error', variant: 'ghost' }
        ]
      }
    ]
  })

  function hydrateProjectDraft(project: TaskProjectItem | null) {
    if (!project) {
      projectSettingsDraft.name = ''
      projectSettingsDraft.description = ''
      projectSettingsDraft.icon = ''
      projectSettingsDraft.statuses = []
      projectSettingsDraft.columns = []
      projectSettingsDraft.responsibles = []
      projectSettingsDraft.types = []
      projectSettingsDraft.boardGroupBy = 'status'
      projectSettingsDraft.boardVisibleFieldKeys = []
      projectSettingsDraft.tableVisibleFieldKeys = []
      projectSettingsDraft.modalVisibleFieldKeys = []
      projectSettingsDraft.showAggregation = true
      projectSettingsDraft.defaults = { responsibleFromCreator: true, clientFromSession: true, showCreatedAt: false }
      return
    }
    const board = projectView(project, 'board')
    const table = projectView(project, 'table')
    projectSettingsDraft.name = project.name
    projectSettingsDraft.description = project.description
    projectSettingsDraft.icon = project.icon
    projectSettingsDraft.statuses = [...project.statuses]
    projectSettingsDraft.columns = [...boardSchemaColumns.value]
    projectSettingsDraft.responsibles = [...project.responsibles]
    projectSettingsDraft.types = [...project.types]
    projectSettingsDraft.boardGroupBy = board.groupByFieldKey || 'status'
    projectSettingsDraft.boardVisibleFieldKeys = [...(board.visibleFieldKeys.length > 0 ? board.visibleFieldKeys : defaultView('board').visibleFieldKeys)]
    projectSettingsDraft.tableVisibleFieldKeys = [...(table.visibleFieldKeys.length > 0 ? table.visibleFieldKeys : defaultView('table').visibleFieldKeys)]
    projectSettingsDraft.modalVisibleFieldKeys = [...((board.modalVisibleFieldKeys || []).length > 0 ? board.modalVisibleFieldKeys : defaultView('board').modalVisibleFieldKeys)]
    projectSettingsDraft.showAggregation = board.showAggregation !== false
    projectSettingsDraft.filters = { ...project.filters }
    projectSettingsDraft.cardFields = { ...project.cardFields }
    projectSettingsDraft.defaults = { ...project.defaults }
  }

  function resetTaskDraft() {
    const project = activeProject.value
    const responsible = project?.defaults.responsibleFromCreator ? currentUserName.value : ''
    const clientId = viewerUserType.value === 'client'
      ? sessionSimulation.clientId
      : (project?.defaults.clientFromSession ? sessionSimulation.clientId : (toNumberId(filters.clientId) || sessionSimulation.clientId))
    taskDraft.id = ''
    taskDraft.title = ''
    taskDraft.description = ''
    taskDraft.contentHtml = ''
    taskDraft.status = statuses.value[0] || ''
    taskDraft.responsible = responsible
    taskDraft.involved = responsible ? [responsible] : []
    taskDraft.clientId = clientId
    taskDraft.clientName = clientLabel(taskDraft.clientId)
    taskDraft.type = ''
    taskDraft.priority = 'media'
    taskDraft.dueDate = ''
    taskDraft.archived = false
    taskDraft.createdBy = currentUserName.value
    taskDraft.createdAt = ''
  }

  function openTaskEditor(task?: TaskItem | null) {
    if (!task) { resetTaskDraft(); taskEditorOpen.value = true; return }
    taskDraft.id = task.id
    taskDraft.title = task.title
    taskDraft.description = task.description
    taskDraft.contentHtml = task.contentHtml
    taskDraft.status = task.status
    taskDraft.responsible = task.responsible
    taskDraft.involved = [...task.involved]
    taskDraft.clientId = task.clientId
    taskDraft.clientName = task.clientName
    taskDraft.type = task.type
    taskDraft.priority = task.priority
    taskDraft.dueDate = task.dueDate
    taskDraft.archived = task.archived
    taskDraft.createdBy = task.createdBy
    taskDraft.createdAt = task.createdAt
    taskEditorOpen.value = true
  }

  function closeTaskEditor() { taskEditorOpen.value = false; resetTaskDraft() }

  async function upsertProjectListsFromTask() {
    const project = activeProject.value
    if (!project) return
    const responsible = normalizeText(taskDraft.responsible, 120)
    const type = normalizeText(taskDraft.type, 120)
    const nextResponsibles = [...project.responsibles]
    const nextTypes = [...project.types]
    if (responsible && !nextResponsibles.some(v => normalizeKey(v) === normalizeKey(responsible))) nextResponsibles.push(responsible)
    taskDraft.involved.forEach((person) => {
      const label = normalizeText(person, 120)
      if (label && !nextResponsibles.some(v => normalizeKey(v) === normalizeKey(label))) nextResponsibles.push(label)
    })
    if (type && !nextTypes.some(v => normalizeKey(v) === normalizeKey(type))) nextTypes.push(type)
    await tasksWorkspace.saveProjectSettings(project.id, { responsibles: nextResponsibles, types: nextTypes })
  }

  async function saveTask() {
    const project = activeProject.value
    if (!project) return
    const title = normalizeText(taskDraft.title, 220)
    if (!title) return
    taskSaving.value = true
    const clientId = viewerUserType.value === 'client' ? sessionSimulation.clientId : Math.max(1, toNumberId(taskDraft.clientId) || sessionSimulation.clientId)
    const payload = {
      title,
      description: normalizeText(taskDraft.description, 5000),
      contentHtml: taskDraft.contentHtml,
      status: normalizeText(taskDraft.status, 120) || project.statuses[0] || 'Raw',
      responsible: normalizeText(taskDraft.responsible, 120),
      involved: [...taskDraft.involved],
      clientId,
      clientName: clientLabel(clientId),
      type: normalizeText(taskDraft.type, 120),
      priority: taskDraft.priority,
      dueDate: normalizeText(taskDraft.dueDate, 30),
      archived: Boolean(taskDraft.archived),
      createdBy: normalizeText(taskDraft.createdBy, 120) || currentUserName.value
    }
    try {
      let savedTaskId = normalizeText(taskDraft.id, 80)
      if (!taskDraft.id) {
        const created = await tasksWorkspace.createTask({ projectId: project.id, ...payload })
        if (!created) return
        taskDraft.id = created.id
        savedTaskId = created.id
      } else {
        const updated = await tasksWorkspace.updateTask(taskDraft.id, payload)
        if (!updated) return
        savedTaskId = updated.id
      }
      await upsertProjectListsFromTask()
      if (savedTaskId) taskDraft.id = savedTaskId
      taskEditorOpen.value = false
    } finally {
      taskSaving.value = false
    }
  }

  async function onCreateProject(option: OmniSelectOption) {
    const name = normalizeText(option.label || option.value, 140)
    if (!name) return
    const created = await tasksWorkspace.createProject(name)
    if (created) hydrateProjectDraft(created)
    Object.assign(filters, DEFAULT_FILTERS)
  }

  function columnsFromStatusDraft() {
    const currentColumns = new Map(projectSettingsDraft.columns.map(column => [normalizeKey(column.label), column] as const))
    return uniqueValues(projectSettingsDraft.statuses)
      .map((label, index) => {
        const existing = currentColumns.get(normalizeKey(label))
        return existing
          ? { ...existing, label, order: (index + 1) * ORDER_STEP }
          : { id: `column-${normalizeKey(label) || index}`, label, color: COLUMN_COLOR_OPTIONS[index % COLUMN_COLOR_OPTIONS.length]?.value as string || 'indigo', order: (index + 1) * ORDER_STEP }
      })
  }

  async function saveProjectSettings() {
    const project = activeProject.value
    if (!project) return
    settingsSaving.value = true
    const currentBoardView = projectView(project, 'board')
    const currentTableView = projectView(project, 'table')
    const views = [
      {
        ...currentBoardView,
        groupByFieldKey: normalizeText(projectSettingsDraft.boardGroupBy, 80) || 'status',
        visibleFieldKeys: [...projectSettingsDraft.boardVisibleFieldKeys],
        modalVisibleFieldKeys: [...projectSettingsDraft.modalVisibleFieldKeys],
        showAggregation: Boolean(projectSettingsDraft.showAggregation)
      },
      {
        ...currentTableView,
        visibleFieldKeys: ['title', ...projectSettingsDraft.tableVisibleFieldKeys.filter(key => key !== 'title')]
      }
    ]
    try {
      const updated = await tasksWorkspace.saveProjectSettings(project.id, {
        name: normalizeText(projectSettingsDraft.name, 140) || project.name,
        description: normalizeText(projectSettingsDraft.description, 300),
        icon: normalizeText(projectSettingsDraft.icon, 40) || project.icon,
        columns: columnsFromStatusDraft(),
        responsibles: [...projectSettingsDraft.responsibles],
        types: [...projectSettingsDraft.types],
        views,
        filters: { ...projectSettingsDraft.filters },
        cardFields: { ...projectSettingsDraft.cardFields },
        defaults: { ...projectSettingsDraft.defaults }
      })
      if (updated) {
        hydrateProjectDraft(updated)
        if (!updated.filters.search) filters.search = ''
        if (!updated.filters.responsible) filters.responsible = ''
        if (!updated.filters.type) filters.type = ''
        if (!updated.filters.client) filters.clientId = ''
      }
      projectSettingsOpen.value = false
    } finally {
      settingsSaving.value = false
    }
  }

  async function deleteProject() {
    const project = activeProject.value
    if (!project || tasksWorkspace.projects.value.length <= 1) return
    if (import.meta.client && !window.confirm(`Excluir o projeto "${project.name}"?`)) return
    if (await tasksWorkspace.deleteProject(project.id)) {
      projectSettingsOpen.value = false
      tableSelectedRows.value = []
      closeTaskEditor()
    }
  }

  function prepareColumnDraft(column: TaskBoardColumn) {
    columnDraft.id = column.id
    columnDraft.label = column.label
    columnDraft.color = column.color || 'indigo'
  }

  function openColumnSettings(column: TaskBoardColumn) {
    prepareColumnDraft(column)
    columnSettingsOpen.value = true
  }

  function closeColumnSettings() {
    columnSettingsOpen.value = false
    columnDraft.id = ''
    columnDraft.label = ''
    columnDraft.color = 'indigo'
  }

  async function saveColumnSettings() {
    const project = activeProject.value
    if (!project || !columnDraft.id) return
    await tasksWorkspace.updateColumn(project.id, columnDraft.id, {
      label: normalizeText(columnDraft.label, 120),
      color: normalizeText(columnDraft.color, 40)
    })
    hydrateProjectDraft(activeProject.value)
    closeColumnSettings()
  }

  async function deleteColumn() {
    const project = activeProject.value
    if (!project || !columnDraft.id || boardSchemaColumns.value.length <= 1) return
    if (import.meta.client && !window.confirm(`Excluir a coluna "${columnDraft.label}"? Os itens vao para a primeira coluna disponivel.`)) return
    await tasksWorkspace.deleteColumn(project.id, columnDraft.id)
    hydrateProjectDraft(activeProject.value)
    closeColumnSettings()
  }

  async function createColumn() {
    const project = activeProject.value
    if (!project || boardGroupBy.value !== 'status') return
    const created = await tasksWorkspace.createColumn(project.id, `Nova coluna ${boardSchemaColumns.value.length + 1}`)
    if (created) hydrateProjectDraft(activeProject.value)
  }

  function focusDraftCard(columnId: string) {
    if (!import.meta.client) return
    nextTick(() => {
      const input = document.querySelector(`[data-draft-card="${columnId}"] input, input[data-draft-card="${columnId}"]`) as HTMLInputElement | null
      input?.focus()
      input?.select()
    })
  }

  function focusBoardTitle(taskId: string) {
    if (!import.meta.client) return
    nextTick(() => {
      const input = document.querySelector(`[data-task-title-input="${taskId}"] input, input[data-task-title-input="${taskId}"]`) as HTMLInputElement | null
      input?.focus()
      input?.select()
    })
  }

  function defaultsForColumn(column: { id: string, status: string, groupFieldKey?: string, value?: string }) {
    const project = activeProject.value
    const clientId = viewerUserType.value === 'client'
      ? sessionSimulation.clientId
      : (project?.defaults.clientFromSession ? sessionSimulation.clientId : (toNumberId(filters.clientId) || sessionSimulation.clientId))
    const responsible = project?.defaults.responsibleFromCreator ? currentUserName.value : ''
    const base = {
      status: boardGroupBy.value === 'status' ? column.status : (statuses.value[0] || 'Raw'),
      responsible,
      involved: responsible ? [responsible] : [],
      clientId,
      clientName: clientLabel(clientId),
      type: '',
      priority: '' as unknown as TaskPriority,
      dueDate: ''
    }
    return { ...base, ...patchForGroupColumn(column) }
  }

  function beginCreateTaskInColumn(column: { id: string, status: string, groupFieldKey?: string, value?: string }) {
    const project = activeProject.value
    if (!project) return
    creatingCards[column.id] = { title: '', firstEnterDone: false, ...defaultsForColumn(column) }
    focusDraftCard(column.id)
  }

  function beginCreateTaskInFirstColumn() {
    const first = boardColumns.value[0]
    if (first) beginCreateTaskInColumn(first)
  }

  function cancelDraftCard(columnId: string) {
    delete creatingCards[columnId]
    delete draftAddedFields[columnId]
    delete draftMenuOpen[columnId]
    delete draftFieldOpen[columnId]
  }

  function onDraftCardFocusOut(event: FocusEvent, column: { id: string, status: string, groupFieldKey?: string, value?: string }) {
    const current = event.currentTarget as HTMLElement | null
    const next = event.relatedTarget as Node | null
    if (current && next && current.contains(next)) return
    if (draftMenuOpen[column.id]) return
    setTimeout(() => {
      if (!creatingCards[column.id]) return
      const fieldStates = draftFieldOpen[column.id]
      if (fieldStates && Object.values(fieldStates).some(Boolean)) return
      if (current && document.activeElement && current.contains(document.activeElement)) return
      commitDraftCard(column, true)
    }, 200)
  }

  async function commitDraftCard(column: { id: string, status: string, groupFieldKey?: string, value?: string }, useDefaultTitle = true, focusAfter = false) {
    const project = activeProject.value
    if (!project) return
    const draft = creatingCards[column.id]
    if (!draft) return
    const title = normalizeText(draft.title, 220) || (useDefaultTitle ? 'Nova task' : '')
    if (!title) return
    const created = await tasksWorkspace.createTask({
      projectId: project.id,
      status: draft.status,
      title,
      responsible: draft.responsible,
      involved: [...draft.involved],
      clientId: draft.clientId,
      clientName: draft.clientName,
      type: draft.type,
      priority: toPriority(draft.priority),
      dueDate: draft.dueDate,
      createdBy: currentUserName.value,
      ...patchForGroupColumn(column)
    })
    delete creatingCards[column.id]
    delete draftAddedFields[column.id]
    delete draftMenuOpen[column.id]
    delete draftFieldOpen[column.id]
    if (created) {
      activeInlineTaskId.value = focusAfter ? created.id : ''
      if (focusAfter) focusBoardTitle(created.id)
    }
  }

  function isDraftFieldVisible(columnId: string, fieldKey: string): boolean {
    const draft = creatingCards[columnId]
    if (!draft) return false
    const project = activeProject.value
    if (!project) return false
    const cf = project.cardFields
    const vf = boardView.value.visibleFieldKeys
    const enabled: Record<string, boolean> = {
      responsible: !!cf.responsible && vf.includes('responsible'),
      involved: !!cf.involved && vf.includes('involved'),
      clientId: viewerUserType.value === 'admin' && !!cf.client && vf.includes('clientId'),
      type: !!cf.type && vf.includes('type'),
      priority: !!cf.priority && vf.includes('priority'),
      dueDate: !!cf.dueDate && vf.includes('dueDate'),
    }
    if (!enabled[fieldKey]) return false
    if (draftAddedFields[columnId]?.includes(fieldKey)) return true
    if (fieldKey === 'responsible') return !!draft.responsible
    if (fieldKey === 'involved') return draft.involved.length > 0
    if (fieldKey === 'clientId') return draft.clientId > 0
    if (fieldKey === 'type') return !!draft.type
    if (fieldKey === 'dueDate') return !!draft.dueDate
    return false
  }

  function draftAvailableFields(columnId: string): Array<{ key: string, label: string, icon: string }> {
    const project = activeProject.value
    if (!project) return []
    const cf = project.cardFields
    const vf = boardView.value.visibleFieldKeys
    const all: Array<{ key: string, label: string, icon: string, enabled: boolean }> = [
      { key: 'type', label: 'Tipo', icon: 'i-lucide-hash', enabled: !!cf.type && vf.includes('type') },
      { key: 'clientId', label: 'Cliente', icon: 'i-lucide-circle-dot', enabled: viewerUserType.value === 'admin' && !!cf.client && vf.includes('clientId') },
      { key: 'dueDate', label: 'Prazo', icon: 'i-lucide-calendar-days', enabled: !!cf.dueDate && vf.includes('dueDate') },
      { key: 'priority', label: 'Prioridade', icon: 'i-lucide-flag', enabled: !!cf.priority && vf.includes('priority') },
      { key: 'responsible', label: 'Responsável', icon: 'i-lucide-user', enabled: !!cf.responsible && vf.includes('responsible') },
      { key: 'involved', label: 'Envolvidos', icon: 'i-lucide-users', enabled: !!cf.involved && vf.includes('involved') },
    ]
    return all.filter(f => f.enabled && !isDraftFieldVisible(columnId, f.key)).map(({ key, label, icon }) => ({ key, label, icon }))
  }

  function addDraftField(columnId: string, fieldKey: string) {
    if (!draftAddedFields[columnId]) draftAddedFields[columnId] = []
    if (!draftAddedFields[columnId].includes(fieldKey)) draftAddedFields[columnId].push(fieldKey)
    if (fieldKey === 'dueDate') {
      setDraftFieldOpen(columnId, 'dueDate', false)
      nextTick(() => setDraftFieldOpen(columnId, 'dueDate', true))
    } else {
      setDraftFieldOpen(columnId, fieldKey, false)
      nextTick(() => setDraftFieldOpen(columnId, fieldKey, true))
    }
  }

  const searchOpen = ref(false)
  function openSearch() {
    searchOpen.value = true
    nextTick(() => {
      const el = document.querySelector('.tasks-toolbar__search input') as HTMLInputElement | null
      el?.focus()
    })
  }
  function closeSearch() { if (!filters.search) searchOpen.value = false }
  function toggleSearch() {
    if (searchOpen.value || filters.search) {
      if (filters.search) filters.search = ''
      searchOpen.value = false
    } else {
      openSearch()
    }
  }

  const responsibleOpen = ref(false)
  const clientOpen = ref(false)
  const typeOpen = ref(false)
  const responsibleInnerOpen = ref(false)
  const clientInnerOpen = ref(false)
  const typeInnerOpen = ref(false)
  watch(responsibleOpen, (open: boolean) => {
    if (open) nextTick(() => { responsibleInnerOpen.value = true })
    else responsibleInnerOpen.value = false
  })
  watch(clientOpen, (open: boolean) => {
    if (open) nextTick(() => { clientInnerOpen.value = true })
    else clientInnerOpen.value = false
  })
  watch(typeOpen, (open: boolean) => {
    if (open) nextTick(() => { typeInnerOpen.value = true })
    else typeInnerOpen.value = false
  })
  function clearFilters() {
    Object.assign(filters, DEFAULT_FILTERS)
    searchOpen.value = false
  }
  function labelFor(options: { label?: string, value: string | number }[], value: string | number) {
    const found = options.find(o => String(o.value) === String(value))
    return String(found?.label ?? value)
  }
  const activeFilterChips = computed(() => {
    const chips: Array<{ key: string, label: string, value: string, onRemove: () => void }> = []
    if (filters.search) chips.push({ key: 'search', label: 'Busca', value: filters.search, onRemove: () => { filters.search = ''; searchOpen.value = false } })
    if (filters.responsible) chips.push({ key: 'responsible', label: 'Responsavel', value: labelFor(responsibleOptions.value, filters.responsible), onRemove: () => { filters.responsible = '' } })
    if (filters.clientId) chips.push({ key: 'client', label: 'Cliente', value: labelFor(clientOptions.value, filters.clientId as any), onRemove: () => { filters.clientId = '' } })
    if (filters.type) chips.push({ key: 'type', label: 'Tipo', value: labelFor(typeOptions.value, filters.type), onRemove: () => { filters.type = '' } })
    return chips
  })
  const hasAnyActiveFilter = computed(() => activeFilterChips.value.length > 0 || !filters.hideArchived)
  function toggleArchive(task: TaskItem) { tasksWorkspace.toggleArchiveTask(task.id) }
  function deleteTask(task: TaskItem) {
    if (import.meta.client && !window.confirm(`Excluir task "${task.title}"?`)) return
    tasksWorkspace.removeTask(task.id)
    if (taskEditorOpen.value && taskDraft.id === task.id) closeTaskEditor()
  }
  function deleteCurrentDraftTask() {
    const task = tasksWorkspace.tasks.value.find(t => t.id === taskDraft.id)
    if (task) deleteTask(task)
  }

  function onDragStart(task: TaskItem, event: DragEvent) {
    dragKind.value = 'task'
    draggingTaskId.value = task.id
    draggingColumnId.value = ''
    event.dataTransfer?.setData('text/plain', task.id)
    if (event.dataTransfer) event.dataTransfer.effectAllowed = 'move'
  }
  function onDragEnd() { draggingTaskId.value = ''; dragKind.value = ''; dropTarget.columnId = ''; dropTarget.index = -1 }
  async function onDropColumn(column: { id?: string, groupFieldKey?: string, value?: string, status: string }) {
    if (dragKind.value === 'task' && draggingTaskId.value) await moveTaskToGroupColumn(draggingTaskId.value, column)
    draggingTaskId.value = ''
    dragKind.value = ''
    dropTarget.columnId = ''
    dropTarget.index = -1
  }
  async function onDropCard(column: { id?: string, groupFieldKey?: string, value?: string, status: string }, index: number) {
    if (dragKind.value === 'task' && draggingTaskId.value) await moveTaskToGroupColumn(draggingTaskId.value, column, index)
    draggingTaskId.value = ''
    dragKind.value = ''
    dropTarget.columnId = ''
    dropTarget.index = -1
  }
  function markDropTarget(columnId: string, index = -1) {
    if (dragKind.value !== 'task') return
    dropTarget.columnId = columnId
    dropTarget.index = index
  }

  function onColumnDragStart(column: TaskBoardColumn, event: DragEvent) {
    dragKind.value = 'column'
    draggingColumnId.value = column.id
    draggingTaskId.value = ''
    event.dataTransfer?.setData('text/plain', column.id)
    if (event.dataTransfer) event.dataTransfer.effectAllowed = 'move'
  }
  function onColumnDragEnd() { draggingColumnId.value = ''; dragKind.value = '' }
  function onDropColumnHeader(targetColumn: TaskBoardColumn, targetIndex: number) {
    const project = activeProject.value
    if (dragKind.value !== 'column' || !project || !draggingColumnId.value || draggingColumnId.value === targetColumn.id) return
    tasksWorkspace.moveColumn(project.id, draggingColumnId.value, targetIndex)
    hydrateProjectDraft(activeProject.value)
    draggingColumnId.value = ''
    dragKind.value = ''
  }

  function updateTaskInline(task: TaskItem, patch: Partial<TaskItem>) {
    tasksWorkspace.updateTask(task.id, patch)
    const project = activeProject.value
    if (!project) return
    const responsible = Object.prototype.hasOwnProperty.call(patch, 'responsible') ? normalizeText(patch.responsible, 120) : ''
    const type = Object.prototype.hasOwnProperty.call(patch, 'type') ? normalizeText(patch.type, 120) : ''
    if (responsible && !project.responsibles.some(v => normalizeKey(v) === normalizeKey(responsible))) {
      tasksWorkspace.saveProjectSettings(project.id, { responsibles: [...project.responsibles, responsible] })
    }
    if (type && !project.types.some(v => normalizeKey(v) === normalizeKey(type))) {
      tasksWorkspace.saveProjectSettings(project.id, { types: [...project.types, type] })
    }
  }

  function onCardFocusOut(event: FocusEvent, task: TaskItem) {
    const current = event.currentTarget as HTMLElement | null
    const next = event.relatedTarget as Node | null
    if (current && next && current.contains(next)) return
    if (activeInlineTaskId.value === task.id) activeInlineTaskId.value = ''
  }

  function isCardFieldVisible(task: TaskItem, key: keyof TaskProjectItem['cardFields']) {
    const project = activeProject.value
    if (!project) return false
    if (key !== 'createdAt' && !project.cardFields[key]) return false
    if (key === 'createdAt' && !project.cardFields.createdAt && !project.defaults.showCreatedAt) return false
    const fieldKey = key === 'client' ? 'clientId' : key
    if (key !== 'createdAt' && !boardView.value.visibleFieldKeys.includes(fieldKey)) return false
    if (activeInlineTaskId.value === task.id) return true
    if (key === 'status') return false
    if (key === 'responsible') return Boolean(normalizeText(task.responsible, 120))
    if (key === 'involved') return Array.isArray(task.involved) && task.involved.length > 0
    if (key === 'client') return viewerUserType.value === 'admin' && Boolean(task.clientName)
    if (key === 'type') return Boolean(normalizeText(task.type, 120))
    if (key === 'dueDate') return Boolean(normalizeText(task.dueDate, 24))
    if (key === 'priority') return Boolean(task.priority)
    if (key === 'createdAt') return activeProject.value?.defaults.showCreatedAt || boardView.value.visibleFieldKeys.includes('createdAt')
    return true
  }

  function isModalFieldVisible(key: string) {
    const visible = boardView.value.modalVisibleFieldKeys || defaultView('board').modalVisibleFieldKeys
    return visible.includes(key)
  }

  function hideColumn(columnId: string) {
    const hidden = new Set(boardView.value.hiddenColumnIds || [])
    hidden.add(columnId)
    updateProjectView('board', { hiddenColumnIds: [...hidden] })
  }
  function showAllColumns() { updateProjectView('board', { hiddenColumnIds: [] }) }
  function toggleAggregation() { updateProjectView('board', { showAggregation: boardView.value.showAggregation === false }) }

  function deleteTasksInColumn(column: { status: string, groupFieldKey?: string, value?: string, tasks?: TaskItem[] }) {
    if (import.meta.client && !window.confirm(`Excluir todos os cards em "${column.status}"?`)) return
    column.tasks?.forEach((task: TaskItem) => tasksWorkspace.removeTask(task.id))
  }

  function createTableTask() {
    const firstStatus = statuses.value[0] || 'Raw'
    const created = tasksWorkspace.createTask({
      projectId: activeProject.value?.id,
      status: firstStatus,
      title: 'Nova task',
      clientId: viewerUserType.value === 'client' ? sessionSimulation.clientId : (toNumberId(filters.clientId) || sessionSimulation.clientId),
      clientName: clientLabel(viewerUserType.value === 'client' ? sessionSimulation.clientId : (toNumberId(filters.clientId) || sessionSimulation.clientId))
    })
    if (created) {
      viewMode.value = 'table'
      tableFocusCell.value = { rowId: created.id, columnKey: 'title', token: Date.now() }
    }
  }

  function onTableCellUpdate(payload: OmniTableCellUpdate) {
    const id = normalizeText(payload.rowId, 120)
    const key = normalizeText(payload.key, 120)
    if (!id || !key) return
    if (key === 'clientId') {
      const clientId = toNumberId(payload.value)
      if (clientId) tasksWorkspace.updateTask(id, { clientId, clientName: clientLabel(clientId) })
      return
    }
    if (key === 'priority') {
      const p = normalizeKey(payload.value)
      const priority: TaskPriority = p === 'alta' || p === 'media' || p === 'baixa' ? p : 'media'
      tasksWorkspace.updateTask(id, { priority })
      return
    }
    if (key === 'archived') { tasksWorkspace.updateTask(id, { archived: Boolean(payload.value) }); return }
    if (key === 'status') { const status = normalizeText(payload.value, 120); if (status) tasksWorkspace.updateTask(id, { status }); return }
    if (key === 'responsible') {
      const responsible = normalizeText(payload.value, 120)
      tasksWorkspace.updateTask(id, { responsible })
      const project = activeProject.value
      if (project && responsible && !project.responsibles.some(v => normalizeKey(v) === normalizeKey(responsible))) {
        tasksWorkspace.saveProjectSettings(project.id, { responsibles: [...project.responsibles, responsible] })
      }
      return
    }
    if (key === 'involved') {
      const involved = normalizeText(payload.value, 500)
        .split(',').map(item => normalizeText(item, 120)).filter(Boolean)
      tasksWorkspace.updateTask(id, { involved })
      return
    }
    if (key === 'type') {
      const type = normalizeText(payload.value, 120)
      tasksWorkspace.updateTask(id, { type })
      const project = activeProject.value
      if (project && type && !project.types.some(v => normalizeKey(v) === normalizeKey(type))) {
        tasksWorkspace.saveProjectSettings(project.id, { types: [...project.types, type] })
      }
      return
    }
    if (key === 'title') { const title = normalizeText(payload.value, 220); if (title) tasksWorkspace.updateTask(id, { title }); return }
    if (key === 'description') { tasksWorkspace.updateTask(id, { description: normalizeText(payload.value, 5000) }); return }
    if (key === 'dueDate') { tasksWorkspace.updateTask(id, { dueDate: normalizeText(payload.value, 24) }) }
  }

  function onTableRowAction(payload: { action: string, row: Record<string, unknown> }) {
    const id = normalizeText(payload.row.id, 120)
    const task = tasksWorkspace.tasks.value.find(t => t.id === id)
    if (!task) return
    if (payload.action === 'edit') { openTaskEditor(task); return }
    if (payload.action === 'archive') { toggleArchive(task); return }
    if (payload.action === 'delete') deleteTask(task)
  }

  function setTaskEditorMode(mode: 'side' | 'center' | 'fullscreen') { taskEditorMode.value = mode }

  function startTaskEditorResize(event: MouseEvent) {
    if (taskEditorMode.value !== 'side' || !import.meta.client) return
    event.preventDefault()
    taskEditorResizing.value = true
    const startX = event.clientX
    const startWidth = taskEditorWidth.value
    const maxWidth = () => Math.min(window.innerWidth - 80, 1120)
    const onMove = (moveEvent: MouseEvent) => {
      const next = startWidth + (startX - moveEvent.clientX)
      taskEditorWidth.value = Math.max(560, Math.min(maxWidth(), next))
    }
    const onUp = () => {
      taskEditorResizing.value = false
      window.removeEventListener('mousemove', onMove)
      window.removeEventListener('mouseup', onUp)
    }
    window.addEventListener('mousemove', onMove)
    window.addEventListener('mouseup', onUp)
  }

  function syncClientFilter() {
    const project = activeProject.value
    if (!project) return
    if (viewerUserType.value === 'client' || !project.filters.client) filters.clientId = ''
  }

  watch(() => tasksWorkspace.activeProjectId.value, () => {
    hydrateProjectDraft(activeProject.value)
    syncClientFilter()
    tableSelectedRows.value = []
    if (taskEditorOpen.value && taskDraft.id && !tasksWorkspace.tasks.value.some(t => t.id === taskDraft.id)) closeTaskEditor()
  }, { immediate: true })

  watch(() => viewerUserType.value, () => { syncClientFilter() }, { immediate: true })

  onMounted(async () => {
    try {
      await pageLoading.withLoading('Carregando tasks...', async () => {
        sessionSimulation.initialize()
        await tasksWorkspace.initialize()
        if (sessionSimulation.isAdmin) await sessionSimulation.refreshClientOptions()
        if (!activeProject.value && tasksWorkspace.projects.value.length > 0) tasksWorkspace.setActiveProject(tasksWorkspace.projects.value[0]!.id)
        hydrateProjectDraft(activeProject.value)
        await nextTick()
        if (import.meta.client) {
          await new Promise<void>((resolve) => { requestAnimationFrame(() => resolve()) })
        }
      })
    } finally {
      pageBootstrapping.value = false
    }
  })

  return {
    // constants
    ORDER_STEP, PRIORITY_OPTIONS, COLUMN_COLOR_OPTIONS, DEFAULT_FILTERS, BOARD_GROUP_OPTIONS, FIELD_DEFS,
    filterSwitchDefs, cardFieldSwitchDefs, modalModeOptions,
    // state
    viewMode, pageBootstrapping, draggingTaskId, draggingColumnId, filters, tableSelectedRows,
    tableFocusCell, activeInlineTaskId, creatingCards, draftAddedFields, draftMenuOpen, draftFieldOpen,
    dragKind, dropTarget, projectSettingsOpen, columnSettingsOpen, taskEditorOpen, taskEditorMode,
    taskEditorWidth, taskEditorResizing, settingsSaving, taskSaving,
    legacyMigrationNotice: tasksWorkspace.legacyMigrationNotice,
    projectSettingsDraft, columnDraft, taskDraft,
    // computed
    viewerUserType, activeProject, projectOptions, clientOptions, currentUserName, taskEditorCssVars,
    boardSchemaColumns, boardView, tableView, boardGroupBy, statuses, statusOptions,
    responsibleOptions, involvedOptions, typeOptions,
    responsibleOptionsAvatar, involvedOptionsAvatar, clientOptionsAvatar,
    peopleMentionLabels, clientMentionLabels, taskMentionLabels,
    projectModel, projectTasks, filteredTasks, boardColumns, tableRows, projectCount, tableColumns,
    searchOpen, responsibleOpen, clientOpen, typeOpen, responsibleInnerOpen, clientInnerOpen, typeInnerOpen,
    activeFilterChips, hasAnyActiveFilter,
    // tracking
    startTracking, pauseTracking, stopTracking, isTracking, isRunning, getElapsedMs, formatElapsed,
    // functions
    setDraftFieldOpen, normalizeText, normalizeKey, toNumberId, dateLabel, dateLabelLong,
    priorityLabel, priorityColor, toPriority, columnColorClass, clientLabel, taskSort,
    fieldLabel, fieldSwitchValue, setFieldSwitch,
    hydrateProjectDraft, resetTaskDraft, openTaskEditor, closeTaskEditor, saveTask,
    onCreateProject, saveProjectSettings, deleteProject,
    prepareColumnDraft, openColumnSettings, closeColumnSettings, saveColumnSettings, deleteColumn, createColumn,
    beginCreateTaskInColumn, beginCreateTaskInFirstColumn, cancelDraftCard, onDraftCardFocusOut, commitDraftCard,
    isDraftFieldVisible, draftAvailableFields, addDraftField,
    openSearch, closeSearch, toggleSearch, clearFilters, labelFor,
    toggleArchive, deleteTask, deleteCurrentDraftTask,
    onDragStart, onDragEnd, onDropColumn, onDropCard, markDropTarget,
    onColumnDragStart, onColumnDragEnd, onDropColumnHeader,
    updateTaskInline, onCardFocusOut, isCardFieldVisible, isModalFieldVisible,
    hideColumn, showAllColumns, toggleAggregation, deleteTasksInColumn,
    createTableTask, onTableCellUpdate, onTableRowAction,
    setTaskEditorMode, startTaskEditorResize, syncClientFilter,
    groupOptionsFor, updateProjectView
  }
}

export type TasksPageContext = ReturnType<typeof useTasksPageContext>
