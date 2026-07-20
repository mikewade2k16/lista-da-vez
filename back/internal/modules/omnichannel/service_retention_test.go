package omnichannel

import "testing"

// A cadeia de retencao (C2) termina SEMPRE numa constante Go — NUNCA em "sem limite". Estes
// testes travam essa regra: conta -> plataforma -> default, com faixa 1..maxDays.

func TestBuildPolicyFallbackChain(t *testing.T) {
	account := map[string]int{"conversation": 30}         // conta vence
	platform := map[string]int{"audit": 200, "ai_io": 45} // plataforma vence onde a conta cala
	// ephemeral: ausente nos dois -> default 30.

	p := buildPolicy(account, platform, defaultMaxRetentionDays)

	cases := []struct {
		name   string
		got    ClassRetention
		days   int
		source string
	}{
		{"conversation", p.Conversation, 30, "account"},
		{"audit", p.Audit, 200, "platform"},
		{"ai_io", p.AIIO, 45, "platform"},
		{"ephemeral", p.Ephemeral, defaultEphemeralDays, "default"},
	}
	for _, c := range cases {
		if c.got.Days != c.days || c.got.Source != c.source {
			t.Errorf("%s = {%d, %q}, quer {%d, %q}", c.name, c.got.Days, c.got.Source, c.days, c.source)
		}
	}
}

func TestPickDaysRejectsOutOfRange(t *testing.T) {
	// 0/negativo = "desligar retencao" pelo painel: PROIBIDO (cai para o default).
	// Acima do teto tambem cai. Nunca vira "nunca apagar".
	account := map[string]int{
		"conversation": 0,                           // invalido -> default
		"audit":        -5,                          // invalido -> default
		"ai_io":        defaultMaxRetentionDays + 1, // acima do teto -> default
		"ephemeral":    45,                          // valido
	}
	p := buildPolicy(account, nil, defaultMaxRetentionDays)

	if p.Conversation.Days != defaultConversationDays || p.Conversation.Source != "default" {
		t.Errorf("conversation=0 devia cair no default, veio {%d,%q}", p.Conversation.Days, p.Conversation.Source)
	}
	if p.Audit.Days != defaultAuditDays || p.Audit.Source != "default" {
		t.Errorf("audit negativo devia cair no default, veio {%d,%q}", p.Audit.Days, p.Audit.Source)
	}
	if p.AIIO.Days != defaultAIIODays || p.AIIO.Source != "default" {
		t.Errorf("ai_io acima do teto devia cair no default, veio {%d,%q}", p.AIIO.Days, p.AIIO.Source)
	}
	if p.Ephemeral.Days != 45 || p.Ephemeral.Source != "account" {
		t.Errorf("ephemeral=45 devia valer, veio {%d,%q}", p.Ephemeral.Days, p.Ephemeral.Source)
	}
}

func TestBuildPolicyEmptyFallsToDefaults(t *testing.T) {
	// Config ausente = o caso comum. Tem de cair na constante, nunca "nunca apagar".
	p := buildPolicy(nil, nil, 0) // maxDays 0 tambem cai no default interno
	if p.Audit.Days != defaultAuditDays || p.Conversation.Days != defaultConversationDays ||
		p.AIIO.Days != defaultAIIODays || p.Ephemeral.Days != defaultEphemeralDays {
		t.Fatalf("politica vazia devia ser toda default: %+v", p)
	}
	if p.Audit.Source != "default" {
		t.Errorf("source devia ser default, veio %q", p.Audit.Source)
	}
}

func TestValidDays(t *testing.T) {
	if validDays(0, 3650) || validDays(-1, 3650) || validDays(3651, 3650) {
		t.Error("0/negativo/acima do teto deviam ser invalidos")
	}
	if !validDays(1, 3650) || !validDays(3650, 3650) {
		t.Error("1 e o teto exato deviam ser validos")
	}
}
