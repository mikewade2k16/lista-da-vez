package tasks

import (
	"context"
	"fmt"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/notifications"
)

func (service *Service) ensureTaskSubscribers(ctx context.Context, accountID, taskID string, userIDs ...string) {
	if len(userIDs) == 0 {
		return
	}
	_ = service.repository.UpsertSubscribers(ctx, accountID, taskID, userIDs)
}

func (service *Service) persistCommentMentions(ctx context.Context, accountID, taskID, commentID string, mentionedUserIDs []string) []string {
	mentionedUserIDs = uniqueUserIDs(mentionedUserIDs...)
	if len(mentionedUserIDs) == 0 {
		return nil
	}
	persisted, err := service.repository.AddCommentMentions(ctx, accountID, taskID, commentID, mentionedUserIDs)
	if err != nil {
		return nil
	}
	return persisted
}

func (service *Service) notifyTaskAssigned(ctx context.Context, access AccessContext, task Task, responsibleUserID string) {
	responsibleUserID = strings.TrimSpace(responsibleUserID)
	if responsibleUserID == "" || responsibleUserID == access.UserID {
		return
	}
	_ = service.notifier.Dispatch(ctx, notifications.DispatchInput{
		AccountID:    access.AccountID,
		UserIDs:      []string{responsibleUserID},
		SourceModule: "tasks",
		SourceEvent:  "task.assigned",
		Title:        "Nova task atribuida",
		Body:         taskAssignedBody(task),
		LinkPath:     taskNotificationLink(task),
		Payload:      taskNotificationPayload(task, nil),
		ResourceType: "task",
		ResourceID:   task.ID,
	})
}

func (service *Service) notifyTaskSubscribers(ctx context.Context, access AccessContext, task Task, sourceEvent, title, body string, excludeUserIDs ...string) {
	subscriberUserIDs, err := service.repository.ListSubscriberUserIDs(ctx, access.AccountID, task.ID)
	if err != nil {
		return
	}
	recipients := uniqueUserIDsExcluding(subscriberUserIDs, excludeUserIDs...)
	if len(recipients) == 0 {
		return
	}
	_ = service.notifier.Dispatch(ctx, notifications.DispatchInput{
		AccountID:    access.AccountID,
		UserIDs:      recipients,
		SourceModule: "tasks",
		SourceEvent:  sourceEvent,
		Title:        title,
		Body:         body,
		LinkPath:     taskNotificationLink(task),
		Payload:      taskNotificationPayload(task, nil),
		ResourceType: "task",
		ResourceID:   task.ID,
	})
}

func (service *Service) notifyTaskMentions(ctx context.Context, access AccessContext, task Task, comment Comment, mentionedUserIDs []string) {
	recipients := uniqueUserIDsExcluding(mentionedUserIDs, access.UserID)
	if len(recipients) == 0 {
		return
	}
	_ = service.notifier.Dispatch(ctx, notifications.DispatchInput{
		AccountID:    access.AccountID,
		UserIDs:      recipients,
		SourceModule: "tasks",
		SourceEvent:  "task.comment_mentioned",
		Title:        "Voce foi mencionado em uma task",
		Body:         taskMentionBody(task),
		LinkPath:     taskNotificationLink(task),
		Payload:      taskNotificationPayload(task, map[string]any{"commentId": comment.ID}),
		ResourceType: "task",
		ResourceID:   task.ID,
	})
}

func taskNotificationLink(task Task) string {
	return fmt.Sprintf("/tasks?boardId=%s&taskId=%s", task.BoardID, task.ID)
}

func taskNotificationPayload(task Task, extra map[string]any) map[string]any {
	payload := map[string]any{
		"taskId":  task.ID,
		"boardId": task.BoardID,
		"title":   task.Title,
	}
	for key, value := range extra {
		payload[key] = value
	}
	return payload
}

func taskAssignedBody(task Task) string {
	return fmt.Sprintf("A task \"%s\" foi atribuida a voce.", task.Title)
}

func taskMentionBody(task Task) string {
	return fmt.Sprintf("Ha um comentario com mencao na task \"%s\".", task.Title)
}

func taskCommentBody(task Task) string {
	return fmt.Sprintf("A task \"%s\" recebeu um novo comentario.", task.Title)
}

func taskMovedBody(task Task) string {
	return fmt.Sprintf("A task \"%s\" foi movida de status.", task.Title)
}

func taskStatusChangedBody(task Task) string {
	return fmt.Sprintf("A task \"%s\" teve o status alterado.", task.Title)
}

func optionalStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func uniqueUserIDs(values ...string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func uniqueUserIDsExcluding(values []string, exclude ...string) []string {
	excluded := make(map[string]struct{}, len(exclude))
	for _, value := range exclude {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			excluded[trimmed] = struct{}{}
		}
	}
	result := make([]string, 0, len(values))
	for _, value := range uniqueUserIDs(values...) {
		if _, ok := excluded[value]; ok {
			continue
		}
		result = append(result, value)
	}
	return result
}
