package observability

import (
	"strings"
	"testing"
)

func sampleLayoutItems() []notifyItem { return sampleItems(t0) }

func TestBuildEmailsDefaultLayoutMatchesDigest(t *testing.T) {
	mails := buildEmails(sampleLayoutItems(), "https://app/x", defaultEmailSettings())
	if len(mails) != 1 {
		t.Fatalf("agrupado = 1 e-mail, veio %d", len(mails))
	}
	subject, body := buildDigest(sampleLayoutItems(), "https://app/x")
	if mails[0].Subject != subject || mails[0].Body != body {
		t.Errorf("o layout padrão deve igualar buildDigest")
	}
	if !strings.HasPrefix(subject, "[Omni] 3 alerta(s) ativo(s) - CRITICO") {
		t.Errorf("assunto: %q", subject)
	}
}

func TestBuildEmailsLayoutOptions(t *testing.T) {
	layout := EmailSettings{SubjectPrefix: "[Loja]", Group: true, Sections: []string{sectionSummary}, IncludeResolved: false, Footer: "Equipe de plantão"}
	mails := buildEmails(sampleLayoutItems(), "https://app/x", layout)
	if len(mails) != 1 {
		t.Fatalf("e-mails: %d", len(mails))
	}
	m := mails[0]
	if !strings.HasPrefix(m.Subject, "[Loja] 3 alerta(s) ativo(s)") {
		t.Errorf("assunto: %q", m.Subject)
	}
	if strings.Contains(m.Body, "Painel:") || strings.Contains(m.Body, "Onde:") || strings.Contains(m.Body, "RESOLVIDOS") {
		t.Errorf("seções/resolvidos deveriam sair:\n%s", m.Body)
	}
	if !strings.Contains(m.Body, "Resumo do monitoramento") || !strings.Contains(m.Body, "Taxa de erros 5xx") || !strings.HasSuffix(m.Body, "Equipe de plantão\n") {
		t.Errorf("corpo:\n%s", m.Body)
	}

	layout.Sections = []string{sectionDetails, sectionLink}
	body := buildEmails(sampleLayoutItems(), "https://app/x", layout)[0].Body
	if strings.Contains(body, "Resumo do monitoramento") || !strings.Contains(body, "Onde: api") || !strings.Contains(body, "Painel: https://app/x") {
		t.Errorf("detalhes e link:\n%s", body)
	}
}

func TestBuildEmailsUngrouped(t *testing.T) {
	layout := defaultEmailSettings()
	layout.Group = false
	mails := buildEmails(sampleLayoutItems(), "", layout)
	if len(mails) != 4 {
		t.Fatalf("um e-mail por alerta: %d", len(mails))
	}
	if mails[0].Subject != "[Omni] CRITICO - Taxa de erros 5xx da API alta" || !strings.HasPrefix(mails[3].Subject, "[Omni] Resolvido - ") {
		t.Errorf("assuntos: %q / %q", mails[0].Subject, mails[3].Subject)
	}
	layout.IncludeResolved = false
	if mails = buildEmails(sampleLayoutItems(), "", layout); len(mails) != 3 {
		t.Errorf("sem resolvidos: %d", len(mails))
	}
	only := []notifyItem{{Alert: alertRow{Severity: sevLow, Title: "x", Detail: map[string]any{}}, Reason: reasonResolved}}
	layout.Group = true
	if mails = buildEmails(only, "", layout); len(mails) != 0 {
		t.Errorf("só resolvidos e includeResolved=false não envia nada")
	}
}

func TestPreviewEmail(t *testing.T) {
	layout := defaultEmailSettings()
	mail := previewEmail(layout, "https://app/x", t0)
	if mail.Subject == "" || !strings.Contains(mail.Body, "NOVOS") || !strings.Contains(mail.Body, "RESOLVIDOS") {
		t.Errorf("prévia: %+v", mail)
	}
	layout.Group = false
	if mail = previewEmail(layout, "", t0); !strings.Contains(mail.Body, "seriam enviados 4 e-mails") {
		t.Errorf("prévia sem agrupar deve avisar:\n%s", mail.Body)
	}
}
