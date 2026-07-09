package calendar

import (
	"context"
	"html"
	"regexp"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/tasks"
)

var (
	taskContextBreakRe = regexp.MustCompile(`(?i)<\s*(br|/p|/div|/li)\b[^>]*>`)
	taskContextTagRe   = regexp.MustCompile(`(?s)<[^>]*>`)
)

func (s *Service) chatTasksContext(ctx context.Context, accountID string, principal auth.Principal, clientID string, visibleClientIDs []string) []AIContextTask {
	tasksvc := s.tasksSvc()
	if tasksvc == nil {
		return nil
	}
	account := strings.TrimSpace(accountID)
	cfg, err := s.store.GetConfig(ctx, account)
	if err != nil {
		s.logTaskWarn(ctx, "calendar: carregar config para contexto de tasks falhou", account, "", err)
		return nil
	}
	boardID := strings.TrimSpace(cfg.Tasks.BoardID)
	if boardID == "" {
		return nil
	}
	access, err := tasksvc.ResolveAccessContext(ctx, principal, account)
	if err != nil {
		s.logTaskWarn(ctx, "calendar: resolver acesso a tasks no contexto do chat falhou", account, "", err)
		return nil
	}
	result, err := tasksvc.ListTasks(ctx, access, tasks.ListTasksInput{
		BoardID:         boardID,
		Limit:           maxContextTasks,
		IncludeArchived: false,
	})
	if err != nil {
		s.logTaskWarn(ctx, "calendar: listar tasks para contexto do chat falhou", account, "", err)
		return nil
	}
	allowedClients := contextClientSet(visibleClientIDs)
	targetClientID := normalizeUUID(clientID)
	out := make([]AIContextTask, 0, len(result.Tasks))
	for _, task := range result.Tasks {
		item := aiContextTaskFromDTO(task)
		if !taskAllowedInChatContext(item.ClientID, targetClientID, allowedClients, access.Perspective) {
			continue
		}
		out = append(out, item)
	}
	return out
}

func aiContextTaskFromDTO(task tasks.TaskDTO) AIContextTask {
	item := AIContextTask{
		ID:            strings.TrimSpace(task.ID),
		BoardID:       strings.TrimSpace(task.BoardID),
		ColumnID:      ptrToStr(task.ColumnID),
		Title:         strings.TrimSpace(task.Title),
		Status:        ptrToStr(task.Status),
		Priority:      strings.TrimSpace(task.Priority),
		DueDate:       ptrToStr(task.DueDate),
		StartDate:     ptrToStr(task.StartDate),
		ResponsibleID: ptrToStr(task.ResponsibleUserID),
		ClientID:      normalizeUUID(ptrToStr(task.ClientAccountID)),
		Description:   taskContextHTMLToText(task.ContentHTML),
		Archived:      task.Archived,
		Version:       task.Version,
	}
	if item.ResponsibleID == "" && task.Responsible != nil {
		item.ResponsibleID = strings.TrimSpace(task.Responsible.ID)
	}
	item.Type = taskContextMetadataString(task.UIMetadata, "type")
	item.DueEndDate = taskContextMetadataString(task.UIMetadata, "dueEndDate")
	item.ClientName = taskContextMetadataString(task.UIMetadata, "clientName")
	item.InvolvedIDs = taskContextMetadataStrings(task.UIMetadata, "involved")
	if item.ClientID == "" {
		item.ClientID = normalizeUUID(taskContextMetadataString(task.UIMetadata, "clientId"))
	}
	return item
}

func contextClientSet(ids []string) map[string]bool {
	out := map[string]bool{}
	for _, raw := range ids {
		if id := normalizeUUID(raw); id != "" {
			out[id] = true
		}
	}
	return out
}

func taskAllowedInChatContext(clientID, targetClientID string, allowedClients map[string]bool, perspective tasks.Perspective) bool {
	if perspective == tasks.PerspectiveClientViewer {
		return true
	}
	if targetClientID != "" {
		return clientID == targetClientID
	}
	if clientID == "" || len(allowedClients) == 0 {
		return true
	}
	return allowedClients[clientID]
}

func taskContextMetadataString(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	value, ok := metadata[key]
	if !ok {
		return ""
	}
	s, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

func taskContextMetadataStrings(metadata map[string]any, key string) []string {
	if metadata == nil {
		return nil
	}
	value, ok := metadata[key]
	if !ok {
		return nil
	}
	switch typed := value.(type) {
	case []string:
		return compactTaskContextStrings(typed)
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return compactTaskContextStrings(out)
	default:
		return nil
	}
}

func compactTaskContextStrings(items []string) []string {
	out := make([]string, 0, len(items))
	seen := map[string]bool{}
	for _, raw := range items {
		item := strings.TrimSpace(raw)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}

func taskContextHTMLToText(value string) string {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return ""
	}
	text := taskContextBreakRe.ReplaceAllString(raw, "\n")
	text = taskContextTagRe.ReplaceAllString(text, " ")
	text = html.UnescapeString(text)
	text = strings.Join(strings.Fields(text), " ")
	return truncateRunes(text, 500)
}
