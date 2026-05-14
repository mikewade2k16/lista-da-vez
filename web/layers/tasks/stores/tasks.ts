import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { useAuthStore } from '~/stores/auth'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import type {
  OrchestratorField,
  OrchestratorView,
  TaskBoardColumn,
  TaskItem,
  TaskPriority,
  TaskProjectCardFieldsConfig,
  TaskProjectDefaultsConfig,
  TaskProjectFiltersConfig,
  TaskProjectItem
} from '../types/tasks'

const LEGACY_STORAGE_KEY = 'omni.admin.tasks.workspace.v1'
const LEGACY_NOTICE_KEY = 'tasks.legacy-migrated.v1'
const ORDER_STEP = 10
const SUPPORTED_COLUMN_COLORS = new Set(['indigo', 'slate', 'blue', 'amber', 'emerald', 'violet', 'rose'])

interface BackendUserMini {
	id?: string
}

interface BackendColumn {
	id?: string
	boardId?: string
	label?: string
	color?: string
	sortOrder?: number
	createdAt?: string
}

interface BackendField {
	id?: string
	boardId?: string
	key?: string
	label?: string
	type?: string
	required?: boolean
	hidden?: boolean
	sortOrder?: number
}

interface BackendView {
	id?: string
	boardId?: string
	name?: string
	type?: string
	config?: Record<string, any>
	sortOrder?: number
}

interface BackendBoard {
	id?: string
	name?: string
	description?: string
	icon?: string
	archived?: boolean
	columns?: BackendColumn[]
	fields?: BackendField[]
	views?: BackendView[]
	createdAt?: string
	updatedAt?: string
}

interface BackendTask {
	id?: string
	boardId?: string
	columnId?: string | null
	title?: string
	contentHtml?: string
	status?: string | null
	priority?: string
	dueDate?: string | null
	archived?: boolean
	sortOrder?: number
	responsible?: BackendUserMini | null
	responsibleUserId?: string | null
	clientAccountId?: string | null
	assignees?: BackendUserMini[]
	trackingTotalMs?: number | null
	version?: number
	createdAt?: string
	updatedAt?: string
}

export interface TasksStoreTaskItem extends TaskItem {
	columnId?: string
	version?: number
	responsibleUserId?: string
	clientAccountId?: string
	trackingTotalMs?: number
}

function normalizeText(value: unknown, max = 240) {
	return String(value ?? '').trim().slice(0, max)
}

function normalizeKey(value: unknown) {
	return normalizeText(value, 160)
		.normalize('NFD')
		.replace(/[\u0300-\u036f]/g, '')
		.toLowerCase()
		.replace(/[^a-z0-9]+/g, '_')
		.replace(/^_+|_+$/g, '')
}

function normalizePriority(value: unknown): TaskPriority {
	const key = normalizeKey(value)
	if (key === 'alta' || key === 'baixa' || key === 'media') {
		return key
	}
	return 'media'
}

function normalizeColumnColor(color: unknown) {
	const key = normalizeKey(color)
	if (key === 'green') {
		return 'emerald'
	}
	return SUPPORTED_COLUMN_COLORS.has(key) ? key : 'slate'
}

function normalizeFieldKey(key: unknown) {
	const normalized = normalizeKey(key)
	if (normalized === 'due_date') {
		return 'dueDate'
	}
	return normalized || 'field'
}

function defaultFiltersConfig(value?: Partial<TaskProjectFiltersConfig>): TaskProjectFiltersConfig {
	return {
		search: value?.search ?? true,
		responsible: value?.responsible ?? true,
		client: value?.client ?? true,
		type: value?.type ?? true,
		hideArchived: value?.hideArchived ?? true
	}
}

function defaultCardFieldsConfig(value?: Partial<TaskProjectCardFieldsConfig>): TaskProjectCardFieldsConfig {
	return {
		status: value?.status ?? true,
		responsible: value?.responsible ?? true,
		involved: value?.involved ?? true,
		client: value?.client ?? true,
		type: value?.type ?? true,
		dueDate: value?.dueDate ?? true,
		priority: value?.priority ?? true,
		createdAt: value?.createdAt ?? false
	}
	}

function defaultProjectDefaults(value?: Partial<TaskProjectDefaultsConfig>): TaskProjectDefaultsConfig {
	return {
		responsibleFromCreator: value?.responsibleFromCreator ?? true,
		clientFromSession: value?.clientFromSession ?? true,
		showCreatedAt: value?.showCreatedAt ?? false
	}
}

