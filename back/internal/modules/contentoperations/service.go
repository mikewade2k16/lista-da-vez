package contentoperations

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

var (
	ErrForbidden = errors.New("content operations forbidden")
	ErrNotReady  = errors.New("content operations dependencies unavailable")
)

type ScopeProvider func(context.Context, string) (Scope, error)
type AccessProvider func(context.Context, auth.Principal, string) (Access, error)

type Service struct {
	repository Repository
	scope      ScopeProvider
	access     AccessProvider
	now        func() time.Time
}

func NewService(repository Repository, scope ScopeProvider, access AccessProvider) *Service {
	return &Service{repository: repository, scope: scope, access: access, now: time.Now}
}

func (s *Service) Brief(ctx context.Context, principal auth.Principal, accountID string) (Brief, error) {
	if s.scope == nil || s.access == nil {
		return Brief{}, ErrNotReady
	}
	allowed, err := s.access(ctx, principal, strings.TrimSpace(accountID))
	if err != nil {
		return Brief{}, err
	}
	if !allowed.Allowed {
		return Brief{}, ErrForbidden
	}
	scope, err := s.scope(ctx, strings.TrimSpace(accountID))
	if err != nil {
		return Brief{}, err
	}
	now := s.now().In(saoPaulo())
	from, to := now.AddDate(0, 0, -45), now.AddDate(0, 0, 21)
	tasks, events, err := s.repository.Load(ctx, scope, from, to)
	if err != nil {
		return Brief{}, err
	}
	return buildBrief(now, scope.Clients, tasks, events), nil
}

// FilterBrief restringe uma leitura ja calculada ao mesmo universo de clientes do consumidor.
// O Crow usa isto para manter exatamente o recorte permission-scoped do chat.
func FilterBrief(brief Brief, clientIDs []string) Brief {
	allowed := make(map[string]struct{}, len(clientIDs))
	for _, id := range clientIDs {
		if id = strings.TrimSpace(id); id != "" {
			allowed[id] = struct{}{}
		}
	}
	brief.Alerts = filterAlerts(brief.Alerts, allowed)
	clients := make([]ClientHealth, 0, len(brief.Clients))
	for _, client := range brief.Clients {
		if _, ok := allowed[client.ClientID]; ok {
			clients = append(clients, client)
		}
	}
	brief.Clients = clients
	brief.Counts = Counts{}
	for _, alert := range brief.Alerts {
		switch alert.Severity {
		case SeverityCritical:
			brief.Counts.Critical++
		case SeverityAttention:
			brief.Counts.Attention++
		default:
			brief.Counts.Info++
		}
	}
	brief.Counts.Total = len(brief.Alerts)
	return brief
}

// PrepareCrowBrief limita detalhes para o prompt sem alterar os contadores totais.
func PrepareCrowBrief(brief Brief) Brief {
	const maxCrowAlerts = 20
	if len(brief.Alerts) > maxCrowAlerts {
		brief.Alerts = append([]Alert(nil), brief.Alerts[:maxCrowAlerts]...)
	}
	return brief
}

func filterAlerts(alerts []Alert, allowed map[string]struct{}) []Alert {
	filtered := make([]Alert, 0, len(alerts))
	for _, alert := range alerts {
		if _, ok := allowed[alert.ClientID]; ok {
			filtered = append(filtered, alert)
		}
	}
	return filtered
}

func saoPaulo() *time.Location {
	location, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		return time.FixedZone("America/Sao_Paulo", -3*60*60)
	}
	return location
}

