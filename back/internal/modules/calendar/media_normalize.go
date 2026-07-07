package calendar

import "strings"

// Sanitizacao da midia de eventos/dia (MediaItem). Extraido de service.go para
// manter o arquivo sob o limite de 450 linhas; a regra de negocio nao mudou.

// mediaPrefix e o prefixo obrigatorio da midia interna, amarrado a account do
// Principal: /uploads/calendar/{accountId}/. Isolamento multi-tenant (contrato C1):
// o file server em /uploads/ e publico e sem escopo de conta, entao a validacao do
// prefixo com o accountId e a UNICA barreira contra referenciar arquivo de outra
// conta. accountID vazio (nao deveria ocorrer no fluxo autenticado) => "" => nenhuma
// url interna casa, toda midia e descartada (fail-safe).
func mediaPrefix(accountID string) string {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return ""
	}
	return "/uploads/calendar/" + accountID + "/"
}

// normalizeMedia sanitiza a lista de anexos: descarta itens cujo url nao esteja sob
// /uploads/calendar/{accountId}/, normaliza type (image/video) e nao deixa size
// negativo. Defesa contra injecao de URL arbitraria (inclusive de outra conta) no
// jsonb da account. accountID = dono do calendario (do Principal, nunca do body).
func normalizeMedia(accountID string, items []MediaItem) []MediaItem {
	prefix := mediaPrefix(accountID)
	out := make([]MediaItem, 0, len(items))
	for _, m := range items {
		m.URL = strings.TrimSpace(m.URL)
		if prefix == "" || !strings.HasPrefix(m.URL, prefix) {
			continue
		}
		m.Type = strings.ToLower(strings.TrimSpace(m.Type))
		if m.Type != "video" {
			m.Type = "image"
		}
		m.ID = strings.TrimSpace(m.ID)
		m.Name = strings.TrimSpace(m.Name)
		m.ContentType = strings.TrimSpace(m.ContentType)
		if m.SizeBytes < 0 {
			m.SizeBytes = 0
		}
		// posterUrl passa pela MESMA regra de prefixo com o accountId do url; se for
		// externo/de outra conta apenas zera o campo (nao descarta o item inteiro).
		m.PosterURL = strings.TrimSpace(m.PosterURL)
		if !strings.HasPrefix(m.PosterURL, prefix) {
			m.PosterURL = ""
		}
		// WAVE 6: clientId/eventId do anexo = UUID valido ou "" (sem cliente/sem item). O
		// eventId liga o anexo a um evento do dia (e a task vinculada). Nao-UUID descartado.
		m.ClientID = normalizeUUID(m.ClientID)
		m.EventID = normalizeUUID(m.EventID)
		out = append(out, m)
	}
	return out
}
