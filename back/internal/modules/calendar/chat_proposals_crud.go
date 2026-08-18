package calendar

import "strings"

// CRUD de ANOTACOES e PERFIL do cliente pelo chat (WAVE 7). A IA nao grava: propoe
// (kind=note|clientProfile) e o front executa no confirm pela API autenticada do usuario
// (PUT /v1/calendar/notes/{month} e PUT /v1/calendar/client-profile). Estes tipos e sanitizers
// estendem o contrato de propostas de chat.go sem inchar aquele arquivo; sanitizeProposal (chat.go)
// despacha a validacao por kind. Espelho: o no "Extrair resposta" do workflow calendar-chat.

// ChatProposalNote e o sub-objeto da proposta de anotacao do mes (kind=note). Content = texto/HTML
// a acrescentar ou definir; Mode = append|replace (vazio => append no front; delete ignora); Month =
// YYYY-MM (vazio => o front usa o mes em foco).
type ChatProposalNote struct {
	Month   string `json:"month,omitempty"`
	Content string `json:"content,omitempty"`
	Mode    string `json:"mode,omitempty"`
}

// ChatProposalProfile e o sub-objeto da proposta de perfil estrategico do cliente (kind=clientProfile).
// Campos nao-vazios sao MESCLADOS (o front faz GET->merge->PUT full-replace, nunca zera o resto);
// ClearFields esvazia campos especificos e ClearAll zera o perfil (delete). O cliente-alvo vai em
// ChatProposalFields.ClientID (resolvido pela IA por nome, ou escolhido pelo dono no cartao).
type ChatProposalProfile struct {
	Segment     string        `json:"segment,omitempty"`
	Positioning string        `json:"positioning,omitempty"`
	Description string        `json:"description,omitempty"`
	History     string        `json:"history,omitempty"`
	SiteURL     string        `json:"siteUrl,omitempty"`
	Instagram   string        `json:"instagram,omitempty"`
	Address     string        `json:"address,omitempty"`
	Objectives  string        `json:"objectives,omitempty"`
	BrandVoice  string        `json:"brandVoice,omitempty"`
	Extra       *ProfileExtra `json:"extra,omitempty"`
	ClearFields []string      `json:"clearFields,omitempty"`
	ClearAll    bool          `json:"clearAll,omitempty"`
}

// profileClearableKeys sao os campos que a IA pode pedir para esvaziar (delete parcial). As chaves
// batem 1:1 com o JSON do front (camelCase dos estaveis + chaves do extra).
var profileClearableKeys = map[string]bool{
	"segment": true, "positioning": true, "description": true, "history": true,
	"siteUrl": true, "instagram": true, "address": true, "objectives": true, "brandVoice": true,
	"audience": true, "offer": true, "pillars": true, "cadence": true,
	"restrictions": true, "performance": true, "assets": true,
}

// canonicalProposalKind normaliza o kind para a forma canonica usada pelo front
// (event|task|taskItem|note|clientProfile); "" = kind desconhecido (proposta descartada).
func canonicalProposalKind(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "event":
		return "event"
	case "task":
		return "task"
	case "taskitem", "task_item", "checklistitem", "checklist_item":
		return "taskItem"
	case "note":
		return "note"
	case "clientprofile", "client_profile", "profile":
		return "clientProfile"
	default:
		return ""
	}
}

// sanitizeContentProposal aplica as regras de event/task (WAVE 5.1): create exige titulo;
// update exige ao menos um campo editavel. Update/delete SEM targetId passam AQUI de proposito
// (WAVE 15): a guarda de alvo resolve o targetId pelo titulo/dia citados na pergunta — descartar
// antes dela matava propostas recuperaveis ("o cliente da X e a Bari" sem targetId). Quem
// descarta o que sobrar sem alvo e dropTargetlessEditable, DEPOIS da guarda.
func sanitizeContentProposal(action string, f ChatProposalFields) bool {
	if action == "create" && strings.TrimSpace(f.Title) == "" {
		return false
	}
	if action == "update" && !proposalHasEditableField(f) {
		return false
	}
	return true
}