func buildBrief(now time.Time, clients []ScopeClient, tasks []TaskSnapshot, events []EventSnapshot) Brief {
	today := dateOnly(now)
	mode := "follow_up"
	switch now.Weekday() {
	case time.Monday:
		mode = "planning"
	case time.Friday, time.Saturday, time.Sunday:
		mode = "closing"
	}
	clientNames := map[string]string{}
	for _, client := range clients {
		name := strings.TrimSpace(client.Name)
		if name == "" {
			name = "Cliente"
		}
		clientNames[client.ID] = name
	}
	alerts := make([]Alert, 0)
	lastPosted := map[string]time.Time{}
	futureContent := map[string]bool{}
	pipeline := map[string]bool{}
	recentBoundary := dateOnly(now.AddDate(0, 0, -14))

	for _, event := range events {
		kind, status := strings.ToLower(strings.TrimSpace(event.Type)), strings.ToLower(strings.TrimSpace(event.Status))
		isContent := kind == "post" || kind == "story" || kind == "reels"
		eventDay := dateOnly(event.Date)
		if isContent && status == "publicado" && eventDay <= today && event.Date.After(lastPosted[event.ClientID]) {
			lastPosted[event.ClientID] = event.Date
		}
		if isContent && eventDay > today && status != "publicado" {
			futureContent[event.ClientID] = true
		}
		if isContent && eventDay < today && eventDay >= recentBoundary && status != "publicado" {
			alerts = append(alerts, makeAlert("calendar_unconfirmed", SeverityCritical, event.ClientID, clientNames[event.ClientID], "Confirmar postagem", fmt.Sprintf("%s estava previsto para %s e ainda não foi marcado como publicado.", event.Title, event.Date.Format("02/01")), "calendar", event.ID, dateOnly(event.Date), "/calendario?date="+dateOnly(event.Date)))
		}
		if kind == "gravacao" && eventDay < today && eventDay >= recentBoundary && event.TaskID == "" {
			alerts = append(alerts, makeAlert("capture_unregistered", SeverityAttention, event.ClientID, clientNames[event.ClientID], "Registrar o que foi gravado", fmt.Sprintf("A captação de %s em %s ainda não está ligada a uma task.", clientNames[event.ClientID], event.Date.Format("02/01")), "calendar", event.ID, dateOnly(event.Date), "/calendario?date="+dateOnly(event.Date)))
		}
	}

	for _, task := range tasks {
		for _, item := range task.Items {
			status := strings.ToLower(strings.TrimSpace(item.Status))
			if status != "" && status != "posted" {
				pipeline[task.ClientID] = true
			}
			var severity Severity
			var title, body string
			switch status {
			case "approved", "scheduled":
				severity, title, body = SeverityCritical, "Conteúdo pronto para postar", fmt.Sprintf("%s, na task %s, já está %s.", item.Title, task.Title, statusLabel(status))
			case "captured":
				severity, title, body = SeverityAttention, "Captação aguardando edição", fmt.Sprintf("%s foi gravado e precisa entrar em edição.", item.Title)
			case "editing":
				if itemAgeDays(now, item.StatusDate, task.Updated) < 3 {
					continue
				}
				severity, title, body = SeverityAttention, "Edição precisa ser finalizada", fmt.Sprintf("%s está em edição há alguns dias.", item.Title)
			case "approval":
				severity, title, body = SeverityAttention, "Conteúdo aguardando aprovação", fmt.Sprintf("%s está em aprovação.", item.Title)
			default:
				continue
			}
			alerts = append(alerts, makeAlert("task_item_"+status, severity, task.ClientID, clientNames[task.ClientID], title, body, "task", task.ID, item.StatusDate, "/tasks?taskId="+task.ID))
		}
	}

	for _, client := range clients {
		last := lastPosted[client.ID]
		withoutRecentPost := last.IsZero() || now.Sub(last) >= 7*24*time.Hour
		withoutPipeline := !pipeline[client.ID] && !futureContent[client.ID]
		if withoutRecentPost {
			body := "Não encontramos uma postagem publicada nos últimos 7 dias."
			title := "Cliente sem postagem recente"
			occurred := ""
			if !last.IsZero() {
				occurred, body = dateOnly(last), fmt.Sprintf("A última postagem confirmada foi em %s.", last.Format("02/01"))
			}
			if withoutPipeline {
				title = "Cliente sem postagem e sem conteúdo em produção"
				body += " Também não há conteúdo em produção ou postagem futura; vale planejar a próxima captação."
			}
			alerts = append(alerts, makeAlert("client_without_post", SeverityCritical, client.ID, clientNames[client.ID], title, body, "client", client.ID, occurred, "/calendario?clientId="+client.ID))
		}
		if !withoutRecentPost && withoutPipeline {
			alerts = append(alerts, makeAlert("schedule_capture", SeverityAttention, client.ID, clientNames[client.ID], "Planejar próxima captação", "Não há conteúdo em produção nem postagem futura identificada.", "client", client.ID, "", "/calendario?clientId="+client.ID))
		}
	}

	severityRank := map[Severity]int{SeverityCritical: 0, SeverityAttention: 1, SeverityInfo: 2}
	sort.SliceStable(alerts, func(i, j int) bool {
		if severityRank[alerts[i].Severity] != severityRank[alerts[j].Severity] {
			return severityRank[alerts[i].Severity] < severityRank[alerts[j].Severity]
		}
		if alerts[i].ClientName != alerts[j].ClientName {
			return alerts[i].ClientName < alerts[j].ClientName
		}
		return alerts[i].ID < alerts[j].ID
	})
	if len(alerts) > 80 {
		alerts = alerts[:80]
	}

	brief := Brief{GeneratedAt: now, Today: today, Mode: mode, Alerts: alerts, Clients: make([]ClientHealth, 0, len(clients))}
	clientHealth := map[string]*ClientHealth{}
	for _, client := range clients {
		health := ClientHealth{ClientID: client.ID, ClientName: clientNames[client.ID]}
		if value := lastPosted[client.ID]; !value.IsZero() {
			health.LastPostedOn = dateOnly(value)
		}
		brief.Clients = append(brief.Clients, health)
		clientHealth[client.ID] = &brief.Clients[len(brief.Clients)-1]
	}
	for _, alert := range alerts {
		switch alert.Severity {
		case SeverityCritical:
			brief.Counts.Critical++
		case SeverityAttention:
			brief.Counts.Attention++
		default:
			brief.Counts.Info++
		}
		if health := clientHealth[alert.ClientID]; health != nil {
			switch alert.Severity {
			case SeverityCritical:
				health.Critical++
			case SeverityAttention:
				health.Attention++
			default:
				health.Info++
			}
		}
	}
	brief.Counts.Total = len(alerts)
	switch mode {
	case "planning":
		brief.Headline = "Planejamento da semana"
		brief.Summary = "Veja quem precisa de captação, edição, aprovação ou postagem."
	case "closing":
		brief.Headline = "Fechamento da semana"
		brief.Summary = "Confirme o que foi publicado e organize o conteúdo que ficou pendente."
	default:
		brief.Headline = "Acompanhamento de conteúdo"
		brief.Summary = "Prioridades calculadas a partir das tasks e do calendário."
	}
	return brief
}

func makeAlert(kind string, severity Severity, clientID, clientName, title, body, sourceKind, sourceID, occurredOn, link string) Alert {
	return Alert{ID: kind + ":" + clientID + ":" + sourceID, Type: kind, Severity: severity, Title: title, Body: body, ClientID: clientID, ClientName: clientName, SourceKind: sourceKind, SourceID: sourceID, OccurredOn: occurredOn, LinkPath: link}
}
func dayStart(value time.Time) time.Time {
	y, m, d := value.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, value.Location())
}
func dateOnly(value time.Time) string { return value.Format(time.DateOnly) }
func itemAgeDays(now time.Time, raw string, fallback time.Time) int {
	parsed, err := time.ParseInLocation(time.DateOnly, raw, now.Location())
	if err != nil {
		parsed = fallback.In(now.Location())
	}
	return int(dayStart(now).Sub(dayStart(parsed)).Hours() / 24)
}
func statusLabel(status string) string {
	if status == "scheduled" {
		return "agendado"
	}
	return "aprovado"
}
