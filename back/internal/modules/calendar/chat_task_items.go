package calendar

import (
	"strings"
	"time"
)

// ChatProposalTaskItem e o payload fechado de kind=taskItem. A task-pai fica em
// fields.targetId; TaskTitle e ItemTitle sao snapshots preenchidos pelo backend.
type ChatProposalTaskItem struct {
	ID            string `json:"id,omitempty"`
	Title         string `json:"title,omitempty"`
	ItemTitle     string `json:"itemTitle,omitempty"`
	TaskTitle     string `json:"taskTitle,omitempty"`
	Status        string `json:"status,omitempty"`
	StatusDate    string `json:"statusDate,omitempty"`
	Completed     *bool  `json:"completed,omitempty"`
	CompletedDate string `json:"completedDate,omitempty"`
}

func normalizeTaskItemField(item *ChatProposalTaskItem) {
	if item == nil {
		return
	}
	item.ID = strings.TrimSpace(item.ID)
	item.Title = strings.TrimSpace(item.Title)
	item.ItemTitle = strings.TrimSpace(item.ItemTitle)
	item.TaskTitle = strings.TrimSpace(item.TaskTitle)
	item.Status = strings.TrimSpace(item.Status)
	if !validTaskItemStatus(item.Status) {
		item.Status = ""
	}
	item.StatusDate = normalizeTaskItemDate(item.StatusDate)
	item.CompletedDate = normalizeTaskItemDate(item.CompletedDate)
}

// validRawTaskItemField distingue campo omitido de valor invalido antes que a
// normalizacao o limpe. Assim um status/data malformado nao degrada silenciosamente
// para outra alteracao nem aciona o fallback de "hoje".
func validRawTaskItemField(item *ChatProposalTaskItem) bool {
	if item == nil {
		return true
	}
	if status := strings.TrimSpace(item.Status); status != "" && !validTaskItemStatus(status) {
		return false
	}
	for _, date := range []string{item.StatusDate, item.CompletedDate} {
		if strings.TrimSpace(date) != "" && normalizeTaskItemDate(date) == "" {
			return false
		}
	}
	return true
}

func validTaskItemStatus(status string) bool {
	switch status {
	case "captured", "editing", "approval", "approved", "scheduled", "posted":
		return true
	default:
		return false
	}
}

func normalizeTaskItemDate(raw string) string {
	date := strings.TrimSpace(raw)
	parsed, err := time.Parse(time.DateOnly, date)
	if err != nil || parsed.Format(time.DateOnly) != date {
		return ""
	}
	return date
}

func sanitizeTaskItemProposal(action string, fields ChatProposalFields) bool {
	item := fields.TaskItem
	if item == nil || strings.TrimSpace(fields.TargetID) == "" {
		return false
	}
	switch action {
	case "create":
		if item.StatusDate != "" && item.Status == "" {
			return false
		}
		if item.CompletedDate != "" && (item.Completed == nil || !*item.Completed) {
			return false
		}
		return item.Title != ""
	case "update":
		return (item.ID != "" || item.ItemTitle != "") && taskItemHasEditableField(item)
	case "delete":
		return item.ID != "" || item.ItemTitle != ""
	default:
		return false
	}
}

func taskItemHasEditableField(item *ChatProposalTaskItem) bool {
	return item != nil && (item.Title != "" || item.Status != "" || item.StatusDate != "" ||
		item.Completed != nil || item.CompletedDate != "")
}

// resolveTaskItemProposals resolve task/item exclusivamente no contexto autorizado.
// Alvo inexistente, inventado ou ambiguo e descartado; nunca chega ao card confirmavel.
func resolveTaskItemProposals(proposals []ChatProposal, tasks []AIContextTask, now time.Time) ([]ChatProposal, int) {
	out := make([]ChatProposal, 0, len(proposals))
	dropped := 0
	for _, proposal := range proposals {
		if proposal.Kind != "taskItem" {
			out = append(out, proposal)
			continue
		}
		task := resolveTaskItemParent(proposal.Fields.TargetID, tasks)
		if task == nil || proposal.Fields.TaskItem == nil {
			dropped++
			continue
		}
		proposal.Fields.TargetID = task.ID
		proposal.Fields.TaskItem.TaskTitle = task.Title
		item := proposal.Fields.TaskItem
		if proposal.Action == "create" {
			item.ID = ""
			item.ItemTitle = ""
			fillTaskItemActionDates(item, nil, now)
			out = append(out, proposal)
			continue
		}
		current := resolveTaskItemTarget(item, task.Items)
		if current == nil {
			dropped++
			continue
		}
		if proposal.Action == "update" && !taskItemDatesMatchState(item, current) {
			dropped++
			continue
		}
		item.ID = current.ID
		item.ItemTitle = current.Title
		if proposal.Action == "update" {
			fillTaskItemActionDates(item, current, now)
		}
		out = append(out, proposal)
	}
	return out, dropped
}