// sanitizeNoteProposal valida a proposta de anotacao (WAVE 7). delete = limpar (sempre valido, mesmo
// sem note: o front limpa o mes em foco); create/update exigem conteudo.
func sanitizeNoteProposal(action string, note *ChatProposalNote) bool {
	if action == "delete" {
		return true
	}
	return note != nil && strings.TrimSpace(note.Content) != ""
}

// sanitizeProfileProposal valida a proposta de perfil (WAVE 7). delete exige clearAll OU clearFields;
// create/update exigem ao menos um campo preenchido. clientId e OPCIONAL (dono escolhe no cartao).
func sanitizeProfileProposal(action string, prof *ChatProposalProfile) bool {
	if prof == nil {
		return false
	}
	if action == "delete" {
		return prof.ClearAll || len(prof.ClearFields) > 0
	}
	return profileProposalHasField(prof)
}

// profileProposalHasField diz se a proposta traz ao menos um campo de perfil preenchido.
func profileProposalHasField(p *ChatProposalProfile) bool {
	if p == nil {
		return false
	}
	stable := []string{p.Segment, p.Positioning, p.Description, p.History,
		p.SiteURL, p.Instagram, p.Address, p.Objectives, p.BrandVoice}
	for _, v := range stable {
		if strings.TrimSpace(v) != "" {
			return true
		}
	}
	return p.Extra != nil && extraHasValue(*p.Extra)
}

// extraHasValue diz se algum campo livre do brief esta preenchido.
func extraHasValue(e ProfileExtra) bool {
	fields := []string{e.Audience, e.Offer, e.Pillars, e.Cadence, e.Restrictions, e.Performance, e.Assets}
	for _, v := range fields {
		if strings.TrimSpace(v) != "" {
			return true
		}
	}
	return false
}

// normalizeNoteField faz trim do conteudo, valida o month (invalido => vazio, o front cai no mes em
// foco) e normaliza o mode (append|replace; vazio => append no front).
func normalizeNoteField(n *ChatProposalNote) {
	if n == nil {
		return
	}
	n.Content = strings.TrimSpace(n.Content)
	n.Month = strings.TrimSpace(n.Month)
	if n.Month != "" && !monthRe.MatchString(n.Month) {
		n.Month = ""
	}
	mode := strings.ToLower(strings.TrimSpace(n.Mode))
	if mode != "append" && mode != "replace" {
		mode = ""
	}
	n.Mode = mode
}

// normalizeProfileField faz trim dos campos do perfil (reusa trimExtra) e canonicaliza clearFields.
func normalizeProfileField(p *ChatProposalProfile) {
	if p == nil {
		return
	}
	p.Segment = strings.TrimSpace(p.Segment)
	p.Positioning = strings.TrimSpace(p.Positioning)
	p.Description = strings.TrimSpace(p.Description)
	p.History = strings.TrimSpace(p.History)
	p.SiteURL = strings.TrimSpace(p.SiteURL)
	p.Instagram = strings.TrimSpace(p.Instagram)
	p.Address = strings.TrimSpace(p.Address)
	p.Objectives = strings.TrimSpace(p.Objectives)
	p.BrandVoice = strings.TrimSpace(p.BrandVoice)
	if p.Extra != nil {
		trimmed := trimExtra(*p.Extra)
		p.Extra = &trimmed
	}
	p.ClearFields = normalizeClearFields(p.ClearFields)
}

// normalizeClearFields mantem so as chaves de campo conhecidas (camelCase do front) e sem duplicatas.
func normalizeClearFields(raw []string) []string {
	if len(raw) == 0 {
		return nil
	}
	out := make([]string, 0, len(raw))
	seen := map[string]bool{}
	for _, r := range raw {
		k := strings.TrimSpace(r)
		if !profileClearableKeys[k] || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, k)
	}
	return out
}
