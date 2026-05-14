package tasks

const (
	PermBoardsView      = "tasks.boards.view"
	PermBoardsManage    = "tasks.boards.manage"
	PermTasksView       = "tasks.tasks.view"
	PermTasksCreate     = "tasks.tasks.create"
	PermTasksEdit       = "tasks.tasks.edit"
	PermTasksDelete     = "tasks.tasks.delete"
	PermTasksAssign     = "tasks.tasks.assign"
	PermTasksComment    = "tasks.tasks.comment"
	PermTrackingUse     = "tasks.tracking.use"
	PermTrackingViewAll = "tasks.tracking.view_all"
	PermRelationsManage = "tasks.relations.manage"
	PermSharesManage    = "tasks.shares.manage"
	PermClientView      = "tasks.client_view"
)

var adminPermissions = []string{
	PermBoardsView,
	PermBoardsManage,
	PermTasksView,
	PermTasksCreate,
	PermTasksEdit,
	PermTasksDelete,
	PermTasksAssign,
	PermTasksComment,
	PermTrackingUse,
	PermTrackingViewAll,
	PermRelationsManage,
	PermSharesManage,
}

var memberPermissions = []string{
	PermBoardsView,
	PermTasksView,
	PermTasksCreate,
	PermTasksEdit,
	PermTasksAssign,
	PermTasksComment,
	PermTrackingUse,
	PermRelationsManage,
}

var clientViewerPermissions = []string{
	PermClientView,
	PermTasksComment,
}
