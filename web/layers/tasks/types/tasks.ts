export type TaskPriority = 'baixa' | 'media' | 'alta'

export interface TaskVideoItem {
  id: string
  name: string
  url: string
  size: number
  contentType: string
  uploadedAt: string
}

// TaskCalendarMediaItem = midia ESPELHADA do evento vinculado no calendario (WAVE 6 cruzamento A,
// read-only). A task nao guarda imagem; aqui e' so exibicao, entao imagem+video do evento aparecem.
// url aponta para /uploads/calendar/{conta}/ (servido global); populada pelo sync do backend.
export interface TaskCalendarMediaItem {
  id: string
  url: string
  name: string
  type: 'image' | 'video'
  sizeBytes: number
  contentType: string
  posterUrl: string
}

export interface TaskProjectFiltersConfig {
  search: boolean
  responsible: boolean
  client: boolean
  type: boolean
  hideArchived: boolean
}

export interface TaskProjectCardFieldsConfig {
  status: boolean
  responsible: boolean
  involved: boolean
  client: boolean
  type: boolean
  dueDate: boolean
  priority: boolean
  createdAt: boolean
}

export interface TaskProjectDefaultsConfig {
  responsibleFromCreator: boolean
  clientFromSession: boolean
  showCreatedAt: boolean
}

export interface TaskBoardColumn {
  id: string
  label: string
  color: string
  order: number
}

export type OrchestratorFieldType =
  | 'title'
  | 'text'
  | 'select'
  | 'multiSelect'
  | 'status'
  | 'person'
  | 'client'
  | 'date'
  | 'priority'
  | 'number'
  | 'checkbox'

export interface OrchestratorField {
  id: string
  key: string
  label: string
  type: OrchestratorFieldType
  required: boolean
  hidden: boolean
  order: number
}

export interface OrchestratorView {
  id: string
  name: string
  type: 'board' | 'table'
  groupByFieldKey: string
  visibleFieldKeys: string[]
  modalVisibleFieldKeys: string[]
  hiddenColumnIds: string[]
  showAggregation: boolean
  sortBy: string
  sortDirection: 'asc' | 'desc'
}

export interface TaskProjectItem {
  id: string
  name: string
  description: string
  icon: string
  columns: TaskBoardColumn[]
  statuses: string[]
  responsibles: string[]
  types: string[]
  fields: OrchestratorField[]
  views: OrchestratorView[]
  activeViewId: string
  filters: TaskProjectFiltersConfig
  cardFields: TaskProjectCardFieldsConfig
  defaults: TaskProjectDefaultsConfig
  createdAt: string
  updatedAt: string
}

export interface TaskItem {
  id: string
  projectId: string
  title: string
  description: string
  contentHtml: string
  status: string
  responsible: string
  involved: string[]
  // clientId = UUID do cliente (core.accounts), gravado em clientAccountId no backend. String vazia
  // = sem cliente. Tasks antigas podem trazer um clientId integer LEGADO (ver tasksClient.isMockClient).
  clientId: string
  clientName: string
  type: string
  priority: TaskPriority
  dueDate: string
  dueEndDate: string
  archived: boolean
  order: number
  createdBy: string
  createdAt: string
  updatedAt: string
  videos?: TaskVideoItem[]
  // calendarMedia = midia do evento vinculado, espelhada read-only (WAVE 6 cruzamento A).
  calendarMedia?: TaskCalendarMediaItem[]
}

export interface TasksWorkspaceState {
  activeProjectId: string
  projects: TaskProjectItem[]
  tasks: TaskItem[]
}
