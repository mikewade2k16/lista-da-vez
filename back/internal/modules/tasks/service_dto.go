package tasks

import "time"

type UserMiniDTO struct {
	ID string `json:"id"`
}

type TaskDTO struct {
	ID                string        `json:"id"`
	BoardID           string        `json:"boardId"`
	ColumnID          *string       `json:"columnId,omitempty"`
	Title             string        `json:"title"`
	ContentHTML       string        `json:"contentHtml,omitempty"`
	Status            *string       `json:"status,omitempty"`
	Priority          string        `json:"priority"`
	DueDate           *string       `json:"dueDate,omitempty"`
	StartDate         *string       `json:"startDate,omitempty"`
	Archived          bool          `json:"archived"`
	SortOrder         float64       `json:"sortOrder"`
	Responsible       *UserMiniDTO  `json:"responsible,omitempty"`
	ResponsibleUserID *string       `json:"responsibleUserId,omitempty"`
	ClientAccountID   *string       `json:"clientAccountId,omitempty"`
	Assignees         []UserMiniDTO `json:"assignees,omitempty"`
	TrackingTotalMs   *int64        `json:"trackingTotalMs,omitempty"`
	Version           int           `json:"version"`
	CreatedAt         string        `json:"createdAt"`
	UpdatedAt         string        `json:"updatedAt"`
}

func (service *Service) BuildTaskDTO(task Task, perspective Perspective) TaskDTO {
	dto := TaskDTO{
		ID:                task.ID,
		BoardID:           task.BoardID,
		ColumnID:          task.ColumnID,
		Title:             task.Title,
		ContentHTML:       task.ContentHTML,
		Status:            task.Status,
		Priority:          task.Priority,
		DueDate:           formatISO(task.DueDate),
		StartDate:         formatISO(task.StartDate),
		Archived:          task.Archived,
		SortOrder:         task.SortOrder,
		ResponsibleUserID: task.ResponsibleUserID,
		Version:           task.Version,
		CreatedAt:         task.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:         task.UpdatedAt.UTC().Format(time.RFC3339),
	}

	if task.ResponsibleUserID != nil {
		dto.Responsible = &UserMiniDTO{ID: *task.ResponsibleUserID}
	}

	if perspective == PerspectiveAgency {
		dto.ClientAccountID = task.ClientAccountID
	}

	return dto
}

func formatISO(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339)
	return &formatted
}
