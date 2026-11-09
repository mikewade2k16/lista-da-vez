package finance

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"regexp"
	"strings"
)

type AccessMessage struct {
	Evidence []json.RawMessage `json:"evidence,omitempty"`

	DraftID string `json:"draftId,omitempty"`
	ID      string `json:"id"`
	Role    string `json:"role"`
	Text    string `json:"text"`
	Action  string `json:"action"`
}
type AccessConversationInput struct {
	FinanceSheetID string `json:"financeSheetId,omitempty"`
	Month          string `json:"month,omitempty"`
	DraftID        string `json:"draftId,omitempty"`
	RequestID      string `json:"requestId"`
	Text           string `json:"text"`
	// Server-side chat state from the previous assistant message; never from the body.
	ChatSheetID string `json:"-"`
	ChatAction  string `json:"-"`
}
type AccessConversation struct {
	Messages []AccessMessage `json:"messages"`
}
type AccessConversationService struct {
	orchestrator *ConversationOrchestrator
	pool         *pgxpool.Pool
	fiscal       *FiscalService
	documents    *DocumentService
}

var accessSecretPattern = regexp.MustCompile(`(?i)(?:senha|password|passwd|pwd|token|cookies?|authorization|api[_ -]?key)\s*(?::|=|é\s|eh\s)\s*\S+`)

