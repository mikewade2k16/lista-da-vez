import { storeToRefs } from 'pinia'
import { useTasksStore } from '../stores/tasks'

export function useTasksWorkspace() {
	const store = useTasksStore()
	const {
		initialized,
		initializing,
		pending,
		errorMessage,
		projects,
		tasks,
		activeProjectId,
		legacyMigrationNotice
	} = storeToRefs(store)

	return {
		initialized,
		initializing,
		pending,
		errorMessage,
		projects,
		tasks,
		activeProjectId,
		legacyMigrationNotice,
		initialize: store.initialize,
		refresh: store.refresh,
		setActiveProject: store.setActiveProject,
		createProject: store.createProject,
		deleteProject: store.deleteProject,
		saveProjectSettings: store.saveProjectSettings,
		createTask: store.createTask,
		updateTask: store.updateTask,
		removeTask: store.removeTask,
		toggleArchiveTask: store.toggleArchiveTask,
		moveTaskToStatus: store.moveTaskToStatus,
		createColumn: store.createColumn,
		updateColumn: store.updateColumn,
		deleteColumn: store.deleteColumn,
		moveColumn: store.moveColumn,
		dismissLegacyMigrationNotice: store.dismissLegacyMigrationNotice
	}
}
