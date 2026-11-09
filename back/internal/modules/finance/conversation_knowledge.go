package finance

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
)

var fiscalQuestion = regexp.MustCompile(`\b(mei|simei|dasn|imposto|impostos|tributo|tributos|tributaria|tributario|aliquota|cnpj|cnae|simples nacional|lucro|lucros|declaracao|nfse|nfs-e|nota fiscal)\b`)
var publicQueryWord = regexp.MustCompile(`[a-z0-9]{2,50}`)

// These are retrieval aliases, never fiscal values or rules.
func publicKnowledgeTerms(text string) []string {
	n := normalizedDocumentLabel(text)
	aliases := map[string]string{"recebimento": "receita", "faturamento": "receita", "anualmente": "anual", "impostos": "imposto"}
	stops := map[string]bool{}
	for _, v := range strings.Fields("qual quais como quanto quando onde porque para pelo pela pelos pelas uma umas uns esse essa isso disso meu minha meus minhas seu sua seus suas voce voces quero saber sobre fonte fontes buscar localizar regra regras explique explicar resuma interpretar significa que dos das de do da os as ao no na nos nas em por com sem se eu ano pode posso deve tenho preciso esta ser") {
		stops[v] = true
	}
	out := []string{}
	seen := map[string]bool{}
	for _, word := range publicQueryWord.FindAllString(n, -1) {
		if alias := aliases[word]; alias != "" {
			word = alias
		}
		if seen[word] || (stops[word] && (word != "das" || !strings.HasSuffix(n, " das"))) {
			continue
		}
		seen[word] = true
		out = append(out, word)
		if len(out) == 12 {
			break
		}
	}
	return out
}
func (s *ConversationOrchestrator) knowledgeReply(ctx context.Context, account, user, text string) (string, []json.RawMessage, bool, error) {
	if !publicKnowledgeQuestion(text) {
		return "", nil, false, nil
	}
	if s == nil || s.plus == nil || s.plus.ports.PublicKnowledge == nil {
		return "A busca de fontes públicas ainda não está disponível neste ambiente. Ela não depende de conectar uma IA.", nil, true, nil
	}
	sc := PlusScope{AccountID: account, ActorID: user}
	if s.plus.ports.Authorize == nil {
		return "", nil, true, plusError("permission_denied", 403)
	}
	if err := s.plus.ports.Authorize(ctx, sc, "finance.sheets.view"); err != nil {
		return "", nil, true, err
	}
	terms := publicKnowledgeTerms(text)
	if len(terms) == 0 {
		return "Qual assunto fiscal você quer consultar?", nil, true, nil
	}
	raw, err := s.plus.ports.PublicKnowledge(ctx, terms)
	if err != nil {
		return "", nil, true, err
	}
	var result struct {
		Message  string            `json:"message"`
		Evidence []json.RawMessage `json:"evidence"`
	}
	if err = json.Unmarshal(raw, &result); err != nil {
		return "", nil, true, err
	}
	if len(result.Evidence) > 3 || len(raw) > 24000 {
		return "", nil, true, plusError("invalid_evidence", 502)
	}
	if err = s.plus.ports.Authorize(ctx, sc, "finance.sheets.view"); err != nil {
		return "", nil, true, err
	}
	return result.Message, result.Evidence, true, nil
}
func (s *AccessConversationService) revalidatePublicEvidence(ctx context.Context, out *AccessConversation) {
	ids := []string{}
	for _, m := range out.Messages {
		for _, raw := range m.Evidence {
			var ref struct {
				ID string `json:"chunkId"`
			}
			if json.Unmarshal(raw, &ref) == nil {
				ids = append(ids, ref.ID)
			}
		}
	}
	if len(ids) == 0 {
		return
	}
	valid := s.orchestrator != nil && s.orchestrator.plus != nil && s.orchestrator.plus.ports.ValidatePublicEvidence != nil
	if valid {
		valid = s.orchestrator.plus.ports.ValidatePublicEvidence(ctx, ids) == nil
	}
	if valid {
		return
	}
	for i := range out.Messages {
		if len(out.Messages[i].Evidence) > 0 {
			out.Messages[i].Evidence = nil
			out.Messages[i].Text = "As fontes desta consulta estão indisponíveis ou foram retiradas. Faça uma nova busca para consultar material publicado."
		}
	}
}

func publicKnowledgeQuestion(text string) bool {
	n := normalizedDocumentLabel(text)
	if plusUnsafe.MatchString(n) {
		return false
	}
	if strings.HasPrefix(n, "buscar fonte") || strings.HasPrefix(n, "localizar regra") {
		return true
	}
	if !fiscalQuestion.MatchString(n) && !strings.HasSuffix(n, " das") {
		return false
	}
	for _, prefix := range []string{"qual ", "quais ", "como ", "quanto ", "quando ", "o que ", "explique ", "me explique ", "resuma ", "preciso saber ", "quero saber ", "posso ", "tenho que "} {
		if strings.HasPrefix(n, prefix) {
			return true
		}
	}
	return false
}