func accessReply(text string, p FiscalProfileView) (string, string, string) {
	n := normalizedDocumentLabel(text)
	switch {
	case n == "cancelar" || n == "parar":
		return "Tudo bem. Mantive os dados já salvos. Como posso ajudar?", "", ""
	case asksConversationEditor(n) && strings.Contains(n, "formulario") && (strings.Contains(n, "nota") || strings.Contains(n, "rascunho")):
		return "Aqui está o formulário solicitado para editar o rascunho.", "document", ""
	case accessSecretPattern.MatchString(text):
		return "Não grave sua senha como mensagem. Use o campo protegido de acesso aqui na conversa.", "access", ""
	case asksConversationEditor(n) && (strings.Contains(n, "gov.br") || strings.Contains(n, "govbr")):
		return "Vou usar a entrada gov.br. A autenticação e as confirmações são feitas no portal; não informe a senha gov.br aqui.", "access", "govbr"
	case asksConversationEditor(n) && (strings.Contains(n, "senha do emissor") || strings.Contains(n, "senha propria") || strings.Contains(n, "usuario e senha") || strings.Contains(n, "cpf/cnpj e senha")):
		return "O acesso será por CPF/CNPJ e senha próprios do Emissor Nacional. Cadastre no campo protegido abaixo; se já estiver salvo, não precisa repetir.", "access", "portal"
	case strings.Contains(n, "test") && (strings.Contains(n, "acesso") || strings.Contains(n, "login") || strings.Contains(n, "portal")):
		if !p.CredentialsSet && p.Profile.PortalLoginMethod != "govbr" {
			return "Ainda falta cadastrar seu acesso. Preencha CPF/CNPJ e senha no campo protegido abaixo.", "access", ""
		}
		return "Use Testar acesso no cartão abaixo. Ele envia o acesso salvo somente ao portal oficial e não emite nota.", "access", ""
	case wantsDocumentPreparation(n):
		return "Vamos preparar os dados para sua revisão. Nenhuma nota será emitida.", "prepare", ""
	case asksConversationEditor(n) && (strings.Contains(n, "anex") || strings.Contains(n, "comprovante") || strings.Contains(n, "extrato") || strings.Contains(n, "imagem") || strings.Contains(n, "pdf")):
		return "Pode anexar o comprovante abaixo. A leitura prepara um rascunho; o acesso ao portal precisa ser validado antes da emissão.", "document", ""
	case asksConversationEditor(n) && (strings.Contains(n, "perfil") || strings.Contains(n, "cadastro fiscal")):
		return "Você pode editar seu cadastro aqui. São os mesmos dados do perfil, sem cópia paralela.", "profile", ""
	case asksConversationEditor(n) && (strings.Contains(n, "acesso") || strings.Contains(n, "login") || strings.Contains(n, "portal") || strings.Contains(n, "senha")):
		if p.CredentialsSet {
			return "Seu acesso já está salvo. Você pode testá-lo ou substituí-lo abaixo; não precisa informar novamente.", "access", ""
		}
		if p.Profile.PortalLoginMethod == "govbr" {
			return "Seu cadastro está como gov.br. Você vai usar gov.br ou CPF/CNPJ e senha próprios do Emissor Nacional?", "access", ""
		}
		return "Para acessar o Emissor Nacional, você usará gov.br ou CPF/CNPJ e senha próprios do portal?", "access", ""
	default:
		return "Posso ajudar com suas finanças, preparar um rascunho ou consultar seu cadastro. O que você quer fazer?", "", ""
	}
}
func (s *AccessConversationService) List(ctx context.Context, account, user string) (AccessConversation, error) {
	rows, err := s.pool.Query(ctx, `select id::text,role,text,action,coalesce(draft_id::text,''),public_evidence from (select id,role,text,action,draft_id,public_evidence,created_at from finance.access_messages where account_id=$1::uuid and user_id=$2::uuid order by created_at desc,id desc limit 60) m order by created_at,id`, account, user)
	if err != nil {
		return AccessConversation{}, err
	}
	defer rows.Close()
	out := AccessConversation{Messages: []AccessMessage{}}
	for rows.Next() {
		var m AccessMessage
		var evidence []byte
		if err = rows.Scan(&m.ID, &m.Role, &m.Text, &m.Action, &m.DraftID, &evidence); err != nil {
			return out, err
		}
		if err = json.Unmarshal(evidence, &m.Evidence); err != nil {
			return out, err
		}
		out.Messages = append(out.Messages, m)
	}
	if err = rows.Err(); err != nil {
		return out, err
	}
	rows.Close()
	s.revalidatePublicEvidence(ctx, &out)
	return out, nil
}
func (s *AccessConversationService) Send(ctx context.Context, account, user string, in AccessConversationInput) (AccessConversation, error) {
	text := strings.TrimSpace(in.Text)
	if !isUUID(in.RequestID) || text == "" || len(text) > 4000 {
		return AccessConversation{}, invalidFiscal("Envie uma mensagem de até 4.000 bytes.")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return AccessConversation{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `select pg_advisory_xact_lock(hashtextextended($1,0))`, "finance:access-chat:"+account+":"+user); err != nil {
		return AccessConversation{}, err
	}
	var duplicate bool
	if err = tx.QueryRow(ctx, `select exists(select 1 from finance.access_messages where account_id=$1::uuid and user_id=$2::uuid and request_id=$3::uuid)`, account, user, in.RequestID).Scan(&duplicate); err != nil {
		return AccessConversation{}, err
	}
	if duplicate {
		if err = tx.Commit(ctx); err != nil {
			return AccessConversation{}, err
		}
		return s.List(ctx, account, user)
	}
	profile, err := s.fiscal.GetProfile(ctx, account, user)
	if err != nil {
		return AccessConversation{}, err
	}
	var previousAction, previousDraft string
	err = tx.QueryRow(ctx, `select action,coalesce(draft_id::text,'') from finance.access_messages where account_id=$1::uuid and user_id=$2::uuid and role='assistant' order by created_at desc,id desc limit 1`, account, user).Scan(&previousAction, &previousDraft)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return AccessConversation{}, err
	}
	asking := conversationAsking(previousAction)
	secret := accessSecretPattern.MatchString(text)
	intent := interpretConversation(text, in.Month)
	routed := text
	if intent.Canonical != "" {
		routed = intent.Canonical
	}
	questionReply := asking && !conversationCommand(text)
	answer, action, method := accessReply(routed, profile)
	if questionReply && !accessSecretPattern.MatchString(text) {
		action, method = "", ""
	}
	draftID := in.DraftID
	if draftID == "" {
		draftID = previousDraft
	}
	if draftID != "" {
		if s.documents == nil {
			return AccessConversation{}, ErrFiscalNotFound
		}
		if _, err = s.documents.Get(ctx, account, user, draftID); err != nil {
			return AccessConversation{}, err
		}
	}
	chatInput := in
	chatInput.ChatAction = previousAction
	chatInput.ChatSheetID = strings.TrimPrefix(previousSheetAction(previousAction), "sheet:")
	var handled bool
	if !secret {
		var shortcut, shortcutAction string
		shortcut, shortcutAction, handled, err = s.conversationShortcut(ctx, account, user, intent, text, draftID, previousAction, chatInput)
		if err != nil {
			return AccessConversation{}, err
		}
		if handled {
			answer, action, method = shortcut, shortcutAction, ""
		}
	}
	if !handled && s.documents != nil && !secret {
		// A recognized command never fills the field that was pending.
		pending := previousAction
		if !questionReply {
			pending = ""
		}
		answer, action, draftID, err = s.prepareConversation(ctx, account, user, draftID, routed, text, answer, action, pending, in.RequestID, in.DraftID != "")
		if err != nil {
			return AccessConversation{}, err
		}
	}
	var publicEvidence []json.RawMessage
	if !handled && s.orchestrator != nil && action == "" && !questionReply && !secret && (!conversationCommand(text) || conversationRead(text)) {
		var known bool
		answer, publicEvidence, known, err = s.orchestrator.knowledgeReply(ctx, account, user, in.Text)
		if err == nil && !known {
			routedInput := chatInput
			routedInput.Text = routed
			answer, action, err = s.orchestrator.ReplyWithAction(ctx, account, user, routedInput)
		}
		if err != nil {
			return AccessConversation{}, err
		}
	}
	// A side request answered during a pending field keeps that field pending.
	if asking && !questionReply && action == "" && draftID != "" && intent.Kind != "cancel" && !secret && s.documents != nil {
		next, question, e := s.pendingQuestion(ctx, account, user, draftID)
		if e != nil {
			return AccessConversation{}, e
		}
		if next != "" {
			action = next
			answer += "\n\nQuando quiser, seguimos com a nota: " + question
		}
	}
	if secret {
		text = "[Dados de acesso omitidos — use o campo protegido]"
		answer = "Use o campo protegido abaixo para informar o acesso. Nenhum desses dados foi salvo no histórico."
		action = "access"
		method = ""
	}
	if method != "" && (profile.Profile.PortalLoginMethod != method || profile.Profile.PortalURL != nationalPortalLogin) {
		p := profile.Profile
		p.PortalLoginMethod = method
		p.PortalURL = nationalPortalLogin
		if _, err = s.fiscal.SaveProfile(ctx, account, user, FiscalProfileInput{Profile: p, ExpectedUpdatedAt: profile.UpdatedAt}); err != nil {
			return AccessConversation{}, err
		}
	}
	for _, m := range []AccessMessage{{Role: "user", Text: text, DraftID: draftID}, {Role: "assistant", Text: answer, Action: action, DraftID: draftID, Evidence: publicEvidence}} {
		if _, err = tx.Exec(ctx, `insert into finance.access_messages(account_id,user_id,request_id,role,text,action,draft_id,public_evidence) values($1::uuid,$2::uuid,$3::uuid,$4,$5,$6,nullif($7,'')::uuid,$8::jsonb)`, account, user, in.RequestID, m.Role, m.Text, m.Action, m.DraftID, evidenceJSON(m.Evidence)); err != nil {
			return AccessConversation{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return AccessConversation{}, err
	}
	return s.List(ctx, account, user)
}

func asksConversationEditor(n string) bool {
	switch n {
	case "gov.br", "govbr", "perfil", "cadastro fiscal", "acesso", "login", "anexar comprovante":
		return true
	}
	for _, prefix := range []string{"abrir ", "quero ", "preciso cadastrar ", "mostrar ", "exibir ", "cadastrar ", "editar ", "atualizar ", "anexar ", "enviar ", "ler ", "usar ", "testar ", "cpf/cnpj e senha", "usuario e senha", "senha propria", "formulario "} {
		if strings.HasPrefix(n, prefix) {
			return true
		}
	}
	return false
}

func evidenceJSON(values []json.RawMessage) json.RawMessage {
	if values == nil {
		return json.RawMessage(`[]`)
	}
	return plusJSON(values)
}

// previousSheetAction keeps the chat's chosen sheet only when the last reply set it.
func previousSheetAction(previousAction string) string {
	if strings.HasPrefix(previousAction, "sheet:") {
		return previousAction
	}
	return ""
}

// conversationShortcut answers turns that bypass preparation: a pending receipt
// proposal, "lançar comprovante", composed requests and resume.
func (s *AccessConversationService) conversationShortcut(ctx context.Context, account, user string, intent conversationIntent, text, draftID, previousAction string, in AccessConversationInput) (string, string, bool, error) {
	if answer, action, handled, err := s.answerReceiptProposal(ctx, account, user, previousAction, text, in); handled || err != nil {
		return answer, action, handled, err
	}
	switch {
	case intent.Canonical == "lancar comprovante":
		answer, action, err := s.proposeReceipt(ctx, account, user, draftID, text, in, receiptProposal{})
		return answer, action, true, err
	case intent.Kind == "multiple":
		action := ""
		if conversationAsking(previousAction) {
			action = previousAction
		}
		return multipleRequestsReply(intent.Parts), action, true, nil
	case intent.Kind == "resume":
		answer, action, err := s.resumeConversation(ctx, account, user, draftID)
		return answer, action, true, err
	}
	return "", "", false, nil
}
