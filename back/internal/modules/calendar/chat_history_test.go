package calendar

import (
	"strings"
	"testing"
)

// toHistory deve reanexar um resumo dos cards ao content das mensagens do assistente, para a
// IA "lembrar" o que propos numa correcao posterior (bug: "nao sei o que veio nos cards").
func TestToHistoryAppendsProposalSummary(t *testing.T) {
	t.Parallel()
	msgs := []ChatMessage{
		{Role: chatRoleUser, Content: "cria posts pra Duby e Bari"},
		{Role: chatRoleAssistant, Content: "Preparei a proposta abaixo.", Proposals: []StoredProposal{
			{ID: "0", Action: "create", Kind: "event", Status: "pending", Fields: ChatProposalFields{
				Title: "Post lancamento", Date: "2026-07-15", Time: "09:00", Type: "post", ClientName: "Duby",
			}},
			{ID: "1", Action: "create", Kind: "task", Status: "accepted", Fields: ChatProposalFields{
				Title: "Gravacao", ClientName: "Bari",
			}},
		}},
	}
	got := toHistory(msgs)
	if len(got) != 2 {
		t.Fatalf("esperava 2 mensagens, veio %d", len(got))
	}
	// A pergunta do usuario nao muda.
	if got[0].Content != "cria posts pra Duby e Bari" {
		t.Fatalf("mensagem do usuario nao deveria mudar: %q", got[0].Content)
	}
	a := got[1].Content
	for _, want := range []string{"Preparei a proposta abaixo.", "criar evento", `"Post lancamento"`,
		"2026-07-15 09:00", "cliente Duby", "(pendente)", "criar tarefa", "cliente Bari", "(aprovado)"} {
		if !strings.Contains(a, want) {
			t.Fatalf("resumo do assistente sem %q:\n%s", want, a)
		}
	}
}

// Mensagem do assistente sem cards mantem o content intacto (sem sufixo de resumo).
func TestToHistoryLeavesPlainAssistantUntouched(t *testing.T) {
	t.Parallel()
	msgs := []ChatMessage{{Role: chatRoleAssistant, Content: "Temos 3 eventos em julho."}}
	got := toHistory(msgs)
	if got[0].Content != "Temos 3 eventos em julho." {
		t.Fatalf("content nao deveria ganhar resumo: %q", got[0].Content)
	}
}

// O resumo respeita maxHistoryCardsPerMsg e sinaliza o excedente.
func TestSummarizeStoredProposalsCapsAndCounts(t *testing.T) {
	t.Parallel()
	list := make([]StoredProposal, 0, maxHistoryCardsPerMsg+5)
	for i := 0; i < maxHistoryCardsPerMsg+5; i++ {
		list = append(list, StoredProposal{
			Action: "create", Kind: "event", Status: "pending",
			Fields: ChatProposalFields{Title: "Card"},
		})
	}
	got := summarizeStoredProposals(list)
	if !strings.Contains(got, "(+5 outros)]") {
		t.Fatalf("esperava sinal de excedente, veio: %s", got)
	}
	if strings.Count(got, "criar evento") != maxHistoryCardsPerMsg {
		t.Fatalf("esperava %d cards no resumo, veio %d", maxHistoryCardsPerMsg, strings.Count(got, "criar evento"))
	}
}

func TestSummarizeStoredProposalsEmpty(t *testing.T) {
	t.Parallel()
	if got := summarizeStoredProposals(nil); got != "" {
		t.Fatalf("sem cards deveria ser vazio, veio: %q", got)
	}
}

// describeStoredProposal cobre note/clientProfile alem de event/task.
func TestDescribeStoredProposalNoteAndProfile(t *testing.T) {
	t.Parallel()
	note := describeStoredProposal(StoredProposal{
		Action: "update", Kind: "note", Status: "pending",
		Fields: ChatProposalFields{Note: &ChatProposalNote{Content: "Foco em lancamentos", Month: "2026-07"}},
	})
	for _, want := range []string{"editar anotacao", `"Foco em lancamentos"`, "mes 2026-07", "(pendente)"} {
		if !strings.Contains(note, want) {
			t.Fatalf("note sem %q: %s", want, note)
		}
	}
	prof := describeStoredProposal(StoredProposal{
		Action: "create", Kind: "clientProfile", Status: "rejected",
		Fields: ChatProposalFields{ClientName: "Perola", Profile: &ChatProposalProfile{Segment: "moda"}},
	})
	for _, want := range []string{"criar perfil do cliente", "de Perola", "(recusado)"} {
		if !strings.Contains(prof, want) {
			t.Fatalf("profile sem %q: %s", want, prof)
		}
	}
}

// fillLeanClientNames nomeia o cliente visivel SEM evento/perfil (bug 1: "faltou Duby/Bari"),
// mas nunca sobrescreve um nome que ja veio do banco.
func TestFillLeanClientNames(t *testing.T) {
	t.Parallel()
	clients := []AIContextClientLean{
		{ID: "id-duby", Name: ""},
		{ID: "id-perola", Name: "Perola"},
		{ID: "id-fora", Name: ""},
	}
	fillLeanClientNames(clients, map[string]string{"id-duby": "Duby", "id-perola": "OUTRO"})
	if clients[0].Name != "Duby" {
		t.Fatalf("cliente sem nome deveria virar Duby, veio %q", clients[0].Name)
	}
	if clients[1].Name != "Perola" {
		t.Fatalf("nome do banco nao deveria ser sobrescrito, veio %q", clients[1].Name)
	}
	if clients[2].Name != "" {
		t.Fatalf("cliente ausente do mapa deveria seguir sem nome, veio %q", clients[2].Name)
	}
}
