package omnichannel

import "testing"

func TestIsWhatsAppGroupExternalID(t *testing.T) {
	t.Parallel()
	cases := []struct {
		value string
		want  bool
	}{
		{value: "120363012345678@g.us", want: true},
		{value: " 120363012345678@G.US ", want: true},
		{value: "5511999999999@s.whatsapp.net", want: false},
		{value: "", want: false},
	}
	for _, tc := range cases {
		if got := isWhatsAppGroupExternalID(tc.value); got != tc.want {
			t.Errorf("isWhatsAppGroupExternalID(%q) = %v, want %v", tc.value, got, tc.want)
		}
	}
}