function defaultViews(): OrchestratorView[] {
	return [
		{
			id: 'view-board',
			name: 'Board',
			type: 'board',
			groupByFieldKey: 'status',
			visibleFieldKeys: ['responsible', 'involved', 'clientId', 'type', 'priority', 'dueDate'],
			modalVisibleFieldKeys: ['description', 'status', 'responsible', 'involved', 'clientId', 'type', 'priority', 'dueDate', 'archived'],
			hiddenColumnIds: [],
			showAggregation: true,
			sortBy: 'order',
			sortDirection: 'asc'
		},
		{
			id: 'view-table',
			name: 'Tabela',
			type: 'table',
			groupByFieldKey: 'status',
			visibleFieldKeys: ['title', 'status', 'responsible', 'involved', 'clientId', 'type', 'priority', 'dueDate', 'archived'],
			modalVisibleFieldKeys: ['description', 'status', 'responsible', 'involved', 'clientId', 'type', 'priority', 'dueDate', 'archived'],
			hiddenColumnIds: [],
			showAggregation: true,
			sortBy: 'order',
			sortDirection: 'asc'
		}
	]
}

function uniqueStrings(values: Array<string | undefined | null>) {
	const seen = new Set<string>()
	const result: string[] = []
	for (const value of values) {
		const normalized = normalizeText(value, 160)
		if (!normalized) {
			continue
		}
		const key = normalizeKey(normalized)
		if (!key || seen.has(key)) {
			continue
		}
		seen.add(key)
		result.push(normalized)
	}
	return result
}

function stripHtml(value: unknown) {
	return normalizeText(String(value ?? '').replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' '), 5000)
}

function toDateOnly(value: unknown) {
	const raw = normalizeText(value, 80)
	if (!raw) {
		return ''
	}
	const date = new Date(raw)
	if (Number.isNaN(date.getTime())) {
		return ''
	}
	return date.toISOString().slice(0, 10)
}

function toOptionalDateTime(value: unknown) {
	const raw = normalizeText(value, 80)
	if (!raw) {
		return undefined
	}
	if (/^\d{4}-\d{2}-\d{2}$/.test(raw)) {
		return `${raw}T00:00:00Z`
	}
	const parsed = new Date(raw)
	if (Number.isNaN(parsed.getTime())) {
		return undefined
	}
	return parsed.toISOString()
}

function looksLikeUUID(value: unknown) {
	return /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(normalizeText(value, 80))
}

function sortColumns(columns: TaskBoardColumn[]) {
	return [...columns].sort((left, right) => left.order - right.order || left.label.localeCompare(right.label))
}

function sortTasks(list: TasksStoreTaskItem[]) {
	return [...list].sort((left, right) => {
		if (left.projectId !== right.projectId) {
			return left.projectId.localeCompare(right.projectId)
		}
		const orderDelta = Number(left.order || 0) - Number(right.order || 0)
		if (orderDelta !== 0) {
			return orderDelta
		}
		return left.createdAt.localeCompare(right.createdAt)
	})
}

function mapView(view: BackendView): OrchestratorView {
	const type = normalizeKey(view.type) === 'table' ? 'table' : 'board'
	const config = view.config || {}
	const fallback = type === 'table' ? defaultViews()[1] : defaultViews()[0]
	return {
		id: normalizeText(view.id, 80) || fallback.id,
		name: normalizeText(view.name, 120) || fallback.name,
		type,
		groupByFieldKey: normalizeFieldKey(config.groupByFieldKey || config.groupByFieldId || fallback.groupByFieldKey),
		visibleFieldKeys: Array.isArray(config.visibleFieldKeys) && config.visibleFieldKeys.length > 0
			? config.visibleFieldKeys.map((key: unknown) => normalizeFieldKey(key)).filter(Boolean)
			: [...fallback.visibleFieldKeys],
		modalVisibleFieldKeys: Array.isArray(config.modalVisibleFieldKeys) && config.modalVisibleFieldKeys.length > 0
			? config.modalVisibleFieldKeys.map((key: unknown) => normalizeFieldKey(key)).filter(Boolean)
			: [...fallback.modalVisibleFieldKeys],
		hiddenColumnIds: Array.isArray(config.hiddenColumnIds)
			? config.hiddenColumnIds.map((columnId: unknown) => normalizeText(columnId, 80)).filter(Boolean)
			: [],
		showAggregation: config.showAggregation !== false,
		sortBy: normalizeFieldKey(config.sortBy || fallback.sortBy),
		sortDirection: normalizeKey(config.sortDirection) === 'desc' ? 'desc' : 'asc'
	}
}

function mapBoardColumns(columns: BackendColumn[]) {
	const mapped = columns.map((column, index) => ({
		id: normalizeText(column.id, 80) || `column-${index + 1}`,
		label: normalizeText(column.label, 120) || `Coluna ${index + 1}`,
		color: normalizeColumnColor(column.color),
		order: Number(column.sortOrder || (index + 1) * ORDER_STEP) || (index + 1) * ORDER_STEP
	}))
	return mapped.length > 0
		? sortColumns(mapped)
		: [
			{ id: 'column-todo', label: 'A fazer', color: 'slate', order: 100 },
			{ id: 'column-doing', label: 'Em andamento', color: 'blue', order: 200 },
			{ id: 'column-done', label: 'Concluido', color: 'emerald', order: 300 }
		]
}

