export type TaskPriority = 'baixa' | 'media' | 'alta'

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
  clientId: number
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
}

export interface TasksWorkspaceState {
  activeProjectId: string
  projects: TaskProjectItem[]
  tasks: TaskItem[]
}