// taskItemDatesMatchState evita persistir card que so falharia ao confirmar: uma
// data isolada so e aplicavel quando o estado correspondente ja existe no item real.
func taskItemDatesMatchState(item *ChatProposalTaskItem, current *AIContextTaskItem) bool {
	if item == nil || current == nil {
		return false
	}
	if item.StatusDate != "" && item.Status == "" && current.Status == "" {
		return false
	}
	if item.CompletedDate != "" && item.Completed == nil && !current.Completed {
		return false
	}
	return true
}

func taskItemResolutionNotice(dropped int, hasOtherProposals bool) string {
	if dropped <= 0 {
		return ""
	}
	if hasOtherProposals {
		return "Preparei os outros cartoes, mas nao consegui identificar com seguranca uma tarefa ou item. Diga o titulo exato para eu montar esse cartao."
	}
	return "Nao consegui identificar com seguranca a tarefa ou o item. Diga o titulo exato para eu montar o cartao."
}

func resolveTaskItemParent(label string, tasks []AIContextTask) *AIContextTask {
	label = strings.TrimSpace(label)
	if label == "" {
		return nil
	}
	for i := range tasks {
		if tasks[i].ID == label {
			return &tasks[i]
		}
	}
	return uniqueTaskByTitle(label, tasks)
}

func uniqueTaskByTitle(label string, tasks []AIContextTask) *AIContextTask {
	needle := foldChatLabel(label)
	var exact *AIContextTask
	exactCount := 0
	for i := range tasks {
		if foldChatLabel(tasks[i].Title) == needle {
			exact = &tasks[i]
			exactCount++
		}
	}
	if exactCount == 1 {
		return exact
	}
	if exactCount > 1 {
		return nil
	}
	var match *AIContextTask
	count := 0
	for i := range tasks {
		if chatLabelsMatch(foldChatLabel(tasks[i].Title), needle) {
			match = &tasks[i]
			count++
		}
	}
	if count != 1 {
		return nil
	}
	return match
}

func resolveTaskItemTarget(selector *ChatProposalTaskItem, items []AIContextTaskItem) *AIContextTaskItem {
	if selector == nil {
		return nil
	}
	if selector.ID != "" {
		for i := range items {
			if items[i].ID == selector.ID {
				return &items[i]
			}
		}
	}
	label := selector.ItemTitle
	if label == "" {
		return nil
	}
	needle := foldChatLabel(label)
	var exact *AIContextTaskItem
	exactCount := 0
	for i := range items {
		if foldChatLabel(items[i].Title) == needle {
			exact = &items[i]
			exactCount++
		}
	}
	if exactCount == 1 {
		return exact
	}
	if exactCount > 1 {
		return nil
	}
	var match *AIContextTaskItem
	count := 0
	for i := range items {
		if chatLabelsMatch(foldChatLabel(items[i].Title), needle) {
			match = &items[i]
			count++
		}
	}
	if count != 1 {
		return nil
	}
	return match
}

func fillTaskItemActionDates(item *ChatProposalTaskItem, current *AIContextTaskItem, now time.Time) {
	if item == nil {
		return
	}
	today := now.In(saoPauloLoc).Format(time.DateOnly)
	if item.Status != "" && item.StatusDate == "" && (current == nil || item.Status != current.Status) {
		item.StatusDate = today
	}
	if item.Completed == nil {
		return
	}
	if !*item.Completed {
		item.CompletedDate = ""
		return
	}
	if item.CompletedDate == "" && (current == nil || !current.Completed) {
		item.CompletedDate = today
	}
}
