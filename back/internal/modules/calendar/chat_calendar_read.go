package calendar

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type chatCalendarRead struct {
	from, to string
}

var (
	chatRelativeDayRe        = regexp.MustCompile(`\b(hoje|ontem|amanha)\b`)
	chatReadIntentRe         = regexp.MustCompile(`\b(temos|tenho|agenda|eventos?|liste|listar|mostre|mostrar|listou|quantos)\b`)
	chatReadOtherIntentRe    = regexp.MustCompile(`\b(crie|criar|adicione|adicionar|altere|alterar|edite|editar|exclua|excluir|apague|apagar|mude|mudar|agende|agendar|marque|marcar|anote|anotar|gere|gerar|cancele|cancelar|conclua|tasks?|tarefas?|checklist|cliente|aprovados?|planejados?|pendentes?|reunioes|reels|stories|compare)\b`)
	chatReadDateRe           = regexp.MustCompile(`\b(0?[1-9]|[12]\d|3[01])[/-](0?[1-9]|1[0-2])(?:[/-](20\d{2}|\d{2}))?\b`)
	chatReadISODayRe         = regexp.MustCompile(`\b20\d{2}-(0[1-9]|1[0-2])-(0[1-9]|[12]\d|3[01])\b`)
	chatReadNamedDayRe       = regexp.MustCompile(`\b(0?[1-9]|[12]\d|3[01]) (?:de )?(janeiro|fevereiro|marco|abril|maio|junho|julho|agosto|setembro|outubro|novembro|dezembro)\b`)
	chatNegatedRelativeDayRe = regexp.MustCompile(`\b(?:nao|nem) (?:e )?(hoje|ontem|amanha)\b`)
	chatRelativeWeekRe       = regexp.MustCompile(`\b(esta|essa|nesta|nessa|desta|dessa) semana\b`)
	chatRelativeMonthRe      = regexp.MustCompile(`\b(este|esse|neste|nesse|deste|desse) mes\b`)
	chatReadNegatedIntentRe  = regexp.MustCompile(`\b(nao|n|nem) (liste|listar|mostre|mostrar)\b|\b(liste|listar|mostre|mostrar) (apenas|so)\b`)
)

func chatRelativeDay(question string, now time.Time) (time.Time, bool) {
	normalized := chatNegatedRelativeDayRe.ReplaceAllString(foldChatLabel(question), "")
	match := chatRelativeDayRe.FindString(normalized)
	if match == "" {
		return time.Time{}, false
	}
	day := now.In(saoPauloLoc)
	switch match {
	case "ontem":
		day = day.AddDate(0, 0, -1)
	case "amanha":
		day = day.AddDate(0, 0, 1)
	}
	return day, true
}

// Only plain calendar listings use a server-computed count. Requests for edits,
// tasks, explicit client filters or status filters retain the conversational path.
func resolveChatCalendarRead(question string, prior []ChatMessage, now time.Time) (chatCalendarRead, bool) {
	folded := foldChatLabel(question)
	if !chatReadIntentRe.MatchString(folded) || chatReadOtherIntentRe.MatchString(folded) || chatReadNegatedIntentRe.MatchString(folded) || !isPlainCalendarRead(folded) {
		return chatCalendarRead{}, false
	}
	if len(chatRelativeDayRe.FindAllString(folded, -1)) > 1 || len(chatReadDateRe.FindAllString(question, -1)) > 1 || len(chatMonthNameRe.FindAllString(folded, -1)) > 1 {
		return chatCalendarRead{}, false
	}
	day, ok := chatRelativeDay(question, now)
	if !ok {
		if chatRelativeWeekRe.MatchString(folded) {
			today := now.In(saoPauloLoc)
			monday := today.AddDate(0, 0, -(int(today.Weekday())+6)%7)
			return chatCalendarRead{from: monday.Format(time.DateOnly), to: monday.AddDate(0, 0, 6).Format(time.DateOnly)}, true
		} else if chatRelativeMonthRe.MatchString(folded) {
			from, to, err := monthBounds(now.In(saoPauloLoc).Format("2006-01"))
			return chatCalendarRead{from: from, to: to}, err == nil
		} else if match := chatReadISODayRe.FindString(question); match != "" {
			day, _ = time.ParseInLocation(time.DateOnly, match, saoPauloLoc)
			ok = !day.IsZero()
		} else if match := chatReadDateRe.FindStringSubmatch(question); len(match) > 0 {
			year := fmt.Sprintf("%d", now.In(saoPauloLoc).Year())
			if match[3] != "" {
				year = match[3]
				if len(year) == 2 {
					year = "20" + year
				}
			}
			day, _ = time.ParseInLocation("2/1/2006", match[1]+"/"+match[2]+"/"+year, saoPauloLoc)
			ok = !day.IsZero()
		} else if match := chatReadNamedDayRe.FindStringSubmatch(folded); len(match) > 0 {
			month := inferChatMonth(question, "", now)
			day, _ = time.ParseInLocation("2006-01-2", month+"-"+match[1], saoPauloLoc)
			ok = !day.IsZero()
		} else if chatMonthNameRe.MatchString(folded) || chatISOYearMonthRe.MatchString(question) {
			month := inferChatMonth(question, "", now)
			from, to, err := monthBounds(month)
			return chatCalendarRead{from: from, to: to}, err == nil
		} else if strings.Contains(folded, "liste") || strings.Contains(folded, "mostre") {
			// Continue the last user request; the model's previous dates and counts
			// are not authoritative and must never become the query's source.
			for i := len(prior) - 1; i >= 0; i-- {
				if prior[i].Role != chatRoleUser {
					continue
				}
				if previous, found := resolveChatCalendarRead(prior[i].Content, nil, now); found {
					return previous, true
				}
			}
		}
	}
	if !ok {
		return chatCalendarRead{}, false
	}
	date := day.Format(time.DateOnly)
	return chatCalendarRead{from: date, to: date}, true
}