function mapBoardFields(fields: BackendField[], existingProject?: TaskProjectItem): OrchestratorField[] {
	const mapped = fields.map((field, index) => ({
		id: normalizeText(field.id, 80) || `field-${index + 1}`,
		key: normalizeFieldKey(field.key),
		label: normalizeText(field.label, 120) || `Campo ${index + 1}`,
		type: (normalizeKey(field.type) || 'text') as OrchestratorField['type'],
		required: Boolean(field.required),
		hidden: Boolean(field.hidden),
		order: Number(field.sortOrder || (index + 1) * ORDER_STEP) || (index + 1) * ORDER_STEP
	}))
	if (mapped.length > 0) {
		return mapped.sort((left, right) => left.order - right.order)
	}
	return existingProject?.fields ? [...existingProject.fields] : []
}

function findColumnByStatus(project: TaskProjectItem | undefined, status: unknown) {
	if (!project) {
		return null
	}
	const normalizedStatus = normalizeKey(status)
	return project.columns.find((column) => normalizeKey(column.label) === normalizedStatus) || null
}

function mapTaskToStoreItem(task: BackendTask, project: TaskProjectItem): TasksStoreTaskItem {
	const explicitColumnId = normalizeText(task.columnId, 80)
	const mappedColumn = explicitColumnId
		? project.columns.find((column) => column.id === explicitColumnId) || null
		: findColumnByStatus(project, task.status)
	const status = normalizeText(mappedColumn?.label || task.status || project.statuses[0] || 'A fazer', 120)
	const responsibleUserId = normalizeText(task.responsibleUserId || task.responsible?.id, 80)
	const clientAccountId = normalizeText(task.clientAccountId, 80)
	return {
		id: normalizeText(task.id, 80),
		projectId: normalizeText(task.boardId, 80) || project.id,
		title: normalizeText(task.title, 220) || 'Nova task',
		description: stripHtml(task.contentHtml),
		contentHtml: normalizeText(task.contentHtml, 50000),
		status,
		responsible: responsibleUserId,
		involved: Array.isArray(task.assignees)
			? task.assignees.map((item) => normalizeText(item?.id, 80)).filter(Boolean)
			: [],
		clientId: 0,
		clientName: clientAccountId ? `Conta ${clientAccountId.slice(0, 8)}` : '',
		type: '',
		priority: normalizePriority(task.priority),
		dueDate: toDateOnly(task.dueDate),
		archived: Boolean(task.archived),
		order: Number(task.sortOrder || 0) || ORDER_STEP,
		createdBy: '',
		createdAt: normalizeText(task.createdAt, 80) || new Date().toISOString(),
		updatedAt: normalizeText(task.updatedAt, 80) || new Date().toISOString(),
		columnId: normalizeText(mappedColumn?.id || explicitColumnId, 80),
		version: Number(task.version || 0) || 0,
		responsibleUserId: responsibleUserId || undefined,
		clientAccountId: clientAccountId || undefined,
		trackingTotalMs: task.trackingTotalMs == null ? undefined : Math.max(0, Number(task.trackingTotalMs) || 0)
	}
}

function buildProject(board: BackendBoard, existingProject: TaskProjectItem | undefined, boardTasks: TasksStoreTaskItem[]): TaskProjectItem {
	const columns = mapBoardColumns(Array.isArray(board.columns) ? board.columns : [])
	const views = Array.isArray(board.views) && board.views.length > 0
		? board.views.map((view) => mapView(view))
		: (existingProject?.views?.length ? [...existingProject.views] : defaultViews())
	const availableViewIds = new Set(views.map((view) => view.id))
	const activeViewId = normalizeText(existingProject?.activeViewId, 80)
	return {
		id: normalizeText(board.id, 80),
		name: normalizeText(board.name, 140) || existingProject?.name || 'Projeto',
		description: normalizeText(board.description, 300) || '',
		icon: normalizeText(board.icon, 40) || existingProject?.icon || 'layout-dashboard',
		columns,
		statuses: columns.map((column) => column.label),
		responsibles: uniqueStrings([...(existingProject?.responsibles || []), ...boardTasks.map((task) => task.responsible)]),
		types: uniqueStrings([...(existingProject?.types || []), ...boardTasks.map((task) => task.type)]),
		fields: mapBoardFields(Array.isArray(board.fields) ? board.fields : [], existingProject),
		views,
		activeViewId: activeViewId && availableViewIds.has(activeViewId) ? activeViewId : views[0]?.id || 'view-board',
		filters: defaultFiltersConfig(existingProject?.filters),
		cardFields: defaultCardFieldsConfig(existingProject?.cardFields),
		defaults: defaultProjectDefaults(existingProject?.defaults),
		createdAt: normalizeText(board.createdAt, 80) || existingProject?.createdAt || new Date().toISOString(),
		updatedAt: normalizeText(board.updatedAt, 80) || existingProject?.updatedAt || new Date().toISOString()
	}
}

