package tasks

import (
	"bytes"
	"encoding/json"
	"strings"
)

// Vinculo de task com modulo do roadmap.
//
// Regra de negocio: uma task vinculada a um modulo do roadmap fica SEMPRE fixada
// (visivel) — `PinnedToRoadmap` e derivado do modulo efetivo, nunca do flag cru
// enviado pelo cliente. Sem modulo, a task nunca fica fixada. So e possivel
// "desfixar" limpando o vinculo do modulo.

// applyCreateRoadmapLink normaliza o vinculo na criacao: trim do modulo (vazio
// vira nil) e pin derivado da presenca do modulo.
func applyCreateRoadmapLink(input *CreateTaskInput) {
	if input.RoadmapModuleID != nil {
		trimmed := strings.TrimSpace(*input.RoadmapModuleID)
		if trimmed == "" {
			input.RoadmapModuleID = nil
		} else {
			input.RoadmapModuleID = &trimmed
		}
	}
	pinned := input.RoadmapModuleID != nil
	input.PinnedToRoadmap = &pinned
}

// applyUpdateRoadmapLink resolve o modulo efetivo apos o patch e deriva o pin.
// Campo ausente mantem o modulo atual (before); presente com null limpa; presente
// com valor faz trim. O flag explicito de pin e ignorado: o pin segue o modulo.
func applyUpdateRoadmapLink(input *UpdateTaskInput, before Task) {
	effectiveModule := before.RoadmapModuleID
	if input.RoadmapModuleID != nil {
		if inner := *input.RoadmapModuleID; inner != nil {
			trimmed := strings.TrimSpace(*inner)
			if trimmed == "" {
				var cleared *string
				input.RoadmapModuleID = &cleared
				effectiveModule = nil
			} else {
				next := &trimmed
				input.RoadmapModuleID = &next
				effectiveModule = next
			}
		} else {
			effectiveModule = nil
		}
	}
	pinned := effectiveModule != nil
	input.PinnedToRoadmap = &pinned
}

// UnmarshalJSON decodifica UpdateTaskInput preservando a distincao entre "campo
// ausente" e "campo presente com null" nos opcionais de dois niveis (**T). O json
// padrao colapsa ambos em ponteiro externo nil; aqui re-detectamos a presenca via
// raw map para que "limpar campo" (null explicito) chegue ao repository.
func (input *UpdateTaskInput) UnmarshalJSON(data []byte) error {
	type alias UpdateTaskInput
	if err := json.Unmarshal(data, (*alias)(input)); err != nil {
		return err
	}
	var present map[string]json.RawMessage
	if err := json.Unmarshal(data, &present); err != nil {
		return err
	}
	markNullablePresent(present["columnId"], &input.ColumnID)
	markNullablePresent(present["status"], &input.Status)
	markNullablePresent(present["dueDate"], &input.DueDate)
	markNullablePresent(present["startDate"], &input.StartDate)
	markNullablePresent(present["responsibleUserId"], &input.ResponsibleUserID)
	markNullablePresent(present["clientAccountId"], &input.ClientAccountID)
	markNullablePresent(present["roadmapModuleId"], &input.RoadmapModuleID)
	return nil
}

// markNullablePresent marca um opcional **T como "presente com null" (ponteiro
// externo nao-nil, interno nil) quando o JSON trouxe null explicito. raw vazio =
// campo ausente. Para valores nao-null o json padrao ja preencheu o ponteiro.
func markNullablePresent[T any](raw json.RawMessage, field ***T) {
	if len(raw) == 0 || *field != nil {
		return
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		var inner *T
		*field = &inner
	}
}