func isPlainCalendarRead(question string) bool {
	// Extra qualifiers (names, statuses, creation requests, etc.) are handled by
	// the existing conversational path, never silently discarded by a listing.
	const words = " o que oq q quais qual quantos quantas temos tenho ha agenda eventos evento calendario calendarios de para pra em no na nos nas do da dos das um uma os as a e ou por favor hoje ontem amanha liste listar lista mostre mostrar mostra eles elas esses essas itens novamente aqui todos todas completo completa corretos corretas vc voce me listou apenas so ne nao n eh dia dias mes ano este esse neste nesse deste desse esta essa nesta nessa desta dessa semana janeiro fevereiro marco abril maio junho julho agosto setembro outubro novembro dezembro "
	for _, word := range strings.Fields(question) {
		if strings.Contains(words, " "+word+" ") {
			continue
		}
		if _, err := strconv.Atoi(word); err == nil {
			continue
		}
		if chatReadDateRe.MatchString(word) || chatISOYearMonthRe.MatchString(word) {
			continue
		}
		return false
	}
	return true
}

func (s *Service) readCalendarQueryEvents(ctx context.Context, accountID string, access ChatAccess, target chatTarget, query chatCalendarRead) ([]AIContextEvent, bool, error) {
	storageID := access.calendarAccountID(accountID)
	var events []AIContextEvent
	var err error
	if target.mode == chatScopeAll {
		events, err = s.store.ListEventsLeanForClients(ctx, storageID, query.from, query.to, access.VisibleClientIDs, maxContextEvents+1)
	} else {
		events, err = s.store.ListEventsLean(ctx, storageID, query.from, query.to, target.clientID, maxContextEvents+1)
	}
	if err != nil {
		return nil, false, err
	}
	truncated := len(events) > maxContextEvents
	if truncated {
		events = events[:maxContextEvents]
	}
	setContextClientNames(events, access.clientNameByID())
	return events, truncated, nil
}

func calendarReadAnswer(query chatCalendarRead, events []AIContextEvent, truncated bool) string {
	day, _ := time.Parse(time.DateOnly, query.from)
	date := day.Format("02/01/2006")
	period := "em " + date
	if query.to != query.from {
		end, _ := time.Parse(time.DateOnly, query.to)
		period = "de " + date + " a " + end.Format("02/01/2006")
	}
	if truncated {
		return fmt.Sprintf("Mostrando os primeiros %d eventos %s nos cards. Há mais eventos nesse período.", len(events), period)
	}
	switch len(events) {
	case 0:
		return "Não há eventos no calendário " + period + " para o escopo selecionado."
	case 1:
		return "Há 1 evento no calendário " + period + ". Confira o card abaixo."
	default:
		return fmt.Sprintf("Há %d eventos no calendário %s. Confira os %d cards abaixo.", len(events), period, len(events))
	}
}

func replaceChatContextEvents(block any, events []AIContextEvent) any {
	switch value := block.(type) {
	case calendarChatContext:
		value.Events = events
		return value
	case AIContextAll:
		value.Events = events
		return value
	default:
		return block
	}
}

func groundedCalendarCardAnswer(answer string, ids []string, items []AIContextEvent) string {
	confirmed := make(map[string]bool, len(items))
	for _, item := range items {
		confirmed[item.ID] = true
	}
	for _, id := range ids {
		if confirmed[strings.TrimSpace(id)] {
			continue
		}
		if len(items) == 0 {
			return "Não consegui confirmar os eventos mencionados com os dados do calendário disponíveis nesta consulta."
		}
		if len(items) == 1 {
			return "Consegui confirmar 1 item do calendário, exibido no card. Os demais itens mencionados não puderam ser confirmados."
		}
		return fmt.Sprintf("Consegui confirmar %d itens do calendário, exibidos nos cards. Os demais itens mencionados não puderam ser confirmados.", len(items))
	}
	return compactCalendarCardAnswer(answer, len(items))
}