export const useTasksStore = defineStore('tasks', () => {
	const runtimeConfig = useRuntimeConfig()
	const auth = useAuthStore()
	const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

	const initialized = ref(false)
	const initializing = ref(false)
	const pending = ref(false)
	const errorMessage = ref('')
	const projects = ref<TaskProjectItem[]>([])
	const tasks = ref<TasksStoreTaskItem[]>([])
	const activeProjectId = ref('')
	const legacyMigrationNotice = ref(false)

	const accountId = computed(() => normalizeText(auth.activeTenantId || auth.tenantContext?.[0]?.id, 80))

	async function request(path: string, options: Record<string, any> = {}) {
		if (auth.isAuthenticated) {
			await auth.ensureSession()
		}
		const resolvedAccountId = accountId.value
		if (!resolvedAccountId) {
			throw new Error('Conta ativa nao disponivel para Tasks.')
		}
		return apiRequest(path, {
			...options,
			headers: {
				...(options.headers || {}),
				'X-Account-Id': resolvedAccountId
			}
		})
	}

	function showLegacyNoticeIfNeeded() {
		if (!import.meta.client) {
			return
		}
		const hasLegacyState = Boolean(localStorage.getItem(LEGACY_STORAGE_KEY))
		if (!hasLegacyState) {
			return
		}
		localStorage.removeItem(LEGACY_STORAGE_KEY)
		if (localStorage.getItem(LEGACY_NOTICE_KEY) === '1') {
			return
		}
		localStorage.setItem(LEGACY_NOTICE_KEY, '1')
		legacyMigrationNotice.value = true
	}

	function dismissLegacyMigrationNotice() {
		legacyMigrationNotice.value = false
	}

	function replaceProject(project: TaskProjectItem) {
		const index = projects.value.findIndex((item) => item.id === project.id)
		if (index >= 0) {
			projects.value[index] = project
			return
		}
		projects.value = [project, ...projects.value]
	}

	function replaceTask(task: TasksStoreTaskItem) {
		const index = tasks.value.findIndex((item) => item.id === task.id)
		if (index >= 0) {
			tasks.value[index] = task
		} else {
			tasks.value = [...tasks.value, task]
		}
		tasks.value = sortTasks(tasks.value)
	}

	async function listBoardTasks(boardId: string) {
		const params = new URLSearchParams()
		params.set('limit', '200')
		params.set('archived', 'true')
		const response = await request(`/v1/tasks/boards/${encodeURIComponent(boardId)}/tasks?${params.toString()}`)
		return Array.isArray(response?.tasks) ? (response.tasks as BackendTask[]) : []
	}

	async function refresh() {
		pending.value = true
		errorMessage.value = ''
		try {
			const currentProjects = new Map(projects.value.map((project) => [project.id, project] as const))
			const response = await request('/v1/tasks/boards')
			const boards = Array.isArray(response?.boards)
				? (response.boards as BackendBoard[]).filter((board) => !board?.archived)
				: []
			const taskEntries = await Promise.all(
				boards.map(async (board) => [normalizeText(board.id, 80), await listBoardTasks(normalizeText(board.id, 80))] as const)
			)
			const boardTasksMap = new Map<string, TasksStoreTaskItem[]>()
			const nextProjects = boards.map((board) => {
				const boardId = normalizeText(board.id, 80)
				const placeholderProject = buildProject(board, currentProjects.get(boardId), [])
				const mappedTasks = (taskEntries.find(([entryBoardId]) => entryBoardId === boardId)?.[1] || [])
					.map((task) => mapTaskToStoreItem(task, placeholderProject))
				boardTasksMap.set(boardId, mappedTasks)
				return buildProject(board, currentProjects.get(boardId), mappedTasks)
			})
			projects.value = nextProjects
			tasks.value = sortTasks(nextProjects.flatMap((project) => boardTasksMap.get(project.id) || []))
			if (!projects.value.some((project) => project.id === activeProjectId.value)) {
				activeProjectId.value = projects.value[0]?.id || ''
			}
		} catch (error) {
			errorMessage.value = getApiErrorMessage(error, 'Nao foi possivel carregar as tasks.')
			throw error
		} finally {
			pending.value = false
		}
	}

	async function initialize() {
		if (initialized.value || initializing.value) {
			return
		}
		initializing.value = true
		try {
			showLegacyNoticeIfNeeded()
			await refresh()
			initialized.value = true
		} finally {
			initializing.value = false
		}
	}

	function setActiveProject(projectId: unknown) {
		const targetId = normalizeText(projectId, 80)
		if (!projects.value.some((project) => project.id === targetId)) {
			return
		}
		activeProjectId.value = targetId
	}

	function buildBoardSlug(name: unknown) {
		const normalized = normalizeKey(name)
		if (!normalized) {
			return `tasks-${Date.now()}`
		}
		return `${normalized}-${Date.now()}`
	}

	async function createProject(rawName: unknown) {
		const name = normalizeText(rawName, 140) || `Projeto ${projects.value.length + 1}`
		const response = await request('/v1/tasks/boards', {
			method: 'POST',
			body: {
				name,
				slug: buildBoardSlug(name),
				description: '',
				icon: 'layout-dashboard'
			}
		})
		const board = response?.board as BackendBoard | undefined
		if (!board?.id) {
			return null
		}
		const project = buildProject(board, undefined, [])
		replaceProject(project)
		activeProjectId.value = project.id
		return project
	}

	async function deleteProject(projectId: string) {
		if (projects.value.length <= 1) {
			return false
		}
		const target = projects.value.find((project) => project.id === normalizeText(projectId, 80))
		if (!target) {
			return false
		}
		await request(`/v1/tasks/boards/${encodeURIComponent(target.id)}`, {
			method: 'PATCH',
			body: { archived: true }
		})
		projects.value = projects.value.filter((project) => project.id !== target.id)
		tasks.value = tasks.value.filter((task) => task.projectId !== target.id)
		if (activeProjectId.value === target.id) {
			activeProjectId.value = projects.value[0]?.id || ''
		}
		return true
	}

	async function saveProjectSettings(
		projectId: string,
		payload: Partial<Pick<TaskProjectItem, 'name' | 'description' | 'icon' | 'statuses' | 'columns' | 'responsibles' | 'types' | 'fields' | 'views' | 'filters' | 'cardFields' | 'defaults' | 'activeViewId'>>
	) {
		const current = projects.value.find((project) => project.id === normalizeText(projectId, 80))
		if (!current) {
			return null
		}
		let remoteBoard: BackendBoard | null = null
		const nextName = Object.prototype.hasOwnProperty.call(payload, 'name') ? normalizeText(payload.name, 140) : current.name
		const nextDescription = Object.prototype.hasOwnProperty.call(payload, 'description') ? normalizeText(payload.description, 300) : current.description
		const nextIcon = Object.prototype.hasOwnProperty.call(payload, 'icon') ? normalizeText(payload.icon, 40) : current.icon
		if (nextName !== current.name || nextDescription !== current.description || nextIcon !== current.icon) {
			const response = await request(`/v1/tasks/boards/${encodeURIComponent(current.id)}`, {
				method: 'PATCH',
				body: {
					name: nextName,
					description: nextDescription,
					icon: nextIcon
				}
			})
			remoteBoard = (response?.board as BackendBoard | undefined) || null
		}
		const nextProject = buildProject(remoteBoard || {
			id: current.id,
			name: nextName,
			description: nextDescription,
			icon: nextIcon,
			columns: payload.columns?.map((column) => ({
				id: column.id,
				label: column.label,
				color: column.color,
				sortOrder: column.order
			})) || current.columns,
			fields: current.fields.map((field) => ({
				id: field.id,
				key: field.key,
				label: field.label,
				type: field.type,
				required: field.required,
				hidden: field.hidden,
				sortOrder: field.order
			})),
			views: (payload.views || current.views).map((view) => ({
				id: view.id,
				name: view.name,
				type: view.type,
				config: {
					groupByFieldKey: view.groupByFieldKey,
					visibleFieldKeys: view.visibleFieldKeys,
					modalVisibleFieldKeys: view.modalVisibleFieldKeys,
					hiddenColumnIds: view.hiddenColumnIds,
					showAggregation: view.showAggregation,
					sortBy: view.sortBy,
					sortDirection: view.sortDirection
				},
				sortOrder: 0
			})),
			createdAt: current.createdAt,
			updatedAt: new Date().toISOString()
		}, {
			...current,
			responsibles: payload.responsibles || current.responsibles,
			types: payload.types || current.types,
			filters: payload.filters || current.filters,
			cardFields: payload.cardFields || current.cardFields,
			defaults: payload.defaults || current.defaults,
			views: payload.views || current.views,
			activeViewId: payload.activeViewId || current.activeViewId,
			fields: payload.fields || current.fields,
			columns: payload.columns || current.columns
		}, tasks.value.filter((task) => task.projectId === current.id))
		nextProject.responsibles = uniqueStrings(payload.responsibles || current.responsibles)
		nextProject.types = uniqueStrings(payload.types || current.types)
		nextProject.filters = defaultFiltersConfig(payload.filters || current.filters)
		nextProject.cardFields = defaultCardFieldsConfig(payload.cardFields || current.cardFields)
		nextProject.defaults = defaultProjectDefaults(payload.defaults || current.defaults)
		nextProject.views = payload.views ? [...payload.views] : nextProject.views
		nextProject.fields = payload.fields ? [...payload.fields] : nextProject.fields
		nextProject.columns = payload.columns ? sortColumns(payload.columns.map((column) => ({ ...column }))) : nextProject.columns
		nextProject.statuses = nextProject.columns.map((column) => column.label)
		nextProject.activeViewId = normalizeText(payload.activeViewId || nextProject.activeViewId, 80) || nextProject.views[0]?.id || 'view-board'
		replaceProject(nextProject)
		return nextProject
	}

	function nextOrder(projectId: string, status: string, excludeTaskId?: string, targetIndex?: number) {
		const list = tasks.value
			.filter((task) => task.projectId === projectId && normalizeKey(task.status) === normalizeKey(status) && task.id !== excludeTaskId && !task.archived)
			.sort((left, right) => left.order - right.order)
		const normalizedIndex = Number.isFinite(Number(targetIndex))
			? Math.max(0, Math.min(Number(targetIndex), list.length))
			: list.length
		const before = normalizedIndex > 0 ? list[normalizedIndex - 1] : null
		const after = normalizedIndex < list.length ? list[normalizedIndex] : null
		if (before && after) {
			return before.order + (after.order - before.order) / 2
		}
		if (before) {
			return before.order + ORDER_STEP
		}
		if (after) {
			return Math.max(ORDER_STEP / 2, after.order / 2)
		}
		return ORDER_STEP
	}

	function resolveResponsibleUserId(currentTask: TasksStoreTaskItem | undefined, nextResponsible: unknown) {
		const normalized = normalizeText(nextResponsible, 80)
		if (!normalized) {
			return null
		}
		if (looksLikeUUID(normalized)) {
			return normalized
		}
		if (currentTask?.responsibleUserId && normalizeText(currentTask.responsible, 120) === normalized) {
			return currentTask.responsibleUserId
		}
		if (normalizeText(auth.user?.id, 80) && normalizeText(auth.user?.name, 120) === normalized) {
			return normalizeText(auth.user?.id, 80)
		}
		return null
	}

	async function createTask(payload: Partial<Omit<TaskItem, 'id' | 'createdAt' | 'updatedAt'>> = {}) {
		const projectId = normalizeText(payload.projectId, 80) || activeProjectId.value || projects.value[0]?.id || ''
		const project = projects.value.find((item) => item.id === projectId)
		if (!project) {
			return null
		}
		const status = normalizeText(payload.status, 120) || project.statuses[0] || 'A fazer'
		const column = findColumnByStatus(project, status)
		const response = await request(`/v1/tasks/boards/${encodeURIComponent(project.id)}/tasks`, {
			method: 'POST',
			body: {
				columnId: column?.id || null,
				title: normalizeText(payload.title, 220) || 'Nova task',
				contentHtml: normalizeText(payload.contentHtml || payload.description, 50000),
				status,
				priority: normalizePriority(payload.priority),
				dueDate: toOptionalDateTime(payload.dueDate),
				sortOrder: Number.isFinite(Number(payload.order)) ? Number(payload.order) : nextOrder(project.id, status),
				responsibleUserId: resolveResponsibleUserId(undefined, payload.responsible),
				clientAccountId: looksLikeUUID((payload as TasksStoreTaskItem).clientAccountId) ? (payload as TasksStoreTaskItem).clientAccountId : null
			}
		})
		const task = response?.task as BackendTask | undefined
		if (!task?.id) {
			return null
		}
		const mapped = mapTaskToStoreItem(task, project)
		replaceTask(mapped)
		const refreshedProject = buildProject({
			id: project.id,
			name: project.name,
			description: project.description,
			icon: project.icon,
			columns: project.columns.map((column) => ({ id: column.id, label: column.label, color: column.color, sortOrder: column.order })),
			fields: project.fields.map((field) => ({ id: field.id, key: field.key, label: field.label, type: field.type, required: field.required, hidden: field.hidden, sortOrder: field.order })),
			views: project.views.map((view) => ({ id: view.id, name: view.name, type: view.type, config: { groupByFieldKey: view.groupByFieldKey } })),
			createdAt: project.createdAt,
			updatedAt: new Date().toISOString()
		}, project, tasks.value.filter((item) => item.projectId === project.id))
		replaceProject({ ...refreshedProject, filters: project.filters, cardFields: project.cardFields, defaults: project.defaults })
		return mapped
	}

	async function updateTask(taskId: string, patch: Partial<Omit<TaskItem, 'id' | 'projectId' | 'createdAt'>>) {
		const currentTask = tasks.value.find((item) => item.id === normalizeText(taskId, 80))
		if (!currentTask) {
			return null
		}
		const project = projects.value.find((item) => item.id === currentTask.projectId)
		if (!project) {
			return null
		}
		const nextStatus = Object.prototype.hasOwnProperty.call(patch, 'status')
			? normalizeText(patch.status, 120)
			: currentTask.status
		const nextColumn = nextStatus ? findColumnByStatus(project, nextStatus) : (currentTask.columnId ? project.columns.find((column) => column.id === currentTask.columnId) || null : null)
		const headers = currentTask.version ? { 'If-Match': String(currentTask.version) } : undefined
		const response = await request(`/v1/tasks/${encodeURIComponent(currentTask.id)}`, {
			method: 'PATCH',
			headers,
			body: {
				columnId: Object.prototype.hasOwnProperty.call(patch, 'status') || Object.prototype.hasOwnProperty.call(patch, 'columnId')
					? (nextColumn?.id || (patch as TasksStoreTaskItem).columnId || null)
					: undefined,
				title: Object.prototype.hasOwnProperty.call(patch, 'title') ? normalizeText(patch.title, 220) : undefined,
				contentHtml: Object.prototype.hasOwnProperty.call(patch, 'contentHtml') || Object.prototype.hasOwnProperty.call(patch, 'description')
					? normalizeText(patch.contentHtml || patch.description, 50000)
					: undefined,
				status: Object.prototype.hasOwnProperty.call(patch, 'status') ? (nextStatus || null) : undefined,
				priority: Object.prototype.hasOwnProperty.call(patch, 'priority') ? normalizePriority(patch.priority) : undefined,
				dueDate: Object.prototype.hasOwnProperty.call(patch, 'dueDate') ? (toOptionalDateTime(patch.dueDate) || null) : undefined,
				archived: Object.prototype.hasOwnProperty.call(patch, 'archived') ? Boolean(patch.archived) : undefined,
				sortOrder: Object.prototype.hasOwnProperty.call(patch, 'order') ? Number(patch.order || 0) : undefined,
				responsibleUserId: Object.prototype.hasOwnProperty.call(patch, 'responsible')
					? resolveResponsibleUserId(currentTask, patch.responsible)
					: undefined,
				clientAccountId: Object.prototype.hasOwnProperty.call(patch, 'clientName') || Object.prototype.hasOwnProperty.call(patch, 'clientId') || Object.prototype.hasOwnProperty.call(patch as object, 'clientAccountId')
					? (looksLikeUUID((patch as TasksStoreTaskItem).clientAccountId) ? (patch as TasksStoreTaskItem).clientAccountId : null)
					: undefined
			}
		})
		const task = response?.task as BackendTask | undefined
		if (!task?.id) {
			return null
		}
		const mapped = mapTaskToStoreItem(task, project)
		replaceTask({
			...mapped,
			responsible: Object.prototype.hasOwnProperty.call(patch, 'responsible') && !mapped.responsibleUserId
				? normalizeText(patch.responsible, 120)
				: mapped.responsible,
			clientName: Object.prototype.hasOwnProperty.call(patch, 'clientName') && !mapped.clientAccountId
				? normalizeText(patch.clientName, 140)
				: mapped.clientName,
			type: Object.prototype.hasOwnProperty.call(patch, 'type') ? normalizeText(patch.type, 120) : mapped.type
		})
		return tasks.value.find((item) => item.id === mapped.id) || null
	}

	async function removeTask(taskId: string) {
		const currentTask = tasks.value.find((item) => item.id === normalizeText(taskId, 80))
		if (!currentTask) {
			return false
		}
		await request(`/v1/tasks/${encodeURIComponent(currentTask.id)}`, {
			method: 'DELETE'
		})
		tasks.value = tasks.value.filter((item) => item.id !== currentTask.id)
		return true
	}

	async function toggleArchiveTask(taskId: string) {
		const currentTask = tasks.value.find((item) => item.id === normalizeText(taskId, 80))
		if (!currentTask) {
			return null
		}
		return updateTask(currentTask.id, { archived: !currentTask.archived })
	}

	async function moveTaskToStatus(taskId: string, status: string, targetIndex?: number) {
		const currentTask = tasks.value.find((item) => item.id === normalizeText(taskId, 80))
		if (!currentTask) {
			return null
		}
		const project = projects.value.find((item) => item.id === currentTask.projectId)
		if (!project) {
			return null
		}
		const targetColumn = findColumnByStatus(project, status)
		const sortOrder = nextOrder(project.id, status, currentTask.id, targetIndex)
		const headers = currentTask.version ? { 'If-Match': String(currentTask.version) } : undefined
		const response = await request(`/v1/tasks/${encodeURIComponent(currentTask.id)}/move`, {
			method: 'POST',
			headers,
			body: {
				columnId: targetColumn?.id || null,
				sortOrder
			}
		})
		const task = response?.task as BackendTask | undefined
		if (!task?.id) {
			return null
		}
		const mapped = mapTaskToStoreItem({ ...task, status }, project)
		replaceTask(mapped)
		return mapped
	}

	async function createColumn(projectId: string, rawLabel: unknown) {
		const project = projects.value.find((item) => item.id === normalizeText(projectId, 80))
		if (!project) {
			return null
		}
		const label = normalizeText(rawLabel, 120) || `Nova coluna ${project.columns.length + 1}`
		const response = await request(`/v1/tasks/boards/${encodeURIComponent(project.id)}/columns`, {
			method: 'POST',
			body: {
				label,
				color: 'slate',
				sortOrder: (project.columns.length + 1) * 100
			}
		})
		const column = response?.column as BackendColumn | undefined
		if (!column?.id) {
			return null
		}
		const nextProject = {
			...project,
			columns: sortColumns([
				...project.columns,
				{
					id: normalizeText(column.id, 80),
					label: normalizeText(column.label, 120) || label,
					color: normalizeColumnColor(column.color),
					order: Number(column.sortOrder || (project.columns.length + 1) * 100) || (project.columns.length + 1) * 100
				}
			])
		}
		nextProject.statuses = nextProject.columns.map((item) => item.label)
		replaceProject(nextProject)
		return nextProject.columns.find((item) => item.id === normalizeText(column.id, 80)) || null
	}

	async function updateColumn(projectId: string, columnId: string, patch: Partial<TaskBoardColumn>) {
		const project = projects.value.find((item) => item.id === normalizeText(projectId, 80))
		const currentColumn = project?.columns.find((item) => item.id === normalizeText(columnId, 80))
		if (!project || !currentColumn) {
			return null
		}
		const response = await request(`/v1/tasks/columns/${encodeURIComponent(currentColumn.id)}`, {
			method: 'PATCH',
			body: {
				label: Object.prototype.hasOwnProperty.call(patch, 'label') ? normalizeText(patch.label, 120) : undefined,
				color: Object.prototype.hasOwnProperty.call(patch, 'color') ? normalizeColumnColor(patch.color) : undefined,
				sortOrder: Object.prototype.hasOwnProperty.call(patch, 'order') ? Number(patch.order || 0) : undefined
			}
		})
		const column = response?.column as BackendColumn | undefined
		if (!column?.id) {
			return null
		}
		const nextProject = {
			...project,
			columns: sortColumns(project.columns.map((item) => item.id === currentColumn.id
				? {
					id: normalizeText(column.id, 80),
					label: normalizeText(column.label, 120) || item.label,
					color: normalizeColumnColor(column.color || item.color),
					order: Number(column.sortOrder || item.order) || item.order
				}
				: item))
		}
		nextProject.statuses = nextProject.columns.map((item) => item.label)
		replaceProject(nextProject)
		return nextProject.columns.find((item) => item.id === currentColumn.id) || null
	}

	async function deleteColumn(projectId: string, columnId: string, fallbackColumnId?: string) {
		const project = projects.value.find((item) => item.id === normalizeText(projectId, 80))
		const currentColumn = project?.columns.find((item) => item.id === normalizeText(columnId, 80))
		if (!project || !currentColumn) {
			return false
		}
		const remapToColumnId = normalizeText(fallbackColumnId, 80) || project.columns.find((item) => item.id !== currentColumn.id)?.id || ''
		await request(`/v1/tasks/columns/${encodeURIComponent(currentColumn.id)}`, {
			method: 'DELETE',
			body: {
				remapToColumnId: remapToColumnId || undefined
			}
		})
		projects.value = projects.value.map((item) => {
			if (item.id !== project.id) {
				return item
			}
			const columns = item.columns.filter((column) => column.id !== currentColumn.id)
			return {
				...item,
				columns,
				statuses: columns.map((column) => column.label)
			}
		})
		if (remapToColumnId) {
			const fallbackColumn = project.columns.find((item) => item.id === remapToColumnId)
			tasks.value = tasks.value.map((task) => task.projectId === project.id && task.columnId === currentColumn.id
				? { ...task, columnId: remapToColumnId, status: fallbackColumn?.label || task.status, updatedAt: new Date().toISOString() }
				: task)
		}
		return true
	}

	async function moveColumn(projectId: string, columnId: string, targetIndex: number) {
		const project = projects.value.find((item) => item.id === normalizeText(projectId, 80))
		const currentIndex = project?.columns.findIndex((item) => item.id === normalizeText(columnId, 80)) ?? -1
		if (!project || currentIndex < 0) {
			return false
		}
		const reordered = [...project.columns]
		const [moving] = reordered.splice(currentIndex, 1)
		if (!moving) {
			return false
		}
		const normalizedIndex = Math.max(0, Math.min(targetIndex, reordered.length))
		reordered.splice(normalizedIndex, 0, moving)
		const nextColumns = reordered.map((item, index) => ({ ...item, order: (index + 1) * 100 }))
		await Promise.all(nextColumns.map((column) => request(`/v1/tasks/columns/${encodeURIComponent(column.id)}`, {
			method: 'PATCH',
			body: { sortOrder: column.order }
		})))
		replaceProject({
			...project,
			columns: nextColumns,
			statuses: nextColumns.map((item) => item.label)
		})
		return true
	}

	return {
		initialized,
		initializing,
		pending,
		errorMessage,
		projects,
		tasks,
		activeProjectId,
		legacyMigrationNotice,
		initialize,
		refresh,
		setActiveProject,
		createProject,
		deleteProject,
		saveProjectSettings,
		createTask,
		updateTask,
		removeTask,
		toggleArchiveTask,
		moveTaskToStatus,
		createColumn,
		updateColumn,
		deleteColumn,
		moveColumn,
		dismissLegacyMigrationNotice
	}
})