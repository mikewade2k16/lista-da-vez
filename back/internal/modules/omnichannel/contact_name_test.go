package omnichannel

import "testing"

func TestLikelyPersonalName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		raw  string
		want bool
	}{
		{raw: "Tamara", want: true},
		{raw: "Maria de Fátima", want: true},
		{raw: "João-Pedro", want: true},
		{raw: "Deus é fiel", want: false},
		{raw: "Só gratidão", want: false},
		{raw: "Loja Crow Visuals", want: false},
		{raw: "5511999999999", want: false},
		{raw: "@tamara", want: false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.raw, func(t *testing.T) {
			t.Parallel()
			_, got := likelyPersonalName(tt.raw)
			if got != tt.want {
				t.Fatalf("likelyPersonalName(%q) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestSafePreferredPersonalNameFallsBackToIdentity(t *testing.T) {
	t.Parallel()
	name, source := safePreferredPersonalName("Deus é fiel", "Tamara")
	if name != "Tamara" || source != "channel" {
		t.Fatalf("got %q/%q", name, source)
	}
}
